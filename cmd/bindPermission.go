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
		// var usr models.User
		// models.Dbcon.Preload("Profile").Find(&usr, &models.User{Username: "jamsheed.on@gmail.com"})
		// fmt.Println(usr.Profile.Email)
		// tok, _ := usr.GenerateResetToken()
		// fmt.Println(tok)
		_, err := models.BindPermission(perm_key, role_key)
		return err
	},
}

func init() {
	roleCmd.AddCommand(bindPermissionCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// bindPermissionCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:

	bindPermissionCmd.Flags().StringVarP(&role_key, "rolename", "r", "", "Role name to select")
	bindPermissionCmd.Flags().StringVarP(&perm_key, "perm", "p", "", "Permission code to bind")
	bindPermissionCmd.MarkFlagRequired("rolename")
	bindPermissionCmd.MarkFlagRequired("perm")
}
