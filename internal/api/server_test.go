package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"nano-indexer/internal/model"
)

func TestHealthzReturnsOK(t *testing.T) {
	server := NewServer()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "{\"status\":\"ok\"}\n" {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}
}

func TestTransfersRejectsMissingFilters(t *testing.T) {
	server := NewServer(&fakeTransferReader{})

	req := httptest.NewRequest(http.MethodGet, "/transfers", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestTransfersRejectsInvalidLimit(t *testing.T) {
	server := NewServer(&fakeTransferReader{})

	req := httptest.NewRequest(http.MethodGet, "/transfers?address=0x1111111111111111111111111111111111111111&limit=201", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestTransfersReturnsRepoResults(t *testing.T) {
	reader := fakeTransferReader{
		transfers: []model.TokenTransfer{{ChainID: 8453, FromAddress: "0x1111111111111111111111111111111111111111"}},
	}
	server := NewServer(&reader)

	req := httptest.NewRequest(http.MethodGet, "/transfers?chain_id=8453&address=0x1111111111111111111111111111111111111111&limit=1", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if reader.lastFindChainID != 8453 || reader.lastFindAddress != "0x1111111111111111111111111111111111111111" || reader.lastFindLimit != 1 {
		t.Fatalf("unexpected repo call: chain=%d address=%q limit=%d", reader.lastFindChainID, reader.lastFindAddress, reader.lastFindLimit)
	}
}

func TestAddressSummaryReturnsRepoResult(t *testing.T) {
	reader := fakeTransferReader{
		summary: model.AddressSummary{ChainID: 8453, Address: "0x1111111111111111111111111111111111111111", SentCount: 2, ReceivedCount: 3, TotalCount: 5, LatestBlock: 10, TokenCount: 1, Tokens: []string{"0xtoken"}},
	}
	server := NewServer(&reader)

	req := httptest.NewRequest(http.MethodGet, "/addresses/0x1111111111111111111111111111111111111111/summary?chain_id=8453", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var summary model.AddressSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.TotalCount != 5 || summary.TokenCount != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

type fakeTransferReader struct {
	transfers []model.TokenTransfer
	summary   model.AddressSummary

	lastFindChainID int64
	lastFindAddress string
	lastFindLimit   int64
}

func (f *fakeTransferReader) Find(_ context.Context, chainID int64, address string, _ string, limit int64) ([]model.TokenTransfer, error) {
	f.lastFindChainID = chainID
	f.lastFindAddress = address
	f.lastFindLimit = limit
	return f.transfers, nil
}

func (f *fakeTransferReader) AddressSummary(_ context.Context, chainID int64, address string) (model.AddressSummary, error) {
	if f.summary.Address == "" {
		return model.AddressSummary{}, fmt.Errorf("missing summary for %s/%d", address, chainID)
	}
	return f.summary, nil
}
