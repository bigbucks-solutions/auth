package controllers

import (
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/rest-api/controllers/types"
	"bigbucks/solution/auth/settings"
	"encoding/json"
	"net/http"
	"strconv"
)

// @Summary List roles
// @Description Get paginated list of roles with user count
// @Tags roles
// @Accept json
// @Produce json
// @Param 		 X-Auth header string true "Authorization"
// @Security 	 JWTAuth
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param role_name query string false "Filter by role name"
// @Param org_id query int false "Filter by organization ID"
// @Success 200 {object} []models.Role
// @Security 	 JWTAuth
// @Router /roles [get]
func ListRoles(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 {
		pageSize = 10
	}

	roleName := r.URL.Query().Get("role_name")
	orgID, _ := strconv.Atoi(r.URL.Query().Get("org_id"))

	roles, total, err := models.ListRoles(page, pageSize, roleName, orgID)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	response := map[string]interface{}{
		"roles": roles,
		"total": total,
		"page":  page,
		"size":  pageSize,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	return 0, nil
}

// @Summary Create new role
// @Description Create a new role in the system
// @Tags roles
// @Accept json
// @Param 		 X-Auth header string true "Authorization"
// @Security 	 JWTAuth
// @Produce json
// @Param role body types.Role true "Role object"
// @Success 201
// @Router /roles [post]
func CreateRole(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	var role types.Role
	err := json.NewDecoder(r.Body).Decode(&role)
	if err != nil {
		return http.StatusBadRequest, err
	}

	code, err := models.CreateRole(&models.Role{Name: role.Name, Description: role.Description})
	if err != nil {
		return code, err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(role)
	return 0, nil
}

// @Summary Create new permission
// @Description Create a new permission in the system
// @Tags permissions
// @Accept json
// @Param 		 X-Auth header string true "Authorization"
// @Security 	 JWTAuth
// @Produce json
// @Param permission body types.CreatePermissionBody true "Permission object"
// @Success 201
// @Router /permissions [post]
func CreatePermission(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	var permission types.CreatePermissionBody
	err := json.NewDecoder(r.Body).Decode(&permission)
	if err != nil {
		return http.StatusBadRequest, err
	}

	code, err := models.CreatePermission(&models.Permission{Resource: permission.Resource, Scope: models.Scope(permission.Scope), Action: permission.Action})
	if err != nil {
		return code, err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(permission)
	return 0, nil
}

// @Summary Bind permission to role
// @Description Associates a permission with a role
// @Tags roles
// @Accept json
// @Produce json
// @Param 		 X-Auth header string true "Authorization"
// @Security 	 JWTAuth
// @Param rolepermission body types.RolePermissionBindingBody true "Binding details"
// @Success 200 {string} string "Permission bound successfully"
// @Router /roles/bind-permission [post]
func BindPermissionToRole(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	var binding types.RolePermissionBindingBody
	if err := json.NewDecoder(r.Body).Decode(&binding); err != nil {
		return http.StatusBadRequest, err
	}

	code, err := models.BindPermission(
		binding.ResourceName,
		binding.Scope,
		binding.ActionName,
		binding.RoleKey,
		ctx.Auth.User.Roles[0].OrgID,
	)
	if err != nil {
		return code, err
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Permission bound successfully"})
	return 0, nil
}
