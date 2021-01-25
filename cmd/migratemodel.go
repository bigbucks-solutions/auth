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
	models "bigbucks/solution/auth/models"
	"fmt"

	"github.com/spf13/cobra"
)

// migratemodelCmd represents the migratemodel command
var migratemodelCmd = &cobra.Command{
	Use:   "migratemodel",
	Short: "Runs the Schema migration on the database",
	Long: `This command runs the migration to reflect the latest database schema changes in GORM model defined in
	this application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Running Database Schema migration...")
		models.Migrate()
	},
}

func init() {
	rootCmd.AddCommand(migratemodelCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// migratemodelCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// migratemodelCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
