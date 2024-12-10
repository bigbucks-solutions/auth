package models

import (
	"bigbucks/solution/auth/models/types"
	"database/sql/driver"
	"encoding/json"

	"gorm.io/gorm"
)

type Scope string

const (
	Own Scope = "own"
	Org Scope = "org"
	All Scope = "all"
)

func (p *Scope) Scan(value interface{}) error {
	*p = Scope(value.(string))
	return nil
}

func (p Scope) Value() (driver.Value, error) {
	return string(p), nil
}

// Role model
type Role struct {
	gorm.Model  `swaggerignore:"true"`
	OrgID       int    `gorm:"index:,unique,composite:name_org"`
	Name        string `gorm:"index:,unique,composite:name_org;not null" validate:"required,min=4"`
	Description string
	Permissions []*Permission `gorm:"many2many:role_permissions;"`
	Users       []*User       `gorm:"many2many:UserOrgRole;JoinForeignKey:RoleID;JoinReferences:UserID;"`
}

// Permission model
type Permission struct {
	gorm.Model `swaggerignore:"true"`
	Resource   string `gorm:"not null;index:idx_resource;index:idx_res_scope_action,unique,priority:1" validate:"alphanum_,min=3"`
	Scope      Scope  `gorm:"not null;index:idx_res_scope_action,unique,priority:2" validate:"required,oneof=own org all,alphanum_,min=3"`
	Action     string `gorm:"not null;index:idx_res_scope_action,unique,priority:3;check:action IN ('read', 'write', 'delete', 'update')" validate:"required,oneof=read write delete update,alphanum_,min=3"`

	Description string
	Roles       []*Role `gorm:"many2many:role_permissions;"`
}

// UserOrgRole many to many relation table
type UserOrgRole struct {
	gorm.Model `swaggerignore:"true"`
	OrgID      int `gorm:"not null"`
	UserID     int `gorm:"not null"`
	RoleID     int `gorm:"not null"`
}

// MarshalJSON Json Dump override method
func (role Role) MarshalJSON() ([]byte, error) {
	var tmp = &types.Role{}
	tmp.Name = role.Name
	return json.Marshal(&tmp)
}

// func (usr *UserOrgRole) BeforeCreate(db *gorm.DB) error {
// 	// ...
// 	fmt.Prinln(usr)
// 	return ""
// }

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
	query := Dbcon.Model(&Role{})

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
