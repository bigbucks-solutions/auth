package constants

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
	// Import Atlas schema package
)

// atlas:schema
// atlas:schema:exclude
type BaseModel struct {
	ID        string         `gorm:"type:char(26);primaryKey"`
	CreatedAt time.Time      `gorm:"index"`
	UpdatedAt time.Time      `gorm:"index"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// BeforeCreate hook to generate ULID before record creation
func (base *BaseModel) BeforeCreate(tx *gorm.DB) error {
	if base.ID == "" {
		entropy := ulid.Monotonic(rand.Reader, 0)
		base.ID = ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
	}
	return nil
}
