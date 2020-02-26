package models

import (
	"github.com/jinzhu/gorm"
)

// Role model
type Role struct {
	gorm.Model
	Name        string `gorm:"not null"`
	Description string
	Permissions []*Permission
	Users       []*User `gorm:"many2many:user_roles;"`
}

// Permission model
type Permission struct {
	gorm.Model
	Code        string `gorm:"unique;not null"`
	Description string
	Resource    string
	RoleID      uint
}
