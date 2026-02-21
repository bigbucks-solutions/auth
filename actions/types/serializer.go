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
	IsLocked bool   `json:"isLocked"`
	IsHidden bool   `json:"isHidden"`
}

type RoleWithId struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ListUserResponse struct {
	ID             string               `json:"id"`
	Username       string               `json:"username"`
	LastLogin      string               `json:"lastLogin"`
	ActiveSessions int64                `json:"activeSessions"`
	Roles          []RoleWithId         `json:"roles"`
	Status         constants.UserStatus `json:"status"`
	Firstname      string               `json:"firstName"`
	Lastname       string               `json:"lastName"`
	Email          string               `json:"email"`
}

type Role struct {
	Name         string `json:"name"`
	IsSystemRole bool   `json:"is_system_role"`
}

type SystemRoleConfig struct {
	RoleName    string                   `json:"role_name"`
	Description string                   `json:"description"`
	Permissions []SystemPermissionConfig `json:"permissions"`
}

type SystemPermissionConfig struct {
	Resource    string `json:"resource"`
	Scope       string `json:"scope"`
	Action      string `json:"action"`
	IsHidden    bool   `json:"is_hidden"`
	Description string `json:"description"`
}

type UserInfo struct {
	Username      string                 `json:"username"`
	Roles         []*UserInfoRole        `json:"roles,omitempty"`
	Profile       UserInfoProfile        `json:"profile,omitempty"`
	IsSocial      bool                   `json:"isSocialAccount"`
	Organizations []UserInfoOrganization `json:"organizations,omitempty"`
}

type UserInfoOrganization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type UserInfoRole struct {
	Name        string          `json:"name" validate:"required"`
	Description string          `json:"description"`
	ExtraAttrs  json.RawMessage `json:"extraAttrs"`
}

type UserInfoProfile struct {
	Firstname   string  `json:"firstName"`
	Lastname    string  `json:"lastName"`
	Phone       string  `json:"phone"`
	Email       string  `json:"email"`
	Picture     *string `json:"avatar"`
	Bio         string  `json:"bio"`
	Designation string  `json:"designation"`
	Country     string  `json:"country"`
	Timezone    string  `json:"timezone"`
}
