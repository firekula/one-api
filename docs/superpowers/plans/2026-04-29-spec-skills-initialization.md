# One API Spec 技能体系初始化 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 One API 项目建立三层架构的 spec 工作流体系（宪章 + 5 技能 + 5 workflow 模板），共 12 个文件。

**Architecture:** 三层结构——`memory/constitution.md` 定义权威技术约束，`.claude/skills/spec-*/SKILL.md` 提供 CLI 用户入口（含完整定量红线），`.agent/workflows/*.md` 提供 Agent 派发模板（精简版）。所有文件简体中文 Markdown。

**Tech Stack:** Markdown (GFM)，纯文本编辑，无需编译

**Source references:**
- `docs/superpowers/specs/2026-04-29-spec-skills-initialization-design.md` — 设计规格
- `docs/architecture/overview.md` — 系统架构（constitution 参考）
- `docs/architecture/data-model.md` — 数据模型（constitution 参考）
- `docs/architecture/relay-system.md` — 中继系统（constitution 参考）
- `docs/development/contribution-guide.md` — 贡献规范（constitution 参考 Git 规范部分）
- `C:/Users/Firekula/.CocosCreator/packages/mcp-inspector-bridge/.claude/skills/_spec_shared.md` — 参考共享约束
- `C:/Users/Firekula/.CocosCreator/packages/mcp-inspector-bridge/.claude/skills/spec-define/SKILL.md` — 参考 define 技能
- `C:/Users/Firekula/.CocosCreator/packages/mcp-inspector-bridge/.claude/skills/spec-architect/SKILL.md` — 参考 architect 技能
- `C:/Users/Firekula/.CocosCreator/packages/mcp-inspector-bridge/.claude/skills/spec-execute/SKILL.md` — 参考 execute 技能
- `C:/Users/Firekula/.CocosCreator/packages/mcp-inspector-bridge/.claude/skills/spec-refactor/SKILL.md` — 参考 refactor 技能
- `C:/Users/Firekula/.CocosCreator/packages/mcp-inspector-bridge/.claude/skills/spec-verify/SKILL.md` — 参考 verify 技能

---

### Task 1: 创建目录结构

**Files:**
- Create: `memory/` (目录)
- Create: `.agent/workflows/` (目录)
- Create: `.claude/skills/spec-define/` (目录)
- Create: `.claude/skills/spec-architect/` (目录)
- Create: `.claude/skills/spec-execute/` (目录)
- Create: `.claude/skills/spec-refactor/` (目录)
- Create: `.claude/skills/spec-verify/` (目录)

- [ ] **Step 1: 创建所有目录**

```bash
mkdir -p memory \
  .agent/workflows \
  .claude/skills/spec-define \
  .claude/skills/spec-architect \
  .claude/skills/spec-execute \
  .claude/skills/spec-refactor \
  .claude/skills/spec-verify
```

- [ ] **Step 2: 验证目录结构**

```bash
ls -d memory .agent/workflows .claude/skills/spec-*/ 2>/dev/null
```

Expected: 7 个目录均存在。

- [ ] **Step 3: 提交**

```bash
git add memory/ .agent/ .claude/
git commit -m "chore: create spec skills directory structure"
```

---

### Task 2: 编写 `memory/constitution.md` — 技术宪章

**Files:**
- Create: `memory/constitution.md`
- Reference: `docs/architecture/overview.md`, `docs/architecture/data-model.md`, `docs/architecture/relay-system.md`, `docs/development/contribution-guide.md`

- [ ] **Step 1: 阅读现有架构文档确认技术细节**

读取 `docs/architecture/overview.md` 的技术栈和目录结构部分，确保 constitution 中的内容与已有文档一致。

- [ ] **Step 2: 编写 constitution.md**

写入以下完整内容：

```markdown
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
```

- [ ] **Step 3: 自检**

确认技术栈版本号与 `go.mod` 一致，目录职责描述与 `docs/architecture/overview.md` 一致。

- [ ] **Step 4: 提交**

```bash
git add memory/constitution.md
git commit -m "docs: add project constitution defining technical constraints"
```

---

### Task 3: 编写 `.claude/skills/_spec_shared.md` — 共享约束

**Files:**
- Create: `.claude/skills/_spec_shared.md`
- Reference: `memory/constitution.md`

- [ ] **Step 1: 编写 _spec_shared.md**

从 constitution.md 提取关键内容，写入以下完整内容：

```markdown
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
```

- [ ] **Step 2: 提交**

```bash
git add .claude/skills/_spec_shared.md
git commit -m "docs: add spec workflow shared constraints"
```

---

### Task 4: 编写 `.claude/skills/spec-define/SKILL.md` — 功能规范定义

**Files:**
- Create: `.claude/skills/spec-define/SKILL.md`

- [ ] **Step 1: 编写 SKILL.md**

写入以下完整内容：

