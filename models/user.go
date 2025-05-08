package models

import (
	"bigbucks/solution/auth/constants"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models/types"
	"bigbucks/solution/auth/passwordreset"
	valids "bigbucks/solution/auth/validations"
	"crypto/rand"
	"encoding/json"
	"io"
	"os"
	"strings"

	// "errors"
	"fmt"
	// "log"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User : User credential model
type User struct {
	constants.BaseModel `json:"-"`
	Username            string               `gorm:"unique;not null" validate:"required,email"`
	Password            string               `gorm:"column:hashed_password" validate:"required,min=8"`
	Organizations       []*Organization      `gorm:"many2many:UserOrgRole;JoinForeignKey:UserID;JoinReferences:OrgID;"`
	Roles               []*Role              `gorm:"many2many:UserOrgRole;JoinForeignKey:UserID;JoinReferences:RoleID;"`
	Profile             Profile              `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	ForgotPassword      ForgotPassword       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	OAuthClient         OAuthClient          `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" validate:"structonly,omitempty"`
	EmailVerified       bool                 `gorm:"default:false"`
	MobileVerified      bool                 `gorm:"default:false"`
	Status              constants.UserStatus `gorm:"default:pending"`
	EmailVerification   EmailVerification
	MobileVerification  MobileVerification
}

// MarshalJSON Json Dump override method
func (usr User) MarshalJSON() ([]byte, error) {
	var tmp = &types.User{}
	tmp.ID = usr.ID
	tmp.Username = usr.Username
	tmp.Roles = usr.Roles
	err := Dbcon.Model(&usr).Association("Profile").Find(&usr.Profile)
	if err != nil {
		return nil, err
	}
	tmp.Profile = usr.Profile
	tmp.IsSocial = Dbcon.Model(&usr).Association("OAuthClient").Count() > 0
	return json.Marshal(&tmp)
}

// GenerateResetToken Generate a reset token to change password
func (usr User) GenerateResetToken() (string, error) {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		loging.Logger.Error("Error generating reset token", err)
		return "", err
	}
	var fg ForgotPassword
	err = Dbcon.Model(&usr).Association("ForgotPassword").Find(&fg)
	if err != nil {
		loging.Logger.Error("Error finding forgot password", err)
		return "", err
	}
	fg.ResetToken = fmt.Sprintf("%x", b)
	fg.Expiry = time.Now().Add(5 * time.Hour)
	if fg.ID == 0 {
		fg.UserID = usr.ID
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
	if usr.ForgotPassword.ResetToken != token || time.Now().After(usr.ForgotPassword.Expiry) {
		customerr.Errors["Token"] = "Invalid or expired token provided"
		return http.StatusForbidden, customerr
	}

	err := Dbcon.Transaction(func(tx *gorm.DB) error {
		tx.Model(&usr).Update("Password", password)
		usr.Password = password
		tx.Model(&usr.ForgotPassword).Update("Expiry", time.Now())
		return nil
	})
	if err != nil {
		return http.StatusForbidden, err
	}
	return 0, nil
}

// BeforeCreate GORM hook hash the password
func (usr *User) BeforeCreate(tx *gorm.DB) (err error) {
	// Call the parent BeforeCreate to generate ULID
	if err = usr.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}

	// Hash password
	if pw, err := bcrypt.GenerateFromPassword([]byte(usr.Password), 0); err == nil {
		usr.Password = string(pw)
	}
	return
}

// BeforeUpdate GORM hook hash the password
func (usr *User) BeforeUpdate(tx *gorm.DB) (err error) {
	if tx.Statement.Changed("Password") {
		x := tx.Statement.Dest.(map[string]interface{})["Password"]
		if pw, err := bcrypt.GenerateFromPassword([]byte(x.(string)), 0); err == nil {
			tx.Statement.SetColumn("Password", string(pw))
		}
	}
	return
}

// Authenticate => check for valid user credentials
func Authenticate(username, password string) (success bool, user User) {
	if err := Dbcon.Where("username = ?", username).Preload("Roles").First(&user).Error; err == gorm.ErrRecordNotFound {
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err == nil {
		success = true
	}
	return
}

// UpdateUserProfile Update user profile data from map<string, dynamic> datastructure
func (usr User) UpdateUserProfile(data map[string][]string, picture []*multipart.FileHeader) (int, error) {
	var filename string = ""
	if len(picture) > 0 {
		uuidWithHyphen := uuid.New()
		fmt.Println(uuidWithHyphen)
		uuid := strings.Replace(uuidWithHyphen.String(), "-", "", -1)
		file, _ := picture[0].Open()
		filename = fmt.Sprintf("%s.jpg", uuid)
		tmpfile, _ := os.Create(fmt.Sprintf("./profile_pics/%s", filename))
		_, err := io.Copy(tmpfile, file)
		if err != nil {
			loging.Logger.Error("Error saving profile picture", err)
			return http.StatusInternalServerError, err
		}
		tmpfile.Close()
	}
	var profile Profile
	err := Dbcon.Model(&usr).Association("Profile").Find(&profile)
	if err != nil {
		loging.Logger.Error("Error finding profile", err)
		return http.StatusInternalServerError, err
	}
	if len(profile.Picture) > 0 {
		os.Remove(fmt.Sprintf("./profile_pics/%s", profile.Picture))
	}
	profile.Email = data["useremail"][0]
	profile.FirstName = data["firstname"][0]
	profile.LastName = data["lastname"][0]
	profile.ContactNumber = data["userphone"][0]
	profile.Picture = filename
	// }
	Dbcon.Save(&profile)
	return 0, nil
}

// Authorize user permissions
func (usr User) Authorize(permission, resource string, orgID int) (bool, error) {
	// customerr := valids.NewErrorDict()
	var cnt int64
	Dbcon.Model(&Permission{}).
		Joins("INNER JOIN role_permissions rp ON rp.permission_id = id").
		Joins("INNER JOIN user_org_roles uor ON org_id = ? AND user_id = ? AND uor.role_id = rp.role_id", orgID, usr.ID).
		Where("UPPER(code) = ? and UPPER(resource) = ?",
			strings.ToUpper(strings.TrimSpace(permission)),
			strings.ToUpper(strings.TrimSpace(resource))).
		Count(&cnt)
	return cnt > 0, nil
}
