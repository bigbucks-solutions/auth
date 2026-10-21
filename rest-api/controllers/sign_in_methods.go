package controllers

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/request_context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// SetPasswordRequest is a password for an account that has none.
type SetPasswordRequest struct {
	Password string `json:"password"`
}

// GetSignInMethods godoc
//
//	@Summary		List how the signed-in user can sign in
//	@Description	Whether the account has a password, the providers linked to it, and those it could link.
//	@Tags			auth
//	@Produce		json
//	@Param			X-Auth	header	string	true	"Authorization"
//	@Security		JWTAuth
//	@Success		200	{object}	actions.SignInMethods
//	@Router			/me/sign-in-methods [get]
func GetSignInMethods(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	user, err := ctx.GetCurrentUserModel()
	if err != nil {
		return http.StatusUnauthorized, err
	}
	methods, err := actions.GetSignInMethods(user.ID)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(methods); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// SetPassword godoc
//
//	@Summary		Set a password
//	@Description	Gives an account that signs in only through Google or Microsoft a password. An account that has one resets it instead.
//	@Tags			auth
//	@Accept			json
//	@Param			X-Auth	header	string				true	"Authorization"
//	@Param			request	body	SetPasswordRequest	true	"request body"
//	@Security		JWTAuth
//	@Success		204	""
//	@Failure		400	{object}	error	"Too short"
//	@Failure		409	{object}	error	"Already has a password"
//	@Router			/me/password [post]
func SetPassword(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	user, err := ctx.GetCurrentUserModel()
	if err != nil {
		return http.StatusUnauthorized, err
	}
	var body SetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, err
	}
	if status, err := actions.SetPassword(user.ID, body.Password); err != nil {
		return status, err
	}
	w.WriteHeader(http.StatusNoContent)
	return 0, nil
}

// UnlinkProvider godoc
//
//	@Summary		Unlink a sign-in provider
//	@Description	Stops the provider signing in to the account. Refused while it is the only way left in.
//	@Tags			auth
//	@Param			X-Auth		header	string	true	"Authorization"
//	@Param			provider	path	string	true	"google or microsoft"
//	@Security		JWTAuth
//	@Success		204	""
//	@Failure		404	{object}	error	"Not linked"
//	@Failure		409	{object}	error	"The only sign-in method left"
//	@Router			/me/sign-in-methods/{provider} [delete]
func UnlinkProvider(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	user, err := ctx.GetCurrentUserModel()
	if err != nil {
		return http.StatusUnauthorized, err
	}
	if status, err := actions.UnlinkProvider(user.ID, mux.Vars(r)["provider"]); err != nil {
		return status, err
	}
	w.WriteHeader(http.StatusNoContent)
	return 0, nil
}