````markdown
---
name: spec:define
description: 将需求/想法转化为严谨的功能规范文档 - 触发: /spec:define, 写spec, 定义规范, 功能说明
---

# Spec 定义工作流 (Define)

将需求转化为严谨的功能规范 `spec.md`，含根因分析、功能需求、文件清单和边界情况。

**产物**: `specs/[feature_name]/spec.md`

## ⚠️ 交付标准（写之前必读）

生成的 spec.md 必须同时满足以下 6 项硬指标，**任何一项不通过禁止提交**：

| # | 自检项 | 合格标准 |
|---|--------|----------|
| 1 | `file:///` 可点击源码链接 | ≥ 3 处。格式：`[文件名:L行号](file:///绝对路径#L行号)`。从工具返回值复制绝对路径，反斜杠换正斜杠。纯文字行号不合格 |
| 2 | 架构/流程 Mermaid 图 | 功能需求章节含 ≥ 1 张 Mermaid 图（时序图/流程图/ER图），禁止仅用文字描述交互流程 |
| 3 | 代码片段 | ≥ 2 处 fenced code block（根因原始代码 + 修复方案代码） |
| 4 | 边界情况 | ≥ 4 条，每条含：场景 + 风险 + 缓解策略 |
| 5 | 现有机制复用说明 | 明确列出复用了哪些已有接口/函数/组件，以及为什么不需新增逻辑（如适用） |
| 6 | 文件清单表格 | Markdown 表格，列出所有改动文件、改动类型（新增/修改/删除）、所属层（Go/React/Config） |

## 执行指令

### 1. 上下文加载

- 阅读 `docs/architecture/overview.md` 确认系统架构
- 阅读 `docs/architecture/data-model.md` 确认数据模型
- 阅读 `docs/architecture/relay-system.md` 确认中继流程（如涉及中继）
- 如有用户指定的 `feature_name`，以此为产物目录名；否则从需求描述中提取关键词

### 2. 深度代码调研（禁止跳过）

**定量红线**：调研阶段必须执行代码搜索或文件读取 **至少 5 次**才允许进入撰写阶段。不足 5 次说明调研深度不够。

必须完成：
- **定位受影响代码**：搜索至少 2 个关键词（Go 函数名、struct 名、中间件名、路由路径、React 组件名），读取每个命中文件的完整上下文（前后至少 30 行）
- **追溯现有机制**：找到相关的 Adaptor 接口实现、中间件链、Controller 函数、React Context。必须回答："现有的哪些机制可以复用？哪些不能？"
- **历史修复记录**：搜索 `git log` 确认是否有过类似改动或回归风险
- **记录绝对路径**：将每个相关文件的绝对路径（从工具返回值中获取）记录下来，后续用于构建 `file:///` 链接
- **严禁**仅凭用户描述开始撰写，必须用代码事实支撑每一个结论

### 3. 起草规范

- 仅创建/更新**一个文件**: `specs/[feature_name]/spec.md`
- **禁止**创建 checklist 清单文件或单独的分析文件
- **语言要求**: 所有输出及文档内容必须使用**简体中文**

### 4. 文档结构

#### 4.1 背景
- 简述问题/需求的来龙去脉
- 每个技术结论必须附带可点击的 `file:///` 源码链接
- 如有相关历史改动，引用 git log 中的 commit hash 和消息

#### 4.2 功能需求
- **根因分析**：逐条列出每个技术原因，每条附 `file:///` 源码链接
- **具体方案**：按改动点逐个列出，每个点必须包含目标文件与行号（`file:///` 链接）和改动前后代码片段
- **Mermaid 图**：涉及多组件交互或状态流转的，用时序图或流程图展示改动前后对比
- **现有机制复用清单**：明确列出"复用了哪些已有接口/函数/组件"及理由

#### 4.3 涉及文件清单
- Markdown 表格列出所有需改动文件（Go / React / 配置文件）
- 标注改动类型（新增/修改/删除）和所属层

#### 4.4 边界情况
- **至少 4 条**边界场景
- 每条必须包含：场景描述 + 风险分析 + 缓解策略
- 典型必查：并发安全（data race）、空值/nil 处理、数据库兼容（SQLite vs MySQL vs PostgreSQL）、API 向后兼容性、前端加载与错误状态、goroutine 泄漏

### 5. 出厂自检（强制执行）

创建 spec.md 后，**必须立即验证**：
- `file:///` 链接数是否 ≥ 3
- 边界情况条目数是否 ≥ 4
- 代码片段数是否 ≥ 2
- Mermaid 图是否 ≥ 1
- 文件清单表格是否包含所有涉及文件

验证不通过则**立即编辑 spec.md 补充**，直到全部达标。

### 6. 输出结果

向用户展示创建的 spec.md 摘要，并在回复末尾附带以下自检表：

