package model

import "time"

type TokenTransfer struct {
	ChainID      int64     `bson:"chain_id" json:"chain_id"`
	BlockNumber  uint64    `bson:"block_number" json:"block_number"`
	BlockHash    string    `bson:"block_hash" json:"block_hash"`
	TxHash       string    `bson:"tx_hash" json:"tx_hash"`
	LogIndex     uint64    `bson:"log_index" json:"log_index"`
	TxIndex      uint64    `bson:"tx_index" json:"tx_index"`
	TokenAddress string    `bson:"token_address" json:"token_address"`
	FromAddress  string    `bson:"from_address" json:"from_address"`
	ToAddress    string    `bson:"to_address" json:"to_address"`
	AmountRaw    string    `bson:"amount_raw" json:"amount_raw"`
	Confirmed    bool      `bson:"confirmed" json:"confirmed"`
	Removed      bool      `bson:"removed" json:"removed"`
	EventTime    time.Time `bson:"event_time" json:"event_time"`
	CreatedAt    time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at" json:"updated_at"`
}

// AddressSummary is the minimal aggregate view for transfers involving one address.
type AddressSummary struct {
	ChainID       int64    `bson:"chain_id" json:"chain_id"`
	Address       string   `bson:"address" json:"address"`
	SentCount     int64    `bson:"sent_count" json:"sent_count"`
	ReceivedCount int64    `bson:"received_count" json:"received_count"`
	TotalCount    int64    `bson:"total_count" json:"total_count"`
	LatestBlock   uint64   `bson:"latest_block" json:"latest_block"`
	TokenCount    int      `bson:"token_count" json:"token_count"`
	Tokens        []string `bson:"tokens" json:"tokens"`
}
