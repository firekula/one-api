# One API Spec 技能体系初始化 — 设计规格

**版本**: v1.0  
**日期**: 2026-04-29  
**状态**: 待实施  
**参考**: `C:/Users/Firekula/.CocosCreator/packages/mcp-inspector-bridge/.claude/skills/`

## 1. 背景与目标

One API 项目目前缺少一套结构化的 spec 工作流体系。参考 mcp-inspector-bridge 项目成熟的 5 阶段 spec 管线（define → architect → execute → refactor → verify），为 One API 适配并初始化一套等效的 spec 技能系统。

**目标**：建立三层架构的 spec 工作流体系——宪章定义约束、workflow 模板给 Agent 派发使用、skill 文件给 CLI 用户调用。

## 2. 整体架构

### 2.1 文件结构

```
项目根目录/
├── memory/
│   └── constitution.md              ← 技术宪章（权威约束来源）
├── .agent/
│   └── workflows/
│       ├── define.md                ← Agent 派发模板：深度调研 → spec.md
│       ├── architect.md             ← Agent 派发模板：spec → plan.md
│       ├── execute.md               ← Agent 派发模板：逐步执行
│       ├── refactor.md              ← Agent 派发模板：代码审计
│       └── verify.md                ← Agent 派发模板：验证报告
└── .claude/
    └── skills/
        ├── _spec_shared.md          ← 共享约束（引用 constitution.md）
        ├── spec-define/
        │   └── SKILL.md             ← CLI 入口：/spec:define
        ├── spec-architect/
        │   └── SKILL.md             ← CLI 入口：/spec:architect
        ├── spec-execute/
        │   └── SKILL.md             ← CLI 入口：/spec:execute
        ├── spec-refactor/
        │   └── SKILL.md             ← CLI 入口：/spec:refactor
        └── spec-verify/
            └── SKILL.md             ← CLI 入口：/spec:verify
```

共 **12 个文件**。

### 2.2 三层职责

| 层 | 位置 | 定位 | 读者 |
|----|------|------|------|
| 宪章 | `memory/constitution.md` | 权威技术约束源，定义"什么是对的" | 人类 + AI |
| 工作流模板 | `.agent/workflows/*.md` | Agent 派发时注入的简洁 prompt，聚焦"怎么做" | Agent（子进程） |
| 技能文件 | `.claude/skills/spec-*/SKILL.md` | CLI 用户调用入口，含完整定量红线和交付自检表 | 用户 + AI |

### 2.3 数据流

```
constitution.md → _spec_shared.md 引用 → 5 个 SKILL.md 引用 _spec_shared.md
                                           ↓
                                    .agent/workflows/*.md（独立使用，内容精简）
```

### 2.4 工作流管线

```
refactor(可选) → define → architect → execute → verify
```

| 阶段 | 命令 | 输入 | 产物 |
|------|------|------|------|
| 审计 | `/spec:refactor` | 目标目录 | 审计清单 |
| 定义 | `/spec:define` | 需求描述 | `specs/[feature]/spec.md` |
| 计划 | `/spec:architect` | spec.md | `specs/[feature]/plan.md` |
| 执行 | `/spec:execute` | plan.md | 代码改动 |
| 验证 | `/spec:verify` | spec.md + 代码 | `specs/[feature]/verify_report.md` |

### 2.5 与参考项目的主要适配差异

| 维度 | 参考项目 (Cocos Creator) | One API |
|------|--------------------------|---------|
| 编译验证 | `npm run build` | `go build ./...` + `npm run build` |
| 语言 | TypeScript | Go + JavaScript (React) |
| 调研方式 | grep TS 源码 | grep Go 源码 + JSX 源码 |
| 层级标签 | `[Frontend]` `[Backend]` | `[Go]` `[React]` `[Build]` `[Test]` `[Docs]` |
| 前端框架 | Vue.js 3 + HTML 拼接 | React 函数组件 + Hooks |
| 视觉审计 | Cocos Creator 编辑器截图 | 浏览器 Web UI 截图 + API JSON |
| 测试 | 手动编辑器验证 | `go test ./...` 自动化测试 |
| 代码规范 | ESLint + TypeScript | gofmt + ESLint |
| 产物路径 | `specs/[feature_name]/` | `specs/[feature_name]/`（保持一致） |

