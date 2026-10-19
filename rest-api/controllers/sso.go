package controllers

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/request_context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
)

// ssoProvider is the provider a /sso/{provider} route names.
func ssoProvider(r *http.Request) (string, bool) {
	provider := mux.Vars(r)["provider"]
	return provider, actions.IsSSOProvider(provider)
}

// ListOrgSSO godoc
//
//	@Summary		List the organization's single sign-on
//	@Description	Every provider, set up or not, with whether this deployment can sign in with it.
//	@Tags			sso
//	@Produce		json
//	@Param			X-Auth				header	string	true	"Authorization"
//	@Param			X-Organization-Id	header	string	true	"Organization ID"
//	@Security		JWTAuth
//	@Success		200	{array}	actions.OrgSSO
//	@Router			/sso [get]
func ListOrgSSO(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	user, err := ctx.GetCurrentUserModel()
	if err != nil {
		return http.StatusUnauthorized, err
	}
	views, err := actions.ListOrgSSO(ctx.CurrentOrgID, user.ID)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(views); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

func writeOrgSSO(w http.ResponseWriter, view *actions.OrgSSO) (int, error) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(view); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// GetOrgSSO godoc
//
//	@Summary		Get the organization's single sign-on
//	@Description	The domain whose sign-ins join the organization, the role they are given, and the domain of the caller's own Google account.
//	@Tags			sso
//	@Produce		json
//	@Param			X-Auth				header	string	true	"Authorization"
//	@Param			X-Organization-Id	header	string	true	"Organization ID"
//	@Param			provider			path	string	true	"Provider (google, microsoft)"
//	@Security		JWTAuth
//	@Success		200	{object}	actions.OrgSSO
//	@Failure		403	{object}	error
//	@Failure		404	{object}	error
//	@Router			/sso/{provider} [get]
func GetOrgSSO(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	provider, ok := ssoProvider(r)
	if !ok {
		return http.StatusNotFound, errors.New("unknown sso provider")
	}
	user, err := ctx.GetCurrentUserModel()
	if err != nil {
		return http.StatusUnauthorized, err
	}
	view, err := actions.GetOrgSSO(ctx.CurrentOrgID, provider, user.ID)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return writeOrgSSO(w, view)
}

// SaveOrgSSO godoc
//
//	@Summary		Set up the organization's single sign-on
//	@Description	Sign-ins on the domain join the organization with the role. Taking a new domain needs the caller's own Google account to be on it.
//	@Tags			sso
//	@Accept			json
//	@Produce		json
//	@Param			X-Auth				header	string						true	"Authorization"
//	@Param			X-Organization-Id	header	string						true	"Organization ID"
//	@Param			provider			path	string						true	"Provider (google, microsoft)"
//	@Param			request				body	actions.SaveOrgSSORequest	true	"request body"
//	@Security		JWTAuth
//	@Success		200	{object}	actions.OrgSSO
//	@Failure		400	{object}	error	"Bad domain or role"
//	@Failure		403	{object}	error	"Domain not proved"
//	@Failure		409	{object}	error	"Domain taken by another organization"
//	@Router			/sso/{provider} [put]
func SaveOrgSSO(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	provider, ok := ssoProvider(r)
	if !ok {
		return http.StatusNotFound, errors.New("unknown sso provider")
	}
	var req actions.SaveOrgSSORequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, err
	}
	user, err := ctx.GetCurrentUserModel()
	if err != nil {
		return http.StatusUnauthorized, err
	}
	view, status, err := actions.SaveOrgSSO(ctx.CurrentOrgID, provider, user.ID, req)
	if err != nil {
		return status, err
	}
	return writeOrgSSO(w, view)
}

// DeleteOrgSSO godoc
//
//	@Summary		Turn off the organization's single sign-on
//	@Description	Releases the domain. People who joined through it stay members.
//	@Tags			sso
//	@Param			X-Auth				header	string	true	"Authorization"
//	@Param			X-Organization-Id	header	string	true	"Organization ID"
//	@Param			provider			path	string	true	"Provider (google, microsoft)"
//	@Security		JWTAuth
//	@Success		204
//	@Router			/sso/{provider} [delete]
func DeleteOrgSSO(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	provider, ok := ssoProvider(r)
	if !ok {
		return http.StatusNotFound, errors.New("unknown sso provider")
	}
	if err := actions.DeleteOrgSSO(ctx.CurrentOrgID, provider); err != nil {
		return http.StatusInternalServerError, err
	}
	w.WriteHeader(http.StatusNoContent)
	return 0, nil
}
