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

// MarkRemovedFrom marks non-removed transfers at and after fromBlock as removed.
func (r *TransferRepo) MarkRemovedFrom(ctx context.Context, chainID int64, fromBlock uint64) error {
	filter := bson.M{"chain_id": chainID, "removed": false, "block_number": bson.M{"$gte": fromBlock}}
	update := bson.M{"$set": bson.M{"removed": true, "updated_at": time.Now().UTC()}}
	if _, err := r.coll.UpdateMany(ctx, filter, update); err != nil {
		return fmt.Errorf("mark removed transfers: %w", err)
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

// AddressSummary aggregates transfer counts and token coverage for one address.
func (r *TransferRepo) AddressSummary(ctx context.Context, chainID int64, address string) (model.AddressSummary, error) {
	address = strings.ToLower(address)
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"chain_id": chainID,
			"removed":  false,
			"$or":      bson.A{bson.M{"from_address": address}, bson.M{"to_address": address}},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":            nil,
			"sent_count":     bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$from_address", address}}, 1, 0}}},
			"received_count": bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$to_address", address}}, 1, 0}}},
			"total_count":    bson.M{"$sum": 1},
			"latest_block":   bson.M{"$max": "$block_number"},
			"tokens":         bson.M{"$addToSet": "$token_address"},
		}}},
		{{Key: "$project", Value: bson.M{
			"_id":            0,
			"chain_id":       chainID,
			"address":        address,
			"sent_count":     1,
			"received_count": 1,
			"total_count":    1,
			"latest_block":   1,
			"token_count":    bson.M{"$size": "$tokens"},
			"tokens":         1,
		}}},
	}
	cursor, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return model.AddressSummary{}, fmt.Errorf("aggregate address summary: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	if cursor.Next(ctx) {
		var summary model.AddressSummary
		if err := cursor.Decode(&summary); err != nil {
			return model.AddressSummary{}, fmt.Errorf("decode address summary: %w", err)
		}
		return summary, nil
	}
	if err := cursor.Err(); err != nil {
		return model.AddressSummary{}, fmt.Errorf("read address summary: %w", err)
	}
	return model.AddressSummary{ChainID: chainID, Address: address, Tokens: []string{}}, nil
}

func (r *TransferRepo) DetectAddress(ctx context.Context, chainID int64, address string) (model.AddressDetection, error) {
	summary, err := r.AddressSummary(ctx, chainID, address)
	if err != nil {
		return model.AddressDetection{}, fmt.Errorf("detect address: %w", err)
	}
	return buildAddressDetection(summary), nil
}

func buildAddressDetection(summary model.AddressSummary) model.AddressDetection {
	detection := model.AddressDetection{
		ChainID: summary.ChainID,
		Address: summary.Address,
		Level:   "normal",
		Tags:    []string{},
		Reasons: []string{},
		Summary: summary,
	}
	if summary.TotalCount == 0 {
		detection.Level = "no_data"
		detection.Reasons = append(detection.Reasons, "no indexed transfers for this address")
		return detection
	}

	if summary.TotalCount >= 20 {
		detection.Score += 40
		detection.Tags = append(detection.Tags, "active_wallet")
		detection.Reasons = append(detection.Reasons, "20 or more indexed transfers")
	} else if summary.TotalCount >= 5 {
		detection.Score += 20
		detection.Tags = append(detection.Tags, "active_wallet")
		detection.Reasons = append(detection.Reasons, "5 or more indexed transfers")
	}
	if summary.TokenCount >= 5 {
		detection.Score += 30
		detection.Tags = append(detection.Tags, "multi_token")
		detection.Reasons = append(detection.Reasons, "interacted with 5 or more tokens")
	} else if summary.TokenCount >= 2 {
		detection.Score += 15
		detection.Tags = append(detection.Tags, "multi_token")
		detection.Reasons = append(detection.Reasons, "interacted with 2 or more tokens")
	}
	if summary.SentCount > 0 && summary.ReceivedCount > 0 {
		detection.Score += 20
		detection.Tags = append(detection.Tags, "two_way_flow")
		detection.Reasons = append(detection.Reasons, "has both sent and received transfers")
	}
	if detection.Score > 100 {
		detection.Score = 100
	}

	if detection.Score >= 70 {
		detection.Level = "smart_money_candidate"
	} else if detection.Score >= 40 {
		detection.Level = "watchlist"
	}
	return detection
}
