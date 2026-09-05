package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"bigbucks/solution/auth/settings"
	"bigbucks/solution/auth/subscriptions"

	"github.com/spf13/viper"
)

// Stripe credentials are supplied by the environment rather than config.json.
// They land inside a map[string]string, which is the part of viper's env
// binding most likely to silently drop values, so it is asserted explicitly.
func TestSubscriptionSecretsEnvironmentBinding(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("SUBSCRIPTIONS_ENABLED", "true")
	t.Setenv("SUBSCRIPTIONS_MODE", "enforce")
	t.Setenv("SUBSCRIPTIONS_PROVIDER", "stripe")
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_binding")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test_binding")

	if err := bindEnvironmentVariables(); err != nil {
		t.Fatalf("bindEnvironmentVariables() error = %v", err)
	}

	var config settings.Settings
	if err := viper.Unmarshal(&config); err != nil {
		t.Fatalf("viper.Unmarshal() error = %v", err)
	}
	applySubscriptionOptions(&config)

	if got := config.Subscriptions.EffectiveMode(); got != subscriptions.ModeEnforce {
		t.Fatalf("EffectiveMode() = %q, want %q", got, subscriptions.ModeEnforce)
	}
	if got := config.Subscriptions.Provider; got != "stripe" {
		t.Fatalf("Provider = %q, want stripe", got)
	}
	if got := config.Subscriptions.Options["secretKey"]; got != "sk_test_binding" {
		t.Fatalf("Options[secretKey] = %q, want sk_test_binding", got)
	}
	if got := config.Subscriptions.Options["webhookSecret"]; got != "whsec_test_binding" {
		t.Fatalf("Options[webhookSecret] = %q, want whsec_test_binding", got)
	}
}

// A deployment that says nothing about subscriptions must stay disabled.
func TestSubscriptionsDefaultToDisabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	if err := bindEnvironmentVariables(); err != nil {
		t.Fatalf("bindEnvironmentVariables() error = %v", err)
	}

	var config settings.Settings
	if err := viper.Unmarshal(&config); err != nil {
		t.Fatalf("viper.Unmarshal() error = %v", err)
	}
	if got := config.Subscriptions.EffectiveMode(); got != subscriptions.ModeDisabled {
		t.Fatalf("EffectiveMode() = %q, want %q", got, subscriptions.ModeDisabled)
	}
}

func TestLoadSubscriptionConfigPreservesStripeIDCase(t *testing.T) {
	t.Setenv("SUBSCRIPTIONS_CONFIG_JSON", "")
	configFile := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configFile, []byte(`{
		"subscriptions": {
			"enabled": true,
			"catalog": {
				"price_1UCHMe2ORKoZ9VxLHWKTZD7r": {"name": "Starter"}
			}
		}
	}`), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("viper.ReadInConfig() error = %v", err)
	}

	var config settings.Settings
	if err := viper.Unmarshal(&config); err != nil {
		t.Fatalf("viper.Unmarshal() error = %v", err)
	}
	if err := loadSubscriptionConfig(&config, configFile); err != nil {
		t.Fatalf("loadSubscriptionConfig() error = %v", err)
	}

	const priceID = "price_1UCHMe2ORKoZ9VxLHWKTZD7r"
	if _, ok := config.Subscriptions.Catalog[priceID]; !ok {
		t.Fatalf("catalog does not contain case-sensitive price ID %q: %#v", priceID, config.Subscriptions.Catalog)
	}
}

func TestSubscriptionConfigJSONOverridesFile(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "subscriptions.json")
	if err := os.WriteFile(configFile, []byte(`{"provider":"file"}`), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	t.Setenv("SUBSCRIPTIONS_CONFIG_JSON", `{"enabled":true,"provider":"stripe","catalog":{"price_MixedCase":{}}}`)

	config := settings.Settings{SubscriptionsConfigFile: configFile}
	if err := loadSubscriptionConfig(&config, ""); err != nil {
		t.Fatalf("loadSubscriptionConfig() error = %v", err)
	}
	if config.Subscriptions.Provider != "stripe" {
		t.Fatalf("Provider = %q, want stripe", config.Subscriptions.Provider)
	}
	if _, ok := config.Subscriptions.Catalog["price_MixedCase"]; !ok {
		t.Fatalf("catalog key casing was not preserved: %#v", config.Subscriptions.Catalog)
	}
}

func TestSubscriptionConfigFileReplacesInlineConfig(t *testing.T) {
	t.Setenv("SUBSCRIPTIONS_CONFIG_JSON", "")
	directory := t.TempDir()
	mainConfigFile := filepath.Join(directory, "config.json")
	subscriptionConfigFile := filepath.Join(directory, "subscriptions.json")
	if err := os.WriteFile(subscriptionConfigFile, []byte(`{"enabled":true,"provider":"stripe"}`), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	config := settings.Settings{
		SubscriptionsConfigFile: "subscriptions.json",
		Subscriptions: subscriptions.Config{
			Provider: "legacy",
			Catalog:  map[string]subscriptions.Product{"price_old": {}},
		},
	}
	if err := loadSubscriptionConfig(&config, mainConfigFile); err != nil {
		t.Fatalf("loadSubscriptionConfig() error = %v", err)
	}
	if config.Subscriptions.Provider != "stripe" || config.Subscriptions.Catalog != nil {
		t.Fatalf("external config was merged instead of replaced: %#v", config.Subscriptions)
	}
}
