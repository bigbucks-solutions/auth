package subscriptions

import (
	"fmt"
	"sort"
	"strings"
	"time"
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
	// PeriodEndGraceHours keeps a line granting for this long after its period
	// ends, so a delayed renewal webhook does not lock a paying organization out
	// at every renewal. Reconciliation normally corrects the state well within
	// it. Defaults to 48.
	PeriodEndGraceHours *int `json:"periodEndGraceHours" mapstructure:"periodEndGraceHours"`
	// Options carries provider-specific settings.
	Options map[string]string `json:"options" mapstructure:"options"`
}

// DefaultPeriodEndGrace applies when PeriodEndGraceHours is unset.
const DefaultPeriodEndGrace = 48 * time.Hour

// PeriodEndGrace resolves PeriodEndGraceHours.
func (config Config) PeriodEndGrace() time.Duration {
	if config.PeriodEndGraceHours == nil {
		return DefaultPeriodEndGrace
	}
	return time.Duration(max(*config.PeriodEndGraceHours, 0)) * time.Hour
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

	// A key must mean one thing: a service counts a cap as stored records and a
	// quota as records created this period, and cannot do both for one key.
	capKeys, quotaKeys := map[string]struct{}{}, map[string]struct{}{}
	for _, product := range config.Catalog {
		for key := range product.Caps {
			capKeys[key] = struct{}{}
		}
		for key := range product.MonthlyQuotas {
			quotaKeys[key] = struct{}{}
		}
	}
	for key := range capKeys {
		if _, alsoQuota := quotaKeys[key]; alsoQuota {
			problems = append(problems, fmt.Sprintf(
				"limit %q is a cap on one price and a monthly quota on another; use one kind per key", key))
		}
	}
	if config.PeriodEndGraceHours != nil && *config.PeriodEndGraceHours < 0 {
		problems = append(problems, "periodEndGraceHours cannot be negative")
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("invalid subscription catalog:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}
