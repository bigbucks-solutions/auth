package subscriptions

import (
	"sort"
	"strings"
)

// Amount is what one price costs in a single currency.
//
// Values are in the currency's smallest unit: fils for AED, cents for USD.
type Amount struct {
	Currency string `json:"currency"`
	// FirstUnitAmount is what the first licence costs. For a graduated price
	// this is tier one; for a flat price it is the unit amount.
	FirstUnitAmount int64 `json:"first_unit_amount"`
	// AdditionalUnitAmount is what each licence beyond the first costs. Equal to
	// FirstUnitAmount on a flat per-unit price.
	AdditionalUnitAmount int64 `json:"additional_unit_amount"`
	// Tiered reports whether the price uses graduated tiers, so a pricing page
	// knows whether to show "from X" plus a per-seat line.
	Tiered bool `json:"tiered"`
}

// PriceDetail is the sellable shape of one catalog price, in every currency the
// provider will accept for it.
type PriceDetail struct {
	PriceID string `json:"price_id"`
	// Interval is "month" or "year".
	Interval string `json:"interval"`
	// DefaultCurrency is what the provider charges when none is requested.
	DefaultCurrency string `json:"default_currency"`
	// Amounts is keyed by lower-case ISO-4217 code.
	Amounts map[string]Amount `json:"amounts"`
}

// AmountIn returns the price in a currency, falling back to the default.
func (detail PriceDetail) AmountIn(currency string) (Amount, bool) {
	if amount, ok := detail.Amounts[strings.ToLower(strings.TrimSpace(currency))]; ok {
		return amount, true
	}
	amount, ok := detail.Amounts[detail.DefaultCurrency]
	return amount, ok
}

// Currencies lists the currencies this price can be bought in, sorted.
func (detail PriceDetail) Currencies() []string {
	currencies := make([]string, 0, len(detail.Amounts))
	for currency := range detail.Amounts {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)
	return currencies
}

// CurrencyConfig decides which currency an organization is quoted in.
type CurrencyConfig struct {
	// Default is used when the organization's country is unknown or unmapped.
	Default string `json:"default" mapstructure:"default"`
	// ByCountry maps an ISO-3166 alpha-2 country code to an ISO-4217 currency.
	// Keys are matched case-insensitively.
	ByCountry map[string]string `json:"byCountry" mapstructure:"byCountry"`
}

// MapsCountries reports whether any country-to-currency mapping is configured.
//
// Callers use this to skip looking an organization's country up at all when
// nothing would be done with it.
func (config CurrencyConfig) MapsCountries() bool {
	return len(config.ByCountry) > 0
}

// For resolves the currency to quote a country in. An empty or unmapped country
// falls back to Default.
//
// Matching is case-insensitive on ISO-3166 alpha-2 codes. Anything else — an
// alpha-3 code, or a country name like "United Arab Emirates" — will not match;
// see LooksLikeCountryCode.
func (config CurrencyConfig) For(country string) string {
	country = strings.ToUpper(strings.TrimSpace(country))
	if country != "" {
		for candidate, currency := range config.ByCountry {
			if strings.ToUpper(strings.TrimSpace(candidate)) == country {
				return strings.ToLower(strings.TrimSpace(currency))
			}
		}
	}
	return strings.ToLower(strings.TrimSpace(config.Default))
}

// LooksLikeCountryCode reports whether a value is shaped like an ISO-3166
// alpha-2 code.
//
// Organization country is free text, so a value such as "UAE" would silently
// fail to map and quietly fall back to the default currency. This lets callers
// tell "unmapped country" apart from "country stored in the wrong format",
// which are very different problems.
func LooksLikeCountryCode(country string) bool {
	country = strings.TrimSpace(country)
	if len(country) != 2 {
		return false
	}
	for _, character := range country {
		if character < 'A' || character > 'z' ||
			(character > 'Z' && character < 'a') {
			return false
		}
	}
	return true
}