```
| 自检项 | 要求 | 实际 | 达标 |
|--------|------|------|------|
| file:/// 链接数 | ≥ 3 | ? | ✅/❌ |
| Mermaid 图 | ≥ 1 | ? | ✅/❌ |
| 代码片段数 | ≥ 2 | ? | ✅/❌ |
| 边界情况条目 | ≥ 4 | ? | ✅/❌ |
| 机制复用说明 | 有 | 有/无 | ✅/❌ |
| 文件清单表格 | 有 | 有/无 | ✅/❌ |
```

完成后提示用户：下一步运行 `/spec:architect` 生成实施计划。
````

- [ ] **Step 2: 提交**

```bash
git add .claude/skills/spec-define/SKILL.md
git commit -m "feat: add spec:define skill for feature specification creation"
```

---

### Task 5: 编写 `.claude/skills/spec-architect/SKILL.md` — 实施计划生成

**Files:**
- Create: `.claude/skills/spec-architect/SKILL.md`

- [ ] **Step 1: 编写 SKILL.md**

写入以下完整内容：

````markdown
---
name: spec:architect
description: 将spec转化为可执行的实施计划 - 触发: /spec:architect, 写plan, 实施计划, 架构计划
---

# 实施计划工作流 (Architect)

将 `spec.md` 转化为可逐步执行的实施计划 `plan.md`，包含文件清单、分步复选框、阶段分组和代码片段。

**产物**: `specs/[feature_name]/plan.md`
**前置**: 必须先有 `specs/[feature_name]/spec.md`（由 `/spec:define` 生成）

## ⚠️ 交付标准（写之前必读）

生成的 plan.md 必须同时满足以下指标，**任何一项不通过禁止提交**：

| # | 检查项 | 合格标准 |
|---|--------|----------|
| 1 | 文件清单表格 | 包含所有涉及文件、所属层（Go/React/Config）、改动性质（新增/修改/删除）的 Markdown 表格 |
| 2 | 分步实施复选框 | 每个步骤都使用 `- [ ]` 复选框格式 |
| 3 | 阶段分组 | 步骤按阶段分组（A: 代码修改 → B: 编译验证 → C: 测试验证 → D: 文档更新） |
| 4 | 层级标签 | 每个步骤标注 `[Go]` / `[React]` / `[Build]` / `[Test]` / `[Docs]` |
| 5 | 编译验证步骤 | Go 变更必须含 `go build ./...` 步骤，React 变更必须含 `npm run build` 步骤 |
| 6 | 测试步骤 | Go 变更必须含 `go test ./...` 步骤 |
| 7 | 代码片段 | 关键改动步骤必须附带代码片段（改动前 → 改动后），不能只写"修改 xxx" |

## 执行指令

### 1. 阅读规范

- 加载 `specs/[feature_name]/spec.md`
- 逐章阅读，提取：根因分析结论、修复方案细节、涉及文件清单、边界情况
- 如 spec.md 不存在，提示用户先运行 `/spec:define`

### 2. 起草计划

- 仅创建/更新**一个文件**: `specs/[feature_name]/plan.md`
- **语言要求**: 所有输出及文件内容必须使用**简体中文**
- **严禁**创建 `tasks.md`，所有任务保留在 `plan.md` 中

### 3. 文档结构

#### 3.1 架构设计

- **文件清单表格**：列出所有涉及文件、所属层（Go/React/Config）、改动性质、一句话说明
- **架构影响评估**：简述数据模型/状态管理/请求链路的影响。如无架构变更，用 `> [!NOTE]` 声明"本次改动不涉及架构变更"
- **关键流程图**：如涉及多组件交互或状态流转，用 Mermaid 时序图或流程图展示

#### 3.2 分步实施

按阶段分组，推荐模板：

```
### 阶段 A: 代码修改
- [ ] [Go] 步骤描述...
- [ ] [React] 步骤描述...

### 阶段 B: 编译验证
- [ ] [Build] 执行 `go build ./...` 确认后端编译通过
- [ ] [Build] 执行 `cd web/default && npm run build` 确认前端编译通过

### 阶段 C: 测试验证
- [ ] [Test] 执行 `go test ./...` 确认测试通过
- [ ] [Test] 新增测试用例验证新功能

### 阶段 D: 文档更新
- [ ] [Docs] 更新 `docs/` 下相关文档（如适用）
```

- **关键步骤必须附代码片段**：涉及新增/修改代码的步骤，用 fenced code block 给出改动前后对比或最终代码
- **步骤粒度**：每个步骤应为一个可独立完成的原子操作。描述超过 3 行即拆分

### 4. 出厂自检

创建 plan.md 后，在末尾输出以下自检表：

