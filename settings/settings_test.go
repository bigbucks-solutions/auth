package settings

import (
	"testing"
	"time"
)

func TestEmailLinkHost(t *testing.T) {
	tests := []struct {
		name     string
		settings Settings
		want     string
	}{
		{
			name:     "uses UI host",
			settings: Settings{UIHost: "https://app.example.com/", BaseHost: "https://api.example.com"},
			want:     "https://app.example.com",
		},
		{
			name:     "falls back to base host",
			settings: Settings{BaseHost: "https://api.example.com/"},
			want:     "https://api.example.com",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.settings.EmailLinkHost(); got != test.want {
				t.Fatalf("EmailLinkHost() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestEmailVerificationPolicyDefaults(t *testing.T) {
	settings := Settings{EmailVerificationSecret: "test-email-verification-secret-32-bytes"}
	ttl, attempts, cooldown, hourlyLimit, err := settings.EmailVerificationPolicy()
	if err != nil {
		t.Fatalf("EmailVerificationPolicy() error = %v", err)
	}
	if ttl != 10*time.Minute || attempts != 5 || cooldown != time.Minute || hourlyLimit != 5 {
		t.Fatalf("unexpected defaults: ttl=%v attempts=%d cooldown=%v hourlyLimit=%d", ttl, attempts, cooldown, hourlyLimit)
	}
}

func TestEmailVerificationPolicyRejectsWeakSecret(t *testing.T) {
	settings := Settings{EmailVerificationSecret: "too-short"}
	if _, _, _, _, err := settings.EmailVerificationPolicy(); err == nil {
		t.Fatal("EmailVerificationPolicy() expected weak-secret error")
	}
}
