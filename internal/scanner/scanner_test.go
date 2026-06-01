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
	states := &fakeSyncStateStore{state: model.SyncState{ChainID: 8453, ScannerName: scannerName, TokenAddress: cfg.Scanner.TokenAddresses[0], FromBlock: 1, LatestScannedBlock: 0}}
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

func TestScanOnceWritesRecentBlocksBeforeProgress(t *testing.T) {
	cfg := testConfig()
	cfg.Eth.Confirmations = 2
	cfg.Eth.BatchSize = 10
	client := &fakeEthReader{
		latest: 10,
		blocks: map[uint64]eth.Block{
			7: {Number: "0x7", Hash: "0xggg", ParentHash: "0xfff", Timestamp: "0x6a"},
			8: {Number: "0x8", Hash: "0xhhh", ParentHash: "0xggg", Timestamp: "0x6b"},
		},
	}
	states := &fakeSyncStateStore{state: model.SyncState{ChainID: 8453, ScannerName: scannerName, TokenAddress: cfg.Scanner.TokenAddresses[0], FromBlock: 1, LatestScannedBlock: 0}}
	transfers := &fakeTransferWriter{}
	blocks := &fakeBlockWriter{}
	scanner := &Scanner{cfg: cfg, ethClient: client, transfers: transfers, states: states, blocks: blocks, logger: testLogger()}

	if err := scanner.ScanOnce(context.Background()); err != nil {
		t.Fatalf("scan once: %v", err)
	}
	if len(blocks.blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks.blocks))
	}
	if fmt.Sprint(client.blockCalls) != "[7 8]" {
		t.Fatalf("expected only recent block calls, got %v", client.blockCalls)
	}
	if states.updateCalls != 1 || states.scannedBlock != 8 || states.confirmedBlock != 8 {
		t.Fatalf("unexpected progress update: calls=%d scanned=%d confirmed=%d", states.updateCalls, states.scannedBlock, states.confirmedBlock)
	}
}

func TestScanOnceBuildsLogBlocksWithoutFetchingHeadersWhenReorgDisabled(t *testing.T) {
	cfg := testConfig()
	cfg.Eth.Confirmations = 0
	cfg.Eth.BatchSize = 10
	log := testTransferLog()
	log.BlockNumber = "0x3"
	log.BlockHash = "0xccc"
	log.BlockTimestamp = "0x64"
	client := &fakeEthReader{latest: 10, logs: []eth.Log{log}}
	states := &fakeSyncStateStore{state: model.SyncState{ChainID: 8453, ScannerName: scannerName, TokenAddress: cfg.Scanner.TokenAddresses[0], FromBlock: 1, LatestScannedBlock: 0}}
	transfers := &fakeTransferWriter{}
	blocks := &fakeBlockWriter{}
	scanner := &Scanner{cfg: cfg, ethClient: client, transfers: transfers, states: states, blocks: blocks, logger: testLogger()}

	if err := scanner.ScanOnce(context.Background()); err != nil {
		t.Fatalf("scan once: %v", err)
	}
	if len(client.blockCalls) != 0 {
		t.Fatalf("expected no block header calls, got %v", client.blockCalls)
	}
	if len(blocks.blocks) != 1 || blocks.blocks[0].BlockNumber != 3 || blocks.blocks[0].BlockHash != "0xccc" {
		t.Fatalf("expected one log-derived block, got %+v", blocks.blocks)
	}
	if len(transfers.transfers) != 1 || transfers.transfers[0].BlockNumber != 3 {
		t.Fatalf("expected transfer from log, got %+v", transfers.transfers)
	}
}

