package parser

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"nano-indexer/internal/eth"
	"nano-indexer/internal/model"
)

func ParseERC20Transfer(chainID int64, log eth.Log) (model.TokenTransfer, error) {
	if len(log.Topics) != 3 {
		return model.TokenTransfer{}, fmt.Errorf("transfer log must have 3 topics, got %d", len(log.Topics))
	}
	if !strings.EqualFold(log.Topics[0], eth.TransferTopic) {
		return model.TokenTransfer{}, fmt.Errorf("unexpected transfer topic %q", log.Topics[0])
	}
	data := strings.TrimPrefix(strings.ToLower(log.Data), "0x")
	if len(data) != 64 {
		return model.TokenTransfer{}, fmt.Errorf("transfer data must be 32 bytes, got %d hex chars", len(data))
	}
	amount := new(big.Int)
	if _, ok := amount.SetString(data, 16); !ok {
		return model.TokenTransfer{}, fmt.Errorf("parse transfer amount %q", log.Data)
	}
	blockNumber, err := eth.ParseHexUint64(log.BlockNumber)
	if err != nil {
		return model.TokenTransfer{}, err
	}
	txIndex, err := eth.ParseHexUint64(log.TransactionIndex)
	if err != nil {
		return model.TokenTransfer{}, err
	}
	logIndex, err := eth.ParseHexUint64(log.LogIndex)
	if err != nil {
		return model.TokenTransfer{}, err
	}
	now := time.Now().UTC()
	eventTime := now
	if strings.TrimSpace(log.BlockTimestamp) != "" {
		timestamp, err := eth.ParseHexUint64(log.BlockTimestamp)
		if err != nil {
			return model.TokenTransfer{}, fmt.Errorf("parse block timestamp: %w", err)
		}
		eventTime = time.Unix(int64(timestamp), 0).UTC()
	}
	return model.TokenTransfer{
		ChainID:      chainID,
		BlockNumber:  blockNumber,
		BlockHash:    strings.ToLower(log.BlockHash),
		TxHash:       strings.ToLower(log.TransactionHash),
		LogIndex:     logIndex,
		TxIndex:      txIndex,
		TokenAddress: strings.ToLower(log.Address),
		FromAddress:  topicAddress(log.Topics[1]),
		ToAddress:    topicAddress(log.Topics[2]),
		AmountRaw:    amount.String(),
		Confirmed:    true,
		Removed:      log.Removed,
		EventTime:    eventTime,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func topicAddress(topic string) string {
	topic = strings.TrimPrefix(strings.ToLower(topic), "0x")
	if len(topic) < 40 {
		return "0x" + topic
	}
	return "0x" + topic[len(topic)-40:]
}
