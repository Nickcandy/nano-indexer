package scanner

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"nano-indexer/internal/config"
	"nano-indexer/internal/eth"
	"nano-indexer/internal/model"
	"nano-indexer/internal/parser"
	"nano-indexer/internal/storage"
)

const scannerName = "erc20_transfer"

type Scanner struct {
	cfg       config.Config
	ethClient *eth.Client
	transfers *storage.TransferRepo
	states    *storage.SyncStateRepo
	logger    *slog.Logger
}

func New(cfg config.Config, ethClient *eth.Client, transfers *storage.TransferRepo, states *storage.SyncStateRepo, logger *slog.Logger) *Scanner {
	return &Scanner{cfg: cfg, ethClient: ethClient, transfers: transfers, states: states, logger: logger}
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
	if err := s.states.UpdateProgress(ctx, state, to, safeBlock); err != nil {
		return err
	}
	s.logger.Info("scanned transfer logs", "token", token, "from", from, "to", to, "logs", len(logs))
	return nil
}
