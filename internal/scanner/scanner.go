package scanner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"nano-indexer/internal/config"
	"nano-indexer/internal/eth"
	"nano-indexer/internal/model"
	"nano-indexer/internal/parser"
	"nano-indexer/internal/storage"
)

const scannerName = "erc20_transfer"

type ethReader interface {
	BlockNumber(ctx context.Context) (uint64, error)
	BlockByNumber(ctx context.Context, number uint64) (eth.Block, error)
	FilterTransferLogs(ctx context.Context, fromBlock, toBlock uint64, tokenAddress string) ([]eth.Log, error)
}

type transferWriter interface {
	UpsertMany(ctx context.Context, transfers []model.TokenTransfer) error
}

type syncStateStore interface {
	GetOrCreate(ctx context.Context, chainID int64, scannerName string, tokenAddress string, fromBlock uint64, confirmations uint64) (model.SyncState, error)
	UpdateProgress(ctx context.Context, state model.SyncState, scannedBlock uint64, confirmedBlock uint64) error
}

type blockWriter interface {
	UpsertMany(ctx context.Context, blocks []model.Block) error
}

type Scanner struct {
	cfg       config.Config
	ethClient ethReader
	transfers transferWriter
	states    syncStateStore
	blocks    blockWriter
	logger    *slog.Logger
}

func New(cfg config.Config, ethClient ethReader, transfers *storage.TransferRepo, states *storage.SyncStateRepo, blocks *storage.BlockRepo, logger *slog.Logger) *Scanner {
	return &Scanner{cfg: cfg, ethClient: ethClient, transfers: transfers, states: states, blocks: blocks, logger: logger}
}

func (s *Scanner) Run(ctx context.Context) error {
	if len(s.cfg.Scanner.TokenAddresses) == 0 {
		return fmt.Errorf("scanner enabled but token addresses are empty")
	}
	ticker := time.NewTicker(s.cfg.Eth.PollInterval)
	defer ticker.Stop()

	for {
		if err := s.ScanOnce(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *Scanner) ScanOnce(ctx context.Context) error {
	latest, err := s.ethClient.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("get latest block: %w", err)
	}
	if latest < s.cfg.Eth.Confirmations {
		return nil
	}
	safeBlock := latest - s.cfg.Eth.Confirmations
	for _, token := range s.cfg.Scanner.TokenAddresses {
		if err := s.scanToken(ctx, token, safeBlock); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scanner) scanToken(ctx context.Context, token string, safeBlock uint64) error {
	state, err := s.states.GetOrCreate(ctx, s.cfg.Eth.ChainID, scannerName, token, s.cfg.Eth.DefaultStartBlock, s.cfg.Eth.Confirmations)
	if err != nil {
		return err
	}
	from := state.LatestScannedBlock + 1
	if from > safeBlock {
		return nil
	}
	to := from + s.cfg.Eth.BatchSize - 1
	if to > safeBlock || to < from {
		to = safeBlock
	}

	logs, err := s.ethClient.FilterTransferLogs(ctx, from, to, token)
	if err != nil {
		return fmt.Errorf("filter transfer logs %s %d-%d: %w", token, from, to, err)
	}
	transfers := make([]model.TokenTransfer, 0, len(logs))
	for _, log := range logs {
		transfer, err := parser.ParseERC20Transfer(s.cfg.Eth.ChainID, log)
		if err != nil {
			return fmt.Errorf("parse transfer log %s/%s: %w", log.TransactionHash, log.LogIndex, err)
		}
		transfers = append(transfers, transfer)
	}
	if err := s.transfers.UpsertMany(ctx, transfers); err != nil {
		return err
	}
	blocks, err := s.fetchBlocks(ctx, from, to)
	if err != nil {
		return err
	}
	if err := s.blocks.UpsertMany(ctx, blocks); err != nil {
		return err
	}
	if err := s.states.UpdateProgress(ctx, state, to, safeBlock); err != nil {
		return err
	}
	s.logger.Info("scanned transfer logs", "token", token, "from", from, "to", to, "logs", len(logs))
	return nil
}

func (s *Scanner) fetchBlocks(ctx context.Context, from uint64, to uint64) ([]model.Block, error) {
	blocks := make([]model.Block, 0, to-from+1)
	for blockNumber := from; blockNumber <= to; blockNumber++ {
		rpcBlock, err := s.ethClient.BlockByNumber(ctx, blockNumber)
		if err != nil {
			return nil, fmt.Errorf("get block %d: %w", blockNumber, err)
		}
		parsedNumber, err := eth.ParseHexUint64(rpcBlock.Number)
		if err != nil {
			return nil, fmt.Errorf("parse block number %d: %w", blockNumber, err)
		}
		if parsedNumber != blockNumber {
			return nil, fmt.Errorf("rpc block number mismatch: requested %d got %d", blockNumber, parsedNumber)
		}
		timestamp, err := eth.ParseHexUint64(rpcBlock.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("parse block timestamp %d: %w", blockNumber, err)
		}
		now := time.Now().UTC()
		blocks = append(blocks, model.Block{
			ChainID:     s.cfg.Eth.ChainID,
			BlockNumber: blockNumber,
			BlockHash:   strings.ToLower(rpcBlock.Hash),
			ParentHash:  strings.ToLower(rpcBlock.ParentHash),
			BlockTime:   time.Unix(int64(timestamp), 0).UTC(),
			Status:      "canonical",
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}
	return blocks, nil
}
