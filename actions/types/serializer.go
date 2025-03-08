package types

type ListRoleResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	UserCount   int64  `json:"userCount"`
}

type ListRolePermission struct {
	Resource string `json:"resource"`
	Scope    string `json:"scope"`
	Action   string `json:"action"`
}
