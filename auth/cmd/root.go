/*
Copyright © 2020 jamsheed

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
	router "bigbucks/solution/auth/http"
	models "bigbucks/solution/auth/models"
	settings "bigbucks/solution/auth/settings"
	valids "bigbucks/solution/auth/validations"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	cfgFile string
	port    string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "auth",
	Short: "A brief description of your application",

	Run: func(cmd *cobra.Command, args []string) {
		var err error
		// models.Dbcon, err = gorm.Open(sqlite.Open("test.db"), &gorm.Config{})

		// defer models.Dbcon.Close()
		server := &settings.Settings{}
		models.Migrate()
		err = viper.Unmarshal(&server)
		var listener net.Listener
		adr := "127.0.0.1:" + server.Port
		listener, err = net.Listen("tcp", adr)
		if err != nil {
			log.Fatal("error")
		}
		handler, err := router.NewHandler(server)

		log.Println("Listening on", listener.Addr().String())
		if err := http.Serve(listener, handler); err != nil {
			log.Fatal(err)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	dsn := "user=bigbucks password=bigbucks DB.name=bigbucks port=5432 sslmode=disable"
	var err error
	models.Dbcon, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	// Automigrate GORM models
	// models.Dbcon.Config.Logger = models.Dbcon.Config.Logger.LogMode(logger.Error)

	err = models.Dbcon.SetupJoinTable(&models.Organization{}, "Users", &models.UserOrgRole{})
	err = models.Dbcon.SetupJoinTable(&models.User{}, "Roles", &models.UserOrgRole{})
	fmt.Println(err)
	// models.Dbcon.Config.
	// models.Dbcon.Logger.LogMode(logger.Error)
	if err != nil {
		panic("failed to connect database")
	}
	valids.InitializeValidations()
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.auth.yaml)")
	rootCmd.Flags().StringVarP(&port, "port", "p", "", "port to listen on")
	viper.BindPFlag("port", rootCmd.Flags().Lookup("port"))
	cobra.OnInitialize(initConfig)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".auth" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigName("config")

	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
