package types

type UserInfo struct {
	Username string  `json:"username"`
	Roles    []*Role `json:"roles,omitempty"`
	Profile  Profile `json:"profile,omitempty"`
	IsSocial bool    `json:"isSocialAccount"`
}

type Profile struct {
	Firstname string  `json:"firstName"`
	Lastname  string  `json:"lastName"`
	Phone     string  `json:"phone"`
	Email     string  `json:"email"`
	Picture   *string `json:"avatar"`
}

type Role struct {
	Name string `json:"name"`
}

type CheckPermissionBody struct {
	Permission string
	Resource   string
	OrgID      int
}
