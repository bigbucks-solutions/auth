package models

import (
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
	Dbcon.AutoMigrate(&User{}, &Profile{}, &Organization{},
		&Role{}, &Permission{}, &UserOrgRole{})

	// Create
	// results := Dbcon.Create(&User{Username: "L1212", Password: "jamsheed"})
	// fmt.Print(results.Error != nil)
	Dbcon.Create(&Role{Name: "SuperUser",
		Description: "Super User Role for Organizations",
		Permissions: []*Permission{{Code: "ACCNT_ALL"}}})
}
