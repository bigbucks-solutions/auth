package controllers

import (
	"bigbucks/solution/auth/request_context"
	"bigbucks/solution/auth/rest-api/controllers/types"
	"encoding/json"
	"errors"
	"net/http"
)

// Authorize godoc
//
//	@Summary	Check user have permission
//	@Tags		auth
//	@Accept		json
//	@Param		request	body	types.CheckPermissionBody	true	"request body"
//	@Param		X-Auth	header	string						true	"Authorization"
//	@Produce	json
//	@Success	200	{object}	types.AuthorizeResponse	""
//	@Failure	400	""
//	@Failure	500	""
//	@Router		/user/authorize [post]
func Authorize(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	var body = &types.CheckPermissionBody{}
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		return http.StatusBadRequest, err
	}
	orgID := ctx.CurrentOrgID
	if orgID == "" {
		orgID = body.OrgID
	}
	status, _ := ctx.PermCache.CheckPermission(&ctx.Context, body.Resource, body.Scope, body.Action, orgID, &ctx.Auth.User)
	err = json.NewEncoder(w).Encode(&types.AuthorizeResponse{Status: status})
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// GetMyPermissions godoc
//
//	@Summary		List the caller's own permissions
//	@Description	Every grant the caller holds in the organization named by
//	@Description	X-Organization-Id, for a client deciding what to show. Grants
//	@Description	are returned as held: the caller expands scope and action
//	@Description	implications the same way enforcement does.
//	@Tags			auth
//	@Param			X-Organization-Id	header	string	true	"Organization"
//	@Produce		json
//	@Success		200	{object}	types.MyPermissionsResponse	""
//	@Failure		400	""
//	@Failure		401	""
//	@Failure		500	""
//	@Router			/me/permissions [get]
func GetMyPermissions(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	if ctx.CurrentOrgID == "" {
		return http.StatusBadRequest, errors.New("no organization in request")
	}

	permissions, roles, err := ctx.PermCache.ListEffectivePermissions(ctx.Context, ctx.CurrentOrgID, &ctx.Auth.User)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(&types.MyPermissionsResponse{
		OrgID:       ctx.CurrentOrgID,
		Roles:       roles,
		Permissions: permissions,
	}); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}
