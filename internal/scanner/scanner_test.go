package scanner

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"nano-indexer/internal/config"
	"nano-indexer/internal/eth"
	"nano-indexer/internal/model"
)

func TestScanOnceReturnsRPCErrorWithoutUpdatingProgress(t *testing.T) {
	cfg := testConfig()
	client := &fakeEthReader{latest: 10, filterErr: fmt.Errorf("rpc down")}
	states := &fakeSyncStateStore{state: model.SyncState{ChainID: 8453, ScannerName: scannerName, TokenAddress: cfg.Scanner.TokenAddresses[0], LatestScannedBlock: 0}}
	transfers := &fakeTransferWriter{}
	blocks := &fakeBlockWriter{}
	scanner := &Scanner{cfg: cfg, ethClient: client, transfers: transfers, states: states, blocks: blocks, logger: testLogger()}

	err := scanner.ScanOnce(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if states.updateCalls != 0 {
		t.Fatalf("expected no progress update, got %d", states.updateCalls)
	}
	if transfers.calls != 0 {
		t.Fatalf("expected no transfer writes, got %d", transfers.calls)
	}
	if blocks.calls != 0 {
		t.Fatalf("expected no block writes, got %d", blocks.calls)
	}
}

func TestScanOnceWritesBlocksBeforeProgress(t *testing.T) {
	cfg := testConfig()
	cfg.Eth.BatchSize = 2
	client := &fakeEthReader{
		latest: 2,
		blocks: map[uint64]eth.Block{
			1: {Number: "0x1", Hash: "0xaaa", ParentHash: "0x000", Timestamp: "0x64"},
			2: {Number: "0x2", Hash: "0xbbb", ParentHash: "0xaaa", Timestamp: "0x65"},
		},
	}
	states := &fakeSyncStateStore{state: model.SyncState{ChainID: 8453, ScannerName: scannerName, TokenAddress: cfg.Scanner.TokenAddresses[0], LatestScannedBlock: 0}}
	transfers := &fakeTransferWriter{}
	blocks := &fakeBlockWriter{}
	scanner := &Scanner{cfg: cfg, ethClient: client, transfers: transfers, states: states, blocks: blocks, logger: testLogger()}

	if err := scanner.ScanOnce(context.Background()); err != nil {
		t.Fatalf("scan once: %v", err)
	}
	if len(blocks.blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks.blocks))
	}
	if states.updateCalls != 1 || states.scannedBlock != 2 || states.confirmedBlock != 2 {
		t.Fatalf("unexpected progress update: calls=%d scanned=%d confirmed=%d", states.updateCalls, states.scannedBlock, states.confirmedBlock)
	}
}

func testConfig() config.Config {
	return config.Config{
		Eth: config.EthConfig{
			ChainID:           8453,
			Confirmations:     0,
			DefaultStartBlock: 1,
			BatchSize:         10,
		},
		Scanner: config.ScannerConfig{TokenAddresses: []string{"0x1111111111111111111111111111111111111111"}},
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeEthReader struct {
	latest    uint64
	filterErr error
	blocks    map[uint64]eth.Block
}

func (f *fakeEthReader) BlockNumber(context.Context) (uint64, error) {
	return f.latest, nil
}

func (f *fakeEthReader) BlockByNumber(_ context.Context, number uint64) (eth.Block, error) {
	block, ok := f.blocks[number]
	if !ok {
		return eth.Block{}, fmt.Errorf("missing block %d", number)
	}
	return block, nil
}

func (f *fakeEthReader) FilterTransferLogs(context.Context, uint64, uint64, string) ([]eth.Log, error) {
	if f.filterErr != nil {
		return nil, f.filterErr
	}
	return nil, nil
}

type fakeTransferWriter struct {
	calls int
}

func (f *fakeTransferWriter) UpsertMany(context.Context, []model.TokenTransfer) error {
	f.calls++
	return nil
}

type fakeSyncStateStore struct {
	state          model.SyncState
	updateCalls    int
	scannedBlock   uint64
	confirmedBlock uint64
}

func (f *fakeSyncStateStore) GetOrCreate(context.Context, int64, string, string, uint64, uint64) (model.SyncState, error) {
	return f.state, nil
}

func (f *fakeSyncStateStore) UpdateProgress(_ context.Context, _ model.SyncState, scannedBlock uint64, confirmedBlock uint64) error {
	f.updateCalls++
	f.scannedBlock = scannedBlock
	f.confirmedBlock = confirmedBlock
	return nil
}

type fakeBlockWriter struct {
	calls  int
	blocks []model.Block
}

func (f *fakeBlockWriter) UpsertMany(_ context.Context, blocks []model.Block) error {
	f.calls++
	f.blocks = append(f.blocks, blocks...)
	return nil
}
