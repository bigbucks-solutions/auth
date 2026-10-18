package oauthutils

import "testing"

func TestIdentityFromClaims(t *testing.T) {
	id := identityFromClaims(map[string]interface{}{
		"email": "ana@acme.com", "email_verified": true, "given_name": "Ana", "family_name": "Lee", "hd": "acme.com",
	})
	if id.Email != "ana@acme.com" || !id.EmailVerified || id.HostedDomain != "acme.com" || id.GivenName != "Ana" {
		t.Fatalf("identityFromClaims() = %+v", id)
	}

	// Some tokens carry email_verified as a string; a personal account has no hd.
	id = identityFromClaims(map[string]interface{}{"email": "ana@gmail.com", "email_verified": "true"})
	if !id.EmailVerified || id.HostedDomain != "" {
		t.Fatalf("identityFromClaims() = %+v", id)
	}

	if id = identityFromClaims(map[string]interface{}{"email": "x@y.com"}); id.EmailVerified {
		t.Fatalf("an address with no email_verified claim was taken as verified")
	}
}
