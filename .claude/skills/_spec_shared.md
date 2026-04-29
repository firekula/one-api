# Spec 工作流共享标准

本文件定义 `/spec:*` 命令族的公共约束，被各 skill 引用。权威技术细节参见 `memory/constitution.md`。

---

## 语言红线

**绝对强制**：所有 AI 输出、文档内容、代码注释（Go doc comments / JSX 行内注释）、Git 提交信息（含 `feat`/`fix` 标头）均必须且只能使用 **简体中文 (Simplified Chinese)**。

## 技术栈约束（源自 `memory/constitution.md`）

- **后端**: Go 1.20+ / Gin 1.10 / GORM 1.25 / MySQL-SQLite-PostgreSQL
- **前端**: React 函数组件 + Hooks / React Context 状态管理
- **缓存**: Redis (可选)
- **架构原则**:
  - 渠道适配器统一实现 `relay/adaptor/interface.go` 中的 `Adaptor` 接口（9 个方法）
  - 中间件链：TokenAuth → Distributor → RelayHandler
  - 优先复用现有 adaptor、middleware、controller 模式，禁止未经授权引入重型第三方依赖
- **环境隔离**:
  - `common/` — 共享基础设施
  - `model/` — 数据模型 + DB 操作
  - `controller/` — 业务逻辑
  - `middleware/` — 请求拦截
  - `relay/adaptor/` — 渠道适配

## 产物路径约定

```
specs/[feature_name]/spec.md          # 功能规范（由 /spec:define 生成）
specs/[feature_name]/plan.md          # 实施计划（由 /spec:architect 生成）
specs/[feature_name]/verify_report.md # 验证报告（由 /spec:verify 生成）
```

## 编译铁律

1. 修改 Go 源码后，必须执行 `go build ./...`，编译失败 → 修复 → 重试
2. 修改前端源码后，必须执行 `cd web/default && npm run build`，编译失败 → 修复 → 重试
3. 双端都修改了则两者均必须通过
4. 最多重试 3 次，超过则输出完整错误摘要并请求用户介入

## 测试铁律

1. 修改 Go 核心逻辑（model/controller/relay/middleware）后，必须执行 `go test ./...`
2. 测试失败 → 根据失败信息修复 → 重新测试
3. 新增功能必须同时新增对应测试用例

## 文件路径格式

- `file:///` 可点击链接格式：`[文件名:L行号](file:///绝对路径#L行号)`
- 绝对路径从工具返回值中获取，反斜杠替换为正斜杠
- 示例：`[relay.go:L45](file:///Y:/Project/Server/one-api/controller/relay.go#L45)`

## Git 规范

- **分支命名**: `feat/<功能>` / `fix/<问题>` / `docs/<内容>` / `refactor/<范围>` / `chore/<杂项>`
- **Commit 格式**: `<type>: <中文简短描述>`，首行不超 72 字符
- **PR Checklist**:
  - 编译通过（`go build ./...` + `npm run build`）
  - 测试通过（`go test ./...`）
  - 相关文档更新
  - PR 描述含变更内容 + 测试方式

## Go 代码风格

- `gofmt` 格式化
- 驼峰命名，包名全小写无下划线
- 错误处理：返回 error，不使用 panic（除非不可恢复）
- 保持与现有代码一致的风格

## React 代码风格

- 函数组件 + Hooks（禁止 class 组件）
- React Context 做状态管理（参照 `src/context/`）
- API 调用封装在 `src/helpers/api.js`
- 常量定义在 `src/constants/`
