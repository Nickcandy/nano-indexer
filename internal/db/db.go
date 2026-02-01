package db

import (
	"nano-indexer/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func InitDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("indexer.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	// 自动建表
	db.AutoMigrate(&model.Transaction{}, &model.SyncState{})
	return db, nil
}