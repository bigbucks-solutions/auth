package subscriptions

import (
	"bigbucks/solution/auth/loging"
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Entitlements is the resolved billing state of one organization. It is the
// payload the frontend renders its billing screens from, and what other
// services read over gRPC to apply their own limits.
type Entitlements struct {
	OrgID string `json:"org_id"`
	// Entitled reports whether the organization may use the application.
	Entitled bool `json:"entitled"`
	// State is the provider-neutral lifecycle state. Branch on this rather than
	// Status.
	State State `json:"state"`
	// Status is the provider's raw status for the most relevant line
	// ("active", "trialing", "past_due", "canceled", ...), or "none" when the
	// organization has never held a subscription. Kept for display.
	Status string `json:"status"`
	// Licenses is the total number of members the plan allows.
	Licenses int64 `json:"licenses"`
	// LicensesUsed counts current members plus, when configured, unexpired
	// pending invitations.
	LicensesUsed int64 `json:"licenses_used"`
	// LicensesAvailable is Licenses-LicensesUsed, floored at zero.
	LicensesAvailable int64 `json:"licenses_available"`
	// OverLimit is true when more licences are consumed than the plan allows,
	// which happens after a downgrade. Existing members keep working; adding
	// more is blocked.
	OverLimit bool `json:"over_limit"`
	// Features is the union of features granted by all active lines.
	Features []string `json:"features"`
	// Limits is every cap and monthly quota the active lines grant, keyed by
	// limit key. A key absent here is not offered on the current plan.
	Limits map[string]LimitGrant `json:"limits"`
	// Plans describes each active line for display.
	Plans []PlanSummary `json:"plans"`
	// TrialEndsAt is when the current trial converts to paid. Set only while
	// trialing.
	TrialEndsAt *time.Time `json:"trial_ends_at"`
	// CurrentPeriodStart and CurrentPeriodEnd bound the earliest-renewing active
	// line's billing period.
	CurrentPeriodStart *time.Time `json:"current_period_start"`
	CurrentPeriodEnd   *time.Time `json:"current_period_end"`
	// EndedAt is when access ended. Set only for the canceled and expired
	// states.
	EndedAt *time.Time `json:"ended_at"`
	// CancelAtPeriodEnd is true when any active line is set to lapse.
	CancelAtPeriodEnd bool `json:"cancel_at_period_end"`
	// ManagedExternally is false when subscriptions are disabled, telling the
	// frontend to hide billing UI entirely.
	ManagedExternally bool `json:"managed_externally"`
	// ResolvedAt is when this snapshot was computed, so a caller holding a
	// cached copy can judge its age.
	ResolvedAt time.Time `json:"resolved_at"`
}

// PlanSummary describes one purchased line.
type PlanSummary struct {
	PriceID           string     `json:"price_id"`
	Name              string     `json:"name"`
	Tier              string     `json:"tier"`
	Quantity          int64      `json:"quantity"`
	Licenses          int64      `json:"licenses"`
	Status            string     `json:"status"`
	CancelAtPeriodEnd bool       `json:"cancel_at_period_end"`
	CurrentPeriodEnd  *time.Time `json:"current_period_end"`
}

// HasFeature reports whether the resolved plan includes a feature.
func (entitlements Entitlements) HasFeature(feature string) bool {
	for _, candidate := range entitlements.Features {
		if candidate == feature {
			return true
		}
	}
	return false
}

// Resolve reads the current entitlement state for an organization.
//
// It takes no locks and is safe on request hot paths. Use RequireLicenses when
// a decision must be serialized against concurrent membership changes.
func Resolve(ctx context.Context, tx *gorm.DB, config Config, orgID string) (Entitlements, error) {
	now := time.Now().UTC()

	items, err := activeItems(ctx, tx, orgID, now.Add(-config.PeriodEndGrace()))
	if err != nil {
		return summarize(config, orgID, nil, nil, now), err
	}
	var latest *SubscriptionItem
	if len(items) == 0 {
		if latest, err = latestItem(ctx, tx, orgID); err != nil {
			return summarize(config, orgID, nil, nil, now), err
		}
	}
	entitlements := summarize(config, orgID, items, latest, now)

	used, err := licensesUsed(ctx, tx, orgID, config.ReservesPendingInvitations())
	if err != nil {
		return entitlements, err
	}
	entitlements.LicensesUsed = used
	if available := entitlements.Licenses - used; available > 0 {
		entitlements.LicensesAvailable = available
	}
	entitlements.OverLimit = entitlements.Entitled && used > entitlements.Licenses

	return entitlements, nil
}

// activeItems returns the organization's currently-granting subscription lines.
//
// A line whose period ended after periodEndedAfter still counts. Renewal is
// only observed when the provider's webhook arrives, and a delayed delivery must
// not lock a paying organization out at every renewal.
func activeItems(ctx context.Context, tx *gorm.DB, orgID string, periodEndedAfter time.Time) ([]SubscriptionItem, error) {
	var items []SubscriptionItem
	err := tx.WithContext(ctx).
		Model(&SubscriptionItem{}).
		Joins("JOIN billing_accounts ON billing_accounts.id = subscription_items.account_id"+
			" AND billing_accounts.deleted_at IS NULL").
		Where("billing_accounts.org_id = ?", orgID).
		Where("subscription_items.active = ?", true).
		Where("subscription_items.current_period_end IS NULL OR subscription_items.current_period_end > ?", periodEndedAfter).
		Order("subscription_items.created_at").
		Find(&items).Error
	return items, err
}

// latestItem returns the organization's most recently changed line of any
// status, or nil if it has never held one.
func latestItem(ctx context.Context, tx *gorm.DB, orgID string) (*SubscriptionItem, error) {
	var items []SubscriptionItem
	err := tx.WithContext(ctx).
		Model(&SubscriptionItem{}).
		Joins("JOIN billing_accounts ON billing_accounts.id = subscription_items.account_id"+
			" AND billing_accounts.deleted_at IS NULL").
		Where("billing_accounts.org_id = ?", orgID).
		Order("subscription_items.updated_at DESC").
		Limit(1).
		Find(&items).Error
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return &items[0], nil
}

// HasSubscriptionHistory reports whether an organization has ever held a
// subscription that got past its first payment. Such an organization is no
// longer eligible for a trial.
func HasSubscriptionHistory(ctx context.Context, tx *gorm.DB, orgID string) (bool, error) {
	var count int64
	err := tx.WithContext(ctx).
		Model(&SubscriptionItem{}).
		Joins("JOIN billing_accounts ON billing_accounts.id = subscription_items.account_id").
		Where("billing_accounts.org_id = ?", orgID).
		Where("subscription_items.status NOT IN ?", []string{"incomplete", "incomplete_expired"}).
		Count(&count).Error
	return count > 0, err
}

// licensesUsed counts distinct members, plus unexpired pending invitations for
// people who are not members yet when reservation is enabled.
func licensesUsed(ctx context.Context, tx *gorm.DB, orgID string, reserveInvitations bool) (int64, error) {
	var members int64
	if err := tx.WithContext(ctx).
		Table("user_org_roles").
		Where("org_id = ?", orgID).
		Distinct("user_id").
		Count(&members).Error; err != nil {
		return 0, err
	}
	if !reserveInvitations {
		return members, nil
	}

	var pending int64
	if err := tx.WithContext(ctx).
		Table("invitations").
		Where("invitations.org_id = ?", orgID).
		Where("invitations.status = ?", "pending").
		Where("invitations.expires_at > ?", time.Now().UTC()).
		Where("invitations.deleted_at IS NULL").
		Where("NOT EXISTS (?)", tx.
			Table("users").
			Select("1").
			Joins("JOIN user_org_roles ON user_org_roles.user_id = users.id"+
				" AND user_org_roles.org_id = invitations.org_id").
			Where("users.username = invitations.email").
			Where("users.deleted_at IS NULL")).
		Count(&pending).Error; err != nil {
		return 0, err
	}
	return members + pending, nil
}

// Policy is the entitlement gate used by handlers and actions.
//
// Every method returns nil when the operation is allowed, a sentinel error
// (ErrNotEntitled, ErrNoLicenses, ErrFeatureUnavailable) when it is denied, and
// a wrapped error when the check itself failed.
type Policy interface {
	// RequireEntitled fails unless the organization has an active subscription.
	RequireEntitled(ctx context.Context, tx *gorm.DB, orgID string) error
	// RequireFeature fails unless the plan includes the named feature.
	RequireFeature(ctx context.Context, tx *gorm.DB, orgID, feature string) error
	// RequireLicenses fails unless the organization can consume `requested`
	// more licences. It serializes against concurrent callers.
	RequireLicenses(ctx context.Context, tx *gorm.DB, orgID string, requested int64) error
	// Entitlements reports current state without denying anything.
	Entitlements(ctx context.Context, tx *gorm.DB, orgID string) (Entitlements, error)
}

// AllowAllPolicy is installed when subscriptions are disabled. Every check
// passes and no billing table is read.
type AllowAllPolicy struct{}

func (AllowAllPolicy) RequireEntitled(context.Context, *gorm.DB, string) error { return nil }

func (AllowAllPolicy) RequireFeature(context.Context, *gorm.DB, string, string) error { return nil }

func (AllowAllPolicy) RequireLicenses(context.Context, *gorm.DB, string, int64) error { return nil }

func (AllowAllPolicy) Entitlements(_ context.Context, _ *gorm.DB, orgID string) (Entitlements, error) {
	return Entitlements{
		OrgID:             orgID,
		Entitled:          true,
		State:             StateNotManaged,
		Status:            "not_managed",
		Features:          []string{},
		Limits:            map[string]LimitGrant{},
		Plans:             []PlanSummary{},
		ManagedExternally: false,
		ResolvedAt:        time.Now().UTC(),
	}, nil
}

// DatabasePolicy enforces entitlements from the projected billing tables.
type DatabasePolicy struct {
	Config Config
}

func (policy DatabasePolicy) Entitlements(ctx context.Context, tx *gorm.DB, orgID string) (Entitlements, error) {
	return Resolve(ctx, tx, policy.Config, orgID)
}

func (policy DatabasePolicy) RequireEntitled(ctx context.Context, tx *gorm.DB, orgID string) error {
	if orgID == "" {
		return ErrNotEntitled
	}
	entitlements, err := Resolve(ctx, tx, policy.Config, orgID)
	if err != nil {
		return fmt.Errorf("resolve entitlements: %w", err)
	}
	if !entitlements.Entitled {
		return ErrNotEntitled
	}
	return nil
}

func (policy DatabasePolicy) RequireFeature(ctx context.Context, tx *gorm.DB, orgID, feature string) error {
	if orgID == "" {
		return ErrNotEntitled
	}
	entitlements, err := Resolve(ctx, tx, policy.Config, orgID)
	if err != nil {
		return fmt.Errorf("resolve entitlements: %w", err)
	}
	if !entitlements.Entitled {
		return ErrNotEntitled
	}
	if !entitlements.HasFeature(feature) {
		return fmt.Errorf("%w: %s", ErrFeatureUnavailable, feature)
	}
	return nil
}

// RequireLicenses locks the organization's billing accounts for the remainder of
// the transaction so two concurrent membership additions cannot both observe the
// same free licence. Callers must therefore run inside a transaction that also
// writes the membership.
func (policy DatabasePolicy) RequireLicenses(ctx context.Context, tx *gorm.DB, orgID string, requested int64) error {
	if orgID == "" {
		return ErrNotEntitled
	}
	if requested <= 0 {
		return nil
	}

	var accounts []BillingAccount
	if err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("org_id = ?", orgID).
		Find(&accounts).Error; err != nil {
		return fmt.Errorf("lock billing account: %w", err)
	}
	if len(accounts) == 0 {
		return ErrNotEntitled
	}

	entitlements, err := Resolve(ctx, tx, policy.Config, orgID)
	if err != nil {
		return fmt.Errorf("resolve entitlements: %w", err)
	}
	if !entitlements.Entitled {
		return ErrNotEntitled
	}
	if entitlements.LicensesUsed+requested > entitlements.Licenses {
		return fmt.Errorf("%w: plan allows %d, %d in use",
			ErrNoLicenses, entitlements.Licenses, entitlements.LicensesUsed)
	}
	return nil
}

// ObservePolicy evaluates the wrapped policy and logs denials without applying
// them. It exists so a deployment can measure the impact of enforcement before
// switching it on.
type ObservePolicy struct {
	Inner Policy
}

func (policy ObservePolicy) Entitlements(ctx context.Context, tx *gorm.DB, orgID string) (Entitlements, error) {
	return policy.Inner.Entitlements(ctx, tx, orgID)
}

func (policy ObservePolicy) RequireEntitled(ctx context.Context, tx *gorm.DB, orgID string) error {
	return policy.observe(orgID, "entitled", policy.Inner.RequireEntitled(ctx, tx, orgID))
}

func (policy ObservePolicy) RequireFeature(ctx context.Context, tx *gorm.DB, orgID, feature string) error {
	return policy.observe(orgID, "feature:"+feature, policy.Inner.RequireFeature(ctx, tx, orgID, feature))
}

func (policy ObservePolicy) RequireLicenses(ctx context.Context, tx *gorm.DB, orgID string, requested int64) error {
	return policy.observe(orgID, "licenses", policy.Inner.RequireLicenses(ctx, tx, orgID, requested))
}

// observe swallows denials but preserves genuine faults, so a broken database
// still surfaces instead of being silently treated as "allowed".
func (policy ObservePolicy) observe(orgID, check string, err error) error {
	if err == nil {
		return nil
	}
	if IsDenial(err) {
		loging.Logger.Infow("subscription check would have denied request",
			"org_id", orgID, "check", check, "reason", err.Error())
		return nil
	}
	return err
}
