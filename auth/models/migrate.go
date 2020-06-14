package models

import (
	// _ "github.com/go-gorm/gorm/dialects/sqlite"
	// _ "gorm.io/driver/sqlite"
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
	Dbcon.AutoMigrate(&UserOrgRole{}, &User{}, &Profile{}, &Organization{}, &OrganizationBranch{},
		&Role{}, &Permission{})

	// Create
	Dbcon.Create(&User{Username: "L1212", Password: "jamsheed"})
	Dbcon.Create(&Role{Name: "SuperUser",
		Description: "Super User Role for Organizations",
		Permissions: []*Permission{{Code: "ACCNT_ALL"}}})
}
