package actions

import (
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/subscriptions"
	"context"
	"sort"
	"strings"
)

// PricingPage is everything a public pricing or upgrade screen needs.
//
// It is shaped as a comparison table: Features and Limits list every key that
// exists across the whole catalog so the frontend can render one row each, and
// every plan reports which of those it includes. That way a tier that lacks a
// feature still gets a row with a cross rather than being silently absent.
type PricingPage struct {
	// Currency is the ISO-4217 code the page should display by default,
	// resolved from the organization's country unless explicitly requested.
	Currency string `json:"currency"`
	// Currencies is every code the organization may actually buy in, sorted. A
	// currency offered by only some plans is excluded, since a switcher that
	// blanks one column is worse than not offering it.
	Currencies []string `json:"currencies"`
	// CurrencyLocked is true once the organization has billed and the provider
	// has pinned it to a single currency. Currencies then holds only that one
	// and the frontend should hide the switcher rather than offer a choice that
	// would fail at checkout.
	CurrencyLocked bool `json:"currency_locked"`
	// Features is every capability in the catalog, ordered for display.
	Features []PricingFeature `json:"features"`
	// Limits is every quota and cap in the catalog, ordered for display.
	Limits []PricingLimit `json:"limits"`
	// Plans is one entry per tier, cheapest first.
	Plans []PricingPlan `json:"plans"`
	// Available is false when billing is disabled for this deployment, in which
	// case the pricing page should not be rendered at all.
	Available bool `json:"available"`
}

// PricingFeature is one comparison row.
type PricingFeature struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// PricingLimit is one comparison row for a numeric allowance.
type PricingLimit struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Unit  string `json:"unit,omitempty"`
	// Period is "month" for allowances that reset, "total" for standing caps.
	Period string `json:"period"`
}

