package storage

import (
	"context"
	"testing"

	"nano-indexer/internal/model"
)

func TestSyncStateUpdateProgressRejectsBackwardMove(t *testing.T) {
	repo := &SyncStateRepo{}
	state := model.SyncState{LatestScannedBlock: 10}

	err := repo.UpdateProgress(context.Background(), state, 9, 9)
	if err == nil {
		t.Fatal("expected error")
	}
}
