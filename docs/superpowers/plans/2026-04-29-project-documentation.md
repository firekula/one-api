# One API 项目文档体系 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 One API 项目建立完整的分层文档体系，共 14 个 Markdown 文件，覆盖部署运维、架构设计、开发贡献、API 参考四类场景。

**Architecture:** 文档按 `getting-started/`、`architecture/`、`development/`、`reference/` 四个目录分层组织，`docs/README.md` 作为总索引按角色和场景导航。全部中文编写，使用 GitHub Flavored Markdown + Mermaid 图表。

**Tech Stack:** Markdown (GFM), Mermaid (图表), 纯文本编辑

**Source references:**
- `README.md` — 部署命令、环境变量列表、FAQ 原文
- `.env.example` — 环境变量
- `main.go` — 启动流程
- `model/main.go` — DB 初始化与迁移
- `model/user.go` — User 模型定义
- `model/channel.go` — Channel 模型定义
- `model/token.go` — Token 模型定义
- `model/option.go` — Option 模型定义
- `model/ability.go` — Ability 模型定义
- `model/log.go` — Log 模型定义
- `model/redemption.go` — Redemption 模型定义
- `router/api.go` — 管理 API 路由定义
- `router/relay.go` — 中继 API 路由定义
- `controller/relay.go` — 中继核心逻辑（重试、错误处理）
- `middleware/distributor.go` — 渠道分配逻辑
- `relay/adaptor/interface.go` — Adaptor 接口定义
- `relay/adaptor/common.go` — 通用请求辅助
- `relay/adaptor/openai/adaptor.go` — OpenAI 兼容适配器
- `relay/adaptor/baiduv2/` — BaiduV2 适配器示例
- `relay/channeltype/define.go` — 渠道类型常量
- `relay/controller/text.go` — 文本中继
- `relay/controller/image.go` — 图像中继
- `common/init.go` — 命令行参数 + 初始化
- `common/config/config.go` — 配置结构
- `web/default/src/constants/channel.constants.js` — 前端渠道选项
- `docs/API.md` — 现有 API 文档（将被整合）

---

### Task 1: 创建目录结构

**Files:**
- Create: `docs/README.md` (占位)
- Create: `docs/getting-started/` (目录)
- Create: `docs/architecture/` (目录)
- Create: `docs/development/` (目录)
- Create: `docs/reference/` (目录)

- [ ] **Step 1: 创建四个子目录**

```bash
mkdir -p docs/getting-started docs/architecture docs/development docs/reference
```

- [ ] **Step 2: 验证目录结构**

```bash
ls -la docs/getting-started docs/architecture docs/development docs/reference
```

Expected: 四个目录均存在且为空。

- [ ] **Step 3: 提交**

```bash
git add docs/getting-started docs/architecture docs/development docs/reference
git commit -m "chore: create docs directory structure"
```

---

### Task 2: 编写 docs/README.md — 文档总索引

**Files:**
- Create: `docs/README.md`

- [ ] **Step 1: 确认关键文档的最终文件名（已由 spec 确定）**

本文档需引用所有 13 篇子文档的相对路径，确认无歧义。

- [ ] **Step 2: 编写 docs/README.md**

内容结构：
- 文档体系简介（一句话）
- **按角色导航** 表格：
  | 角色 | 推荐阅读 |
  |------|---------|
  | 运维人员 | getting-started/ 全部 |
  | API 使用者 | getting-started/user-manual.md, reference/admin-api.md |
  | 开发者 | architecture/ 全部, development/ 全部 |
- **按场景导航** 列表：
  - 我要部署 → `getting-started/quick-start.md`
  - 我要配置环境变量 → `getting-started/configuration.md`
  - 我要使用 API → `getting-started/user-manual.md`
  - 我要了解架构 → `architecture/overview.md`
  - 我要添加新渠道 → `development/adaptor-development.md`
  - 我要排查问题 → `reference/faq.md`
  - 我要开发/贡献 → `development/setup.md` → `development/contribution-guide.md`
- **文档目录树**：列出全部 14 个文件的路径 + 一句话简介
- **外部资源**：GitHub 仓库、在线演示、相关项目链接

- [ ] **Step 3: 自检**

确认所有指向子文档的链接路径正确（相对路径，如 `getting-started/quick-start.md`）。

- [ ] **Step 4: 提交**

```bash
git add docs/README.md
git commit -m "docs: add documentation index page"
```

---

### Task 3: 编写 getting-started/quick-start.md — 快速部署

**Files:**
- Create: `docs/getting-started/quick-start.md`
- Reference: `README.md:128-178` (部署章节), `README.md:219-221` (手动部署), `README.md:234-267` (第三方平台)
- Reference: `Dockerfile`, `docker-compose.yml`

- [ ] **Step 1: 阅读参考源文件**

```bash
# 确认 Dockerfile 内容（镜像构建方式）
head -30 Dockerfile
# 确认 docker-compose 文件
cat docker-compose.yml 2>/dev/null || echo "no docker-compose.yml"
```

- [ ] **Step 2: 编写 quick-start.md**

从 README.md 的部署章节提取并重组内容：

