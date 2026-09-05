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
	// Registers the Stripe subscription adapter. It stays dormant unless
	// configuration selects it.
	_ "bigbucks/solution/auth/contrib/stripe"
	grpc_auth "bigbucks/solution/auth/grpc-auth"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/permission_cache"
	router "bigbucks/solution/auth/rest-api"
	sessionstore "bigbucks/solution/auth/session_store"
	"bigbucks/solution/auth/subscriptions"
	"context"
	"encoding/json"
	"errors"
	"os/signal"
	"syscall"
	"time"

	settings "bigbucks/solution/auth/settings"
	valids "bigbucks/solution/auth/validations"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
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
		if err = loadSubscriptionConfig(settings.Current, viper.ConfigFileUsed()); err != nil {
			loging.Logger.Fatalln(err)
		}
		applySubscriptionOptions(settings.Current)
		settings.Current.Clean()
		settings.Current.LoadKeys()

		config := loging.Config{
			Level: settings.Current.LogLevel, // reads from config file/env
		}
		loging.Initialize(config)
		defer loging.Logger.Sync() //nolint:errcheck
		loging.Logger.Infof("Email links configured for UI host %s", settings.Current.EmailLinkHost())

		// dsn := "user=bigbucks password=bigbucks DB.name=bigbucks port=5432 host=localhost sslmode=disable"
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable", settings.Current.DBHost, settings.Current.DBUsername, settings.Current.DBPassword, settings.Current.DBName, settings.Current.DBPort)
		models.Dbcon, err = gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Info), TranslateError: true})
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
		err = models.Dbcon.SetupJoinTable(&models.Role{}, "Permissions", &models.RolePermission{})
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
	session_store := sessionstore.NewSessionStore(settings)
	handler, err := router.NewHandler(settings, perm_cache, session_store)
	if err != nil {
		return fmt.Errorf("initialize HTTP handler: %w", err)
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
	perm_cache := permission_cache.NewPermissionCache(settings)
	session_store := sessionstore.NewSessionStore(settings)
	auth_server := grpc_auth.NewGRPCServer(settings, *perm_cache, *session_store)
	grpcServer = grpc.NewServer(grpc.ChainUnaryInterceptor(
		grpc_zap.UnaryServerInterceptor(loging.InterceptorLogger(loging.Logger.Desugar())),
		auth_server.JWTInterceptor,
	))

	reflection.Register(grpcServer)
	grpc_auth.RegisterAuthServer(grpcServer, auth_server)
	loging.Logger.Infoln("GRPC Server Started at ", listener.Addr().String())
	if err = grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		loging.Logger.Errorln(err)
		return err
	}
	return nil
}

func HandleGracefulShutdown(g *errgroup.Group) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	select {
	case receivedSignal := <-interrupt:
		loging.Logger.Warnf("Received %s, attempting graceful shutdown...", receivedSignal)
	case <-ctx.Done():
	}
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

	if err := bindEnvironmentVariables(); err != nil {
		panic(fmt.Errorf("failed to bind configuration environment variables: %w", err))
	}
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func bindEnvironmentVariables() error {
	bindings := map[string][]string{
		"uiHost":                           {"UI_HOST", "UIHOST"},
		"googleClientIDs":                  {"GOOGLE_CLIENT_IDS"},
		"emailVerificationSecret":          {"EMAIL_VERIFICATION_SECRET"},
		"emailVerificationTTLSeconds":      {"EMAIL_VERIFICATION_TTL_SECONDS"},
		"emailVerificationMaxAttempts":     {"EMAIL_VERIFICATION_MAX_ATTEMPTS"},
		"emailVerificationResendSeconds":   {"EMAIL_VERIFICATION_RESEND_SECONDS"},
		"emailVerificationHourlySendLimit": {"EMAIL_VERIFICATION_HOURLY_SEND_LIMIT"},
		"subscriptions.enabled":            {"SUBSCRIPTIONS_ENABLED"},
		"subscriptions.mode":               {"SUBSCRIPTIONS_MODE"},
		"subscriptions.provider":           {"SUBSCRIPTIONS_PROVIDER"},
		"subscriptionsConfigFile":          {"SUBSCRIPTIONS_CONFIG_FILE"},
	}
	for option, environmentVariables := range subscriptionOptionEnv {
		bindings[subscriptionOptionKey(option)] = environmentVariables
	}
	for key, environmentVariables := range bindings {
		arguments := append([]string{key}, environmentVariables...)
		if err := viper.BindEnv(arguments...); err != nil {
			return err
		}
	}
	return nil
}

