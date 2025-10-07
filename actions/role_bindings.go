package actions

import (
	"bigbucks/solution/auth/actions/types"
	"bigbucks/solution/auth/constants"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/permission_cache"
	valids "bigbucks/solution/auth/validations"
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CreateRole : Creates new Role
func CreateRole(role *models.Role) (string, int, error) {
	err := valids.Validate.Struct(role)
	loging.Logger.Debug(role, err)
	customerr := valids.NewErrorDict()
	if err != nil {
		customerr.GetErrorTranslations(err)
		return "", http.StatusBadRequest, customerr
	}
	loging.Logger.Info(fmt.Sprintf("Creating Role %s", role.Name))
	if err := models.Dbcon.Create(role).Error; err != nil {
		loging.Logger.Error(err)
		if nerr := models.ParseError(err); errors.Is(nerr, models.ErrDuplicateKey) {
			customerr.Errors["name"] = "Role with same name already exists"
			return "", http.StatusConflict, customerr
		}
		return "", http.StatusConflict, err
	}
	return role.ID, 0, nil
}

// CreateSystemRole : Creates a system-managed role with locked permissions
func CreateSystemRole(role *models.Role, systemPermissions []models.Permission) (string, int, error) {
	customerr := valids.NewErrorDict()

	err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
		// Mark as system role
		role.IsSystemRole = true

		// Create the role
		if err := tx.Create(role).Error; err != nil {
			return err
		}

		// Assign system permissions as locked
		for _, perm := range systemPermissions {
			// Ensure permission exists
			var existingPerm models.Permission
			if err := tx.Where(&models.Permission{
				Resource: perm.Resource,
				Scope:    perm.Scope,
				Action:   perm.Action,
			}).FirstOrCreate(&existingPerm, perm).Error; err != nil {
				return err
			}

			// Create locked role-permission binding
			rolePermission := models.RolePermission{
				RoleID:       role.ID,
				PermissionID: existingPerm.ID,
				IsLocked:     true,
				IsHidden:     false, // You can set this based on requirements
				AssignedBy:   "system",
				CreatedAt:    time.Now(),
			}

			if err := tx.Create(&rolePermission).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		loging.Logger.Error(err)
		if nerr := models.ParseError(err); errors.Is(nerr, models.ErrDuplicateKey) {
			customerr.Errors["name"] = "Role with same name already exists"
			return "", http.StatusConflict, customerr
		}
		return "", http.StatusConflict, err
	}

	return role.ID, 0, nil
}

// CreatePermission : Creates new Permission object
func CreatePermission(perm *models.Permission) (int, error) {
	err := valids.Validate.Struct(perm)
	loging.Logger.Debug(perm, err)
	customerr := valids.NewErrorDict()
	if err != nil {
		customerr.GetErrorTranslations(err)
		return http.StatusBadRequest, customerr
	}
	loging.Logger.Info("Creating Permission..")
	if err := models.Dbcon.Create(perm).Error; err != nil {
		loging.Logger.Error(err)
		if nerr := models.ParseError(err); errors.Is(nerr, models.ErrDuplicateKey) {
			customerr.Errors["code"] = "Permission with same code already exists"
			return http.StatusConflict, customerr
		}
		return http.StatusConflict, err
	}
	return 0, nil
}

// UpdateRole : Updates existing Role
func UpdateRole(roleID string, updatedRole *models.Role, perm_cache *permission_cache.PermissionCache, ctx context.Context) (int, error) {
	err := valids.Validate.Struct(updatedRole)
	loging.Logger.Debug("Update Body", updatedRole, err)
	customerr := valids.NewErrorDict()
	if err != nil {
		customerr.GetErrorTranslations(err)
		return http.StatusBadRequest, customerr
	}

	var role models.Role
	if err := models.Dbcon.First(&role, "id = ? and org_id = ?", roleID, updatedRole.OrgID).Error; err != nil {
		loging.Logger.Error(err)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			customerr.Errors["role"] = "Role not found"
			return http.StatusNotFound, customerr
		}
		return http.StatusInternalServerError, err
	}

	// Prevent editing system role
	if role.IsSystemRole {
		customerr.Errors["role"] = "System role cannot be edited"
		return http.StatusNotAcceptable, customerr
	}

	oldRoleName := role.Name
	// Update role in transaction
	err = models.Dbcon.Transaction(func(tx *gorm.DB) error {
		role.Name = updatedRole.Name
		role.Description = updatedRole.Description
		role.ExtraAttrs = updatedRole.ExtraAttrs

		if err := tx.Save(&role).Error; err != nil {
			return err
		}

		if err := perm_cache.UpdateRoleName(ctx, updatedRole.OrgID, oldRoleName, updatedRole.Name); err != nil {
			loging.Logger.Warn("Failed to update cache after role name change",
				zap.String("oldName", oldRoleName),
				zap.String("newName", updatedRole.Name),
				zap.Error(err))
			// Don't fail the transaction for cache errors
		}

		return nil
	})

	if err != nil {
		loging.Logger.Error(err)
		if nerr := models.ParseError(err); errors.Is(nerr, models.ErrDuplicateKey) {
			customerr.Errors["name"] = "Role with same name already exists"
			return http.StatusConflict, customerr
		}
		return http.StatusConflict, err
	}
	return 0, nil
}

