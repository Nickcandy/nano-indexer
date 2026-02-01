package model

import (
	"time"
	"gorm.io/gorm"
)

type Transaction struct {
	gorm.Model
	Hash        string `gorm:"uniqueIndex"`
	BlockNumber uint64 `gorm:"index"`
	From        string
	To          string
	Value       string
	Timestamp   time.Time
}