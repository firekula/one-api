# 配置参考

## 配置方式概览

One API 支持三种配置方式，优先级从高到低如下：

1. **命令行参数** — 优先级最高
2. **环境变量** — 优先级次之
3. **`.env` 文件** — 优先级最低

项目根目录下提供了 `.env.example` 文件，复制并修改即可：

```shell
cp .env.example .env
```

程序启动时会自动加载 `.env` 文件中的配置。环境变量中存在同名变量时，会覆盖 `.env` 中的值。

---

## 环境变量完整参考

### 数据库

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `SQL_DSN` | string | 空 | MySQL/PostgreSQL 连接字符串。不设置则使用 SQLite。需提前创建好数据库，程序会自动建表。示例（MySQL）：`root:123456@tcp(localhost:3306)/oneapi`；示例（PostgreSQL）：`postgres://postgres:123456@localhost:5432/oneapi` |
| `SQLITE_PATH` | string | `one-api.db` | SQLite 数据库文件路径 |
| `SQL_MAX_IDLE_CONNS` | int | `100` | 数据库最大空闲连接数 |
| `SQL_MAX_OPEN_CONNS` | int | `1000` | 数据库最大打开连接数。如遇到 Error 1040（连接数过多），可适当降低此值 |
| `SQL_MAX_LIFETIME` | int | `60` | 数据库连接的最大生命周期，单位分钟 |
| `SQLITE_BUSY_TIMEOUT` | int | `3000` | SQLite 忙等待超时时间，单位毫秒 |
| `LOG_SQL_DSN` | string | 空 | 为 `logs` 表使用独立的数据库，仅支持 MySQL/PostgreSQL |

### 缓存与同步

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `REDIS_CONN_STRING` | string | 空 | Redis 连接字符串。启用后需同时设置 `SYNC_FREQUENCY`。示例：`redis://default:redispw@localhost:49153`。Sentinel/Cluster 模式下使用逗号分隔多个地址：`host1:port1,host2:port2`，并配合 `REDIS_PASSWORD` 和 `REDIS_MASTER_NAME` |
| `REDIS_PASSWORD` | string | 空 | Redis 密码（Sentinel/Cluster 模式使用） |
| `REDIS_MASTER_NAME` | string | 空 | Redis Sentinel/Cluster 的主节点名称 |
| `MEMORY_CACHE_ENABLED` | bool | `false` | 是否启用内存缓存。启用 Redis 后会自动启用 |
| `SYNC_FREQUENCY` | int | `600` | 从数据库同步配置的间隔，单位秒。启用 Redis 时必须设置 |

### 节点模式

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `NODE_TYPE` | string | `master` | 节点类型，可选 `master` 或 `slave`。Master 节点负责数据库迁移和配置管理 |
| `FRONTEND_BASE_URL` | string | 空 | 从节点（slave）设置此值后，会将前端页面请求重定向到指定地址。示例：`https://openai.justsong.cn`。在主节点上设置会被忽略 |
| `SESSION_SECRET` | string | 随机生成 | 固定会话密钥。多节点部署时各个节点必须设置相同值。示例：`random_string` |

### 速率限制

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `GLOBAL_API_RATE_LIMIT` | int | `480` | 单 IP 每 3 分钟内允许的最大 API 请求次数 |
| `GLOBAL_WEB_RATE_LIMIT` | int | `240` | 单 IP 每 3 分钟内允许的最大 Web 请求次数 |

### 渠道运维

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `CHANNEL_UPDATE_FREQUENCY` | int | 空 | 渠道余额更新间隔，单位分钟。不设置则不更新 |
| `CHANNEL_TEST_FREQUENCY` | int | 空 | 渠道可用性测试间隔，单位分钟。不设置则不测试 |
| `POLLING_INTERVAL` | int | 空 | 批量更新之间的请求间隔，单位秒。不设置则无间隔 |

### 批量处理

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `BATCH_UPDATE_ENABLED` | bool | `false` | 启用数据库批量更新聚合（减少数据库连接数），启用后用户额度更新存在一定延迟 |
| `BATCH_UPDATE_INTERVAL` | int | `5` | 批量更新聚合的时间间隔，单位秒 |

### 请求代理

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `RELAY_TIMEOUT` | int | 空 | 中继请求超时时间，单位秒。不设置则无超时 |
| `RELAY_PROXY` | string | 空 | API 请求使用的代理地址。示例：`http://localhost:7890` |
| `USER_CONTENT_REQUEST_TIMEOUT` | int | `30` | 用户上传内容（如图片）的下载超时时间，单位秒 |
| `USER_CONTENT_REQUEST_PROXY` | string | 空 | 用户上传内容请求使用的代理地址 |

