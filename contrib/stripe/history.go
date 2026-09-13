package stripe

import (
	"bigbucks/solution/auth/loging"
	"context"
	"fmt"

	stripesdk "github.com/stripe/stripe-go/v82"
)

// subscriptionHistory summarizes a customer's subscriptions at Stripe.
type subscriptionHistory struct {
	// CurrentID and CurrentStatus identify a subscription that still holds the
	// customer, if any. Another checkout would bill them a second time.
	CurrentID     string
	CurrentStatus stripesdk.SubscriptionStatus
	// Subscribed reports whether the customer has ever held a subscription that
	// got past its first payment. It disqualifies them from another trial.
	Subscribed bool
}

// subscriptionHistory reads a customer's subscriptions directly from Stripe.
//
// Checkout decisions deliberately bypass the local projection: a customer who
// completed checkout moments ago may not have had their webhook processed yet,
// and that is exactly when a second checkout is most likely.
func (provider *Provider) subscriptionHistory(ctx context.Context, customerID string) (subscriptionHistory, error) {
	var history subscriptionHistory
	params := &stripesdk.SubscriptionListParams{
		Customer: stripesdk.String(customerID),
		Status:   stripesdk.String("all"),
	}
	params.Limit = stripesdk.Int64(100)

	for subscription, err := range provider.client.V1Subscriptions.List(ctx, params) {
		if err != nil {
			return history, fmt.Errorf("list subscriptions for %s: %w", customerID, err)
		}
		switch subscription.Status {
		case stripesdk.SubscriptionStatusIncomplete, stripesdk.SubscriptionStatusIncompleteExpired:
			// The first payment never completed, so nothing was granted and no
			// trial was consumed. An incomplete subscription expires on its own.
			continue
		case stripesdk.SubscriptionStatusActive, stripesdk.SubscriptionStatusTrialing,
			stripesdk.SubscriptionStatusPastDue, stripesdk.SubscriptionStatusUnpaid,
			stripesdk.SubscriptionStatusPaused:
			if history.CurrentID == "" {
				history.CurrentID = subscription.ID
				history.CurrentStatus = subscription.Status
			}
		}
		history.Subscribed = true
		if history.CurrentID != "" {
			break
		}
	}
	return history, nil
}

// expireOpenCheckoutSessions closes checkout pages the customer left open.
//
// Without this, a buyer who opened checkout in two tabs, or went back and chose a
// different plan, could complete both and be billed for two subscriptions.
// Failures are logged rather than returned: a session that cannot be expired has
// usually just been completed, which the subscription check catches next time.
func (provider *Provider) expireOpenCheckoutSessions(ctx context.Context, customerID string) {
	params := &stripesdk.CheckoutSessionListParams{
		Customer: stripesdk.String(customerID),
		Status:   stripesdk.String(string(stripesdk.CheckoutSessionStatusOpen)),
	}
	params.Limit = stripesdk.Int64(20)

	for session, err := range provider.client.V1CheckoutSessions.List(ctx, params) {
		if err != nil {
			loging.Logger.Warnw("could not list open checkout sessions",
				"customer_id", customerID, "error", err.Error())
			return
		}
		if _, err := provider.client.V1CheckoutSessions.Expire(ctx, session.ID, &stripesdk.CheckoutSessionExpireParams{}); err != nil {
			loging.Logger.Warnw("could not expire superseded checkout session",
				"customer_id", customerID, "session_id", session.ID, "error", err.Error())
		}
	}
}
