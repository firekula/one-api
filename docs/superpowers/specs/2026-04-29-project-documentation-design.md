# One API 项目文档体系设计规格

**版本**: v1.0  
**日期**: 2026-04-29  
**状态**: 待实施

## 1. 背景与目标

One API 是一个开源的 LLM API 网关，通过标准 OpenAI API 格式统一访问 27+ 个大模型提供商。项目已有 README.md（部署配置为主）和 docs/API.md（简略管理 API 列表），但缺少架构设计、开发者指南、完整 API 参考等软件工程所需的关键文档。

**目标**：建立一套分层、按角色导航的文档体系，覆盖运维人员、API 使用者、开发者/贡献者三类读者。

## 2. 文档体系结构

采用分层式组织，通过中心索引页按角色和场景导航。

```
docs/
  README.md                          # 文档总索引（按角色 + 按场景导航）

  getting-started/                   # 🚀 面向运维 & 终端用户
    quick-start.md                   # 快速部署
    configuration.md                 # 环境变量 & 命令行参数完整参考
    user-manual.md                   # 使用说明（令牌、渠道、兑换码、充值）

  architecture/                      # 🏗️ 面向开发者 & 架构师
    overview.md                      # 系统架构总览 + 核心设计决策
    data-model.md                    # 数据库表结构 & 关系
    relay-system.md                  # 请求中继流程 + 适配器模式
    multi-node.md                    # 多机部署架构

  development/                       # 🔧 面向贡献者
    setup.md                         # 开发环境搭建
    adaptor-development.md           # 如何添加新的渠道适配器
    contribution-guide.md            # 贡献规范

  reference/                         # 📚 速查参考
    admin-api.md                     # 管理 API 完整参考
    faq.md                           # 常见问题 + 故障排查
```

共 **13 篇文档 + 1 个索引页 = 14 个文件**。中文编写，不要求多语言。

## 3. 各文档大纲

### 3.1 docs/README.md — 文档总索引

- **按角色导航**：
  - 运维人员 → getting-started/
  - API 使用者 → getting-started/user-manual + reference/admin-api
  - 开发者/贡献者 → architecture/ + development/
- **按场景导航**：
  - 我要部署 → quick-start
  - 我要配置环境变量 → configuration
  - 我要用 API → user-manual
  - 我要加新渠道 → adaptor-development
  - 我要排查问题 → faq
  - 我要了解架构 → overview
- **文档目录树**，每篇一句话描述
- **外部资源链接**（GitHub、演示站、相关项目）

### 3.2 getting-started/quick-start.md — 快速部署

- Docker 一键部署（SQLite / MySQL 两种命令）
- Docker Compose 部署
- 手动部署步骤（git clone → 构建前端 → 构建后端 → 运行）
- 宝塔面板一键部署
- Sealos / Zeabur / Render 第三方平台部署
- 部署后首次登录（root / 123456）+ 必须改密码的警告
- 初始配置三步：创建渠道 → 创建令牌 → 在客户端使用
- Nginx 反向代理 + HTTPS（Let's Encrypt certbot）
- Docker 版本升级（watchtower 命令）
- 初始账户与密码安全提醒

### 3.3 getting-started/configuration.md — 配置参考

- 配置方式说明（环境变量 / .env 文件 / 命令行参数），优先级
- 环境变量完整列表，按功能分类：
  - 数据库（SQL_DSN、SQL_MAX_IDLE_CONNS、SQL_MAX_OPEN_CONNS、SQL_CONN_MAX_LIFETIME、SQLITE_BUSY_TIMEOUT、SQLITE_PATH、LOG_SQL_DSN）
  - 缓存与同步（REDIS_CONN_STRING、REDIS_PASSWORD、REDIS_MASTER_NAME、MEMORY_CACHE_ENABLED、SYNC_FREQUENCY）
  - 节点模式（NODE_TYPE、FRONTEND_BASE_URL、SESSION_SECRET）
  - 速率限制（GLOBAL_API_RATE_LIMIT、GLOBAL_WEB_RATE_LIMIT）
  - 渠道运维（CHANNEL_UPDATE_FREQUENCY、CHANNEL_TEST_FREQUENCY、POLLING_INTERVAL）
  - 批量处理（BATCH_UPDATE_ENABLED、BATCH_UPDATE_INTERVAL）
  - 请求代理（RELAY_TIMEOUT、RELAY_PROXY、USER_CONTENT_REQUEST_TIMEOUT、USER_CONTENT_REQUEST_PROXY）
  - 编码器缓存（TIKTOKEN_CACHE_DIR、DATA_GYM_CACHE_DIR）
  - Gemini 专属（GEMINI_SAFETY_SETTING、GEMINI_VERSION）
  - 指标监控（ENABLE_METRIC、METRIC_QUEUE_SIZE、METRIC_SUCCESS_RATE_THRESHOLD）
  - 初始化（INITIAL_ROOT_TOKEN、INITIAL_ROOT_ACCESS_TOKEN）
  - UI（THEME、ENFORCE_INCLUDE_USAGE、TEST_PROMPT）