```
| 检查项 | 要求 | 实际 | 达标 |
|--------|------|------|------|
| 文件清单表格 | 有 + 含层级 | 有/无 | ✅/❌ |
| 复选框步骤数 | ≥ 3 | ? | ✅/❌ |
| 阶段分组 | 有 | 有/无 | ✅/❌ |
| 层级标签 | 有 | 有/无 | ✅/❌ |
| 编译验证步骤 | 有 | 有/无 | ✅/❌ |
| 测试步骤 | 有 | 有/无 | ✅/❌ |
| 代码片段 | ≥ 1 | ? | ✅/❌ |
```

### 5. 输出结果

- 向用户展示创建的 plan.md 摘要
- 总结实施步骤数及预计改动范围
- 提示用户：下一步运行 `/spec:execute` 开始逐步执行，或运行 `/spec:refactor` 先做代码审计
````

- [ ] **Step 2: 提交**

```bash
git add .claude/skills/spec-architect/SKILL.md
git commit -m "feat: add spec:architect skill for implementation plan generation"
```

---

### Task 6: 编写 `.claude/skills/spec-execute/SKILL.md` — 计划执行

**Files:**
- Create: `.claude/skills/spec-execute/SKILL.md`

- [ ] **Step 1: 编写 SKILL.md**

写入以下完整内容：

````markdown
---
name: spec:execute
description: 按计划逐步执行代码修改并强制编译验证 - 触发: /spec:execute, 执行计划, 实施, 开始改代码
---

# 计划执行工作流 (Execute)

按照 `plan.md` 中的复选框顺序逐步执行代码修改，强制编译验证和测试验证，完成后同步状态。

**前置**: 必须先有 `specs/[feature_name]/plan.md`（由 `/spec:architect` 生成）

## ⚠️ 执行铁律（开始前必读）

| # | 规则 | 说明 |
|---|------|------|
| 1 | **逐步执行** | 按 plan.md 中复选框的顺序逐个执行，不能跳步 |
| 2 | **编译必过** | Go 修改后执行 `go build ./...`，React 修改后执行 `cd web/default && npm run build`，两者均修改则均须通过 |
| 3 | **测试必过** | 修改核心逻辑后执行 `go test ./...`，测试失败视为执行失败 |
| 4 | **状态同步** | 每完成一个步骤，立即将 plan.md 中对应项标记为 `- [x]` |
| 5 | **失败即停** | 编译失败、测试失败或执行出错，立即修复并重试。最多重试 3 次，超过则输出错误摘要请求用户介入 |
| 6 | **禁止偏离** | 严禁执行 plan.md 中没有列出的改动。发现遗漏应报告用户，不能自行补充 |

## 执行指令

### 1. 阅读计划

- 打开 `specs/[feature_name]/plan.md`
- 定位到包含复选框的 **分步实施** 章节
- 通读所有步骤，理解整体改动范围和依赖关系
- 如 plan.md 不存在，提示用户先运行 `/spec:architect`

### 2. 逐步执行

- 按顺序执行所有未选中项目 `- [ ]`
- **语言要求**: 执行过程中给出的任何解释、日志或反馈必须使用**简体中文**
- **并行优化**: 如果多个步骤之间无依赖关系（如修改不同文件），可同时执行提高效率
- **代码修改规范**：
  - 每次修改后，简要说明改动理由（1-2 句话）
  - 如果 plan.md 中的代码片段与实际文件内容存在冲突（如行号偏移），以实际文件为准进行调整，并报告偏差
  - Go 代码遵循 `gofmt` 格式，React 代码遵循项目 ESLint 配置

### 3. 编译验证（强制，不可跳过）

- **Go 变更**：执行 `go build ./...`
- **React 变更**：执行 `cd web/default && npm run build`
- **双端变更**：两者均执行
- 如果编译失败：
  1. 读取完整的报错信息
  2. 定位到出错文件和行号
  3. 修复错误
  4. 重新执行编译
  5. 重复直到编译通过（最多 3 轮）
- **编译通过后**，将 plan.md 中的编译验证步骤标记为 `- [x]`

### 4. 测试验证（强制）

- 执行 `go test ./...`（仅 Go 变更时需要）
- 如有测试失败，分析失败原因，修复代码或更新测试
- 测试通过后，将 plan.md 中的测试步骤标记为 `- [x]`

### 5. 状态同步

- 所有步骤执行完毕后，确认 plan.md 中所有 `- [ ]` 都已变为 `- [x]`
- 如有步骤未完成，明确标注原因

### 6. 完成汇报

在回复末尾**必须输出以下汇报表格**：

```
| 阶段 | 步骤 | 状态 | 备注 |
|------|------|------|------|
| A | 步骤描述... | ✅ | — |
| B | go build ./... | ✅/❌ | 编译日志摘要 |
| B | npm run build | ✅/❌ | 编译日志摘要 |
| C | go test ./... | ✅/❌ | 测试结果摘要 |
| D | 文档更新 | ✅ | — |
```

汇报后提醒用户：
- 在浏览器中验证 Web UI 效果
- 通过 API 测试验证功能正确性
- 下一步运行 `/spec:verify` 进行完整验证
````

