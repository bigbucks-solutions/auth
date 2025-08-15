package models

import (
	"encoding/json"
	"time"
)

// AuthLog : GORM model for social oauth data
type AuthLog struct {
	UserID  string          `gorm:"index:idx_user_login,priority:1"`
	LoginAt time.Time       `gorm:"index:idx_user_login,priority:2,sort:desc"`
	Attrs   json.RawMessage `gorm:"type:jsonb"`
}
