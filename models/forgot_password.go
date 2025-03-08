package models

import (
	"time"

	"gorm.io/gorm"
)

// ForgotPassword : GORM model to track password reset token
type ForgotPassword struct {
	gorm.Model `json:"-"`
	UserID     string
	ResetToken string
	Expiry     time.Time
}
