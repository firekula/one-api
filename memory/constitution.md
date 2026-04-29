# One API 技术宪章

本文件定义 One API 项目的权威技术约束。所有 spec 技能、代码审查、AI 辅助开发均以此文件为最高准则。

---

## 语言红线

**绝对强制**：所有 AI 输出、文档内容、代码注释（Go doc comments / JSX 行内注释）、Git 提交信息（含 `feat`/`fix` 标头）均必须且只能使用 **简体中文 (Simplified Chinese)**。

## 技术栈

| 层级 | 技术 | 版本 |
|------|------|------|
| 语言 | Go | 1.20+ |
| Web 框架 | Gin | 1.10 |
| ORM | GORM | 1.25 |
| 数据库 | SQLite / MySQL / PostgreSQL | — |
| 缓存 | Redis (可选) | — |
| 前端 | React | — |

## 架构原则

1. **Adaptor 接口统一**：所有渠道适配器必须实现 `relay/adaptor/interface.go` 中定义的 `Adaptor` 接口（9 个方法）。渠道差异封装在接口实现内部，业务逻辑层无需感知具体渠道
2. **中间件链顺序**：TokenAuth → Distributor → RelayHandler。鉴权先于分发，分发先于处理
3. **复用优先**：新增功能优先复用现有 adaptor、middleware、controller、model 模式。禁止未经授权引入重型第三方依赖
4. **Master/Slave 分离**：Master 负责数据库迁移和配置管理，Slave 仅处理 API 中继

## 目录职责

| 目录 | 职责 |
|------|------|
| `main.go` | 入口，组装中间件、注册路由、启动服务 |
| `common/` | 共享基础设施：config、logger、database、加密、验证码、i18n、网络工具 |
| `model/` | 数据模型定义 + 数据库 CRUD 操作 |
| `controller/` | HTTP 请求处理函数（业务逻辑层） |
| `middleware/` | Gin 中间件：鉴权、CORS、分发、限流、日志、恢复 |
| `router/` | 路由注册：API路由、Dashboard路由、中继路由、Web路由 |
| `relay/` | 请求中继核心：adaptor 接口 + 渠道实现 + 请求/响应模型 |
| `web/` | React 前端 |

## 编译铁律

1. 修改 Go 源码后，必须执行 `go build ./...`，编译失败 → 修复 → 重试
2. 修改前端源码后，必须执行 `cd web/default && npm run build`，编译失败 → 修复 → 重试
3. 双端都修改了则两者均必须通过
4. 最多重试 3 次，超过则输出完整错误摘要并请求用户介入

## 测试铁律

1. 修改 Go 核心逻辑（model/controller/relay/middleware）后，必须执行 `go test ./...`
2. 测试失败 → 根据失败信息修复 → 重新测试
3. 新增功能必须同时新增对应测试用例

## Git 规范

- **分支命名**：`feat/<功能>` / `fix/<问题>` / `docs/<内容>` / `refactor/<范围>` / `chore/<杂项>`
- **Commit 格式**：`<type>: <中文简短描述>`，首行不超 72 字符
- **PR Checklist**：
  - [ ] 编译通过（`go build ./...` + `npm run build`）
  - [ ] 测试通过（`go test ./...`）
  - [ ] 相关文档更新
  - [ ] PR 描述含变更内容 + 测试方式

## Go 代码风格

- `gofmt` 格式化（`gofmt -w .`）
- 驼峰命名，包名全小写无下划线
- 错误处理：返回 error，不使用 panic（除非不可恢复）
- 保持与现有代码一致的风格：相同功能的变量命名、相同的文件组织方式
- 导出函数/类型/接口必须写 Go doc comment

## React 代码风格

- 函数组件 + Hooks（禁止 class 组件）
- React Context 做状态管理（参照 `web/default/src/context/`）
- API 调用封装在 `web/default/src/helpers/api.js`
- 常量定义在 `web/default/src/constants/`
- 遵循项目已有 ESLint 配置

## 产物路径约定

```
specs/[feature_name]/spec.md          # 功能规范（由 /spec:define 生成）
specs/[feature_name]/plan.md          # 实施计划（由 /spec:architect 生成）
specs/[feature_name]/verify_report.md # 验证报告（由 /spec:verify 生成）
```

## 文件路径格式

- `file:///` 可点击链接格式：`[文件名:L行号](file:///绝对路径#L行号)`
- 绝对路径从工具返回值中获取，反斜杠替换为正斜杠
- 示例：`[relay.go:L45](file:///Y:/Project/Server/one-api/controller/relay.go#L45)`