// subscriptionOptionEnv maps a provider option to the environment variables that
// can supply it. Billing credentials are kept out of config.json entirely.
var subscriptionOptionEnv = map[string][]string{
	"secretKey":     {"STRIPE_SECRET_KEY"},
	"webhookSecret": {"STRIPE_WEBHOOK_SECRET"},
	"webhookPath":   {"STRIPE_WEBHOOK_PATH"},
}

func subscriptionOptionKey(option string) string {
	return "subscriptions.options." + option
}

// applySubscriptionOptions copies environment-supplied provider options into the
// decoded settings.
//
// Provider options live in a map[string]string. Viper resolves environment
// bindings for such keys through Get, but does not include them in the tree it
// hands to Unmarshal, so map entries silently stay empty unless copied across
// here. See TestSubscriptionSecretsEnvironmentBinding.
func applySubscriptionOptions(config *settings.Settings) {
	if config == nil {
		return
	}
	for option := range subscriptionOptionEnv {
		value := strings.TrimSpace(viper.GetString(subscriptionOptionKey(option)))
		if value == "" {
			continue
		}
		if config.Subscriptions.Options == nil {
			config.Subscriptions.Options = make(map[string]string, len(subscriptionOptionEnv))
		}
		config.Subscriptions.Options[option] = value
	}
}

func loadSubscriptionConfig(config *settings.Settings, mainConfigFile string) error {
	if config == nil {
		return nil
	}

	if raw := strings.TrimSpace(os.Getenv("SUBSCRIPTIONS_CONFIG_JSON")); raw != "" {
		loaded, err := decodeSubscriptionConfig([]byte(raw))
		if err != nil {
			return fmt.Errorf("decode SUBSCRIPTIONS_CONFIG_JSON: %w", err)
		}
		config.Subscriptions = loaded
		return nil
	}

	if configFile := strings.TrimSpace(config.SubscriptionsConfigFile); configFile != "" {
		if !filepath.IsAbs(configFile) && mainConfigFile != "" {
			configFile = filepath.Join(filepath.Dir(mainConfigFile), configFile)
		}
		data, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("read subscription config %q: %w", configFile, err)
		}
		loaded, err := decodeSubscriptionConfig(data)
		if err != nil {
			return fmt.Errorf("decode subscription config %q: %w", configFile, err)
		}
		config.Subscriptions = loaded
		return nil
	}

	if mainConfigFile == "" {
		return nil
	}
	data, err := os.ReadFile(mainConfigFile)
	if err != nil {
		return fmt.Errorf("read inline subscription config: %w", err)
	}
	var inline struct {
		Subscriptions json.RawMessage `json:"subscriptions"`
	}
	if err := json.Unmarshal(data, &inline); err != nil {
		return nil
	}
	if len(inline.Subscriptions) == 0 {
		return nil
	}
	loaded, err := decodeSubscriptionConfig(inline.Subscriptions)
	if err != nil {
		return fmt.Errorf("decode inline subscription config: %w", err)
	}
	config.Subscriptions = loaded
	return nil
}

func decodeSubscriptionConfig(data []byte) (subscriptions.Config, error) {
	var config subscriptions.Config
	err := json.Unmarshal(data, &config)
	return config, err
}
