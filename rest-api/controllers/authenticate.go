package controllers

import (
	cnst "bigbucks/solution/auth/constants"
	jwtops "bigbucks/solution/auth/jwt-ops"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	oauth "bigbucks/solution/auth/oauthutils"
	"bigbucks/solution/auth/request_context"
	"bigbucks/solution/auth/settings"
	"encoding/json"
	"net/http"

	googleAuthIDTokenVerifier "github.com/futurenda/google-auth-id-token-verifier"
)

// Struct for parsing json body login credentials
type JsonCred struct {
	Password  string `json:"password"`
	Username  string `json:"username"`
	ReCaptcha string `json:"recaptcha"`
}

// Struct for parsing Google oauth login credentials
type GoogleSigninCred struct {
	IdToken     string `json:"idToken"`
	AccessToken string `json:"accessToken"`
}

var googleIdTokenver googleAuthIDTokenVerifier.Verifier = googleAuthIDTokenVerifier.Verifier{}

func init() {
	// googleIdTokenver := googleAuthIDTokenVerifier.Verifier{}
}

// PasswordReset godoc
// @Summary      Authenticate with username and pssword
// @Description  Authenticate user with password and issue jwt token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body  JsonCred  true  "request body"
// @Success      200  string  ""
// @Failure      400  ""
// @Failure      404  ""
// @Failure      500  ""
// @Router       /signin [post]
func Signin(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	// time.Sleep(50 * time.Second)
	var cred JsonCred
	err := json.NewDecoder(r.Body).Decode(&cred)
	if err != nil {
		return http.StatusBadRequest, err
	}
	success, user := models.Authenticate(cred.Username, cred.Password)
	if !success {
		return http.StatusUnauthorized, nil
	}
	return printToken(w, r, &user, ctx.Settings)
}

func GoogleSignin(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	var googCred GoogleSigninCred
	err := json.NewDecoder(r.Body).Decode(&googCred)
	if err != nil {
		return http.StatusBadRequest, err
	}
	err = googleIdTokenver.VerifyIDToken(googCred.IdToken, cnst.GoogleClientID)
	if err == nil {
		success, user, _ := oauth.GoogleAuthenticate(googCred.IdToken, googCred.AccessToken)
		if !success {
			return http.StatusUnauthorized, nil
		}
		return printToken(w, r, &user, ctx.Settings)
	}
	return http.StatusBadRequest, err
	// return printToken(w, r, &user, &ctx.Settings)
}

func FbSignin(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	var googCred GoogleSigninCred
	err := json.NewDecoder(r.Body).Decode(&googCred)
	if err != nil {
		loging.Logger.Error("Error decoding json", err)
		return http.StatusBadRequest, err
	}
	success, user, _ := oauth.FBAuthenticate(googCred.AccessToken)
	if !success {
		return http.StatusUnauthorized, nil
	}
	return printToken(w, r, &user, ctx.Settings)
	// return printToken(w, r, &user, &ctx.Settings)
}

func RenewToken(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	user, _ := ctx.GetCurrentUserModel()
	return printToken(w, r, user, ctx.Settings)
}

func printToken(w http.ResponseWriter, _ *http.Request, user *models.User, _ *settings.Settings) (int, error) {
	signed, err := jwtops.SignJWT(user)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	w.WriteHeader(http.StatusAccepted)
	w.Header().Set("Content-Type", "text")
	_, err = w.Write([]byte(signed))
	if err != nil {
		loging.Logger.Error("Error writing to response on token print", err)
		return http.StatusInternalServerError, err
	}
	return 0, nil
}
