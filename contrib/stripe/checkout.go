package stripe

import (
	"bigbucks/solution/auth/subscriptions"
	"context"
	"fmt"
	"strings"

	stripesdk "github.com/stripe/stripe-go/v82"
)

// StartCheckout opens a Stripe Checkout session for an organization.
//
// The organization is the Stripe customer; the owner's email is only the
// address receipts go to. Checkout is preferred over building a payment form
// because Stripe then owns card collection, SCA, tax and invoicing.
func (provider *Provider) StartCheckout(ctx context.Context, request subscriptions.CheckoutRequest) (subscriptions.RedirectSession, error) {
	var session subscriptions.RedirectSession

	if _, ok := provider.catalog[request.PriceID]; !ok {
		return session, fmt.Errorf("price %q is not in the configured catalog", request.PriceID)
	}
	if request.SuccessURL == "" || request.CancelURL == "" {
		return session, fmt.Errorf("success and cancel urls are required")
	}
	quantity := request.Quantity
	if quantity < 1 {
		quantity = 1
	}
	if quantity > provider.config.MaxQuantity {
		return session, fmt.Errorf("quantity %d exceeds the maximum of %d", quantity, provider.config.MaxQuantity)
	}

	// The currency must be one the price actually carries, otherwise Stripe
	// rejects the session. Checking here turns a provider error into a clear
	// 400 and, more importantly, stops a page that quoted USD from silently
	// charging the price's default currency.
	currency := strings.ToLower(strings.TrimSpace(request.Currency))
	if currency != "" {
		details, err := provider.InspectPrices(ctx, []string{request.PriceID})
		if err != nil {
			return session, fmt.Errorf("verify price currency: %w", err)
		}
		detail, known := details[request.PriceID]
		if !known {
			return session, fmt.Errorf("price %q could not be read", request.PriceID)
		}
		if _, offered := detail.Amounts[currency]; !offered {
			return session, fmt.Errorf("price %q is not available in %s (offered: %s)",
				request.PriceID, currency, strings.Join(detail.Currencies(), ", "))
		}
		// Stripe pins a customer to the currency of their first invoice and
		// rejects later subscriptions in any other. Checking here turns that
		// into an explanatory error instead of an opaque provider failure.
		locked, err := provider.LockedCurrency(ctx, request.OrgID)
		if err != nil {
			return session, fmt.Errorf("verify organization currency: %w", err)
		}
		if locked != "" && locked != currency {
			return session, fmt.Errorf(
				"organization already bills in %s and cannot be charged in %s; "+
					"currency is fixed once billing has started", locked, currency)
		}

		if currency == detail.DefaultCurrency {
			// Passing the default explicitly is harmless but unnecessary.
			currency = ""
		}
	}

	customerID, err := provider.ensureCustomer(ctx, request.OrgID, request.OrgName, request.OwnerEmail)
	if err != nil {
		return session, err
	}

	lineItem := &stripesdk.CheckoutSessionCreateLineItemParams{
		Price:    stripesdk.String(request.PriceID),
		Quantity: stripesdk.Int64(quantity),
	}
	if provider.config.AllowQuantityAdjustment {
		// Lets the buyer choose how many licences to purchase inside Checkout,
		// which is how per-seat pricing is bought without a bespoke UI.
		lineItem.AdjustableQuantity = &stripesdk.CheckoutSessionCreateLineItemAdjustableQuantityParams{
			Enabled: stripesdk.Bool(true),
			Minimum: stripesdk.Int64(1),
			Maximum: stripesdk.Int64(provider.config.MaxQuantity),
		}
	}

	params := &stripesdk.CheckoutSessionCreateParams{
		Mode:       stripesdk.String(string(stripesdk.CheckoutSessionModeSubscription)),
		Customer:   stripesdk.String(customerID),
		LineItems:  []*stripesdk.CheckoutSessionCreateLineItemParams{lineItem},
		SuccessURL: stripesdk.String(request.SuccessURL),
		CancelURL:  stripesdk.String(request.CancelURL),
		// Echoed back on checkout.session.completed, and a useful audit trail in
		// the Stripe dashboard.
		ClientReferenceID: stripesdk.String(request.OrgID),
		SubscriptionData: &stripesdk.CheckoutSessionCreateSubscriptionDataParams{
			Metadata: map[string]string{metadataOrgID: request.OrgID},
		},
	}
	if currency != "" {
		params.Currency = stripesdk.String(currency)
	}
	if provider.config.AllowPromotionCodes {
		params.AllowPromotionCodes = stripesdk.Bool(true)
	}
	if provider.config.TrialPeriodDays > 0 {
		params.SubscriptionData.TrialPeriodDays = stripesdk.Int64(provider.config.TrialPeriodDays)
	}

	created, err := provider.client.V1CheckoutSessions.Create(ctx, params)
	if err != nil {
		return session, fmt.Errorf("create checkout session: %w", err)
	}
	return subscriptions.RedirectSession{ID: created.ID, URL: created.URL}, nil
}

