# nano-indexer 项目立项书

> 日期：2026-05-28  
> 阶段：立项  
> 项目周期：12-16 周面试准备主项目

## 1. 背景

当前准备方向是 Web3 / Go / 链上数据 / Crypto trading infra。与其从零新建一个临时 demo，本项目选择把 `nano-indexer` 定位成长期可扩展的链上数据底座。

项目先解决一个清晰问题：把 EVM 链上的 ERC20 `Transfer` 事件稳定索引到本地数据库，并提供可查询的数据结构。这个能力既能展示 Go 后端工程能力，也能支撑后续 `smart-money-radar` 的资金流分析。

## 2. 项目目标

MVP 要交付一个本地可运行、能在面试中讲清楚的链上事件索引器：

- 从 RPC 按 block range 拉取 ERC20 `Transfer` logs。
- 解析 log 到 `token_transfers`。
- 使用 `chain_id + tx_hash + log_index` 保证幂等。
- 使用 `sync_states` 支持断点续扫。
- 使用 confirmation window 控制 reorg 风险。
- 提供最小 HTTP 查询 API。
- 文档说明运行方式、核心设计、索引策略和后续扩展。

## 3. 成功标准

12 周内至少达到：

- README 能在 1 分钟内说明项目价值、MVP 范围和下一步。
- 本地能跑一次 ERC20 `Transfer` 扫描 demo。
- MongoDB 中有结构化 `token_transfers` 数据。
- 能按 address 或 token 查询转账记录。
- 能讲清 confirmation、reorg、幂等、断点续扫和 Mongo 索引设计。
- 能说明如何扩展到 `smart-money-radar`。

## 4. 面试价值

| 能力点 | 项目体现 |
|---|---|
| Web3 业务 | block、transaction、receipt、log、ERC20 Transfer、confirmation、reorg |
| Go 后端 | context、slog、配置、HTTP API、优雅退出、单元测试 |
| 数据库 | Mongo schema、唯一索引、复合索引、幂等写入、分页查询 |
| 系统设计 | batch scanner、断点续扫、RPC 限流、失败重试、reorg 处理 |
| 数据产品 | 地址画像、鲸鱼异动、早期资金流、smart money 分析 |

## 5. 范围

### 5.1 MVP 范围

- 单进程 scanner。
- 单链或配置化链 ID。
- 配置列表中的 ERC20 token。
- MongoDB 持久化。
- 最小 Echo HTTP API。
- 单元测试覆盖 parser、幂等写入和进度推进。

### 5.2 非目标

- 不做全链 token 自动发现。
- 不做多链分布式调度。
- 不做 DEX swap / PnL。
- 不做复杂前端。
- 不做生产级监控、告警、权限和任务队列。

## 6. 阶段计划

### Phase 1：项目骨架

- Go module。
- 配置加载。
- `slog` 日志。
- Mongo 连接。
- Echo server。
- README 和技术文档。

### Phase 2：ERC20 Transfer MVP

- `token_transfers` 数据模型。
- ERC20 `Transfer` parser。
- `eth_getLogs` / FilterLogs 扫描。
- 幂等写入。
- block range batch scan。

### Phase 3：可靠性补强

- `blocks` 集合。
- `sync_states` 集合。
- confirmation window。
- retry / backoff。
- reorg 检测设计与最小实现。

### Phase 4：查询与展示

- `GET /healthz`。
- `GET /transfers`。
- `GET /addresses/:address/summary`。
- README demo 流程。

### Phase 5：服务 smart-money-radar

- 地址统计。
- token 统计。
- 大额转账检测。
- 早期买入地址检测。
- Markdown report 或最小 API。

## 7. 风险与控制

| 风险 | 控制方式 |
|---|---|
| RPC 限流或超时 | batch size 可配置，所有请求传递 context，失败显式返回 error |
| reorg 导致数据不一致 | MVP 先扫 safe block，后续保存 block hash 并回滚受影响区间 |
| 数据重复写入 | `chain_id + tx_hash + log_index` 唯一索引 |
| 查询变慢 | 围绕 address、token、block_number 建复合索引 |
| 范围膨胀 | 第一阶段只做 ERC20 Transfer，不做 DEX 和 PnL |

## 8. 当前结论

项目应先做小而完整的 ERC20 Transfer indexer。不要一开始做 smart money、PnL 或 dashboard。只要 scanner、Mongo 数据、查询 API 和文档闭环，就已经足够支撑 Web3 后端面试的主项目叙事。
