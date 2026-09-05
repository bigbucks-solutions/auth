package actions

import (
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/subscriptions"
	"context"
	"net/http"

	"gorm.io/gorm"
)

// requireMembershipLicense checks that an organization may take on one more
// member.
//
// It must be called inside the same transaction that writes the membership:
// the policy takes a row lock on the organization's billing account, which is
// what stops two concurrent additions from both claiming the last licence.
//
// Users who already hold a role in the organization consume a licence they have
// already been counted for, so granting them an additional role is always
// allowed.
func requireMembershipLicense(ctx context.Context, tx *gorm.DB, orgID, userID string) error {
	var existingRoles int64
	if err := tx.WithContext(ctx).Model(&models.UserOrgRole{}).
		Where("org_id = ? AND user_id = ?", orgID, userID).
		Count(&existingRoles).Error; err != nil {
		return err
	}
	if existingRoles > 0 {
		return nil
	}

	policy := subscriptions.CurrentPolicy()
	if err := policy.RequireEntitled(ctx, tx, orgID); err != nil {
		return err
	}
	return policy.RequireLicenses(ctx, tx, orgID, 1)
}

// subscriptionStatus maps an entitlement denial onto an HTTP status, returning
// ok=false for anything that is not a denial so callers keep their own error
// handling for real faults.
func subscriptionStatus(err error) (int, bool) {
	if err == nil || !subscriptions.IsDenial(err) {
		return 0, false
	}
	status := subscriptions.HTTPStatus(err)
	if status == http.StatusInternalServerError {
		return 0, false
	}
	return status, true
}
