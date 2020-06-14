package models

import (
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User credential model
type User struct {
	gorm.Model
	Username      string          `gorm:"unique;not null" validate:"required,email"`
	Password      string          `gorm:"column:hashed_password" validate:"required,min=8"`
	Organizations []*Organization `gorm:"many2many:org_users;"`
	Roles         []*Role         `gorm:"many2many:user_org_roles;association_jointable_foreignkey:user_id;jointable_foreignkey:role_id;"`
	Profile       Profile
}

// BeforeSave hash password
func (user *User) BeforeSave(scope *gorm.DB) (err error) {
	if pw, err := bcrypt.GenerateFromPassword([]byte(user.Password), 0); err == nil {
		user.Password = string(pw)
	}
	return
}

// Authenticate => check for valid user credentials
func Authenticate(username, password string) (success bool, user User) {
	if err := Dbcon.Where("username = ?", username).First(&user).Error; err == gorm.ErrRecordNotFound {
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err == nil {
		success = true
	}
	return
}

// Profile => User profile detailing model
type Profile struct {
	gorm.Model
	UserID        int
	FirstName     string
	LastName      string
	ContactNumber string
	Email         string
}
