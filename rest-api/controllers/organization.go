package controllers

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/request_context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// GetOrg returns complete organization details to an authenticated member of
// that organization.
//
//	@Summary	Get organization details
//	@Description	Gets complete organization details, including its users. The authenticated user must belong to the requested organization.
//	@Tags		auth
//	@Produce	json
//	@Param		X-Auth	header	string	true	"Authorization"
//	@Security	JWTAuth
//	@Param		org_id	path	string	true	"Organization ID"
//	@Success	200	{object}	models.OrganizationDetails
//	@Failure	401	{object}	error
//	@Failure	403	{object}	error
//	@Failure	404	{object}	error
//	@Router		/organizations/{org_id} [get]
func GetOrg(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	vars := mux.Vars(r)
	orgID := vars["org_id"]
	org, err := models.GetOrganization(orgID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return http.StatusNotFound, errors.New("organization not found")
	}
	if err != nil {
		return http.StatusInternalServerError, err
	}

	isMember, err := models.IsOrganizationMember(orgID, ctx.Auth.User.Username)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !isMember {
		return http.StatusForbidden, errors.New("forbidden")
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(org.Details())
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// Create Organisation godoc
//
//	@Summary		Create a new organization
//	@Description	Create a new organization. Accepts either application/json with a logo_url string, or multipart/form-data with an optional logo file upload.
//	@Tags			auth
//	@Accept			mpfd
//	@Accept			json
//	@Produce		json
//	@Param			X-Auth		header	string	true	"Authorization"
//	@Security		JWTAuth
//	@Param			request		body		actions.Organization	false	"Organization details (JSON)"
//	@Param			name		formData	string	false	"Organization name"
//	@Param			email		formData	string	false	"Contact email"
//	@Param			phone		formData	string	false	"Contact phone"
//	@Param			address		formData	string	false	"Address"
//	@Param			city		formData	string	false	"City"
//	@Param			postal_code	formData	string	false	"Postal code"
//	@Param			state		formData	string	false	"State or county"
//	@Param			country		formData	string	false	"Country"
//	@Param			latitude	formData	number	false	"Latitude"
//	@Param			longitude	formData	number	false	"Longitude"
//	@Param			logo_url	formData	string	false	"Logo URL (alternative to file upload)"
//	@Param			logo		formData	file	false	"Logo image file"
//	@Param			website		formData	string	false	"Website URL"
//	@Param			description	formData	string	false	"Company description"
//	@Param			tax_id		formData	string	false	"Tax ID"
//	@Success		200		{object}	models.Organization	"Created organization details"
//	@Failure		400		{object}	error					"Bad request"
//	@Failure		401		{object}	error					"Unauthorized"
//	@Failure		500		{object}	error					"Internal server error"
//	@Router			/organizations [post]
func CreateOrg(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	org, code, err := actions.OrganizationFromRequest(r)
	if err != nil {
		return code, err
	}
	loging.Logger.Debug("User ID", ctx.Auth)
	return actions.CreateOrganisationFromAuthenticatedUser(org, ctx.Auth.User.Username, ctx.PermCache, ctx.Context)
}
