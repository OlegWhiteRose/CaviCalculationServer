package main

import (
	"rip/internal/app/ds"
	"rip/internal/app/dsn"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	_ = godotenv.Load()
	db, err := gorm.Open(postgres.Open(dsn.FromEnv()), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	err = db.AutoMigrate(
		&ds.CaviGroup{},
		&ds.CaviCalculation{},
		&ds.CaviCalculationGroup{},
		&ds.User{},
	)
	if err != nil {
		panic("cant migrate db")
	}
}

