package models

import (
	"bigbucks/solution/auth/constants"
	"time"
)

// InvitationStatus represents the status of an invitation
type InvitationStatus string

const (
	InvitationStatusPending  InvitationStatus = "pending"
	InvitationStatusAccepted InvitationStatus = "accepted"
	InvitationStatusExpired  InvitationStatus = "expired"
	InvitationStatusRevoked  InvitationStatus = "revoked"
)

// Invitation represents an invitation to join an organization with a specific role
type Invitation struct {
	constants.BaseModel `json:"-"`
	Email               string           `gorm:"not null" validate:"required,email"`
	InviterID           string           `gorm:"not null"`
	OrgID               string           `gorm:"not null"`
	RoleID              string           `gorm:"not null"`
	Status              InvitationStatus `gorm:"default:pending"`
	Token               string           `gorm:"unique;not null"`
	ExpiresAt           time.Time        `gorm:"not null"`
	AcceptedAt          *time.Time

	// Relationships
	Inviter      User         `gorm:"foreignKey:InviterID"`
	Organization Organization `gorm:"foreignKey:OrgID"`
	Role         Role         `gorm:"foreignKey:RoleID"`
}

// IsExpired checks if the invitation has expired
func (i *Invitation) IsExpired() bool {
	return time.Now().After(i.ExpiresAt)
}

// CanBeAccepted checks if the invitation can be accepted
func (i *Invitation) CanBeAccepted() bool {
	return i.Status == InvitationStatusPending && !i.IsExpired()
}
