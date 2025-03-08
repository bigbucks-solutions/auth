package types

type User struct {
	ID       string      `json:"id"`
	Username string      `json:"username"`
	Roles    interface{} `json:"roles,omitempty"`
	Profile  interface{} `json:"profile,omitempty"`
	IsSocial bool        `json:"isSocialAccount"`
}

type Profile struct {
	Firstname string  `json:"firstName"`
	Lastname  string  `json:"lastName"`
	Phone     string  `json:"phone"`
	Email     string  `json:"email"`
	Picture   *string `json:"avatar"`
}

type Role struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Permission struct {
	Resource string `json:"resource"`
	Scope    string `json:"scope"`
	Action   string `json:"action"`
}
