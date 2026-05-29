package storage

import (
	"context"
	"fmt"
	"time"

	"nano-indexer/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type BlockRepo struct {
	coll *mongo.Collection
}

// NewBlockRepo returns a repository for the blocks collection.
func NewBlockRepo(db *mongo.Database) *BlockRepo {
	return &BlockRepo{coll: db.Collection("blocks")}
}

// EnsureIndexes creates the indexes needed for block lookup and idempotent writes.
func (r *BlockRepo) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "chain_id", Value: 1}, {Key: "block_number", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "chain_id", Value: 1}, {Key: "block_hash", Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("create blocks indexes: %w", err)
	}
	return nil
}

// UpsertMany idempotently stores block metadata keyed by chain and block number.
func (r *BlockRepo) UpsertMany(ctx context.Context, blocks []model.Block) error {
	if len(blocks) == 0 {
		return nil
	}
	models := make([]mongo.WriteModel, 0, len(blocks))
	for _, block := range blocks {
		block.UpdatedAt = time.Now().UTC()
		set := bson.M{
			"chain_id":     block.ChainID,
			"block_number": block.BlockNumber,
			"block_hash":   block.BlockHash,
			"parent_hash":  block.ParentHash,
			"block_time":   block.BlockTime,
			"status":       block.Status,
			"updated_at":   block.UpdatedAt,
		}
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"chain_id": block.ChainID, "block_number": block.BlockNumber}).
			SetUpdate(bson.M{"$set": set, "$setOnInsert": bson.M{"created_at": block.CreatedAt}}).
			SetUpsert(true))
	}
	if _, err := r.coll.BulkWrite(ctx, models); err != nil {
		return fmt.Errorf("upsert blocks: %w", err)
	}
	return nil
}
