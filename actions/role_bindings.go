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
	"strconv"

	"gorm.io/gorm"
)

// CreateRole : Creates new Role
func CreateRole(role *models.Role) (int, error) {
	err := valids.Validate.Struct(role)
	loging.Logger.Debug(role, err)
	customerr := valids.NewErrorDict()
	if err != nil {
		customerr.GetErrorTranslations(err)
		return http.StatusBadRequest, customerr
	}
	loging.Logger.Info(fmt.Sprintf("Creating Role %s", role.Name))
	if err := models.Dbcon.Create(role).Error; err != nil {
		loging.Logger.Error(err)
		if nerr := models.ParseError(err); errors.Is(nerr, models.ErrDuplicateKey) {
			customerr.Errors["name"] = "Role with same name already exists"
			return http.StatusConflict, customerr
		}
		return http.StatusConflict, err
	}
	return 0, nil
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

// BindPermission : Binds the permission to the role specified
func BindPermission(resource, scope, action, roleKey string, orgID int, perm_cache *permission_cache.PermissionCache, ctx context.Context) (int, error) {
	var role models.Role
	var perm models.Permission
	customerr := valids.NewErrorDict()
	err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&role, "name = ? and org_id = ?", roleKey, orgID).Error; err != nil {
			return err
		}
		tx.Where(&models.Permission{Resource: resource, Scope: models.Scope(scope), Action: models.Action(action)}).FirstOrCreate(&perm)

		if err := tx.Model(&role).Association("Permissions").Append(&perm); err != nil {
			return err
		}

		err := perm_cache.AddRoleToPermKey(ctx, strconv.Itoa(orgID), role.Name, resource, scope, action)
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

func UnBindPermission(resource, scope, action, roleKey string, orgID int, perm_cache *permission_cache.PermissionCache, ctx context.Context) (int, error) {
	var role models.Role
	var perm models.Permission
	customerr := valids.NewErrorDict()
	err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&role, "name = ? and org_id = ?", roleKey, orgID).Error; err != nil {
			return err
		}
		tx.Where(&models.Permission{Resource: resource, Scope: models.Scope(scope), Action: models.Action(action)}).First(&perm)
		if err := tx.Model(&role).Association("Permissions").Delete(&perm); err != nil {
			return err
		}
		err := perm_cache.RemoveRoleFromPermKey(ctx, strconv.Itoa(orgID), role.Name, resource, scope, action)
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
func BindUserRole(userName string, roleKey string, orgID int) (int, error) {
	var role models.Role
	var userOrgRole models.UserOrgRole

	customerr := valids.NewErrorDict()

	err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
		// Find the user
		var user models.User
		if err := tx.Where("LOWER(username) = LOWER(?)", userName).First(&user).Error; err != nil {
			return err
		}
		// Find the role
		if err := tx.First(&role, "name = ? AND org_id = ?", roleKey, orgID).Error; err != nil {
			return err
		}

		// Create user-org-role binding
		userOrgRole = models.UserOrgRole{
			UserID: int(user.ID),
			RoleID: int(role.ID),
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
