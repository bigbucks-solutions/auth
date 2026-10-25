package auth_test

import (
	"bigbucks/solution/auth/settings"
	"bytes"
	"testing"

	"github.com/spf13/viper"
)

func TestParseGrant(t *testing.T) {
	resource, scope, action, err := settings.ParseGrant(" Sales : ORG : create ")
	if err != nil {
		t.Fatalf("ParseGrant() error = %v", err)
	}
	if resource != "sales" || scope != "org" || action != "create" {
		t.Fatalf("ParseGrant() = %q %q %q", resource, scope, action)
	}
	for _, bad := range []string{"sales:org", "sales::read", "a:b:c:d", ""} {
		if _, _, _, err := settings.ParseGrant(bad); err == nil {
			t.Fatalf("ParseGrant(%q) accepted a malformed grant", bad)
		}
	}
}

// Auth names no roles of its own: a deployment that configures none gets none.
func TestDefaultRolesAreOnlyWhatIsConfigured(t *testing.T) {
	original := settings.Current
	defer func() { settings.Current = original }()
	settings.Current = &settings.Settings{}
	if roles := settings.DefaultRoles(); len(roles) != 0 {
		t.Fatalf("DefaultRoles() = %+v, want none when unconfigured", roles)
	}
	settings.Current = &settings.Settings{DefaultRoles: []settings.DefaultRole{{Name: "Clerk"}}}
	if roles := settings.DefaultRoles(); len(roles) != 1 || roles[0].Name != "Clerk" {
		t.Fatalf("DefaultRoles() = %+v, want the configured Clerk", roles)
	}
}

// config.json is read through viper, which lower-cases map keys — the reason
// a role's name is a field rather than a key.
func TestDefaultRolesDecodeFromConfig(t *testing.T) {
	v := viper.New()
	v.SetConfigType("json")
	config := []byte(`{"defaultRoles": [
		{"name": "Cashier", "description": "Takes money", "grants": ["payment:org:create"]}
	]}`)
	if err := v.ReadConfig(bytes.NewReader(config)); err != nil {
		t.Fatalf("ReadConfig() error = %v", err)
	}
	var s settings.Settings
	if err := v.Unmarshal(&s); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(s.DefaultRoles) != 1 || s.DefaultRoles[0].Name != "Cashier" ||
		s.DefaultRoles[0].Grants[0] != "payment:org:create" {
		t.Fatalf("decoded %+v", s.DefaultRoles)
	}
}
