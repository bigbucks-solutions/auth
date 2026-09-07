package actions

import (
	"context"
	"errors"
	"testing"

	"bigbucks/solution/auth/subscriptions"
)

// fakeProvider stands in for a billing adapter, with a switchable price source.
type fakeProvider struct {
	details map[string]subscriptions.PriceDetail
	err     error
	trial   int64
}

func (provider *fakeProvider) Name() string                    { return "fake" }
func (provider *fakeProvider) Webhooks() []subscriptions.Route { return nil }
func (provider *fakeProvider) TrialPeriodDays() int64          { return provider.trial }
func (provider *fakeProvider) InspectPrices(context.Context, []string) (map[string]subscriptions.PriceDetail, error) {
	if provider.err != nil {
		return nil, provider.err
	}
	return provider.details, nil
}

func twoTierConfig() subscriptions.Config {
	return subscriptions.Config{
		Enabled:  true,
		Mode:     subscriptions.ModeEnforce,
		Provider: "fake",
		Features: map[string]subscriptions.FeatureDefinition{
			"cloud_access":       {Label: "Cloud access", Order: 1},
			"advanced_reporting": {Label: "Advanced reporting", Order: 2},
			// Declared but sold on no tier: it must still render a row so the
			// comparison table can show a cross against every plan.
			"api_access": {Label: "API access", Order: 3},
		},
		Limits: map[string]subscriptions.LimitDefinition{
			"invoices": {Label: "Invoices", Unit: "per month", Order: 1},
			"skus":     {Label: "Products", Unit: "SKUs", Order: 2},
		},
		Catalog: map[string]subscriptions.Product{
			"price_starter_m": {
				Name: "Starter", Tier: "starter", Order: 1, LicensesPerUnit: 1,
				Features:      []string{"cloud_access"},
				MonthlyQuotas: map[string]int64{"invoices": 5000},
				Caps:          map[string]int64{"skus": 5000},
			},
			"price_starter_y": {
				Name: "Starter", Tier: "starter", Order: 1, LicensesPerUnit: 1,
				Features:      []string{"cloud_access"},
				MonthlyQuotas: map[string]int64{"invoices": 5000},
				Caps:          map[string]int64{"skus": 5000},
			},
			"price_growth_m": {
				Name: "Growth", Tier: "growth", Order: 2, Highlight: true, LicensesPerUnit: 1,
				Features:      []string{"cloud_access", "advanced_reporting"},
				MonthlyQuotas: map[string]int64{"invoices": 10000},
				Caps:          map[string]int64{"skus": 10000},
			},
		},
	}
}

func installModule(t *testing.T, config subscriptions.Config, provider subscriptions.Provider) {
	t.Helper()
	subscriptions.SetModule(&subscriptions.Module{
		Config:   config,
		Provider: provider,
		Policy:   subscriptions.DatabasePolicy{Config: config},
	})
	t.Cleanup(func() { subscriptions.SetModule(nil) })
}

// The two prices of a tier must collapse into one column with both intervals
// hanging off it, or the pricing page grows a column per billing period.
func TestBillingPlansGroupsIntervalsIntoOneTier(t *testing.T) {
	installModule(t, twoTierConfig(), &fakeProvider{details: multiCurrencyDetails(), trial: 7})

	page := BillingPlans(context.Background(), PricingRequest{})

	if !page.Available {
		t.Fatal("page should be available when billing is enabled")
	}
	if len(page.Plans) != 2 {
		t.Fatalf("got %d plans, want 2 (one per tier)", len(page.Plans))
	}
	if page.Currency != "aed" {
		t.Fatalf("Currency = %q, want aed", page.Currency)
	}
	if page.TrialPeriodDays != 7 {
		t.Fatalf("TrialPeriodDays = %d, want 7", page.TrialPeriodDays)
	}

	starter := page.Plans[0]
	if starter.Tier != "starter" {
		t.Fatalf("first plan = %q, want starter (Order should sort it first)", starter.Tier)
	}
	if len(starter.Pricing) != 2 {
		t.Fatalf("starter has %d intervals, want month and year", len(starter.Pricing))
	}
	if got := starter.Pricing["month"].Amounts["aed"].FirstUnitAmount; got != 5000 {
		t.Fatalf("starter monthly first user = %d, want 5000", got)
	}
	if got := starter.Pricing["year"].Amounts["aed"].AdditionalUnitAmount; got != 19900 {
		t.Fatalf("starter annual extra user = %d, want 19900", got)
	}
	if got := starter.Limits["invoices"]; got != 5000 {
		t.Fatalf("starter invoices = %d, want 5000", got)
	}

	growth := page.Plans[1]
	if !growth.Highlight {
		t.Fatal("growth should be highlighted")
	}
	if len(growth.Pricing) != 1 {
		t.Fatalf("growth has %d intervals, want 1", len(growth.Pricing))
	}
}

