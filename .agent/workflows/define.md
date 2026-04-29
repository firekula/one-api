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
