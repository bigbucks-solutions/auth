package models

import (
	"bigbucks/solution/auth/constants"
	"bigbucks/solution/auth/models/types"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// Role model
type Role struct {
	constants.BaseModel `swaggerignore:"true"`
	OrgID               string `gorm:"index:,unique,composite:name_org"`
	Name                string `gorm:"index:,unique,composite:name_org;not null" validate:"required,min=4"`
	Description         string
	IsSystemRole        bool            `gorm:"default:false"` // Mark if this is a system-managed role
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

	Description     string
	IsSystemManaged bool    `gorm:"default:false"` // Mark if this is a system-managed permission
	Roles           []*Role `gorm:"many2many:role_permissions;"`
}

// RolePermission junction table with additional metadata
type RolePermission struct {
	RoleID       string `gorm:"primaryKey"`
	PermissionID uint   `gorm:"primaryKey"`
	IsLocked     bool   `gorm:"default:false"`    // Cannot be removed by users
	IsHidden     bool   `gorm:"default:false"`    // Not visible in UI
	AssignedBy   string `gorm:"default:'system'"` // 'system' or 'user'
	CreatedAt    time.Time
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
	tmp.Description = role.Description
	tmp.IsSystemRole = role.IsSystemRole
	return json.Marshal(&tmp)
}

// Helper methods for role management
func (r *Role) IsSystemManaged() bool {
	return r.IsSystemRole
}

func (r *Role) CanModifyPermissions() bool {
	return !r.IsSystemRole
}
