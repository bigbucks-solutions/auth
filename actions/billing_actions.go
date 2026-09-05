package actions

import (
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/settings"
	"bigbucks/solution/auth/subscriptions"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"gorm.io/gorm"
)

// ErrBillingUnavailable is returned when billing endpoints are called while
// subscriptions are disabled or the provider cannot host checkout.
var ErrBillingUnavailable = errors.New("billing is not enabled for this deployment")

// BillingContact is who a provider's customer record is created for.
//
// The organization is the customer; the owner is only the person receipts and
// dunning notices are addressed to.
type BillingContact struct {
	OrgID   string
	OrgName string
	Email   string
}

// OrganizationBillingContact resolves the billing identity of an organization.
//
// The email is taken from the holder of the organization's Owner role — the
// account that created it — and falls back to the organization's own contact
// address if that user has since been removed.
func OrganizationBillingContact(ctx context.Context, orgID string) (BillingContact, error) {
	var contact BillingContact

	var org models.Organization
	if err := models.Dbcon.WithContext(ctx).Where("id = ?", orgID).First(&org).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return contact, fmt.Errorf("organization %s not found", orgID)
		}
		return contact, err
	}

	contact = BillingContact{OrgID: orgID, OrgName: org.Name, Email: org.ContactEmail}

	var ownerEmail string
	err := models.Dbcon.WithContext(ctx).
		Table("users").
		Select("users.username").
		Joins("JOIN user_org_roles ON user_org_roles.user_id = users.id").
		Joins("JOIN roles ON roles.id = user_org_roles.role_id").
		Where("user_org_roles.org_id = ?", orgID).
		Where("roles.name = ?", "Owner").
		Where("users.deleted_at IS NULL").
		Order("users.created_at").
		Limit(1).
		Scan(&ownerEmail).Error
	if err != nil {
		return contact, err
	}
	if ownerEmail != "" {
		contact.Email = ownerEmail
	}
	return contact, nil
}

// OrganizationCountry returns an organization's ISO-3166 country code, used to
// decide which currency to quote. It returns an empty string rather than an
// error: an unknown country falls back to the configured default currency,
// which is not worth failing a pricing page over.
func OrganizationCountry(ctx context.Context, orgID string) string {
	if models.Dbcon == nil {
		return ""
	}
	var country string
	if err := models.Dbcon.WithContext(ctx).
		Model(&models.Organization{}).
		Where("id = ?", orgID).
		Limit(1).
		Pluck("country", &country).Error; err != nil {
		loging.Logger.Warnw("could not read organization country for pricing",
			"org_id", orgID, "error", err.Error())
		return ""
	}
	return strings.TrimSpace(country)
}

// OrganizationEntitlements reports an organization's current billing state.
func OrganizationEntitlements(ctx context.Context, orgID string) (subscriptions.Entitlements, error) {
	return subscriptions.CurrentPolicy().Entitlements(ctx, models.Dbcon, orgID)
}

// BillingCatalog lists the purchasable plans, keyed by provider price id. It is
// empty when subscriptions are disabled.
func BillingCatalog() map[string]subscriptions.Product {
	config := subscriptions.CurrentModule().Config
	if config.EffectiveMode() == subscriptions.ModeDisabled {
		return map[string]subscriptions.Product{}
	}
	catalog := make(map[string]subscriptions.Product, len(config.Catalog))
	for priceID, product := range config.Catalog {
		catalog[priceID] = product
	}
	return catalog
}

// StartBillingCheckout opens a provider-hosted purchase flow for an organization.
func StartBillingCheckout(ctx context.Context, orgID, priceID, currency string, quantity int64, successPath, cancelPath string) (subscriptions.RedirectSession, int, error) {
	var session subscriptions.RedirectSession

	portal, ok := subscriptions.CurrentModule().Portal()
	if !ok {
		return session, http.StatusNotImplemented, ErrBillingUnavailable
	}
	if strings.TrimSpace(priceID) == "" {
		return session, http.StatusBadRequest, errors.New("price_id is required")
	}

	contact, err := OrganizationBillingContact(ctx, orgID)
	if err != nil {
		return session, http.StatusNotFound, err
	}

	session, err = portal.StartCheckout(ctx, subscriptions.CheckoutRequest{
		OrgID:      contact.OrgID,
		OrgName:    contact.OrgName,
		OwnerEmail: contact.Email,
		PriceID:    priceID,
		Quantity:   quantity,
		Currency:   currency,
		SuccessURL: ResolveBillingRedirect(successPath, "/billing/success"),
		CancelURL:  ResolveBillingRedirect(cancelPath, "/billing/cancelled"),
	})
	if err != nil {
		// A currency the price does not carry, or one the organization is no
		// longer free to choose, is the caller's mistake rather than the
		// provider being unreachable.
		if strings.Contains(err.Error(), "is not available in") ||
			strings.Contains(err.Error(), "currency is fixed once billing has started") {
			return session, http.StatusBadRequest, err
		}
		return session, http.StatusBadGateway, err
	}
	return session, 0, nil
}

