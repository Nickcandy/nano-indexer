package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nano-indexer/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type TransferRepo struct {
	coll *mongo.Collection
}

func NewTransferRepo(db *mongo.Database) *TransferRepo {
	return &TransferRepo{coll: db.Collection("token_transfers")}
}

func (r *TransferRepo) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "chain_id", Value: 1}, {Key: "tx_hash", Value: 1}, {Key: "log_index", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "chain_id", Value: 1}, {Key: "block_number", Value: 1}}},
		{Keys: bson.D{{Key: "chain_id", Value: 1}, {Key: "token_address", Value: 1}, {Key: "block_number", Value: -1}}},
		{Keys: bson.D{{Key: "chain_id", Value: 1}, {Key: "from_address", Value: 1}, {Key: "block_number", Value: -1}}},
		{Keys: bson.D{{Key: "chain_id", Value: 1}, {Key: "to_address", Value: 1}, {Key: "block_number", Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("create token_transfers indexes: %w", err)
	}
	return nil
}

func (r *TransferRepo) UpsertMany(ctx context.Context, transfers []model.TokenTransfer) error {
	if len(transfers) == 0 {
		return nil
	}
	models := make([]mongo.WriteModel, 0, len(transfers))
	for _, transfer := range transfers {
		transfer.UpdatedAt = time.Now().UTC()
		set := bson.M{
			"chain_id":      transfer.ChainID,
			"block_number":  transfer.BlockNumber,
			"block_hash":    transfer.BlockHash,
			"tx_hash":       transfer.TxHash,
			"log_index":     transfer.LogIndex,
			"tx_index":      transfer.TxIndex,
			"token_address": transfer.TokenAddress,
			"from_address":  transfer.FromAddress,
			"to_address":    transfer.ToAddress,
			"amount_raw":    transfer.AmountRaw,
			"confirmed":     transfer.Confirmed,
			"removed":       transfer.Removed,
			"event_time":    transfer.EventTime,
			"updated_at":    transfer.UpdatedAt,
		}
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"chain_id": transfer.ChainID, "tx_hash": transfer.TxHash, "log_index": transfer.LogIndex}).
			SetUpdate(bson.M{"$set": set, "$setOnInsert": bson.M{"created_at": transfer.CreatedAt}}).
			SetUpsert(true))
	}
	if _, err := r.coll.BulkWrite(ctx, models); err != nil {
		return fmt.Errorf("upsert token transfers: %w", err)
	}
	return nil
}

func (r *TransferRepo) Find(ctx context.Context, chainID int64, address string, token string, limit int64) ([]model.TokenTransfer, error) {
	filter := bson.M{"chain_id": chainID, "removed": false}
	if address != "" {
		addr := strings.ToLower(address)
		filter["$or"] = bson.A{bson.M{"from_address": addr}, bson.M{"to_address": addr}}
	}
	if token != "" {
		filter["token_address"] = strings.ToLower(token)
	}
	cursor, err := r.coll.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "block_number", Value: -1}, {Key: "log_index", Value: -1}}).SetLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("find token transfers: %w", err)
	}
	defer cursor.Close(ctx)

	var transfers []model.TokenTransfer
	if err := cursor.All(ctx, &transfers); err != nil {
		return nil, fmt.Errorf("decode token transfers: %w", err)
	}
	return transfers, nil
}
