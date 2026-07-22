package models

import (
	"bigbucks/solution/auth/constants"
	"time"
)

const (
	SuperOrganization = "00000000000000000000000000"
)

// Organization : GORM model for organization/company (MAIN)
type Organization struct {
	constants.BaseModel `json:"-" gorm:"embedded"`
	Name                string `gorm:"not null" validate:"required,min=4"`
	Address             string
	City                string
	PostalCode          string
	State               string
	Country             string
	Latitude            float64
	Longitude           float64
	LogoURL             string
	TaxID               string
	ContactEmail        string  `validate:"required,email"`
	ContactNumber       string  `validate:"omitempty,min=5"`
	WebsiteURL          string  `validate:"omitempty,url"`
	CompanyDescription  string  `gorm:"type:text" validate:"omitempty,max=500"`
	Users               []*User `gorm:"many2many:UserOrgRole;JoinForeignKey:OrgID;JoinReferences:UserID;" validate:"required,len=1,dive"`
}

// OrganizationDetails is the complete organization representation returned by
// the organization details API.
type OrganizationDetails struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Address            string    `json:"address"`
	City               string    `json:"city"`
	PostalCode         string    `json:"postal_code"`
	State              string    `json:"state"`
	Country            string    `json:"country"`
	Latitude           float64   `json:"latitude"`
	Longitude          float64   `json:"longitude"`
	LogoURL            string    `json:"logo_url"`
	TaxID              string    `json:"tax_id"`
	ContactEmail       string    `json:"contact_email"`
	ContactNumber      string    `json:"contact_number"`
	WebsiteURL         string    `json:"website_url"`
	CompanyDescription string    `json:"company_description"`
	Users              []*User   `json:"users"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// GetOrganization gets an organization and its users by primary key.
func GetOrganization(orgID string) (Organization, error) {
	var org Organization
	err := Dbcon.Preload("Users").Preload("Users.Roles").First(&org, "id = ?", orgID).Error
	return org, err
}

// IsOrganizationMember reports whether the user belongs to an organization.
func IsOrganizationMember(orgID, username string) (bool, error) {
	var count int64
	err := Dbcon.Model(&UserOrgRole{}).
		Joins("JOIN users ON users.id = user_org_roles.user_id").
		Where("user_org_roles.org_id = ? AND users.username = ?", orgID, username).
		Count(&count).Error
	return count > 0, err
}

// Details converts an Organization to its complete API representation.
func (org Organization) Details() OrganizationDetails {
	return OrganizationDetails{
		ID:                 org.ID,
		Name:               org.Name,
		Address:            org.Address,
		City:               org.City,
		PostalCode:         org.PostalCode,
		State:              org.State,
		Country:            org.Country,
		Latitude:           org.Latitude,
		Longitude:          org.Longitude,
		LogoURL:            org.LogoURL,
		TaxID:              org.TaxID,
		ContactEmail:       org.ContactEmail,
		ContactNumber:      org.ContactNumber,
		WebsiteURL:         org.WebsiteURL,
		CompanyDescription: org.CompanyDescription,
		Users:              org.Users,
		CreatedAt:          org.CreatedAt,
		UpdatedAt:          org.UpdatedAt,
	}
}