// PricingPlan is one column of the comparison table.
type PricingPlan struct {
	Tier        string `json:"tier"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Highlight   bool   `json:"highlight"`
	// Features lists the capability keys this tier includes. Keys absent from
	// this list but present in PricingPage.Features are not included.
	Features []string `json:"features"`
	// Limits maps a limit key to its value for this tier. -1 means unlimited.
	// A key missing here is not offered at all on this tier.
	Limits map[string]int64 `json:"limits"`
	// Pricing is keyed by billing interval, "month" and/or "year".
	Pricing map[string]subscriptions.PriceDetail `json:"pricing"`
}

// BillingPlans assembles the pricing page.
//
// Display copy comes from configuration; the amounts come from the billing
// provider, so they cannot drift from what checkout charges. If the provider
// cannot be reached the plans are still returned with their features and limits
// and no amounts, which degrades a pricing page rather than breaking it.
func BillingPlans(ctx context.Context, request PricingRequest) PricingPage {
	page := PricingPage{
		Features:   []PricingFeature{},
		Limits:     []PricingLimit{},
		Plans:      []PricingPlan{},
		Currencies: []string{},
	}

	module := subscriptions.CurrentModule()
	config := module.Config
	if config.EffectiveMode() == subscriptions.ModeDisabled || len(config.Catalog) == 0 {
		return page
	}
	page.Available = true

	page.Features = pricingFeatures(config)
	page.Limits = pricingLimits(config)
	page.Plans = pricingPlans(ctx, module, config)
	page.Currencies = sharedCurrencies(page.Plans)

	// Only read the organization's country when a mapping exists to use it. A
	// single-currency deployment should not pay for a query it will discard.
	if request.Country == "" && request.OrgID != "" && config.Currencies.MapsCountries() {
		request.Country = organizationCountryForPricing(ctx, request.OrgID)
	}

	// An organization that has already billed cannot change currency, so it is
	// offered only the one it is pinned to.
	if locked, ok := lockedCurrency(ctx, module, request.OrgID, page.Currencies); ok {
		page.Currencies = []string{locked}
		page.CurrencyLocked = true
		page.Currency = locked
		return page
	}

	page.Currency = resolveCurrency(config, request, page.Currencies, page.Plans)
	return page
}

// organizationCountryForPricing reads an organization's country and warns when
// it is stored in a shape that cannot map to a currency.
//
// The column is free text populated from a form field, so "UAE" or "United Arab
// Emirates" are as likely as "AE". Those would otherwise fall back to the
// default currency with no indication that anything was wrong.
func organizationCountryForPricing(ctx context.Context, orgID string) string {
	country := OrganizationCountry(ctx, orgID)
	if country != "" && !subscriptions.LooksLikeCountryCode(country) {
		loging.Logger.Warnw("organization country is not an ISO-3166 alpha-2 code, "+
			"so it cannot select a currency; falling back to the default",
			"org_id", orgID, "country", country)
		return ""
	}
	return country
}

// lockedCurrency reports the currency an organization is already committed to.
//
// It returns false when the organization is still free to choose, when the
// provider does not pin currencies, or when the pinned currency is not one the
// catalog can sell — the last case being a misconfiguration that should not
// leave the page with no purchasable currency at all.
func lockedCurrency(ctx context.Context, module *subscriptions.Module, orgID string, available []string) (string, bool) {
	if orgID == "" || len(available) == 0 {
		return "", false
	}
	locker, ok := module.CurrencyLock()
	if !ok {
		return "", false
	}

	locked, err := locker.LockedCurrency(ctx, orgID)
	if err != nil {
		// Failing to read the lock must not break the pricing page. The worst
		// case is that a switcher is shown and checkout rejects the change,
		// which is the behaviour that existed before this check.
		loging.Logger.Warnw("could not read the organization's locked billing currency",
			"org_id", orgID, "error", err.Error())
		return "", false
	}
	locked = strings.ToLower(strings.TrimSpace(locked))
	if locked == "" {
		return "", false
	}
	for _, candidate := range available {
		if candidate == locked {
			return locked, true
		}
	}
	loging.Logger.Warnw("organization is pinned to a currency the catalog cannot sell",
		"org_id", orgID, "locked_currency", locked, "available", available)
	return "", false
}

// PricingRequest carries what the caller knows about who is being quoted.
type PricingRequest struct {
	// Currency is an explicit override, normally from a currency switcher.
	// Ignored if the catalog does not offer it.
	Currency string
	// Country is the organization's ISO-3166 alpha-2 code, used to pick a
	// default when no override is given.
	Country string
	// OrgID identifies the organization being quoted. When set, an organization
	// that has already billed is restricted to the currency it is pinned to.
	OrgID string
}

// sharedCurrencies returns the currencies available on every plan and interval.
//
// Intersecting rather than unioning means the switcher can never put the page
// into a state where one column has no price.
func sharedCurrencies(plans []PricingPlan) []string {
	var shared map[string]struct{}

	for _, plan := range plans {
		for _, detail := range plan.Pricing {
			available := map[string]struct{}{}
			for currency := range detail.Amounts {
				available[currency] = struct{}{}
			}
			if shared == nil {
				shared = available
				continue
			}
			for currency := range shared {
				if _, ok := available[currency]; !ok {
					delete(shared, currency)
				}
			}
		}
	}

	currencies := make([]string, 0, len(shared))
	for currency := range shared {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)
	return currencies
}

// resolveCurrency picks what to quote: an explicit override wins, then the
// organization's country, then the configured default, then whatever the first
// plan actually offers.
func resolveCurrency(config subscriptions.Config, request PricingRequest, available []string, plans []PricingPlan) string {
	offered := func(currency string) bool {
		currency = strings.ToLower(strings.TrimSpace(currency))
		if currency == "" {
			return false
		}
		for _, candidate := range available {
			if candidate == currency {
				return true
			}
		}
		return false
	}

	if requested := strings.ToLower(strings.TrimSpace(request.Currency)); offered(requested) {
		return requested
	}
	if derived := config.Currencies.For(request.Country); offered(derived) {
		return derived
	}
	if fallback := strings.ToLower(strings.TrimSpace(config.Currencies.Default)); offered(fallback) {
		return fallback
	}
	// Nothing configured matched. Quote the first plan's default rather than
	// returning an empty string the frontend cannot index by.
	for _, plan := range plans {
		for _, interval := range sortedIntervals(plan.Pricing) {
			if currency := plan.Pricing[interval].DefaultCurrency; currency != "" {
				return currency
			}
		}
	}
	return ""
}

// sortedIntervals gives map iteration a stable order, so the fallback currency
// is deterministic rather than whichever key Go happened to visit first.
func sortedIntervals(pricing map[string]subscriptions.PriceDetail) []string {
	intervals := make([]string, 0, len(pricing))
	for interval := range pricing {
		intervals = append(intervals, interval)
	}
	sort.Strings(intervals)
	return intervals
}

// pricingFeatures lists every capability the catalog references, so a tier that
// lacks one still renders a row.
func pricingFeatures(config subscriptions.Config) []PricingFeature {
	type ordered struct {
		feature PricingFeature
		order   int
	}
	seen := map[string]ordered{}

	record := func(key string) {
		if _, exists := seen[key]; exists {
			return
		}
		definition, described := config.Features[key]
		label := definition.Label
		if !described || label == "" {
			// An undescribed key still belongs on the page; falling back to the
			// key is uglier than a label but better than hiding what was sold.
			label = key
		}
		seen[key] = ordered{
			feature: PricingFeature{Key: key, Label: label, Description: definition.Description},
			order:   definition.Order,
		}
	}

	for key := range config.Features {
		record(key)
	}
	for _, product := range config.Catalog {
		for _, key := range product.Features {
			record(key)
		}
	}

	features := make([]PricingFeature, 0, len(seen))
	for _, entry := range seen {
		features = append(features, entry.feature)
	}
	sort.Slice(features, func(i, j int) bool {
		left, right := seen[features[i].Key], seen[features[j].Key]
		if left.order != right.order {
			return left.order < right.order
		}
		return features[i].Key < features[j].Key
	})
	return features
}

// pricingLimits lists every quota and cap key, tagging each with whether it
// resets monthly or is a standing cap.
func pricingLimits(config subscriptions.Config) []PricingLimit {
	periods := map[string]string{}
	for _, product := range config.Catalog {
		for key := range product.MonthlyQuotas {
			periods[key] = "month"
		}
		for key := range product.Caps {
			periods[key] = "total"
		}
	}

	limits := make([]PricingLimit, 0, len(periods))
	for key, period := range periods {
		definition := config.Limits[key]
		label := definition.Label
		if label == "" {
			label = key
		}
		limits = append(limits, PricingLimit{
			Key: key, Label: label, Unit: definition.Unit, Period: period,
		})
	}
	sort.Slice(limits, func(i, j int) bool {
		left, right := config.Limits[limits[i].Key], config.Limits[limits[j].Key]
		if left.Order != right.Order {
			return left.Order < right.Order
		}
		return limits[i].Key < limits[j].Key
	})
	return limits
}

// pricingPlans folds the catalog's price entries into one entry per tier, with
// each billing interval hanging off it.
func pricingPlans(ctx context.Context, module *subscriptions.Module, config subscriptions.Config) []PricingPlan {
	details := inspectPrices(ctx, module, config)

	byTier := map[string]*PricingPlan{}
	order := map[string]int{}

	for priceID, product := range config.Catalog {
		tier := product.Tier
		if tier == "" {
			// A catalog entry with no tier is its own column, keyed by price so
			// two untiered entries cannot collide.
			tier = priceID
		}

		plan, exists := byTier[tier]
		if !exists {
			plan = &PricingPlan{
				Tier:        tier,
				Name:        product.Name,
				Description: product.Description,
				Highlight:   product.Highlight,
				Features:    []string{},
				Limits:      map[string]int64{},
				Pricing:     map[string]subscriptions.PriceDetail{},
			}
			byTier[tier] = plan
			order[tier] = product.Order
		}

		// Features and limits are properties of the tier, so the monthly and
		// annual entries agree; merging is only defensive against a typo in one
		// of the two.
		for _, feature := range product.Features {
			if !containsString(plan.Features, feature) {
				plan.Features = append(plan.Features, feature)
			}
		}
		for key, value := range product.MonthlyQuotas {
			plan.Limits[key] = value
		}
		for key, value := range product.Caps {
			plan.Limits[key] = value
		}

		detail, known := details[priceID]
		if !known {
			// No amount available; still expose the price id so checkout works.
			detail = subscriptions.PriceDetail{
				PriceID: priceID,
				Amounts: map[string]subscriptions.Amount{},
			}
		}
		interval := detail.Interval
		if interval == "" {
			interval = "unknown"
		}
		plan.Pricing[interval] = detail
	}

	plans := make([]PricingPlan, 0, len(byTier))
	for _, plan := range byTier {
		sort.Strings(plan.Features)
		plans = append(plans, *plan)
	}
	sort.Slice(plans, func(i, j int) bool {
		if order[plans[i].Tier] != order[plans[j].Tier] {
			return order[plans[i].Tier] < order[plans[j].Tier]
		}
		return plans[i].Tier < plans[j].Tier
	})
	return plans
}

// inspectPrices fetches live amounts, degrading to none on failure.
func inspectPrices(ctx context.Context, module *subscriptions.Module, config subscriptions.Config) map[string]subscriptions.PriceDetail {
	inspector, ok := module.Prices()
	if !ok {
		return map[string]subscriptions.PriceDetail{}
	}

	priceIDs := make([]string, 0, len(config.Catalog))
	for priceID := range config.Catalog {
		priceIDs = append(priceIDs, priceID)
	}

	details, err := inspector.InspectPrices(ctx, priceIDs)
	if err != nil {
		// A pricing page without amounts is degraded; one that fails to load is
		// broken. Prefer the former and make the cause visible in logs.
		loging.Logger.Errorw("could not read prices from the billing provider",
			"error", err.Error())
		return map[string]subscriptions.PriceDetail{}
	}
	return details
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
