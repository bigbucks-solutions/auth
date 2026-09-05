package subscriptions

import (
	"bigbucks/solution/auth/constants"
	"time"
)

// BillingAccount links an organization to a customer record at a billing
// provider.
//
// The unique key is (org_id, provider) rather than org_id alone so a second
// provider can be added later without migrating live billing state.
type BillingAccount struct {
	constants.BaseModel
	OrgID    string `gorm:"not null;index;uniqueIndex:idx_billing_account_org_provider" json:"org_id"`
	Provider string `gorm:"not null;uniqueIndex:idx_billing_account_org_provider" json:"provider"`
	// ProviderCustomerID is the provider's customer identifier (cus_... for Stripe).
	ProviderCustomerID string `gorm:"not null;uniqueIndex" json:"provider_customer_id"`
	// Email is the address the customer record was created with, normally the
	// organization owner's.
	Email string `json:"email"`
}

// SubscriptionItem is a projection of one purchased line at the provider.
//
// Only raw provider state is stored. What a line grants is resolved from the
// configured catalog at read time, so catalog edits apply immediately instead of
// requiring webhooks to be replayed.
type SubscriptionItem struct {
	constants.BaseModel
	AccountID string `gorm:"not null;index;uniqueIndex:idx_subscription_item_external" json:"account_id"`
	Provider  string `gorm:"not null;uniqueIndex:idx_subscription_item_external" json:"provider"`
	// ExternalID is the provider's line identifier (si_... for Stripe).
	ExternalID string `gorm:"not null;uniqueIndex:idx_subscription_item_external" json:"external_id"`
	// SubscriptionID groups lines belonging to the same provider subscription.
	SubscriptionID string `gorm:"not null;index" json:"subscription_id"`
	// PriceID keys into Config.Catalog.
	PriceID string `gorm:"not null;index" json:"price_id"`
	// Status is the provider's raw subscription status, kept for display.
	Status string `gorm:"not null" json:"status"`
	// Active is the resolved answer to "does this line currently grant access".
	Active            bool  `gorm:"not null;index" json:"active"`
	Quantity          int64 `gorm:"not null" json:"quantity"`
	CancelAtPeriodEnd bool  `gorm:"not null" json:"cancel_at_period_end"`

	CurrentPeriodStart *time.Time `json:"current_period_start"`
	CurrentPeriodEnd   *time.Time `gorm:"index" json:"current_period_end"`
	// EventAt is the provider timestamp of the event that produced this row. It
	// guards against out-of-order webhook delivery.
	EventAt time.Time `gorm:"not null" json:"event_at"`
}

// WebhookEvent records processed provider events so redelivery is idempotent.
type WebhookEvent struct {
	constants.BaseModel
	Provider  string    `gorm:"not null;uniqueIndex:idx_webhook_event_provider_event" json:"provider"`
	EventID   string    `gorm:"not null;uniqueIndex:idx_webhook_event_provider_event" json:"event_id"`
	EventType string    `gorm:"not null;index" json:"event_type"`
	EventAt   time.Time `gorm:"not null" json:"event_at"`
}