func TestScanOnceSkipsRollbackWhenRecentBlockHashesMatch(t *testing.T) {
	cfg := testConfig()
	cfg.Eth.Confirmations = 2
	cfg.Eth.BatchSize = 1
	client := &fakeEthReader{
		latest: 5,
		blocks: map[uint64]eth.Block{
			1: {Number: "0x1", Hash: "0xaaa", ParentHash: "0x000", Timestamp: "0x64"},
			2: {Number: "0x2", Hash: "0xbbb", ParentHash: "0xaaa", Timestamp: "0x65"},
			3: {Number: "0x3", Hash: "0xccc", ParentHash: "0xbbb", Timestamp: "0x66"},
		},
	}
	states := &fakeSyncStateStore{state: model.SyncState{
		ChainID: 8453, ScannerName: scannerName, TokenAddress: cfg.Scanner.TokenAddresses[0],
		FromBlock: 1, LatestScannedBlock: 2,
	}}
	transfers := &fakeTransferWriter{}
	blocks := &fakeBlockWriter{canonical: []model.Block{
		{ChainID: 8453, BlockNumber: 1, BlockHash: "0xaaa", Status: "canonical"},
		{ChainID: 8453, BlockNumber: 2, BlockHash: "0xbbb", Status: "canonical"},
	}}
	scanner := &Scanner{cfg: cfg, ethClient: client, transfers: transfers, states: states, blocks: blocks, logger: testLogger()}

	if err := scanner.ScanOnce(context.Background()); err != nil {
		t.Fatalf("scan once: %v", err)
	}
	if states.rollbackCalls != 0 {
		t.Fatalf("expected no rollback, got %d", states.rollbackCalls)
	}
	if blocks.orphanCalls != 0 {
		t.Fatalf("expected no orphan mark, got %d", blocks.orphanCalls)
	}
	if transfers.removedCalls != 0 {
		t.Fatalf("expected no transfer removal, got %d", transfers.removedCalls)
	}
	if states.scannedBlock != 3 {
		t.Fatalf("expected scan to continue at block 3, got %d", states.scannedBlock)
	}
}

func TestScanOnceRollsBackAndRescansFromReorgMismatch(t *testing.T) {
	cfg := testConfig()
	cfg.Eth.Confirmations = 3
	cfg.Eth.BatchSize = 10
	client := &fakeEthReader{
		latest: 8,
		blocks: map[uint64]eth.Block{
			1: {Number: "0x1", Hash: "0xaaa", ParentHash: "0x000", Timestamp: "0x64"},
			2: {Number: "0x2", Hash: "0xnew2", ParentHash: "0xaaa", Timestamp: "0x65"},
			3: {Number: "0x3", Hash: "0xnew3", ParentHash: "0xnew2", Timestamp: "0x66"},
			4: {Number: "0x4", Hash: "0xnew4", ParentHash: "0xnew3", Timestamp: "0x67"},
			5: {Number: "0x5", Hash: "0xnew5", ParentHash: "0xnew4", Timestamp: "0x68"},
		},
	}
	states := &fakeSyncStateStore{state: model.SyncState{
		ChainID: 8453, ScannerName: scannerName, TokenAddress: cfg.Scanner.TokenAddresses[0],
		FromBlock: 1, LatestScannedBlock: 4,
	}}
	transfers := &fakeTransferWriter{}
	blocks := &fakeBlockWriter{canonical: []model.Block{
		{ChainID: 8453, BlockNumber: 2, BlockHash: "0xold2", Status: "canonical"},
		{ChainID: 8453, BlockNumber: 3, BlockHash: "0xold3", Status: "canonical"},
		{ChainID: 8453, BlockNumber: 4, BlockHash: "0xold4", Status: "canonical"},
	}}
	scanner := &Scanner{cfg: cfg, ethClient: client, transfers: transfers, states: states, blocks: blocks, logger: testLogger()}

	if err := scanner.ScanOnce(context.Background()); err != nil {
		t.Fatalf("scan once: %v", err)
	}
	if blocks.orphanFrom != 2 {
		t.Fatalf("expected orphan from block 2, got %d", blocks.orphanFrom)
	}
	if transfers.removedFrom != 2 {
		t.Fatalf("expected remove transfers from block 2, got %d", transfers.removedFrom)
	}
	if states.rollbackTo != 1 {
		t.Fatalf("expected rollback to block 1, got %d", states.rollbackTo)
	}
	if states.scannedBlock != 5 {
		t.Fatalf("expected rescan through safe block 5, got %d", states.scannedBlock)
	}
	if len(blocks.blocks) != 3 || blocks.blocks[0].BlockNumber != 3 {
		t.Fatalf("expected recent rescan blocks 3-5, got %+v", blocks.blocks)
	}
}