// Every feature in the catalog gets a row, including one no tier sells, so the
// table can render a cross rather than omitting the row entirely.
func TestBillingPlansListsEveryFeatureAsAComparisonRow(t *testing.T) {
	installModule(t, twoTierConfig(), &fakeProvider{})

	page := BillingPlans(context.Background(), PricingRequest{})

	if len(page.Features) != 3 {
		t.Fatalf("got %d feature rows, want 3", len(page.Features))
	}
	wantOrder := []string{"cloud_access", "advanced_reporting", "api_access"}
	for i, key := range wantOrder {
		if page.Features[i].Key != key {
			t.Fatalf("feature[%d] = %q, want %q (Order should drive this)", i, page.Features[i].Key, key)
		}
	}
	if page.Features[0].Label != "Cloud access" {
		t.Fatalf("label = %q, want the configured display label", page.Features[0].Label)
	}

	// api_access is sold on no tier.
	for _, plan := range page.Plans {
		if containsString(plan.Features, "api_access") {
			t.Fatalf("plan %q should not include api_access", plan.Tier)
		}
	}
}

// Limits must be tagged with whether they reset, since the two behave very
// differently and the page has to say "per month" or not.
func TestBillingPlansTagsLimitPeriods(t *testing.T) {
	installModule(t, twoTierConfig(), &fakeProvider{})

	page := BillingPlans(context.Background(), PricingRequest{})

	periods := map[string]string{}
	for _, limit := range page.Limits {
		periods[limit.Key] = limit.Period
	}
	if periods["invoices"] != "month" {
		t.Fatalf("invoices period = %q, want month", periods["invoices"])
	}
	if periods["skus"] != "total" {
		t.Fatalf("skus period = %q, want total", periods["skus"])
	}
}

// A provider outage should degrade the page, not break it: features and limits
// still render, and the price ids survive so checkout keeps working.
func TestBillingPlansSurvivesProviderFailure(t *testing.T) {
	installModule(t, twoTierConfig(), &fakeProvider{err: errors.New("stripe unreachable")})

	page := BillingPlans(context.Background(), PricingRequest{})

	if !page.Available || len(page.Plans) != 2 {
		t.Fatalf("plans should still render without amounts, got %d", len(page.Plans))
	}
	if len(page.Features) != 3 {
		t.Fatal("features should still render without amounts")
	}
	for _, plan := range page.Plans {
		for _, detail := range plan.Pricing {
			if detail.PriceID == "" {
				t.Fatal("price id must survive so checkout still works")
			}
			if len(detail.Amounts) != 0 {
				t.Fatal("amounts should be absent, not invented")
			}
		}
	}
}

// With billing off the page must be empty and flagged, so the frontend hides it
// rather than rendering an empty pricing table.
func TestBillingPlansEmptyWhenDisabled(t *testing.T) {
	subscriptions.SetModule(nil)
	t.Cleanup(func() { subscriptions.SetModule(nil) })

	page := BillingPlans(context.Background(), PricingRequest{})

	if page.Available {
		t.Fatal("Available should be false when billing is disabled")
	}
	if len(page.Plans) != 0 || len(page.Features) != 0 {
		t.Fatal("disabled billing should expose no plans or features")
	}
}

