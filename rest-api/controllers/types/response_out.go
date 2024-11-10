package types

type SimpleResponse struct {
	Message string `json:"message" example:"message"`
}

type AuthorizeResponse struct {
	Status bool `json:"status"`
}

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