// ListRolePermission: Returns all the permissions bound to the role
func ListRolePermission(roleID string, orgID string, ctx context.Context) ([]types.ListRolePermission, int, error) {
	var role models.Role
	customerr := valids.NewErrorDict()
	if err := models.Dbcon.First(&role, "id = ? and org_id = ?", roleID, orgID).Error; err != nil {
		loging.Logger.Error(err)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			customerr.Errors["role"] = "Role not found"
			return nil, http.StatusNotFound, customerr
		}
		return nil, http.StatusInternalServerError, err
	}
	var rolePermissions []types.ListRolePermission

	// Query permissions with metadata from junction table
	query := `
		SELECT p.resource, p.scope, p.action,
		       rp.is_locked, rp.is_hidden
		FROM permissions p
		JOIN role_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = ? AND (rp.is_hidden = false)
	`

	rows, err := models.Dbcon.Raw(query, roleID).Rows()
	if err != nil {
		loging.Logger.Error(err)
		return nil, http.StatusInternalServerError, err
	}
	defer rows.Close() //nolint:errcheck

	for rows.Next() {
		var perm types.ListRolePermission
		if err := rows.Scan(&perm.Resource, &perm.Scope, &perm.Action,
			&perm.IsLocked, &perm.IsHidden); err != nil {
			loging.Logger.Error(err)
			continue
		}
		rolePermissions = append(rolePermissions, perm)
	}

	return rolePermissions, 0, nil
}

// BindPermission : Binds the permission to the role specified
func BindPermission(resource, scope, action, roleID string, orgID string, perm_cache *permission_cache.PermissionCache, ctx context.Context) (int, error) {
	var role models.Role
	var perm models.Permission
	customerr := valids.NewErrorDict()
	err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&role, "id = ? and org_id = ?", roleID, orgID).Error; err != nil {
			return err
		}
		// Find or create permission
		if err := tx.Where(&models.Permission{
			Resource: resource,
			Scope:    constants.Scope(scope),
			Action:   constants.Action(action),
		}).FirstOrCreate(&perm).Error; err != nil {
			return err
		}

		// Check if binding already exists
		var existingBinding models.RolePermission
		if err := tx.Where("role_id = ? AND permission_id = ?", roleID, perm.ID).First(&existingBinding).Error; err == nil {
			return errors.New("permission already bound to role")
		}

		// Create user-assigned binding
		rolePermission := models.RolePermission{
			RoleID:       roleID,
			PermissionID: perm.ID,
			IsLocked:     false,
			IsHidden:     false,
			AssignedBy:   "user",
			CreatedAt:    time.Now(),
		}

		if err := tx.Create(&rolePermission).Error; err != nil {
			return err
		}

		err := perm_cache.AddRoleToPermKey(ctx, orgID, role.Name, resource, scope, action)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		customerr.Errors["Error"] = err.Error()
		loging.Logger.Error(err)
		return http.StatusConflict, customerr
	}
	return 0, nil
}

