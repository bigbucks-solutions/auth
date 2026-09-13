package subscriptions

import (
	"bigbucks/solution/auth/constants"
	"strings"
	"testing"
	"time"
)

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("time.Parse(%q) error = %v", value, err)
	}
	return parsed
}

func timeAt(t *testing.T, value string) *time.Time {
	t.Helper()
	parsed := mustTime(t, value)
	return &parsed
}

func TestQuotaWindow(t *testing.T) {
	tests := []struct {
		name      string
		anchor    string
		now       string
		wantStart string
		wantEnd   string
	}{
		{"calendar month without an anchor", "", "2026-09-13T12:00:00Z", "2026-09-01T00:00:00Z", "2026-10-01T00:00:00Z"},
		{"resets on the anchor day", "2026-01-14T09:30:00Z", "2026-09-13T12:00:00Z", "2026-08-14T09:30:00Z", "2026-09-14T09:30:00Z"},
		{"the reset instant opens the next window", "2026-01-14T09:30:00Z", "2026-09-14T09:30:00Z", "2026-09-14T09:30:00Z", "2026-10-14T09:30:00Z"},
		{"a short month clamps to its last day", "2026-01-31T00:00:00Z", "2026-02-15T00:00:00Z", "2026-01-31T00:00:00Z", "2026-02-28T00:00:00Z"},
		{"the anchor day returns after a short month", "2026-01-31T00:00:00Z", "2026-03-05T00:00:00Z", "2026-02-28T00:00:00Z", "2026-03-31T00:00:00Z"},
		{"leap-year february", "2028-01-30T00:00:00Z", "2028-02-20T00:00:00Z", "2028-01-30T00:00:00Z", "2028-02-29T00:00:00Z"},
		{"a yearly plan still resets monthly", "2025-06-10T00:00:00Z", "2026-09-13T00:00:00Z", "2026-09-10T00:00:00Z", "2026-10-10T00:00:00Z"},
		{"a year boundary", "2026-03-31T00:00:00Z", "2027-01-02T00:00:00Z", "2026-12-31T00:00:00Z", "2027-01-31T00:00:00Z"},
		{"an anchor still in the future during a trial", "2026-09-20T00:00:00Z", "2026-09-13T00:00:00Z", "2026-08-20T00:00:00Z", "2026-09-20T00:00:00Z"},
		{"a future anchor across a year boundary", "2027-01-10T00:00:00Z", "2026-12-13T00:00:00Z", "2026-12-10T00:00:00Z", "2027-01-10T00:00:00Z"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var anchor *time.Time
			if test.anchor != "" {
				anchor = timeAt(t, test.anchor)
			}
			start, end := QuotaWindow(anchor, mustTime(t, test.now))
			if want := mustTime(t, test.wantStart); !start.Equal(want) {
				t.Errorf("start = %s, want %s", start, want)
			}
			if want := mustTime(t, test.wantEnd); !end.Equal(want) {
				t.Errorf("end = %s, want %s", end, want)
			}
		})
	}
}

func lifecycleConfig() Config {
	return Config{
		Catalog: map[string]Product{
			"price_starter": {
				Name: "Starter", Tier: "starter", LicensesPerUnit: 1,
				Features:      []string{"cloud_access"},
				MonthlyQuotas: map[string]int64{"invoices": 5000},
				Caps:          map[string]int64{"skus": 5000},
			},
			"price_addon": {
				Name:          "Extra capacity",
				MonthlyQuotas: map[string]int64{"invoices": 1000},
				Caps:          map[string]int64{"skus": -1},
			},
		},
	}
}

func TestSummarizeNeverSubscribed(t *testing.T) {
	now := mustTime(t, "2026-09-13T00:00:00Z")
	entitlements := summarize(lifecycleConfig(), "org-1", nil, nil, now)

	if entitlements.Entitled || entitlements.State != StateNone || entitlements.Status != "none" {
		t.Fatalf("got entitled=%v state=%q status=%q, want not entitled, none, none",
			entitlements.Entitled, entitlements.State, entitlements.Status)
	}
	if entitlements.EndedAt != nil {
		t.Fatalf("EndedAt = %v, want nil for an organization that never subscribed", entitlements.EndedAt)
	}
	if len(entitlements.Limits) != 0 || !entitlements.ManagedExternally || !entitlements.ResolvedAt.Equal(now) {
		t.Fatalf("unexpected snapshot: %#v", entitlements)
	}
}

