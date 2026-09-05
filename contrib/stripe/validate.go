package stripe

import (
	"bigbucks/solution/auth/subscriptions"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	stripesdk "github.com/stripe/stripe-go/v82"
)

// ValidateCatalog checks every configured price against Stripe before the
// service accepts traffic.
//
// The mistake this exists for is a price id in config.json that does not match
// Stripe. Nothing rejects it at runtime: checkout succeeds, the webhook lands,
// and the projection finds no catalog entry — so the organization is billed and
// granted zero licences, visible only as a log line. Failing to start is much
// louder.
func (provider *Provider) ValidateCatalog(ctx context.Context, catalog map[string]subscriptions.Product) error {
	priceIDs := make([]string, 0, len(catalog))
	for priceID := range catalog {
		priceIDs = append(priceIDs, priceID)
	}
	sort.Strings(priceIDs)

	var problems []string
	// Tracks (tier, interval) so two prices cannot claim the same slot: the
	// pricing page keys on it, and the loser would silently vanish.
	slots := map[string]string{}
	defaultCurrencies := map[string][]string{}

	for _, priceID := range priceIDs {
		price, err := provider.client.V1Prices.Retrieve(ctx, priceID, &stripesdk.PriceRetrieveParams{})
		if err != nil {
			var stripeErr *stripesdk.Error
			if errors.As(err, &stripeErr) && stripeErr.HTTPStatusCode == 404 {
				problems = append(problems, fmt.Sprintf(
					"%s does not exist in Stripe (wrong id, or created in the other of test/live mode)", priceID))
				continue
			}
			// Anything else — network, auth, rate limit — means we could not
			// check, which must not be reported as a bad catalog.
			return fmt.Errorf("%w: retrieve price %s: %v",
				subscriptions.ErrValidationUnavailable, priceID, err)
		}

		if !price.Active {
			problems = append(problems, fmt.Sprintf(
				"%s is archived in Stripe and cannot be purchased", priceID))
		}
		if price.Recurring == nil {
			problems = append(problems, fmt.Sprintf(
				"%s is a one-time price; the catalog only supports subscriptions", priceID))
			continue
		}

		product := catalog[priceID]
		interval := string(price.Recurring.Interval)
		if product.Tier != "" {
			slot := product.Tier + "/" + interval
			if other, taken := slots[slot]; taken {
				problems = append(problems, fmt.Sprintf(
					"%s and %s are both tier %q billed per %s; one would be hidden on the pricing page",
					other, priceID, product.Tier, interval))
			}
			slots[slot] = priceID
		}

		currency := strings.ToLower(string(price.Currency))
		defaultCurrencies[currency] = append(defaultCurrencies[currency], priceID)
	}

	// Stripe refuses a Checkout Session whose line items disagree on default
	// currency, so a split catalog breaks purchasing rather than display.
	if len(defaultCurrencies) > 1 {
		var described []string
		for currency, prices := range defaultCurrencies {
			sort.Strings(prices)
			described = append(described, fmt.Sprintf("%s (%s)", currency, strings.Join(prices, ", ")))
		}
		sort.Strings(described)
		problems = append(problems, fmt.Sprintf(
			"prices have different default currencies: %s; Stripe requires one default across the catalog",
			strings.Join(described, "; ")))
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("stripe catalog does not match configuration:\n  - %s",
			strings.Join(problems, "\n  - "))
	}
	return nil
}
