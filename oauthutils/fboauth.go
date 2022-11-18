package oauthutils

import (
	"bigbucks/solution/auth/models"
	"errors"
	"fmt"

	fb "github.com/huandu/facebook/v2"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const FB_SUFFIX = "facebook"

// Authenticate => SignIn/SignUp using facebook OAuth
func FBAuthenticate(accesstoken string) (success bool, user models.User, err error) {

	res, err := fb.Get("/me/?fields=email,first_name,last_name,hometown,gender,birthday", fb.Params{
		"access_token": accesstoken,
	})
	username := fmt.Sprintf("%s+%s", res.Get("email"), FB_SUFFIX)
	if err := models.Dbcon.First(&user, "username = ?", username).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		models.Dbcon.Create(&models.User{Username: username, Password: accesstoken, Profile: models.Profile{
			FirstName: res.Get("first_name").(string), LastName: res.Get("last_name").(string), Email: res.Get("email").(string)},
			OAuthClient: models.OAuthClient{Source: FB_SUFFIX,
				Details: datatypes.JSON([]byte(fmt.Sprintf(`{"accessToken": "%s"}`, accesstoken)))},
		})
	} else {
		user.OAuthClient = models.OAuthClient{Source: FB_SUFFIX,
			Details: datatypes.JSON([]byte(fmt.Sprintf(`{"accessToken": "%s"}`, accesstoken)))}

	}
	return true, user, nil
}