- 每个变量：名称、类型、默认值、示例、说明
- 命令行参数（--port、--log-dir、--version、--help）
- 生产环境推荐配置清单

### 3.4 getting-started/user-manual.md — 使用说明

- **核心概念**：渠道（上游 API Key 的封装）、令牌（分发给用户的 Key）、兑换码、用户分组、渠道分组、额度
- **Web 管理界面**：各页面功能概述
- **操作指南**：
  - 创建第一个渠道（以 OpenAI 为例，图解步骤）
  - 创建令牌（设置额度、有效期、IP 白名单、模型限制）
  - 在第三方客户端使用（ChatGPT Next Web、Cherry Studio、LangChain 等）
  - 指定渠道：`Authorization: Bearer ONE_API_KEY-CHANNEL_ID`
- **额度规则**：公式、倍率概念、流式/非流式 token 计数差异
- **兑换码**：批量生成、导出、用户兑换流程
- **用户管理**：注册方式（邮箱 / GitHub / 飞书 / OIDC / 微信）、分组、充值
- **系统公告与自定义设置**

### 3.5 architecture/overview.md — 系统架构总览

- 项目定位（一段话）
- 整体架构图（Mermaid graph：客户端 → Nginx → One API → 下游渠道）
- 技术栈列表：Go 1.20+ / Gin / GORM / React / SQLite-MySQL-PostgreSQL / Redis
- 目录结构一览及每个目录职责一句话：
  - `main.go` — 入口，组装启动
  - `common/` — 共享基础设施（config、logger、DB、加密、验证码、i18n）
  - `model/` — 数据模型 + DB 操作
  - `controller/` — HTTP 处理函数
  - `middleware/` — Gin 中间件
  - `router/` — 路由注册
  - `relay/` — 请求中继核心
  - `web/` — React 前端
- 核心设计决策（why）：
  - Adaptor 接口统一不同渠道
  - 渠道分配在中间件层（Distributor）
  - Master/Slave 节点分离
  - GORM AutoMigrate 做数据库版本管理
- 关键外部依赖及用途表

### 3.6 architecture/data-model.md — 数据模型

- ER 图（Mermaid erDiagram）
- 各表详解：
  - **users**：Id、Username、Password、DisplayName、Role（root/admin/user）、Status、Quota、UsedQuota、AccessToken、Group、AffCode、Email 等
  - **channels**：Id、Type、Key、Status、Name、Weight、BaseURL、Models、Group、ModelMapping、Priority、Config（JSON）、SystemPrompt、Balance、UsedQuota 等
  - **tokens**：Id、UserId、Key、Status、Name、RemainQuota、UnlimitedQuota、ExpiredTime、AllowIps、Models 等
  - **abilities**：Id、ChannelId、Model、Enabled — 渠道→模型的多对多映射
  - **redemptions**：Id、Key、Quota、Status、Creator、Redeemer
  - **logs**：Id、UserId、ChannelId、ModelName、PromptTokens、CompletionTokens、Quota、Content、CreatedAt
  - **options**：Key、Value — 系统配置 KV 存储
- 关键关系：User 1:N Token、Channel 1:N Ability、User 1:N Redemption
- 状态枚举值汇总
- 数据库迁移机制说明（GORM AutoMigrate 启动时自动执行）
- SQLite / MySQL / PostgreSQL 选择建议（规模 + 特性对比表）

