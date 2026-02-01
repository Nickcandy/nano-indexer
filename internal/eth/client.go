package eth

import (
	"context"
	"math/big"
	"nano-indexer/pkg/logger"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
)

// Client wraps the ethclient.Client for Ethereum JSON-RPC interactions.
type Client struct {
	client *ethclient.Client
}

// NewClient creates a new Ethereum client.
func NewClient(rpcURL string) (*Client, error) {
	c, err := ethclient.Dial(rpcURL)
	if err != nil {
		logger.Log.Fatal("无法连接以太坊客户端: ", zap.Error(err))
		return nil, err
	}
	return &Client{client: c}, nil
}

// GetLatestBlockHeader retrieves the latest block header.
func (c *Client) GetLatestBlockHeader(ctx context.Context) (*types.Header, error) {
	return c.client.HeaderByNumber(ctx, nil)
}

// GetBlockByNumber retrieves a block by its number.
func (c *Client) GetBlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	return c.client.BlockByNumber(ctx, number)
}

// Close closes the underlying client connection.
func (c *Client) Close() {
	c.client.Close()
}