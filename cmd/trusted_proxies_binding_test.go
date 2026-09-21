package cmd

import (
	"testing"

	"bigbucks/solution/auth/clientip"
	"bigbucks/solution/auth/settings"

	"github.com/spf13/viper"
)

// TRUSTED_PROXIES arrives as one comma-separated string. It has to survive
// viper's decode into []string and still build a resolver, or auth would
// silently fall back to the defaults on a host that needs more.
func TestTrustedProxiesEnvironmentBinding(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1, ::1, 34.197.58.226, 10.10.0.0/16")

	if err := bindEnvironmentVariables(); err != nil {
		t.Fatalf("bindEnvironmentVariables() error = %v", err)
	}
	var config settings.Settings
	if err := viper.Unmarshal(&config); err != nil {
		t.Fatalf("viper.Unmarshal() error = %v", err)
	}
	if len(config.TrustedProxies) == 0 {
		t.Fatal("TrustedProxies is empty; TRUSTED_PROXIES was not bound")
	}
	if _, err := clientip.New(config.TrustedProxies); err != nil {
		t.Fatalf("clientip.New(%q) error = %v", config.TrustedProxies, err)
	}
}