### 3.7 architecture/relay-system.md — 请求中继系统

- 中继请求完整生命周期（Mermaid 时序图）：
  1. Token 鉴权（TokenAuth middleware）
  2. 请求体解析 + 模型名提取
  3. 渠道分配（Distribute middleware：负载均衡选渠道）
  4. Adaptor 初始化 + Request 转换
  5. 向渠道发起 HTTP 请求
  6. Adaptor DoResponse 转换响应 + 提取 usage
  7. 额度扣减（token & channel）
  8. 日志写入
- **Adaptor 接口详解**：每个方法（Init/GetRequestURL/SetupRequestHeader/ConvertRequest/ConvertImageRequest/DoRequest/DoResponse/GetModelList/GetChannelName）的输入输出、调用时机、职责边界
- **两种渠道模式**：
  - OpenAI 兼容直通（`openai.Adaptor`）：请求体基本透传，仅做 URL/Header 调整
  - 非 OpenAI 格式适配（如 Anthropic、Gemini、Baidu）：完整请求/响应格式转换
- **负载均衡算法**：`CacheGetRandomSatisfiedChannel` 实现 — 同分组过滤 → 模型匹配 → 按权重加权随机
- **失败重试机制**：`controller/relay.go` 中的重试循环 — 4xx 不重试（429 例外）、5xx 切换渠道重试，最多 `RetryTimes` 次
- **Stream 模式**：SSE 逐块读取+转换+转发，流控和错误处理
- **额度消费计算**：`分组倍率 × 模型倍率 × (prompt_tokens + completion_tokens × completion_ratio)`

### 3.8 architecture/multi-node.md — 多机部署架构

- 主从架构 Mermaid 图
- Master 节点职责：数据库迁移、配置管理、前端服务
- Slave 节点职责：仅处理 API 中继请求
- 部署前提：
  1. 同一 `SESSION_SECRET`
  2. 共用 MySQL/PostgreSQL（禁用 SQLite）
  3. Slave 设 `NODE_TYPE=slave`
  4. 所有节点配置 `SYNC_FREQUENCY` + Redis
  5. Slave 可选 `FRONTEND_BASE_URL` 重定向前端请求
- Redis 的作用：缓存共享 + 配置同步中介
- 同步流程：Master 写 DB → Slave 定期从 DB/Redis 拉取配置
- 典型部署拓扑（单机 / 主+单从 / 主+多从+Redis 集群）
- 从节点的限制与注意事项

### 3.9 development/setup.md — 开发环境搭建

- 前置依赖清单：Go 1.20+、Node.js 18+、npm、Git，可选的 MySQL/Redis
- 克隆仓库
- 后端环境配置：
  - `cp .env.example .env`
  - 编辑 `.env` 配置数据库（默认 SQLite 无需额外配置）
  - `go mod download`
  - `go run main.go`
- 前端环境配置（以 `web/default` 为例）：
  - `cd web/default && npm install`
  - `npm start`（开发服务器，默认端口可配代理到后端 3000）
- 开发联调：前端 dev server proxy 配置示例
- 调试开关：`GIN_MODE=debug`、`DEBUG=true`（打印请求体）、`DEBUG_SQL=true`
- 运行测试：`go test ./...`
- 构建生产版本：前端 `npm run build`、后端 `go build`

### 3.10 development/adaptor-development.md — 渠道适配器开发指南

- **何时需要新适配器**：渠道 API 非 OpenAI 兼容格式（需请求/响应转换）
- **何时不需要**：渠道已是 OpenAI 兼容 API → 直接在 `relay/adaptor/openai/compatible.go` 注册
- **目录结构惯例**：`relay/adaptor/<provider>/` 包含 `constants.go`（模型常量）、`main.go`（核心转换逻辑），可选 `adaptor.go`、`model.go`
- **实现 Adaptor 接口分步教程**：
  1. 定义渠道类型常量 `relay/channeltype/`
  2. `GetChannelName()` — 返回渠道显示名称
  3. `GetModelList()` — 返回支持模型列表
  4. `GetRequestURL(meta)` — 根据模式构造目标 URL
  5. `SetupRequestHeader(c, req, meta)` — 签名/鉴权头
  6. `ConvertRequest(c, mode, request)` — OpenAI 格式 → 渠道格式
  7. `DoResponse(c, resp, meta)` — 渠道响应 → OpenAI 格式 + 提取 usage
  8. 在 `relay/adaptor/` 入口注册适配器