// A catalog key with no matching definition still has to appear, or a plan would
// advertise nothing after someone forgets to add display copy.
func TestBillingPlansFallsBackToKeyWhenUndescribed(t *testing.T) {
	config := subscriptions.Config{
		Enabled: true, Mode: subscriptions.ModeEnforce, Provider: "fake",
		Catalog: map[string]subscriptions.Product{
			"price_x": {Name: "Solo", Tier: "solo", Features: []string{"undocumented_feature"}},
		},
	}
	installModule(t, config, &fakeProvider{})

	page := BillingPlans(context.Background(), PricingRequest{})

	if len(page.Features) != 1 {
		t.Fatalf("got %d features, want 1", len(page.Features))
	}
	if page.Features[0].Label != "undocumented_feature" {
		t.Fatalf("Label = %q, want the key as a fallback", page.Features[0].Label)
	}
}

// multiCurrencyDetails mirrors what Stripe returns for prices that carry
// currency_options: AED everywhere, USD on all but the annual Starter.
func multiCurrencyDetails() map[string]subscriptions.PriceDetail {
	return map[string]subscriptions.PriceDetail{
		"price_starter_m": {
			PriceID: "price_starter_m", Interval: "month", DefaultCurrency: "aed",
			Amounts: map[string]subscriptions.Amount{
				"aed": {Currency: "aed", FirstUnitAmount: 5000, AdditionalUnitAmount: 2000, Tiered: true},
				"usd": {Currency: "usd", FirstUnitAmount: 1400, AdditionalUnitAmount: 550, Tiered: true},
			},
		},
		"price_starter_y": {
			PriceID: "price_starter_y", Interval: "year", DefaultCurrency: "aed",
			Amounts: map[string]subscriptions.Amount{
				"aed": {Currency: "aed", FirstUnitAmount: 49900, AdditionalUnitAmount: 19900, Tiered: true},
				"usd": {Currency: "usd", FirstUnitAmount: 13900, AdditionalUnitAmount: 5500, Tiered: true},
			},
		},
		"price_growth_m": {
			PriceID: "price_growth_m", Interval: "month", DefaultCurrency: "aed",
			Amounts: map[string]subscriptions.Amount{
				"aed": {Currency: "aed", FirstUnitAmount: 9000, AdditionalUnitAmount: 3500, Tiered: true},
				"usd": {Currency: "usd", FirstUnitAmount: 2500, AdditionalUnitAmount: 999, Tiered: true},
			},
		},
	}
}

func currencyConfig() subscriptions.Config {
	config := twoTierConfig()
	config.Currencies = subscriptions.CurrencyConfig{
		Default:   "aed",
		ByCountry: map[string]string{"AE": "aed", "US": "usd", "GB": "gbp"},
	}
	return config
}

// The organization's country picks the currency, and an explicit override wins.
func TestBillingPlansResolvesCurrency(t *testing.T) {
	tests := []struct {
		name    string
		request PricingRequest
		want    string
	}{
		{"country drives the default", PricingRequest{Country: "US"}, "usd"},
		{"country is matched case-insensitively", PricingRequest{Country: "us"}, "usd"},
		{"home country", PricingRequest{Country: "AE"}, "aed"},
		{"explicit override beats country", PricingRequest{Country: "AE", Currency: "usd"}, "usd"},
		{"no country falls back to the configured default", PricingRequest{}, "aed"},
		// GBP is mapped for the country but no price carries it.
		{"unsupported mapped currency falls back", PricingRequest{Country: "GB"}, "aed"},
		{"unmapped country falls back", PricingRequest{Country: "JP"}, "aed"},
		{"an override the catalog cannot sell is ignored", PricingRequest{Currency: "jpy"}, "aed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			installModule(t, currencyConfig(), &fakeProvider{details: multiCurrencyDetails()})

			page := BillingPlans(context.Background(), test.request)

			if page.Currency != test.want {
				t.Fatalf("Currency = %q, want %q", page.Currency, test.want)
			}
		})
	}
}

