package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"nano-indexer/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type SyncStateRepo struct {
	coll *mongo.Collection
}

func NewSyncStateRepo(db *mongo.Database) *SyncStateRepo {
	return &SyncStateRepo{coll: db.Collection("sync_states")}
}

func (r *SyncStateRepo) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "chain_id", Value: 1}, {Key: "scanner_name", Value: 1}, {Key: "token_address", Value: 1}}, Options: options.Index().SetUnique(true)},
	})
	if err != nil {
		return fmt.Errorf("create sync_states indexes: %w", err)
	}
	return nil
}

func (r *SyncStateRepo) GetOrCreate(ctx context.Context, chainID int64, scannerName string, tokenAddress string, fromBlock uint64, confirmations uint64) (model.SyncState, error) {
	filter := bson.M{"chain_id": chainID, "scanner_name": scannerName, "token_address": tokenAddress}
	var state model.SyncState
	err := r.coll.FindOne(ctx, filter).Decode(&state)
	if err == nil {
		return state, nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return model.SyncState{}, fmt.Errorf("find sync state: %w", err)
	}
	now := time.Now().UTC()
	latestScannedBlock := uint64(0)
	if fromBlock > 0 {
		latestScannedBlock = fromBlock - 1
	}
	state = model.SyncState{
		ChainID: chainID, ScannerName: scannerName, TokenAddress: tokenAddress,
		FromBlock: fromBlock, LatestScannedBlock: latestScannedBlock, Confirmations: confirmations,
		Status: "running", UpdatedAt: now,
	}
	_, err = r.coll.UpdateOne(ctx, filter, bson.M{"$setOnInsert": state}, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return model.SyncState{}, fmt.Errorf("create sync state: %w", err)
	}
	return state, nil
}

func (r *SyncStateRepo) UpdateProgress(ctx context.Context, state model.SyncState, scannedBlock uint64, confirmedBlock uint64) error {
	if scannedBlock < state.LatestScannedBlock {
		return fmt.Errorf("refuse to move sync state backward from %d to %d", state.LatestScannedBlock, scannedBlock)
	}
	filter := bson.M{"chain_id": state.ChainID, "scanner_name": state.ScannerName, "token_address": state.TokenAddress}
	update := bson.M{"$set": bson.M{
		"latest_scanned_block":   scannedBlock,
		"latest_confirmed_block": confirmedBlock,
		"status":                 "running",
		"last_error":             "",
		"updated_at":             time.Now().UTC(),
	}}
	if _, err := r.coll.UpdateOne(ctx, filter, update); err != nil {
		return fmt.Errorf("update sync state progress: %w", err)
	}
	return nil
}