- **以 BaiduV2 适配器为完整示例** walkthrough
- **前端联动**：在 `web/default/src/constants/channel.constants.js` 添加渠道选项
- **测试清单**：非流式 / 流式 / 错误响应 / 多模态（若有）
- 提交 PR 前 checklist

### 3.11 development/contribution-guide.md — 贡献规范

- 行为准则（引用 Contributor Covenant）
- Issue 提交流程（使用 GitHub 模板：bug_report / feature_request）
- 开发流程：Fork → Clone → Branch → Code → Commit → Push → PR
- 分支命名建议：`feat/xxx`、`fix/xxx`、`docs/xxx`
- Commit message 规范（参考项目历史）：`feat:` / `fix:` / `docs:` / `style:` / `chore:` / `refactor:`
- Go 代码风格：遵循 `gofmt`，保持与现有代码一致（命名、包组织）
- React 代码风格：遵循 ESLint 配置，功能组件 + Hooks
- 新增渠道的 PR checklist
- PR 描述模板

### 3.12 reference/admin-api.md — 管理 API 完整参考

- **鉴权方式**：Cookie（登录后）和 Access Token（`Authorization: Bearer <token>`）
- **通用响应格式**：`{"success": bool, "message": string, "data": any}`
- **API 列表**，按模块分节：
  - 用户管理：`GET/POST/PUT/DELETE /api/user/...`（CRUD、搜索、充值、自我管理）
  - 渠道管理：`GET/POST/PUT/DELETE /api/channel/...`（CRUD、搜索、测试、余额更新、模型列表）
  - 令牌管理：`GET/POST/PUT/DELETE /api/token/...`
  - 兑换码：`GET/POST/PUT/DELETE /api/redemption/...`
  - 日志：`GET/DELETE /api/log/...`（查询、统计、搜索、删除）
  - 系统选项：`GET/PUT /api/option/`
  - 认证相关：`POST /api/user/register`、`POST /api/user/login`、OAuth 端点
  - 仪表板：`GET /api/user/dashboard`
- **每个接口**：方法 + 路径、鉴权要求、请求体 JSON 示例、成功响应 JSON 示例、可能的错误码
- 未列出 API 的补充方式说明（浏览器开发者工具抓取前端请求）

### 3.13 reference/faq.md — 常见问题与故障排查

- **部署问题**：启动后空白页、端口占用、数据库文件权限、Docker 网络模式
- **渠道问题**：测试报错 `invalid character '<'`（CloudFlare 封禁）、无可用渠道、429 负载饱和、渠道自动禁用
- **额度问题**：额度计算规则详解、账户额度 vs 令牌额度、额度不足排查
- **网络/代理**：代理配置、超时设置
- **性能调优**：数据库连接数过多、缓存延迟、批量更新开关
- **数据库问题**：`数据库一致性已被破坏` 错误修复、升级后数据迁移
- **OAuth 登录**：飞书/GitHub/OIDC/微信 各配置要点
- **日志排查**：查看日志位置、关键日志字段含义
- 如何报告 bug（日志片段 + 环境信息 + 复现步骤）
- 获取帮助渠道（GitHub Issues）

## 4. 技术约束

- **语言**：全部中文，不要求多语言
- **格式**：Markdown，兼容 GitHub Flavored Markdown
- **图表**：使用 Mermaid 语法（GitHub 原生渲染支持）
- **存放路径**：`docs/` 目录，文档索引为 `docs/README.md`
- **现有文档处理**：
  - 项目根 README.md 不修改
  - docs/API.md 内容将被整合到 `reference/admin-api.md` 中并扩展，原文件保留为兼容链接

## 5. 非目标（本次不做）

- 多语言翻译（EN/JA）
- 视频教程
- API Playground / 交互式文档
- 自动生成代码文档（godoc）
- 文档站点（如 VitePress / Docusaurus），仅使用 GitHub 原生 Markdown 渲染
