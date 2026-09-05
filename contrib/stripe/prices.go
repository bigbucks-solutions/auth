package stripe

import (
	"bigbucks/solution/auth/subscriptions"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	stripesdk "github.com/stripe/stripe-go/v82"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

// priceCacheTTL bounds how stale a pricing page can be.
//
// Prices are immutable in Stripe — changing what you charge means creating a new
// price and updating the catalog — so the only thing this expires is a catalog
// edit that points at a different price id.
const priceCacheTTL = 10 * time.Minute

type cachedPrice struct {
	detail  subscriptions.PriceDetail
	fetched time.Time
}

var (
	priceCacheMu    sync.RWMutex
	priceCache      = map[string]cachedPrice{}
	priceFetchGroup singleflight.Group
)

// InspectPrices reports what each price actually costs, reading the amounts from
// Stripe rather than from local configuration so a pricing page can never
// advertise a figure checkout will not charge.
func (provider *Provider) InspectPrices(ctx context.Context, priceIDs []string) (map[string]subscriptions.PriceDetail, error) {
	details := make(map[string]subscriptions.PriceDetail, len(priceIDs))
	var missing []string

	priceCacheMu.RLock()
	for _, priceID := range priceIDs {
		if entry, ok := priceCache[priceID]; ok && time.Since(entry.fetched) < priceCacheTTL {
			details[priceID] = entry.detail
			continue
		}
		missing = append(missing, priceID)
	}
	priceCacheMu.RUnlock()

	for _, priceID := range missing {
		value, err, _ := priceFetchGroup.Do(priceID, func() (any, error) {
			// Another request may have populated the cache after the initial read.
			priceCacheMu.RLock()
			entry, ok := priceCache[priceID]
			priceCacheMu.RUnlock()
			if ok && time.Since(entry.fetched) < priceCacheTTL {
				return entry.detail, nil
			}

			// Neither tiers nor currency_options are returned unless expanded.
			expansions := provider.priceExpansions
			if len(expansions) == 0 {
				expansions = configuredPriceExpansions(subscriptions.CurrencyConfig{})
			}
			price, err := provider.client.V1Prices.Retrieve(ctx, priceID, &stripesdk.PriceRetrieveParams{
				Params: stripesdk.Params{Expand: expansions},
			})
			if err != nil {
				return nil, fmt.Errorf("retrieve price %s: %w", priceID, err)
			}

			detail := priceDetail(price)
			priceCacheMu.Lock()
			priceCache[priceID] = cachedPrice{detail: detail, fetched: time.Now()}
			priceCacheMu.Unlock()
			return detail, nil
		})
		if err != nil {
			return nil, err
		}
		details[priceID] = value.(subscriptions.PriceDetail)
	}
	return details, nil
}

func configuredPriceExpansions(config subscriptions.CurrencyConfig) []*string {
	expansions := []*string{
		stripesdk.String("tiers"),
		stripesdk.String("currency_options"),
	}
	seen := map[string]struct{}{}
	addCurrency := func(currency string) {
		currency = strings.ToLower(strings.TrimSpace(currency))
		if len(currency) != 3 {
			return
		}
		if _, exists := seen[currency]; exists {
			return
		}
		for _, character := range currency {
			if character < 'a' || character > 'z' {
				return
			}
		}
		seen[currency] = struct{}{}
		expansions = append(expansions, stripesdk.String("currency_options."+currency+".tiers"))
	}

	addCurrency(config.Default)
	for _, currency := range config.ByCountry {
		addCurrency(currency)
	}
	return expansions
}

// LockedCurrency reports the currency an organization is already committed to.
//
// Stripe sets Customer.currency from the first invoice and refuses later
// subscriptions in any other currency, so once an organization has billed, that
// is the only currency it can be offered.
//
// An organization with no billing account has never billed and is still free to
// choose, which is also the common case on a pricing page — so no API call is
// made for it.
func (provider *Provider) LockedCurrency(ctx context.Context, orgID string) (string, error) {
	if orgID == "" {
		return "", nil
	}

	var account subscriptions.BillingAccount
	err := provider.db.WithContext(ctx).
		Where("org_id = ? AND provider = ?", orgID, ProviderName).
		First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	customer, err := provider.client.V1Customers.Retrieve(ctx, account.ProviderCustomerID,
		&stripesdk.CustomerRetrieveParams{})
	if err != nil {
		return "", fmt.Errorf("retrieve customer %s: %w", account.ProviderCustomerID, err)
	}
	// A customer that exists but has never been invoiced has no currency yet.
	return strings.ToLower(string(customer.Currency)), nil
}

// priceDetail flattens a Stripe price into a per-currency amount table.
//
// The price's own currency is the default; every entry in currency_options adds
// another the customer may be charged in.
func priceDetail(price *stripesdk.Price) subscriptions.PriceDetail {
	detail := subscriptions.PriceDetail{
		PriceID:         price.ID,
		DefaultCurrency: strings.ToLower(string(price.Currency)),
		Amounts:         map[string]subscriptions.Amount{},
	}
	if price.Recurring != nil {
		detail.Interval = string(price.Recurring.Interval)
	}

	detail.Amounts[detail.DefaultCurrency] = amountFrom(
		detail.DefaultCurrency, price.UnitAmount, tiersOf(price.Tiers))

	for currency, options := range price.CurrencyOptions {
		if options == nil {
			continue
		}
		code := strings.ToLower(currency)
		if code == detail.DefaultCurrency {
			continue
		}
		amount := amountFrom(code, options.UnitAmount, currencyOptionTiers(options.Tiers))
		// A currency with neither tiers nor a unit amount is not actually
		// sellable; listing it would show a 0 on the pricing page.
		if amount.FirstUnitAmount == 0 && amount.AdditionalUnitAmount == 0 {
			continue
		}
		detail.Amounts[code] = amount
	}
	return detail
}

// tier is the shape shared by a price's tiers and its per-currency tiers.
type tier struct {
	unitAmount int64
	flatAmount int64
}

func tiersOf(tiers []*stripesdk.PriceTier) []tier {
	converted := make([]tier, 0, len(tiers))
	for _, entry := range tiers {
		if entry == nil {
			continue
		}
		converted = append(converted, tier{unitAmount: entry.UnitAmount, flatAmount: entry.FlatAmount})
	}
	return converted
}

func currencyOptionTiers(tiers []*stripesdk.PriceCurrencyOptionsTier) []tier {
	converted := make([]tier, 0, len(tiers))
	for _, entry := range tiers {
		if entry == nil {
			continue
		}
		converted = append(converted, tier{unitAmount: entry.UnitAmount, flatAmount: entry.FlatAmount})
	}
	return converted
}

// amountFrom reduces a tier ladder to the first-unit / additional-unit pair a
// pricing page renders.
func amountFrom(currency string, unitAmount int64, tiers []tier) subscriptions.Amount {
	if len(tiers) == 0 {
		// Flat per-unit price: every licence costs the same.
		return subscriptions.Amount{
			Currency:             currency,
			FirstUnitAmount:      unitAmount,
			AdditionalUnitAmount: unitAmount,
		}
	}

	amount := subscriptions.Amount{Currency: currency, Tiered: true}

	// A first tier may price its units (unit_amount) or charge a flat fee for
	// the whole tier (flat_amount). Both are used to mean "the base plan".
	first := tiers[0]
	if first.unitAmount > 0 {
		amount.FirstUnitAmount = first.unitAmount
	} else {
		amount.FirstUnitAmount = first.flatAmount
	}

	// The last tier is the open-ended one that prices every additional licence.
	amount.AdditionalUnitAmount = tiers[len(tiers)-1].unitAmount
	if len(tiers) == 1 {
		amount.AdditionalUnitAmount = amount.FirstUnitAmount
	}
	return amount
}
