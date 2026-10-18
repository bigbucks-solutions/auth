package oauthutils

import (
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"go.uber.org/zap"
	"google.golang.org/api/idtoken"
	"gorm.io/datatypes"
	"gorm.io/gorm"
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

// GoogleAuthenticate signs in, or signs up, the user [id] names.
//
// What is kept of the sign-in is the account's Workspace domain, which is
// what proves an organization's single sign-on domain; the Google tokens
// themselves are not stored.
func GoogleAuthenticate(id GoogleIdentity) (user models.User, err error) {
	loging.Logger.Debugln("Google OAuth::", zap.String("Logged User", id.Email))
	username := fmt.Sprintf("%s+%s", id.Email, GOOGLE_SUFFIX)
	details, err := json.Marshal(map[string]string{"hd": id.HostedDomain})
	if err != nil {
		return user, err
	}

	err = models.Dbcon.First(&user, "username = ?", username).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		password, err := unusablePassword()
		if err != nil {
			return user, err
		}
		user = models.User{Username: username, Password: password, EmailVerified: id.EmailVerified,
			Profile:     models.Profile{FirstName: id.GivenName, LastName: id.FamilyName, Email: id.Email},
			OAuthClient: models.OAuthClient{Source: GOOGLE_SUFFIX, Details: datatypes.JSON(details)},
		}
		return user, models.Dbcon.Create(&user).Error
	}
	if err != nil {
		return user, err
	}

	var client models.OAuthClient
	err = models.Dbcon.Where("user_id = ? AND source = ?", user.ID, GOOGLE_SUFFIX).
		Order("updated_at DESC").First(&client).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		err = models.Dbcon.Create(&models.OAuthClient{UserID: user.ID, Source: GOOGLE_SUFFIX,
			Details: datatypes.JSON(details)}).Error
	case err == nil:
		err = models.Dbcon.Model(&client).Update("details", datatypes.JSON(details)).Error
	}
	return user, err
}

// unusablePassword is a password nobody knows, for an account that only
// signs in through Google. It is hashed like any other on create.
func unusablePassword() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
