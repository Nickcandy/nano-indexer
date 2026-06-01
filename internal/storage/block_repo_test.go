package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"nano-indexer/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestBlockRepoMarkOrphanedFrom(t *testing.T) {
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
	repo := NewBlockRepo(db)
	if err := repo.EnsureIndexes(ctx); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}

	now := time.Now().UTC()
	blocks := []model.Block{
		{ChainID: 8453, BlockNumber: 1, BlockHash: "0x1", ParentHash: "0x0", Status: "canonical", BlockTime: now, CreatedAt: now, UpdatedAt: now},
		{ChainID: 8453, BlockNumber: 2, BlockHash: "0x2", ParentHash: "0x1", Status: "canonical", BlockTime: now, CreatedAt: now, UpdatedAt: now},
		{ChainID: 8453, BlockNumber: 3, BlockHash: "0x3", ParentHash: "0x2", Status: "canonical", BlockTime: now, CreatedAt: now, UpdatedAt: now},
	}
	if err := repo.UpsertMany(ctx, blocks); err != nil {
		t.Fatalf("upsert blocks: %v", err)
	}
	if err := repo.MarkOrphanedFrom(ctx, 8453, 2); err != nil {
		t.Fatalf("mark orphaned: %v", err)
	}

	canonicalCount, err := repo.coll.CountDocuments(ctx, bson.M{"chain_id": int64(8453), "status": "canonical"})
	if err != nil {
		t.Fatalf("count canonical blocks: %v", err)
	}
	if canonicalCount != 1 {
		t.Fatalf("expected one canonical block, got %d", canonicalCount)
	}
	orphanedCount, err := repo.coll.CountDocuments(ctx, bson.M{"chain_id": int64(8453), "status": "orphaned"})
	if err != nil {
		t.Fatalf("count orphaned blocks: %v", err)
	}
	if orphanedCount != 2 {
		t.Fatalf("expected two orphaned blocks, got %d", orphanedCount)
	}
}
