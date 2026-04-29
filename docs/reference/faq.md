# 常见问题（FAQ）

---

## 1. 部署问题

### Q: 部署后访问出现空白页面？

A: 检查前端是否正确构建。

- **Docker 部署**：等待 1-2 分钟让前端服务启动完成。
- **手动部署**：确认 `web/default/build` 目录存在且包含完整的前端文件。如果目录为空或缺失，执行以下命令重新构建：

  ```shell
  cd web
  npm install
  npm run build
  ```

- **Nginx 反向代理**：检查 `proxy_pass` 配置是否正确。确认下列配置项指向 One API 服务：

  ```nginx
  location / {
      proxy_pass http://127.0.0.1:3000;
      proxy_set_header Host $host;
      proxy_set_header X-Real-IP $remote_addr;
      proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
      proxy_set_header X-Forwarded-Proto $scheme;
  }
  ```

- 如果服务端返回 404 或空白页，检查 Web UI 路由 `/` 是否被后端正确拦截。参考 GitHub issue [#97](https://github.com/songquanpeng/one-api/issues/97)。

### Q: Docker 容器无法启动？

A: 按以下顺序排查：

1. **端口冲突**：默认使用 3000 端口，执行 `docker ps` 检查该端口是否已被占用。如有冲突，更换主机映射端口：
   ```shell
   docker run -p 3001:3000 justsong/one-api
   ```

2. **权限问题**：部分环境需要添加特权参数：
   ```shell
   docker run --privileged=true justsong/one-api
   ```

3. **数据库连接（MySQL 模式）**：确认目标数据库已创建：
   ```shell
   mysql -u root -p -e "CREATE DATABASE oneapi CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
   ```

4. **查看容器日志**：获取详细的错误信息：
   ```shell
   docker logs one-api
   ```

5. **网络模式**：使用 MySQL 且 MySQL 在宿主机时，添加 `--network="host"` 或正确配置容器网络。

### Q: SQLite 数据如何持久化？

A: Docker 部署时必须挂载 volume，否则容器重启后数据丢失：

```shell
docker run -v /home/ubuntu/data/one-api:/data justsong/one-api
```

数据和 `one-api.db` 数据库文件保存在宿主机的 `/home/ubuntu/data/one-api/` 目录下。容器重启或重建后，数据不会丢失。

可以通过 `SQLITE_PATH` 环境变量自定义 SQLite 数据库文件路径。

### Q: 如何使用 MySQL 替代 SQLite？

A: 设置 `SQL_DSN` 环境变量：

```shell
SQL_DSN="root:password@tcp(localhost:3306)/oneapi"
```

步骤：

1. 使用 MySQL client 创建数据库（需提前创建，程序不会自动创建数据库，但会自动建表）：
   ```sql
   CREATE DATABASE oneapi CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
   ```

2. Docker 部署时添加 `--network="host"` 以访问宿主机 MySQL：
   ```shell
   docker run --network="host" -e SQL_DSN="root:password@tcp(localhost:3306)/oneapi" justsong/one-api
   ```

3. 也支持 PostgreSQL，DSN 格式：
   ```shell
   SQL_DSN="postgres://postgres:password@localhost:5432/oneapi"
   ```

### Q: 如何配置多节点部署？

A: One API 支持 master-slave 架构：

- **Master 节点**：负责数据库迁移和配置管理。设置 `NODE_TYPE=master`（默认值）。
- **Slave 节点**：只处理请求，不做数据库迁移。需设置 `NODE_TYPE=slave` 和 `FRONTEND_BASE_URL`（可选，设置后将前端请求重定向到 master 地址）。

所有节点必须：
- 连接同一数据库（使用相同的 `SQL_DSN`）
- 使用相同的 `SESSION_SECRET`
- （推荐）启用 Redis 同步配置：`REDIS_CONN_STRING` + `SYNC_FREQUENCY`

---

## 2. 渠道问题

### Q: 渠道测试报错 `invalid character '<' looking for beginning of value`？

A: 返回值不是合法 JSON，而是 HTML 页面。通常原因是：

- 部署服务器的 IP 地址或代理节点被上游服务商（如 CloudFlare）封禁，返回了验证页面。
- 检查代理设置是否正常（`RELAY_PROXY` 环境变量）。
- 尝试更换代理或更换服务器 IP。
- 在浏览器中直接访问上游 API 地址，确认是否正常返回 JSON 而不是 HTML 页面。

### Q: 提示"当前分组负载已饱和，请稍后再试"？

A: 上游渠道返回了 HTTP 429 Too Many Requests，意味着请求被限流了。

解决方案：

- 等待限流自动解除。
- 添加多个同类型渠道做负载均衡，避免单个渠道过载。
- 如果频繁触发，检查渠道的 API Key 是否有调用频率限制，或联系上游服务商提高限额。

### Q: 提示"当前分组下对于模型 xxx 无可用渠道"？

A: 逐一检查以下几点：

1. **用户分组和渠道分组是否匹配**：用户所属的分组（Group）必须与渠道的分组设置一致。
2. **渠道是否已启用**：渠道列表页面检查状态是否为绿色（已启用）。
3. **渠道的模型列表是否包含请求的模型名**：编辑渠道，确认"模型"字段包含了正在请求的模型名称（如 `gpt-4`）。
4. **渠道是否被自动禁用**：如果启用了 `ENABLE_METRIC=true`，成功率低的渠道会被自动禁用（status=3）。检查渠道状态是否变为 3（已禁用）。
5. **渠道的权重设置**：权重为 0 的渠道不会被分配请求。

### Q: 渠道被自动禁用了？

A: 启用了 `ENABLE_METRIC=true` 后，当渠道的成功率低于 `METRIC_SUCCESS_RATE_THRESHOLD`（默认 0.8）时，系统会自动将其禁用（status=3）。

处理方法：

1. 在 Web UI 渠道列表中找到该渠道，点击"启用"。
2. 检查渠道的 API Key 是否有效、余额是否充足。
3. 检查渠道的代理/网络连接是否正常。
4. 如果频繁自动禁用，可以适当调低 `METRIC_SUCCESS_RATE_THRESHOLD`（如设为 0.6），或调整 `METRIC_QUEUE_SIZE`（默认 10）改变统计窗口大小。
5. 如果不想使用自动禁用功能，可以设置 `ENABLE_METRIC=false`。

### Q: 如何测试渠道是否可用？

A: 三种方式：

1. **Web UI**：渠道列表页面，点击对应渠道的"测试"按钮。测试结果会显示响应时间和是否成功。
2. **API 测试**：
   ```
   GET /api/channel/test/:id
   ```
3. **自动定时测试**：设置环境变量 `CHANNEL_TEST_FREQUENCY`（单位：分钟），系统会按设定间隔自动测试所有渠道。

### Q: 如何配置多个渠道的负载均衡？

A: 创建多个同类型渠道，分别填入不同的 API Key。系统会自动按权重分配请求：

- 每个渠道有权重设置（Weight），权重越高被分配到的请求越多。
- 多个渠道共同分担请求压力，避免单点过载。
- 如果某个渠道被自动禁用，系统会自动将请求切换到其他可用渠道。

---

## 3. 额度问题

### Q: 额度是怎么计算的？

A: 额度计算公式：

```
消费额度 = 分组倍率 x 模型倍率 x (prompt_tokens + completion_tokens x completion_ratio)
```

- **分组倍率**：在系统设置中配置，用于区分不同用户组的消费比例。
- **模型倍率**：在系统设置中配置，用于区分不同模型的消费比例。
- **completion_ratio（补全倍率）**：区分 prompt 和 completion 的计价差异。GPT-3.5 固定为 1.33，GPT-4 固定为 2，与 OpenAI 官方定价对齐。

> 注意：One API 的默认倍率已经与 OpenAI 官方定价一致，无需手动调整。CompletionRatio 可以在系统设置页面查看和修改。

非流模式（non-stream）下，官方接口会返回消耗的总 token 数，但 prompt 和 completion 的消耗倍率不一样，系统会自动区分计算。

### Q: 账户额度足够为什么提示"额度不足"？

A: "账户额度"和"令牌额度"是分开的：

- **账户额度**（Quota）：用户的总额度，管理员可调整。
- **令牌额度**（Token Quota）：每个令牌独立的上限，由管理员在创建令牌时设置。

即使账户额度充足，如果令牌的剩余额度（`remain_quota`）不足，请求也会被拒绝。请检查令牌的剩余额度，或在令牌管理中增加令牌额度。

令牌额度可以理解为"该令牌最大可用额度"，由用户在创建令牌时自行设定。

### Q: 额度扣减和实际 token 数不一致？

A: 流式（stream）模式下，部分渠道不返回 usage 信息，One API 会使用 tiktoken 库估算 token 数，估算值与实际值可能存在偏差。

解决方案：

- 设置环境变量 `ENFORCE_INCLUDE_USAGE=true`，强制在 stream 模式下要求渠道返回 usage 信息。
- 如果渠道支持返回 usage，设置此变量后额度计算将更加精准。
- 在离线或不稳定网络环境下，可设置 `TIKTOKEN_CACHE_DIR` 缓存编码文件，避免每次启动都联网下载。

### Q: 如何调整模型倍率？

A: 在 Web UI 中进入"系统设置"页面，找到"模型倍率"相关配置项进行修改。修改后立即生效，无需重启服务。

模型倍率支持按模型名称单独设置，也可以设置默认倍率。

---

## 4. 网络与代理

### Q: 如何配置代理访问外网 API？

A: 设置以下环境变量：

```shell
# API 中继请求代理（主要）
RELAY_PROXY=http://proxy_host:port

# 用户上传内容下载代理（可选，如图片）
USER_CONTENT_REQUEST_PROXY=http://proxy_host:port
```

- `RELAY_PROXY`：所有的 API 中继请求（如调用 OpenAI、Claude 等）将通过此代理发送。
- `USER_CONTENT_REQUEST_PROXY`：下载用户上传的内容（如图片）时使用的代理，不设置则使用 `RELAY_PROXY` 的值。

### Q: 请求超时如何调整？

A: 分两层配置：

1. **One API 层**：设置 `RELAY_TIMEOUT=<秒数>`。对于 GPT-4 等慢模型，建议设置 300 秒以上。不设置则无超时间限制。
2. **Nginx 层**（如果使用反向代理），同步调整超时：
   ```nginx
   proxy_read_timeout 300s;
   proxy_connect_timeout 75s;
   proxy_send_timeout 300s;
   ```

用户上传内容下载的超时可单独设置 `USER_CONTENT_REQUEST_TIMEOUT`（默认 30 秒）。

### Q: ChatGPT Next Web 报错 `Failed to fetch`？

A: 可能原因：

1. 部署时不要设置 `BASE_URL`（One API 已经接管了 API 地址路由）。
2. 检查接口地址和 API Key 是否填写正确：
   - 接口地址应为 One API 的部署地址（如 `https://your-domain.com`）。
   - API Key 应为在 One API 中创建的 Token。
3. 检查是否启用了 HTTPS。浏览器会拦截 HTTPS 域名下的 HTTP 请求（混合内容），需确保 One API 也通过 HTTPS 提供服务。

---

## 5. 性能调优

### Q: 数据库连接数过多（Error 1040: Too many connections）？

A: 以下方法逐级排查：

1. **减小最大连接数**：设置环境变量 `SQL_MAX_OPEN_CONNS`（默认 1000），建议调整为 100-200：
   ```shell
   SQL_MAX_OPEN_CONNS=200
   ```

2. **启用批量更新**：设置 `BATCH_UPDATE_ENABLED=true`，将多次数据库写入聚合成一批，显著减少数据库连接占用：
   ```shell
   BATCH_UPDATE_ENABLED=true
   BATCH_UPDATE_INTERVAL=5
   ```

3. **启用 Redis 缓存**：配置 `REDIS_CONN_STRING` 和 `SYNC_FREQUENCY`，减少数据库读取频率。

4. **也建议同时调整** `SQL_MAX_IDLE_CONNS`（默认 100）和 `SQL_MAX_LIFETIME`（默认 60 分钟）以优化连接池行为。

### Q: 启用 Redis 后数据有延迟？

A: Redis 的配置同步间隔由 `SYNC_FREQUENCY` 控制，默认 600 秒（10 分钟）。

- 减小 `SYNC_FREQUENCY` 可以降低数据延迟，例如设为 60 秒。
- 但频率过高会增加数据库访问压力，请根据实际需求平衡。

```shell
SYNC_FREQUENCY=60
```

> 注意：启用 Redis 时，`SYNC_FREQUENCY` 必须设置，否则 Redis 不会生效。

### Q: 什么时候用批量更新（BATCH_UPDATE_ENABLED）？

A: 当数据库连接数成为瓶颈时启用 `BATCH_UPDATE_ENABLED=true`。

启用后：

- 优点：大幅减少数据库写入次数，降低连接占用，提升整体吞吐量。
- 缺点：额度更新会有延迟（默认 `BATCH_UPDATE_INTERVAL=5` 秒），不适合对额度实时性要求极高的场景。

如果对额度实时性要求很高（用户需要立即看到额度变化），建议不启用批量更新，而是优化数据库连接池配置或启用 Redis 缓存。

---

## 6. 数据库问题

### Q: 升级后数据会丢失吗？

A: 取决于使用的数据库类型和部署方式：

- **MySQL**：数据存储在外部数据库中，升级容器/程序不会丢失数据。
- **SQLite**：数据存储在 `one-api.db` 文件中。Docker 部署时必须挂载 volume：
  ```shell
  docker run -v /home/ubuntu/data/one-api:/data justsong/one-api
  ```
  否则容器销毁后数据会丢失。升级前建议备份数据库文件。

### Q: 升级前数据库需要做变更吗？

A: 一般情况下不需要。One API 使用 GORM AutoMigrate，在启动时会自动检测并调整表结构。

如有特殊的数据库迁移需求，维护者会在 [Release Notes](https://github.com/songquanpeng/one-api/releases) 中说明，并提供迁移脚本。

升级步骤建议：

1. 阅读当前版本的 Release Notes。
2. 备份数据库。
3. 拉取新版本镜像或代码。
4. 重启服务。

### Q: 手动修改数据库后报"数据库一致性已被破坏"？

A: 此错误表示 `abilities` 表中存在引用已删除渠道（`channels` 表）的记录。

原因：你删除了 `channels` 表中的渠道记录，但 `abilities` 表中对应的能力记录没有被同步清理。

修复方法：

```sql
-- 清理 abilities 表中 channel_id 不存在的记录
DELETE FROM abilities WHERE channel_id NOT IN (SELECT id FROM channels);
```

建议通过 Web UI 或 API 管理渠道的创建、修改和删除，避免直接操作数据库导致数据不一致。

---

## 7. OAuth 登录

### Q: GitHub OAuth 如何配置？

A: 步骤如下：

1. 登录 GitHub，进入 Settings -> Developer settings -> OAuth Apps -> New OAuth App。
2. 填写应用信息：
   - **Homepage URL**：One API 的部署地址（如 `https://your-domain.com`）。
   - **Authorization callback URL**：`https://your-domain.com/api/oauth/github`。
3. 创建应用后，获取 **Client ID** 和 **Client Secret**。
4. 在 One API 系统设置页面中填写上述信息。

### Q: 飞书登录回调地址怎么填？

A: 在飞书开放平台的应用配置中：

- 进入"安全设置"页面。
- **重定向 URL** 填写：`https://your-domain.com/api/oauth/lark`。

确保域名 `your-domain.com` 替换为你的实际部署域名。

### Q: OIDC 支持哪些提供商？

A: One API 支持任何符合 OIDC（OpenID Connect）协议的提供商，包括但不限于：

- Auth0
- Keycloak
- Azure AD（Microsoft Entra ID）
- Google Cloud Identity
- Okta

在系统设置页面配置以下参数：

- **Issuer URL**（issuer）：OIDC 提供商的发行者 URL。
- **Client ID**（client_id）：OIDC 应用客户端 ID。
- **Client Secret**（client_secret）：OIDC 应用客户端密钥。

---

## 8. 日志排查

### Q: 日志文件在哪里？

A: 日志位置取决于部署方式：

- **Docker 部署**：容器日志通过 `docker logs one-api` 查看标准输出。文件日志在容器内的 `--log-dir` 指定目录。
- **挂载 volume 后**：日志位于宿主机挂载目录下，默认路径为 `/home/ubuntu/data/one-api/logs/`。
- **手动部署**：日志目录由 `--log-dir` 命令行参数指定，默认为 `./logs`。

### Q: 如何查看请求是否有错误？

A: 两种方式：

1. **Web UI**：进入"日志"页面，可以按用户、渠道、模型筛选日志。点击具体日志项可查看请求和响应的详细内容。
2. **API 查询**：
   ```
   GET /api/log/self
   ```
   用户可以查询自己的请求日志。

日志页面会显示请求状态码、响应时间、消耗 token 数等关键信息，便于定位问题。

### Q: 可以单独存储日志到独立数据库吗？

A: 可以。设置 `LOG_SQL_DSN` 环境变量，为 `logs` 表指定独立的 MySQL/PostgreSQL 数据库：

```shell
LOG_SQL_DSN="root:password@tcp(localhost:3306)/oneapi_logs"
```

这样可以将业务数据和日志数据物理分离，减轻主数据库压力。此配置仅支持 MySQL 和 PostgreSQL，不支持 SQLite。

---

## 9. 安全与加固

### Q: 如何修改会话密钥？

A: 设置环境变量 `SESSION_SECRET`。多节点部署时所有节点必须使用相同的值：

```shell
SESSION_SECRET=<your_random_string>
```

> **注意**：不要使用示例值 `random_string`，请使用足够长的随机字符串（建议 32 字符以上）。

### Q: 如何限制 API 请求频率？

A: 设置以下环境变量：

```shell
# 单 IP 每 3 分钟内允许的最大 API 请求次数
GLOBAL_API_RATE_LIMIT=180

# 单 IP 每 3 分钟内允许的最大 Web 请求次数
GLOBAL_WEB_RATE_LIMIT=60
```

超出限制的请求会返回 429 Too Many Requests。

### Q: 如何设置首次启动时自动创建 Token？

A: 设置环境变量：

```shell
# 首次启动时自动为 root 用户创建普通 Token
INITIAL_ROOT_TOKEN=<your_token>

# 首次启动时自动为 root 用户创建 Access Token
INITIAL_ROOT_ACCESS_TOKEN=<your_access_token>
```

仅在首次启动时生效，后续启动不会覆盖已有配置。

---

## 10. 获取帮助

### 提交 Issue 前请确认

1. 已搜索 [GitHub Issues](https://github.com/songquanpeng/one-api/issues) 确认没有重复。
2. 已阅读本文档和 [配置参考](https://github.com/songquanpeng/one-api/blob/main/docs/getting-started/configuration.md)。
3. 使用的是最新版本（检查 [Releases](https://github.com/songquanpeng/one-api/releases)）。

### Issue 模板

报告问题时请提供以下信息：

- **One API 版本号**：如 v0.6.10。
- **部署方式**：Docker / 手动部署 / 其他。
- **部署环境**：操作系统、数据库类型（SQLite / MySQL / PostgreSQL）、是否使用 Redis。
- **复现步骤**：详细描述如何复现问题。
- **错误日志**：相关的错误日志片段（注意隐藏敏感信息如 API Key）。
- **截图**：如有界面问题，附上截图会更有帮助。

### 链接

- GitHub Issues：<https://github.com/songquanpeng/one-api/issues>
- Release Notes：<https://github.com/songquanpeng/one-api/releases>
- 配置参考：[docs/getting-started/configuration.md](../getting-started/configuration.md)