1. **Docker 一键部署**
   - SQLite 模式命令（含 `-p`、`-e TZ`、`-v` 参数说明）
   - MySQL 模式命令（含 `-e SQL_DSN`）
   - 数据持久化路径说明
   - `--privileged=true` 的适用场景

2. **Docker Compose 部署**
   - 命令：`docker-compose up -d`
   - 查看状态：`docker-compose ps`

3. **手动部署**
   - `git clone` → `cd web/default && npm install && npm run build` → `go mod download && go build` → `./one-api`

4. **宝塔面板部署**
   - 简要步骤引用

5. **第三方平台部署**
   - Sealos 一键部署按钮
   - Zeabur 步骤
   - Render 直接部署镜像

6. **部署后操作**
   - ⚠️ 改密码警告（root / 123456）
   - 初始三步：创建渠道 → 创建令牌 → 客户端使用

7. **Nginx 反向代理配置**
   - 完整配置块（从 README 复制）
   - certbot HTTPS 步骤

8. **版本升级**
   - Docker watchtower 命令

- [ ] **Step 3: 自检**

确认所有 shell 命令可执行（变量占位符用 `<>` 标注清楚）。

- [ ] **Step 4: 提交**

```bash
git add docs/getting-started/quick-start.md
git commit -m "docs: add quick start guide"
```

---

### Task 4: 编写 getting-started/configuration.md — 配置参考

**Files:**
- Create: `docs/getting-started/configuration.md`
- Reference: `README.md:353-418` (环境变量), `README.md:419-423` (命令行参数), `.env.example`
- Reference: `common/init.go:13-18` (flag 定义), `common/config/config.go` (配置结构体)

- [ ] **Step 1: 阅读 common/config/config.go 确认所有配置项**

```bash
cat common/config/config.go
```

记录所有配置字段名、类型、默认值，确保文档与代码一致。

- [ ] **Step 2: 编写 configuration.md**

1. **配置方式概览**
   - 三种方式的优先级：命令行参数 > 环境变量 > .env 文件
   - `.env.example` 文件使用方法

2. **环境变量完整参考**（按功能分类，每个变量：名称、类型、默认值、示例、说明）

   数据库类：
   | 变量 | 默认值 | 说明 |
   |------|--------|------|
   | `SQL_DSN` | (空) | MySQL/PostgreSQL 连接串，不设置则用 SQLite |
   | `SQLITE_PATH` | `one-api.db` | SQLite 数据库文件路径 |
   | ... | | |

   缓存与同步类 / 节点模式类 / 速率限制类 / 渠道运维类 / 批量处理类 / 请求代理类 / 编码器缓存类 / Gemini专属 / 指标监控类 / 初始化类 / UI类 — 全部从 README 提取并验证。

   每个分类用 `###` 三级标题，变量用表格呈现。

3. **命令行参数**
   - `--port`：监听端口，默认 3000
   - `--log-dir`：日志目录，默认 `./logs`
   - `--version`：打印版本号并退出
   - `--help`：打印帮助信息

4. **生产环境推荐配置清单**
   - 必须设置：`SQL_DSN`（MySQL）、`SESSION_SECRET`、`TZ`
   - 建议设置：`REDIS_CONN_STRING`、`SYNC_FREQUENCY`、`CHANNEL_UPDATE_FREQUENCY`
   - 安全相关：`GLOBAL_API_RATE_LIMIT`、`GLOBAL_WEB_RATE_LIMIT`

- [ ] **Step 3: 与 common/config/config.go 交叉验证**

确保每个文档中的变量名与代码中的配置键一致。

- [ ] **Step 4: 提交**

```bash
git add docs/getting-started/configuration.md
git commit -m "docs: add configuration reference"
```

---

### Task 5: 编写 getting-started/user-manual.md — 使用说明

**Files:**
- Create: `docs/getting-started/user-manual.md`
- Reference: `README.md:323-347` (使用方法), `README.md:437-465` (常见问题中的额度相关)
- Reference: `controller/channel.go` (渠道 API), `controller/token.go` (令牌 API), `controller/user.go` (用户 API)

- [ ] **Step 1: 阅读 controller 文件了解功能边界**

快速浏览 `controller/channel.go`、`controller/token.go`、`controller/user.go`、`controller/redemption.go` 的开头部分，了解各控制器提供的功能。

- [ ] **Step 2: 编写 user-manual.md**

1. **核心概念解释**
   - 渠道（Channel）：上游 API Key 的封装，包含 Base URL、模型列表、分组
   - 令牌（Token）：分发给终端用户的 API Key，有额度和权限控制
   - 兑换码（Redemption）：预生成的充值码
   - 用户分组 / 渠道分组：用于控制不同用户群可见的渠道
   - 额度（Quota）：内部计费单位，公式见下

2. **Web 管理界面概览**
   - 侧边栏各菜单项功能简述
   - 管理面板 vs 普通用户面板的区别

3. **操作指南**
   - 创建第一个渠道（以 OpenAI 为例）
     - 类型选 OpenAI，填入 API Key，设置模型列表，保存
   - 创建令牌
     - 设置名称、额度、过期时间、IP 白名单、允许模型
   - 在客户端中使用
     - 设置 `OPENAI_API_BASE` 和 `OPENAI_API_KEY`
     - ChatGPT Next Web / Cherry Studio / LangChain 示例
   - 指定渠道：`Authorization: Bearer sk-xxx-CHANNEL_ID`

