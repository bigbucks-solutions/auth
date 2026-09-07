// Package stripe adapts Stripe Billing to the subscriptions package.
//
// Importing this package registers the adapter; it is selected by setting
// `subscriptions.provider` to "stripe" in configuration. Nothing in here runs
// unless subscriptions are enabled.
package stripe

import (
	"bigbucks/solution/auth/subscriptions"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	stripesdk "github.com/stripe/stripe-go/v82"
	"gorm.io/gorm"
)

const (
	// ProviderName is the value configuration must use to select this adapter.
	ProviderName = "stripe"
	// DefaultWebhookPath is where Stripe should be pointed if not overridden.
	DefaultWebhookPath = "/api/v1/billing/stripe/webhook"

	// metadataOrgID ties a Stripe customer back to an organization. It is the
	// authoritative link; the local billing_accounts row is a cache of it.
	metadataOrgID = "bigbucks_org_id"

	maxWebhookBytes = 1 << 20
)

// Config is the adapter's slice of subscriptions.Config.Options.
type Config struct {
	// SecretKey is the Stripe API secret (sk_...).
	SecretKey string
	// WebhookSecret verifies inbound webhook signatures (whsec_...).
	WebhookSecret string
	// WebhookPath is the route the webhook handler is mounted at.
	WebhookPath string
	// AllowPromotionCodes exposes the promo-code field in Checkout.
	AllowPromotionCodes bool
	// AllowQuantityAdjustment lets the buyer change licence count inside
	// Checkout. Enable this for per-seat prices.
	AllowQuantityAdjustment bool
	// MaxQuantity caps the adjustable quantity selector.
	MaxQuantity int64
	// TrialPeriodDays grants new checkout subscriptions a free trial. Zero
	// disables trials.
	TrialPeriodDays int64
	// GraceOnPastDue keeps access while Stripe retries a failed payment.
	// Disable it to cut access the moment an invoice goes past due.
	GraceOnPastDue bool
}

// Provider implements subscriptions.Provider and subscriptions.BillingPortal.
type Provider struct {
	config          Config
	catalog         map[string]subscriptions.Product
	priceExpansions []*string
	client          *stripesdk.Client
	db              *gorm.DB
}

// Compile-time proof the adapter satisfies both interfaces.
var (
	_ subscriptions.Provider      = (*Provider)(nil)
	_ subscriptions.BillingPortal = (*Provider)(nil)
	_ subscriptions.TrialProvider = (*Provider)(nil)
)

func init() {
	subscriptions.Register(ProviderName, New)
}

// New builds the adapter from generic subscription configuration.
func New(config subscriptions.Config, db *gorm.DB) (subscriptions.Provider, error) {
	adapterConfig, err := parseConfig(config)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, errors.New("database connection is required")
	}
	if len(config.Catalog) == 0 {
		return nil, errors.New("catalog must list at least one price")
	}
	for priceID := range config.Catalog {
		if !strings.HasPrefix(priceID, "price_") {
			return nil, fmt.Errorf("catalog key %q is not a Stripe price id", priceID)
		}
	}

	return &Provider{
		config:          adapterConfig,
		catalog:         config.Catalog,
		priceExpansions: configuredPriceExpansions(config.Currencies),
		client:          stripesdk.NewClient(adapterConfig.SecretKey),
		db:              db,
	}, nil
}

func parseConfig(config subscriptions.Config) (Config, error) {
	options := config.Options
	adapterConfig := Config{
		SecretKey:               options["secretKey"],
		WebhookSecret:           options["webhookSecret"],
		WebhookPath:             options["webhookPath"],
		AllowPromotionCodes:     boolOption(options, "allowPromotionCodes", false),
		AllowQuantityAdjustment: boolOption(options, "allowQuantityAdjustment", true),
		MaxQuantity:             intOption(options, "maxQuantity", 999),
		TrialPeriodDays:         intOption(options, "trialPeriodDays", 7),
		GraceOnPastDue:          boolOption(options, "graceOnPastDue", true),
	}

	if adapterConfig.SecretKey == "" {
		return Config{}, errors.New("options.secretKey is required (set STRIPE_SECRET_KEY)")
	}
	if !strings.HasPrefix(adapterConfig.SecretKey, "sk_") && !strings.HasPrefix(adapterConfig.SecretKey, "rk_") {
		return Config{}, errors.New("options.secretKey must be a Stripe secret or restricted key")
	}
	if adapterConfig.WebhookSecret == "" {
		return Config{}, errors.New("options.webhookSecret is required (set STRIPE_WEBHOOK_SECRET)")
	}
	if adapterConfig.WebhookPath == "" {
		adapterConfig.WebhookPath = DefaultWebhookPath
	}
	if !strings.HasPrefix(adapterConfig.WebhookPath, "/") {
		return Config{}, errors.New("options.webhookPath must start with /")
	}
	if adapterConfig.MaxQuantity < 1 {
		adapterConfig.MaxQuantity = 1
	}
	if adapterConfig.TrialPeriodDays < 0 {
		return Config{}, errors.New("options.trialPeriodDays cannot be negative")
	}
	return adapterConfig, nil
}

func boolOption(options map[string]string, key string, fallback bool) bool {
	raw, ok := options[key]
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}
	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return value
}

func intOption(options map[string]string, key string, fallback int64) int64 {
	raw, ok := options[key]
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

// Name identifies the adapter.
func (provider *Provider) Name() string { return ProviderName }

// TrialPeriodDays reports the trial Checkout applies to new subscriptions.
func (provider *Provider) TrialPeriodDays() int64 { return provider.config.TrialPeriodDays }

// Webhooks exposes the signature-verified Stripe endpoint. It must stay outside
// the authenticated handler chain: Stripe presents a signature, not a session.
func (provider *Provider) Webhooks() []subscriptions.Route {
	return []subscriptions.Route{{
		Method:  "POST",
		Path:    provider.config.WebhookPath,
		Handler: http.HandlerFunc(provider.handleWebhook),
	}}
}

// ensureCustomer returns the Stripe customer id for an organization, creating
// both the Stripe customer and the local billing account on first use.
//
// The organization is the customer; the owner's address is only where Stripe
// sends receipts. Recording org_id in customer metadata means the link survives
// even if the local row is lost.
func (provider *Provider) ensureCustomer(ctx context.Context, orgID, orgName, email string) (string, error) {
	if orgID == "" {
		return "", errors.New("organization id is required")
	}

	var account subscriptions.BillingAccount
	err := provider.db.WithContext(ctx).
		Where("org_id = ? AND provider = ?", orgID, ProviderName).
		First(&account).Error
	if err == nil {
		return account.ProviderCustomerID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	params := &stripesdk.CustomerCreateParams{
		Name:     stripesdk.String(orgName),
		Metadata: map[string]string{metadataOrgID: orgID},
	}
	if email != "" {
		params.Email = stripesdk.String(email)
	}
	customer, err := provider.client.V1Customers.Create(ctx, params)
	if err != nil {
		return "", fmt.Errorf("create stripe customer: %w", err)
	}

	account = subscriptions.BillingAccount{
		OrgID:              orgID,
		Provider:           ProviderName,
		ProviderCustomerID: customer.ID,
		Email:              email,
	}
	// A concurrent request may have created the row between the lookup and here.
	// Prefer the row that won over the customer we just made.
	if err := provider.db.WithContext(ctx).
		Where("org_id = ? AND provider = ?", orgID, ProviderName).
		FirstOrCreate(&account).Error; err != nil {
		return "", fmt.Errorf("persist billing account: %w", err)
	}
	return account.ProviderCustomerID, nil
}
