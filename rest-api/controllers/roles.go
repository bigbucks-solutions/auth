package controllers

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/constants"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/request_context"
	"bigbucks/solution/auth/rest-api/controllers/types"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
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
// @Success 200 {object} []models.Role
// @Security 	 JWTAuth
// @Router /roles [get]
func ListRoles(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 {
		pageSize = 10
	}

	roleName := r.URL.Query().Get("role_name")
	orgID := ctx.CurrentOrgID

	roles, total, err := actions.ListRoles(page, pageSize, roleName, orgID)
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
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		return http.StatusInternalServerError, err
	}
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
func CreateRole(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	var role types.Role
	err := json.NewDecoder(r.Body).Decode(&role)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return http.StatusBadRequest, err
	}

	_, code, err := actions.CreateRole(&models.Role{Name: role.Name, Description: role.Description, OrgID: ctx.CurrentOrgID, ExtraAttrs: role.ExtraAttrs})
	if err != nil {
		return code, err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(role)
	if err != nil {
		return http.StatusInternalServerError, err
	}
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
func CreatePermission(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	var permission types.CreatePermissionBody
	err := json.NewDecoder(r.Body).Decode(&permission)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return http.StatusBadRequest, err
	}

	code, err := actions.CreatePermission(&models.Permission{Resource: permission.Resource, Scope: constants.Scope(permission.Scope), Action: constants.Action(permission.Action)})
	if err != nil {
		return code, err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(permission)
	if err != nil {
		return http.StatusInternalServerError, err
	}
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
func BindPermissionToRole(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	var binding types.RolePermissionBindingBody
	if err := json.NewDecoder(r.Body).Decode(&binding); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return http.StatusBadRequest, err
	}
	code, err := actions.BindPermission(
		binding.Resource,
		binding.Scope,
		binding.Action,
		binding.RoleId,
		ctx.CurrentOrgID,
		ctx.PermCache,
		ctx.Context,
	)
	if err != nil {
		return code, err
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]string{"message": "Permission bound successfully"})
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// @Summary UnBind permission to role
// @Description Removes a permission with a role
// @Tags roles
// @Accept json
// @Produce json
// @Param 		 X-Auth header string true "Authorization"
// @Security 	 JWTAuth
// @Param rolepermission body types.RolePermissionBindingBody true "UnBinding details"
// @Success 200 {string} string "Permission unbound successfully"
// @Router /roles/unbind-permission [post]
func UnBindPermissionToRole(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	var binding types.RolePermissionBindingBody
	if err := json.NewDecoder(r.Body).Decode(&binding); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return http.StatusBadRequest, err
	}
	code, err := actions.UnBindPermission(
		binding.Resource,
		binding.Scope,
		binding.Action,
		binding.RoleId,
		ctx.CurrentOrgID,
		ctx.PermCache,
		ctx.Context,
	)
	if err != nil {
		w.WriteHeader(code)
		return code, err
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]string{"message": "Permission unbound successfully"})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// @Summary Bind role to user
// @Description Associates a role with a user in an organization
// @Tags roles
// @Accept json
// @Produce json
// @Param        X-Auth header string true "Authorization"
// @Security     JWTAuth
// @Param binding body types.UserRoleBindingBody true "User role binding details"
// @Success 200 {string} string "Role bound to user successfully"
// @Router /roles/bind-user [post]
func BindRoleToUser(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	var binding types.UserRoleBindingBody
	if err := json.NewDecoder(r.Body).Decode(&binding); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return http.StatusBadRequest, err
	}
	// if binding.OrgID == 0 {
	// 	binding.OrgID = ctx.Auth.User.Roles[0].OrgID
	// }

	code, err := actions.BindUserRole(
		binding.UserID,
		binding.RoleID,
		binding.OrgID,
	)
	if err != nil {
		return code, err
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]string{"message": "Role bound to user successfully"})
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// @Summary UnBind role to user
// @Description Removes a role with a user in an organization
// @Tags roles
// @Accept json
// @Produce json
// @Param        X-Auth header string true "Authorization"
// @Security     JWTAuth
// @Param binding body types.UserRoleBindingBody true "User role binding details"
// @Success 200 {string} string "Role bound to user successfully"
// @Router /roles/unbind-user [post]
func UnBindRoleToUser(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	var binding types.UserRoleBindingBody
	if err := json.NewDecoder(r.Body).Decode(&binding); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return http.StatusBadRequest, err
	}
	code, err := actions.UnBindUserRole(
		binding.UserID,
		binding.RoleID,
		binding.OrgID,
	)
	if err != nil {
		return code, err
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]string{"message": "Role unbound to user successfully"})
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// @Summary List permission of a role
// @Description Lists permissions of a role
// @Tags roles
// @Accept json
// @Produce json
// @Param        X-Auth header string true "Authorization"
// @Security     JWTAuth
// @Success 200 {array} types.ListRolePermission
// @Router /roles/:role_id/permissions [post]
func ListPermissionsOfRole(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	role_id := mux.Vars(r)["role_id"]
	permissions, code, err := actions.ListRolePermission(role_id, ctx.CurrentOrgID, ctx.Context)
	if err != nil {
		return code, err
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(permissions)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}
