package models

import (
	"time"

	"gorm.io/gorm"
)

type EmailVerification struct {
	gorm.Model
	UserID         string    `gorm:"type:char(26);uniqueIndex;not null"`
	CodeDigest     []byte    `gorm:"type:bytea;not null"`
	Email          string    `gorm:"not null;index"`
	FailedAttempts uint      `gorm:"not null;default:0"`
	ExpiresAt      time.Time `gorm:"not null;index"`
	LastSentAt     time.Time `gorm:"not null"`
	SendWindowAt   time.Time `gorm:"not null"`
	SendCount      uint      `gorm:"not null;default:1"`
	ConsumedAt     *time.Time
}

type MobileVerification struct {
	gorm.Model
	UserID       uint
	Token        string
	MobileNumber string
	ExpiresAt    time.Time
}
