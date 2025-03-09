package types

import "encoding/json"

type Role struct {
	Name        string          `json:"name" validate:"required"`
	Description string          `json:"description"`
	ExtraAttrs  json.RawMessage `json:"extraAttrs"`
}

type CheckPermissionBody struct {
	Scope    string
	Resource string
	Action   string
	OrgID    int
}

type CreatePermissionBody struct {
	Resource string `validate:"required,valid_resources,alphanum_,min=3"`
	Scope    string `validate:"required,valid_scopes,alphanum_,min=3"`
	Action   string ` validate:"required,valid_actions,alphanum_,min=3"`
}

type SignupRequestBody struct {
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=6"`
	FirstName string `json:"firstName" validate:"required"`
	LastName  string `json:"lastName" validate:"required"`
}

type RolePermissionBindingBody struct {
	Resource string `json:"resource"`
	Scope    string `json:"scope"`
	Action   string `json:"action"`
	RoleId   string `json:"role_id"`
}

type UserRoleBindingBody struct {
	RoleID   string `json:"role_id" validate:"required"`
	UserName string `json:"user_name" validate:"required"`
	OrgID    string `json:"org_id"`
}
