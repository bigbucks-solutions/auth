package subscriptions

import (
	"bigbucks/solution/auth/loging"
	"context"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Entitlements is the resolved billing state of one organization. It is the
// payload the frontend renders its billing screens from.
type Entitlements struct {
	OrgID string `json:"org_id"`
	// Entitled reports whether the organization may use the application.
	Entitled bool `json:"entitled"`
	// Status is the most relevant provider status across active lines
	// ("active", "trialing", "past_due", ...), or "none" when nothing is held.
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
	// Plans describes each active line for display.
	Plans []PlanSummary `json:"plans"`
	// CurrentPeriodEnd is the earliest renewal date across active lines.
	CurrentPeriodEnd *time.Time `json:"current_period_end"`
	// CancelAtPeriodEnd is true when any active line is set to lapse.
	CancelAtPeriodEnd bool `json:"cancel_at_period_end"`
	// ManagedExternally is false when subscriptions are disabled, telling the
	// frontend to hide billing UI entirely.
	ManagedExternally bool `json:"managed_externally"`
}

// PlanSummary describes one purchased line.
type PlanSummary struct {
	PriceID           string     `json:"price_id"`
	Name              string     `json:"name"`
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
	entitlements := Entitlements{
		OrgID:             orgID,
		Status:            "none",
		Features:          []string{},
		Plans:             []PlanSummary{},
		ManagedExternally: true,
	}

	items, err := activeItems(ctx, tx, orgID)
	if err != nil {
		return entitlements, err
	}

	features := make(map[string]struct{})
	for _, item := range items {
		product, configured := config.Catalog[item.PriceID]
		if !configured {
			// An unmapped price still proves the organization is paying, but it
			// cannot grant licences or features until the catalog describes it.
			loging.Logger.Warnw("subscription price is not in the catalog",
				"org_id", orgID, "price_id", item.PriceID)
		}
		licenses := product.Licenses(item.Quantity)
		entitlements.Entitled = true
		entitlements.Licenses += licenses
		for _, feature := range product.Features {
			features[feature] = struct{}{}
		}
		if item.CancelAtPeriodEnd {
			entitlements.CancelAtPeriodEnd = true
		}
		if entitlements.Status == "none" || item.Status == "active" {
			entitlements.Status = item.Status
		}
		if item.CurrentPeriodEnd != nil &&
			(entitlements.CurrentPeriodEnd == nil || item.CurrentPeriodEnd.Before(*entitlements.CurrentPeriodEnd)) {
			entitlements.CurrentPeriodEnd = item.CurrentPeriodEnd
		}
		entitlements.Plans = append(entitlements.Plans, PlanSummary{
			PriceID:           item.PriceID,
			Name:              product.Name,
			Quantity:          item.Quantity,
			Licenses:          licenses,
			Status:            item.Status,
			CancelAtPeriodEnd: item.CancelAtPeriodEnd,
			CurrentPeriodEnd:  item.CurrentPeriodEnd,
		})
	}

	for feature := range features {
		entitlements.Features = append(entitlements.Features, feature)
	}
	sort.Strings(entitlements.Features)

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
func activeItems(ctx context.Context, tx *gorm.DB, orgID string) ([]SubscriptionItem, error) {
	var items []SubscriptionItem
	err := tx.WithContext(ctx).
		Model(&SubscriptionItem{}).
		Joins("JOIN billing_accounts ON billing_accounts.id = subscription_items.account_id"+
			" AND billing_accounts.deleted_at IS NULL").
		Where("billing_accounts.org_id = ?", orgID).
		Where("subscription_items.active = ?", true).
		Where("subscription_items.current_period_end IS NULL OR subscription_items.current_period_end > ?", time.Now().UTC()).
		Order("subscription_items.created_at").
		Find(&items).Error
	return items, err
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
		Status:            "not_managed",
		Features:          []string{},
		Plans:             []PlanSummary{},
		ManagedExternally: false,
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
