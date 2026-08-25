# One API (AGENTS.md)

通过标准 OpenAI API 格式访问所有大模型的 API 网关：渠道管理、令牌/额度、负载均衡、多模型适配、流式转发。Go 后端 + React 前端（多主题，构建产物 `embed` 进二进制）。

## Project

- 栈：Go 1.20（CI 用 ^1.22）+ gin + GORM（sqlite/mysql/postgres）+ Redis（可选）；前端 React/CRA，位于 `web/`（default/berry/air 三个主题）。
- 入口：`main.go`（`common.Init()` → `model.InitDB()` → Redis → `router.SetRouter(server, buildFS)` → 监听 `:3000`，可用 `PORT` 覆盖）。
- 模块路径：`github.com/songquanpeng/one-api/...`。配置走环境变量 + `.env`（godotenv autoload），见 `common/config/`、`.env.example`。

## Commands

- 测试：`go test ./...`（CI 用 `go test -cover -coverprofile=coverage.txt ./...`）
- 运行：`go run .`（sqlite 需要 CGO，Windows 本地需 `$env:CGO_ENABLED="1"`）
- 构建后端：`go build -o one-api .`（发布时 `-ldflags "-X 'github.com/songquanpeng/one-api/common.Version=$(cat VERSION)'"`，见 Dockerfile）
- 构建前端：`cd web && sh build.sh`（逐主题 `npm install` + `npm run build`，产物进 `web/build/<theme>`，前端改动必须重新构建才会被 `//go:embed web/build/*` 收录）
- 前端单独开发：在 `web/default` 等主题目录 `npm start`；提交主题需同步 `common/config/config.go` 的 `ValidThemes` 与 `web/THEMES`

## Architecture

- `router/` — 路由注册：`api.go`（管理 API）、`relay.go`（OpenAI 兼容转发入口）、`web.go`（静态页面）、`dashboard.go`。
- `controller/` — HTTP handler：`user/token/channel/option/redemption/log/billing` + `auth/`；`relay.go`、`channel-test.go` 等。
- `model/` — GORM 模型与数据访问（`user.go`、`channel.go`、`token.go`、`log.go`、`option.go`、`ability.go`、`cache.go`）。
- `relay/` — 核心转发层：`controller/`（text/audio/image/anthropic 等入站处理）、`adaptor/`（30+ 家模型适配器，新增渠道在此加目录）、`channeltype/`、`relaymode/`、`apitype/`、`meta/`、`billing/`。
- `middleware/` — gin 中间件：RequestId、Language、日志、认证。
- `common/` — 通用：`config/`、`logger/`、`i18n/`、`redis.go`、`crypto.go`、`helper/`、`image/`、`network/`、`rate-limit.go`。
- `web/` — React 前端（多主题），构建产物由 `main.go` 的 `//go:embed` 嵌入。

## Conventions

- 日志统一走 `common/logger`（`SysLogf`/`FatalLog`），不要直接 `fmt.Println` 打日志。
- 后端错误消息走 i18n（`common/i18n`），按请求语言翻译；API 响应格式遵循 OpenAI 兼容协议。
- 渠道/模型新增：在 `relay/adaptor/` 加适配器目录并注册 `channeltype`，参照现有适配器（如 `openai/`、`anthropic/`）实现 `relay/adaptor/interface.go` 的接口。
- 请求转发优先直接透传 body，模型映射/重写会重构请求体，可能丢失未支持字段（README 有明确警告）。
- 前端构建需 `DISABLE_ESLINT_PLUGIN='true'`（build.sh 已内置）；前端提交请保持主题一致。
- 数据库模型变更要同时处理自动迁移（`model.InitDB` 相关逻辑）。

## Notes

- 本地工作区存在 `one-api.exe` 与 `test-logs/`，为运行/调试残留，勿提交。
- 管理 API 可通过系统访问令牌调用，见 `docs/API.md`。
