package controllers

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/request_context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

func GetOrg(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])
	org, _, _ := models.GetOrganization(id)
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(org)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

func CreateOrg(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	var org actions.Organization
	err := json.NewDecoder(r.Body).Decode(&org)
	if err != nil {
		return http.StatusBadRequest, err
	}
	loging.Logger.Debug("User ID", ctx.Auth)

	code, err := actions.CreateOrganisationFromAuthenticatedUser(&org, ctx.Auth.User.Username, ctx.PermCache, ctx.Context)
	return code, err
}
