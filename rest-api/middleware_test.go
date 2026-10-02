package rest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/gorilla/mux"
)

func TestAccessLogQueryParams(t *testing.T) {
	values := url.Values{
		"country":      {"IN"},
		"currency":     {"inr"},
		"filter":       {"active", "trialing"},
		"access_token": {"do-not-log"},
		"code":         {"do-not-log"},
	}

	got := accessLogQueryParams(values)
	want := map[string][]string{
		"country":      {"IN"},
		"currency":     {"inr"},
		"filter":       {"active", "trialing"},
		"access_token": {"[REDACTED]"},
		"code":         {"[REDACTED]"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("accessLogQueryParams() = %#v, want %#v", got, want)
	}

	values.Set("country", "AE")
	if got["country"][0] != "IN" {
		t.Fatal("logged query parameters must not alias the request values")
	}
}

// The organization a request is about decides which tenant every check below it
// answers for, so reading it wrong is a cross-tenant bug rather than a nuisance:
// a caller naming their own organization in the header and somebody else's in
// the path must be checked against the one in the path.
func TestRequestOrgID(t *testing.T) {
	tests := []struct {
		name   string
		vars   map[string]string
		header string
		want   string
	}{
		{name: "path only", vars: map[string]string{"org_id": "PATHORG"}, want: "PATHORG"},
		{name: "header only", header: "HEADERORG", want: "HEADERORG"},
		{
			name:   "path wins over a header naming another organization",
			vars:   map[string]string{"org_id": "PATHORG"},
			header: "HEADERORG",
			want:   "PATHORG",
		},
		{
			name:   "blank path value falls back to the header",
			vars:   map[string]string{"org_id": "  "},
			header: "HEADERORG",
			want:   "HEADERORG",
		},
		{name: "neither", want: ""},
		{name: "header is trimmed", header: " HEADERORG ", want: "HEADERORG"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/PATHORG", nil)
			if test.header != "" {
				request.Header.Set("X-Organization-Id", test.header)
			}
			if test.vars != nil {
				request = mux.SetURLVars(request, test.vars)
			}
			if got := requestOrgID(request); got != test.want {
				t.Fatalf("requestOrgID() = %q, want %q", got, test.want)
			}
		})
	}
}
