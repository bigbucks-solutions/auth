package models

import (
	"strings"
	"testing"

	"gorm.io/datatypes"
)

func TestMicrosoftIdentity(t *testing.T) {
	for _, tc := range []struct {
		name    string
		details string
		want    string
	}{
		{"tenant and object", `{"tid":"t1","oid":"o1"}`, "t1/o1"},
		{"no object ID", `{"tid":"t1"}`, ""},
		{"a Google link's details", `{"hd":"acme.example"}`, ""},
		{"not JSON", `not json`, ""},
		{"empty", ``, ""},
	} {
		if got := MicrosoftIdentity(datatypes.JSON(tc.details)); got != tc.want {
			t.Errorf("%s: MicrosoftIdentity(%s) = %q, want %q", tc.name, tc.details, got, tc.want)
		}
	}
	// The same object ID in another tenant is another identity.
	if (MicrosoftDetails{TenantID: "t1", ObjectID: "o"}).Identity() == (MicrosoftDetails{TenantID: "t2", ObjectID: "o"}).Identity() {
		t.Error("identities in two tenants compared equal")
	}
}

func TestHashPassword(t *testing.T) {
	if hashed, err := hashPassword(""); err != nil || hashed != "" {
		t.Fatalf(`hashPassword("") = %q, %v; want "" (no password)`, hashed, err)
	}
	hashed, err := hashPassword("password123")
	if err != nil || hashed == "" || hashed == "password123" {
		t.Fatalf("hashPassword() = %q, %v; want a hash", hashed, err)
	}
	long := strings.Repeat("a", MaxPasswordBytes+1)
	if hashed, err := hashPassword(long); err == nil {
		t.Fatalf("hashPassword(%d bytes) = %q; want an error, not the password as typed", len(long), hashed)
	}
	if _, err := hashPassword(strings.Repeat("a", MaxPasswordBytes)); err != nil {
		t.Fatalf("hashPassword(%d bytes) failed: %v", MaxPasswordBytes, err)
	}
}

func TestMergeOutcomeAccountsRetired(t *testing.T) {
	for _, tc := range []struct {
		outcome MergeOutcome
		want    int
	}{
		{MergeOutcome{}, 0},
		{MergeOutcome{Merged: []string{"a+google", "a+microsoft"}}, 2},
		{MergeOutcome{Renamed: "a+google", Merged: []string{"a+microsoft"}}, 2},
		{MergeOutcome{Renamed: "a+google"}, 1},
		{MergeOutcome{Conflicts: []string{"a+google: deactivated"}}, 0},
	} {
		if got := tc.outcome.AccountsRetired(); got != tc.want {
			t.Errorf("%+v.AccountsRetired() = %d, want %d", tc.outcome, got, tc.want)
		}
	}
}
