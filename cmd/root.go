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
	grpc_auth "bigbucks/solution/auth/grpc-auth"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/permission_cache"
	router "bigbucks/solution/auth/rest-api"
	"context"
	"os/signal"
	"syscall"
	"time"

	settings "bigbucks/solution/auth/settings"
	valids "bigbucks/solution/auth/validations"
	"fmt"
	"net"
	"net/http"
	"os"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	cfgFile    string
	port       string
	httpServer *http.Server
	grpcServer *grpc.Server
	ctx        context.Context
	cancel     context.CancelFunc
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "auth",
	Short: "A brief description of your application",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		var err error
		settings.Current = &settings.Settings{}

		err = viper.Unmarshal(&settings.Current)
		if err != nil {
			loging.Logger.Fatalln(err)
		}
		settings.Current.LoadKeys()

		// dsn := "user=bigbucks password=bigbucks DB.name=bigbucks port=5432 host=localhost sslmode=disable"
		dsn := fmt.Sprintf("host=%s user=%s password=%s DB.name=%s port=%s sslmode=disable", settings.Current.DBHost, settings.Current.DBUsername, settings.Current.DBPassword, settings.Current.DBName, settings.Current.DBPort)
		models.Dbcon, err = gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Info)})
		if err != nil {
			panic("failed to connect database")
		}
		// Automigrate GORM models
		// models.Dbcon.Config.Logger = models.Dbcon.Config.Logger.LogMode(logger.Error)

		err = models.Dbcon.SetupJoinTable(&models.Organization{}, "Users", &models.UserOrgRole{})
		if err != nil {
			fmt.Println("Error setting up join table:", err)
		}
		err = models.Dbcon.SetupJoinTable(&models.User{}, "Roles", &models.UserOrgRole{})
		if err != nil {
			fmt.Println("Error setting up join table:", err)
		}
	},

	Run: func(cmd *cobra.Command, args []string) {
		var g *errgroup.Group
		ctx = context.Background()
		ctx, cancel = context.WithCancel(ctx)
		g, ctx = errgroup.WithContext(ctx)
		// models.Dbcon, err = gorm.Open(sqlite.Open("test.db"), &gorm.Config{})

		// defer models.Dbcon.Close()

		g.Go(func() error { return startGrpcServer(settings.Current) })
		g.Go(func() error { return startHttpServer(settings.Current) })
		HandleGracefulShutdown(g)
	},
}

func startHttpServer(settings *settings.Settings) (err error) {
	perm_cache := permission_cache.NewPermissionCache(settings)
	handler, err := router.NewHandler(settings, perm_cache)
	if err != nil {
		return
	}
	// loggedRouter := handlers.LoggingHandler(loging.ZapWrapper, handler)
	httpServer = &http.Server{
		Addr:         fmt.Sprintf("0.0.0.0:%s", settings.Port),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      handler,
	}
	loging.Logger.Infoln("HTTP/1 Server Started at", httpServer.Addr)
	if err = httpServer.ListenAndServe(); err != http.ErrServerClosed {
		loging.Logger.Errorln(err)
		return err
	}
	return nil
}

func startGrpcServer(settings *settings.Settings) (err error) {
	listener, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		panic(err)
	}
	var auth_server = grpc_auth.NewGRPCServer(settings)
	grpcServer = grpc.NewServer(grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
		grpc_zap.UnaryServerInterceptor(loging.Logger.Desugar()),
		auth_server.JWTInterceptor,
	)))

	reflection.Register(grpcServer)
	grpc_auth.RegisterAuthServer(grpcServer, auth_server)
	loging.Logger.Infoln("GRPC Server Started at ", listener.Addr().String())
	if err = grpcServer.Serve(listener); err != nil {
		loging.Logger.Errorln(err)
	}
	return
}

func HandleGracefulShutdown(g *errgroup.Group) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	select {
	case <-interrupt:
		break
	case <-ctx.Done():
		break
	}
	loging.Logger.Warn("Interupt recieved, Attempting graceful shutdown...")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if httpServer != nil {
		_ = httpServer.Shutdown(shutdownCtx)
	}
	if grpcServer != nil {
		grpcServer.GracefulStop()
	}
	err := g.Wait()
	if err != nil {
		loging.Logger.Errorln("msg", "server returning an error", "error", err)
		os.Exit(2)
	}

}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {

	valids.InitializeValidations()
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.auth.yaml)")
	rootCmd.Flags().StringVarP(&port, "port", "p", "", "port to listen on")
	_ = viper.BindPFlag("port", rootCmd.Flags().Lookup("port"))
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
