package models

import "bigbucks/solution/auth/constants"

const (
	SuperOrganization = "00000000000000000000000000"
)

// Organization : GORM model for organization/company (MAIN)
type Organization struct {
	constants.BaseModel `json:"-" gorm:"embedded"`
	Name                string `gorm:"not null" validate:"required,min=4"`
	Address             string
	ContactEmail        string `validate:"required,email"`
	ContactNumber       string `validate:"omitempty,min=5"`
	Country             string
	WebsiteURL          string  `validate:"omitempty,url"`
	CompanyDescription  string  `gorm:"type:text" validate:"omitempty,max=500"`
	Users               []*User `gorm:"many2many:UserOrgRole;JoinForeignKey:OrgID;JoinReferences:UserID;" validate:"required,len=1,dive"`
}

// GetOrganization : Get Organization Detail with primary key
func GetOrganization(OrgID int) (Organization, int, error) {
	var org Organization
	Dbcon.Preload("Users").Preload("Users.Roles").First(&org, OrgID)
	return org, 0, nil
}
