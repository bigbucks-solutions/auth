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
func BindPermission(resource, scope, action, roleKey string, orgID int, perm_cache *permission_cache.PermissionCache) (int, error) {
	var role models.Role
	var perm models.Permission
	customerr := valids.NewErrorDict()
	err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&role, "name = ? and org_id = ?", roleKey, orgID).Error; err != nil {
			return err
		}

		if err := tx.Where("LOWER(resource) = LOWER(?) AND LOWER(scope) = LOWER(?) AND LOWER(action) = LOWER(?)",
			resource, scope, action).Find(&perm).Error; err != nil {
			return err
		}

		if err := tx.Model(&role).Association("Permissions").Append(&perm); err != nil {
			return err
		}

		err := perm_cache.AddRoleToPermKey(context.Background(), strconv.Itoa(orgID), role.Name, resource, scope, action)
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
