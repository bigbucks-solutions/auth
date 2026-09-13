package grpc_auth

import (
	"bigbucks/solution/auth/settings"
	"bigbucks/solution/auth/subscriptions"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type stubEntitlementsPolicy struct {
	entitlements subscriptions.Entitlements
	err          error
}

func (policy stubEntitlementsPolicy) RequireEntitled(context.Context, *gorm.DB, string) error {
	return nil
}
func (policy stubEntitlementsPolicy) RequireFeature(context.Context, *gorm.DB, string, string) error {
	return nil
}
func (policy stubEntitlementsPolicy) RequireLicenses(context.Context, *gorm.DB, string, int64) error {
	return nil
}
func (policy stubEntitlementsPolicy) Entitlements(_ context.Context, _ *gorm.DB, orgID string) (subscriptions.Entitlements, error) {
	entitlements := policy.entitlements
	entitlements.OrgID = orgID
	return entitlements, policy.err
}

func newTestEntitlementsService(mode subscriptions.Mode, policy stubEntitlementsPolicy) *EntitlementsService {
	module := &subscriptions.Module{Config: subscriptions.Config{Mode: mode}, Policy: policy}
	return &EntitlementsService{
		module: func() *subscriptions.Module { return module },
		db:     func() *gorm.DB { return nil },
		isMember: func(orgID, username string) (bool, error) {
			return orgID == "org-1" && username == "member@example.com", nil
		},
	}
}

func serviceCaller() context.Context {
	return context.WithValue(context.Background(), callerKey{}, Caller{Service: "inventory"})
}

func userCaller(username string) context.Context {
	return context.WithValue(context.Background(), callerKey{},
		Caller{UserID: "user-1", User: settings.UserInfo{Username: username}})
}

func paidEntitlements() subscriptions.Entitlements {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	return subscriptions.Entitlements{
		Entitled:          true,
		State:             subscriptions.StateActive,
		Status:            "active",
		ManagedExternally: true,
		Features:          []string{"cloud_access"},
		Licenses:          3,
		Limits: map[string]subscriptions.LimitGrant{
			"invoices": {Kind: subscriptions.LimitKindMonthlyQuota, Limit: 100, PeriodStart: &start, PeriodEnd: &end},
			"skus":     {Kind: subscriptions.LimitKindCap, Limit: subscriptions.Unlimited},
		},
		Plans: []subscriptions.PlanSummary{
			{PriceID: "price_1", Name: "Starter", Tier: "starter", Quantity: 3, Licenses: 3, Status: "active"},
		},
		ResolvedAt: time.Now().UTC(),
	}
}

func TestGetEntitlementsMapsTheSnapshot(t *testing.T) {
	server := newTestEntitlementsService(subscriptions.ModeEnforce, stubEntitlementsPolicy{entitlements: paidEntitlements()})

	response, err := server.GetEntitlements(serviceCaller(), &GetEntitlementsRequest{OrgId: "org-1"})
	if err != nil {
		t.Fatalf("GetEntitlements() error = %v", err)
	}
	if !response.Entitled || !response.Managed || !response.Enforced ||
		response.State != SubscriptionState_SUBSCRIPTION_STATE_ACTIVE || response.OrgId != "org-1" {
		t.Fatalf("unexpected snapshot: %v", response)
	}
	invoices := response.Limits["invoices"]
	if invoices.Kind != LimitKind_LIMIT_KIND_MONTHLY_QUOTA || invoices.Limit != 100 || invoices.Unlimited ||
		invoices.PeriodStart == nil || invoices.PeriodEnd == nil {
		t.Fatalf("invoices = %v, want a 100 monthly quota with its window", invoices)
	}
	skus := response.Limits["skus"]
	if skus.Kind != LimitKind_LIMIT_KIND_CAP || !skus.Unlimited || skus.Limit != 0 || skus.PeriodStart != nil {
		t.Fatalf("skus = %v, want an unlimited cap with no window", skus)
	}
	if len(response.Plans) != 1 || response.Plans[0].Tier != "starter" || response.ResolvedAt == nil {
		t.Fatalf("plans = %v resolved_at = %v", response.Plans, response.ResolvedAt)
	}
}

func TestEntitlementsAuthorization(t *testing.T) {
	server := newTestEntitlementsService(subscriptions.ModeEnforce, stubEntitlementsPolicy{entitlements: paidEntitlements()})
	tests := []struct {
		name  string
		ctx   context.Context
		orgID string
		want  codes.Code
	}{
		{"a service may read any organization", serviceCaller(), "org-2", codes.OK},
		{"a member may read their organization", userCaller("member@example.com"), "org-1", codes.OK},
		{"a user may not read another organization", userCaller("member@example.com"), "org-2", codes.PermissionDenied},
		{"an outsider is denied", userCaller("outsider@example.com"), "org-1", codes.PermissionDenied},
		{"an unauthenticated context is rejected", context.Background(), "org-1", codes.Unauthenticated},
		{"org_id is required", serviceCaller(), "  ", codes.InvalidArgument},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := server.GetEntitlements(test.ctx, &GetEntitlementsRequest{OrgId: test.orgID})
			if got := status.Code(err); got != test.want {
				t.Fatalf("code = %s, want %s (err %v)", got, test.want, err)
			}
		})
	}
}

