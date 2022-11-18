package models

import (
	"bigbucks/solution/auth/models/types"
	valids "bigbucks/solution/auth/validations"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"gorm.io/gorm"
)

// Role model
type Role struct {
	gorm.Model
	Name        string `gorm:"unique;not null" validate:"required,min=4"`
	Description string
	Permissions []*Permission `gorm:"many2many:role_permissions;"`
	Users       []*User       `gorm:"many2many:UserOrgRole;JoinForeignKey:RoleID;JoinReferences:UserID;"`
}

// Permission model
type Permission struct {
	gorm.Model
	Code        string `gorm:"unique;not null"`
	Description string
	Resource    string
	Roles       []*Role `gorm:"many2many:role_permissions;"`
}

// UserOrgRole many to many relation table
type UserOrgRole struct {
	gorm.Model
	OrgID  int `gorm:"not null"`
	UserID int `gorm:"not null"`
	RoleID int `gorm:"not null"`
}

// MarshalJSON Json Dump override method
func (role Role) MarshalJSON() ([]byte, error) {
	var tmp = &types.Role{}
	tmp.Name = role.Name
	return json.Marshal(&tmp)
}

// CreateRole : Creates new Role
func CreateRole(role *Role) (int, error) {
	err := valids.Validate.Struct(role)
	fmt.Println(role, err)
	customerr := valids.NewErrorDict()
	if err != nil {
		customerr.GetErrorTranslations(err)
		return http.StatusBadRequest, customerr
	}
	fmt.Println("creating Role..")
	if err := Dbcon.Create(role).Error; err != nil {
		fmt.Println(err)
		if nerr := ParseError(err); errors.Is(nerr, ErrDuplicateKey) {
			customerr.Errors["name"] = "Role with same name already exists"
			return http.StatusConflict, customerr
		}
		return http.StatusConflict, err
	}
	return 0, nil
}

// CreatePermission : Creates new Permission object
func CreatePermission(perm *Permission) (int, error) {
	err := valids.Validate.Struct(perm)
	fmt.Println(perm, err)
	customerr := valids.NewErrorDict()
	if err != nil {
		customerr.GetErrorTranslations(err)
		return http.StatusBadRequest, customerr
	}
	fmt.Println("creating Permission..")
	if err := Dbcon.Create(perm).Error; err != nil {
		fmt.Println(err)
		if nerr := ParseError(err); errors.Is(nerr, ErrDuplicateKey) {
			customerr.Errors["code"] = "Permsiion with same code already exists"
			return http.StatusConflict, customerr
		}
		return http.StatusConflict, err
	}
	return 0, nil
}

// BindPermission : Binds the permission to the role specified
func BindPermission(permKey, roleKey string) (int, error) {
	var role Role
	var perm Permission
	customerr := valids.NewErrorDict()
	err := Dbcon.First(&role, "name = ?", roleKey).Error
	if err != nil {
		// if errors.Is(err, gorm.ErrRecordNotFound) {
		customerr.Errors["Role"] = err.Error()
		// }
		return http.StatusConflict, customerr
	}
	err = Dbcon.First(&perm, "code = ?", permKey).Error
	if err != nil {
		customerr.Errors["Permission"] = err.Error()
		return http.StatusConflict, customerr
	}
	err = Dbcon.Model(&role).Association("Permissions").Append(&perm)
	if err != nil {
		customerr.Errors["Error"] = err.Error()
		return http.StatusConflict, customerr
	}
	return 0, nil
}

// func (usr *UserOrgRole) BeforeCreate(db *gorm.DB) error {
// 	// ...
// 	fmt.Prinln(usr)
// 	return ""
// }