// Only currencies present on every plan may be offered, or switching would
// leave a column with no price.
func TestBillingPlansOffersOnlyCurrenciesSharedByAllPlans(t *testing.T) {
	details := multiCurrencyDetails()
	// Growth loses USD, so USD must disappear from the switcher entirely.
	delete(details["price_growth_m"].Amounts, "usd")

	installModule(t, currencyConfig(), &fakeProvider{details: details})

	page := BillingPlans(context.Background(), PricingRequest{})

	if len(page.Currencies) != 1 || page.Currencies[0] != "aed" {
		t.Fatalf("Currencies = %v, want [aed] only", page.Currencies)
	}
	// A country mapped to USD must not select a currency the page cannot show.
	page = BillingPlans(context.Background(), PricingRequest{Country: "US"})
	if page.Currency != "aed" {
		t.Fatalf("Currency = %q, want aed when usd is not shared", page.Currency)
	}
}

func TestBillingPlansListsSharedCurrencies(t *testing.T) {
	installModule(t, currencyConfig(), &fakeProvider{details: multiCurrencyDetails()})

	page := BillingPlans(context.Background(), PricingRequest{})

	if len(page.Currencies) != 2 || page.Currencies[0] != "aed" || page.Currencies[1] != "usd" {
		t.Fatalf("Currencies = %v, want sorted [aed usd]", page.Currencies)
	}
}

// lockingProvider pins an organization to a currency, as Stripe does once a
// customer has been invoiced.
type lockingProvider struct {
	fakeProvider
	locked map[string]string
	err    error
}

func (provider *lockingProvider) LockedCurrency(_ context.Context, orgID string) (string, error) {
	if provider.err != nil {
		return "", provider.err
	}
	return provider.locked[orgID], nil
}

// Once an organization has billed, offering it any other currency would produce
// a checkout that Stripe rejects. The switcher must collapse to the one it is
// pinned to.
func TestBillingPlansLocksCurrencyForBilledOrganizations(t *testing.T) {
	provider := &lockingProvider{
		fakeProvider: fakeProvider{details: multiCurrencyDetails()},
		locked:       map[string]string{"org-billed": "usd"},
	}
	installModule(t, currencyConfig(), provider)

	// An organization already billing in USD is offered only USD, even though
	// its country maps to AED and AED is the configured default.
	page := BillingPlans(context.Background(), PricingRequest{OrgID: "org-billed", Country: "AE"})

	if !page.CurrencyLocked {
		t.Fatal("CurrencyLocked should be true for an organization that has billed")
	}
	if page.Currency != "usd" {
		t.Fatalf("Currency = %q, want usd (the locked currency wins over country)", page.Currency)
	}
	if len(page.Currencies) != 1 || page.Currencies[0] != "usd" {
		t.Fatalf("Currencies = %v, want [usd] only", page.Currencies)
	}
}

// An explicit override must not be able to escape the lock, or checkout fails.
func TestBillingPlansLockBeatsExplicitOverride(t *testing.T) {
	provider := &lockingProvider{
		fakeProvider: fakeProvider{details: multiCurrencyDetails()},
		locked:       map[string]string{"org-billed": "aed"},
	}
	installModule(t, currencyConfig(), provider)

	page := BillingPlans(context.Background(), PricingRequest{OrgID: "org-billed", Currency: "usd"})

	if page.Currency != "aed" {
		t.Fatalf("Currency = %q, want aed; an override must not escape the lock", page.Currency)
	}
	if !page.CurrencyLocked {
		t.Fatal("CurrencyLocked should be true")
	}
}

// An organization that has never billed is still free to choose.
func TestBillingPlansDoesNotLockUnbilledOrganizations(t *testing.T) {
	provider := &lockingProvider{
		fakeProvider: fakeProvider{details: multiCurrencyDetails()},
		locked:       map[string]string{},
	}
	installModule(t, currencyConfig(), provider)

	page := BillingPlans(context.Background(), PricingRequest{OrgID: "org-new", Country: "US"})

	if page.CurrencyLocked {
		t.Fatal("CurrencyLocked should be false before the first invoice")
	}
	if page.Currency != "usd" {
		t.Fatalf("Currency = %q, want usd from the country", page.Currency)
	}
	if len(page.Currencies) != 2 {
		t.Fatalf("Currencies = %v, want both offered", page.Currencies)
	}
}

