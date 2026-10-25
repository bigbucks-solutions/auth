package actions

import (
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	valids "bigbucks/solution/auth/validations"
	"net/http"
	"strings"
)

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
