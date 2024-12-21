package models

import (
	"gorm.io/gorm"

	_ "ariga.io/atlas-provider-gorm/gormschema"
)

var (
	Dbcon *gorm.DB
)

// Migrate function
func Migrate() {

	_ = Dbcon.AutoMigrate(&UserOrgRole{})
	_ = Dbcon.AutoMigrate(&User{}, &Profile{}, &OAuthClient{}, &Organization{},
		&Role{}, &Permission{}, &UserOrgRole{}, &ForgotPassword{})

	// Create
	// results := Dbcon.Create(&User{Username: "L1212", Password: "jamsheed"})
	// fmt.Print(results.Error != nil)
	// Dbcon.Create(&Role{Name: "SuperUser",
	// 	Description: "Super User Role for Organizations",
	// 	Permissions: []*Permission{{Code: "ACCNT_ALL"}}})
}
