package actions

import (
	"bigbucks/solution/auth/emailservice"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	valids "bigbucks/solution/auth/validations"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
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
			} else {
				customerr.Errors["token"] = "Invitation cannot be accepted"
			}
			return customerr
		}

		// Get user and validate email matches
		var user models.User
		if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				customerr.Errors["user_id"] = "User not found"
				return customerr
			}
			return err
		}

		if user.Username != invitation.Email {
			customerr.Errors["email"] = "Invitation email does not match user email"
			return customerr
		}

		// Check if user is already a member of the organization
		var existingUserOrgRole models.UserOrgRole
		if err := tx.Where("user_id = ? AND org_id = ?", user.ID, invitation.OrgID).First(&existingUserOrgRole).Error; err == nil {
			customerr.Errors["user"] = "User is already a member of this organization"
			return customerr
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

	if err != nil {
		loging.Logger.Error("Error accepting invitation", err)
		return http.StatusBadRequest, err
	}

	return http.StatusOK, nil
}

// ListInvitationsParams contains parameters for listing invitations
type ListInvitationsParams struct {
	OrgID    string
	Page     int
	PageSize int
	Status   *models.InvitationStatus
	OrderBy  string // Field to order by (email, status, created_at, expires_at, role_name, inviter_name)
	OrderDir string // Order direction (asc, desc)
	Search   string // Search term to filter by inviter or invitee email
}

// ListInvitations lists invitations for an organization with ordering support
func ListInvitations(params ListInvitationsParams) ([]models.Invitation, int64, int, error) {
	customerr := valids.NewErrorDict()

	if params.OrgID == "" {
		customerr.Errors["org_id"] = "Organization ID is required"
		return nil, 0, http.StatusBadRequest, customerr
	}

	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 10
	}

	// Validate and set default ordering
	if params.OrderBy == "" {
		params.OrderBy = "created_at"
	}
	if params.OrderDir == "" {
		params.OrderDir = "desc"
	}

	// Validate order direction
	if params.OrderDir != "asc" && params.OrderDir != "desc" {
		params.OrderDir = "desc"
	}

	// Validate order by field
	validOrderFields := map[string]string{
		"email":        "invitations.email",
		"status":       "invitations.status",
		"sentAt":       "invitations.created_at",
		"expiresAt":    "invitations.expires_at",
		"role_name":    "roles.name",
		"inviter_name": "users.username",
	}

	orderField, isValid := validOrderFields[params.OrderBy]
	if !isValid {
		orderField = "invitations.created_at"
		params.OrderDir = "desc"
	}

	var invitations []models.Invitation
	var total int64

	// Build base query for counting
	countQuery := models.Dbcon.Model(&models.Invitation{}).
		Joins("LEFT JOIN users ON invitations.inviter_id = users.id").
		Where("invitations.org_id = ?", params.OrgID)

	// Filter by status if provided
	if params.Status != nil {
		countQuery = countQuery.Where("invitations.status = ?", *params.Status)
	}

	// Filter by search term if provided (search in invitee email or inviter email)
	if params.Search != "" {
		searchTerm := "%" + strings.ToLower(params.Search) + "%"
		countQuery = countQuery.Where(
			"LOWER(invitations.email) LIKE ? OR LOWER(users.username) LIKE ?",
			searchTerm, searchTerm,
		)
	}

	// Get total count
	if err := countQuery.Count(&total).Error; err != nil {
		loging.Logger.Error("Error counting invitations", err)
		return nil, 0, http.StatusInternalServerError, err
	}

	// Build query for selecting data with joins for ordering and data fetching
	selectQuery := models.Dbcon.Model(&models.Invitation{}).
		Select(`invitations.*,
			roles.name as role_name,
			users.username as inviter_username,
			users.id as inviter_id,
			profiles.first_name as inviter_first_name,
			profiles.last_name as inviter_last_name`).
		Joins("LEFT JOIN roles ON invitations.role_id = roles.id").
		Joins("LEFT JOIN users ON invitations.inviter_id = users.id").
		Joins("LEFT JOIN profiles ON users.id = profiles.user_id").
		Where("invitations.org_id = ?", params.OrgID)

	// Apply the same filters to select query
	if params.Status != nil {
		selectQuery = selectQuery.Where("invitations.status = ?", *params.Status)
	}

	if params.Search != "" {
		searchTerm := "%" + strings.ToLower(params.Search) + "%"
		selectQuery = selectQuery.Where(
			"LOWER(invitations.email) LIKE ? OR LOWER(users.username) LIKE ?",
			searchTerm, searchTerm,
		)
	}

	// Build order clause
	orderClause := orderField + " " + strings.ToUpper(params.OrderDir)

	// Get paginated results with single query (no N+1 problem)
	offset := (params.Page - 1) * params.PageSize

	// Create a custom struct to hold the joined data
	type InvitationWithJoinedData struct {
		models.Invitation
		RoleName         string `gorm:"column:role_name"`
		InviterUsername  string `gorm:"column:inviter_username"`
		InviterID        string `gorm:"column:inviter_id"`
		InviterFirstName string `gorm:"column:inviter_first_name"`
		InviterLastName  string `gorm:"column:inviter_last_name"`
	}

	var joinedResults []InvitationWithJoinedData
	if err := selectQuery.
		Order(orderClause).
		Offset(offset).Limit(params.PageSize).
		Find(&joinedResults).Error; err != nil {
		loging.Logger.Error("Error fetching invitations", err)
		return nil, 0, http.StatusInternalServerError, err
	}

	// Convert joined results back to Invitation objects with populated relationships
	invitations = make([]models.Invitation, len(joinedResults))
	for i, result := range joinedResults {
		invitations[i] = result.Invitation

		// Populate the Role relationship
		invitations[i].Role = models.Role{
			Name: result.RoleName,
		}
		invitations[i].Role.ID = invitations[i].RoleID

		// Populate the Inviter relationship
		invitations[i].Inviter = models.User{
			Username: result.InviterUsername,
		}
		invitations[i].Inviter.ID = result.InviterID
		invitations[i].Inviter.Profile = models.Profile{
			FirstName: result.InviterFirstName,
			LastName:  result.InviterLastName,
		}
	}

	return invitations, total, http.StatusOK, nil
}

