package models

import (
	"gorm.io/gorm"
)

// Organization : GORM model for organization/company (MAIN)
type Organization struct {
	gorm.Model    `json:"-"`
	Name          string `gorm:"not null" validate:"required,min=4"`
	Address       string
	ContactEmail  string `validate:"required,email"`
	ContactNumber string `validate:"omitempty,min=8,numeric"`
	// Branches      []OrganizationBranch `gorm:"foreignkey:ParentOrg;"`
	Users []*User `gorm:"many2many:UserOrgRole;JoinForeignKey:OrgID;JoinReferences:UserID;" validate:"required,len=1,dive"`
}

// GetOrganization : Get Organization Detail with primary key
func GetOrganization(OrgID int) (Organization, int, error) {
	var org Organization
	Dbcon.Preload("Users").Preload("Users.Roles").First(&org, OrgID)
	return org, 0, nil
}