## 3. 各文件内容规格

### 3.1 `memory/constitution.md` — 技术宪章

项目权威技术约束文档。定义：

- **技术栈**：Go 1.20+ / Gin 1.10 / GORM 1.25 / MySQL-SQLite-PostgreSQL / Redis / React
- **架构原则**：
  - 渠道适配器统一实现 `relay/adaptor/interface.go` 中的 `Adaptor` 接口（9 个方法）
  - 中间件链：TokenAuth → Distributor → RelayHandler
  - 优先复用现有 adaptor、middleware、controller 模式
  - 禁止未经授权引入重型第三方依赖
- **目录职责**：`common/` 基础设施 / `model/` 数据层 / `controller/` 业务逻辑 / `middleware/` 请求拦截 / `relay/adaptor/` 渠道适配
- **语言红线**：所有 AI 输出、文档、代码注释、Git 提交信息必须使用简体中文
- **编译铁律**：改 Go 后 `go build ./...`，改 React 后 `cd web/default && npm run build`，双端修改两者均过
- **测试铁律**：修改核心逻辑后 `go test ./...`
- **Git 规范**：分支命名 `feat|fix|docs|refactor|chore/<描述>`，commit `<type>: <中文描述>`
- **Go 代码风格**：gofmt、驼峰命名、error 返回（非 panic）
- **React 代码风格**：函数组件 + Hooks、React Context、API 调用封装
- **产物路径**：`specs/[feature_name]/spec.md | plan.md | verify_report.md`
- **文件路径格式**：`[文件名:L行号](file:///绝对路径#L行号)`

### 3.2 `.claude/skills/_spec_shared.md` — 共享约束

从 `constitution.md` 提取关键约束，被 5 个 skill 文件引用。内容：

- 语言红线（引用 constitution.md）
- 技术栈摘要（Go/Gin/GORM/React/SQLite-MySQL-PostgreSQL）
- 编译铁律（Go + React 双端）
- 测试铁律
- 产物路径约定
- 文件路径格式规范（`file:///` 链接）
- Go 代码风格要点
- React 代码风格要点
- Git 规范

### 3.3 `.claude/skills/spec-define/SKILL.md` — 功能规范定义

**触发词**: `/spec:define`, `写spec`, `定义规范`, `功能说明`

**产物**: `specs/[feature_name]/spec.md`

**交付标准（6 项硬指标）**：

| # | 自检项 | 合格标准 |
|---|--------|----------|
| 1 | `file:///` 可点击源码链接 | ≥ 3 处 |
| 2 | 架构/流程 Mermaid 图 | ≥ 1 张 |
| 3 | 代码片段 | ≥ 2 处 |
| 4 | 边界情况 | ≥ 4 条（场景 + 风险 + 缓解策略） |
| 5 | 现有机制复用说明 | 明确列出复用的接口/函数/组件 |
| 6 | 文件清单表格 | 含改动类型、所属层 |

**执行流程**：
1. 上下文加载 — 阅读 `docs/architecture/` 下关键文档
2. 深度代码调研 — 至少 5 次代码搜索/文件读取
3. 起草规范 — 单一 spec.md 文件
4. 文档结构 — 背景 → 功能需求（根因+方案+Mermaid+复用清单）→ 文件清单 → 边界情况
5. 出厂自检 — 验证 6 项指标
6. 输出结果 — 摘要 + 自检表

### 3.4 `.claude/skills/spec-architect/SKILL.md` — 实施计划生成

**触发词**: `/spec:architect`, `写plan`, `实施计划`, `架构计划`

**产物**: `specs/[feature_name]/plan.md`

**前置**: spec.md 必须存在

**交付标准（7 项）**：

| # | 检查项 | 合格标准 |
|---|--------|----------|
| 1 | 文件清单表格 | 含所属层（Go/React/Config） |
| 2 | 分步实施复选框 | `- [ ]` 格式 |
| 3 | 阶段分组 | A:代码修改 B:编译验证 C:测试 D:文档更新 |
| 4 | 层级标签 | `[Go]` `[React]` `[Build]` `[Test]` `[Docs]` |
| 5 | 编译验证步骤 | `go build ./...` 和/或 `npm run build` |
| 6 | 测试步骤 | `go test ./...` |
| 7 | 代码片段 | 关键步骤附改动前后对比 |

