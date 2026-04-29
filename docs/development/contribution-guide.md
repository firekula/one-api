# 贡献指南

感谢你考虑为 One API 做出贡献！本文档将引导你了解贡献流程和规范。

---

## 1. 行为准则

本项目遵循 [Contributor Covenant](https://www.contributor-covenant.org/) 行为准则。参与本项目即表示你同意遵守该准则。

- 尊重所有参与者，无论其技术水平、性别、性取向、残疾、种族或宗教信仰。
- 鼓励建设性讨论，反对人身攻击、侮辱性言论和歧视性言论。
- 接受他人的 constructive feedback，专注于解决问题和改进项目。

---

## 2. 如何报告 Bug

### 提交前检查

在提交 Bug 报告前，请先确认：

- [ ] 搜索 [现有 Issues](https://github.com/songquanpeng/one-api/issues)，确认没有相似的已提交问题。
- [ ] 已升级到最新版本，确认问题仍然存在。
- [ ] 已完整查看过项目 [README](https://github.com/songquanpeng/one-api)，尤其是常见问题（FAQ）部分。

### 使用 Bug Report 模板

点击 [New Issue](https://github.com/songquanpeng/one-api/issues/new/choose) 选择 **Bug Report** 模板，填写以下信息：

**环境信息**

- 操作系统（Windows / macOS / Linux）
- 部署方式（Docker / 手动编译 / 宝塔面板 / 其他）
- One API 版本号
- Go 版本（如手动编译）

**复现步骤**

详细描述如何复现该问题，步骤清晰可操作。

**期望行为 vs 实际行为**

- 期望行为：你认为应该发生什么。
- 实际行为：实际发生了什么。

**日志片段**

粘贴相关的错误日志或截图。如果涉及 API 调用，请附上请求和响应（注意脱敏）。

### 注意事项

- 不在模板范围内的无关信息请尽量精简。
- 跟进 Issue 期间，请配合维护者进行排查，必要时提供更多信息。
- **不遵循模板或规则提交的 Issue 可能会被直接关闭。**

---

## 3. 如何提 Feature Request

### 使用 Feature Request 模板

点击 [New Issue](https://github.com/songquanpeng/one-api/issues/new/choose) 选择 **Feature Request** 模板。

### 内容要求

在模板中清晰描述以下内容：

- **应用场景**：什么情况下需要这个功能？解决了什么痛点？
- **期望行为**：希望系统具体做什么？描述功能的理想表现。
- **备选方案**：目前是否有其他方式可以绕过？如果有，为什么不够好？

### 标签

提交后请尽量为 Issue 打上合适的标签（如 `enhancement`、`feat` 等），方便分类和检索。

---

## 4. 开发流程

```
Fork 仓库 → Clone 到本地 → 创建分支 → 编写代码 → 提交代码 → Push 到 Fork → 创建 Pull Request
```

### 详细步骤

```bash
# 1. 在 GitHub 上 Fork 仓库
#    访问 https://github.com/songquanpeng/one-api 点击 Fork

# 2. Clone 到本地
git clone https://github.com/YOUR_USERNAME/one-api.git
cd one-api

# 3. 添加上游仓库（upstream）
git remote add upstream https://github.com/songquanpeng/one-api.git

# 4. 创建功能分支（分支命名规则见下一节）
git checkout -b feat/my-feature

# 5. 开发
#     参考 docs/development/setup.md 搭建开发环境

# 6. 提交代码
git add .
git commit -m "feat: 简短描述你的改动"

# 7. 推送到你的 Fork
git push origin feat/my-feature

# 8. 在 GitHub 上创建 Pull Request
#     访问你的 Fork 仓库，点击 "Compare & pull request"
```

### 保持同步

在开发过程中，建议定期同步上游仓库的最新代码：

```bash
git fetch upstream
git rebase upstream/main
```

---

## 5. 分支命名

请使用以下前缀命名分支，保持清晰和一致：

| 前缀          | 用途         |
|---------------|--------------|
| `feat/xxx`    | 新功能       |
| `fix/xxx`     | Bug 修复     |
| `docs/xxx`    | 文档变更     |
| `style/xxx`   | 代码格式调整（不影响功能） |
| `refactor/xxx`| 代码重构     |
| `chore/xxx`   | 构建配置、依赖管理、杂项 |

示例：`feat/support-gpt5`、`fix/login-error`、`docs/update-readme`。

---

## 6. Commit Message 规范

本项目使用 [Conventional Commits](https://www.conventionalcommits.org/) 规范，CI 流程中通过 commitlint 自动校验。

### 格式

```
<type>: <简短描述>
```

### 允许的类型

| 类型       | 说明                         |
|------------|------------------------------|
| `feat`     | 新功能                       |
| `fix`      | Bug 修复                     |
| `docs`     | 文档变更                     |
| `style`    | 代码格式调整（不影响功能）   |
| `chore`    | 构建配置、依赖管理、杂项     |
| `refactor` | 代码重构（既不修复 Bug 也不添加功能） |

### 要求

- 第一行不超过 **72 个字符**。
- 描述使用中文或英文均可，但要简洁明了，直击要点。
- 如果此项提交关联某个 Issue，可在描述中或 body 中关联。

### 示例（取自项目提交历史）

```
feat: support Gemini openai compatible api
feat: add OpenAI compatible channel
fix: fix cannot select test model when searching
docs: update ByteDance Doubao model link in README
chore: update prompt
style: improve code formatting and structure in ChannelsTable
refactor: split relay handler into separate package
```

---

## 7. Go 代码风格

### 格式化

- 提交前必须使用 `gofmt` 格式化代码：

```bash
gofmt -w .
```

### 命名规范

- **变量/函数**：驼峰命名（`camelCase` / `CamelCase`）。
- **包名**：全小写，不包含下划线。
- **常量**：根据作用域决定，导出常量使用 `CamelCase`。

### 错误处理

- 函数返回 `error`，由调用方决定如何处理。
- **不使用 `panic`**，除非遇到不可恢复的异常（如启动时配置错误）。
- 善用 `errors.Wrap` 或 `fmt.Errorf("context: %w", err)` 包装错误信息。

### 代码组织

- 保持与现有代码一致的风格：相同功能的变量命名、相同的文件组织方式。
- 新增渠道适配器放置在 `relay/adaptor/<provider>/` 目录下。
- 频道类型常量和元数据分别定义在 `relay/channeltype/` 目录下。
- **不要引入不必要的第三方依赖。** 如果确实需要新依赖，请在 PR 描述中说明理由。

---

## 8. React 前端代码风格

### 组件编写

- 使用 **函数组件 + Hooks**，不使用 class 组件。
- 遵循项目中已有的 ESLint 配置。

### 状态管理

- 使用 React Context 进行全局状态管理。参照 `web/default/src/context/` 目录下现有的模式。
- 组件内部状态优先使用 `useState` / `useReducer`。

### API 调用

- 统一使用 `web/default/src/helpers/api.js` 中封装好的函数进行 API 调用。

### 常量和配置

- 常量定义在 `web/default/src/constants/` 目录下，按模块拆分。
- 渠道相关常量请添加到 `channel.constants.js`。

---

## 9. 新增渠道 PR Checklist

新增一个渠道适配器涉及前后端多个文件。请在 PR 描述中逐项检查：

```markdown
- [ ] 后端：`relay/adaptor/<provider>/` 完整实现
- [ ] 后端：`relay/channeltype/define.go` 添加类型常量
- [ ] 后端：`relay/channeltype/` 中注册渠道元数据
- [ ] 后端：适配器注册表（`relay/adaptor/`）添加映射
- [ ] 前端：`web/default/src/constants/channel.constants.js` 添加渠道类型选项
- [ ] 测试验证：非流式请求、流式请求、错误处理、usage 提取
- [ ] PR 描述填写完整
```

---

## 10. PR 描述模板

创建 Pull Request 时，请按以下模板填写描述：

```markdown
## 变更内容

简要描述本次 PR 做了什么改变。

## 测试方式

描述如何验证本次变更是有效的，例如：

- 单元测试命令及结果
- 手动测试的操作步骤
- 测试覆盖的场景

## 截图

如果有前端变更，请附上前后的界面截图。

## 关联 Issue

Closes #xxx
```

### PR 要求

- PR 标题遵循 commit message 规范（使用 `<type>: <描述>` 格式）。
- 如果 PR 涉及较大改动，请先提 Issue 进行讨论，确认方向后再进行开发。
- 确保 CI 流程（单元测试、commitlint）全部通过。
- Review 过程中请保持开放心态，积极回应评审意见。

---

## 11. CI 流程

本项目使用 GitHub Actions 进行持续集成，配置在 `.github/workflows/ci.yml`。

### 触发条件

- 仅当代码推送至 `main` 分支时触发。

### 包含的检查

| Job          | 说明                           |
|--------------|--------------------------------|
| Unit tests   | 运行 `go test -cover ./...`，上传覆盖率报告至 Codecov |
| Commit lint  | 使用 commitlint 校验 commit message 格式 |

### 本地运行测试

在提交前建议本地运行测试，确保不引入回归：

```bash
go test -cover ./...
```

---

## 12. 其他资源

- [开发环境搭建](docs/development/setup.md)
- [渠道适配器开发指南](docs/development/adaptor-development.md)
- [项目 README](https://github.com/songquanpeng/one-api)
- [Issues](https://github.com/songquanpeng/one-api/issues)

---

再次感谢你的贡献！