// Failing to read the lock must degrade to the previous behaviour rather than
// breaking the pricing page.
func TestBillingPlansSurvivesLockLookupFailure(t *testing.T) {
	provider := &lockingProvider{
		fakeProvider: fakeProvider{details: multiCurrencyDetails()},
		err:          errors.New("stripe unreachable"),
	}
	installModule(t, currencyConfig(), provider)

	page := BillingPlans(context.Background(), PricingRequest{OrgID: "org-billed", Country: "US"})

	if page.CurrencyLocked {
		t.Fatal("an unreadable lock must not be reported as locked")
	}
	if page.Currency != "usd" || len(page.Currencies) != 2 {
		t.Fatalf("page should fall back to normal resolution, got %q / %v", page.Currency, page.Currencies)
	}
}

// A currency the catalog can no longer sell must not empty the page.
func TestBillingPlansIgnoresUnsellableLockedCurrency(t *testing.T) {
	provider := &lockingProvider{
		fakeProvider: fakeProvider{details: multiCurrencyDetails()},
		locked:       map[string]string{"org-legacy": "gbp"},
	}
	installModule(t, currencyConfig(), provider)

	page := BillingPlans(context.Background(), PricingRequest{OrgID: "org-legacy"})

	if page.CurrencyLocked {
		t.Fatal("a lock the catalog cannot honour must not be applied")
	}
	if len(page.Currencies) != 2 {
		t.Fatalf("Currencies = %v, want the normal set", page.Currencies)
	}
}

// The deployment maps AE to AED and IN to INR, with everything else quoted in
// USD. Matches the currencies block in config.json.
func productionCurrencyConfig() subscriptions.Config {
	config := twoTierConfig()
	config.Currencies = subscriptions.CurrencyConfig{
		Default:   "usd",
		ByCountry: map[string]string{"AE": "aed", "IN": "inr"},
	}
	return config
}

func threeCurrencyDetails() map[string]subscriptions.PriceDetail {
	amounts := func() map[string]subscriptions.Amount {
		return map[string]subscriptions.Amount{
			"usd": {Currency: "usd", FirstUnitAmount: 1400, AdditionalUnitAmount: 550, Tiered: true},
			"aed": {Currency: "aed", FirstUnitAmount: 5000, AdditionalUnitAmount: 2000, Tiered: true},
			"inr": {Currency: "inr", FirstUnitAmount: 120000, AdditionalUnitAmount: 48000, Tiered: true},
		}
	}
	return map[string]subscriptions.PriceDetail{
		"price_starter_m": {PriceID: "price_starter_m", Interval: "month", DefaultCurrency: "usd", Amounts: amounts()},
		"price_starter_y": {PriceID: "price_starter_y", Interval: "year", DefaultCurrency: "usd", Amounts: amounts()},
		"price_growth_m":  {PriceID: "price_growth_m", Interval: "month", DefaultCurrency: "usd", Amounts: amounts()},
	}
}

func TestBillingPlansAppliesConfiguredCountryMapping(t *testing.T) {
	tests := []struct {
		name    string
		country string
		want    string
	}{
		{"UAE gets dirhams", "AE", "aed"},
		{"India gets rupees", "IN", "inr"},
		{"United States falls to the default", "US", "usd"},
		{"United Kingdom falls to the default", "GB", "usd"},
		{"an unknown country falls to the default", "ZZ", "usd"},
		{"no country falls to the default", "", "usd"},
		{"lower-case country still maps", "ae", "aed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			installModule(t, productionCurrencyConfig(), &fakeProvider{details: threeCurrencyDetails()})

			page := BillingPlans(context.Background(), PricingRequest{Country: test.country})

			if page.Currency != test.want {
				t.Fatalf("country %q → currency %q, want %q", test.country, page.Currency, test.want)
			}
		})
	}
}

// The country only picks a default; an explicit switcher choice still wins.
func TestBillingPlansCountryMappingYieldsToOverride(t *testing.T) {
	installModule(t, productionCurrencyConfig(), &fakeProvider{details: threeCurrencyDetails()})

	page := BillingPlans(context.Background(), PricingRequest{Country: "AE", Currency: "usd"})

	if page.Currency != "usd" {
		t.Fatalf("Currency = %q, want usd from the explicit override", page.Currency)
	}
}