func UnBindPermission(resource, scope, action, roleID string, orgID string, perm_cache *permission_cache.PermissionCache, ctx context.Context) (int, error) {
	var role models.Role
	var perm models.Permission
	customerr := valids.NewErrorDict()

	err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&role, "id = ? and org_id = ?", roleID, orgID).Error; err != nil {
			return err
		}

		if err := tx.Where(&models.Permission{
			Resource: resource,
			Scope:    constants.Scope(scope),
			Action:   constants.Action(action),
		}).First(&perm).Error; err != nil {
			return err
		}

		// Check if the binding is locked
		var rolePermission models.RolePermission
		if err := tx.Where("role_id = ? AND permission_id = ?", roleID, perm.ID).First(&rolePermission).Error; err != nil {
			return errors.New("permission binding not found")
		}

		if rolePermission.IsLocked {
			return errors.New("cannot remove locked system permission")
		}

		// Delete the binding
		if err := tx.Delete(&rolePermission).Error; err != nil {
			return err
		}

		// Update cache
		err := perm_cache.RemoveRoleFromPermKey(ctx, orgID, role.Name, resource, scope, action)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		customerr.Errors["Error"] = err.Error()
		return http.StatusConflict, customerr
	}
	return 0, nil
}

// System management functions

// AssignSystemPermissionToRole : Assigns a locked system permission to any role
func AssignSystemPermissionToRole(roleID, orgID, resource, scope, action string, isHidden bool, perm_cache *permission_cache.PermissionCache, ctx context.Context) error {
	err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
		var role models.Role
		if err := tx.First(&role, "id = ? and org_id = ?", roleID, orgID).Error; err != nil {
			return err
		}

		var perm models.Permission
		if err := tx.Where(&models.Permission{
			Resource: resource,
			Scope:    constants.Scope(scope),
			Action:   constants.Action(action),
		}).FirstOrCreate(&perm).Error; err != nil {
			return err
		}

		// Create or update locked binding
		rolePermission := models.RolePermission{
			RoleID:       roleID,
			PermissionID: perm.ID,
			IsLocked:     true,
			IsHidden:     isHidden,
			AssignedBy:   "system",
			CreatedAt:    time.Now(),
		}

		err := tx.Save(&rolePermission).Error
		if err != nil {
			return err
		}

		// Update cache
		err = perm_cache.AddRoleToPermKey(ctx, orgID, role.Name, resource, scope, action)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// RemoveSystemPermissionFromRole : Removes a system permission (system use only)
func RemoveSystemPermissionFromRole(roleID, orgID, resource, scope, action string) error {
	return models.Dbcon.Transaction(func(tx *gorm.DB) error {
		var perm models.Permission
		if err := tx.Where(&models.Permission{
			Resource: resource,
			Scope:    constants.Scope(scope),
			Action:   constants.Action(action),
		}).First(&perm).Error; err != nil {
			return err
		}

		return tx.Where("role_id = ? AND permission_id = ? AND assigned_by = 'system'",
			roleID, perm.ID).Delete(&models.RolePermission{}).Error
	})
}

// BindUserRole binds a role to a user for a specific organization
func BindUserRole(userID string, roleID string, orgID string) (int, error) {
	var role models.Role
	var userOrgRole models.UserOrgRole

	customerr := valids.NewErrorDict()

	err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
		// Find the user
		var user models.User
		if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		// Find the role
		if err := tx.First(&role, "id = ? AND org_id = ?", roleID, orgID).Error; err != nil {
			return err
		}
		// Check if the user already has the role
		if err := tx.Where("user_id = ? AND role_id = ? AND org_id = ?", user.ID, role.ID, orgID).First(&userOrgRole).Error; err == nil {
			return errors.New("user already has the role")
		}

		// Create user-org-role binding
		userOrgRole = models.UserOrgRole{
			UserID: user.ID,
			RoleID: role.ID,
			OrgID:  orgID,
		}

		if err := tx.Create(&userOrgRole).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		loging.Logger.Error(err)
		customerr.Errors["Error"] = err.Error()
		return http.StatusConflict, customerr
	}

	return 0, nil
}

