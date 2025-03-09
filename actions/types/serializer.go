package types

import "encoding/json"

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