func TestSummarizeTrialAnchorsQuotasToTheBillingCycle(t *testing.T) {
	now := mustTime(t, "2026-09-13T00:00:00Z")
	trialEnd := timeAt(t, "2026-09-20T00:00:00Z")
	items := []SubscriptionItem{{
		PriceID: "price_starter", Status: "trialing", Active: true, Quantity: 3,
		TrialEnd: trialEnd, BillingCycleAnchor: trialEnd,
		CurrentPeriodStart: timeAt(t, "2026-09-13T00:00:00Z"), CurrentPeriodEnd: trialEnd,
	}}

	entitlements := summarize(lifecycleConfig(), "org-1", items, nil, now)

	if !entitlements.Entitled || entitlements.State != StateTrialing {
		t.Fatalf("got entitled=%v state=%q, want entitled trialing", entitlements.Entitled, entitlements.State)
	}
	if entitlements.TrialEndsAt == nil || !entitlements.TrialEndsAt.Equal(*trialEnd) {
		t.Fatalf("TrialEndsAt = %v, want %s", entitlements.TrialEndsAt, trialEnd)
	}
	if entitlements.Licenses != 3 {
		t.Fatalf("Licenses = %d, want 3", entitlements.Licenses)
	}

	invoices := entitlements.Limits["invoices"]
	if invoices.Kind != LimitKindMonthlyQuota || invoices.Limit != 5000 {
		t.Fatalf("invoices = %#v, want a 5000 monthly quota", invoices)
	}
	if invoices.PeriodStart == nil || !invoices.PeriodStart.Equal(mustTime(t, "2026-08-20T00:00:00Z")) ||
		invoices.PeriodEnd == nil || !invoices.PeriodEnd.Equal(*trialEnd) {
		t.Fatalf("invoices window = [%v, %v), want [2026-08-20, 2026-09-20)", invoices.PeriodStart, invoices.PeriodEnd)
	}

	skus := entitlements.Limits["skus"]
	if skus.Kind != LimitKindCap || skus.Limit != 5000 || skus.PeriodStart != nil {
		t.Fatalf("skus = %#v, want a 5000 cap with no period", skus)
	}
	if entitlements.Plans[0].Tier != "starter" {
		t.Fatalf("plan tier = %q, want starter", entitlements.Plans[0].Tier)
	}
}

func TestSummarizeAccumulatesLimitsAcrossLines(t *testing.T) {
	now := mustTime(t, "2026-09-13T00:00:00Z")
	items := []SubscriptionItem{
		{PriceID: "price_starter", Status: "active", Active: true, Quantity: 1},
		{PriceID: "price_addon", Status: "active", Active: true, Quantity: 1},
	}

	entitlements := summarize(lifecycleConfig(), "org-1", items, nil, now)

	if got := entitlements.Limits["invoices"].Limit; got != 6000 {
		t.Fatalf("invoices limit = %d, want quotas from both lines summed to 6000", got)
	}
	if !entitlements.Limits["skus"].IsUnlimited() {
		t.Fatalf("skus = %#v, want unlimited because one line grants unlimited", entitlements.Limits["skus"])
	}
}

