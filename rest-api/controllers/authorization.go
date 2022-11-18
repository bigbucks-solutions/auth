package controllers

import (
	"bigbucks/solution/auth/rest-api/controllers/types"
	"bigbucks/solution/auth/settings"
	"encoding/json"
	"net/http"
)

// Authorization godoc
// @Summary      Check user have permission
// @Tags         auth
// @Accept       json
// @Param        request  body  types.CheckPermissionBody  true  "request body"
// @Param 		 X-Auth header string true "Authorization"
// @Security 	 JWTAuth
// @Produce      json
// @Success      200  {object}  types.AuthorizeResponse  ""
// @Failure      400  ""
// @Failure      500  ""
// @Router       /user/authorize [post]
func Authorize(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	var body = &types.CheckPermissionBody{}
	json.NewDecoder(r.Body).Decode(&body)
	user, _ := ctx.GetCurrentUserModel()
	status, _ := user.Authorize(body.Permission, body.Resource, body.OrgID)
	json.NewEncoder(w).Encode(&types.AuthorizeResponse{Status: status})
	return 0, nil
}
