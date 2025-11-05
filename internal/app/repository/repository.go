package repository

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(dsn string) (*Repository, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{}) 
	if err != nil {
		return nil, err
	}

	return &Repository{
		db: db,
	}, nil
}

// DB возвращает экземпляр базы данных для прямых запросов
func (r *Repository) DB() *gorm.DB {
	return r.db
}
