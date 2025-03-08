package actions

import (
	"bigbucks/solution/auth/actions/types"
	"bigbucks/solution/auth/models"
)

// ListRoles returns paginated list of roles with user count and filtering
func ListRoles(page, pageSize int, roleName string, orgID string) ([]types.ListRoleResponse, int64, error) {
	var roles []types.ListRoleResponse
	var total int64

	offset := (page - 1) * pageSize
	query := models.Dbcon.Model(&models.Role{})

	// Apply filters if provided
	if roleName != "" {
		query = query.Where("LOWER(name) LIKE LOWER(?)", roleName+"%")
	}
	if orgID != models.SuperOrganization {
		query = query.Where("roles.org_id = ?", orgID)
	}

	// Get total count with filters
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get roles with user count
	err := query.
		Select("roles.id as id, roles.name, roles.description, COUNT(DISTINCT user_org_roles.user_id) as user_count").
		Joins("LEFT JOIN user_org_roles ON user_org_roles.role_id = roles.id").
		Group("roles.id").
		Offset(offset).
		Limit(pageSize).
		Scan(&roles).Error

	if err != nil {
		return nil, 0, err
	}

	return roles, total, nil
}