- [ ] **Step 2: 提交**

```bash
git add .claude/skills/spec-execute/SKILL.md
git commit -m "feat: add spec:execute skill for step-by-step plan execution"
```

---

### Task 7: 编写 `.claude/skills/spec-refactor/SKILL.md` — 代码重构审计

**Files:**
- Create: `.claude/skills/spec-refactor/SKILL.md`

- [ ] **Step 1: 编写 SKILL.md**

写入以下完整内容：

````markdown
---
name: spec:refactor
description: 新功能开发前对目标代码进行审计并清理遗留问题 - 触发: /spec:refactor, 重构, 清理代码, 代码审计
---

# 代码重构工作流 (Refactor)

在开发新功能前，对目标代码进行审计并清理遗留问题。零功能变更，仅改善代码质量。

## ⚠️ 重构铁律（开始前必读）

| # | 规则 | 说明 |
|---|------|------|
| 1 | **零功能变更** | 重构不能改变任何业务逻辑行为，只改善代码质量 |
| 2 | **编译必过** | 每一轮重构后都必须执行 `go build ./...` 验证 |
| 3 | **测试必过** | 重构后 `go test ./...` 必须通过 |
| 4 | **问题先列后改** | 必须先完成审计清单，经用户确认后才开始修改代码 |

## 执行指令

### 1. 审计

**定量红线**：必须使用代码搜索或文件读取 **至少扫描 5 个文件**才允许输出审计结论。

审计动作：
- 扫描用户指定的目标目录/文件
- 对照 `_spec_shared.md` 中的技术栈约束和代码风格检查违规项
- Go 代码检查项：
  - 死代码（未使用的函数、变量、类型、import）
  - 缺失 Go doc comment（导出函数/类型/接口）
  - 命名不规范（不符合 Go 社区约定）
  - 错误处理不当（忽略 error 返回值）
  - goroutine 泄漏风险（缺少 context 取消或超时机制）
  - 潜在的数据竞争（共享变量缺少 sync 保护）
  - 魔法数字（应提取为命名常量）
- React 代码检查项：
  - 未使用的 import / 组件 / 变量
  - class 组件（应改造为函数组件 + Hooks）
  - 直接操作 DOM（应使用 React 状态管理）
  - 重复代码（应提取为共享组件或工具函数）
- **语言要求**：所有反馈必须使用**简体中文**

### 2. 问题清单

审计完成后，**必须先输出问题清单再动代码**：

```
| # | 严重度 | 文件 | 行号 | 问题描述 | 建议修复 |
|---|--------|------|------|---------|---------|
| 1 | 🔴 高 | controller/xxx.go | L42 | 未处理的 error 返回值 | 添加 if err != nil 检查 |
| 2 | 🟡 中 | model/yyy.go | L18 | 导出函数缺少 doc comment | 补充 Go doc comment |
| 3 | 🟢 低 | web/.../Component.js | L95 | 变量命名使用拼音 | 改为英文驼峰命名 |
```

严重度分类：
- 🔴 **高**：违反 `_spec_shared.md` 的硬性规则、goroutine 泄漏、数据竞争、未处理 error、存在 `.js` 文件（应迁移为符合规范的代码）
- 🟡 **中**：代码质量问题（重复代码、缺失注释、过时模式、未使用 import）
- 🟢 **低**：风格建议（命名、格式、微优化）

**输出清单后暂停，等待用户确认再进行修改。**

### 3. 重构执行

用户确认后，按严重度从高到低依次清理：
- 修复错误处理（不可忽略 error 返回值）
- 补充 Go doc comment（导出函数/类型/接口）
- 提取重复代码为共享函数
- 清除死代码（未使用的 import、变量、函数）
- React：class 组件 → 函数组件 + Hooks
- 魔法数字 → 命名常量
- 每次修改后用 1-2 句话说明改动理由

### 4. 编译与测试验证（强制）

- 执行 `go build ./...` 确认编译通过
- 如改动涉及前端，执行 `cd web/default && npm run build`
- 执行 `go test ./...` 确认测试通过
- 如失败，立即修复并重新编译/测试（最多 3 轮）

### 5. 完成汇报

在回复末尾**必须输出以下汇报表格**：

```
| # | 文件 | 改动类型 | 改动说明 | 功能影响 |
|---|------|---------|---------|---------|
| 1 | controller/xxx.go | 错误处理 | 补充 error 检查 | 无 |
| 2 | model/yyy.go | 注释补充 | 补充 Go doc comment | 无 |
```

- 最后一行明确声明：**"以上改动均不涉及功能逻辑变更，仅为代码质量改善。"**
- 提示用户：运行 `/spec:define` 开始定义功能规范
````

- [ ] **Step 2: 提交**

```bash
git add .claude/skills/spec-refactor/SKILL.md
git commit -m "feat: add spec:refactor skill for pre-development code audit"
```

