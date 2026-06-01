package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"nano-indexer/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestSyncStateUpdateProgressRejectsBackwardMove(t *testing.T) {
	repo := &SyncStateRepo{}
	state := model.SyncState{LatestScannedBlock: 10}

	err := repo.UpdateProgress(context.Background(), state, 9, 9)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSyncStateRollbackScanner(t *testing.T) {
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
	repo := NewSyncStateRepo(db)
	if err := repo.EnsureIndexes(ctx); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}

	now := time.Now().UTC()
	_, err = repo.coll.InsertMany(ctx, []any{
		model.SyncState{ChainID: 8453, ScannerName: "erc20_transfer", TokenAddress: "0xtoken1", LatestScannedBlock: 5, LatestConfirmedBlock: 5, Status: "running", UpdatedAt: now},
		model.SyncState{ChainID: 8453, ScannerName: "erc20_transfer", TokenAddress: "0xtoken2", LatestScannedBlock: 2, LatestConfirmedBlock: 5, Status: "running", UpdatedAt: now},
		model.SyncState{ChainID: 1, ScannerName: "erc20_transfer", TokenAddress: "0xtoken3", LatestScannedBlock: 5, LatestConfirmedBlock: 5, Status: "running", UpdatedAt: now},
	})
	if err != nil {
		t.Fatalf("insert sync states: %v", err)
	}

	if err := repo.RollbackScanner(ctx, 8453, "erc20_transfer", 3, 4); err != nil {
		t.Fatalf("rollback scanner: %v", err)
	}

	var rolledBack model.SyncState
	if err := repo.coll.FindOne(ctx, bson.M{"chain_id": int64(8453), "token_address": "0xtoken1"}).Decode(&rolledBack); err != nil {
		t.Fatalf("find rolled back state: %v", err)
	}
	if rolledBack.LatestScannedBlock != 3 || rolledBack.LatestConfirmedBlock != 4 {
		t.Fatalf("unexpected rolled back state: %+v", rolledBack)
	}

	unchangedCount, err := repo.coll.CountDocuments(ctx, bson.M{"latest_scanned_block": uint64(5)})
	if err != nil {
		t.Fatalf("count unchanged states: %v", err)
	}
	if unchangedCount != 1 {
		t.Fatalf("expected one unchanged high state from another chain, got %d", unchangedCount)
	}
}
