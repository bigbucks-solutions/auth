package actions

import (
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/subscriptions"
	valids "bigbucks/solution/auth/validations"
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// InviteUserParams contains parameters for inviting a user
type InviteUserParams struct {
	Email     string
	OrgID     string
	RoleID    string
	InviterID string
}

// InviteUserToOrg creates an invitation for a user to join an organization with a specific role
func InviteUserToOrg(params InviteUserParams) (*models.Invitation, int, error) {
	customerr := valids.NewErrorDict()

	// Validate parameters
	if params.Email == "" {
		customerr.Errors["email"] = "Email is required"
	}
	if params.OrgID == "" {
		customerr.Errors["org_id"] = "Organization ID is required"
	}
	if params.RoleID == "" {
		customerr.Errors["role_id"] = "Role ID is required"
	}
	if params.InviterID == "" {
		customerr.Errors["inviter_id"] = "Inviter ID is required"
	}

	if len(customerr.Errors) > 0 {
		return nil, http.StatusBadRequest, customerr
	}

	var invitation *models.Invitation
	err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
		// Check if organization exists
		var org models.Organization
		if err := tx.Where("id = ?", params.OrgID).First(&org).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				customerr.Errors["org_id"] = "Organization not found"
				return customerr
			}
			return err
		}

		// Check if role exists and belongs to the organization
		var role models.Role
		if err := tx.Where("id = ? AND org_id = ?", params.RoleID, params.OrgID).First(&role).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				customerr.Errors["role_id"] = "Role not found in the specified organization"
				return customerr
			}
			return err
		}

		// Check if inviter exists and belongs to the organization
		var inviter models.User
		if err := tx.Where("id = ?", params.InviterID).First(&inviter).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				customerr.Errors["inviter_id"] = "Inviter not found"
				return customerr
			}
			return err
		}

		// Check if inviter has permission in the organization
		var userOrgRole models.UserOrgRole
		if err := tx.Where("user_id = ? AND org_id = ?", params.InviterID, params.OrgID).First(&userOrgRole).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				customerr.Errors["inviter_id"] = "Inviter does not belong to the organization"
				return customerr
			}
			return err
		}

		// Check if user is already a member of the organization
		var existingUser models.User
		if err := tx.Where("username = ?", params.Email).First(&existingUser).Error; err == nil {
			// User exists, check if they're already in the organization
			var existingUserOrgRole models.UserOrgRole
			if err := tx.Where("user_id = ? AND org_id = ?", existingUser.ID, params.OrgID).First(&existingUserOrgRole).Error; err == nil {
				customerr.Errors["email"] = "User is already a member of this organization"
				return customerr
			}
		}

		// Check if there's already a pending invitation for this email and organization
		var existingInvitation models.Invitation
		if err := tx.Where("email = ? AND org_id = ? AND status = ?", params.Email, params.OrgID, models.InvitationStatusPending).First(&existingInvitation).Error; err == nil {
			// If invitation exists and is expired, mark it as expired and allow new invitation
			if existingInvitation.IsExpired() {
				existingInvitation.Status = models.InvitationStatusExpired
				if err := tx.Save(&existingInvitation).Error; err != nil {
					return err
				}
			} else {
				// If invitation is still valid, don't allow new invitation
				customerr.Errors["email"] = "A pending invitation already exists for this email in this organization"
				return customerr
			}
		}

		// Reserve a licence up front. Pending invitations count towards usage, so
		// the administrator is told here rather than letting the invitee discover
		// the organization is full when they try to accept.
		policy := subscriptions.CurrentPolicy()
		if err := policy.RequireEntitled(context.Background(), tx, params.OrgID); err != nil {
			return err
		}
		if err := policy.RequireLicenses(context.Background(), tx, params.OrgID, 1); err != nil {
			return err
		}

		// Generate unique token
		token, err := generateInvitationToken()
		if err != nil {
			return err
		}

		// Create invitation
		invitation = &models.Invitation{
			Email:     params.Email,
			InviterID: params.InviterID,
			OrgID:     params.OrgID,
			RoleID:    params.RoleID,
			Status:    models.InvitationStatusPending,
			Token:     token,
			ExpiresAt: time.Now().Add(7 * 24 * time.Hour), // 7 days expiration
		}

		if err := tx.Create(invitation).Error; err != nil {
			return err
		}

		// Preload relationships for return
		if err := tx.Preload("Inviter").Preload("Organization").Preload("Role").First(invitation, "id = ?", invitation.ID).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		if status, denied := subscriptionStatus(err); denied {
			loging.Logger.Warnw("invitation blocked by subscription",
				"org_id", params.OrgID, "error", err.Error())
			return nil, status, err
		}
		loging.Logger.Error("Error inviting user to organization", err)
		return nil, http.StatusBadRequest, err
	}

	// Send invitation email
	go func() {
		if err := SendInvitationEmail(invitation); err != nil {
			loging.Logger.Error("Failed to send invitation email", err)
		}
	}()

	return invitation, http.StatusCreated, nil
}