### 3.5 `.claude/skills/spec-execute/SKILL.md` — 计划执行

**触发词**: `/spec:execute`, `执行计划`, `实施`, `开始改代码`

**前置**: plan.md 必须存在

**执行铁律（6 条）**：
1. 逐步执行，不跳步
2. 编译必过（Go + React 双端）
3. 测试必过（`go test ./...`）
4. 状态同步（每步标记 `- [x]`）
5. 失败即停（最多 3 次重试，超限请求介入）
6. 禁止偏离（不执行 plan 外改动）

**编译时**：Go 改 → `go build ./...`、React 改 → `npm run build`、双端改 → 两者均执行

**完成汇报**：阶段 × 步骤 × 状态表格

### 3.6 `.claude/skills/spec-refactor/SKILL.md` — 代码重构审计

**触发词**: `/spec:refactor`, `重构`, `清理代码`, `代码审计`

**重构铁律（4 条）**：
1. 零功能变更
2. 编译必过
3. 测试必过
4. 问题先列后改（用户确认后才动代码）

**审计检查项**：
- Go：死代码、缺失 doc comment、命名不规范、error 处理不当、goroutine 泄漏风险、数据竞争、魔法数字
- React：未使用 import/组件、class 组件、直接 DOM 操作、重复代码

**严重度分类**：🔴高（违反硬性规则）/ 🟡中（代码质量问题）/ 🟢低（风格建议）

### 3.7 `.claude/skills/spec-verify/SKILL.md` — 功能验证

**触发词**: `/spec:verify`, `验证`, `检查`, `验收`

**产物**: `specs/[feature_name]/verify_report.md`

**前置**: spec.md 存在 + 代码修改已完成

**验证标准（6 项）**：
1. 环境确认（`go build` + `go test` 通过）
2. 功能审计（逐条对比 spec.md 功能需求）
3. 边界测试（逐条对比 spec.md 边界情况）
4. API 验证（端点状态码 + 响应结构）
5. 回归检查（渠道中继 / 令牌鉴权 / 额度扣减 / Web UI / 管理 API）
6. 最终结论（通过 / 不通过）

### 3.8 `.agent/workflows/*.md` — Agent 派发模板

每个 workflow 文件是对应 skill 的简洁版，供 Agent 派发时注入。内容从 SKILL.md 精简而来，保留核心执行指令和交付标准，省略详细的背景说明和示例。

| workflow | 对应 skill | 精简后关键内容 |
|----------|-----------|---------------|
| `define.md` | spec:define | 6 项指标 + 执行 6 步 + 文档结构 |
| `architect.md` | spec:architect | 7 项检查 + 执行 5 步 + 4 阶段模板 |
| `execute.md` | spec:execute | 6 条铁律 + 执行 6 步 + 汇报格式 |
| `refactor.md` | spec:refactor | 4 条铁律 + 审计检查项 + 严重度分类 |
| `verify.md` | spec:verify | 6 项验证标准 + 验证 7 步 + 报告模板 |

## 4. 技术约束

- **语言**：全部简体中文（技能文件、workflow 模板、constitution 均中文）
- **格式**：Markdown，兼容 GitHub Flavored Markdown
- **路径**：
  - 技能文件：`.claude/skills/spec-*/SKILL.md`（Claude Code 标准技能路径）
  - Workflow 模板：`.agent/workflows/*.md`
  - 宪章：`memory/constitution.md`
  - Spec 产物：`specs/[feature_name]/`
- **不涉及代码变更**：本次仅创建配置文件和文档，不修改项目源代码

## 5. 非目标（本次不做）

- 不创建示例 spec/plan/verify_report（等第一个实际 feature 时产生）
- 不修改项目 `.claude/settings.json`（技能由 Claude Code 自动发现）
- 不创建多语言版本

## 6. 与现有项目文件的协调

- 不修改已有 `docs/` 下的任何文档
- 不修改项目根 `README.md`
- 新增的 `memory/constitution.md` 与已有 `docs/architecture/overview.md` 互补：constitution 定义规范约束，overview 描述架构事实
