package actions

import (
	"testing"

	"bigbucks/solution/auth/settings"
)

// The billing provider redirects the browser to whatever URL it is handed, so an
// unvalidated caller-supplied value would be an open redirect on an
// authenticated flow. Only paths and same-origin URLs may survive.
func TestResolveBillingRedirectRejectsForeignOrigins(t *testing.T) {
	previous := settings.Current
	t.Cleanup(func() { settings.Current = previous })
	settings.Current = &settings.Settings{UIHost: "https://app.bigbucks.solutions"}

	tests := []struct {
		name      string
		candidate string
		want      string
	}{
		{
			name:      "empty falls back to the default page",
			candidate: "",
			want:      "https://app.bigbucks.solutions/billing",
		},
		{
			name:      "relative path is anchored to the ui origin",
			candidate: "/settings/billing?status=done",
			want:      "https://app.bigbucks.solutions/settings/billing?status=done",
		},
		{
			name:      "same origin absolute url is kept",
			candidate: "https://app.bigbucks.solutions/settings/billing",
			want:      "https://app.bigbucks.solutions/settings/billing",
		},
		{
			name:      "different host is rejected",
			candidate: "https://evil.example.com/harvest",
			want:      "https://app.bigbucks.solutions/billing",
		},
		{
			name:      "downgrade to http is rejected",
			candidate: "http://app.bigbucks.solutions/settings",
			want:      "https://app.bigbucks.solutions/billing",
		},
		{
			// "//evil.com" is protocol-relative: a browser reads it as a host,
			// not a path, so it must not be treated as a local redirect.
			name:      "protocol relative url is rejected",
			candidate: "//evil.example.com/harvest",
			want:      "https://app.bigbucks.solutions/billing",
		},
		{
			name:      "lookalike subdomain is rejected",
			candidate: "https://app.bigbucks.solutions.evil.example.com/x",
			want:      "https://app.bigbucks.solutions/billing",
		},
		{
			name:      "javascript scheme is rejected",
			candidate: "javascript:alert(1)",
			want:      "https://app.bigbucks.solutions/billing",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ResolveBillingRedirect(test.candidate, "/billing"); got != test.want {
				t.Fatalf("ResolveBillingRedirect(%q) = %q, want %q", test.candidate, got, test.want)
			}
		})
	}
}

func TestResolveBillingRedirectWithoutSettings(t *testing.T) {
	previous := settings.Current
	t.Cleanup(func() { settings.Current = previous })
	settings.Current = nil

	if got := ResolveBillingRedirect("https://evil.example.com", "/billing"); got != "/billing" {
		t.Fatalf("ResolveBillingRedirect() = %q, want %q", got, "/billing")
	}
}
