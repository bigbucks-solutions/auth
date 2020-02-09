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
  "fmt"
  "os"
	"log"
	"net"
	"net/http"
  "github.com/spf13/cobra"
  router "github.com/bigbucks/solution/auth/http"
  settings "github.com/bigbucks/solution/auth/settings"
  homedir "github.com/mitchellh/go-homedir"
  "github.com/spf13/viper"
  "github.com/spf13/pflag"

)


var (
  cfgFile string
  port string
)


// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
  Use:   "auth",
  Short: "A brief description of your application",
  Long: `A longer description that spans multiple lines and likely contains
        examples and usage of using your application. For example:

        Cobra is a CLI library for Go that empowers applications.
        This application is a tool to generate the needed files
        to quickly create a Cobra application.`,
  // Uncomment the following line if your bare application
  // has an action associated with it:
  Run: func(cmd *cobra.Command, args []string) { 
      var listener net.Listener
      adr := "127.0.0.1:" + port
      listener, err := net.Listen("tcp", adr)
      if err != nil{
        log.Fatal("error")
      }
      log.Println("Listening on", adr)
      server := getRunParams(cmd.Flags())
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
  if err := rootCmd.Execute(); err != nil {
    fmt.Println(err)
    os.Exit(1)
  }
}

func init() {
  cobra.OnInitialize(initConfig)
  rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.auth.yaml)")
  rootCmd.Flags().StringVarP(&port, "port", "p", "8080", "port to listen on")
  viper.BindPFlag("port", rootCmd.Flags().Lookup("port"))
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
    viper.SetConfigName(".auth")
  }

  viper.AutomaticEnv() // read in environment variables that match

  // If a config file is found, read it in.
  if err := viper.ReadInConfig(); err == nil {
    fmt.Println("Using config file:", viper.ConfigFileUsed())
  }
}

func getRunParams(flags *pflag.FlagSet) *settings.Server {
  server := &settings.Server{}


	if val, set := getParamB(flags, "baseurl"); set {
		server.BaseURL = val
	}

	if val, set := getParamB(flags, "address"); set {
		server.Address = val
	}

	if val, set := getParamB(flags, "port"); set {
		server.Port = val
	}

	if val, set := getParamB(flags, "log"); set {
		server.Log = val
	}

	if val, set := getParamB(flags, "key"); set {
		server.TLSKey = val
	}

	if val, set := getParamB(flags, "cert"); set {
		server.TLSCert = val
	}
	return server
}

func getParamB(flags *pflag.FlagSet, key string) (string, bool) {
	value, _ := flags.GetString(key)

	if flags.Changed(key) {
		return value, true
	}
	if viper.IsSet(key) {
		return viper.GetString(key), true

	}
	return value, false
}

func getParam(flags *pflag.FlagSet, key string) string {
	val, _ := getParamB(flags, key)
	return val
}

