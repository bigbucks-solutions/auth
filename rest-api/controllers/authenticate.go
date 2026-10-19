package controllers

import (
	"bigbucks/solution/auth/constants"
	jwtops "bigbucks/solution/auth/jwt-ops"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	oauth "bigbucks/solution/auth/oauthutils"
	"bigbucks/solution/auth/request_context"
	"encoding/json"
	"errors"
	"net/http"

	"bigbucks/solution/auth/actions"
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

// Signin godoc
//
//	@Summary		Authenticate with username and pssword
//	@Description	Authenticate user with password and issue jwt token
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		JsonCred	true	"request body"
//	@Success		200		{string}	string		"JWT token"
//	@Failure		400		{object}	error		"Bad request"
//	@Failure		404		{object}	error		"Not found"
//	@Failure		500		{object}	error		"Internal server error"
//	@Router			/signin [post]
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
	if !user.EmailVerified {
		return http.StatusForbidden, errors.New("email_verification_required")
	}
	if user.Status == constants.UserStatusInactive {
		return http.StatusForbidden, errors.New("account_inactive")
	}
	userAgent := r.UserAgent()
	ip := clientIP(r)
	// JWT expiration time (e.g., 24 hours)

	sessionId, err := ctx.SessionStore.CreateSession(user.ID, user.Username, userAgent, ip, constants.SESSION_EXPIRY)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	go func() {
		if err := user.LogLoginActivity(map[string]interface{}{
			"ip":         ip,
			"user_agent": userAgent,
		}); err != nil {
			loging.Logger.Error("Error logging login activity", err)
		}
	}()

	return printToken(w, r, &user, sessionId)
}

func GoogleSignin(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	var googCred GoogleSigninCred
	err := json.NewDecoder(r.Body).Decode(&googCred)
	if err != nil {
		return http.StatusBadRequest, err
	}
	identity, err := oauth.VerifyGoogleIDToken(r.Context(), googCred.IdToken, ctx.Settings.GoogleClientIDs)
	if err != nil {
		return http.StatusBadRequest, err
	}
	// The account is named after the address, so an address Google has not
	// verified could be anyone's.
	if identity.Email == "" || !identity.EmailVerified {
		return http.StatusUnauthorized, errors.New("email_verification_required")
	}
	user, err := oauth.GoogleAuthenticate(identity)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return finishProviderSignin(w, r, ctx, user, models.SSOProviderGoogle, identity.HostedDomain)
}

// Struct for parsing Microsoft sign-in credentials: the ID token the web app's
// server got for the user's authorization code.
type MicrosoftSigninCred struct {
	IdToken string `json:"idToken"`
}

// MicrosoftSignin godoc
//
//	@Summary		Sign in with Microsoft
//	@Description	Signs in, or signs up, a Microsoft Entra ID (work or school) account from its ID token, and joins it to the organization that set up single sign-on for its tenant. Personal Microsoft accounts are refused.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		MicrosoftSigninCred	true	"request body"
//	@Success		202		{string}	string				"JWT token"
//	@Failure		400		{object}	error				"Token not valid"
//	@Failure		401		{object}	error				"Sign-in name held by another Microsoft account"
//	@Failure		404		{object}	error				"Microsoft sign-in is not configured"
//	@Router			/signin/microsoft [post]
func MicrosoftSignin(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	if len(ctx.Settings.MicrosoftClientIDs) == 0 {
		return http.StatusNotFound, errors.New("microsoft sign-in is not configured")
	}
	var cred MicrosoftSigninCred
	if err := json.NewDecoder(r.Body).Decode(&cred); err != nil {
		return http.StatusBadRequest, err
	}
	identity, err := oauth.VerifyMicrosoftIDToken(r.Context(), cred.IdToken, ctx.Settings.MicrosoftClientIDs)
	if err != nil {
		return http.StatusBadRequest, err
	}
	user, err := oauth.MicrosoftAuthenticate(identity)
	if errors.Is(err, oauth.ErrMicrosoftAccountMismatch) {
		loging.Logger.Warnw("microsoft sign-in under a name another identity holds",
			"username", identity.Username, "tenant", identity.TenantID)
		return http.StatusUnauthorized, err
	}
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return finishProviderSignin(w, r, ctx, user, models.SSOProviderMicrosoft, identity.TenantID)
}

// finishProviderSignin is the end every provider sign-in shares: refuse an
// inactive account, join the organization that set up sign-on for
// [identifier] (a Workspace domain, a tenant), and issue the token.
func finishProviderSignin(w http.ResponseWriter, r *http.Request, ctx *request_context.Context,
	user models.User, provider, identifier string) (int, error) {
	if user.Status == constants.UserStatusInactive {
		return http.StatusForbidden, errors.New("account_inactive")
	}
	// Failing to join is not failing to sign in.
	if _, err := actions.JoinOrgBySSODomain(r.Context(), &user, provider, identifier); err != nil {
		loging.Logger.Errorw("sso join failed", "user_id", user.ID, "provider", provider, "error", err.Error())
	}
	// The token carries the user's organizations, the one just joined among them.
	if err := models.Dbcon.Preload("Roles").First(&user, "id = ?", user.ID).Error; err != nil {
		return http.StatusInternalServerError, err
	}
	sessionId, err := ctx.SessionStore.CreateSession(user.ID, user.Username, r.UserAgent(), clientIP(r), constants.SESSION_EXPIRY)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return printToken(w, r, &user, sessionId)
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
	sessionId, err := ctx.SessionStore.CreateSession(user.ID, user.Username, r.UserAgent(), clientIP(r), constants.SESSION_EXPIRY)
	if err != nil {
		loging.Logger.Error("Error creating session", err)
		return http.StatusInternalServerError, err
	}
	return printToken(w, r, &user, sessionId)
	// return printToken(w, r, &user, &ctx.Settings)
}

func SignOut(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	// Get session ID from context (set by middleware)
	sessionID := ctx.Auth.ID

	// Revoke the session
	if err := ctx.SessionStore.RevokeSession(sessionID); err != nil {
		loging.Logger.Error("Error revoking session", err)
		return 0, nil
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "Logged out successfully"}); err != nil {
		return http.StatusInternalServerError, err
	}

	return 0, nil
}

func RenewToken(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	user, _ := ctx.GetCurrentUserModel()

	return printToken(w, r, user, ctx.Auth.ID)
}

func printToken(w http.ResponseWriter, _ *http.Request, user *models.User, sessionId string) (int, error) {
	signed, err := jwtops.SignJWT(user, sessionId)
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