// RevokeInvitation revokes a pending invitation
func RevokeInvitation(invitationID string, orgID string) (int, error) {
	customerr := valids.NewErrorDict()

	if invitationID == "" {
		customerr.Errors["invitation_id"] = "Invitation ID is required"
		return http.StatusBadRequest, customerr
	}
	if orgID == "" {
		customerr.Errors["org_id"] = "Organization ID is required"
		return http.StatusBadRequest, customerr
	}

	err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
		var invitation models.Invitation
		if err := tx.Where("id = ? AND org_id = ?", invitationID, orgID).First(&invitation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				customerr.Errors["invitation_id"] = "Invitation not found"
				return customerr
			}
			return err
		}

		// Can only revoke pending invitations
		if invitation.Status != models.InvitationStatusPending {
			customerr.Errors["invitation"] = "Only pending invitations can be revoked"
			return customerr
		}

		// Update status to revoked
		invitation.Status = models.InvitationStatusRevoked
		if err := tx.Save(&invitation).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		loging.Logger.Error("Error revoking invitation", err)
		return http.StatusBadRequest, err
	}

	return http.StatusOK, nil
}

// generateInvitationToken generates a secure random token for invitations
func generateInvitationToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// SendInvitationEmail sends an invitation email to the invited user
func SendInvitationEmail(invitation *models.Invitation) error {
	// Get base URL from environment or use default
	baseURL := os.Getenv("APP_BASE_URL")
	if baseURL == "" {
		baseURL = "https://app.bigbucks.com" // Default base URL
	}

	// Prepare email template parameters
	params := map[string]interface{}{
		"Subject":          fmt.Sprintf("Invitation to join %s", invitation.Organization.Name),
		"OrganizationName": invitation.Organization.Name,
		"RoleName":         invitation.Role.Name,
		"InviterName":      "Team", // You might want to get the actual inviter name
		"InvitationLink":   fmt.Sprintf("%s/invitations/accept?token=%s", baseURL, invitation.Token),
		"ExpirationDate":   invitation.ExpiresAt.Format("January 2, 2006 at 3:04 PM"),
	}

	// Send the invitation email
	err := emailservice.SendEmail(invitation.Email, "./templates/invitation.html", params)
	if err != nil {
		loging.Logger.Errorf("Failed to send invitation email to %s: %v", invitation.ID, err)
		// Fall back to logging the invitation details for debugging
		loging.Logger.Debugf("Invitation details - Email: %s, Organization: %s, Role: %s, Token: %s",
			invitation.Email, invitation.Organization.Name, invitation.Role.Name, invitation.Token)
		return fmt.Errorf("failed to send invitation email: %w", err)
	}

	loging.Logger.Debugf("Successfully sent invitation email to %s for organization %s with role %s",
		invitation.Email, invitation.Organization.Name, invitation.Role.Name)

	return nil
}

