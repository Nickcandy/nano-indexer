# nano-indexer 技术方案

> 日期：2026-05-28  
> 目标：定义 ERC20 Transfer MVP 的最小技术边界

## 1. 总体架构

```text
RPC Provider
   ↓
Eth Client
   ↓
Scanner / Cursor
   ↓
ERC20 Transfer Parser
   ↓
Mongo Repositories
   ↓
Echo Query API
   ↓
Smart Money Radar
```

第一阶段只实现到 Echo Query API。`Smart Money Radar` 只作为后续消费方。

## 2. 目录规划

```text
cmd/indexer/
  main.go

internal/config/
  config.go

internal/eth/
  client.go
  logs.go

internal/scanner/
  scanner.go
  cursor.go

internal/parser/
  erc20.go

internal/storage/
  mongo.go
  block_repo.go
  transfer_repo.go
  sync_state_repo.go

internal/model/
  block.go
  token_transfer.go
  sync_state.go

internal/api/
  server.go
  handlers.go

pkg/logger/
  logger.go
```

这是目标结构，不要求一次性全部建完。每一阶段只新增当阶段需要的文件。

## 3. 核心集合

### 3.1 `blocks`

用途：记录已扫描区块，用于 reorg 检测和数据追溯。

字段：

| 字段 | 说明 |
|---|---|
| `chain_id` | 链 ID |
| `block_number` | 区块高度 |
| `block_hash` | 当前区块 hash |
| `parent_hash` | 父区块 hash |
| `block_time` | 区块时间 |
| `status` | `canonical` / `orphaned` |
| `created_at` | 创建时间 |
| `updated_at` | 更新时间 |

索引：

```text
unique: { chain_id: 1, block_number: 1 }
index:  { chain_id: 1, block_hash: 1 }
```

### 3.2 `token_transfers`

用途：保存 ERC20 `Transfer` 事件。

字段：

| 字段 | 说明 |
|---|---|
| `chain_id` | 链 ID |
| `block_number` | 区块高度 |
| `block_hash` | 区块 hash |
| `tx_hash` | 交易 hash |
| `log_index` | log 在交易 receipt 中的位置 |
| `tx_index` | 交易在 block 中的位置 |
| `token_address` | token 合约地址 |
| `from_address` | 转出地址 |
| `to_address` | 转入地址 |
| `amount_raw` | 原始整数金额，字符串存储 |
| `event_time` | 事件时间 |
| `confirmed` | 是否已确认 |
| `removed` | 是否因 reorg 标记移除 |
| `created_at` | 创建时间 |
| `updated_at` | 更新时间 |

核心唯一索引：

```text
unique: { chain_id: 1, tx_hash: 1, log_index: 1 }
```

查询索引：

```text
index: { chain_id: 1, block_number: 1 }
index: { chain_id: 1, token_address: 1, block_number: -1 }
index: { chain_id: 1, from_address: 1, block_number: -1 }
index: { chain_id: 1, to_address: 1, block_number: -1 }
```

### 3.3 `sync_states`

用途：记录扫描进度，支持断点续扫。

字段：

| 字段 | 说明 |
|---|---|
| `chain_id` | 链 ID |
| `scanner_name` | 扫描器名称 |
| `token_address` | token 合约地址，可为空 |
| `from_block` | 初始扫描区块 |
| `latest_scanned_block` | 已扫描到的最高区块 |
| `latest_confirmed_block` | 当前 safe block |
| `confirmations` | 确认数 |
| `status` | `running` / `paused` / `failed` |
| `last_error` | 最近一次错误 |
| `updated_at` | 更新时间 |

索引：

```text
unique: { chain_id: 1, scanner_name: 1, token_address: 1 }
```

## 4. 扫描流程

第一版只扫描 safe block：

```text
safe_block = latest_block - confirmations
```

流程：

```text
1. 读取配置。
2. 连接 RPC。
3. 连接 MongoDB。
4. 读取 sync_states。
5. 获取链上 latest block。
6. 计算 safe_block。
7. 从 latest_scanned_block + 1 扫到 safe_block。
8. 按 batch 拉取 ERC20 Transfer logs。
9. 解析 logs。
10. 在 MongoDB 中幂等 upsert token_transfers。
11. 更新 blocks 和 sync_states。
12. 等待 poll_interval 后继续。
```

伪代码：

```go
for {
    latest, err := ethClient.BlockNumber(ctx)
    if err != nil {
        return err
    }

    safeBlock := latest - confirmations
    state, err := syncRepo.Get(ctx, chainID, scannerName, tokenAddress)
    if err != nil {
        return err
    }

    from := state.LatestScannedBlock + 1
    to := min(from+batchSize-1, safeBlock)

    logs, err := ethClient.FilterTransferLogs(ctx, from, to, tokenAddress)
    if err != nil {
        return err
    }

    transfers, err := parser.ParseERC20Transfers(logs)
    if err != nil {
        return err
    }

    if err := transferRepo.UpsertMany(ctx, transfers); err != nil {
        return err
    }

    if err := syncRepo.UpdateProgress(ctx, to, safeBlock); err != nil {
        return err
    }

    time.Sleep(pollInterval)
}
```

真实实现必须处理 `ctx.Done()`，不能让 scanner 在退出时继续 sleep 或继续发 RPC。

## 5. Reorg 策略

MVP：

- 只扫描 `latest_block - confirmations` 之前的数据。
- 已写入数据默认 `confirmed = true`。
- 文档说明生产级 reorg 处理方式。

后续：

1. 保存 `block_number`、`block_hash`、`parent_hash`。
2. 每轮扫描前检查最近 N 个已确认区块 hash。
3. 如果 Mongo 中 hash 与链上 hash 不一致：
   - 将旧 block 标记为 `orphaned`。
   - 将旧 transfers 标记为 `removed = true`。
   - 从共同祖先附近重新扫描。

## 6. API

MVP API：

```text
GET /healthz
GET /transfers?address=0x...&token=0x...&limit=50
GET /addresses/:address/summary
GET /addresses/:address/detection
```

API 要明确处理：

- 参数缺失。
- 地址格式错误。
- 数据为空。
- Mongo 查询失败。

`/addresses/:address/detection` 是 `smart-money-radar` 的 MVP 入口，只基于当前已索引的 ERC20 Transfer 聚合做透明启发式评分：

- 高频转账。
- 多 token 覆盖。
- 同时存在转入和转出。

第一版不计算 PnL，不拉外部价格，不实时查询链上数据。

错误不能吞掉，handler 返回明确 HTTP 状态码和错误信息。

## 7. 配置

建议配置：

```yaml
server:
  port: 8080

eth:
  chain_id: 8453
  rpc_url: ""
  poll_interval: 10s
  default_start_block: 0
  confirmations: 12
  batch_size: 1000

mongo:
  uri: mongodb://localhost:27017
  database: nano_indexer

scanner:
  enabled: true
  token_addresses: []
```

环境变量覆盖：

```text
RPC_URL
CHAIN_ID
CONFIRMATIONS
DEFAULT_START_BLOCK
MONGO_URI
MONGO_DATABASE
```

## 8. 测试策略

优先写单元测试：

- ERC20 `Transfer` parser：正常 log、topic 不匹配、data 长度错误。
- transfer repo：重复写入不会产生重复数据。
- sync state repo：进度只能向前推进。
- scanner：RPC 出错时返回 error，不更新进度。

运行方式按仓库约定：

```bash
go test -v ./service -run TestXXX
```

当前还没有 service 包时，先按实际包路径运行对应测试；后续如果建立 `service` 层，再统一到约定命令。
