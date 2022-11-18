package oauthutils

import (
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"errors"
	"fmt"

	googleAuthIDTokenVerifier "github.com/futurenda/google-auth-id-token-verifier"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const GOOGLE_SUFFIX = "google"

// Authenticate => SignIn/SignUp using google OAuth
func GoogleAuthenticate(idtoken, accesstoken string) (success bool, user models.User, err error) {
	claimSet, _ := googleAuthIDTokenVerifier.Decode(idtoken)
	loging.Logger.Debugln("Google OAuth::", zap.String("Logged User", claimSet.Email))
	username := fmt.Sprintf("%s+%s", claimSet.Email, GOOGLE_SUFFIX)
	if err := models.Dbcon.First(&user, "username = ?", username).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		models.Dbcon.Create(&models.User{Username: username, Password: idtoken, Profile: models.Profile{
			FirstName: claimSet.GivenName, LastName: claimSet.FamilyName, Email: claimSet.Email},
			OAuthClient: models.OAuthClient{Source: GOOGLE_SUFFIX,
				Details: datatypes.JSON([]byte(fmt.Sprintf(`{"idToken": "%s", "accessToken": "%s"}`, idtoken, accesstoken)))},
		})
	} else {
		user.OAuthClient = models.OAuthClient{Source: GOOGLE_SUFFIX,
			Details: datatypes.JSON([]byte(fmt.Sprintf(`{"idToken": "%s", "accessToken": "%s"}`, idtoken, accesstoken)))}

	}
	return true, user, nil
}
