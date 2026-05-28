# nano-indexer

> 立项时间：2026-05-28  
> 项目性质：Web3 / Go / 链上数据方向面试主项目  
> 当前阶段：立项与技术方案整理

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
| Reorg 策略 | MVP 使用 confirmation window，后续补 block hash 校验 |

## 本地运行

前置条件：

- Go 1.24+
- MongoDB，本地默认地址为 `mongodb://localhost:27017`

配置环境变量：

```powershell
$env:SERVER_PORT="8080"
$env:MONGO_URI="mongodb://localhost:27017"
$env:MONGO_DATABASE="nano_indexer"
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

运行测试：

```powershell
go test ./...
```

## 当前已完成

- Go module。
- 环境变量配置加载。
- `log/slog` 日志入口。
- MongoDB 连接与 ping。
- Echo HTTP server。
- `GET /healthz`。
- config 和 healthz 单元测试。

## 下一步

1. 定义 `blocks`、`token_transfers`、`sync_states` 三个核心集合与索引。
2. 实现 ERC20 `Transfer` log parser 的单元测试。
3. 实现最小 scanner：读取进度、拉 logs、解析、写入、推进进度。
4. 暴露转账查询 API。
