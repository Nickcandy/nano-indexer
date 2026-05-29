# nano-indexer

> 立项时间：2026-05-28  
> 项目性质：Web3 / Go / 链上数据方向面试主项目  
> 当前阶段：minimal ERC20 indexer MVP

## 项目定位

`nano-indexer` 是一个 EVM 链上事件索引器，用来从 RPC 拉取、解析并存储 ERC20 `Transfer` 事件。

第一阶段目标不是做大而全的链上数据平台，而是做一个本地可运行、能清楚讲解工程取舍的链上数据底座：

- 按区块范围扫描指定 ERC20 token 的 `Transfer` logs。
- 将链上 log 解析成结构化转账数据。
- 用 `chain_id + tx_hash + log_index` 保证幂等写入。
- 用 `sync_state` 支持断点续扫。
- 用 confirmation window 降低 reorg 风险。
- 提供最小查询 API，方便展示地址或 token 的转账记录。

这个项目后续会作为 `smart-money-radar` 的数据来源，用于地址画像、鲸鱼异动、早期资金流和聪明钱分析。

## 文档

- [项目立项书](docs/project-charter.md)
- [技术方案](docs/technical-design.md)

## MVP 范围

第一版只做能闭环的索引器：

1. 配置 RPC、chain、token、起始区块、batch size 和 confirmations。
2. 从 MongoDB 读取扫描进度。
3. 获取链上 latest block，计算 safe block。
4. 按 batch 拉取 ERC20 `Transfer` logs。
5. 解析并幂等写入 `token_transfers`。
6. 更新 `blocks` 和 `sync_states`。
7. 提供 `GET /healthz` 和转账查询 API。

## 非目标

第一阶段不做：

- 全链所有 ERC20 自动发现。
- 多链分布式扫描。
- DEX swap、成本价、PnL 计算。
- 复杂前端 dashboard。
- 生产级告警、任务队列和权限系统。

这些内容只作为后续扩展或面试追问方案保留。

## 默认技术决策

| 主题 | 决策 |
|---|---|
| 语言 | Go 1.24+ |
| API | Echo |
| 存储 | MongoDB |
| 日志 | `log/slog` |
| 架构 | 简化版 Clean Architecture / DDD |
| MVP 扫描对象 | 配置列表中的 ERC20 token |
| Reorg 策略 | MVP 使用 confirmation window，并落库 block hash；后续补自动校验和回滚 |

## 本地运行

前置条件：

- Go 1.24+
- MongoDB，本地默认地址为 `mongodb://localhost:27017`

配置环境变量：

```powershell
$env:SERVER_PORT="8080"
$env:MONGO_URI="mongodb://localhost:27017"
$env:MONGO_DATABASE="nano_indexer"
$env:RPC_URL="https://your-rpc.example"
$env:CHAIN_ID="8453"
$env:CONFIRMATIONS="12"
$env:DEFAULT_START_BLOCK="12345678"
$env:BATCH_SIZE="100"
$env:POLL_INTERVAL="10s"
$env:SCANNER_ENABLED="true"
$env:TOKEN_ADDRESSES="0xTokenAddress"
```

启动服务：

```powershell
go run ./cmd/indexer
```

健康检查：

```powershell
Invoke-RestMethod http://localhost:8080/healthz
```

预期返回：

```json
{"status":"ok"}
```

查询转账：

```powershell
Invoke-RestMethod "http://localhost:8080/transfers?chain_id=8453&address=0x1111111111111111111111111111111111111111&limit=50"
Invoke-RestMethod "http://localhost:8080/transfers?chain_id=8453&token=0xTokenAddress&limit=50"
```

查询地址摘要：

```powershell
Invoke-RestMethod "http://localhost:8080/addresses/0x1111111111111111111111111111111111111111/summary?chain_id=8453"
```

本地 demo 验证点：

1. `sync_states` 中对应 token 的 `latest_scanned_block` 持续推进。
2. `token_transfers` 中写入 `chain_id + tx_hash + log_index` 去重后的转账记录。
3. `blocks` 中写入已扫描区间的 block hash 和 parent hash。
4. `/transfers` 能查出刚写入的数据。

运行测试：

```powershell
go test ./...
```

Mongo 幂等写入集成测试默认跳过；如需运行，先设置测试库地址：

```powershell
$env:NANO_INDEXER_MONGO_TEST_URI="mongodb://localhost:27017"
go test -v ./internal/storage -run TestTransferRepoUpsertManyIsIdempotent
```

## 当前已完成

- Go module。
- 环境变量配置加载。
- `log/slog` 日志入口。
- MongoDB 连接与 ping。
- Echo HTTP server。
- `GET /healthz`。
- `GET /transfers`。
- `GET /addresses/:address/summary`。
- ERC20 `Transfer` scanner。
- `blocks`、`token_transfers`、`sync_states` 集合与索引。
- config、healthz、parser、API、scanner 和 storage 关键单元测试。

## 当前限制

- reorg 当前只通过 confirmation window 降低风险。
- `blocks` 已保存 hash 和 parent hash，但还没有自动 hash 校验、orphan 标记或区间回滚。
- scanner 是单进程、按配置 token 扫描，不做全链 token 发现。

## 下一步

1. 增加最近 N 个已确认 block hash 校验。
2. 检测到 reorg 后标记 orphan block 和 removed transfers。
3. 从共同祖先附近重新扫描。
4. 为 `smart-money-radar` 增加地址/token 统计 API。
