package subscriptions

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

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

// BillingPortal is implemented by providers that host checkout and self-service
// subscription management. It is optional: a provider that only ingests
// webhooks does not need to implement it.
type BillingPortal interface {
	StartCheckout(ctx context.Context, request CheckoutRequest) (RedirectSession, error)
	StartPortal(ctx context.Context, request PortalRequest) (RedirectSession, error)
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
