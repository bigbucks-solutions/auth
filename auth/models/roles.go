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
	// CreatedAt time.Time
	// DeletedAt gorm.DeletedAt
}

// func (usr *UserOrgRole) BeforeCreate(db *gorm.DB) error {
// 	// ...
// 	fmt.Prinln(usr)
// 	return ""
// }