func TestSummarizePicksTheHealthiestActiveState(t *testing.T) {
	now := mustTime(t, "2026-09-13T00:00:00Z")
	tests := []struct {
		statuses []string
		want     State
	}{
		{[]string{"past_due", "active"}, StateActive},
		{[]string{"past_due", "trialing"}, StateTrialing},
		{[]string{"past_due"}, StatePastDue},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.statuses, "+"), func(t *testing.T) {
			var items []SubscriptionItem
			for _, status := range test.statuses {
				items = append(items, SubscriptionItem{PriceID: "price_starter", Status: status, Active: true, Quantity: 1})
			}
			if got := summarize(lifecycleConfig(), "org-1", items, nil, now).State; got != test.want {
				t.Fatalf("State = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSummarizeLapsedStates(t *testing.T) {
	now := mustTime(t, "2026-09-13T00:00:00Z")
	updated := mustTime(t, "2026-09-01T00:00:00Z")
	tests := []struct {
		name        string
		latest      SubscriptionItem
		wantState   State
		wantEndedAt *time.Time
	}{
		{
			name: "cancelled uses the provider's end time",
			latest: SubscriptionItem{Status: "canceled",
				EndedAt: timeAt(t, "2026-09-05T00:00:00Z"), CurrentPeriodEnd: timeAt(t, "2026-09-10T00:00:00Z")},
			wantState:   StateCanceled,
			wantEndedAt: timeAt(t, "2026-09-05T00:00:00Z"),
		},
		{
			name:      "a first payment that never completed was never a subscription",
			latest:    SubscriptionItem{Status: "incomplete_expired", CurrentPeriodEnd: timeAt(t, "2026-09-10T00:00:00Z")},
			wantState: StateNone,
		},
		{
			name:        "an active line past the grace window has expired at its period end",
			latest:      SubscriptionItem{Status: "active", Active: true, CurrentPeriodEnd: timeAt(t, "2026-09-08T00:00:00Z")},
			wantState:   StateExpired,
			wantEndedAt: timeAt(t, "2026-09-08T00:00:00Z"),
		},
		{
			name:        "unpaid falls back to when the line last changed",
			latest:      SubscriptionItem{BaseModel: constants.BaseModel{UpdatedAt: updated}, Status: "unpaid"},
			wantState:   StateExpired,
			wantEndedAt: &updated,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			latest := test.latest
			entitlements := summarize(lifecycleConfig(), "org-1", nil, &latest, now)

			if entitlements.Entitled {
				t.Fatal("a lapsed organization must not be entitled")
			}
			if entitlements.State != test.wantState {
				t.Fatalf("State = %q, want %q", entitlements.State, test.wantState)
			}
			if entitlements.Status != latest.Status {
				t.Fatalf("Status = %q, want the raw provider status %q", entitlements.Status, latest.Status)
			}
			switch {
			case test.wantEndedAt == nil && entitlements.EndedAt != nil:
				t.Fatalf("EndedAt = %v, want nil", entitlements.EndedAt)
			case test.wantEndedAt != nil && (entitlements.EndedAt == nil || !entitlements.EndedAt.Equal(*test.wantEndedAt)):
				t.Fatalf("EndedAt = %v, want %v", entitlements.EndedAt, test.wantEndedAt)
			}
		})
	}
}

func TestPeriodEndGrace(t *testing.T) {
	zero, six, negative := 0, 6, -3
	tests := []struct {
		name  string
		hours *int
		want  time.Duration
	}{
		{"defaults to 48 hours", nil, 48 * time.Hour},
		{"zero disables the grace", &zero, 0},
		{"explicit hours apply", &six, 6 * time.Hour},
		{"negative is floored at zero", &negative, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := (Config{PeriodEndGraceHours: test.hours}).PeriodEndGrace(); got != test.want {
				t.Fatalf("PeriodEndGrace() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestConfigValidateRejectsLifecycleMistakes(t *testing.T) {
	negative := -1
	tests := []struct {
		name    string
		config  Config
		wantErr string
	}{
		{
			name: "a key used as both a cap and a quota",
			config: Config{
				Enabled: true,
				Limits:  map[string]LimitDefinition{"invoices": {Label: "Invoices"}},
				Catalog: map[string]Product{
					"price_a": {LicensesPerUnit: 1, MonthlyQuotas: map[string]int64{"invoices": 10}},
					"price_b": {LicensesPerUnit: 1, Caps: map[string]int64{"invoices": 10}},
				},
			},
			wantErr: `limit "invoices" is a cap on one price and a monthly quota on another`,
		},
		{
			name:    "a negative grace period",
			config:  Config{Enabled: true, PeriodEndGraceHours: &negative},
			wantErr: "periodEndGraceHours cannot be negative",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.Validate()
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("Validate() = %v, want it to mention %q", err, test.wantErr)
			}
		})
	}
}
