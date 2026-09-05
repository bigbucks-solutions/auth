package controllers

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/request_context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// checkoutRequestBody is the payload for starting a purchase.
type checkoutRequestBody struct {
	// PriceID identifies the plan, as listed by the catalog endpoint.
	PriceID string `json:"price_id"`
	// Quantity is the number of licences to buy. Defaults to 1.
	Quantity int64 `json:"quantity"`
	// Currency is the ISO-4217 code to charge in. Must be one the price offers,
	// as listed by GET /billing/plans. Empty uses the price's default.
	Currency string `json:"currency"`
	// SuccessURL and CancelURL are paths on the configured UI origin. Absolute
	// URLs pointing elsewhere are rejected and replaced with defaults.
	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`
}

// portalRequestBody is the payload for opening the management portal.
type portalRequestBody struct {
	ReturnURL string `json:"return_url"`
	// Flow deep-links to one task: "cancel", "update_plan" or "payment_method".
	// Empty opens the portal home, which also lists past invoices.
	Flow string `json:"flow"`
}

// GetBillingCatalog godoc
//
//	@Summary		List purchasable plans
//	@Description	Returns the configured plan catalog keyed by provider price id. An empty object means billing is disabled for this deployment.
//	@Tags			billing
//	@Produce		json
//	@Param			X-Auth	header	string	true	"Authorization"
//	@Security		JWTAuth
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	error
//	@Router			/billing/catalog [get]
func GetBillingCatalog(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(actions.BillingCatalog()); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// GetBillingPlans godoc
//
//	@Summary		Get the pricing page
//	@Description	Returns everything a pricing or upgrade screen needs: every feature and limit in the catalog as comparison rows, one plan per tier with the features and limits it includes, and live amounts read from the billing provider. `available: false` means billing is disabled and the page should not be rendered.
//	@Tags			billing
//	@Produce		json
//	@Param			X-Auth	header	string	true	"Authorization"
//	@Param			country	query	string	false	"Organization ISO-3166 alpha-2 country code"
//	@Param			currency	query	string	false	"Explicit ISO-4217 currency override"
//	@Security		JWTAuth
//	@Success		200	{object}	actions.PricingPage
//	@Failure		401	{object}	error
//	@Router			/billing/plans [get]
func GetBillingPlans(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	// A country supplied by the frontend is authoritative. BillingPlans falls
	// back to the stored organization country only when it is omitted.
	request := billingPricingRequest(r, ctx)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(actions.BillingPlans(ctx.Context, request)); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

func billingPricingRequest(r *http.Request, ctx *request_context.Context) actions.PricingRequest {
	return actions.PricingRequest{
		Currency: r.URL.Query().Get("currency"),
		Country:  r.URL.Query().Get("country"),
		OrgID:    ctx.CurrentOrgID,
	}
}

// GetBillingSubscription godoc
//
//	@Summary		Get the current organization's subscription
//	@Description	Returns entitlement state: whether the organization may use the app, how many licences the plan allows, how many are consumed, and which features are unlocked. When billing is disabled, managed_externally is false and the frontend should hide billing UI.
//	@Tags			billing
//	@Produce		json
//	@Param			X-Auth				header	string	true	"Authorization"
//	@Param			X-Organization-Id	header	string	true	"Organization ID"
//	@Security		JWTAuth
//	@Success		200	{object}	subscriptions.Entitlements
//	@Failure		401	{object}	error
//	@Failure		403	{object}	error
//	@Router			/billing/subscription [get]
func GetBillingSubscription(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	if ctx.CurrentOrgID == "" {
		return http.StatusForbidden, errors.New("organization is required")
	}

	entitlements, err := actions.OrganizationEntitlements(ctx.Context, ctx.CurrentOrgID)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(entitlements); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// CreateBillingCheckoutSession godoc
//
//	@Summary		Start a subscription purchase
//	@Description	Creates a provider-hosted checkout session and returns the URL the browser must be redirected to. The organization becomes the billing customer; the owner's email receives receipts.
//	@Tags			billing
//	@Accept			json
//	@Produce		json
//	@Param			X-Auth				header	string					true	"Authorization"
//	@Param			X-Organization-Id	header	string					true	"Organization ID"
//	@Security		JWTAuth
//	@Param			body				body	controllers.checkoutRequestBody	true	"Checkout options"
//	@Success		200	{object}	subscriptions.RedirectSession
//	@Failure		400	{object}	error
//	@Failure		401	{object}	error
//	@Failure		501	{object}	error
//	@Router			/billing/checkout-session [post]
func CreateBillingCheckoutSession(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	if ctx.CurrentOrgID == "" {
		return http.StatusForbidden, errors.New("organization is required")
	}

	var body checkoutRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, errors.New("invalid request body")
	}

	session, status, err := actions.StartBillingCheckout(
		ctx.Context, ctx.CurrentOrgID, body.PriceID, body.Currency, body.Quantity, body.SuccessURL, body.CancelURL)
	if err != nil {
		return status, err
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(session); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// CreateBillingPortalSession godoc
//
//	@Summary		Open the billing management portal
//	@Description	Creates a provider-hosted portal session where an administrator can change licence counts, switch plan, update payment details, download invoices and cancel. Pass `flow` to deep-link straight to one of those tasks. Licence changes made there are prorated by the provider and arrive back over webhook.
//	@Tags			billing
//	@Accept			json
//	@Produce		json
//	@Param			X-Auth				header	string					true	"Authorization"
//	@Param			X-Organization-Id	header	string					true	"Organization ID"
//	@Security		JWTAuth
//	@Param			body				body	controllers.portalRequestBody	false	"Portal options"
//	@Success		200	{object}	subscriptions.RedirectSession
//	@Failure		401	{object}	error
//	@Failure		409	{object}	error
//	@Failure		501	{object}	error
//	@Router			/billing/portal-session [post]
func CreateBillingPortalSession(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	if ctx.CurrentOrgID == "" {
		return http.StatusForbidden, errors.New("organization is required")
	}

	// The body is optional: with no return_url the provider sends the browser
	// back to the default billing page.
	var body portalRequestBody
	_ = json.NewDecoder(r.Body).Decode(&body)

	session, status, err := actions.StartBillingPortal(
		ctx.Context, ctx.CurrentOrgID, body.ReturnURL, body.Flow)
	if err != nil {
		return status, err
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(session); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// GetBillingInvoices godoc
//
//	@Summary		List past invoices
//	@Description	Returns the organization's billing history, newest first, for rendering in your own UI. Drafts are excluded. Each entry carries a hosted page and a PDF link. An organization that has never purchased returns an empty list.
//	@Tags			billing
//	@Produce		json
//	@Param			X-Auth				header	string	true	"Authorization"
//	@Param			X-Organization-Id	header	string	true	"Organization ID"
//	@Security		JWTAuth
//	@Param			limit	query	int	false	"Maximum invoices to return (default 12, max 100)"
//	@Success		200	{array}		subscriptions.Invoice
//	@Failure		401	{object}	error
//	@Failure		403	{object}	error
//	@Failure		501	{object}	error
//	@Router			/billing/invoices [get]
func GetBillingInvoices(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	if ctx.CurrentOrgID == "" {
		return http.StatusForbidden, errors.New("organization is required")
	}

	var limit int64
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return http.StatusBadRequest, errors.New("limit must be a number")
		}
		limit = parsed
	}

	invoices, status, err := actions.BillingInvoices(ctx.Context, ctx.CurrentOrgID, limit)
	if err != nil {
		return status, err
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(invoices); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}
