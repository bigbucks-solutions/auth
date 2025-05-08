package models

import (
	"time"

	"gorm.io/gorm"
)

type EmailVerification struct {
	gorm.Model
	UserID    uint
	Token     string
	Email     string
	ExpiresAt time.Time
}

type MobileVerification struct {
	gorm.Model
	UserID       uint
	Token        string
	MobileNumber string
	ExpiresAt    time.Time
}