// StartPortal opens the Stripe billing portal, where an administrator can change
// licence counts, update payment details, download invoices and cancel.
//
// Using the portal rather than building these flows means seat changes are
// prorated by Stripe automatically, and the resulting subscription update
// arrives here as a normal webhook.
func (provider *Provider) StartPortal(ctx context.Context, request subscriptions.PortalRequest) (subscriptions.RedirectSession, error) {
	var session subscriptions.RedirectSession

	if request.ReturnURL == "" {
		return session, fmt.Errorf("return url is required")
	}

	customerID, err := provider.ensureCustomer(ctx, request.OrgID, request.OrgName, request.OwnerEmail)
	if err != nil {
		return session, err
	}

	params := &stripesdk.BillingPortalSessionCreateParams{
		Customer:  stripesdk.String(customerID),
		ReturnURL: stripesdk.String(request.ReturnURL),
	}

	flowData, err := portalFlowData(request)
	if err != nil {
		return session, err
	}
	params.FlowData = flowData

	created, err := provider.client.V1BillingPortalSessions.Create(ctx, params)
	if err != nil {
		return session, fmt.Errorf("create billing portal session: %w", err)
	}
	return subscriptions.RedirectSession{ID: created.ID, URL: created.URL}, nil
}

// portalFlowData deep-links the portal to a single task.
//
// Cancel and plan changes act on one subscription, so they need its id; without
// it Stripe rejects the session. Returning nil opens the portal home.
func portalFlowData(request subscriptions.PortalRequest) (*stripesdk.BillingPortalSessionCreateFlowDataParams, error) {
	requiresSubscription := func() error {
		if request.SubscriptionID == "" {
			return fmt.Errorf("the %q flow needs an active subscription", request.Flow)
		}
		return nil
	}

	switch request.Flow {
	case subscriptions.PortalFlowHome:
		return nil, nil

	case subscriptions.PortalFlowCancel:
		if err := requiresSubscription(); err != nil {
			return nil, err
		}
		return &stripesdk.BillingPortalSessionCreateFlowDataParams{
			Type: stripesdk.String(string(stripesdk.BillingPortalSessionFlowTypeSubscriptionCancel)),
			SubscriptionCancel: &stripesdk.BillingPortalSessionCreateFlowDataSubscriptionCancelParams{
				Subscription: stripesdk.String(request.SubscriptionID),
			},
		}, nil

	case subscriptions.PortalFlowUpdatePlan:
		// One flow covers both changing tier and changing licence count: Stripe's
		// update screen exposes the quantity selector alongside the plan list.
		if err := requiresSubscription(); err != nil {
			return nil, err
		}
		return &stripesdk.BillingPortalSessionCreateFlowDataParams{
			Type: stripesdk.String(string(stripesdk.BillingPortalSessionFlowTypeSubscriptionUpdate)),
			SubscriptionUpdate: &stripesdk.BillingPortalSessionCreateFlowDataSubscriptionUpdateParams{
				Subscription: stripesdk.String(request.SubscriptionID),
			},
		}, nil

	case subscriptions.PortalFlowPaymentMethod:
		return &stripesdk.BillingPortalSessionCreateFlowDataParams{
			Type: stripesdk.String(string(stripesdk.BillingPortalSessionFlowTypePaymentMethodUpdate)),
		}, nil

	default:
		return nil, fmt.Errorf("unknown portal flow %q", request.Flow)
	}
}
