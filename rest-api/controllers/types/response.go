package types

type SimpleResponse struct {
	Message string `json:"message" example:"message"`
}

type AuthorizeResponse struct {
	Status bool `json:"status"`
}
