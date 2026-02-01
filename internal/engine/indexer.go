package engine

import (
	"context"
	"time"
	"nano-indexer/internal/eth"
	"nano-indexer/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"nano-indexer/internal/config"
	"nano-indexer/internal/model"
	"math/big"
)

type Indexer struct {
	client   *eth.Client
	database *gorm.DB
	interval time.Duration
}

func NewIndexer(client *eth.Client, database *gorm.DB, interval time.Duration) *Indexer {
	return &Indexer{
		client:   client,
		database: database,
		interval: interval,
	}
}

func (i *Indexer) Start(ctx context.Context) {
	cfg := config.LoadConfig()
	var startBlock uint64
		// 从数据库中读取上次处理成功的区块高度
	var syncState model.SyncState
	result := i.database.First(&syncState)
	if result.Error != nil {
		// 如果没有记录，则使用默认起始区块号
		startBlock = cfg.Server.Eth.DefaultStartBlock
	} else {
		startBlock = syncState.LastBlockNum + 1 // 从上次处理成功的区块号+1开始
	}
	for {
		// 获取最新区块高度
		header, err := i.client.GetLatestBlockHeader(ctx)		
		if err != nil {
			logger.Log.Error("获取最新区块高度失败", zap.Error(err))
			continue
		}
		latestBlockNum := header.Number.Uint64()
		// 处理新区块
		for blockNum := startBlock; blockNum <= latestBlockNum; blockNum++ {
			block, err := i.client.GetBlockByNumber(ctx, big.NewInt(int64(blockNum)))
			if err != nil {
				logger.Log.Error("获取区块失败", zap.Uint64("blockNum", blockNum), zap.Error(err))
				break
			}
			// 处理区块中的交易
			for _, tx := range block.Transactions() {
				txModel := model.Transaction{
					Hash:      tx.Hash().Hex(),
					From:      "", // 需要额外处理获取发送方地址
					To:        "", // 需要额外处理获取接收方地址
					Value:     tx.Value().String(),
					BlockNumber:  blockNum,
					Timestamp: time.Unix(int64(block.Time()), 0),
				}
				i.database.Create(&txModel)
			}
			// 更新同步状态
			syncState.LastBlockNum = blockNum
			i.database.Save(&syncState)
			logger.Log.Info("已处理区块", zap.Uint64("blockNum", blockNum))
		}

		// 等待间隔
		select {
		case <-time.After(i.interval):
		case <-ctx.Done():
			return
		}
	}
}