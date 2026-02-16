package controllers

import (
	"bigbucks/solution/auth/constants"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/request_context"
	webauthnservice "bigbucks/solution/auth/webauthn"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/gorilla/mux"
)

// webAuthnService is the shared WebAuthn service instance, set during route initialization.
var webAuthnService *webauthnservice.Service

// SetWebAuthnService sets the WebAuthn service (called from route setup).
func SetWebAuthnService(svc *webauthnservice.Service) {
	webAuthnService = svc
}

// ---- Registration ----

// BeginWebAuthnRegistration godoc
//
//	@Summary		Begin WebAuthn credential registration
//	@Description	Starts the WebAuthn registration ceremony for the authenticated user. Returns a CredentialCreationOptions JSON for the browser.
//	@Tags			webauthn
//	@Produce		json
//	@Success		200	{object}	protocol.CredentialCreation	"WebAuthn creation options"
//	@Failure		401	{object}	error						"Unauthorized"
//	@Failure		500	{object}	error						"Internal server error"
//	@Security		JWTAuth
//	@Router			/webauthn/register/begin [post]
func BeginWebAuthnRegistration(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	user, err := ctx.GetCurrentUserModel()
	if err != nil || user == nil {
		return http.StatusUnauthorized, err
	}

	waUser, err := models.LoadWebAuthnUserFromModel(user)
	if err != nil {
		loging.Logger.Error("Failed to load WebAuthn user", err)
		return http.StatusInternalServerError, err
	}

	options, err := webAuthnService.BeginRegistration(waUser)
	if err != nil {
		loging.Logger.Error("BeginRegistration failed", err)
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(options); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// FinishWebAuthnRegistration godoc
//
//	@Summary		Finish WebAuthn credential registration
//	@Description	Completes the registration ceremony by validating the authenticator response and storing the credential.
//	@Tags			webauthn
//	@Accept			json
//	@Produce		json
//	@Param			name	query		string	false	"Friendly name for the credential"
//	@Success		200		{object}	map[string]interface{}	"Credential registered"
//	@Failure		400		{object}	error	"Bad request"
//	@Failure		401		{object}	error	"Unauthorized"
//	@Failure		500		{object}	error	"Internal server error"
//	@Security		JWTAuth
//	@Router			/webauthn/register/finish [post]
func FinishWebAuthnRegistration(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	user, err := ctx.GetCurrentUserModel()
	if err != nil || user == nil {
		return http.StatusUnauthorized, err
	}

	waUser, err := models.LoadWebAuthnUserFromModel(user)
	if err != nil {
		loging.Logger.Error("Failed to load WebAuthn user", err)
		return http.StatusInternalServerError, err
	}

	// Parse the credential creation response from the browser
	//extract credential property from request json body

	parsedResponse, err := protocol.ParseCredentialCreationResponseBody(r.Body)
	if err != nil {
		loging.Logger.Error("Failed to parse credential creation response", err, r.Body)
		return http.StatusBadRequest, err
	}

	credentialName := r.URL.Query().Get("name")
	if credentialName == "" {
		credentialName = "My Passkey"
	}

	dbCred, err := webAuthnService.FinishRegistration(waUser, credentialName, parsedResponse)
	if err != nil {
		loging.Logger.Error("FinishRegistration failed", err)
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"message":      "Credential registered successfully",
		"credentialId": dbCred.ID,
		"name":         dbCred.Name,
	}); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// ---- Authentication (Login) ----

type webAuthnLoginRequest struct {
	Username  string `json:"username"`
	Mediation string `json:"mediation,omitempty"`
}

// BeginWebAuthnLogin godoc
//
//	@Summary		Begin WebAuthn login
//	@Description	Starts the WebAuthn authentication ceremony. Pass username for credential-bound login, or omit for discoverable (passkey) login.
//	@Tags			webauthn
//	@Accept			json
//	@Produce		json
//	@Param			request	body		webAuthnLoginRequest	false	"Optional username"
//	@Success		200		{object}	protocol.CredentialAssertion	"WebAuthn assertion options"
//	@Failure		400		{object}	error	"Bad request"
//	@Failure		500		{object}	error	"Internal server error"
//	@Router			/webauthn/login/begin [post]
func BeginWebAuthnLogin(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	var req webAuthnLoginRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return http.StatusBadRequest, err
		}
	}

	options, err := webAuthnService.BeginMediatedLogin(req.Username, req.Mediation)
	if err != nil {
		loging.Logger.Error("BeginLogin failed", err)
		return http.StatusBadRequest, err
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(options); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// FinishWebAuthnLogin godoc
//
//	@Summary		Finish WebAuthn login
//	@Description	Completes the authentication ceremony, validates the authenticator response, and issues a JWT.
//	@Tags			webauthn
//	@Accept			json
//	@Produce		json
//	@Param			username	query		string	false	"Username (must match begin request)"
//	@Success		202			{string}	string	"JWT token"
//	@Failure		400			{object}	error	"Bad request"
//	@Failure		401			{object}	error	"Unauthorized"
//	@Failure		500			{object}	error	"Internal server error"
//	@Router			/webauthn/login/finish [post]
func FinishWebAuthnLogin(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	username := r.URL.Query().Get("username")

	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(r.Body)
	if err != nil {
		loging.Logger.Error("Failed to parse credential request response", err)
		return http.StatusBadRequest, err
	}

	user, err := webAuthnService.FinishLogin(username, parsedResponse)
	if err != nil {
		loging.Logger.Error("FinishLogin failed", err)
		return http.StatusUnauthorized, err
	}

	// Create session and issue JWT — same flow as password signin
	userAgent := r.UserAgent()
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
	}

	sessionID, err := ctx.SessionStore.CreateSession(user.ID, user.Username, userAgent, ip, constants.SESSION_EXPIRY)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	go func() {
		if err := user.LogLoginActivity(map[string]interface{}{
			"ip":         ip,
			"user_agent": userAgent,
			"method":     "webauthn",
		}); err != nil {
			loging.Logger.Error("Error logging login activity", err)
		}
	}()

	return printToken(w, r, user, sessionID)
}

// ---- Credential Management ----

// ListWebAuthnCredentials godoc
//
//	@Summary		List WebAuthn credentials
//	@Description	Returns all registered WebAuthn credentials for the authenticated user.
//	@Tags			webauthn
//	@Produce		json
//	@Success		200	{array}		map[string]interface{}	"List of credentials"
//	@Failure		401	{object}	error					"Unauthorized"
//	@Failure		500	{object}	error					"Internal server error"
//	@Security		JWTAuth
//	@Router			/webauthn/credentials [get]
func ListWebAuthnCredentialsCtrl(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	user, err := ctx.GetCurrentUserModel()
	if err != nil || user == nil {
		return http.StatusUnauthorized, err
	}

	creds, err := models.ListWebAuthnCredentials(user.ID)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	type credentialResponse struct {
		ID           uint   `json:"id"`
		Name         string `json:"name"`
		CreatedAt    string `json:"createdAt"`
		Discoverable bool   `json:"discoverable"`
		Transport    string `json:"transport"`
	}

	var resp []credentialResponse
	for _, c := range creds {
		resp = append(resp, credentialResponse{
			ID:           c.ID,
			Name:         c.Name,
			CreatedAt:    c.CreatedAt.Format("2006-01-02T15:04:05Z"),
			Discoverable: c.Discoverable,
			Transport:    c.Transport,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// DeleteWebAuthnCredential godoc
//
//	@Summary		Delete a WebAuthn credential
//	@Description	Removes a registered WebAuthn credential by ID.
//	@Tags			webauthn
//	@Param			credential_id	path		int		true	"Credential ID"
//	@Success		200				{object}	map[string]string	"Deleted"
//	@Failure		400				{object}	error				"Bad request"
//	@Failure		401				{object}	error				"Unauthorized"
//	@Failure		404				{object}	error				"Not found"
//	@Security		JWTAuth
//	@Router			/webauthn/credentials/{credential_id} [delete]
func DeleteWebAuthnCredential(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	user, err := ctx.GetCurrentUserModel()
	if err != nil || user == nil {
		return http.StatusUnauthorized, err
	}

	vars := mux.Vars(r)
	credIDStr := vars["credential_id"]
	credID, err := strconv.ParseUint(credIDStr, 10, 64)
	if err != nil {
		return http.StatusBadRequest, err
	}

	if err := models.DeleteWebAuthnCredential(uint(credID), user.ID); err != nil {
		return http.StatusNotFound, err
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "Credential deleted successfully",
	}); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// HasWebAuthnCredentials godoc
//
//	@Summary		Check if user has WebAuthn credentials
//	@Description	Returns whether a given username has WebAuthn credentials registered (for login UI flow decisions).
//	@Tags			webauthn
//	@Produce		json
//	@Param			username	query		string	true	"Username to check"
//	@Success		200			{object}	map[string]bool	"has_credentials"
//	@Failure		400			{object}	error			"Bad request"
//	@Router			/webauthn/check [get]
func HasWebAuthnCredentials(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	username := r.URL.Query().Get("username")
	if username == "" {
		return http.StatusBadRequest, nil
	}

	user, err := models.LoadWebAuthnUserByUsername(username)
	if err != nil {
		// Don't reveal whether user exists — just say no credentials
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"has_credentials": false})
		return 0, nil
	}

	hasCredentials := len(user.WebAuthnCredentials()) > 0
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]bool{"has_credentials": hasCredentials}); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}