---

### Task 8: 编写 `.claude/skills/spec-verify/SKILL.md` — 功能验证

**Files:**
- Create: `.claude/skills/spec-verify/SKILL.md`

- [ ] **Step 1: 编写 SKILL.md**

写入以下完整内容：

````markdown
---
name: spec:verify
description: 对已完成功能进行视觉审计、边界测试、API验证和回归检查 - 触发: /spec:verify, 验证, 检查, 验收
---

# 功能验证工作流 (Verify)

对已完成的功能进行视觉审计、边界测试、API 验证和回归检查，生成验证报告。

**前置**: 必须先有 `specs/[feature_name]/spec.md` 和已完成的代码修改
**产物**: `specs/[feature_name]/verify_report.md`

## ⚠️ 验证标准（开始前必读）

验证报告必须同时满足以下指标：

| # | 检查项 | 合格标准 |
|---|--------|----------|
| 1 | 环境确认 | 明确记录 `go build ./...` 和 `go test ./...` 是否通过 |
| 2 | 功能审计 | 逐条对比 spec.md 的功能需求，每条给出 ✅/❌ 判定 |
| 3 | 边界测试 | 逐条对比 spec.md 的边界情况，每条给出测试结果 |
| 4 | API 验证 | API 端点返回正确的状态码和响应结构 |
| 5 | 回归检查 | 确认已有功能无退化（渠道中继、令牌鉴权、额度扣减） |
| 6 | 最终结论 | 明确给出 **通过 / 不通过** 的总判定 |

## 验证指令

### 1. 环境确认

- 执行 `go build ./...`，确认后端编译通过
- 执行 `go test ./...`，确认全部测试通过
- 如改动涉及前端，执行 `cd web/default && npm run build`，确认前端编译通过
- **记录编译与测试结果**：通过/失败 + 时间戳

### 2. 功能审计

- 加载 `specs/[feature_name]/spec.md` 的 **功能需求** 章节
- **逐条对比**规范要求与实际实现，输出结果表格：

```
| # | 功能需求描述 | 判定 | 差异说明 |
|---|-------------|------|---------|
| 1 | 新渠道 xxx 可成功中继非流式请求 | ✅ 符合 | — |
| 2 | 配置文件支持热更新 | ❌ 不符 | 需重启服务才生效 |
```

- 如果涉及前端变更，请求用户在浏览器中操作新功能并提供截图验证
- 如果涉及 API 变更，使用 curl 或 API 测试工具验证端点行为

### 3. 边界测试

- 加载 spec.md 的 **边界情况** 章节
- **逐条转化为测试用例**并执行验证：

```
| # | 边界场景 | 验证方式 | 期望结果 | 实际结果 | 判定 |
|---|---------|---------|---------|---------|------|
| 1 | 空 API Key 的渠道创建 | 发送空 Key 的创建请求 | 返回参数错误 | 返回 400 | ✅ |
| 2 | 并发请求同一令牌 | `ab -n 100 -c 10 /v1/chat/completions` | 所有请求正常完成 | ? | ✅/❌ |
| 3 | 10MB 超大请求体 | curl 发送 10MB payload | 返回 413 或正常处理 | ? | ✅/❌ |
| 4 | 删除渠道后立即请求 | 删除渠道后立即调用 API | 返回"无可用渠道" | ? | ✅/❌ |
```

- 检查服务端日志是否存在新增的 panic 或 error 信息

### 4. API 验证

- 列出本次改动涉及的所有 API 端点
- 对每个端点执行测试请求，验证：
  - HTTP 状态码符合预期
  - 响应 JSON 结构完整且字段类型正确
  - 错误情况下返回标准 OpenAI error 格式（`{"error": {"message": "...", "type": "..."}}`）
- 输出结果表格：

```
| 端点 | 方法 | 请求体 | 期望状态码 | 实际状态码 | 响应时间 | 判定 |
|------|------|--------|-----------|-----------|---------|------|
| /v1/chat/completions | POST | {"model":"gpt-4o","messages":[...]} | 200 | 200 | 1.2s | ✅ |
| /v1/chat/completions | POST | {"model":"unknown"} | 400/503 | 503 | 0.1s | ✅ |
```

### 5. 回归检查

确认修改未破坏原有功能。重点检查项：
- [ ] 渠道中继正常（随机选 3 个已有渠道，测试非流式和流式请求）
- [ ] 令牌鉴权正常（正确令牌通过、错误令牌返回 401、过期令牌拒绝）
- [ ] 额度扣减正常（请求后用户额度和渠道 used_quota 正确减少）
- [ ] Web UI 各页面正常加载（仪表板、渠道列表、令牌列表、日志页面）
- [ ] 管理 API 正常响应（`GET /api/status`、`GET /api/user/self`）

### 6. 输出报告

