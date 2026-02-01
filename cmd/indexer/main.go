package main

import (
	"context"
	"nano-indexer/internal/config"
	"nano-indexer/internal/db"
	"nano-indexer/internal/engine"
	"nano-indexer/internal/eth"
	"nano-indexer/pkg/logger"
	"os"
	"os/signal"
	"syscall"
	"go.uber.org/zap"
)

func main() {
	// 1. 初始化日志
	logger.InitLogger()
	defer logger.Log.Sync()

	// 2. 加载配置
	cfg := config.LoadConfig()

	// 3. 初始化以太坊客户端
	ethClient, err := eth.NewClient(cfg.Server.Eth.RpcUrl)
	if err != nil {
		logger.Log.Fatal("初始化以太坊客户端失败", zap.Error(err))
	}
	defer ethClient.Close()

	// 创建db
	database, err := db.InitDB()
	if err != nil {
		logger.Log.Fatal("初始化数据库失败", zap.Error(err))
	}
	
	// 4. 创建一个可取消的 Context，用于控制所有协程的退出
	// signal.NotifyContext 会在收到特定信号时自动调用 cancel()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 5. 初始化并启动索引器引擎
	indexer := engine.NewIndexer(ethClient, database, cfg.Server.Eth.PollInterval)
	
	// 这里不需要用 go indexer.Start(ctx)，直接阻塞运行即可
	// 因为我们依赖 ctx 的取消来结束 Start 里的 for 循环
	logger.Log.Info("🚀 Nano-Indexer 已启动，按 Ctrl+C 退出")
	
	indexer.Start(ctx)

	// 6. 走到这里说明 indexer.Start 因为 ctx 结束而退出了
	logger.Log.Info("👋 程序已安全关闭")
}