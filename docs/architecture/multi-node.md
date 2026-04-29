# 多机部署架构

## 主从架构图

```mermaid
graph TD
    A[用户请求] --> B[Nginx 负载均衡]
    B --> C[Master 节点]
    B --> D[Slave 节点 1]
    B --> E[Slave 节点 2]
    C --> F[(MySQL 数据库)]
    D --> F
    E --> F
    C --> G[Redis 缓存]
    D --> G
    E --> G
    C --> H[Web 前端服务]
    H --> I[用户浏览器]
    D -.-> |FRONTEND_BASE_URL 重定向| C
```

## Master 节点职责

Master 节点是完整功能节点，承担以下职责：

- **数据库迁移**：`model.InitDB()` 中，只有 `config.IsMasterNode == true` 的节点会执行 `model.migrateDB()`，自动建表和维护表结构。Slave 节点连接数据库后直接返回，不执行迁移。
- **配置管理**：`model.InitOptionMap()` 从本地配置变量初始化系统配置项。当 Redis 启用时，`model.SyncOptions()` 会定期从数据库拉取配置并写入缓存。
- **渠道缓存初始化**：`model.InitChannelCache()` 从数据库加载渠道列表，构建分组-模型-渠道的映射关系，并按优先级排序。`go model.SyncChannelCache()` 以 `SYNC_FREQUENCY` 为周期定期同步。
- **提供 Web 前端服务**：Master 节点内嵌 React 前端构建产物，直接提供管理面板和前端页面。Slave 节点可通过 `FRONTEND_BASE_URL` 将前端请求重定向到 Master。
- **处理 API 中继请求**：Master 节点同样承担 API 请求的中继转发，与 Slave 节点无异。
- **定时任务**：
  - 渠道可用性测试（`AutomaticallyTestChannels`，由 `CHANNEL_TEST_FREQUENCY` 环境变量控制）
  - 批量更新配额和用量（`BatchUpdateEnabled`，由 `BATCH_UPDATE_ENABLED` 环境变量控制）
  - 配置同步（`SyncOptions`、`SyncChannelCache`，由 `MemoryCacheEnabled` 控制）

## Slave 节点职责

Slave 节点是轻量功能节点，承担以下职责：

- **仅处理 API 中继请求**：Slave 节点仅转发 API 请求到上游渠道，不执行管理功能。
- **不执行数据库迁移**：`model.InitDB()` 在 `IsMasterNode == false` 时直接返回，跳过 `migrateDB()`。
- **不执行配置初始化**：Slave 节点虽然会调用 `model.InitOptionMap()` 初始化本地配置变量，但不执行管理面板所需的全部初始化流程。
- **通过 Redis 缓存读取配置**：当 Redis 启用且 `SYNC_FREQUENCY` 设置后，Slave 节点定期从 Redis 同步配置和渠道信息。缓存命中时零数据库访问。
- **可选前端重定向**：通过设置 `FRONTEND_BASE_URL`，Slave 节点将所有前端页面请求以 HTTP 301（永久重定向）转发到 Master 节点。

## 部署前提条件

### 1. 同一 `SESSION_SECRET`

所有节点的 Session 加密密钥必须一致，确保用户在负载均衡下的 Cookie 跨节点有效。Session 被 Master 签发后，Slave 节点必须能正确解密验证。

```bash
# 所有节点设置相同的值
export SESSION_SECRET=your-consistent-secret-key
```

未设置时，`SESSION_SECRET` 默认为随机 UUID，各节点之间会不一致。

### 2. 共用 MySQL/PostgreSQL

多机部署必须使用远程数据库（MySQL 或 PostgreSQL），所有节点连接同一个数据库实例。**不能使用 SQLite**，因为 SQLite 是本地文件数据库，无法在多机之间共享。

```bash
export SQL_DSN=root:password@tcp(remote-mysql-host:3306)/oneapi
# 或使用 PostgreSQL
export SQL_DSN=postgres://user:password@remote-pg-host:5432/oneapi
```

### 3. Slave 设 `NODE_TYPE=slave`

明确指定每个节点的角色。`NODE_TYPE` 环境变量控制 `config.IsMasterNode` 的值：

```go
var IsMasterNode = os.Getenv("NODE_TYPE") != "slave"
```

- 不设置或设置为 `master`：该节点为主节点，执行数据库迁移和完整初始化
- 设置为 `slave`：该节点为从节点，跳过数据库迁移，仅处理 API 中继

```bash
# 从节点必须设置
export NODE_TYPE=slave
```

### 4. 配置 `SYNC_FREQUENCY` + Redis

所有节点都需要配置 Redis 连接和同步频率。Redis 启用需要同时设置 `REDIS_CONN_STRING` 和 `SYNC_FREQUENCY` 两个环境变量：

```go
// common/redis.go
if os.Getenv("REDIS_CONN_STRING") == "" {
    RedisEnabled = false  // Redis 连接串未设置
    return nil
}
if os.Getenv("SYNC_FREQUENCY") == "" {
    RedisEnabled = false  // 同步频率未设置
    return nil
}
```

`SYNC_FREQUENCY` 单位为秒，默认值为 600（10 分钟）。推荐设置为 60 秒以获得更及时的配置同步。

```bash
export REDIS_CONN_STRING=redis://:password@remote-redis-host:6379/0
export SYNC_FREQUENCY=60
```

启用 Redis 后，系统会自动启用内存缓存（`MemoryCacheEnabled = true`），并启动 `SyncOptions` 和 `SyncChannelCache` 两个后台协程。

