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
	"net/http"
	"time"

	"gorm.io/gorm"
)

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

	// Before the row is written, not after: `FirstOrCreate` below never runs the
	// `models.Permission` struct validator, so an unvetted triple would be
	// created verbatim and then quietly match nothing.
	resource, scope, action, verr := validPermissionTriple(resource, scope, action)
	if verr != nil {
		loging.Logger.Error(verr)
		return http.StatusBadRequest, verr
	}

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

	// Normalized to match how BindPermission spells a row, but deliberately not
	// validated: rows predating that validation may hold a wildcard or a
	// retired resource name, and unbinding is how they get cleaned up.
	resource, scope, action = lowerTriple(resource, scope, action)

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
	resource, scope, action, verr := validPermissionTriple(resource, scope, action)
	if verr != nil {
		loging.Logger.Error(verr)
		return verr
	}

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
	// Normalized but not validated, for the reason given in UnBindPermission.
	resource, scope, action = lowerTriple(resource, scope, action)

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
