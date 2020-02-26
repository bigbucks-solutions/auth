package models

import (
	"github.com/jinzhu/gorm"
	"golang.org/x/crypto/bcrypt"
)

// User credential model
type User struct {
	gorm.Model
	Username      string          `gorm:"unique;not null"`
	Password      string          `gorm:"column:hashed_password"`
	Organizations []*Organization `gorm:"many2many:org_users;"`
	Roles         []*Role         `gorm:"many2many:user_roles;"`
	Profile       Profile
}

// BeforeSave hash password
func (user *User) BeforeSave(scope *gorm.Scope) (err error) {
	if pw, err := bcrypt.GenerateFromPassword([]byte(user.Password), 0); err == nil {
		scope.SetColumn("hashed_password", string(pw))
	}
	return
}

// Authenticate => check for valid user credentials
func Authenticate(username, password string) (success bool, user User) {
	if err := Dbcon.Where("username = ?", username).First(&user).Error; gorm.IsRecordNotFoundError(err) {
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
