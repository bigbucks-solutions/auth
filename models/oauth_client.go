package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ForgotPassword : GORM model for social oauth data
type OAuthClient struct {
	gorm.Model `json:"-" swaggerignore:"true"`
	UserID     int
	Source     string         `gorm:"not null" validate:"required,oneof=google facebook"`
	Details    datatypes.JSON `swaggerignore:"true"`
}
