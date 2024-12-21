package models

import (
	"bigbucks/solution/auth/models/types"
	"database/sql/driver"
	"encoding/json"

	"gorm.io/gorm"
)

type Scope string
type Action string

const (
	ScopeAll        Scope = "all"
	ScopeOrg        Scope = "org"
	ScopeAssociated Scope = "associated"
	ScopeOwn        Scope = "own"
)

const (
	ActionWrite  Action = "write"
	ActionCreate Action = "create"
	ActionRead   Action = "read"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

var Scopes = []Scope{ScopeAll, ScopeOrg, ScopeAssociated, ScopeOwn}

var Actions = []Action{ActionWrite, ActionCreate, ActionUpdate, ActionDelete, ActionRead}

var Resources = []string{"users", "masterdata", "inventory", "roles", "permissions", "accounts", "transactions"}

func (p *Action) Scan(value interface{}) error {
	*p = Action(value.(string))
	return nil
}
func (p Action) Value() (driver.Value, error) {
	return string(p), nil
}

func (p *Scope) Scan(value interface{}) error {
	*p = Scope(value.(string))
	return nil
}

func (p Scope) Value() (driver.Value, error) {
	return string(p), nil
}

// Role model
type Role struct {
	gorm.Model  `swaggerignore:"true"`
	OrgID       int    `gorm:"index:,unique,composite:name_org"`
	Name        string `gorm:"index:,unique,composite:name_org;not null" validate:"required,min=4"`
	Description string
	Permissions []*Permission `gorm:"many2many:role_permissions;"`
	Users       []*User       `gorm:"many2many:UserOrgRole;JoinForeignKey:RoleID;JoinReferences:UserID;"`
}

// Permission model
type Permission struct {
	gorm.Model `swaggerignore:"true"`
	Resource   string `gorm:"not null;index:idx_resource;index:idx_res_scope_action,unique,priority:1" validate:"alphanum_,min=3"`
	Scope      Scope  `gorm:"not null;index:idx_res_scope_action,unique,priority:2" validate:"required,oneof=own org associated all,alphanum_,min=3"`
	Action     Action `gorm:"not null;index:idx_res_scope_action,unique,priority:3;check:action IN ('read', 'write', 'delete', 'update', 'create')" validate:"required,oneof=read write delete update create,alphanum_,min=3"`

	Description string
	Roles       []*Role `gorm:"many2many:role_permissions;"`
}

// UserOrgRole many to many relation table
type UserOrgRole struct {
	OrgID  int `gorm:"not null"`
	UserID int `gorm:"not null"`
	RoleID int `gorm:"not null"`
}

// MarshalJSON Json Dump override method
func (role Role) MarshalJSON() ([]byte, error) {
	var tmp = &types.Role{}
	tmp.Name = role.Name
	return json.Marshal(&tmp)
}
