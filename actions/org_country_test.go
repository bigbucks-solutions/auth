package actions

import (
	"net/http/httptest"
	"strings"
	"testing"

	valids "bigbucks/solution/auth/validations"
)

// Organization country feeds the billing currency mapping, which matches
// upper-case ISO-3166 alpha-2 codes exactly. Anything else has to be rejected at
// the edge rather than silently failing to map later.
func TestOrganizationCountryValidation(t *testing.T) {
	valids.InitializeValidations()

	tests := []struct {
		name    string
		country string
		wantErr bool
		stored  string
	}{
		{name: "upper-case code", country: "AE", stored: "AE"},
		{name: "lower-case is normalised, not rejected", country: "ae", stored: "AE"},
		{name: "surrounding whitespace is trimmed", country: "  in  ", stored: "IN"},
		{name: "empty is allowed", country: "", stored: ""},
		{name: "alpha-3 is rejected", country: "ARE", wantErr: true},
		{name: "country name is rejected", country: "United Arab Emirates", wantErr: true},
		{name: "not a real code", country: "ZZ", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := `{"name":"Bigbucks Ltd","email":"owner@example.com","country":"` + test.country + `"}`
			request := httptest.NewRequest("POST", "/organizations", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")

			org, _, err := OrganizationFromRequest(request)
			if err != nil {
				t.Fatalf("OrganizationFromRequest() error = %v", err)
			}
			if org.Country != test.stored && !test.wantErr {
				t.Fatalf("normalised country = %q, want %q", org.Country, test.stored)
			}

			err = valids.Validate.Struct(org)
			if test.wantErr {
				if err == nil {
					t.Fatalf("validation accepted %q, want it rejected", test.country)
				}
				return
			}
			if err != nil {
				t.Fatalf("validation rejected %q: %v", test.country, err)
			}
		})
	}
}
