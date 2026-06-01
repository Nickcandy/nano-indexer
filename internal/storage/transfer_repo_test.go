package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"nano-indexer/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestTransferRepoUpsertManyIsIdempotent(t *testing.T) {
	uri := os.Getenv("NANO_INDEXER_MONGO_TEST_URI")
	if uri == "" {
		t.Skip("set NANO_INDEXER_MONGO_TEST_URI to run Mongo integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := Connect(ctx, uri)
	if err != nil {
		t.Fatalf("connect mongo: %v", err)
	}
	defer func() { _ = Disconnect(ctx, client) }()

	db := client.Database("nano_indexer_test")
	if err := db.Drop(ctx); err != nil {
		t.Fatalf("drop test database: %v", err)
	}
	repo := NewTransferRepo(db)
	if err := repo.EnsureIndexes(ctx); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}

	transfer := model.TokenTransfer{
		ChainID:      8453,
		BlockNumber:  10,
		BlockHash:    "0xblock",
		TxHash:       "0xtx",
		LogIndex:     1,
		TxIndex:      0,
		TokenAddress: "0xtoken",
		FromAddress:  "0x1111111111111111111111111111111111111111",
		ToAddress:    "0x2222222222222222222222222222222222222222",
		AmountRaw:    "100",
		Confirmed:    true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := repo.UpsertMany(ctx, []model.TokenTransfer{transfer}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := repo.UpsertMany(ctx, []model.TokenTransfer{transfer}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	count, err := repo.coll.CountDocuments(ctx, map[string]any{"chain_id": int64(8453), "tx_hash": "0xtx", "log_index": uint64(1)})
	if err != nil {
		t.Fatalf("count transfers: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one transfer, got %d", count)
	}
}

func TestTransferRepoMarkRemovedFrom(t *testing.T) {
	uri := os.Getenv("NANO_INDEXER_MONGO_TEST_URI")
	if uri == "" {
		t.Skip("set NANO_INDEXER_MONGO_TEST_URI to run Mongo integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := Connect(ctx, uri)
	if err != nil {
		t.Fatalf("connect mongo: %v", err)
	}
	defer func() { _ = Disconnect(ctx, client) }()

	db := client.Database("nano_indexer_test")
	if err := db.Drop(ctx); err != nil {
		t.Fatalf("drop test database: %v", err)
	}
	repo := NewTransferRepo(db)
	if err := repo.EnsureIndexes(ctx); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}

	now := time.Now().UTC()
	transfers := []model.TokenTransfer{
		{ChainID: 8453, BlockNumber: 1, TxHash: "0xtx1", LogIndex: 1, TokenAddress: "0xtoken", FromAddress: "0x1111111111111111111111111111111111111111", ToAddress: "0x2222222222222222222222222222222222222222", AmountRaw: "1", Confirmed: true, CreatedAt: now, UpdatedAt: now},
		{ChainID: 8453, BlockNumber: 2, TxHash: "0xtx2", LogIndex: 1, TokenAddress: "0xtoken", FromAddress: "0x1111111111111111111111111111111111111111", ToAddress: "0x2222222222222222222222222222222222222222", AmountRaw: "2", Confirmed: true, CreatedAt: now, UpdatedAt: now},
		{ChainID: 8453, BlockNumber: 3, TxHash: "0xtx3", LogIndex: 1, TokenAddress: "0xtoken", FromAddress: "0x1111111111111111111111111111111111111111", ToAddress: "0x2222222222222222222222222222222222222222", AmountRaw: "3", Confirmed: true, CreatedAt: now, UpdatedAt: now},
	}
	if err := repo.UpsertMany(ctx, transfers); err != nil {
		t.Fatalf("upsert transfers: %v", err)
	}
	if err := repo.MarkRemovedFrom(ctx, 8453, 2); err != nil {
		t.Fatalf("mark removed: %v", err)
	}

	activeCount, err := repo.coll.CountDocuments(ctx, bson.M{"chain_id": int64(8453), "removed": false})
	if err != nil {
		t.Fatalf("count active transfers: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected one active transfer, got %d", activeCount)
	}
	removedCount, err := repo.coll.CountDocuments(ctx, bson.M{"chain_id": int64(8453), "removed": true})
	if err != nil {
		t.Fatalf("count removed transfers: %v", err)
	}
	if removedCount != 2 {
		t.Fatalf("expected two removed transfers, got %d", removedCount)
	}
}
