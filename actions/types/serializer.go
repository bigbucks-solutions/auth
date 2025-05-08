package types

import (
	"bigbucks/solution/auth/constants"
	"encoding/json"
)

type ListRoleResponse struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	UserCount   int64           `json:"userCount"`
	ExtraAttrs  json.RawMessage `json:"extraAttrs"`
}

type ListRolePermission struct {
	Resource string `json:"resource"`
	Scope    string `json:"scope"`
	Action   string `json:"action"`
}

type RoleWithId struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ListUserResponse struct {
	ID        string               `json:"id"`
	Username  string               `json:"username"`
	Roles     []RoleWithId         `json:"roles"`
	Status    constants.UserStatus `json:"status"`
	Firstname string               `json:"firstName"`
	Lastname  string               `json:"lastName"`
	Email     string               `json:"email"`
}
