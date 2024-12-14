package actions

import "bigbucks/solution/auth/models"

// ListRoles returns paginated list of roles with user count and filtering
func ListRoles(page, pageSize int, roleName string, orgID int) ([]struct {
	Name        string
	Description string
	UserCount   int64
}, int64, error) {
	var roles []struct {
		Name        string
		Description string
		UserCount   int64
	}
	var total int64

	offset := (page - 1) * pageSize
	query := models.Dbcon.Model(&models.Role{})

	// Apply filters if provided
	if roleName != "" {
		query = query.Where("LOWER(name) LIKE LOWER(?)", "%"+roleName+"%")
	}
	if orgID > 0 {
		query = query.Where("roles.org_id = ?", orgID)
	}

	// Get total count with filters
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get roles with user count
	err := query.
		Select("roles.name, roles.description, COUNT(DISTINCT user_org_roles.user_id) as user_count").
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
