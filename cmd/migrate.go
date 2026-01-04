/*
Copyright © 2026 jamsheed

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
	"bigbucks/solution/auth/settings"
	"database/sql"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/spf13/cobra"
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long:  `Run database schema migrations using golang-migrate.`,
}

var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply all pending migrations",
	Long:  `Apply all pending database migrations.`,
	Run: func(cmd *cobra.Command, args []string) {
		m := getMigrate()
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			fmt.Printf("Error applying migrations: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Successfully applied all migrations")
	},
}

var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Rollback the last migration",
	Long:  `Rollback the last applied database migration.`,
	Run: func(cmd *cobra.Command, args []string) {
		m := getMigrate()
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			fmt.Printf("Error rolling back migration: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Successfully rolled back last migration")
	},
}

var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show migration status",
	Long:  `Show the current migration version and status.`,
	Run: func(cmd *cobra.Command, args []string) {
		m := getMigrate()
		version, dirty, err := m.Version()
		if err != nil && err != migrate.ErrNilVersion {
			fmt.Printf("Error getting migration status: %v\n", err)
			os.Exit(1)
		}
		if err == migrate.ErrNilVersion {
			fmt.Println("No migrations have been applied yet")
		} else {
			fmt.Printf("Current version: %d\n", version)
			if dirty {
				fmt.Println("Status: DIRTY (migration failed, manual intervention required)")
			} else {
				fmt.Println("Status: CLEAN")
			}
		}
	},
}

var migrateForceCmd = &cobra.Command{
	Use:   "force [version]",
	Short: "Force set migration version without running migrations",
	Long:  `Force set the migration version. Use this to fix dirty state.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var version int
		_, _ = fmt.Sscanf(args[0], "%d", &version)
		m := getMigrate()
		if err := m.Force(version); err != nil {
			fmt.Printf("Error forcing version: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully forced version to %d\n", version)
	},
}

func getMigrate() *migrate.Migrate {
	settings := settings.Current

	// Build connection string
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		settings.DBHost,
		settings.DBPort,
		settings.DBUsername,
		settings.DBPassword,
		settings.DBName,
		settings.DBSSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		fmt.Printf("Error connecting to database: %v\n", err)
		os.Exit(1)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		fmt.Printf("Error creating driver: %v\n", err)
		os.Exit(1)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://./migrations",
		"postgres",
		driver,
	)
	if err != nil {
		fmt.Printf("Error creating migrate instance: %v\n", err)
		os.Exit(1)
	}

	return m
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateDownCmd)
	migrateCmd.AddCommand(migrateStatusCmd)
	migrateCmd.AddCommand(migrateForceCmd)
}
