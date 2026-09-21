package clientip

import (
	"net/http"
	"testing"
)

func request(remote string, headers map[string][]string) *http.Request {
	req, _ := http.NewRequest(http.MethodPost, "http://auth.local/api/v1/signup", nil)
	req.RemoteAddr = remote
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	return req
}

func TestFromRequest(t *testing.T) {
	// The QA topology: nginx on loopback in front of auth, and the web app
	// reaching auth through the host's public address.
	resolver, err := New([]string{"127.0.0.1", "::1", "34.197.58.226"})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		remote  string
		headers map[string][]string
		want    string
	}{
		{
			name:   "an untrusted peer is the client, whatever it claims",
			remote: "203.0.113.9:51000",
			headers: map[string][]string{
				"X-Forwarded-For": {"198.51.100.1"},
				"X-Real-IP":       {"198.51.100.1"},
			},
			want: "203.0.113.9",
		},
		{
			name:    "browser -> nginx -> auth",
			remote:  "127.0.0.1:40000",
			headers: map[string][]string{"X-Forwarded-For": {"198.51.100.7"}},
			want:    "198.51.100.7",
		},
		{
			name:   "browser -> nginx -> web -> nginx -> auth skips our own hops",
			remote: "127.0.0.1:40000",
			headers: map[string][]string{
				"X-Forwarded-For": {"198.51.100.7, 34.197.58.226"},
				// nginx overwrites X-Real-IP with its own peer: the web server.
				"X-Real-IP": {"34.197.58.226"},
			},
			want: "198.51.100.7",
		},
		{
			name:   "a forged entry sent by the client stays to the left and is ignored",
			remote: "127.0.0.1:40000",
			headers: map[string][]string{
				"X-Forwarded-For": {"10.9.9.9, 1.2.3.4, 198.51.100.7, 34.197.58.226"},
			},
			want: "198.51.100.7",
		},
		{
			name:   "several X-Forwarded-For headers read as one chain",
			remote: "127.0.0.1:40000",
			headers: map[string][]string{
				"X-Forwarded-For": {"1.2.3.4", "198.51.100.7, 34.197.58.226"},
			},
			want: "198.51.100.7",
		},
		{
			name:    "garbage stops the walk at the last trusted hop",
			remote:  "127.0.0.1:40000",
			headers: map[string][]string{"X-Forwarded-For": {"Unknown, 34.197.58.226"}},
			want:    "34.197.58.226",
		},
		{
			name:    "only trusted hops: the furthest one",
			remote:  "127.0.0.1:40000",
			headers: map[string][]string{"X-Forwarded-For": {"::1, 34.197.58.226"}},
			want:    "::1",
		},
		{
			name:    "no X-Forwarded-For: X-Real-IP from a trusted peer",
			remote:  "127.0.0.1:40000",
			headers: map[string][]string{"X-Real-IP": {"198.51.100.7"}},
			want:    "198.51.100.7",
		},
		{
			name:   "no headers: the peer",
			remote: "127.0.0.1:40000",
			want:   "127.0.0.1",
		},
		{
			name:    "IPv6 peer and IPv4-mapped hop",
			remote:  "[::1]:40000",
			headers: map[string][]string{"X-Forwarded-For": {"::ffff:198.51.100.7"}},
			want:    "198.51.100.7",
		},
		{
			name:    "a hop with a port",
			remote:  "127.0.0.1:40000",
			headers: map[string][]string{"X-Forwarded-For": {"198.51.100.7:5555"}},
			want:    "198.51.100.7",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolver.FromRequest(request(test.remote, test.headers)); got != test.want {
				t.Fatalf("FromRequest = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDefaultsTrustLoopbackAndPrivateOnly(t *testing.T) {
	resolver, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	forged := map[string][]string{"X-Forwarded-For": {"198.51.100.7"}}

	// A container gateway is trusted by default.
	if got := resolver.FromRequest(request("172.17.0.1:40000", forged)); got != "198.51.100.7" {
		t.Fatalf("private peer: got %q", got)
	}
	// A public peer is not.
	if got := resolver.FromRequest(request("203.0.113.9:40000", forged)); got != "203.0.113.9" {
		t.Fatalf("public peer: got %q", got)
	}
}

func TestNoneTrustsNothing(t *testing.T) {
	resolver, err := New([]string{"none"})
	if err != nil {
		t.Fatal(err)
	}
	got := resolver.FromRequest(request("127.0.0.1:40000", map[string][]string{
		"X-Forwarded-For": {"198.51.100.7"},
	}))
	if got != "127.0.0.1" {
		t.Fatalf("got %q, want the peer", got)
	}
}

func TestNewAcceptsCommaSeparatedEnvValue(t *testing.T) {
	// TRUSTED_PROXIES arrives as one string when set through the environment.
	resolver, err := New([]string{"127.0.0.1, 10.10.0.0/16"})
	if err != nil {
		t.Fatal(err)
	}
	got := resolver.FromRequest(request("10.10.3.4:1", map[string][]string{
		"X-Forwarded-For": {"198.51.100.7"},
	}))
	if got != "198.51.100.7" {
		t.Fatalf("got %q", got)
	}
}

func TestNewRejectsNonsense(t *testing.T) {
	if _, err := New([]string{"nginx.internal"}); err == nil {
		t.Fatal("expected an error for a hostname")
	}
}