创建 `specs/[feature_name]/verify_report.md`，**必须包含以下结构**：

```markdown
# 验证报告 - [功能名称]

## 测试环境
- 日期：YYYY-MM-DD
- Go 版本：1.x
- 编译状态：通过/失败
- 测试状态：通过/失败（x/x 测试通过）
- 前端构建：通过/失败/未涉及

## 功能审计结果
（逐条表格 + 判定）

## 边界测试结果
（逐条表格 + 判定）

## API 验证结果
（逐端点表格 + 判定）

## 回归检查结果
- [ ] 渠道中继正常（已测 x 个渠道）
- [ ] 令牌鉴权正常
- [ ] 额度扣减正常
- [ ] Web UI 正常加载
- [ ] 管理 API 正常响应

## 最终结论
**✅ 通过** / **❌ 不通过**

（如不通过，附不通过原因及建议修复方案）
```

- **语言要求**: 报告所有文字描述均须使用**简体中文**

### 7. 汇报

回复用户时，简要总结验证结论：
- 通过了多少项 / 总共多少项
- 如有不通过项，列出最关键的 1-2 个问题
- 建议下一步操作（修复后重新验证 / 合并到 main / 发布）
````

- [ ] **Step 2: 提交**

```bash
git add .claude/skills/spec-verify/SKILL.md
git commit -m "feat: add spec:verify skill for feature verification and regression check"
```

---

### Task 9: 编写 5 个 `.agent/workflows/*.md` — Agent 派发模板

**Files:**
- Create: `.agent/workflows/define.md`
- Create: `.agent/workflows/architect.md`
- Create: `.agent/workflows/execute.md`
- Create: `.agent/workflows/refactor.md`
- Create: `.agent/workflows/verify.md`

Each workflow is a simplified version of the corresponding SKILL.md, retaining core execution instructions and delivery standards while omitting detailed background explanations.

- [ ] **Step 1: 编写 `.agent/workflows/define.md`**

写入以下内容：

```markdown
# Spec Define — Agent 派发模板

将需求转化为 `specs/[feature_name]/spec.md`。

## 硬指标
- `file:///` 链接 ≥ 3
- Mermaid 图 ≥ 1
- 代码片段 ≥ 2
- 边界情况 ≥ 4（场景 + 风险 + 缓解）
- 机制复用说明
- 文件清单表格（含改动类型和所属层）

## 步骤
1. 阅读 `docs/architecture/overview.md`（和 data-model.md、relay-system.md 如涉及）
2. 代码调研 ≥ 5 次搜索/读取，定位受影响代码，追溯现有机制
3. 起草 `specs/[feature_name]/spec.md`：背景 → 功能需求（根因+方案+Mermaid+复用）→ 文件清单 → 边界
4. 自检：验证 6 项硬指标全部达标
5. 输出摘要 + 自检表，提示运行 `/spec:architect`

## 语言
全部简体中文。
```

- [ ] **Step 2: 编写 `.agent/workflows/architect.md`**

写入以下内容：

```markdown
# Spec Architect — Agent 派发模板

将 `specs/[feature_name]/spec.md` 转化为 `specs/[feature_name]/plan.md`。

## 硬指标
- 文件清单表格（含 Go/React/Config 层）
- 复选框步骤 ≥ 3
- 阶段分组（A:代码 B:编译 C:测试 D:文档）
- 层级标签 `[Go]`/`[React]`/`[Build]`/`[Test]`/`[Docs]`
- 编译步骤：`go build ./...` + `npm run build`
- 测试步骤：`go test ./...`
- 关键步骤附代码片段

## 步骤
1. 加载 spec.md，提取根因、方案、文件清单
2. 起草 plan.md：架构设计（文件清单+影响评估+流程图）→ 分步实施（四阶段复选框+代码片段）
3. 出厂自检：验证 7 项全部达标
4. 输出摘要 + 自检表，提示运行 `/spec:execute`

## 语言
全部简体中文。
```

- [ ] **Step 3: 编写 `.agent/workflows/execute.md`**

写入以下内容：

```markdown
# Spec Execute — Agent 派发模板

按 `specs/[feature_name]/plan.md` 逐步执行代码修改。

## 铁律
1. 逐步执行，不跳步
2. 编译必过：Go 改 → `go build ./...`，React 改 → `npm run build`
3. 测试必过：`go test ./...`
4. 每步完成后将 plan.md 中 `- [ ]` 标记为 `- [x]`
5. 失败即停，最多重试 3 次
6. 禁止执行 plan 外改动

## 步骤
1. 打开 plan.md，通读分步实施章节
2. 按 A→B→C→D 顺序执行，每次修改后说明改动理由
3. 全阶段 B：执行编译验证，失败则修复重试
4. 全阶段 C：执行测试验证，失败则修复重试
5. 确认所有步骤已标记 `- [x]`
6. 输出汇报表格：阶段 × 步骤 × 状态，提示运行 `/spec:verify`

