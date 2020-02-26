package models

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
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
	Dbcon.AutoMigrate(&User{}, &Profile{}, &Organization{}, &OrganizationBranch{},
		&Role{}, &Permission{})

	// Create
	Dbcon.Create(&User{Username: "L1212", Password: "jamsheed"})
}
