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

// ListRolePermission: Returns all the permissions bound to the role
func ListRolePermission(roleKey string, orgID string, ctx context.Context) ([]types.ListRolePermission, int, error) {
	var role models.Role
	var permissions []models.Permission
	customerr := valids.NewErrorDict()
	if err := models.Dbcon.First(&role, "id = ? and org_id = ?", roleKey, orgID).Error; err != nil {
		loging.Logger.Error(err)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			customerr.Errors["role"] = "Role not found"
			return nil, http.StatusNotFound, customerr
		}
		return nil, http.StatusInternalServerError, err
	}
	if err := models.Dbcon.Model(&role).Association("Permissions").Find(&permissions); err != nil {
		loging.Logger.Error(err)
		return nil, http.StatusInternalServerError, err
	}
	var rolePermissions []types.ListRolePermission
	for _, perm := range permissions {
		rolePermissions = append(rolePermissions, types.ListRolePermission{
			Resource: perm.Resource,
			Scope:    string(perm.Scope),
			Action:   string(perm.Action),
		})
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
		tx.Where(&models.Permission{Resource: resource, Scope: constants.Scope(scope), Action: constants.Action(action)}).FirstOrCreate(&perm)

		if err := tx.Model(&role).Association("Permissions").Append(&perm); err != nil {
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
		tx.Where(&models.Permission{Resource: resource, Scope: constants.Scope(scope), Action: constants.Action(action)}).First(&perm)
		if err := tx.Model(&role).Association("Permissions").Delete(&perm); err != nil {
			return err
		}
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
		// Find the role
		if err := tx.First(&role, "id = ? AND org_id = ?", roleID, orgID).Error; err != nil {
			return err
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
