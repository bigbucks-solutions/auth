// Package subscriptions provides an optional, provider-agnostic billing layer.
//
// The package is inert unless it is explicitly enabled in configuration: a
// disabled module installs AllowAllPolicy, so every entitlement check passes and
// no billing tables are read. Provider adapters (for example contrib/stripe)
// register themselves through Register and are selected by configuration.
package subscriptions

import (
	"bigbucks/solution/auth/loging"
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// Mode controls how strictly entitlement checks are applied.
type Mode string

const (
	// ModeDisabled skips all checks. The billing tables are never read.
	ModeDisabled Mode = "disabled"
	// ModeObserve evaluates every check and logs what would have been denied,
	// but always allows the request. Use this to measure impact before enforcing.
	ModeObserve Mode = "observe"
	// ModeEnforce evaluates every check and denies on failure.
	ModeEnforce Mode = "enforce"
)

var (
	// ErrNotEntitled means the organization has no active subscription.
	ErrNotEntitled = errors.New("organization does not have an active subscription")
	// ErrNoLicenses means the organization has no free licenses left.
	ErrNoLicenses = errors.New("organization has no available licenses")
	// ErrFeatureUnavailable means the plan does not include the requested feature.
	ErrFeatureUnavailable = errors.New("organization plan does not include this feature")
	// ErrProviderUnsupported means the configured provider cannot serve the request.
	ErrProviderUnsupported = errors.New("subscription provider does not support this operation")
)

// Product describes what one catalog entry grants. Entries are keyed by the
// provider's price identifier (for Stripe, a price_... id).
//
// Licences are resolved from configuration at read time rather than being
// frozen into the database, so changing the catalog takes effect immediately
// without replaying webhooks.
type Product struct {
	// Name is a human-readable label surfaced to the frontend.
	Name string `json:"name" mapstructure:"name"`
	// Tier groups the prices that sell the same plan at different billing
	// intervals, so a pricing page can render one column with a monthly/annual
	// toggle instead of one column per price.
	Tier string `json:"tier" mapstructure:"tier"`
	// Order sorts tiers on the pricing page, cheapest first.
	Order int `json:"order" mapstructure:"order"`
	// Highlight marks the tier a pricing page should emphasise.
	Highlight bool `json:"highlight" mapstructure:"highlight"`
	// Description is a short marketing line shown under the tier name.
	Description string `json:"description" mapstructure:"description"`

	// Features are capability keys checked by RequireFeature. Each should have
	// a matching entry in Config.Features to give it a display label.
	Features []string `json:"features" mapstructure:"features"`
	// LicensesPerUnit is multiplied by the purchased quantity. Use this for
	// per-seat prices.
	LicensesPerUnit int64 `json:"licensesPerUnit" mapstructure:"licensesPerUnit"`
	// IncludedLicenses is added once regardless of quantity. Use this for flat
	// plans that bundle a fixed number of licences.
	IncludedLicenses int64 `json:"includedLicenses" mapstructure:"includedLicenses"`

	// MonthlyQuotas are allowances that reset every month, such as invoices.
	// A negative value means unlimited.
	MonthlyQuotas map[string]int64 `json:"monthlyQuotas" mapstructure:"monthlyQuotas"`
	// Caps are point-in-time limits on stored records, such as SKUs. They never
	// reset; deleting a record frees capacity. A negative value means unlimited.
	Caps map[string]int64 `json:"caps" mapstructure:"caps"`
}

// FeatureDefinition is the display metadata for a capability key.
//
// The key itself is the enforcement identifier and must never change; these
// fields are copy and may.
type FeatureDefinition struct {
	Label       string `json:"label" mapstructure:"label"`
	Description string `json:"description" mapstructure:"description"`
	// Order sorts the feature within a pricing page's comparison list.
	Order int `json:"order" mapstructure:"order"`
}

// LimitDefinition is the display metadata for a quota or cap key.
type LimitDefinition struct {
	Label string `json:"label" mapstructure:"label"`
	// Unit is the noun being counted, for rendering "5,000 invoices".
	Unit  string `json:"unit" mapstructure:"unit"`
	Order int    `json:"order" mapstructure:"order"`
}

// Licenses returns how many licences this product grants at the given quantity.
func (product Product) Licenses(quantity int64) int64 {
	if quantity < 0 {
		quantity = 0
	}
	return product.IncludedLicenses + product.LicensesPerUnit*quantity
}

// Config selects and configures an optional subscription provider.
type Config struct {
	Enabled  bool   `json:"enabled" mapstructure:"enabled"`
	Mode     Mode   `json:"mode" mapstructure:"mode"`
	Provider string `json:"provider" mapstructure:"provider"`
	// Catalog maps a provider price id to what it grants.
	Catalog map[string]Product `json:"catalog" mapstructure:"catalog"`
	// Features describes every capability key the catalog can reference, so a
	// pricing page can render one comparison row per feature — including the
	// ones a given tier does *not* include. Defined once here rather than
	// repeated against every price.
	Features map[string]FeatureDefinition `json:"features" mapstructure:"features"`
	// Limits describes every quota and cap key, for the same reason.
	Limits map[string]LimitDefinition `json:"limits" mapstructure:"limits"`
	// Currencies decides which currency an organization is quoted in.
	Currencies CurrencyConfig `json:"currencies" mapstructure:"currencies"`
	// ReservePendingInvitations counts unexpired pending invitations against the
	// licence limit, so an administrator is told at invite time rather than
	// letting invitees fail at accept time. Defaults to true.
	ReservePendingInvitations *bool `json:"reservePendingInvitations" mapstructure:"reservePendingInvitations"`
	// Options carries provider-specific settings.
	Options map[string]string `json:"options" mapstructure:"options"`
}

// EffectiveMode resolves Mode, falling back to Enabled for configurations that
// only set the boolean.
func (config Config) EffectiveMode() Mode {
	if mode := Mode(strings.ToLower(strings.TrimSpace(string(config.Mode)))); mode != "" {
		return mode
	}
	if config.Enabled {
		return ModeEnforce
	}
	return ModeDisabled
}

// ReservesPendingInvitations reports whether pending invitations consume a
// licence. Defaults to true when unset.
func (config Config) ReservesPendingInvitations() bool {
	return config.ReservePendingInvitations == nil || *config.ReservePendingInvitations
}

// Route describes an unauthenticated HTTP endpoint owned by a provider.
//
// Only endpoints that authenticate themselves belong here — in practice, signed
// webhooks. Anything that needs a session goes through the normal REST handler
// chain so it inherits authentication and permission checks.
type Route struct {
	Method  string
	Path    string
	Handler http.Handler
}

// Provider is implemented by billing-engine adapters.
type Provider interface {
	Name() string
	// Webhooks returns endpoints the provider verifies itself.
	Webhooks() []Route
}

// RedirectSession is a provider-hosted page the browser should be sent to.
type RedirectSession struct {
	// ID is the provider's session identifier, useful for logging.
	ID string `json:"id,omitempty"`
	// URL is the destination the frontend must redirect to.
	URL string `json:"url"`
}

// CheckoutRequest asks a provider to start a purchase.
//
// OrgName and OwnerEmail are supplied by the caller rather than looked up here,
// which keeps this package independent of the application's data models.
type CheckoutRequest struct {
	OrgID      string
	OrgName    string
	OwnerEmail string
	PriceID    string
	Quantity   int64
	// Currency is the ISO-4217 code to charge in. Empty uses the price's
	// default. It must be one the price actually offers, or the provider
	// rejects the session.
	Currency   string
	SuccessURL string
	CancelURL  string
}

// PortalFlow deep-links the provider's portal straight to one task, so a button
// on your billing page lands on that screen instead of the portal's home.
type PortalFlow string

const (
	// PortalFlowHome opens the portal's overview.
	PortalFlowHome PortalFlow = ""
	// PortalFlowCancel opens the cancellation flow.
	PortalFlowCancel PortalFlow = "cancel"
	// PortalFlowUpdatePlan opens plan selection, used for both changing tier and
	// changing the number of licences.
	PortalFlowUpdatePlan PortalFlow = "update_plan"
	// PortalFlowPaymentMethod opens card entry.
	PortalFlowPaymentMethod PortalFlow = "payment_method"
)

// PortalRequest asks a provider to open its self-service management page.
type PortalRequest struct {
	OrgID      string
	OrgName    string
	OwnerEmail string
	ReturnURL  string
	// Flow deep-links to a specific task. Empty opens the portal home.
	Flow PortalFlow
	// SubscriptionID is required by the cancel and plan-change flows, which act
	// on one subscription rather than the whole account.
	SubscriptionID string
}

// Invoice is one past or upcoming bill, for rendering billing history in your
// own UI rather than sending the customer to the provider.
type Invoice struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	// Status is the provider's raw status: paid, open, draft, void, uncollectible.
	Status   string `json:"status"`
	Currency string `json:"currency"`
	// Total and AmountPaid are in the currency's smallest unit.
	Total       int64      `json:"total"`
	AmountPaid  int64      `json:"amount_paid"`
	Created     time.Time  `json:"created"`
	PeriodStart *time.Time `json:"period_start"`
	PeriodEnd   *time.Time `json:"period_end"`
	// HostedURL is the provider-hosted invoice page.
	HostedURL string `json:"hosted_url,omitempty"`
	// PDFURL downloads the invoice.
	PDFURL string `json:"pdf_url,omitempty"`
}

