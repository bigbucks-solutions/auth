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
	"bigbucks/solution/auth/constants"
	"bigbucks/solution/auth/models"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	description, resource, scope, action string
)

// createPermissionCmd represents the createPermission command
var createPermissionCmd = &cobra.Command{
	Use:   "create-permission",
	Short: "Creates a permission object to database",
	Long: `Create a permission object which can be binded to role,
This binded permissions are checked against role during authorizations
For example:
	auth create-permission ACCNT_ALL --description "all account permission" --resource "accounts"
`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		_, err := actions.CreatePermission(&models.Permission{Description: description,
			Resource: resource, Scope: constants.Scope(scope), Action: constants.Action(action)})
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println("createPermission called")
	},
}

func init() {
	rootCmd.AddCommand(createPermissionCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// createPermissionCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	createPermissionCmd.Flags().StringVarP(&description, "description", "d", "", "description for permission")
	createPermissionCmd.Flags().StringVarP(&resource, "resource", "r", "", "resource affected")
	createPermissionCmd.Flags().StringVarP(&scope, "scope", "s", "", "scope of resource")
	createPermissionCmd.Flags().StringVarP(&action, "action", "a", "", "action on resource[write, read, delete, update]")

}
