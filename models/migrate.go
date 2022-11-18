package models

import (
	"fmt"

	"gorm.io/gorm"
)

var (
	Dbcon *gorm.DB
)

// Migrate function
func Migrate() {
	// db, err := gorm.Open("sqlite3", "test.db")
	// if err != nil {
	// 	panic("failed to connect database")
	// }
	// defer db.Close()

	// Migrate the schema
	// err := Dbcon.SetupJoinTable(&Organization{}, "Users", &UserOrgRole{})
	// err = Dbcon.SetupJoinTable(&User{}, "Roles", &UserOrgRole{})
	// fmt.Println(err)
	Dbcon.AutoMigrate(&UserOrgRole{})
	Dbcon.AutoMigrate(&User{}, &Profile{}, &OAuthClient{}, &Organization{},
		&Role{}, &Permission{}, &UserOrgRole{}, &ForgotPassword{})

	// Create
	results := Dbcon.Create(&User{Username: "L1212", Password: "jamsheed"})
	fmt.Print(results.Error != nil)
	Dbcon.Create(&Role{Name: "SuperUser",
		Description: "Super User Role for Organizations",
		Permissions: []*Permission{{Code: "ACCNT_ALL"}}})
}
