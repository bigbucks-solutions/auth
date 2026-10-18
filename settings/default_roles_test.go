package settings

import (
	"bytes"
	"testing"

	"github.com/spf13/viper"
)

func TestParseGrant(t *testing.T) {
	resource, scope, action, err := ParseGrant(" Sales : ORG : create ")
	if err != nil {
		t.Fatalf("ParseGrant() error = %v", err)
	}
	if resource != "sales" || scope != "org" || action != "create" {
		t.Fatalf("ParseGrant() = %q %q %q", resource, scope, action)
	}
	for _, bad := range []string{"sales:org", "sales::read", "a:b:c:d", ""} {
		if _, _, _, err := ParseGrant(bad); err == nil {
			t.Fatalf("ParseGrant(%q) accepted a malformed grant", bad)
		}
	}
}

func TestDefaultRolesFallsBackToStaff(t *testing.T) {
	original := Current
	defer func() { Current = original }()

	Current = &Settings{}
	roles := DefaultRoles()
	if len(roles) != 1 || roles[0].Name != "Staff" {
		t.Fatalf("DefaultRoles() = %+v, want the built-in Staff role", roles)
	}

	// An explicit empty list turns default roles off.
	Current = &Settings{DefaultRoles: []DefaultRole{}}
	if roles := DefaultRoles(); len(roles) != 0 {
		t.Fatalf("DefaultRoles() = %+v, want none", roles)
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
	var s Settings
	if err := v.Unmarshal(&s); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(s.DefaultRoles) != 1 || s.DefaultRoles[0].Name != "Cashier" ||
		s.DefaultRoles[0].Grants[0] != "payment:org:create" {
		t.Fatalf("decoded %+v", s.DefaultRoles)
	}
}

func TestBuiltinStaffGrantsAreWellFormed(t *testing.T) {
	for _, grant := range BuiltinDefaultRoles[0].Grants {
		if _, scope, _, err := ParseGrant(grant); err != nil || scope == "all" {
			t.Fatalf("built-in grant %q: err=%v scope=%q", grant, err, scope)
		}
	}
}