### 编码器缓存

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `TIKTOKEN_CACHE_DIR` | string | 空 | tiktoken 编码器缓存目录。程序启动时会联网下载编码文件（如 `gpt-3.5-turbo`），在不稳定网络或离线环境下可配置此目录缓存数据，并迁移到离线环境使用 |
| `DATA_GYM_CACHE_DIR` | string | 空 | 作用与 `TIKTOKEN_CACHE_DIR` 一致，但优先级较低 |

### Gemini 专属

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `GEMINI_SAFETY_SETTING` | string | `BLOCK_NONE` | Gemini 安全设置 |
| `GEMINI_VERSION` | string | `v1` | One API 所使用的 Gemini API 版本 |

### 指标监控

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `ENABLE_METRIC` | bool | `false` | 启用渠道自动禁用功能。当请求失败率超过阈值时自动禁用渠道 |
| `METRIC_QUEUE_SIZE` | int | `10` | 成功率统计队列大小 |
| `METRIC_SUCCESS_RATE_THRESHOLD` | float | `0.8` | 成功率阈值。低于此值将自动禁用渠道 |

### 初始化

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `INITIAL_ROOT_TOKEN` | string | 空 | 首次启动时自动创建 root 用户的 Token |
| `INITIAL_ROOT_ACCESS_TOKEN` | string | 空 | 首次启动时自动创建 root 用户的 Access Token |

### UI 与行为

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `THEME` | string | `default` | UI 主题。可选值请参见 `web/README.md` |
| `ENFORCE_INCLUDE_USAGE` | bool | `false` | 是否强制在 stream 模式下返回 usage 信息 |
| `TEST_PROMPT` | string | `Output only your specific model name with no additional text.` | 测试模型时使用的用户 prompt |

---

## 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--port <port_number>` | `3000` | 监听端口 |
| `--log-dir <log_dir>` | `./logs` | 日志目录 |
| `--version` | - | 打印版本信息并退出 |
| `--help` | - | 打印帮助信息并退出 |

端口优先级：`--port` 命令行参数 > `PORT` 环境变量 > 默认值 `3000`。

---

## 生产环境推荐配置

### 必须设置

```shell
# 使用 MySQL 而非 SQLite，所有节点连接同一数据库
SQL_DSN=root:123456@tcp(localhost:3306)/oneapi

# 固定会话密钥，多节点部署时各节点必须一致
SESSION_SECRET=<your_random_string>

# 时区
TZ=Asia/Shanghai
```

### 建议设置

```shell
# Redis 连接字符串，启用缓存和跨节点同步
REDIS_CONN_STRING=redis://default:redispw@localhost:49153

# 配置同步频率，建议设为 60 秒
SYNC_FREQUENCY=60

# 渠道余额更新频率，建议每天一次（1440 分钟）
CHANNEL_UPDATE_FREQUENCY=1440
```

### 安全加固

```shell
# API 速率限制：每 3 分钟 180 次
GLOBAL_API_RATE_LIMIT=180

# Web 速率限制：每 3 分钟 60 次
GLOBAL_WEB_RATE_LIMIT=60
```

### Docker 部署示例

```shell
docker run --name one-api -d --restart always \
  -p 3000:3000 \
  -e SQL_DSN="root:123456@tcp(localhost:3306)/oneapi" \
  -e SESSION_SECRET="your_random_string" \
  -e TZ=Asia/Shanghai \
  -e REDIS_CONN_STRING="redis://default:redispw@localhost:49153" \
  -e SYNC_FREQUENCY=60 \
  -e CHANNEL_UPDATE_FREQUENCY=1440 \
  -e GLOBAL_API_RATE_LIMIT=180 \
  -e GLOBAL_WEB_RATE_LIMIT=60 \
  -v /home/ubuntu/data/one-api:/data \
  justsong/one-api
```

---

## 注意事项

- **数据库**：使用 `SQL_DSN` 时，请确保数据库已提前创建，程序会自动创建表结构。
- **连接数**：如果遇到 MySQL Error 1040（连接数过多），请酌情降低 `SQL_MAX_OPEN_CONNS` 的值。
- **会话密钥**：`SESSION_SECRET` 不要使用示例值 `random_string`，请替换为随机字符串。
- **节点类型**：只有 master 节点会执行数据库迁移，slave 节点需连接到同一数据库，并设置 `NODE_TYPE=slave`。
- **Redis 要求**：启用 Redis 时，`SYNC_FREQUENCY` 必须设置，否则 Redis 不会生效。
