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

// FindCanonicalRange returns canonical blocks in ascending block number order.
func (r *BlockRepo) FindCanonicalRange(ctx context.Context, chainID int64, from uint64, to uint64) ([]model.Block, error) {
	filter := bson.M{
		"chain_id": chainID,
		"status":   "canonical",
		"block_number": bson.M{
			"$gte": from,
			"$lte": to,
		},
	}
	cursor, err := r.coll.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "block_number", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("find canonical blocks: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var blocks []model.Block
	if err := cursor.All(ctx, &blocks); err != nil {
		return nil, fmt.Errorf("decode canonical blocks: %w", err)
	}
	return blocks, nil
}

// MarkOrphanedFrom marks canonical blocks at and after fromBlock as orphaned.
func (r *BlockRepo) MarkOrphanedFrom(ctx context.Context, chainID int64, fromBlock uint64) error {
	filter := bson.M{"chain_id": chainID, "status": "canonical", "block_number": bson.M{"$gte": fromBlock}}
	update := bson.M{"$set": bson.M{"status": "orphaned", "updated_at": time.Now().UTC()}}
	if _, err := r.coll.UpdateMany(ctx, filter, update); err != nil {
		return fmt.Errorf("mark orphaned blocks: %w", err)
	}
	return nil
}
