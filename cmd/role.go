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
	"errors"

	"github.com/spf13/cobra"
)

// roleCmd represents the role command
var roleCmd = &cobra.Command{
	Use:   "role",
	Short: "Admin actions for role",
	Long: `Role subcommand actions go here. For example:
	auth role -h`,
	// Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("Invalid command")
	},
}

func init() {
	rootCmd.AddCommand(roleCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// roleCmd.PersistentFlags().String("role", "", "Role to make updations against.")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// roleCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
