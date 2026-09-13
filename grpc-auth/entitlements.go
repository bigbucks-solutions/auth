package grpc_auth

import (
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/subscriptions"
	context "context"
	"fmt"
	"strings"
	"time"

	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

const maxBatchOrganizations = 100

// EntitlementsService lets other services read an organization's subscription
// state, features and limits so they can apply their own business rules.
//
// It serves read-only snapshots. Callers evaluate features and limits against
// a snapshot themselves: they own the usage counts, so a server-side check would
// only repeat their comparison at the cost of a round trip.
//
// Every answer is resolved from the local billing projection, never from the
// provider. Callers should still cache a snapshot briefly rather than calling on
// every record they touch.
type EntitlementsService struct {
	UnimplementedEntitlementsServer
	module   func() *subscriptions.Module
	db       func() *gorm.DB
	isMember func(orgID, username string) (bool, error)
}

// NewEntitlementsService serves from the process-wide subscription module.
func NewEntitlementsService() *EntitlementsService {
	return &EntitlementsService{
		module:   subscriptions.CurrentModule,
		db:       func() *gorm.DB { return models.Dbcon },
		isMember: models.IsOrganizationMember,
	}
}

func (server *EntitlementsService) GetEntitlements(ctx context.Context, request *GetEntitlementsRequest) (*OrgEntitlements, error) {
	orgID := strings.TrimSpace(request.GetOrgId())
	if err := server.authorize(ctx, orgID); err != nil {
		return nil, err
	}
	entitlements, err := server.resolve(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return toOrgEntitlements(entitlements, server.enforced()), nil
}

func (server *EntitlementsService) BatchGetEntitlements(ctx context.Context, request *BatchGetEntitlementsRequest) (*BatchGetEntitlementsResponse, error) {
	caller, ok := CallerFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "caller is not authenticated")
	}
	if !caller.IsService() {
		return nil, status.Error(codes.PermissionDenied, "batch lookups are available to service callers only")
	}
	orgIDs := request.GetOrgIds()
	if len(orgIDs) == 0 {
		return nil, status.Error(codes.InvalidArgument, "org_ids is required")
	}
	if len(orgIDs) > maxBatchOrganizations {
		return nil, status.Error(codes.InvalidArgument,
			fmt.Sprintf("at most %d org_ids may be requested at once", maxBatchOrganizations))
	}

	enforced := server.enforced()
	response := &BatchGetEntitlementsResponse{
		Entitlements: make(map[string]*OrgEntitlements, len(orgIDs)),
	}
	for _, orgID := range orgIDs {
		orgID = strings.TrimSpace(orgID)
		if orgID == "" {
			return nil, status.Error(codes.InvalidArgument, "org_ids must not contain empty values")
		}
		if _, resolved := response.Entitlements[orgID]; resolved {
			continue
		}
		entitlements, err := server.resolve(ctx, orgID)
		if err != nil {
			return nil, err
		}
		response.Entitlements[orgID] = toOrgEntitlements(entitlements, enforced)
	}
	return response, nil
}

// authorize admits service callers for any organization, and users only for
// organizations they belong to.
func (server *EntitlementsService) authorize(ctx context.Context, orgID string) error {
	caller, ok := CallerFromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "caller is not authenticated")
	}
	if orgID == "" {
		return status.Error(codes.InvalidArgument, "org_id is required")
	}
	if caller.IsService() {
		return nil
	}
	member, err := server.isMember(orgID, caller.User.Username)
	if err != nil {
		loging.Logger.Errorw("grpc organization membership check failed",
			"org_id", orgID, "error", err.Error())
		return status.Error(codes.Internal, "could not verify organization membership")
	}
	if !member {
		return status.Error(codes.PermissionDenied, "caller is not a member of this organization")
	}
	return nil
}

func (server *EntitlementsService) resolve(ctx context.Context, orgID string) (subscriptions.Entitlements, error) {
	entitlements, err := server.module().Policy.Entitlements(ctx, server.db(), orgID)
	if err != nil {
		loging.Logger.Errorw("grpc entitlement resolution failed", "org_id", orgID, "error", err.Error())
		return entitlements, status.Error(codes.Internal, "could not resolve entitlements")
	}
	return entitlements, nil
}

