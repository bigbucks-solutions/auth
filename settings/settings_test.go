package settings

import "testing"

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
