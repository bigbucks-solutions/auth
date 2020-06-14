package models

import (
	"fmt"
	"net/http"
	"strings"

	valids "bigbucks/solution/auth/validations"

	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

// 23280666501018 amz test code
// Organization model
type Organization struct {
	gorm.Model
	Name          string `gorm:"not null" validate:"required,min=4"`
	Address       string
	ContactEmail  string               `validate:"required,email"`
	ContactNumber string               `validate:"omitempty,min=8,numeric"`
	Branches      []OrganizationBranch `gorm:"foreignkey:ParentOrg;"`
	Users         []*User              `gorm:"many2many:org_users;" validate:"required,len=1,dive"`
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

func CreateOrganization(org *Organization) (int, error) {
	// valids.Validate.RegisterStructValidation(UserStructLevelValidation, Organization{})
	// Dbcon.fi
	err := valids.Validate.Struct(org)
	customerr := valids.NewErrorDict()

	if err != nil {
		customerr.GetErrorTranslations(err)
		return http.StatusBadRequest, customerr
	}
	var SuperUserRole Role
	Dbcon.Find(&SuperUserRole, Role{Name: "SuperUser"})
	// org.Users[0].Roles = []*Role{&SuperUserRole}
	// org.Users[0].Roles =
	if err := Dbcon.Create(org).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.username") {
			customerr.Errors["username"] = "Username already exists"
		}
		fmt.Println(err)
		return http.StatusConflict, customerr
	}
	if err := Dbcon.Create(&UserOrgRole{OrgID: int(org.ID),
		UserID: int(org.Users[0].ID),
		RoleID: int(SuperUserRole.ID)}).Error; err != nil {
		return 400, nil
	}
	return 0, nil
}

func UserStructLevelValidation(sl validator.StructLevel) {

	org := sl.Current().Interface().(Organization)
	validatorTmp := sl.Validator()
	valids.Validate.RegisterStructValidation(nil, Organization{})
	err := validatorTmp.StructExcept(org, "Users")
	fmt.Println("Custom Validation", err)
	// if len(user.FirstName) == 0 && len(user.LastName) == 0 {
	// 	sl.ReportError(user.FirstName, "fname", "FirstName", "fnameorlname", "")
	// 	sl.ReportError(user.LastName, "lname", "LastName", "fnameorlname", "")
	// }

	// plus can do more, even with different tag than "fnameorlname"
}
