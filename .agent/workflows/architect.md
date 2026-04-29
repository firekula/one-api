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
