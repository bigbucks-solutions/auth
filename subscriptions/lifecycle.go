package subscriptions

import (
	"bigbucks/solution/auth/loging"
	"sort"
	"time"
)

// State is the provider-neutral lifecycle state of an organization's
// subscription.
//
// Services should branch on State rather than Entitlements.Status, which carries
// the provider's own vocabulary and would couple them to Stripe.
type State string

const (
	// StateNotManaged means billing is disabled in this deployment, so nothing
	// is limited.
	StateNotManaged State = "not_managed"
	// StateNone means the organization has never held a subscription, or its
	// first payment never completed.
	StateNone State = "none"
	// StateTrialing means a trial is running. Access is granted.
	StateTrialing State = "trialing"
	// StateActive means the subscription is paid and current.
	StateActive State = "active"
	// StatePastDue means a renewal payment failed and the provider is still
	// retrying. Access is granted while the grace period lasts.
	StatePastDue State = "past_due"
	// StateCanceled means the subscription was cancelled and access has ended.
	StateCanceled State = "canceled"
	// StateExpired means access lapsed for any other reason: a failed payment was
	// never recovered, the subscription was paused, or its period ended without
	// a renewal being observed.
	StateExpired State = "expired"
)

// LimitKind distinguishes allowances that reset from standing caps.
type LimitKind string

const (
	// LimitKindCap is a point-in-time limit on stored records. It never resets;
	// deleting a record frees capacity.
	LimitKindCap LimitKind = "cap"
	// LimitKindMonthlyQuota is an allowance that resets every month, anchored to
	// the subscription's billing cycle.
	LimitKindMonthlyQuota LimitKind = "monthly_quota"
)

// Unlimited is the limit value meaning "no limit".
const Unlimited int64 = -1

// LimitGrant is what the active plans allow for one limit key.
type LimitGrant struct {
	Kind LimitKind `json:"kind"`
	// Limit is the allowance, or Unlimited.
	Limit int64 `json:"limit"`
	// PeriodStart and PeriodEnd bound the window a monthly quota is counted
	// over, as [start, end). They are unset for caps.
	PeriodStart *time.Time `json:"period_start,omitempty"`
	PeriodEnd   *time.Time `json:"period_end,omitempty"`
}

// IsUnlimited reports whether the grant places no limit.
func (grant LimitGrant) IsUnlimited() bool { return grant.Limit < 0 }

