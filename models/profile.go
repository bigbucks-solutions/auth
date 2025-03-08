package models

import (
	"bigbucks/solution/auth/models/types"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

// Profile : GORM model User profile
type Profile struct {
	gorm.Model    `json:"-"`
	UserID        string `json:"-"`
	FirstName     string
	LastName      string
	ContactNumber string `json:"phone"`
	Email         string
	Picture       string `json:"file"`
}

// MarshalJSON Json Dump override method for Profile struct
func (prf Profile) MarshalJSON() ([]byte, error) {
	var tmp = &types.Profile{}
	tmp.Email = prf.Email
	tmp.Firstname = prf.FirstName
	tmp.Lastname = prf.LastName
	tmp.Phone = prf.ContactNumber
	if prf.Picture != "" {
		var s = fmt.Sprintf("/avatar/%s", prf.Picture)
		tmp.Picture = &s
	}
	return json.Marshal(&tmp)
}
