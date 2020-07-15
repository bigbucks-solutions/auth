package models

import (
	"encoding/json"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User : User credential model
type User struct {
	gorm.Model    `json:"-"`
	Username      string          `gorm:"unique;not null" validate:"required,email"`
	Password      string          `gorm:"column:hashed_password" validate:"required,min=8"`
	Organizations []*Organization `gorm:"many2many:UserOrgRole;JoinForeignKey:UserID;JoinReferences:OrgID;"`
	Roles         []*Role         `gorm:"many2many:UserOrgRole;JoinForeignKey:UserID;JoinReferences:RoleID;"`
	Profile       Profile
}

// Profile : GORM model User profile
type Profile struct {
	gorm.Model    `json:"-"`
	UserID        int
	FirstName     string
	LastName      string
	ContactNumber string
	Email         string
}

// MarshalJSON Json Dump override method
func (usr User) MarshalJSON() ([]byte, error) {
	var tmp struct {
		Username string `json:"username"`
		Roles    []*Role
		Profile  Profile `json:"omitempty"`
	}
	tmp.Username = usr.Username
	tmp.Roles = usr.Roles
	tmp.Profile = usr.Profile
	return json.Marshal(&tmp)
}

// BeforeSave GORM hook hash the password
func (usr *User) BeforeSave(tx *gorm.DB) (err error) {
	if pw, err := bcrypt.GenerateFromPassword([]byte(usr.Password), 0); err == nil {
		// tx.Statement.Set("hashed_password", string(pw))
		usr.Password = string(pw)
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
