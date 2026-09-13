package grpc_auth

import (
	"strings"
	"testing"
)

func TestParseServiceKeysMatchesConfiguredServices(t *testing.T) {
	inventoryKey := strings.Repeat("a", MinServiceKeyLength)
	// Keys may carry "=" padding; only the first "=" separates the name.
	billingKey := strings.Repeat("b", MinServiceKeyLength) + "=="

	keys, err := ParseServiceKeys(" inventory=" + inventoryKey + " , billing=" + billingKey + ",")
	if err != nil {
		t.Fatalf("ParseServiceKeys() error = %v", err)
	}
	if keys.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", keys.Len())
	}

	for presented, want := range map[string]string{inventoryKey: "inventory", billingKey: "billing"} {
		if got, ok := keys.Match(presented); !ok || got != want {
			t.Fatalf("Match() = %q, %v, want %q", got, ok, want)
		}
	}
	for _, presented := range []string{"", "wrong", inventoryKey + "x"} {
		if name, ok := keys.Match(presented); ok {
			t.Fatalf("Match(%q) matched %q, want no match", presented, name)
		}
	}
}

func TestParseServiceKeysEmptyConfiguresNothing(t *testing.T) {
	keys, err := ParseServiceKeys("  ")
	if err != nil {
		t.Fatalf("ParseServiceKeys() error = %v", err)
	}
	if keys.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", keys.Len())
	}
	if _, ok := keys.Match(""); ok {
		t.Fatal("an empty key must never match")
	}
}

func TestParseServiceKeysRejectsBadEntries(t *testing.T) {
	key := strings.Repeat("k", MinServiceKeyLength)
	tests := map[string]string{
		"a bare key":                   key,
		"a missing name":               "=" + key,
		"a short key":                  "inventory=short",
		"a repeated name":              "inventory=" + key + ",inventory=" + strings.Repeat("z", MinServiceKeyLength),
		"a key shared by two services": "inventory=" + key + ",billing=" + key,
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseServiceKeys(raw)
			if err == nil {
				t.Fatal("ParseServiceKeys() error = nil, want an error")
			}
			if strings.Contains(err.Error(), key) {
				t.Fatalf("error %q leaks the key", err)
			}
		})
	}
}
