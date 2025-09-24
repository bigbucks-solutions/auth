/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/settings"
	"context"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

// bindPermissionCmd represents the bindPermission command
var setupSuperOrgCmd = &cobra.Command{
	Use:   "setup-superorg",
	Short: "Initializes the super organization and a super admin user",
	Long:  `This command initializes the super organization and a super admin user.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		org := &models.Organization{Name: "Super Organization", ContactEmail: "jamshi.onnet@gmail.com"}
		org.ID = models.SuperOrganization
		var role models.Role
		err := models.Dbcon.Transaction(func(tx *gorm.DB) error {
			role = models.Role{Name: "Super Admin", OrgID: models.SuperOrganization}
			err := tx.FirstOrCreate(&role).Error
			if err != nil {
				loging.Logger.Error(err)
				return err
			}
			// Create the organization without users
			err = tx.FirstOrCreate(org).Error
			if err != nil {
				return err
			}
			user := &models.User{
				Username: "jamshi.onnet@gmail.com",
				Password: "Jamsheed",
				Profile:  models.Profile{FirstName: "Jamsheed", Email: "jamshi.onnet@gmail.com"},
			}
			err = tx.FirstOrCreate(user).Error
			if err != nil {
				return err
			}

			// Create the user-org-role relationship manually
			userOrgRole := &models.UserOrgRole{
				OrgID:  models.SuperOrganization,
				UserID: user.ID,
				RoleID: role.ID,
			}

			return tx.Create(userOrgRole).Error
		})
		if err != nil {
			loging.Logger.Error(err)
		}
		_ = actions.AssignSystemPermissionToRole(role.ID, models.SuperOrganization, "session", "all", "write", false, permission_cache.NewPermissionCache(settings.Current), context.Background())
		_ = actions.AssignSystemPermissionToRole(role.ID, models.SuperOrganization, "user", "all", "write", false, permission_cache.NewPermissionCache(settings.Current), context.Background())
		_ = actions.AssignSystemPermissionToRole(role.ID, models.SuperOrganization, "role", "all", "write", false, permission_cache.NewPermissionCache(settings.Current), context.Background())
		_ = actions.AssignSystemPermissionToRole(role.ID, models.SuperOrganization, "masterdata", "all", "write", false, permission_cache.NewPermissionCache(settings.Current), context.Background())
		return err

	}}

func init() {
	rootCmd.AddCommand(setupSuperOrgCmd)

}