func TestBatchGetEntitlements(t *testing.T) {
	server := newTestEntitlementsService(subscriptions.ModeEnforce, stubEntitlementsPolicy{entitlements: paidEntitlements()})

	if _, err := server.BatchGetEntitlements(userCaller("member@example.com"),
		&BatchGetEntitlementsRequest{OrgIds: []string{"org-1"}}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("user batch code = %s, want PermissionDenied", status.Code(err))
	}

	response, err := server.BatchGetEntitlements(serviceCaller(),
		&BatchGetEntitlementsRequest{OrgIds: []string{"org-1", "org-1", "org-2"}})
	if err != nil {
		t.Fatalf("BatchGetEntitlements() error = %v", err)
	}
	if len(response.Entitlements) != 2 || response.Entitlements["org-2"].OrgId != "org-2" {
		t.Fatalf("entitlements = %v, want org-1 and org-2 once each", response.Entitlements)
	}

	tooMany := make([]string, maxBatchOrganizations+1)
	for index := range tooMany {
		tooMany[index] = fmt.Sprintf("org-%d", index)
	}
	for name, orgIDs := range map[string][]string{"empty": nil, "too many": tooMany, "blank id": {"org-1", ""}} {
		if _, err := server.BatchGetEntitlements(serviceCaller(), &BatchGetEntitlementsRequest{OrgIds: orgIDs}); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("%s: code = %s, want InvalidArgument", name, status.Code(err))
		}
	}
}

// Callers apply limits themselves, so the snapshot must tell them when not to
// block: in observe mode, and when billing is disabled altogether.
func TestGetEntitlementsReportsWhenNotToEnforce(t *testing.T) {
	observed := newTestEntitlementsService(subscriptions.ModeObserve, stubEntitlementsPolicy{entitlements: paidEntitlements()})
	response, err := observed.GetEntitlements(serviceCaller(), &GetEntitlementsRequest{OrgId: "org-1"})
	if err != nil {
		t.Fatalf("GetEntitlements() error = %v", err)
	}
	if response.Enforced || !response.Managed {
		t.Fatalf("observe mode = managed:%v enforced:%v, want managed and not enforced", response.Managed, response.Enforced)
	}

	unmanaged, _ := subscriptions.AllowAllPolicy{}.Entitlements(context.Background(), nil, "org-1")
	disabled := newTestEntitlementsService(subscriptions.ModeDisabled, stubEntitlementsPolicy{entitlements: unmanaged})
	response, err = disabled.GetEntitlements(serviceCaller(), &GetEntitlementsRequest{OrgId: "org-1"})
	if err != nil {
		t.Fatalf("GetEntitlements() error = %v", err)
	}
	if response.Managed || response.Enforced || !response.Entitled ||
		response.State != SubscriptionState_SUBSCRIPTION_STATE_NOT_MANAGED || len(response.Limits) != 0 {
		t.Fatalf("billing disabled = %v, want unmanaged, unenforced, entitled, no limits", response)
	}
}

func TestResolutionFailureIsInternal(t *testing.T) {
	server := newTestEntitlementsService(subscriptions.ModeEnforce, stubEntitlementsPolicy{err: errors.New("database is down")})
	_, err := server.GetEntitlements(serviceCaller(), &GetEntitlementsRequest{OrgId: "org-1"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %s, want Internal", status.Code(err))
	}
	if st, _ := status.FromError(err); st.Message() == "database is down" {
		t.Fatal("internal error details must not be returned to callers")
	}
}
