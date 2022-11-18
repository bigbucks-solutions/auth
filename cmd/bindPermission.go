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
	"bigbucks/solution/auth/models"

	"github.com/spf13/cobra"
)

var (
	role_key, perm_key string
)

// bindPermissionCmd represents the bindPermission command
var bindPermissionCmd = &cobra.Command{
	Use:   "bind-permission",
	Short: "Bind the specified permission to the role",
	Long: `Bind the permission to the role
For example:
	auth role bind-permission ROLE_NAME PERM_NAME`,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := models.BindPermission(perm_key, role_key)
		return err
	},
}

func init() {
	roleCmd.AddCommand(bindPermissionCmd)
	bindPermissionCmd.Flags().StringVarP(&role_key, "rolename", "r", "", "Role name to select")
	bindPermissionCmd.Flags().StringVarP(&perm_key, "perm", "p", "", "Permission code to bind")
	bindPermissionCmd.MarkFlagRequired("rolename")
	bindPermissionCmd.MarkFlagRequired("perm")
}
