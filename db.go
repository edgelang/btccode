package main

import (
	"os"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

var (
	DB *gorm.DB
)

func init() {
	var err error
	DB, err = gorm.Open("sqlite3", "watch.db")
	if err != nil {
		log.Crit("could not open db", "error", err)
		os.Exit(1)
	}
	var tx transaction
	if !DB.HasTable(&tx) {
		DB.CreateTable(&tx)
	}
}
