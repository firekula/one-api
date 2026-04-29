# 开发环境搭建指南

本文档帮助你在本地搭建 One API 的开发环境。

---

## 1. 前置依赖

| 软件 | 最低版本 | 说明 |
|------|----------|------|
| Go | 1.20+ | 后端编译与运行 |
| Node.js | 18+ | 前端编译与运行 |
| npm | 随 Node.js 自带 | 前端包管理 |
| Git | - | 版本控制 |
| MySQL | 8.0+（可选） | 需要测试数据库功能时 |
| Redis | 7+（可选） | 需要测试缓存功能时 |

> 默认使用 SQLite，无需安装 MySQL。如需使用 MySQL，请参考 `.env` 中的 `SQL_DSN` 配置。
> Redis 为可选依赖，项目中 Redis 连接失败时会自动禁用缓存相关功能。

---

## 2. 克隆项目

```bash
git clone https://github.com/songquanpeng/one-api.git
cd one-api
```

---

## 3. 后端环境

### 3.1 配置环境变量

```bash
cp .env.example .env
```

默认使用 SQLite，无需额外配置。如需切换为 MySQL，编辑 `.env`：

```bash
SQL_DSN="root:123456@tcp(localhost:3306)/oneapi"
```

### 3.2 下载依赖

```bash
go mod download
```

### 3.3 启动后端

```bash
go run main.go --port 3000 --log-dir ./logs
```

启动后访问 http://localhost:3000 即可看到界面。

> `--port` 默认值为 3000，可通过环境变量 `PORT` 设置。
> `--log-dir` 指定日志输出目录，不传则日志输出到标准输出。

---

## 4. 前端环境

### 4.1 安装依赖并启动

```bash
cd web/default
npm install
npm start
```

默认在 http://localhost:3000 启动开发服务器（由 `package.json` 中的 `proxy` 字段转发 API 请求到 Go 后端）。

### 4.2 前后端联调

`web/default/package.json` 中已配置代理（无需手动添加）：

```json
"proxy": "http://localhost:3000"
```

该配置让 React 开发服务器将 `/api` 等请求转发到 Go 后端（端口 3000），实现前后端分离开发。

---

## 5. 调试开关

| 环境变量 | 效果 |
|----------|------|
| `GIN_MODE=debug` | Gin 框架调试模式，输出更详细的错误信息 |
| `DEBUG=true` | 在 Relay 日志中打印请求体内容 |
| `DEBUG_SQL=true` | 在控制台打印所有 SQL 查询语句 |

设置方式（二选一）：

1. 在 `.env` 文件中添加：
   ```
   GIN_MODE=debug
   DEBUG=true
   DEBUG_SQL=true
   ```

2. 启动时临时设置：
   ```bash
   DEBUG=true GIN_MODE=debug go run main.go
   ```

---

## 6. 运行测试

```bash
# 运行全部测试
go test ./...

# 运行指定包测试
go test ./relay/adaptor/aws/...

# 输出详细日志
go test -v ./...

# 运行指定测试函数
go test -v -run TestChannelTest ./controller/...
```

---

## 7. 构建生产版本

### 7.1 构建前端

```bash
cd web/default
npm run build
```

构建产物输出到 `web/build/default/` 目录。

### 7.2 构建后端

```bash
cd ../..
go build -ldflags "-s -w" -o one-api
```

- `-ldflags "-s -w"` 用于去除调试符号，减小二进制体积。
- 输出可执行文件 `one-api`（Windows 下为 `one-api.exe`）。

### 7.3 运行生产版本

```bash
./one-api --port 3000 --log-dir ./logs
```

---

## 8. 项目结构速览

```
one-api/
├── main.go                        # 后端入口，启动流程
├── router/
│   ├── api.go                     # API 路由定义
│   └── relay.go                   # Relay 路由定义
├── controller/                    # 控制器（请求处理逻辑）
├── model/                         # 数据模型与数据库操作
├── middleware/                    # 中间件（鉴权、日志等）
├── common/                        # 公共工具与配置
│   ├── config/                    # 运行时配置
│   └── logger/                    # 日志模块
├── relay/
│   └── adaptor/
│       └── <provider>/            # 各渠道适配器（如 openai, aws 等）
├── web/
│   └── default/
│       ├── package.json           # 前端依赖与脚本
│       └── src/
│           ├── pages/             # 前端页面
│           └── components/        # 前端组件
└── .env.example                   # 环境变量模板
```

### 关键文件说明

| 路径 | 说明 |
|------|------|
| `main.go` | 后端入口，完成初始化配置、数据库连接、Redis 初始化、路由注册和 HTTP 服务启动 |
| `router/api.go` | 业务 API 路由（令牌、渠道、用户、日志等 CRUD 接口） |
| `router/relay.go` | AI 模型转发路由（接收上游请求并转发至对应渠道） |
| `controller/` | 控制器层，处理 HTTP 请求并调用 model 层 |
| `model/` | 数据模型，封装 GORM 数据库操作 |
| `middleware/` | Gin 中间件（`RequestId`、`Language`、日志记录等） |
| `relay/adaptor/<provider>/` | 各 AI 厂商的渠道适配器实现，每个目录对应一种 API 协议适配 |
| `web/default/src/pages/` | React 页面组件 |
| `web/default/src/components/` | React 通用 UI 组件 |