// enforced reports whether callers should block on denials. In observe mode they
// evaluate and log them but allow the operation, matching the REST layer.
func (server *EntitlementsService) enforced() bool {
	return server.module().Config.EffectiveMode() == subscriptions.ModeEnforce
}

var subscriptionStates = map[subscriptions.State]SubscriptionState{
	subscriptions.StateNotManaged: SubscriptionState_SUBSCRIPTION_STATE_NOT_MANAGED,
	subscriptions.StateNone:       SubscriptionState_SUBSCRIPTION_STATE_NONE,
	subscriptions.StateTrialing:   SubscriptionState_SUBSCRIPTION_STATE_TRIALING,
	subscriptions.StateActive:     SubscriptionState_SUBSCRIPTION_STATE_ACTIVE,
	subscriptions.StatePastDue:    SubscriptionState_SUBSCRIPTION_STATE_PAST_DUE,
	subscriptions.StateCanceled:   SubscriptionState_SUBSCRIPTION_STATE_CANCELED,
	subscriptions.StateExpired:    SubscriptionState_SUBSCRIPTION_STATE_EXPIRED,
}

var limitKinds = map[subscriptions.LimitKind]LimitKind{
	subscriptions.LimitKindCap:          LimitKind_LIMIT_KIND_CAP,
	subscriptions.LimitKindMonthlyQuota: LimitKind_LIMIT_KIND_MONTHLY_QUOTA,
}

func toOrgEntitlements(entitlements subscriptions.Entitlements, enforced bool) *OrgEntitlements {
	limits := make(map[string]*EntitlementLimit, len(entitlements.Limits))
	for key, grant := range entitlements.Limits {
		limits[key] = toEntitlementLimit(grant)
	}
	plans := make([]*EntitlementPlan, 0, len(entitlements.Plans))
	for _, plan := range entitlements.Plans {
		plans = append(plans, &EntitlementPlan{
			PriceId:           plan.PriceID,
			Name:              plan.Name,
			Tier:              plan.Tier,
			Quantity:          plan.Quantity,
			Licenses:          plan.Licenses,
			ProviderStatus:    plan.Status,
			CancelAtPeriodEnd: plan.CancelAtPeriodEnd,
			CurrentPeriodEnd:  toTimestamp(plan.CurrentPeriodEnd),
		})
	}

	return &OrgEntitlements{
		OrgId:              entitlements.OrgID,
		Entitled:           entitlements.Entitled,
		State:              subscriptionStates[entitlements.State],
		ProviderStatus:     entitlements.Status,
		Managed:            entitlements.ManagedExternally,
		Enforced:           enforced,
		TrialEndsAt:        toTimestamp(entitlements.TrialEndsAt),
		CurrentPeriodStart: toTimestamp(entitlements.CurrentPeriodStart),
		CurrentPeriodEnd:   toTimestamp(entitlements.CurrentPeriodEnd),
		EndedAt:            toTimestamp(entitlements.EndedAt),
		CancelAtPeriodEnd:  entitlements.CancelAtPeriodEnd,
		Features:           append([]string(nil), entitlements.Features...),
		Licenses:           entitlements.Licenses,
		LicensesUsed:       entitlements.LicensesUsed,
		LicensesAvailable:  entitlements.LicensesAvailable,
		OverLimit:          entitlements.OverLimit,
		Limits:             limits,
		Plans:              plans,
		ResolvedAt:         timestamppb.New(entitlements.ResolvedAt),
	}
}

func toEntitlementLimit(grant subscriptions.LimitGrant) *EntitlementLimit {
	limit := &EntitlementLimit{
		Kind:        limitKinds[grant.Kind],
		Unlimited:   grant.IsUnlimited(),
		PeriodStart: toTimestamp(grant.PeriodStart),
		PeriodEnd:   toTimestamp(grant.PeriodEnd),
	}
	if !limit.Unlimited {
		limit.Limit = grant.Limit
	}
	return limit
}

func toTimestamp(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return timestamppb.New(*value)
}