// StartBillingPortal opens the provider's self-service management page, where an
// administrator can change licence counts, switch plan, update payment details,
// download invoices or cancel.
//
// The flow deep-links to one of those tasks so a button on the billing page
// lands on that screen rather than the portal's home.
func StartBillingPortal(ctx context.Context, orgID, returnPath, flow string) (subscriptions.RedirectSession, int, error) {
	var session subscriptions.RedirectSession

	portal, ok := subscriptions.CurrentModule().Portal()
	if !ok {
		return session, http.StatusNotImplemented, ErrBillingUnavailable
	}

	portalFlow, err := parsePortalFlow(flow)
	if err != nil {
		return session, http.StatusBadRequest, err
	}

	contact, err := OrganizationBillingContact(ctx, orgID)
	if err != nil {
		return session, http.StatusNotFound, err
	}

	request := subscriptions.PortalRequest{
		OrgID:      contact.OrgID,
		OrgName:    contact.OrgName,
		OwnerEmail: contact.Email,
		ReturnURL:  ResolveBillingRedirect(returnPath, "/billing"),
		Flow:       portalFlow,
	}

	// Cancel and plan-change act on a specific subscription. Resolving it here
	// means the frontend never has to know provider identifiers.
	if portalFlow == subscriptions.PortalFlowCancel || portalFlow == subscriptions.PortalFlowUpdatePlan {
		subscriptionID, err := ActiveSubscriptionID(ctx, orgID)
		if err != nil {
			return session, http.StatusInternalServerError, err
		}
		if subscriptionID == "" {
			return session, http.StatusConflict,
				fmt.Errorf("organization has no active subscription to %s", flow)
		}
		request.SubscriptionID = subscriptionID
	}

	session, err = portal.StartPortal(ctx, request)
	if err != nil {
		return session, http.StatusBadGateway, err
	}
	return session, 0, nil
}

// parsePortalFlow validates a caller-supplied flow name.
func parsePortalFlow(flow string) (subscriptions.PortalFlow, error) {
	switch subscriptions.PortalFlow(strings.ToLower(strings.TrimSpace(flow))) {
	case subscriptions.PortalFlowHome:
		return subscriptions.PortalFlowHome, nil
	case subscriptions.PortalFlowCancel:
		return subscriptions.PortalFlowCancel, nil
	case subscriptions.PortalFlowUpdatePlan:
		return subscriptions.PortalFlowUpdatePlan, nil
	case subscriptions.PortalFlowPaymentMethod:
		return subscriptions.PortalFlowPaymentMethod, nil
	default:
		return "", fmt.Errorf("unknown flow %q; expected cancel, update_plan or payment_method", flow)
	}
}

// ActiveSubscriptionID returns the provider subscription an organization is
// currently billed under, or an empty string if it has none.
func ActiveSubscriptionID(ctx context.Context, orgID string) (string, error) {
	var subscriptionID string
	err := models.Dbcon.WithContext(ctx).
		Table("subscription_items").
		Select("subscription_items.subscription_id").
		Joins("JOIN billing_accounts ON billing_accounts.id = subscription_items.account_id"+
			" AND billing_accounts.deleted_at IS NULL").
		Where("billing_accounts.org_id = ?", orgID).
		Where("subscription_items.active = ?", true).
		Where("subscription_items.deleted_at IS NULL").
		Order("subscription_items.created_at DESC").
		Limit(1).
		Scan(&subscriptionID).Error
	return subscriptionID, err
}

// BillingInvoices returns an organization's billing history, newest first.
func BillingInvoices(ctx context.Context, orgID string, limit int64) ([]subscriptions.Invoice, int, error) {
	lister, ok := subscriptions.CurrentModule().Invoices()
	if !ok {
		return nil, http.StatusNotImplemented, ErrBillingUnavailable
	}
	invoices, err := lister.ListInvoices(ctx, orgID, limit)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	return invoices, 0, nil
}

// ResolveBillingRedirect turns a caller-supplied return location into an
// absolute URL on the configured UI origin.
//
// The provider redirects the browser to whatever is passed here, so an
// unvalidated value would be an open redirect. Only paths, or absolute URLs
// already on the UI origin, are accepted; anything else falls back to the
// supplied default path.
func ResolveBillingRedirect(candidate, defaultPath string) string {
	var base string
	if settings.Current != nil {
		base = strings.TrimRight(settings.Current.EmailLinkHost(), "/")
	}
	fallback := base + defaultPath

	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return fallback
	}

	if strings.HasPrefix(candidate, "/") && !strings.HasPrefix(candidate, "//") {
		return base + candidate
	}

	parsed, err := url.Parse(candidate)
	if err != nil || base == "" {
		return fallback
	}
	baseParsed, err := url.Parse(base)
	if err != nil {
		return fallback
	}
	if parsed.Scheme == baseParsed.Scheme && parsed.Host == baseParsed.Host {
		return parsed.String()
	}
	return fallback
}