4. **额度规则**
   - 公式：`分组倍率 × 模型倍率 × (prompt_tokens + completion_tokens × completion_ratio)`
   - 倍率概念：分组倍率（用户分组 × 渠道分组）、模型倍率（每个模型不同）
   - 账户额度 vs 令牌额度的区别

5. **兑换码**
   - 管理员：批量生成 → 导出 → 分发
   - 用户：在 TopUp 页面输入兑换码充值

6. **用户管理**
   - 注册方式：邮箱 / GitHub OAuth / 飞书 OAuth / OIDC / 微信
   - 分组设置、手动充值

7. **系统设置**
   - 公告、充值链接、新用户初始额度、首页/关于页自定义

- [ ] **Step 3: 自检**

确认操作步骤的 UI 路径与实际前端一致（参照 `web/default/src/pages/` 页面结构）。

- [ ] **Step 4: 提交**

```bash
git add docs/getting-started/user-manual.md
git commit -m "docs: add user manual"
```

---

### Task 6: 编写 architecture/overview.md — 系统架构总览

**Files:**
- Create: `docs/architecture/overview.md`
- Reference: `main.go` (启动流程), `router/main.go` (路由组装), `common/init.go`, `go.mod`

- [ ] **Step 1: 梳理完整目录结构**

```bash
find . -maxdepth 2 -type d ! -path './.git/*' ! -path './node_modules/*' ! -path './web/*/node_modules/*' | sort
```

- [ ] **Step 2: 编写 overview.md**

1. **项目定位**（一段话）：OpenAI 兼容 API 网关，统一分发 27+ LLM 提供商

2. **整体架构图**（Mermaid graph TD）：
   ```
   客户端 → Nginx(可选) → One API(Gin) → 下游渠道
                                    ↓
                              MySQL/SQLite + Redis
   ```

3. **技术栈**
   | 层级 | 技术 | 版本 |
   |------|------|------|
   | 语言 | Go | 1.20+ |
   | Web框架 | Gin | 1.10 |
   | ORM | GORM | 1.25 |
   | 数据库 | SQLite / MySQL / PostgreSQL | — |
   | 缓存 | Redis (可选) | — |
   | 前端 | React | — |

4. **目录结构一览**
   - `main.go` — 入口，组装中间件、路由、启动服务
   - `common/` — 共享基础设施：config、logger、database、加密、验证码、i18n、网络工具
   - `model/` — 数据模型定义 + 数据库 CRUD 操作
   - `controller/` — HTTP 请求处理函数（业务逻辑层）
   - `middleware/` — Gin 中间件：鉴权、CORS、分发、限流、日志、恢复
   - `router/` — 路由注册：API路由、Dashboard路由、中继路由、Web路由
   - `relay/` — 请求中继核心：adaptor 接口 + 28 个渠道实现 + 请求/响应模型
   - `web/` — React 前端（两个主题：default、berry、air）
   - `docs/` — 项目文档
   - `bin/` — 数据库迁移脚本

5. **核心设计决策**
   - Adaptor 接口统一渠道：每个渠道只需实现 9 个方法即可接入
   - 渠道分配在中间件层（Distributor）：鉴权后、处理前完成渠道选择
   - Master/Slave 节点分离：Master 负责 DB 迁移和配置管理，Slave 只做中继
   - GORM AutoMigrate：数据库版本管理绑定在启动流程中，零运维介入

6. **关键外部依赖**（从 go.mod 提取主要依赖及用途表）

- [ ] **Step 3: 自检**

Mermaid 语法正确，目录结构与实际一致。

- [ ] **Step 4: 提交**

```bash
git add docs/architecture/overview.md
git commit -m "docs: add architecture overview"
```

---

### Task 7: 编写 architecture/data-model.md — 数据模型

**Files:**
- Create: `docs/architecture/data-model.md`
- Reference: `model/main.go:137-164` (migrateDB), `model/user.go:19-54` (User struct), `model/channel.go:20-41` (Channel struct)
- Reference: `model/token.go` (Token struct), `model/ability.go`, `model/redemption.go`, `model/option.go`, `model/log.go`

- [ ] **Step 1: 读取所有模型文件确认字段**

```bash
cat model/token.go | head -40
cat model/ability.go | head -30
cat model/redemption.go | head -30
cat model/option.go | head -20
cat model/log.go | head -40
```

- [ ] **Step 2: 编写 data-model.md**

1. **ER 图**（Mermaid erDiagram）
   ```
   User ||--o{ Token : owns
   User ||--o{ Redemption : redeems
   Channel ||--o{ Ability : has
   User ||--o{ Log : generates
   ```

