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
	"bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/settings"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var (
	role_key string
	org_id   string
)

// bindPermissionCmd represents the bindPermission command
var bindPermissionCmd = &cobra.Command{
	Use:   "bind-permission",
	Short: "Bind the specified permission to the role",
	Long: `Bind the permission to the role
For example:
	auth role bind-permission ROLE_NAME PERM_NAME`,
	RunE: func(cmd *cobra.Command, args []string) error {
		orgID, err := strconv.Atoi(org_id)
		if err != nil {
			return fmt.Errorf("invalid org_id: %v", err)
		}
		_, err = actions.BindPermission(resource, scope, action, role_key, orgID, permission_cache.NewPermissionCache(settings.Current))
		return err
	},
}

func init() {
	roleCmd.AddCommand(bindPermissionCmd)
	bindPermissionCmd.Flags().StringVarP(&role_key, "rolename", "", "", "Role name to select")
	bindPermissionCmd.Flags().StringVarP(&resource, "resource", "r", "", "Permission resource to bind")
	bindPermissionCmd.Flags().StringVarP(&scope, "scope", "s", "", "Permission scope bind")
	bindPermissionCmd.Flags().StringVarP(&action, "action", "a", "", "Permission action bind")
	bindPermissionCmd.Flags().StringVarP(&org_id, "orgid", "o", "", "Role OrgID to bind")
	bindPermissionCmd.MarkFlagRequired("rolename")
	bindPermissionCmd.MarkFlagRequired("resource")
	bindPermissionCmd.MarkFlagRequired("scope")
	bindPermissionCmd.MarkFlagRequired("action")
	bindPermissionCmd.MarkFlagRequired("orgid")
}
