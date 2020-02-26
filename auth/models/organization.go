package models

import (
	// "time"

	"github.com/jinzhu/gorm"
)

// Organization model
type Organization struct {
	gorm.Model
	Name          string `gorm:"not null"`
	Address       string
	ContactEmail  string
	ContactNumber string
	Branches      []OrganizationBranch `gorm:"foreignkey:ParentOrg;"`
	Users         []*User              `gorm:"many2many:org_users;"`
}

// OrganizationBranch model
type OrganizationBranch struct {
	gorm.Model
	Name          string `gorm:"not null"`
	Address       string
	ContactEmail  string
	ContactNumber string
	ParentOrg     uint
}
