package model

import "time"

type SyncState struct {
	ChainID              int64     `bson:"chain_id" json:"chain_id"`
	ScannerName          string    `bson:"scanner_name" json:"scanner_name"`
	TokenAddress         string    `bson:"token_address" json:"token_address"`
	FromBlock            uint64    `bson:"from_block" json:"from_block"`
	LatestScannedBlock   uint64    `bson:"latest_scanned_block" json:"latest_scanned_block"`
	LatestConfirmedBlock uint64    `bson:"latest_confirmed_block" json:"latest_confirmed_block"`
	Confirmations        uint64    `bson:"confirmations" json:"confirmations"`
	Status               string    `bson:"status" json:"status"`
	LastError            string    `bson:"last_error,omitempty" json:"last_error,omitempty"`
	UpdatedAt            time.Time `bson:"updated_at" json:"updated_at"`
}
