package eth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	url        string
	httpClient *http.Client
}

type Log struct {
	Address          string   `json:"address"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"`
	BlockNumber      string   `json:"blockNumber"`
	BlockHash        string   `json:"blockHash"`
	TransactionHash  string   `json:"transactionHash"`
	TransactionIndex string   `json:"transactionIndex"`
	LogIndex         string   `json:"logIndex"`
	Removed          bool     `json:"removed"`
}

func NewClient(url string) *Client {
	return &Client{
		url:        strings.TrimSpace(url),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) BlockNumber(ctx context.Context) (uint64, error) {
	var hex string
	if err := c.call(ctx, "eth_blockNumber", []any{}, &hex); err != nil {
		return 0, err
	}
	return ParseHexUint64(hex)
}

func (c *Client) FilterTransferLogs(ctx context.Context, fromBlock, toBlock uint64, tokenAddress string) ([]Log, error) {
	filter := map[string]any{
		"fromBlock": Uint64Hex(fromBlock),
		"toBlock":   Uint64Hex(toBlock),
		"address":   strings.ToLower(tokenAddress),
		"topics":    []any{TransferTopic},
	}
	var logs []Log
	if err := c.call(ctx, "eth_getLogs", []any{filter}, &logs); err != nil {
		return nil, err
	}
	return logs, nil
}

func (c *Client) call(ctx context.Context, method string, params []any, result any) error {
	if c.url == "" {
		return fmt.Errorf("rpc url is empty")
	}
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return fmt.Errorf("marshal rpc request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create rpc request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send rpc request %s: %w", method, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read rpc response %s: %w", method, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("rpc %s returned status %d: %s", method, resp.StatusCode, string(respBody))
	}

	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("decode rpc response %s: %w", method, err)
	}
	if envelope.Error != nil {
		return fmt.Errorf("rpc %s error %d: %s", method, envelope.Error.Code, envelope.Error.Message)
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("decode rpc result %s: %w", method, err)
	}
	return nil
}

func Uint64Hex(n uint64) string {
	return "0x" + strconv.FormatUint(n, 16)
}

func ParseHexUint64(value string) (uint64, error) {
	value = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "0x")
	if value == "" {
		return 0, nil
	}
	n, err := strconv.ParseUint(value, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("parse hex uint64 %q: %w", value, err)
	}
	return n, nil
}
