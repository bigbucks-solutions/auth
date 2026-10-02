package types

import (
	"bigbucks/solution/auth/actions/types"
	"bigbucks/solution/auth/permission_cache"
)

type SimpleResponse struct {
	Message string `json:"message" example:"message"`
}

type AuthorizeResponse struct {
	Status bool `json:"status"`
}

// MyPermissionsResponse is what a client reads to decide what to put on a
// screen: the grants the caller holds in one organization, as held — the
// caller expands scope and action implications the way enforcement does.
type MyPermissionsResponse struct {
	OrgID       string                              `json:"orgId"`
	Roles       []string                            `json:"roles"`
	Permissions []permission_cache.PermissionTriple `json:"permissions"`
}

type UserInfo = types.UserInfo
type Organization = types.UserInfoOrganization
type Role = types.UserInfoRole
type Profile = types.UserInfoProfile

type ListRolesPagedResponse struct {
	Roles []types.ListRoleResponse `json:"roles"`
	Total int64                    `json:"total"`
	Page  int                      `json:"page"`
	Size  int                      `json:"size"`
}
