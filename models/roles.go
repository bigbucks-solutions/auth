package models

import (
	"bigbucks/solution/auth/constants"
	"bigbucks/solution/auth/models/types"
	"encoding/json"

	"gorm.io/gorm"
)

// Role model
type Role struct {
	constants.BaseModel `swaggerignore:"true"`
	OrgID               string `gorm:"index:,unique,composite:name_org"`
	Name                string `gorm:"index:,unique,composite:name_org;not null" validate:"required,min=4"`
	Description         string
	ExtraAttrs          json.RawMessage `gorm:"type:jsonb"`
	Permissions         []*Permission   `gorm:"many2many:role_permissions;"`
	Users               []*User         `gorm:"many2many:UserOrgRole;JoinForeignKey:RoleID;JoinReferences:UserID;"`
}

// Permission model
type Permission struct {
	gorm.Model `swaggerignore:"true"`
	Resource   string           `gorm:"not null;index:idx_resource;index:idx_res_scope_action,unique,priority:1" validate:"alphanum_,min=3"`
	Scope      constants.Scope  `gorm:"not null;index:idx_res_scope_action,unique,priority:2" validate:"required,oneof=own org associated all,alphanum_,min=3"`
	Action     constants.Action `gorm:"not null;index:idx_res_scope_action,unique,priority:3;check:action IN ('read', 'write', 'delete', 'update', 'create')" validate:"required,oneof=read write delete update create,alphanum_,min=3"`

	Description string
	Roles       []*Role `gorm:"many2many:role_permissions;"`
}

// UserOrgRole many to many relation table
type UserOrgRole struct {
	OrgID  string `gorm:"not null"`
	UserID string `gorm:"not null"`
	RoleID string `gorm:"not null"`
}

// MarshalJSON Json Dump override method
func (role Role) MarshalJSON() ([]byte, error) {
	var tmp = &types.Role{}
	tmp.Name = role.Name
	return json.Marshal(&tmp)
}
