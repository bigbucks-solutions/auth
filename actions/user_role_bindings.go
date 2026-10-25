package actions

import (
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	valids "bigbucks/solution/auth/validations"
	"context"
	"errors"
	"net/http"

	"gorm.io/gorm"
)

// BindUserRole binds a role to a user for a specific organization
func BindUserRole(userID string, roleID string, orgID string) (int, error) {
	var role models.Role
	var userOrgRole models.UserOrgRole

	customerr := valids.NewErrorDict()

	err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
		// Find the user
		var user models.User
		if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		// Find the role
		if err := tx.First(&role, "id = ? AND org_id = ?", roleID, orgID).Error; err != nil {
			return err
		}
		// Check if the user already has the role
		if err := tx.Where("user_id = ? AND role_id = ? AND org_id = ?", user.ID, role.ID, orgID).First(&userOrgRole).Error; err == nil {
			return errors.New("user already has the role")
		}

		// Binding a role to someone outside the organization admits a new member
		// and therefore consumes a licence. Existing members are unaffected.
		if err := requireMembershipLicense(context.Background(), tx, orgID, user.ID); err != nil {
			return err
		}

		// Create user-org-role binding
		userOrgRole = models.UserOrgRole{
			UserID: user.ID,
			RoleID: role.ID,
			OrgID:  orgID,
		}

		if err := tx.Create(&userOrgRole).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		loging.Logger.Error(err)
		customerr.Errors["Error"] = err.Error()
		if status, denied := subscriptionStatus(err); denied {
			return status, customerr
		}
		return http.StatusConflict, customerr
	}

	return 0, nil
}

// UnBindUserRole binds a role to a user for a specific organization
func UnBindUserRole(userID string, roleID string, orgID string) (int, error) {
	var role models.Role
	customerr := valids.NewErrorDict()
	err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
		// Find the user
		var user models.User
		if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}

		// Find the role - use FOR UPDATE to lock the row during transaction
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&role, "id = ? AND org_id = ?", roleID, orgID).Error; err != nil {
			return err
		}

		// Lock ALL UserOrgRole rows for this ORGANIZATION to prevent concurrent modifications
		// This ensures our count check is accurate across all users in the organization,
		// preventing concurrent removals from different users that could leave an org without admins
		if err := tx.Exec("SELECT 1 FROM user_org_roles WHERE org_id = ? FOR UPDATE", orgID).Error; err != nil {
			return err
		}
		// Check if the user is an admin and if removing this role would leave the org without admins
		if role.Name == "Super Admin" || role.Name == "Admin" || role.Name == "Owner" {
			// Count all admin roles in the organization EXCLUDING the one we're about to remove
			var remainingAdminCount int64
			err := tx.Model(&models.UserOrgRole{}).
				Joins("JOIN roles ON user_org_roles.role_id = roles.id").
				Where("roles.name IN ? AND user_org_roles.org_id = ? AND NOT (user_org_roles.user_id = ? AND user_org_roles.role_id = ?)",
					[]string{"Super Admin", "Admin", "Owner"},
					orgID,
					user.ID,
					role.ID).
				Count(&remainingAdminCount).Error
			if err != nil {
				return err
			}

			if remainingAdminCount == 0 {
				return errors.New("cannot remove the last admin role from the organization")
			}
		}

		// Delete the user-org-role binding with explicit WHERE conditions
		result := tx.Where("user_id = ? AND role_id = ? AND org_id = ?", user.ID, role.ID, orgID).Delete(&models.UserOrgRole{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("no user-role binding found to delete")
		}

		return nil
	})
	if err != nil {
		loging.Logger.Error(err)
		customerr.Errors["Error"] = err.Error()
		return http.StatusConflict, customerr
	}
	return 0, nil

}