// UnBindUserRole binds a role to a user for a specific organization
func UnBindUserRole(userID string, roleID string, orgID string) (int, error) {
	var role models.Role
	customerr := valids.NewErrorDict()
	err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
		// Find the user
		var user models.User
		if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}

		// Find the role - use FOR UPDATE to lock the row during transaction
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&role, "id = ? AND org_id = ?", roleID, orgID).Error; err != nil {
			return err
		}

		// Lock ALL UserOrgRole rows for this ORGANIZATION to prevent concurrent modifications
		// This ensures our count check is accurate across all users in the organization,
		// preventing concurrent removals from different users that could leave an org without admins
		if err := tx.Exec("SELECT 1 FROM user_org_roles WHERE org_id = ? FOR UPDATE", orgID).Error; err != nil {
			return err
		}
		// Check if the user is an admin and if removing this role would leave the org without admins
		if role.Name == "Super Admin" || role.Name == "Admin" {
			// Count all admin roles in the organization EXCLUDING the one we're about to remove
			var remainingAdminCount int64
			err := tx.Model(&models.UserOrgRole{}).
				Joins("JOIN roles ON user_org_roles.role_id = roles.id").
				Where("roles.name IN ? AND user_org_roles.org_id = ? AND NOT (user_org_roles.user_id = ? AND user_org_roles.role_id = ?)",
					[]string{"Super Admin", "Admin"},
					orgID,
					user.ID,
					role.ID).
				Count(&remainingAdminCount).Error
			if err != nil {
				return err
			}

			if remainingAdminCount == 0 {
				return errors.New("cannot remove the last admin role from the organization")
			}
		}

		// Delete the user-org-role binding with explicit WHERE conditions
		result := tx.Where("user_id = ? AND role_id = ? AND org_id = ?", user.ID, role.ID, orgID).Delete(&models.UserOrgRole{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("no user-role binding found to delete")
		}

		return nil
	})
	if err != nil {
		loging.Logger.Error(err)
		customerr.Errors["Error"] = err.Error()
		return http.StatusConflict, customerr
	}
	return 0, nil

}

// DeleteRole : Deletes a role if it has no associated users
func DeleteRole(roleID string, orgID string, perm_cache *permission_cache.PermissionCache, ctx context.Context) (int, error) {
	customerr := valids.NewErrorDict()

	err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
		// First, check if role exists and belongs to the org
		var role models.Role
		if err := tx.Where("id = ? AND org_id = ?", roleID, orgID).First(&role).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				customerr.Errors["role_id"] = "Role not found"
				return customerr
			}
			return err
		}

		// Check if it's a system role - system roles cannot be deleted
		if role.IsSystemRole {
			customerr.Errors["role"] = "Cannot delete system role"
			return customerr
		}

		// Check if role has any associated users
		var userCount int64
		if err := tx.Model(&models.UserOrgRole{}).Where("role_id = ? AND org_id = ?", roleID, orgID).Count(&userCount).Error; err != nil {
			return err
		}

		if userCount > 0 {
			customerr.Errors["role"] = fmt.Sprintf("Cannot delete role. It is currently assigned to %d user(s). Please unassign all users from this role before deletion.", userCount)
			return customerr
		}

		// Delete all role-permission bindings first
		if err := tx.Where("role_id = ?", roleID).Delete(&models.RolePermission{}).Error; err != nil {
			return err
		}

		// Delete the role
		if err := tx.Unscoped().Delete(&role).Error; err != nil {
			return err
		}

		loging.Logger.Info(fmt.Sprintf("Successfully deleted role %s (ID: %s) from organization %s", role.Name, roleID, orgID))
		return nil
	})

	// Clear permission cache for this organization
	if perm_cache != nil {
		go func() {
			_ = perm_cache.Cleanup(ctx, orgID)
			_ = perm_cache.EnsureCacheForOrg(ctx, orgID)
		}()
	}

	if err != nil {
		if len(customerr.Errors) > 0 {
			return http.StatusBadRequest, customerr
		}
		loging.Logger.Error("Failed to delete role", zap.Error(err))
		return http.StatusInternalServerError, err
	}

	return 0, nil
}
