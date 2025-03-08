package actions

import (
	"bigbucks/solution/auth/loging"
	. "bigbucks/solution/auth/models"
	valids "bigbucks/solution/auth/validations"
	"errors"
	"fmt"
	"net/http"

	"gorm.io/gorm"
)

// CreateOrganization : Create new Organization with a super user attached
func CreateOrganization(org *Organization) (int, error) {
	err := valids.Validate.Struct(org)
	loging.Logger.Debugln(err)
	customerr := valids.NewErrorDict()
	if err != nil {
		customerr.GetErrorTranslations(err)
		return http.StatusBadRequest, customerr
	}
	var SuperUserRole Role = Role{Name: "SuperUser"}
	Dbcon.FirstOrCreate(&SuperUserRole, "name = ?", "SuperUser")
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
		if len(org.Users) > 0 {
			if err := tx.Create(org.Users).Error; err != nil {
				if nerr := ParseError(err); errors.Is(nerr, ErrDuplicateKey) {
					customerr.Errors["username"] = "Username already exists"
					return nerr
				}
				return err
			}
			if err := tx.Create(&UserOrgRole{OrgID: org.ID,
				UserID: org.Users[0].ID,
				RoleID: SuperUserRole.ID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return http.StatusConflict, customerr
	}
	return 0, nil
}
