package parser

import (
	"testing"
	"time"

	"nano-indexer/internal/eth"
)

func TestParseERC20Transfer(t *testing.T) {
	log := eth.Log{
		Address:          "0xToken",
		Topics:           []string{eth.TransferTopic, "0x0000000000000000000000001111111111111111111111111111111111111111", "0x0000000000000000000000002222222222222222222222222222222222222222"},
		Data:             "0x0000000000000000000000000000000000000000000000000000000000000064",
		BlockNumber:      "0xa",
		BlockHash:        "0xBlock",
		TransactionHash:  "0xTx",
		TransactionIndex: "0x1",
		LogIndex:         "0x2",
	}

	transfer, err := ParseERC20Transfer(8453, log)
	if err != nil {
		t.Fatalf("parse transfer: %v", err)
	}
	if transfer.AmountRaw != "100" {
		t.Fatalf("expected amount 100, got %q", transfer.AmountRaw)
	}
	if transfer.FromAddress != "0x1111111111111111111111111111111111111111" {
		t.Fatalf("unexpected from address %q", transfer.FromAddress)
	}
	if transfer.ToAddress != "0x2222222222222222222222222222222222222222" {
		t.Fatalf("unexpected to address %q", transfer.ToAddress)
	}
	if transfer.BlockNumber != 10 || transfer.TxIndex != 1 || transfer.LogIndex != 2 {
		t.Fatalf("unexpected indexes: block=%d tx=%d log=%d", transfer.BlockNumber, transfer.TxIndex, transfer.LogIndex)
	}
}

func TestParseERC20TransferUsesBlockTimestamp(t *testing.T) {
	log := eth.Log{
		Address:          "0xToken",
		Topics:           []string{eth.TransferTopic, "0x0000000000000000000000001111111111111111111111111111111111111111", "0x0000000000000000000000002222222222222222222222222222222222222222"},
		Data:             "0x0000000000000000000000000000000000000000000000000000000000000064",
		BlockNumber:      "0xa",
		BlockHash:        "0xBlock",
		BlockTimestamp:   "0x64",
		TransactionHash:  "0xTx",
		TransactionIndex: "0x1",
		LogIndex:         "0x2",
	}

	transfer, err := ParseERC20Transfer(8453, log)
	if err != nil {
		t.Fatalf("parse transfer: %v", err)
	}
	if !transfer.EventTime.Equal(time.Unix(100, 0).UTC()) {
		t.Fatalf("expected event time from blockTimestamp, got %s", transfer.EventTime)
	}
}

func TestParseERC20TransferRejectsWrongTopic(t *testing.T) {
	_, err := ParseERC20Transfer(1, eth.Log{
		Topics: []string{"0xdead", "0x1", "0x2"},
		Data:   "0x0000000000000000000000000000000000000000000000000000000000000064",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