2. **各表字段详解**

   **users** 表：
   | 字段 | 类型 | 约束 | 说明 |
   |------|------|------|------|
   | Id | int | PK | 自增主键 |
   | Username | string | unique, index, max=12 | 登录名 |
   | Password | string | not null, min=8 | bcrypt 哈希 |
   | DisplayName | string | index, max=20 | 显示名 |
   | Role | int | default:1 | 0=guest, 1=user, 10=admin, 100=root |
   | Status | int | default:1 | 1=enabled, 2=disabled, 3=deleted |
   | Email | string | index | 邮箱 |
   | GitHubId | string | index | GitHub OAuth ID |
   | WeChatId | string | index | 微信 OAuth ID |
   | LarkId | string | index | 飞书 OAuth ID |
   | OidcId | string | index | OIDC ID |
   | AccessToken | string | char(32), unique | 系统管理令牌 |
   | Quota | int64 | default:0 | 剩余额度 |
   | UsedQuota | int64 | default:0 | 已用额度 |
   | RequestCount | int | default:0 | 请求次数 |
   | Group | string | default:"default" | 用户分组 |
   | AffCode | string | unique | 邀请码 |
   | InviterId | int | index | 邀请人ID |

   **channels** 表：同样列出完整字段。
   **tokens** 表：同样列出完整字段。
   **abilities** 表：Id、ChannelId、Model、Enabled。
   **redemptions** 表：Id、Key、Quota、Status、Creator、Redeemer。
   **logs** 表：Id、UserId、ChannelId、ModelName、PromptTokens、CompletionTokens、Quota、Content、CreatedAt。
   **options** 表：Key、Value。

3. **状态枚举值汇总**
   - ChannelStatus: 0=未知, 1=启用, 2=手动禁用, 3=自动禁用
   - UserStatus: 1=启用, 2=禁用, 3=已删除
   - UserRole: 0=guest, 1=user, 10=admin, 100=root

4. **数据库迁移机制**
   - GORM AutoMigrate 在 `model/migrateDB()` 中按顺序创建/更新表
   - 启动时自动执行，无需手动 SQL
   - 历史迁移脚本在 `bin/migration_v*.sql`

5. **数据库选择建议**
   | 场景 | 推荐 | 原因 |
   |------|------|------|
   | 个人/低并发 | SQLite | 零配置，单文件 |
   | 生产/高并发 | MySQL | 连接池、并发写入 |
   | 多机部署 | MySQL/PostgreSQL | 支持远程连接 |