// ResendInvitation resends an existing invitation or creates a new one if expired
func ResendInvitation(invitationID string, orgID string, inviterID string) (*models.Invitation, int, error) {
	customerr := valids.NewErrorDict()

	if invitationID == "" {
		customerr.Errors["invitation_id"] = "Invitation ID is required"
		return nil, http.StatusBadRequest, customerr
	}
	if orgID == "" {
		customerr.Errors["org_id"] = "Organization ID is required"
		return nil, http.StatusBadRequest, customerr
	}
	if inviterID == "" {
		customerr.Errors["inviter_id"] = "Inviter ID is required"
		return nil, http.StatusBadRequest, customerr
	}

	var invitation *models.Invitation
	err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
		// Find the existing invitation
		var existingInvitation models.Invitation
		if err := tx.Preload("Organization").Preload("Role").Where("id = ? AND org_id = ?", invitationID, orgID).First(&existingInvitation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				customerr.Errors["invitation_id"] = "Invitation not found"
				return customerr
			}
			return err
		}

		// Check if the inviter has permission in the organization
		var userOrgRole models.UserOrgRole
		if err := tx.Where("user_id = ? AND org_id = ?", inviterID, orgID).First(&userOrgRole).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				customerr.Errors["inviter_id"] = "Inviter does not belong to the organization"
				return customerr
			}
			return err
		}

		// Check if user is already a member of the organization
		var existingUser models.User
		if err := tx.Where("username = ?", existingInvitation.Email).First(&existingUser).Error; err == nil {
			var existingUserOrgRole models.UserOrgRole
			if err := tx.Where("user_id = ? AND org_id = ?", existingUser.ID, orgID).First(&existingUserOrgRole).Error; err == nil {
				customerr.Errors["email"] = "User is already a member of this organization"
				return customerr
			}
		}

		if existingInvitation.Status == models.InvitationStatusAccepted {
			customerr.Errors["invitation"] = "Cannot resend an accepted invitation"
			return customerr
		}

		if existingInvitation.Status == models.InvitationStatusRevoked {
			customerr.Errors["invitation"] = "Cannot resend a revoked invitation"
			return customerr
		}

		// If invitation is expired or still pending, create a new one
		if existingInvitation.Status == models.InvitationStatusExpired || existingInvitation.IsExpired() {
			// Mark old invitation as expired
			existingInvitation.Status = models.InvitationStatusExpired
			if err := tx.Save(&existingInvitation).Error; err != nil {
				return err
			}

			// Generate new token
			token, err := generateInvitationToken()
			if err != nil {
				return err
			}

			// Create new invitation
			invitation = &models.Invitation{
				Email:     existingInvitation.Email,
				InviterID: inviterID,
				OrgID:     orgID,
				RoleID:    existingInvitation.RoleID,
				Status:    models.InvitationStatusPending,
				Token:     token,
				ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
			}

			if err := tx.Create(invitation).Error; err != nil {
				return err
			}

			// Preload relationships
			if err := tx.Preload("Inviter").Preload("Organization").Preload("Role").First(invitation, "id = ?", invitation.ID).Error; err != nil {
				return err
			}
		} else {
			// If invitation is still pending and not expired, just resend the same invitation
			invitation = &existingInvitation
		}

		return nil
	})

	if err != nil {
		loging.Logger.Error("Error resending invitation", err)
		if _, isCustomErr := err.(*valids.ValidationErrors); isCustomErr {
			return nil, http.StatusBadRequest, err
		}
		return nil, http.StatusInternalServerError, err
	}

	// Send invitation email
	go func() {
		if err := SendInvitationEmail(invitation); err != nil {
			loging.Logger.Error("Failed to send invitation email", err)
		}
	}()

	return invitation, http.StatusOK, nil
}

// ExpireOldInvitations marks old pending invitations as expired
func ExpireOldInvitations() error {
	result := models.Dbcon.Model(&models.Invitation{}).
		Where("status = ? AND expires_at < ?", models.InvitationStatusPending, time.Now()).
		Update("status", models.InvitationStatusExpired)

	if result.Error != nil {
		loging.Logger.Error("Error expiring old invitations", result.Error)
		return result.Error
	}

	if result.RowsAffected > 0 {
		loging.Logger.Infof("Expired %d old invitations", result.RowsAffected)
	}

	return nil
}
