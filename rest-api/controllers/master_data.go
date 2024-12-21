package controllers

import (
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/request_context"
	"encoding/json"
	"net/http"
	"slices"
)

// @Summary Get resources
// @Tags permissions
// @Accept json
// @Produce json
// @Param 		 X-Auth header string true "Authorization"
// @Security 	 JWTAuth
// @Success 200 {object} []string
// @Security 	 JWTAuth
// @Router /master-data/resources [get]
func GetResources(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	loging.Logger.Debugln(ctx.Context.Value(permission_cache.UserPerm("userPerm")))
	loging.Logger.Debugln("GetResources")

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(models.Resources)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// @Summary Get scopes
// @Tags permissions
// @Accept json
// @Produce json
// @Param 		 X-Auth header string true "Authorization"
// @Security 	 JWTAuth
// @Success 200 {object} []string
// @Security 	 JWTAuth
// @Router /master-data/scopes [get]
func GetScopes(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	loging.Logger.Debugln("GetScopes")
	currentScope, _ := ctx.GetCurrentScope()
	scopes := models.Scopes
	if *currentScope != models.ScopeAll {
		scopes = slices.DeleteFunc(scopes, func(s models.Scope) bool {
			return s == models.ScopeAll
		})
	}
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(scopes)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// @Summary Get actions
// @Tags permissions
// @Accept json
// @Produce json
// @Param 		 X-Auth header string true "Authorization"
// @Security 	 JWTAuth
// @Success 200 {object} []string
// @Security 	 JWTAuth
// @Router /master-data/actions [get]
func GetActions(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	loging.Logger.Debugln("GetActions")
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(models.Actions)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}