- [ ] **Step 3: 与各 model/*.go 文件交叉验证字段**

确保 ER 图和字段表与 struct tag 一致。

- [ ] **Step 4: 提交**

```bash
git add docs/architecture/data-model.md
git commit -m "docs: add data model reference"
```

---

### Task 8: 编写 architecture/relay-system.md — 请求中继系统

**Files:**
- Create: `docs/architecture/relay-system.md`
- Reference: `controller/relay.go` (中继入口+重试), `middleware/distributor.go` (渠道分配), `middleware/auth.go` (TokenAuth)
- Reference: `relay/adaptor/interface.go` (Adaptor接口), `relay/adaptor/common.go` (通用请求辅助)
- Reference: `relay/adaptor/openai/adaptor.go` (OpenAI兼容适配器示例)
- Reference: `relay/controller/text.go`, `relay/controller/image.go` (中继模式分支)

- [ ] **Step 1: 阅读中继相关源文件**

```bash
cat relay/adaptor/interface.go
cat relay/adaptor/common.go
cat relay/controller/text.go | head -60
cat relay/controller/helper.go | head -60
```

- [ ] **Step 2: 编写 relay-system.md**

1. **请求生命周期**（Mermaid 时序图）
   ```
   Client → TokenAuth → 解析请求体 → Distributor(选渠道) → SetupContext → Adaptor.ConvertRequest → HTTP请求 → Adaptor.DoResponse → 额度扣减 → 日志记录 → Client
   ```

2. **8 步详解**：
   - (1) TokenAuth 中间件：从 `Authorization: Bearer sk-xxx` 提取令牌，查 DB/Cache 验证，设置 userId
   - (2) 请求体解析：提取 `model` 字段 → 存为 `RequestModel`；提取 `stream` 字段 → 存为 `IsStream`
   - (3) 渠道分配（Distributor）：见下文负载均衡
   - (4) Adaptor 初始化 + 转换：`a.Init(meta)` → `a.ConvertRequest(c, mode, body)` → `a.GetRequestURL(meta)`
   - (5) HTTP 请求：`a.DoRequest(c, meta, body)` → `DoRequestHelper` → `client.HTTPClient.Do(req)`
   - (6) DoResponse：流式 `StreamHandler`，非流式 `Handler`，统一提取 `model.Usage`
   - (7) 额度扣减：`model.DecreaseUserQuota()` + `model.UpdateChannelUsedQuota()`
   - (8) 日志写入：`model.RecordLog()`

3. **Adaptor 接口详解**
   | 方法 | 职责 | 调用位置 |
   |------|------|---------|
   | `Init(meta)` | 设置渠道类型 | relay controller |
   | `GetRequestURL(meta)` | 构造目标 URL | DoRequestHelper |
   | `SetupRequestHeader(c,req,meta)` | 设置鉴权头 | DoRequestHelper |
   | `ConvertRequest(c,mode,body)` | OpenAI→渠道格式 | relay controller |
   | `DoRequest(c,meta,body)` | 发起 HTTP 请求 | relay controller |
   | `DoResponse(c,resp,meta)` | 渠道响应→OpenAI格式+usage | relay controller |
   | `GetModelList()` | 返回模型列表 | channel 注册 |
   | `GetChannelName()` | 返回渠道名 | channel 注册 |

4. **两种渠道模式对比**
   - OpenAI 兼容直通（Azure、Ollama、Groq 等）：请求体基本透传，仅调整 URL/Header
   - 非 OpenAI 适配（Anthropic、Gemini、Baidu 等）：完整双向格式转换

5. **负载均衡算法**
   - 核心函数：`model.CacheGetRandomSatisfiedChannel(group, model, ignoreFirst)`
   - 流程：按分组过滤 → 按模型匹配 → 按权重加权随机选择
   - 权重高者优先，等权重时均匀分布

6. **失败重试机制**
   - 位置：`controller/relay.go` 的 `Relay()` 函数
   - 规则：4xx 不重试（除去 429），5xx 切换渠道重试
   - 最大重试次数：`config.RetryTimes`
   - 重试时 `ignoreFirst=true` 排除上次失败的渠道

7. **Stream 模式处理**
   - SSE 协议：`text/event-stream`，逐行读取 `data: {...}\n\n`
   - 流式适配：在 `StreamHandler` 中逐块转换 + 实时转发
   - usage 处理：强制开启 `stream_options.include_usage`

8. **额度消费公式**
   ```
   消费额度 = 分组倍率 × 模型倍率 × (prompt_tokens + completion_tokens × completion_ratio)
   ```

- [ ] **Step 3: 验证代码引用**

确保所有函数名、文件路径与实际代码一致。

- [ ] **Step 4: 提交**

```bash
git add docs/architecture/relay-system.md
git commit -m "docs: add relay system architecture"
```

---

### Task 9: 编写 architecture/multi-node.md — 多机部署架构

**Files:**
- Create: `docs/architecture/multi-node.md`
- Reference: `README.md:223-231` (多机部署), `model/main.go:111-164` (InitDB 中 IsMasterNode 逻辑)
- Reference: `common/config/config.go` (IsMasterNode, NODE_TYPE 相关), `common/redis.go`

- [ ] **Step 1: 阅读多机部署相关代码**

```bash
grep -rn "IsMasterNode\|NODE_TYPE\|FRONTEND_BASE_URL\|SYNC_FREQUENCY" --include="*.go" | head -20
cat common/redis.go
```

- [ ] **Step 2: 编写 multi-node.md**

1. **主从架构图**（Mermaid graph）
   ```
   用户 → Nginx(负载均衡) → Master(管理+中继) + Slave1(中继) + Slave2(中继)
                              ↓                    ↓            ↓
                            MySQL ──────── Redis(缓存共享)
   ```

2. **Master 节点职责**
   - 执行数据库迁移（`IsMasterNode` 判断）
   - 管理 Options 配置（`InitOptionMap`）
   - 提供 Web 前端服务
   - 处理中继请求（同时承担）

3. **Slave 节点职责**
   - 仅处理 API 中继请求
   - 不执行 DB 迁移
   - 通过 Redis 缓存读取配置，零 DB 访问（缓存命中时）

4. **部署前提条件**（5 条逐条说明）
   - `SESSION_SECRET` 一致
   - 共用 MySQL/PostgreSQL
   - Slave 设 `NODE_TYPE=slave`
   - 配置 `SYNC_FREQUENCY` + Redis
   - Slave 可选 `FRONTEND_BASE_URL` 重定向前端

5. **Redis 的三种角色**
   - 配置缓存（避免 Slave 频繁查 DB）
   - 限流计数器（`GLOBAL_API_RATE_LIMIT`）
   - 渠道/用户状态同步

6. **同步流程**
   - Master 启动 → 从 DB 加载配置 → 写入缓存
   - Slave 启动 → 从 Redis 加载缓存 → 定期从 Redis/DB 同步（`SYNC_FREQUENCY` 秒）

7. **典型部署拓扑**
   - 单机（默认）：SQLite，零配置
   - 主+单从：MySQL + Redis 单实例
   - 主+多从+Redis集群：生产级高可用

8. **从节点限制**
   - 不能在 Slave 上修改配置（会被 Master 覆盖）
   - 不能在 Slave 上访问管理面板（可重定向到 Master）

- [ ] **Step 3: 自检**

架构图与实际部署方式一致。

- [ ] **Step 4: 提交**

```bash
git add docs/architecture/multi-node.md
git commit -m "docs: add multi-node deployment architecture"
```

---

### Task 10: 编写 development/setup.md — 开发环境搭建

**Files:**
- Create: `docs/development/setup.md`
- Reference: `README.md:199-213` (手动部署/编译), `go.mod`, `main.go`, `web/default/package.json`

- [ ] **Step 1: 确认前端项目结构**

```bash
ls web/
cat web/default/package.json
```

- [ ] **Step 2: 编写 setup.md**

1. **前置依赖**
   - Go 1.20+
   - Node.js 18+ & npm
   - Git
   - (可选) MySQL 8.0+
   - (可选) Redis 7+

2. **克隆项目**
   ```bash
   git clone https://github.com/songquanpeng/one-api.git
   cd one-api
   ```

3. **后端环境**
   ```bash
   cp .env.example .env
   # 编辑 .env，默认 SQLite 无需改动
   go mod download
   go run main.go --port 3000 --log-dir ./logs
   ```
   访问 http://localhost:3000

4. **前端环境（web/default）**
   ```bash
   cd web/default
   npm install
   npm start  # 启动在 localhost:3001
   ```

5. **前后端联调**
   在 `web/default/package.json` 中添加 proxy 配置：
   ```json
   "proxy": "http://localhost:3000"
   ```
   前端 API 请求自动代理到后端。

6. **调试开关**
   - `GIN_MODE=debug`：Gin 调试模式
   - `DEBUG=true`：打印请求体
   - `DEBUG_SQL=true`：打印 SQL 日志

7. **运行测试**
   ```bash
   go test ./...
   ```

8. **构建生产版本**
   ```bash
   cd web/default && npm run build
   cd ../.. && go build -ldflags "-s -w" -o one-api
   ```

- [ ] **Step 3: 实际执行验证命令**

确认 `go mod download` 和 `npm install` 的依赖名称正确。

- [ ] **Step 4: 提交**

```bash
git add docs/development/setup.md
git commit -m "docs: add development environment setup guide"
```

---

### Task 11: 编写 development/adaptor-development.md — 渠道适配器开发指南

**Files:**
- Create: `docs/development/adaptor-development.md`
- Reference: `relay/adaptor/interface.go` (Adaptor 接口), `relay/adaptor/baiduv2/` (完整适配器示例)
- Reference: `relay/adaptor/openai/compatible.go` (OpenAI兼容注册), `relay/channeltype/define.go` (渠道类型常量)
- Reference: `web/default/src/constants/channel.constants.js` (前端渠道选项)

- [ ] **Step 1: 阅读完整 BaiduV2 适配器作为 walkthrough 素材**

```bash
cat relay/adaptor/baiduv2/main.go
cat relay/adaptor/baiduv2/constants.go
cat relay/channeltype/define.go | head -80
cat relay/adaptor/openai/compatible.go
```

- [ ] **Step 2: 编写 adaptor-development.md**

1. **判断：需要新适配器？**
   - 需要：渠道 API 格式非 OpenAI 兼容（如 Anthropic Messages API、Gemini generateContent API）
   - 不需要：渠道已是 OpenAI 兼容格式 → 直接在 `openai/compatible.go` 注册渠道类型+BaseURL+模型列表

2. **适配器目录结构**
   ```
   relay/adaptor/<provider>/
     constants.go   # 模型常量 Map
     main.go        # ConvertRequest + DoResponse 核心逻辑
     adaptor.go     # (可选) 自定义 Adaptor 结构体
     model.go       # (可选) 渠道专用请求/响应模型
   ```

3. **分步实现教程**（以 BaiduV2 为完整示例）

   **Step 1** — 在 `relay/channeltype/define.go` 中定义渠道类型常量：
   ```go
   const BaiduV2 = 44
   ```

   **Step 2** — 在 `relay/channeltype/` 中注册渠道元数据（名称+BaseURL+支持的模型列表）

   **Step 3** — 创建 `relay/adaptor/baiduv2/constants.go`，定义模型常量 Map：
   ```go
   var ModelList = []string{"ernie-4.0-turbo-8k", ...}
   ```

   **Step 4** — 创建 `relay/adaptor/baiduv2/main.go`，实现核心转换：
   - `GetRequestURL(meta)` → `fmt.Sprintf("%s?access_token=%s", baseURL, token)`
   - `ConvertRequest(c, mode, body)` → 构造 Baidu 格式请求体
   - `DoResponse(c, resp, meta)` → 解析 Baidu 响应，转换为 OpenAI ChatCompletion 格式 + 提取 Usage

   **Step 5** — 在适配器注册表（`relay/adaptor/` 入口）中添加新渠道类型的映射

   **Step 6** — 前端联动：在 `web/default/src/constants/channel.constants.js` 中添加渠道选项

4. **Adaptor 接口方法实现要点**
   - `GetRequestURL`：处理不同模式（chat/images/embeddings）返回不同 URL
   - `SetupRequestHeader`：签名鉴权（Bearer、API-Key、HMAC 等）
   - `ConvertRequest`：处理 system message、stream_options、reasoning_effort 等特殊字段
   - `DoResponse`：分 StreamHandler 和普通 Handler 两路，务必提取 usage

5. **测试清单**
   - [ ] 非流式请求 → 返回正确 `choices[0].message.content`
   - [ ] 流式请求 → SSE 逐块正确
   - [ ] 错误响应 → 返回标准 OpenAI error 格式
   - [ ] usage 提取 → `prompt_tokens` + `completion_tokens` 正确
   - [ ] 多模态（若有）→ 图片 URL/base64 传递正确

6. **提交 PR 前 Checklist**

- [ ] **Step 3: 与 BaiduV2 代码交叉验证**

确保 walkthrough 步骤与实际代码一致。

- [ ] **Step 4: 提交**

```bash
git add docs/development/adaptor-development.md
git commit -m "docs: add adaptor development guide"
```

---

### Task 12: 编写 development/contribution-guide.md — 贡献规范

**Files:**
- Create: `docs/development/contribution-guide.md`
- Reference: `.github/ISSUE_TEMPLATE/bug_report.md`, `.github/ISSUE_TEMPLATE/feature_request.md`
- Reference: `.github/workflows/ci.yml` (CI 流程)
- Reference: 项目 git log 的 commit message 模式

- [ ] **Step 1: 阅读 Issue 模板和 CI 配置**

```bash
cat .github/ISSUE_TEMPLATE/bug_report.md
cat .github/ISSUE_TEMPLATE/feature_request.md
cat .github/workflows/ci.yml
```

- [ ] **Step 2: 编写 contribution-guide.md**

1. **行为准则**（简短，引用 Contributor Covenant）

2. **如何报告 Bug**
   - 使用 GitHub Issues + bug_report 模板
   - 提供：环境信息（OS、部署方式、版本）、复现步骤、日志片段、期望 vs 实际行为

3. **如何提 Feature Request**
   - 使用 feature_request 模板
   - 描述：使用场景、期望行为、备选方案

4. **开发流程**
   ```
   Fork → git clone → git checkout -b feat/xxx → 开发 → git commit → git push → 创建 PR
   ```

5. **分支命名**
   - `feat/xxx` — 新功能
   - `fix/xxx` — Bug 修复
   - `docs/xxx` — 文档
   - `chore/xxx` — 杂项

6. **Commit Message 规范**
   ```
   feat: 简短描述
   fix: 简短描述
   docs: 简短描述
   style: 简短描述
   chore: 简短描述
   refactor: 简短描述
   ```
   参考项目历史风格，中文或英文均可。

7. **Go 代码风格**
   - `gofmt` 格式化
   - 与现有代码保持一致的命名风格（驼峰、包名小写）
   - 错误处理：返回 error 而非 panic

8. **React 代码风格**
   - 功能组件 + Hooks
   - 遵循已有 ESLint 配置
   - 状态管理：React Context（`src/context/`）

9. **新增渠道 PR Checklist**
   - [ ] 后端：`relay/adaptor/<provider>/` 完整实现
   - [ ] 后端：`relay/channeltype/` 注册
   - [ ] 后端：在适配器入口注册映射
   - [ ] 前端：`channel.constants.js` 添加选项
   - [ ] 测试：非流式 + 流式 + 错误 + usage
   - [ ] PR 描述填写完整

10. **PR 描述模板**
    ```
    ## 变更内容
    ## 测试方式
    ## 截图（如有前端变更）
    ```

- [ ] **Step 3: 提交**

```bash
git add docs/development/contribution-guide.md
git commit -m "docs: add contribution guide"
```

---

### Task 13: 编写 reference/admin-api.md — 管理 API 完整参考

**Files:**
- Create: `docs/reference/admin-api.md`
- Reference: `router/api.go` (完整 API 路由), `router/relay.go` (中继路由), `controller/*.go` (各控制器)
- Reference: `docs/API.md` (现有 API 文档，将被整合)
- Reference: `middleware/auth.go` (鉴权逻辑)

- [ ] **Step 1: 从 router/api.go 提取完整 API 列表**

逐条列出所有路由的方法、路径、鉴权中间件、handler 函数名。

- [ ] **Step 2: 阅读 controller 文件，提取请求/响应格式**

阅读各 controller 的函数签名和 JSON 绑定结构，提取请求体字段和响应格式。

- [ ] **Step 3: 编写 admin-api.md**

1. **鉴权方式**
   - Cookie（浏览器登录后自动携带）
   - Access Token：`Authorization: Bearer <user_access_token>`
   - Access Token 获取：Web UI → 个人设置 → 生成系统访问令牌

2. **通用响应格式**
   ```json
   {
     "success": true | false,
     "message": "操作描述",
     "data": {}
   }
   ```

3. **API 列表**（按模块分节）

   **用户管理** (`/api/user/...`)：
   | 方法 | 路径 | 鉴权 | 说明 | 请求体 |
   |------|------|------|------|--------|
   | POST | /api/user/register | Turnstile | 注册 | `{"username":"", "password":"", "email":""}` |
   | POST | /api/user/login | — | 登录 | `{"username":"", "password":""}` |
   | GET | /api/user/self | UserAuth | 获取当前用户 | — |
   | PUT | /api/user/self | UserAuth | 更新当前用户 | `{"display_name":"", "email":""}` |
   | GET | /api/user/dashboard | UserAuth | 仪表板数据 | — |
   | GET | /api/user/token | UserAuth | 生成访问令牌 | — |
   | GET | /api/user/available_models | UserAuth | 可用模型列表 | — |
   | GET | /api/user/ | AdminAuth | 所有用户列表 | Query: `p`, `order` |
   | GET | /api/user/search | AdminAuth | 搜索用户 | Query: `keyword` |
   | GET | /api/user/:id | AdminAuth | 用户详情 | — |
   | POST | /api/user/ | AdminAuth | 创建用户 | `{"username":"","password":"","display_name":""}` |
   | PUT | /api/user/ | AdminAuth | 编辑用户 | `{"id":1, "quota":1000, ...}` |
   | POST | /api/user/manage | AdminAuth | 管理操作 | `{"id":1, "action":"disable"}` |
   | DELETE | /api/user/:id | AdminAuth | 删除用户 | — |
   | POST | /api/topup | AdminAuth | 充值 | `{"user_id":1, "quota":100000}` |

   每个 API 给出请求体 JSON 示例和成功响应 JSON 示例。

   同理展开：
   **渠道管理** — `/api/channel/...` (14 个端点)
   **令牌管理** — `/api/token/...` (6 个端点)
   **兑换码** — `/api/redemption/...` (6 个端点)
   **日志** — `/api/log/...` (8 个端点)
   **系统选项** — `/api/option/` (2 个端点)
   **认证** — OAuth 端点 (5 个)
   **通用** — `/api/status`, `/api/models`, `/api/notice`, `/api/about`, `/api/home_page_content`

4. **中继 API** (`/v1/...` 路由)
   - 说明：这些 API 面向终端用户，使用 Token（非 Access Token）鉴权
   - 列出所有 `/v1/chat/completions`、`/v1/images/generations` 等端点
   - 标注已实现 vs `RelayNotImplemented`

- [ ] **Step 4: 与 router/api.go 交叉验证**

逐条对比确保无遗漏。

- [ ] **Step 5: 提交**

```bash
git add docs/reference/admin-api.md
git commit -m "docs: add admin API reference"
```

---

### Task 14: 编写 reference/faq.md — 常见问题与故障排查

**Files:**
- Create: `docs/reference/faq.md`
- Reference: `README.md:436-465` (现有 FAQ 9 条)

- [ ] **Step 1: 从 README 提取现有 FAQ 内容**

README.md 第 436-465 行的 9 条 FAQ 作为基础。

- [ ] **Step 2: 编写 faq.md**

按问题类别分节，每节 2-5 个 Q&A，每个问题用 `### Q: ...` 格式，答案紧接其后。

1. **部署问题**
   - Q: 部署后访问出现空白页面？
   - Q: Docker 容器无法启动？（端口占用、权限问题）
   - Q: SQLite 数据如何持久化？

2. **渠道问题**
   - Q: 渠道测试报错 `invalid character '<'`？
   - Q: 提示"当前分组负载已饱和"？
   - Q: 提示"无可用渠道"？
   - Q: 渠道被自动禁用怎么回事？

3. **额度问题**
   - Q: 额度怎么计算的？
   - Q: 账户额度足够为什么提示不足？
   - Q: 额度扣减和实际 token 不一致？

4. **网络与代理**
   - Q: 如何配置代理访问外网 API？
   - Q: 请求超时如何调整？

5. **性能调优**
   - Q: 数据库连接数过多（Error 1040）？
   - Q: 启用 Redis 后数据有延迟？
   - Q: 什么时候用批量更新？

6. **数据库问题**
   - Q: 升级后数据会丢失吗？
   - Q: 升级前数据库需要做变更吗？
   - Q: 手动修改数据库后报"数据库一致性已被破坏"？

7. **OAuth 登录**
   - Q: GitHub OAuth 如何配置？
   - Q: 飞书登录回调地址怎么填？
   - Q: OIDC 支持哪些提供商？

8. **日志排查**
   - Q: 日志文件在哪里？
   - Q: 如何查看请求是否有错误？

9. **获取帮助**
   - GitHub Issues 链接
   - 如何提供有效信息（日志片段 + 环境 + 复现步骤）

- [ ] **Step 3: 提交**

```bash
git add docs/reference/faq.md
git commit -m "docs: add FAQ and troubleshooting guide"
```

---

### Task 15: 最终审查与收尾

**Files:**
- Modify: `docs/API.md` (添加兼容性说明)
- Modify: `docs/README.md` (确保所有链接正确)

- [ ] **Step 1: 更新 docs/API.md 添加指向新文档的链接**

在 `docs/API.md` 顶部添加：
```markdown
> **注意**：本文档内容已整合到 [管理 API 完整参考](reference/admin-api.md) 中。
> 本文件保留作为兼容链接，推荐查阅完整版本。
```

- [ ] **Step 2: 验证 docs/README.md 中的所有链接**

```bash
# 确认所有被引用的文件都存在
for f in \
  getting-started/quick-start.md \
  getting-started/configuration.md \
  getting-started/user-manual.md \
  architecture/overview.md \
  architecture/data-model.md \
  architecture/relay-system.md \
  architecture/multi-node.md \
  development/setup.md \
  development/adaptor-development.md \
  development/contribution-guide.md \
  reference/admin-api.md \
  reference/faq.md; do
  test -f "docs/$f" && echo "✓ docs/$f" || echo "✗ MISSING: docs/$f"
done
```

- [ ] **Step 3: 列出所有新文件确认完整**

```bash
find docs -name "*.md" -type f | sort
```

Expected: 14+ 个 .md 文件（含原 API.md 和索引）。

- [ ] **Step 4: 最终提交**

```bash
git add docs/
git commit -m "docs: add comprehensive project documentation (14 documents across 4 categories)"
```
