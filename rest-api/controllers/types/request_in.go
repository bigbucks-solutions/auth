package types

type Role struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
}

type CheckPermissionBody struct {
	Scope    string
	Resource string
	Action   string
	OrgID    int
}

type CreatePermissionBody struct {
	Resource string `validate:"required,alphanum_,min=3"`
	Scope    string `validate:"required,oneof=own org all,alphanum_,min=3"`
	Action   string ` validate:"required,oneof=read write delete update,alphanum_,min=3"`
}

type SignupRequestBody struct {
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=6"`
	FirstName string `json:"firstName" validate:"required"`
	LastName  string `json:"lastName" validate:"required"`
}

type RolePermissionBindingBody struct {
	ResourceName string `json:"resource_name"`
	Scope        string `json:"scope"`
	ActionName   string `json:"action_name"`
	RoleKey      string `json:"role_key"`
}

type UserRoleBindingBody struct {
	RoleKey  string `json:"role_key" validate:"required"`
	UserName string `json:"user_name" validate:"required"`
	OrgID    int    `json:"org_id"`
}