// AcceptInvitation accepts an invitation using the token
func AcceptInvitation(token string, userID string) (int, error) {
	customerr := valids.NewErrorDict()

	if token == "" {
		customerr.Errors["token"] = "Token is required"
		return http.StatusBadRequest, customerr
	}
	if userID == "" {
		customerr.Errors["user_id"] = "User ID is required"
		return http.StatusBadRequest, customerr
	}

	err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
		// Find invitation by token
		var invitation models.Invitation
		if err := tx.Preload("Role").Where("token = ?", token).First(&invitation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				customerr.Errors["token"] = "Invalid invitation token"
				return customerr
			}
			return err
		}

		// Validate invitation can be accepted
		if !invitation.CanBeAccepted() {
			if invitation.IsExpired() {
				customerr.Errors["token"] = "Invitation has expired"
				customerr.Errors["_status"] = strconv.Itoa(http.StatusNotAcceptable)
			} else {
				customerr.Errors["token"] = "Invitation cannot be accepted"
				customerr.Errors["_status"] = strconv.Itoa(http.StatusNotAcceptable)
			}
			return customerr
		}

		// Get user and validate email matches
		var user models.User
		if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				customerr.Errors["user_id"] = "User not found"
				customerr.Errors["_status"] = strconv.Itoa(http.StatusExpectationFailed)
				return customerr
			}
			return err
		}

		if models.AccountEmail(user.Username) != invitation.Email {
			customerr.Errors["email"] = "Invitation email does not match user email"
			customerr.Errors["_status"] = strconv.Itoa(http.StatusExpectationFailed)
			return customerr
		}

		// Check if user is already a member of the organization
		var existingUserOrgRole models.UserOrgRole
		if err := tx.Where("user_id = ? AND org_id = ?", user.ID, invitation.OrgID).First(&existingUserOrgRole).Error; err == nil {
			customerr.Errors["user"] = "User is already a member of this organization"
			return customerr
		}

		// Consume a licence in the same transaction that records the membership,
		// so concurrent acceptances cannot both take the last one.
		if err := requireMembershipLicense(context.Background(), tx, invitation.OrgID, user.ID); err != nil {
			return err
		}

		// Create user-org-role binding
		userOrgRole := models.UserOrgRole{
			UserID: user.ID,
			RoleID: invitation.RoleID,
			OrgID:  invitation.OrgID,
		}

		if err := tx.Create(&userOrgRole).Error; err != nil {
			return err
		}

		// Update invitation status
		now := time.Now()
		invitation.Status = models.InvitationStatusAccepted
		invitation.AcceptedAt = &now

		if err := tx.Save(&invitation).Error; err != nil {
			return err
		}

		return nil
	})

	//check error type and return appropriate status code

	if err != nil {
		if status, denied := subscriptionStatus(err); denied {
			loging.Logger.Warnw("invitation acceptance blocked by subscription",
				"user_id", userID, "error", err.Error())
			return status, err
		}
		switch err := err.(type) {
		case *valids.ValidationErrors:
			loging.Logger.Error("Validation error accepting invitation", err)
			if err.Errors["_status"] != "" {
				status, convErr := strconv.Atoi(err.Errors["_status"])
				if convErr == nil {
					delete(err.Errors, "_status")
					return status, err
				}
			}
			return http.StatusBadRequest, err
		default:
			loging.Logger.Error("Error accepting invitation", err)
			return http.StatusInternalServerError, err
		}
	}

	return http.StatusOK, nil
}
