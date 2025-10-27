package actions

import (
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/permission_cache"
	valids "bigbucks/solution/auth/validations"
	"context"
	"net/http"

	"gorm.io/gorm"
)

type Organization struct {
	Name               string `json:"name" validate:"required,min=4"`
	ContactEmail       string `json:"email" validate:"required,email"`
	ContactNumber      string `json:"phone" validate:"omitempty,valid_phone,min=5"`
	Address            string `json:"address"`
	Country            string `json:"country"`
	WebsiteURL         string `json:"website" validate:"omitempty,url"`
	CompanyDescription string `json:"description" validate:"omitempty,max=500"`
}

// CreateOrganization : Create new Organization with a super user attached
func CreateOrganisationFromAuthenticatedUser(org *Organization, userName string, perm_cache *permission_cache.PermissionCache, ctx context.Context) (int, error) {
	err := valids.Validate.Struct(org)
	if err != nil {
		return http.StatusBadRequest, err
	}
	var orgModel models.Organization
	orgModel.Name = org.Name
	orgModel.Address = org.Address
	orgModel.ContactEmail = org.ContactEmail
	orgModel.ContactNumber = org.ContactNumber
	orgModel.Country = org.Country
	orgModel.WebsiteURL = org.WebsiteURL
	orgModel.CompanyDescription = org.CompanyDescription
	var SuperAdminRole models.Role
	err = models.Dbcon.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Users").Create(&orgModel).Error; err != nil {
			return err
		}

		// Create Admin role per organization
		SuperAdminRole = models.Role{
			Name:         "Admin",
			IsSystemRole: true,
			OrgID:        orgModel.ID,
		}
		if err := tx.Create(&SuperAdminRole).Error; err != nil {
			return err
		}
		// Link user to organization with super admin role
		var userID string
		err := tx.Model(&models.User{}).Where("username = ?", userName).Select("id").Take(&userID).Error
		if err != nil {
			return err
		}

		if err := tx.Create(&models.UserOrgRole{OrgID: orgModel.ID,
			UserID: userID,
			RoleID: SuperAdminRole.ID}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return http.StatusConflict, err
	}
	err = AssignSystemPermissionToRole(SuperAdminRole.ID, orgModel.ID, "session", "all", "write", false, perm_cache, ctx)
	if err != nil {
		return http.StatusConflict, err
	}
	err = AssignSystemPermissionToRole(SuperAdminRole.ID, orgModel.ID, "user", "all", "write", false, perm_cache, ctx)
	if err != nil {
		return http.StatusConflict, err
	}
	err = AssignSystemPermissionToRole(SuperAdminRole.ID, orgModel.ID, "role", "all", "write", false, perm_cache, ctx)
	if err != nil {
		return http.StatusConflict, err
	}
	err = AssignSystemPermissionToRole(SuperAdminRole.ID, orgModel.ID, "masterdata", "all", "write", false, perm_cache, ctx)
	if err != nil {
		return http.StatusConflict, err
	}

	return 0, nil
}
