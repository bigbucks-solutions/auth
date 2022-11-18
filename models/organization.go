package models

import (
	"errors"
	"fmt"
	"net/http"

	"bigbucks/solution/auth/loging"
	valids "bigbucks/solution/auth/validations"

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

// OrganizationBranch model
// type OrganizationBranch struct {
// 	gorm.Model
// 	Name          string `gorm:"not null"`
// 	Address       string
// 	ContactEmail  string
// 	ContactNumber string
// 	ParentOrg     uint
// }

// GetOrganization : Get Organization Detail with primary key
func GetOrganization(OrgID int) (Organization, int, error) {
	var org Organization
	Dbcon.Preload("Users").Preload("Users.Roles").First(&org, OrgID)
	return org, 0, nil
}

// CreateOrganization : Create new Organization with a super user attached
func CreateOrganization(org *Organization) (int, error) {
	err := valids.Validate.Struct(org)
	loging.Logger.Debugln(err)
	customerr := valids.NewErrorDict()
	if err != nil {
		customerr.GetErrorTranslations(err)
		return http.StatusBadRequest, customerr
	}
	var SuperUserRole Role
	Dbcon.Find(&SuperUserRole, Role{Name: "SuperUser"})
	err = Dbcon.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Users").Create(org).Error; err != nil {
			fmt.Println(err)
			return err
		}
		for _, usr := range org.Users {
			usr.Profile = Profile{
				Email: usr.Username,
			}
		}
		if err := tx.Create(org.Users).Error; err != nil {
			if nerr := ParseError(err); errors.Is(nerr, ErrDuplicateKey) {
				customerr.Errors["username"] = "Username already exists"
				return nerr
			}
			return err
		}
		if err := tx.Create(&UserOrgRole{OrgID: int(org.ID),
			UserID: int(org.Users[0].ID),
			RoleID: int(SuperUserRole.ID)}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return http.StatusConflict, customerr
	}
	return 0, nil
}

// func UserStructLevelValidation(sl validator.StructLevel) {

// 	org := sl.Current().Interface().(Organization)
// 	validatorTmp := sl.Validator()
// 	valids.Validate.RegisterStructValidation(nil, Organization{})
// 	err := validatorTmp.StructExcept(org, "Users")
// 	fmt.Println("Custom Validation", err)
// 	// if len(user.FirstName) == 0 && len(user.LastName) == 0 {
// 	// 	sl.ReportError(user.FirstName, "fname", "FirstName", "fnameorlname", "")
// 	// 	sl.ReportError(user.LastName, "lname", "LastName", "fnameorlname", "")
// 	// }

// 	// plus can do more, even with different tag than "fnameorlname"
// }
