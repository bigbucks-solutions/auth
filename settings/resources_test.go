package settings_test

import (
	"bigbucks/solution/auth/settings"
	"slices"
	"testing"
)

func TestGrantableResourcesCombines(t *testing.T) {
	settings.Current = &settings.Settings{
		ExtraPermResources: []string{" Sales ", "party", "inventory", "", "SALES"},
	}
	got := settings.GrantableResources()

	for _, want := range []string{"user", "role", "billing", "organization", "inventory", "sales", "party"} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %q from %v", want, got)
		}
	}
	// Trimmed, lower-cased, and listed once however it was written.
	var sales int
	for _, r := range got {
		if r == "sales" {
			sales++
		}
	}
	if sales != 1 {
		t.Errorf("sales appears %d times in %v, want 1", sales, got)
	}
	if slices.Contains(got, "") {
		t.Errorf("empty resource kept: %v", got)
	}
}

func TestGrantableResourcesWithoutSettings(t *testing.T) {
	settings.Current = nil
	if got := settings.GrantableResources(); !slices.Contains(got, "user") {
		t.Errorf("auth's own resources should survive a nil settings: %v", got)
	}
}
