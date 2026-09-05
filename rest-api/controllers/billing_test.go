package controllers

import (
	"bigbucks/solution/auth/request_context"
	"net/http/httptest"
	"testing"
)

func TestBillingPricingRequestUsesFrontendCountry(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/v1/billing/plans?country=IN&currency=inr", nil)
	ctx := &request_context.Context{CurrentOrgID: "org_123"}

	pricingRequest := billingPricingRequest(request, ctx)

	if pricingRequest.Country != "IN" {
		t.Fatalf("Country = %q, want IN", pricingRequest.Country)
	}
	if pricingRequest.Currency != "inr" {
		t.Fatalf("Currency = %q, want inr", pricingRequest.Currency)
	}
	if pricingRequest.OrgID != "org_123" {
		t.Fatalf("OrgID = %q, want org_123", pricingRequest.OrgID)
	}
}
