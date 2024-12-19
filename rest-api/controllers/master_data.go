package controllers

import (
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/request_context"
	"encoding/json"
	"net/http"
)

// @Summary Get resources
// @Tags permissions
// @Accept json
// @Produce json
// @Param 		 X-Auth header string true "Authorization"
// @Security 	 JWTAuth
// @Success 200 {object} []string
// @Security 	 JWTAuth
// @Router /resources [get]
func GetResources(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	loging.Logger.Debugln("GetResources")

	resources := []string{"users", "roles", "permissions", "organizations"}
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(resources)
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
// @Router /scopes [get]
func GetScopes(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	loging.Logger.Debugln("GetScopes")
	scopes := []string{"own", "org", "all"}
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
// @Router /actions [get]
func GetActions(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	loging.Logger.Debugln("GetActions")
	actions := []string{"write", "update", "delete", "create", "read"}
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(actions)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}
