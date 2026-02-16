package types

import "bigbucks/solution/auth/actions/types"

type SimpleResponse struct {
	Message string `json:"message" example:"message"`
}

type AuthorizeResponse struct {
	Status bool `json:"status"`
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