// summarize folds subscription lines into entitlements. It is pure so the
// lifecycle rules can be tested without a database; licence consumption is
// counted separately by Resolve.
//
// active holds the lines currently granting access. latest is the most recently
// changed line of any status and is consulted only when nothing is active, to
// tell an organization whose access ended apart from one that never paid.
func summarize(config Config, orgID string, active []SubscriptionItem, latest *SubscriptionItem, now time.Time) Entitlements {
	entitlements := Entitlements{
		OrgID:             orgID,
		State:             StateNone,
		Status:            "none",
		Features:          []string{},
		Limits:            map[string]LimitGrant{},
		Plans:             []PlanSummary{},
		ManagedExternally: true,
		ResolvedAt:        now,
	}

	features := make(map[string]struct{})
	var anchor *time.Time
	for _, item := range active {
		product, configured := config.Catalog[item.PriceID]
		if !configured {
			// An unmapped price still proves the organization is paying, but it
			// cannot grant licences, features or limits until the catalog
			// describes it.
			loging.Logger.Warnw("subscription price is not in the catalog",
				"org_id", orgID, "price_id", item.PriceID)
		}
		licenses := product.Licenses(item.Quantity)
		entitlements.Entitled = true
		entitlements.Licenses += licenses
		for _, feature := range product.Features {
			features[feature] = struct{}{}
		}
		for key, value := range product.Caps {
			mergeLimit(entitlements.Limits, key, LimitKindCap, value)
		}
		for key, value := range product.MonthlyQuotas {
			mergeLimit(entitlements.Limits, key, LimitKindMonthlyQuota, value)
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
			entitlements.CurrentPeriodStart = item.CurrentPeriodStart
		}
		if item.Status == "trialing" && item.TrialEnd != nil &&
			(entitlements.TrialEndsAt == nil || item.TrialEnd.Before(*entitlements.TrialEndsAt)) {
			entitlements.TrialEndsAt = item.TrialEnd
		}
		if anchor == nil {
			anchor = firstSet(item.BillingCycleAnchor, item.CurrentPeriodStart)
		}
		entitlements.Plans = append(entitlements.Plans, PlanSummary{
			PriceID:           item.PriceID,
			Name:              product.Name,
			Tier:              product.Tier,
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

	switch {
	case entitlements.Entitled:
		entitlements.State = activeState(active)
		start, end := QuotaWindow(anchor, now)
		for key, grant := range entitlements.Limits {
			if grant.Kind != LimitKindMonthlyQuota {
				continue
			}
			grant.PeriodStart, grant.PeriodEnd = &start, &end
			entitlements.Limits[key] = grant
		}
	case latest != nil:
		entitlements.Status = latest.Status
		entitlements.State = lapsedState(latest.Status)
		if entitlements.State != StateNone {
			ended := latest.UpdatedAt
			entitlements.EndedAt = firstSet(latest.EndedAt, latest.CurrentPeriodEnd, &ended)
		}
	}
	return entitlements
}

// activeState picks the healthiest state across granting lines, so one
// struggling add-on does not mark a paid-up organization as past due.
func activeState(active []SubscriptionItem) State {
	var trialing, pastDue bool
	for _, item := range active {
		switch item.Status {
		case "active":
			return StateActive
		case "trialing":
			trialing = true
		case "past_due":
			pastDue = true
		}
	}
	switch {
	case trialing:
		return StateTrialing
	case pastDue:
		return StatePastDue
	default:
		// A granting line with a status this package does not know is still
		// granting; report it as active rather than inventing a failure.
		return StateActive
	}
}

// lapsedState classifies an organization with nothing currently granting from
// the status of its most recently changed line.
func lapsedState(status string) State {
	switch status {
	case "canceled":
		return StateCanceled
	case "", "none", "incomplete", "incomplete_expired":
		// A checkout whose first payment never completed never granted access.
		return StateNone
	default:
		// unpaid, paused, removed, or a line still marked active whose period
		// ended beyond the grace window without a renewal arriving.
		return StateExpired
	}
}

// mergeLimit adds one product's allowance to the running total. Allowances from
// several lines accumulate, and unlimited on any line wins.
func mergeLimit(limits map[string]LimitGrant, key string, kind LimitKind, value int64) {
	if value < 0 {
		value = Unlimited
	}
	grant, exists := limits[key]
	if !exists {
		limits[key] = LimitGrant{Kind: kind, Limit: value}
		return
	}
	if grant.IsUnlimited() || value == Unlimited {
		grant.Limit = Unlimited
	} else {
		grant.Limit += value
	}
	limits[key] = grant
}

// QuotaWindow returns the monthly window containing now, as [start, end).
//
// Windows repeat monthly from the billing-cycle anchor, so a subscription that
// renews on the 14th resets its quotas on the 14th, yearly plans included. When
// a month lacks the anchor's day the window starts on that month's last day, and
// later months return to the anchor's day, which matches how providers bill.
// With no anchor the calendar month in UTC is used.
func QuotaWindow(anchor *time.Time, now time.Time) (start, end time.Time) {
	now = now.UTC()
	if anchor == nil || anchor.IsZero() {
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 1, 0)
	}

	origin := anchor.UTC()
	months := (now.Year()-origin.Year())*12 + int(now.Month()) - int(origin.Month())
	start = addMonthsClamped(origin, months)
	for start.After(now) {
		months--
		start = addMonthsClamped(origin, months)
	}
	end = addMonthsClamped(origin, months+1)
	for !now.Before(end) {
		months++
		start = end
		end = addMonthsClamped(origin, months+1)
	}
	return start, end
}

// addMonthsClamped offsets origin by whole months, clamping the day to the
// target month's length. It always counts from origin, so a Jan 31 anchor yields
// Feb 28 and then Mar 31 rather than drifting to the 28th.
func addMonthsClamped(origin time.Time, months int) time.Time {
	total := int(origin.Month()) - 1 + months
	yearOffset := floorDiv(total, 12)
	month := time.Month(total - yearOffset*12 + 1)
	year := origin.Year() + yearOffset
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	return time.Date(year, month, min(origin.Day(), lastDay),
		origin.Hour(), origin.Minute(), origin.Second(), origin.Nanosecond(), time.UTC)
}

func floorDiv(dividend, divisor int) int {
	quotient := dividend / divisor
	if dividend%divisor != 0 && (dividend < 0) != (divisor < 0) {
		quotient--
	}
	return quotient
}

func firstSet(candidates ...*time.Time) *time.Time {
	for _, candidate := range candidates {
		if candidate != nil && !candidate.IsZero() {
			return candidate
		}
	}
	return nil
}
