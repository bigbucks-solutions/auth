package actions

import (
	"bigbucks/solution/auth/actions/types"
	"bigbucks/solution/auth/constants"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	sessionstore "bigbucks/solution/auth/session_store"
	valids "bigbucks/solution/auth/validations"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ListUsersParams defines the parameters for user listing
type ListUsersParams struct {
	OrgID        string
	Page         int
	PageSize     int
	Status       *constants.UserStatus
	RoleID       *string
	SearchPrefix *string
}

// ListUsers returns a paginated list of users for an organization with optional filters
func ListUsers(params ListUsersParams) ([]models.User, int64, int, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	offset := (params.Page - 1) * params.PageSize

	var users []models.User
	var totalCount int64
	customerr := valids.NewErrorDict()

	query := models.Dbcon.Model(&models.User{}).
		Joins("INNER JOIN user_org_roles uor ON uor.user_id = users.id").
		Joins("LEFT JOIN profiles ON profiles.user_id = users.id").
		Where("uor.org_id = ?", params.OrgID).
		Preload("Profile").
		Preload("Roles", func(db *gorm.DB) *gorm.DB {
			return db.Where("org_id = ?", params.OrgID)
		}).
		// Add preload for LastLogin
		Preload("LastLogin", func(db *gorm.DB) *gorm.DB {
			return db.Order("login_at DESC").Limit(1)
		})

	// Apply status filter if provided
	if params.Status != nil {
		query = query.Where("users.status = ?", *params.Status)
	}

	// Apply role filter if provided
	if params.RoleID != nil && *params.RoleID != "" {
		query = query.Joins("INNER JOIN user_org_roles ur ON ur.user_id = users.id AND ur.role_id = ?", *params.RoleID)
	}

	// Apply search prefix filter if provided
	if params.SearchPrefix != nil && *params.SearchPrefix != "" {
		searchTerm := strings.ToLower(*params.SearchPrefix) + "%"
		query = query.Where(
			"LOWER(users.username) LIKE ? OR LOWER(profiles.email) LIKE ? OR "+
				"LOWER(profiles.first_name) LIKE ? OR LOWER(profiles.last_name) LIKE ?",
			searchTerm, searchTerm, searchTerm, searchTerm)
	}

	// Count total matching records
	if err := query.Count(&totalCount).Error; err != nil {
		customerr.Errors["Database"] = "Error counting users: " + err.Error()
		return nil, 0, http.StatusInternalServerError, customerr
	}

	// Get paginated results
	if err := query.Distinct("users.*").
		Limit(params.PageSize).
		Offset(offset).
		Order("users.created_at DESC").
		Find(&users).Error; err != nil {
		customerr.Errors["Database"] = "Error fetching users: " + err.Error()
		loging.Logger.Error("Error fetching users", err)
		return nil, 0, http.StatusInternalServerError, customerr
	}

	return users, totalCount, 0, nil
}

// UserListResponse represents the response for user listing
type UserListResponse struct {
	Users      []types.ListUserResponse `json:"users"`
	TotalCount int64                    `json:"total"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"page_size"`
}

// ListUsersForOrg is a controller-friendly wrapper around ListUsers
func ListUsersForOrg(orgID string, page, pageSize int, status *constants.UserStatus, roleID, searchPrefix *string, sessionStore *sessionstore.SessionStore) (*UserListResponse, int, error) {
	params := ListUsersParams{
		OrgID:        orgID,
		Page:         page,
		PageSize:     pageSize,
		Status:       status,
		RoleID:       roleID,
		SearchPrefix: searchPrefix,
	}

	users, totalCount, code, err := ListUsers(params)
	if err != nil {
		return nil, code, err
	}

	// Convert users to response format
	userResponses := make([]types.ListUserResponse, len(users))
	for i, user := range users {
		// Extract role names
		roleNames := make([]types.RoleWithId, len(user.Roles))
		for j, role := range user.Roles {
			roleNames[j].Name = role.Name
			roleNames[j].ID = role.ID
		}
		sess_count, _ := sessionStore.GetUserSessionCount(user.ID)
		// Map user data to response format
		userResponses[i] = types.ListUserResponse{
			ID:             user.ID,
			Username:       user.Username,
			LastLogin:      user.LastLogin.LoginAt.UTC().Format(time.RFC3339),
			ActiveSessions: sess_count,
			Roles:          roleNames,
			Status:         user.Status,
			Firstname:      user.Profile.FirstName,
			Lastname:       user.Profile.LastName,
			Email:          user.Profile.Email,
		}
	}

	return &UserListResponse{
		Users:      userResponses,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
	}, 0, nil
}
