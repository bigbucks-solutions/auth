package models

import (
	"bigbucks/solution/auth/passwordreset"
	valids "bigbucks/solution/auth/validations"
	"crypto/rand"
	"encoding/json"

	// "errors"
	"fmt"
	// "log"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// User : User credential model
type User struct {
	gorm.Model     `json:"-"`
	Username       string          `gorm:"unique;not null" validate:"required,email"`
	Password       string          `gorm:"column:hashed_password" validate:"required,min=8"`
	Organizations  []*Organization `gorm:"many2many:UserOrgRole;JoinForeignKey:UserID;JoinReferences:OrgID;"`
	Roles          []*Role         `gorm:"many2many:UserOrgRole;JoinForeignKey:UserID;JoinReferences:RoleID;"`
	Profile        Profile         `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	ForgotPassword ForgotPassword  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	OAuthClient    OAuthClient     `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
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

// ForgotPassword : GORM model to track password reset token
type ForgotPassword struct {
	gorm.Model `json:"-"`
	UserID     int
	ResetToken string
	Expiry     time.Time
}

// ForgotPassword : GORM model to track password reset token
type OAuthClient struct {
	gorm.Model `json:"-"`
	UserID     int
	Source     string `gorm:"not null" validate:"required,oneof=google facebook"`
	Details    datatypes.JSON
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

// GenerateResetToken Generate a reset token to change password
func (usr User) GenerateResetToken() (string, error) {
	b := make([]byte, 8)
	rand.Read(b)
	var fg ForgotPassword
	Dbcon.Model(&usr).Association("ForgotPassword").Find(&fg)
	fg.ResetToken = fmt.Sprintf("%x", b)
	fg.Expiry = time.Now().Add(5 * time.Hour)
	if fg.ID == 0 {
		fg.UserID = int(usr.ID)
		Dbcon.Model(&fg).Create(&fg)
	} else {
		Dbcon.Model(&fg).Save(&fg)
	}
	// TODO: make this async
	passwordreset.SendResetEmail(fg.ResetToken, usr.Profile.Email, usr.Username)
	return fmt.Sprintf("%x", b), nil
}

// ChangePassword change user password with reset token
func (usr User) ChangePassword(token, password string) (int, error) {
	customerr := valids.NewErrorDict()
	// var fg ForgotPassword
	// err := Dbcon.Model(&usr).Association("ForgotPassword").Find(&fg, "reset_token = ?", token)
	if usr.ForgotPassword.ResetToken != token || time.Now().After(usr.ForgotPassword.Expiry) {
		customerr.Errors["Token"] = "Invalid or expired token provided"
		return http.StatusForbidden, customerr
	}

	err := Dbcon.Transaction(func(tx *gorm.DB) error {

		// usr.ForgotPassword.Expiry = time.Now()
		tx.Model(&usr).Update("Password", password)
		usr.Password = password
		tx.Model(&usr.ForgotPassword).Update("Expiry", time.Now())
		return nil
	})
	if err != nil {
		return http.StatusForbidden, err
	}
	// Dbcon.Update(&usr)
	return 0, nil
}

// BeforeCreate GORM hook hash the password
func (usr *User) BeforeCreate(tx *gorm.DB) (err error) {
	if pw, err := bcrypt.GenerateFromPassword([]byte(usr.Password), 0); err == nil {
		usr.Password = string(pw)
	}
	return
}

// BeforeUpdate GORM hook hash the password
func (usr *User) BeforeUpdate(tx *gorm.DB) (err error) {
	if tx.Statement.Changed("Password") {
		x := tx.Statement.Dest.(map[string]interface{})["Password"]
		// fmt.Println(x)
		// fmt.Println("changed password")
		if pw, err := bcrypt.GenerateFromPassword([]byte(x.(string)), 0); err == nil {
			tx.Statement.SetColumn("Password", string(pw))
		}
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
		// fmt.Println(err)

	}

	return
}