### 5. Slave 可选 `FRONTEND_BASE_URL`

Slave 节点可通过此变量将前端页面请求重定向到 Master 节点，避免用户在 Slave 上访问管理面板：

```bash
# 在 Slave 节点上设置，将前端请求重定向到 Master
export FRONTEND_BASE_URL=https://master-node-url
```

设置后，Slave 节点会将所有未匹配的前端路由以 HTTP 301 重定向到 Master 节点。Master 节点上设置此变量会被忽略。

## Redis 的三种角色

### 配置缓存

渠道列表、令牌信息、系统 Options 等都缓存到 Redis，避免 Slave 节点频繁查询数据库。Slave 节点从 Redis 读取配置，在缓存有效期内实现零数据库访问。

### 限流计数器

全局 API 速率限制（`GLOBAL_API_RATE_LIMIT`）和全局 Web 速率限制（`GLOBAL_WEB_RATE_LIMIT`）的 IP 计数存储在 Redis 中，跨所有节点共享。这使得限流在整个集群范围内生效，而非单节点独立计数。

### 状态同步

当渠道启用/禁用状态通过管理面板变更后，Master 节点通过 Redis 通知所有 Slave 节点刷新本地缓存，确保配置变更快速生效。

## 同步流程

### Master 节点启动

1. 连接数据库
2. 执行数据库迁移（`migrateDB`）
3. 加载 Options 到内存（`InitOptionMap`）
4. 初始化 Redis 客户端
5. 从数据库加载渠道缓存（`InitChannelCache`）
6. 启动后台协程，每隔 `SYNC_FREQUENCY` 秒从数据库同步配置到 Redis：
   - `go model.SyncOptions(SyncFrequency)`：同步 Options
   - `go model.SyncChannelCache(SyncFrequency)`：同步渠道列表

### Slave 节点启动

1. 连接数据库（但不执行迁移）
2. 初始化 Redis 客户端
3. 从 Redis 加载缓存（若启用了 `MemoryCacheEnabled`）
4. 启动后台协程，每隔 `SYNC_FREQUENCY` 秒从 Redis 拉取更新

### 内存缓存层级

当 `MEMORY_CACHE_ENABLED=true`（开启 Redis 时自动启用）时：

1. 优先从内存缓存查找渠道和配置
2. 内存未命中时查询 Redis/DB
3. 渠道分组-模型映射在内存中按优先级排序，提高路由效率

### 黑名单同步

用户封禁状态通过 Redis 同步。当用户在 Master 上被封禁后，封禁状态写入 Redis，所有节点共享相同的黑名单。

## 典型部署拓扑

### 单机部署（默认）

```
+------------------+
|    One API       |
|  SQLite + 无 Redis |
|  单节点 Master    |
+------------------+
```

- 使用 SQLite 本地数据库
- 无需 Redis
- 适合个人使用、低并发场景
- 无高可用保障

### 主 + 单从

```
            +----------+
            |  Nginx   |
            +----------+
                  |
         +--------+--------+
         |                  |
  +------+------+   +------+------+
  | Master 节点  |   | Slave 节点  |
  | MySQL+Redis  |   | Redis 缓存  |
  +-------------+   +-------------+
         |                  |
         +--------+---------+
                  |
          +-------+-------+
          |    MySQL      |
          |    Redis      |
          +---------------+
```

- 使用 MySQL 数据库
- Redis 单实例
- 适合中小团队，提高可用性
- Master 故障时可通过 DNS 切换临时将 Slave 提升为 Master

### 主 + 多从 + Redis 集群

```
            +----------+
            |  Nginx   |
            | ip_hash  |
            +----------+
                  |
         +--------+--------+--------+
         |        |        |        |
  +------+------+  +------+------+  +------+------+
  | Master 节点  |  | Slave 节点 1|  | Slave 节点 2|
  |  MySQL+Redis |  | Redis 缓存  |  | Redis 缓存  |
  +-------------+  +-------------+  +-------------+
         |               |               |
         +--------+------+-------+------+
                  |               |
          +-------+-------+  +---+-----------+
          |    MySQL      |  | Redis         |
          |   主从/集群   |  | Sentinel/Cluster|
          +---------------+  +---------------+
```

- MySQL 主从复制或集群
- Redis Sentinel 或 Cluster 模式
- Nginx 前置负载均衡，推荐使用 `ip_hash` 策略保持会话亲和性，或 `least_conn` 策略均匀分发请求
- 从节点可根据负载水平弹性伸缩
- 适合生产级高并发场景

## 从节点限制与注意事项

- **不能修改配置**：在 Slave 节点上通过 Web UI 或 API 修改的配置（系统设置、渠道、令牌等）可能会被 Master 的下一次同步覆盖。所有配置修改应在 Master 节点上完成。
- **不能访问管理面板**：Slave 节点不提供完整的管理后台。建议设置 `FRONTEND_BASE_URL`，将对前端页面的请求自动重定向到 Master。
- **不运行定时任务**：渠道测试（`AutomaticallyTestChannels`）和批量更新（`BatchUpdateEnabled`）仅在配置了对应环境变量的节点上运行。建议仅在 Master 节点上设置这些环境变量。
- **Redis 不可用时的回退**：Slave 节点启动时，如果 Redis 不可用且本地缓存未命中，会直接回退到数据库查询。此时延迟会增加，但仍可正常服务 API 请求。
- **更新部署顺序**：更新版本时应先更新 Master 节点，确认迁移和初始化正常完成后，再逐个更新 Slave 节点。避免 Slave 节点的代码版本与数据库结构不兼容。
