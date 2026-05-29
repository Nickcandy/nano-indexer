package model

import "time"

// Block stores the minimal chain block metadata needed for future reorg checks.
type Block struct {
	ChainID     int64     `bson:"chain_id" json:"chain_id"`
	BlockNumber uint64    `bson:"block_number" json:"block_number"`
	BlockHash   string    `bson:"block_hash" json:"block_hash"`
	ParentHash  string    `bson:"parent_hash" json:"parent_hash"`
	BlockTime   time.Time `bson:"block_time" json:"block_time"`
	Status      string    `bson:"status" json:"status"`
	CreatedAt   time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at" json:"updated_at"`
}
