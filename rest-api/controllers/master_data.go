package controllers

import (
	"bigbucks/solution/auth/constants"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/request_context"
	"bigbucks/solution/auth/settings"
	"encoding/json"
	"net/http"
	"slices"
)

// @Summary	Get resources
// @Tags		permissions
// @Accept		json
// @Produce	json
// @Param		X-Auth	header	string	true	"Authorization"
// @Success	200	{object}	[]string
// @Router		/master-data/resources [get]
func GetResources(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	loging.Logger.Debugln(ctx.Context.Value(permission_cache.UserPerm))
	loging.Logger.Debugln("GetResources")

	w.Header().Set("Content-Type", "application/json")
	// The same list the bind path validates against: anything offered here has
	// to be bindable, and nothing else should be offered.
	err := json.NewEncoder(w).Encode(settings.GrantableResources())
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// @Summary	Get scopes
// @Tags		permissions
// @Accept		json
// @Produce	json
// @Param		X-Auth	header	string	true	"Authorization"
// @Success	200	{object}	[]string
// @Router		/master-data/scopes [get]
func GetScopes(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	loging.Logger.Debugln("GetScopes")
	currentScope, _ := ctx.GetCurrentScope()
	// Cloned, not aliased: slices.DeleteFunc edits its argument's backing array
	// in place, so narrowing `constants.Scopes` directly would drop "all" from
	// the package-level list for the rest of the process — every later scope
	// check, including the one BindPermission now makes, would reject it.
	scopes := slices.Clone(constants.Scopes)
	if *currentScope != constants.ScopeAll {
		scopes = slices.DeleteFunc(scopes, func(s constants.Scope) bool {
			return s == constants.ScopeAll
		})
	}
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(scopes)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// @Summary	Get actions
// @Tags		permissions
// @Accept		json
// @Produce	json
// @Param		X-Auth	header	string	true	"Authorization"
// @Success	200	{object}	[]string
// @Router		/master-data/actions [get]
func GetActions(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	loging.Logger.Debugln("GetActions")
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(constants.Actions)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}
