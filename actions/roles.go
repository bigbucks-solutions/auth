package actions

import (
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