// InvoiceLister is implemented by providers that can report billing history.
type InvoiceLister interface {
	ListInvoices(ctx context.Context, orgID string, limit int64) ([]Invoice, error)
}

// CatalogValidator is implemented by providers that can check the configured
// catalog against what actually exists at the provider.
//
// The returned error must distinguish a genuine mismatch, which should stop the
// process, from an inability to reach the provider, which should not — see
// ErrValidationUnavailable.
type CatalogValidator interface {
	ValidateCatalog(ctx context.Context, catalog map[string]Product) error
}

// ErrValidationUnavailable means the catalog could not be checked, as opposed to
// being checked and found wrong. A provider outage must not stop a deploy.
var ErrValidationUnavailable = errors.New("catalog validation unavailable")

// Validate reports configuration mistakes that need no provider to detect.
//
// These are all cases where the deployment would start happily and then behave
// wrongly, so they are worth failing on rather than logging.
func (config Config) Validate() error {
	if config.EffectiveMode() == ModeDisabled {
		return nil
	}

	var problems []string
	for priceID, product := range config.Catalog {
		// A plan that grants no licences would sell access nobody can use: the
		// owner alone already puts the organization over its limit.
		if product.LicensesPerUnit <= 0 && product.IncludedLicenses <= 0 {
			problems = append(problems, fmt.Sprintf(
				"%s grants no licences; set licensesPerUnit or includedLicenses", priceID))
		}
		for _, feature := range product.Features {
			if _, described := config.Features[feature]; !described {
				problems = append(problems, fmt.Sprintf(
					"%s references feature %q which has no entry under subscriptions.features",
					priceID, feature))
			}
		}
		for limit := range product.MonthlyQuotas {
			if _, described := config.Limits[limit]; !described {
				problems = append(problems, fmt.Sprintf(
					"%s references quota %q which has no entry under subscriptions.limits",
					priceID, limit))
			}
		}
		for limit := range product.Caps {
			if _, described := config.Limits[limit]; !described {
				problems = append(problems, fmt.Sprintf(
					"%s references cap %q which has no entry under subscriptions.limits",
					priceID, limit))
			}
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("invalid subscription catalog:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}

// BillingPortal is implemented by providers that host checkout and self-service
// subscription management. It is optional: a provider that only ingests
// webhooks does not need to implement it.
type BillingPortal interface {
	StartCheckout(ctx context.Context, request CheckoutRequest) (RedirectSession, error)
	StartPortal(ctx context.Context, request PortalRequest) (RedirectSession, error)
}

// Amount is what one price costs in a single currency.
//
// Values are in the currency's smallest unit: fils for AED, cents for USD.
type Amount struct {
	Currency string `json:"currency"`
	// FirstUnitAmount is what the first licence costs. For a graduated price
	// this is tier one; for a flat price it is the unit amount.
	FirstUnitAmount int64 `json:"first_unit_amount"`
	// AdditionalUnitAmount is what each licence beyond the first costs. Equal to
	// FirstUnitAmount on a flat per-unit price.
	AdditionalUnitAmount int64 `json:"additional_unit_amount"`
	// Tiered reports whether the price uses graduated tiers, so a pricing page
	// knows whether to show "from X" plus a per-seat line.
	Tiered bool `json:"tiered"`
}

// PriceDetail is the sellable shape of one catalog price, in every currency the
// provider will accept for it.
type PriceDetail struct {
	PriceID string `json:"price_id"`
	// Interval is "month" or "year".
	Interval string `json:"interval"`
	// DefaultCurrency is what the provider charges when none is requested.
	DefaultCurrency string `json:"default_currency"`
	// Amounts is keyed by lower-case ISO-4217 code.
	Amounts map[string]Amount `json:"amounts"`
}

// AmountIn returns the price in a currency, falling back to the default.
func (detail PriceDetail) AmountIn(currency string) (Amount, bool) {
	if amount, ok := detail.Amounts[strings.ToLower(strings.TrimSpace(currency))]; ok {
		return amount, true
	}
	amount, ok := detail.Amounts[detail.DefaultCurrency]
	return amount, ok
}

// Currencies lists the currencies this price can be bought in, sorted.
func (detail PriceDetail) Currencies() []string {
	currencies := make([]string, 0, len(detail.Amounts))
	for currency := range detail.Amounts {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)
	return currencies
}

// CurrencyConfig decides which currency an organization is quoted in.
type CurrencyConfig struct {
	// Default is used when the organization's country is unknown or unmapped.
	Default string `json:"default" mapstructure:"default"`
	// ByCountry maps an ISO-3166 alpha-2 country code to an ISO-4217 currency.
	// Keys are matched case-insensitively.
	ByCountry map[string]string `json:"byCountry" mapstructure:"byCountry"`
}

// MapsCountries reports whether any country-to-currency mapping is configured.
//
// Callers use this to skip looking an organization's country up at all when
// nothing would be done with it.
func (config CurrencyConfig) MapsCountries() bool {
	return len(config.ByCountry) > 0
}

// For resolves the currency to quote a country in. An empty or unmapped country
// falls back to Default.
//
// Matching is case-insensitive on ISO-3166 alpha-2 codes. Anything else — an
// alpha-3 code, or a country name like "United Arab Emirates" — will not match;
// see LooksLikeCountryCode.
func (config CurrencyConfig) For(country string) string {
	country = strings.ToUpper(strings.TrimSpace(country))
	if country != "" {
		for candidate, currency := range config.ByCountry {
			if strings.ToUpper(strings.TrimSpace(candidate)) == country {
				return strings.ToLower(strings.TrimSpace(currency))
			}
		}
	}
	return strings.ToLower(strings.TrimSpace(config.Default))
}

// LooksLikeCountryCode reports whether a value is shaped like an ISO-3166
// alpha-2 code.
//
// Organization country is free text, so a value such as "UAE" would silently
// fail to map and quietly fall back to the default currency. This lets callers
// tell "unmapped country" apart from "country stored in the wrong format",
// which are very different problems.
func LooksLikeCountryCode(country string) bool {
	country = strings.TrimSpace(country)
	if len(country) != 2 {
		return false
	}
	for _, character := range country {
		if character < 'A' || character > 'z' ||
			(character > 'Z' && character < 'a') {
			return false
		}
	}
	return true
}

// PriceInspector is implemented by providers that can report what their prices
// actually cost.
//
// Amounts deliberately live at the provider, not in local configuration: a
// pricing page that advertises a figure the checkout does not charge is worse
// than one that fails to load.
type PriceInspector interface {
	InspectPrices(ctx context.Context, priceIDs []string) (map[string]PriceDetail, error)
}

// TrialProvider is implemented by providers that grant a trial when checkout
// creates a subscription. A zero duration means no trial is available.
type TrialProvider interface {
	TrialPeriodDays() int64
}

// CurrencyLocker is implemented by providers that pin a customer to a single
// currency once recurring billing has started.
//
// Stripe does exactly this: a customer's currency is set from their first
// invoice and every later subscription must match it. Without asking, a currency
// switcher would happily offer an existing AED customer a USD plan and only fail
// at checkout.
type CurrencyLocker interface {
	// LockedCurrency returns the currency an organization is already committed
	// to, or an empty string if it has not billed yet and is still free to
	// choose.
	LockedCurrency(ctx context.Context, orgID string) (string, error)
}

// Factory creates a configured provider adapter.
type Factory func(config Config, db *gorm.DB) (Provider, error)

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Factory)
)

// Register makes a provider factory available to the application. Adapters call
// this from an init function, so importing the adapter package is enough to
// make it selectable by configuration.
func Register(name string, factory Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[normalize(name)] = factory
}

func normalize(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// Module is the assembled subscription layer.
type Module struct {
	Config   Config
	Provider Provider
	Policy   Policy
}

// NewModule builds the subscription layer described by config. A disabled
// configuration returns a module with no provider and an allow-all policy, so
// callers never need to nil-check it.
func NewModule(config Config, db *gorm.DB) (*Module, error) {
	mode := config.EffectiveMode()
	switch mode {
	case ModeDisabled:
		return &Module{Config: config, Policy: AllowAllPolicy{}}, nil
	case ModeObserve, ModeEnforce:
	default:
		return nil, fmt.Errorf("invalid subscription mode %q", config.Mode)
	}

	if db == nil {
		return nil, errors.New("database connection is required when subscriptions are enabled")
	}

	name := normalize(config.Provider)
	if name == "" {
		return nil, errors.New("subscription provider is required when subscriptions are enabled")
	}
	registryMu.RLock()
	factory, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("subscription provider %q is not registered", config.Provider)
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}

	provider, err := factory(config, db)
	if err != nil {
		return nil, fmt.Errorf("configure subscription provider %q: %w", name, err)
	}

	// Check the catalog against the provider before serving traffic. A price id
	// that does not exist there would otherwise let a customer pay and receive
	// nothing, which is only visible as a log warning.
	if validator, ok := provider.(CatalogValidator); ok {
		validateCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		switch err := validator.ValidateCatalog(validateCtx, config.Catalog); {
		case err == nil:
		case errors.Is(err, ErrValidationUnavailable):
			// Being unable to check is not the same as having checked and found
			// a problem; a provider outage must not block a deploy.
			loging.Logger.Warnw("could not verify the subscription catalog against the provider",
				"provider", name, "error", err.Error())
		default:
			return nil, err
		}
	}

	var policy Policy = DatabasePolicy{Config: config}
	if mode == ModeObserve {
		policy = ObservePolicy{Inner: DatabasePolicy{Config: config}}
	}
	return &Module{Config: config, Provider: provider, Policy: policy}, nil
}

// Enabled reports whether entitlement state is backed by a real provider.
func (module *Module) Enabled() bool {
	return module != nil && module.Provider != nil
}

// Portal returns the provider's hosted checkout capability, if it has one.
func (module *Module) Portal() (BillingPortal, bool) {
	if !module.Enabled() {
		return nil, false
	}
	portal, ok := module.Provider.(BillingPortal)
	return portal, ok
}

// Prices returns the provider's price-inspection capability, if it has one.
func (module *Module) Prices() (PriceInspector, bool) {
	if !module.Enabled() {
		return nil, false
	}
	inspector, ok := module.Provider.(PriceInspector)
	return inspector, ok
}

// TrialPeriodDays returns the provider's effective checkout trial duration.
func (module *Module) TrialPeriodDays() int64 {
	if !module.Enabled() {
		return 0
	}
	provider, ok := module.Provider.(TrialProvider)
	if !ok {
		return 0
	}
	return provider.TrialPeriodDays()
}

// CurrencyLock returns the provider's currency-pinning capability, if it has one.
func (module *Module) CurrencyLock() (CurrencyLocker, bool) {
	if !module.Enabled() {
		return nil, false
	}
	locker, ok := module.Provider.(CurrencyLocker)
	return locker, ok
}

// Invoices returns the provider's billing-history capability, if it has one.
func (module *Module) Invoices() (InvoiceLister, bool) {
	if !module.Enabled() {
		return nil, false
	}
	lister, ok := module.Provider.(InvoiceLister)
	return lister, ok
}

// Webhooks returns the provider's self-verifying endpoints.
func (module *Module) Webhooks() []Route {
	if !module.Enabled() {
		return nil
	}
	return module.Provider.Webhooks()
}

var (
	moduleMu      sync.RWMutex
	currentModule = &Module{Policy: AllowAllPolicy{}}
)

// SetModule installs the process-wide module. Passing nil restores the inert
// allow-all module.
func SetModule(module *Module) {
	moduleMu.Lock()
	defer moduleMu.Unlock()
	if module == nil {
		currentModule = &Module{Policy: AllowAllPolicy{}}
		return
	}
	if module.Policy == nil {
		module.Policy = AllowAllPolicy{}
	}
	currentModule = module
}

// CurrentModule returns the installed module. It is never nil.
func CurrentModule() *Module {
	moduleMu.RLock()
	defer moduleMu.RUnlock()
	return currentModule
}

// CurrentPolicy returns the installed policy. It is never nil.
func CurrentPolicy() Policy {
	return CurrentModule().Policy
}

// IsDenial reports whether err is an entitlement denial rather than a fault.
func IsDenial(err error) bool {
	return errors.Is(err, ErrNotEntitled) ||
		errors.Is(err, ErrNoLicenses) ||
		errors.Is(err, ErrFeatureUnavailable)
}

// HTTPStatus maps an entitlement denial to the status code the API returns.
// Unknown errors map to 500.
func HTTPStatus(err error) int {
	switch {
	case errors.Is(err, ErrNotEntitled), errors.Is(err, ErrFeatureUnavailable):
		return http.StatusPaymentRequired
	case errors.Is(err, ErrNoLicenses):
		return http.StatusConflict
	case errors.Is(err, ErrProviderUnsupported):
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}
