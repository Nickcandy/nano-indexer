package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nano-indexer/internal/api"
	"nano-indexer/internal/config"
	"nano-indexer/internal/eth"
	"nano-indexer/internal/scanner"
	"nano-indexer/internal/storage"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("indexer stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	if err := config.LoadDotEnv(".env"); err != nil {
		logger.Warn("load .env", "error", err)
	}
	cfg := config.Load()
	logger.Info("loaded config",
		"mongo_database", cfg.Mongo.Database,
		"scanner_enabled", cfg.Scanner.Enabled,
		"chain_id", cfg.Eth.ChainID,
		"token_count", len(cfg.Scanner.TokenAddresses),
		"default_start_block", cfg.Eth.DefaultStartBlock,
	)
	if cfg.Scanner.Enabled {
		if cfg.Eth.RPCURL == "" {
			return fmt.Errorf("SCANNER_ENABLED=true but RPC_URL is empty")
		}
		if len(cfg.Scanner.TokenAddresses) == 0 {
			return fmt.Errorf("SCANNER_ENABLED=true but TOKEN_ADDRESSES is empty")
		}
	} else {
		logger.Warn("scanner disabled; api will start but no chain data will be written", "enable_with", "SCANNER_ENABLED=true")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mongoClient, err := storage.Connect(ctx, cfg.Mongo.URI)
	if err != nil {
		return fmt.Errorf("connect mongo: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := storage.Disconnect(shutdownCtx, mongoClient); err != nil {
			logger.Error("disconnect mongo", "error", err)
		}
	}()

	if err := storage.Ping(ctx, mongoClient); err != nil {
		return fmt.Errorf("ping mongo database %q: %w", cfg.Mongo.Database, err)
	}
	db := mongoClient.Database(cfg.Mongo.Database)
	transferRepo := storage.NewTransferRepo(db)
	syncStateRepo := storage.NewSyncStateRepo(db)
	if err := transferRepo.EnsureIndexes(ctx); err != nil {
		return err
	}
	if err := syncStateRepo.EnsureIndexes(ctx); err != nil {
		return err
	}

	if cfg.Scanner.Enabled {
		ethClient := eth.NewClient(cfg.Eth.RPCURL)
		indexer := scanner.New(cfg, ethClient, transferRepo, syncStateRepo, logger)
		go func() {
			if err := indexer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("scanner stopped", "error", err)
				stop()
			}
		}()
	}

	server := api.NewServer(transferRepo)
	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting http server", "addr", cfg.Server.Addr())
		err := server.Start(cfg.Server.Addr())
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}
		logger.Info("indexer stopped")
		return nil
	case err := <-errCh:
		return err
	}
}
