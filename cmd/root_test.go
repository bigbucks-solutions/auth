package cmd

import (
	"testing"

	"bigbucks/solution/auth/settings"

	"github.com/spf13/viper"
)

func TestUIHostEnvironmentBinding(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("UI_HOST", "https://app.nsmail.dev")
	viper.SetDefault("baseHost", "https://auth.nsmail.dev")

	if err := bindEnvironmentVariables(); err != nil {
		t.Fatalf("bindEnvironmentVariables() error = %v", err)
	}

	var config settings.Settings
	if err := viper.Unmarshal(&config); err != nil {
		t.Fatalf("viper.Unmarshal() error = %v", err)
	}

	if got, want := config.EmailLinkHost(), "https://app.nsmail.dev"; got != want {
		t.Fatalf("EmailLinkHost() = %q, want %q", got, want)
	}
}

func TestEmailVerificationEnvironmentBinding(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("EMAIL_VERIFICATION_SECRET", "environment-email-verification-secret")

	if err := bindEnvironmentVariables(); err != nil {
		t.Fatalf("bindEnvironmentVariables() error = %v", err)
	}

	var config settings.Settings
	if err := viper.Unmarshal(&config); err != nil {
		t.Fatalf("viper.Unmarshal() error = %v", err)
	}
	if got := config.EmailVerificationSecret; got != "environment-email-verification-secret" {
		t.Fatalf("EmailVerificationSecret = %q", got)
	}
}