## 语言
全部简体中文。
```

- [ ] **Step 4: 编写 `.agent/workflows/refactor.md`**

写入以下内容：

```markdown
# Spec Refactor — Agent 派发模板

新功能开发前，对目标代码进行审计清理。零功能变更。

## 铁律
1. 零功能变更
2. 编译必过（`go build ./...`）
3. 测试必过（`go test ./...`）
4. 问题先列后改（用户确认后才动代码）

## Go 检查项
死代码、缺失 doc comment、命名不规范、error 处理不当、goroutine 泄漏、数据竞争、魔法数字

## React 检查项
未使用 import/组件、class 组件、直接 DOM 操作、重复代码

## 严重度
🔴高：违反硬性规则 | 🟡中：质量问题 | 🟢低：风格建议

## 步骤
1. 扫描目标目录 ≥ 5 个文件
2. 输出问题清单表格（严重度+文件+行号+问题+建议），等待用户确认
3. 确认后按严重度高→低依次清理
4. 执行 `go build ./...` + `go test ./...` 验证
5. 输出汇报表格，声明"以上改动均为代码质量改善"，提示运行 `/spec:define`

## 语言
全部简体中文。
```

- [ ] **Step 5: 编写 `.agent/workflows/verify.md`**

写入以下内容：

```markdown
# Spec Verify — Agent 派发模板

对已完成功能进行验证，生成 `specs/[feature_name]/verify_report.md`。

## 标准
1. 环境确认（`go build` + `go test` 通过）
2. 功能审计（逐条对比 spec.md，✅/❌）
3. 边界测试（逐条测试 spec.md 边界情况）
4. API 验证（端点状态码+响应结构）
5. 回归检查（渠道中继/令牌鉴权/额度扣减/Web UI/管理 API）
6. 最终结论（通过/不通过）

## 步骤
1. 执行 `go build ./...` + `go test ./...`，记录编译和测试状态
2. 逐条对比 spec.md 功能需求，输出审计表格
3. 逐条测试边界情况，输出测试表格
4. 测试所有涉及 API 端点，输出验证表格
5. 执行回归检查清单
6. 创建 verify_report.md（含全部 6 个章节）
7. 汇报总结：通过 N/总数，不通过时列出关键问题，建议下一步

## 语言
全部简体中文。
```

- [ ] **Step 6: 一次性提交所有 5 个 workflow 文件**

```bash
git add .agent/workflows/define.md .agent/workflows/architect.md .agent/workflows/execute.md .agent/workflows/refactor.md .agent/workflows/verify.md
git commit -m "feat: add agent workflow templates for spec pipeline"
```

---

### Task 10: 最终验证

**Files:**
- Verify: 所有 12 个文件存在且内容完整

- [ ] **Step 1: 验证所有文件存在**

```bash
echo "=== 宪章 ===" && test -f memory/constitution.md && echo "✓ memory/constitution.md" || echo "✗ MISSING"
echo "=== 共享约束 ===" && test -f .claude/skills/_spec_shared.md && echo "✓ .claude/skills/_spec_shared.md" || echo "✗ MISSING"
echo "=== 技能文件 ===" && \
  test -f .claude/skills/spec-define/SKILL.md && echo "✓ spec-define" || echo "✗ spec-define" && \
  test -f .claude/skills/spec-architect/SKILL.md && echo "✓ spec-architect" || echo "✗ spec-architect" && \
  test -f .claude/skills/spec-execute/SKILL.md && echo "✓ spec-execute" || echo "✗ spec-execute" && \
  test -f .claude/skills/spec-refactor/SKILL.md && echo "✓ spec-refactor" || echo "✗ spec-refactor" && \
  test -f .claude/skills/spec-verify/SKILL.md && echo "✓ spec-verify" || echo "✗ spec-verify"
echo "=== Workflow 模板 ===" && \
  test -f .agent/workflows/define.md && echo "✓ define" || echo "✗ define" && \
  test -f .agent/workflows/architect.md && echo "✓ architect" || echo "✗ architect" && \
  test -f .agent/workflows/execute.md && echo "✓ execute" || echo "✗ execute" && \
  test -f .agent/workflows/refactor.md && echo "✓ refactor" || echo "✗ refactor" && \
  test -f .agent/workflows/verify.md && echo "✓ verify" || echo "✗ verify"
```

Expected: 全部 12 项显示 ✓。

- [ ] **Step 2: 验证 _spec_shared.md 被所有 SKILL.md 引用**

```bash
grep -l "_spec_shared" .claude/skills/spec-*/SKILL.md | wc -l
```

Expected: 5（全部 5 个 skill 文件均引用共享约束）。

- [ ] **Step 3: 提交**

```bash
git add .agent/ .claude/ memory/
git commit -m "chore: final verification of spec skills file structure

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```
