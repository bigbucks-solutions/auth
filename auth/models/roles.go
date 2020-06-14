package models

import (
	"gorm.io/gorm"
)

// Role model
type Role struct {
	gorm.Model
	Name        string `gorm:"not null"`
	Description string
	Permissions []*Permission `gorm:"many2many:role_permissions;"`
	Users       []*User       `gorm:"many2many:user_org_roles;association_jointable_foreignkey:user_id;jointable_foreignkey:role_id;"`
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