func TestScanOnceReturnsReorgCheckRPCErrorWithoutMutating(t *testing.T) {
	cfg := testConfig()
	cfg.Eth.Confirmations = 1
	client := &fakeEthReader{
		latest:   4,
		blockErr: fmt.Errorf("rpc down"),
	}
	states := &fakeSyncStateStore{state: model.SyncState{
		ChainID: 8453, ScannerName: scannerName, TokenAddress: cfg.Scanner.TokenAddresses[0],
		FromBlock: 1, LatestScannedBlock: 3,
	}}
	transfers := &fakeTransferWriter{}
	blocks := &fakeBlockWriter{canonical: []model.Block{{ChainID: 8453, BlockNumber: 3, BlockHash: "0xold", Status: "canonical"}}}
	scanner := &Scanner{cfg: cfg, ethClient: client, transfers: transfers, states: states, blocks: blocks, logger: testLogger()}

	err := scanner.ScanOnce(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if states.rollbackCalls != 0 || states.updateCalls != 0 {
		t.Fatalf("expected no state mutation, rollback=%d update=%d", states.rollbackCalls, states.updateCalls)
	}
	if blocks.orphanCalls != 0 || transfers.removedCalls != 0 {
		t.Fatalf("expected no mark mutation, orphan=%d removed=%d", blocks.orphanCalls, transfers.removedCalls)
	}
}

func TestScanOnceReturnsErrorWhenScannedCanonicalBlockIsMissing(t *testing.T) {
	cfg := testConfig()
	cfg.Eth.Confirmations = 2
	client := &fakeEthReader{latest: 5}
	states := &fakeSyncStateStore{state: model.SyncState{
		ChainID: 8453, ScannerName: scannerName, TokenAddress: cfg.Scanner.TokenAddresses[0],
		FromBlock: 1, LatestScannedBlock: 3,
	}}
	transfers := &fakeTransferWriter{}
	blocks := &fakeBlockWriter{canonical: []model.Block{{ChainID: 8453, BlockNumber: 3, BlockHash: "0xccc", Status: "canonical"}}}
	scanner := &Scanner{cfg: cfg, ethClient: client, transfers: transfers, states: states, blocks: blocks, logger: testLogger()}

	err := scanner.ScanOnce(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if states.updateCalls != 0 {
		t.Fatalf("expected no progress update, got %d", states.updateCalls)
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

func testTransferLog() eth.Log {
	return eth.Log{
		Address:          "0x1111111111111111111111111111111111111111",
		Topics:           []string{eth.TransferTopic, "0x0000000000000000000000002222222222222222222222222222222222222222", "0x0000000000000000000000003333333333333333333333333333333333333333"},
		Data:             "0x0000000000000000000000000000000000000000000000000000000000000064",
		BlockNumber:      "0x1",
		BlockHash:        "0xaaa",
		BlockTimestamp:   "0x64",
		TransactionHash:  "0xtx",
		TransactionIndex: "0x0",
		LogIndex:         "0x0",
	}
}

type fakeEthReader struct {
	latest     uint64
	filterErr  error
	blockErr   error
	blocks     map[uint64]eth.Block
	logs       []eth.Log
	blockCalls []uint64
}

func (f *fakeEthReader) BlockNumber(context.Context) (uint64, error) {
	return f.latest, nil
}

func (f *fakeEthReader) BlockByNumber(_ context.Context, number uint64) (eth.Block, error) {
	f.blockCalls = append(f.blockCalls, number)
	if f.blockErr != nil {
		return eth.Block{}, f.blockErr
	}
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
	return f.logs, nil
}

type fakeTransferWriter struct {
	calls        int
	removedCalls int
	removedFrom  uint64
	transfers    []model.TokenTransfer
}

func (f *fakeTransferWriter) UpsertMany(_ context.Context, transfers []model.TokenTransfer) error {
	f.calls++
	f.transfers = append(f.transfers, transfers...)
	return nil
}

func (f *fakeTransferWriter) MarkRemovedFrom(_ context.Context, _ int64, fromBlock uint64) error {
	f.removedCalls++
	f.removedFrom = fromBlock
	return nil
}

type fakeSyncStateStore struct {
	state          model.SyncState
	updateCalls    int
	rollbackCalls  int
	scannedBlock   uint64
	confirmedBlock uint64
	rollbackTo     uint64
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

func (f *fakeSyncStateStore) RollbackScanner(_ context.Context, _ int64, _ string, rollbackTo uint64, confirmedBlock uint64) error {
	f.rollbackCalls++
	f.rollbackTo = rollbackTo
	f.confirmedBlock = confirmedBlock
	return nil
}

type fakeBlockWriter struct {
	calls       int
	orphanCalls int
	orphanFrom  uint64
	blocks      []model.Block
	canonical   []model.Block
}

func (f *fakeBlockWriter) UpsertMany(_ context.Context, blocks []model.Block) error {
	f.calls++
	f.blocks = append(f.blocks, blocks...)
	return nil
}

func (f *fakeBlockWriter) FindCanonicalRange(_ context.Context, _ int64, from uint64, to uint64) ([]model.Block, error) {
	var blocks []model.Block
	for _, block := range f.canonical {
		if block.BlockNumber >= from && block.BlockNumber <= to {
			blocks = append(blocks, block)
		}
	}
	return blocks, nil
}

func (f *fakeBlockWriter) MarkOrphanedFrom(_ context.Context, _ int64, fromBlock uint64) error {
	f.orphanCalls++
	f.orphanFrom = fromBlock
	return nil
}
