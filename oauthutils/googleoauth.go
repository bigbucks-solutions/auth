package oauthutils

import (
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"go.uber.org/zap"
	"google.golang.org/api/idtoken"
	"gorm.io/datatypes"
)

const GOOGLE_SUFFIX = "google"

var googleIssuers = []string{"accounts.google.com", "https://accounts.google.com"}

// GoogleIdentity is who a verified Google ID token says the user is.
type GoogleIdentity struct {
	Email         string
	EmailVerified bool
	GivenName     string
	FamilyName    string
	// HostedDomain is the token's `hd` claim: the Google Workspace domain of
	// the account, empty for a personal one.
	HostedDomain string
}

// VerifyGoogleIDToken checks [idToken] was signed by Google for one of
// [clientIDs] and has not expired, and reads the identity in it.
func VerifyGoogleIDToken(ctx context.Context, idToken string, clientIDs []string) (GoogleIdentity, error) {
	payload, err := idtoken.Validate(ctx, idToken, "")
	if err != nil {
		return GoogleIdentity{}, err
	}
	if !slices.Contains(googleIssuers, payload.Issuer) {
		return GoogleIdentity{}, fmt.Errorf("google id token: unexpected issuer %q", payload.Issuer)
	}
	if !slices.Contains(clientIDs, payload.Audience) {
		return GoogleIdentity{}, errors.New("google id token: issued to another client")
	}
	return identityFromClaims(payload.Claims), nil
}

func identityFromClaims(claims map[string]interface{}) GoogleIdentity {
	text := func(key string) string {
		value, _ := claims[key].(string)
		return value
	}
	verified, _ := claims["email_verified"].(bool)
	if !verified {
		// Some tokens carry it as a string.
		verified = text("email_verified") == "true"
	}
	return GoogleIdentity{
		Email:         text("email"),
		EmailVerified: verified,
		GivenName:     text("given_name"),
		FamilyName:    text("family_name"),
		HostedDomain:  text("hd"),
	}
}

// GoogleAuthenticate signs in, or signs up, the user [id] names: the one
// account at the address, with Google linked to it. See
// models.SignInWithProvider for what linking does to an existing account.
//
// What is kept of the sign-in is the account's Workspace domain, which is
// what proves an organization's single sign-on domain; the Google tokens
// themselves are not stored.
func GoogleAuthenticate(id GoogleIdentity) (models.LinkedAccount, error) {
	loging.Logger.Debugln("Google OAuth::", zap.String("Logged User", id.Email))
	details, err := json.Marshal(map[string]string{"hd": id.HostedDomain})
	if err != nil {
		return models.LinkedAccount{}, err
	}
	return models.SignInWithProvider(models.ProviderSignIn{
		Address: id.Email, EmailVerified: id.EmailVerified, Provider: GOOGLE_SUFFIX,
		Details: datatypes.JSON(details), FirstName: id.GivenName, LastName: id.FamilyName,
	})
}
