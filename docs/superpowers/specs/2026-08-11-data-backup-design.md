# 数据导入导出功能设计

日期：2026-08-11
状态：已评审（brainstorming 流程）

## 背景与目标

部署升级/迁移时，用户希望手动导出核心业务数据并在新部署上导入，替代直接拷贝 SQLite 文件（易因挂载路径错位、WAL 三件套遗漏导致数据丢失）。本功能提供**管理面板 + 管理 API** 的 JSON 快照导出/导入。

## 范围

### 包含（核心业务数据）

- Channel（含 Key 与模型映射；Ability 为派生数据，不单独导出，导入时重建）
- User（含 password 哈希、access_token、aff_code 原值）
- Token（含 Key、额度）
- Redemption（含 Key）
- Option（系统设置，含敏感键 *Secret/*Token）

### 排除

- Log（独立 LOG_DB、量大、无迁移价值）
- Ability（随 Channel 重建）
- 数据库 id 原值（导入时重建 id 并维护映射，避免目标库 id 冲突）

## JSON 格式

```json
{
  "schemaVersion": 1,
  "exportedAt": 1786438131,
  "data": {
    "channels": [ { "id": 1, "type": 1, "key": "sk-...", "name": "...", "base_url": "...", "models": "...", "groups": "...", "status": 1, "priority": 0, ... } ],
    "users":    [ { "id": 1, "username": "root", "password": "<bcrypt哈希>", "role": 100, "status": 1, "quota": 500000, "access_token": "...", "aff_code": "...", ... } ],
    "tokens":   [ { "id": 1, "user_id": 1, "key": "...", "status": 1, "expired_time": -1, "remain_quota": 500000, "unlimited_quota": false, ... } ],
    "redemptions": [ { "id": 1, "key": "...", "status": 1, "quota": 1000, ... } ],
    "options":  [ { "key": "SystemName", "value": "One API" }, ... ]
  }
}
```

- `schemaVersion` 用于未来格式演进；当前固定为 1
- 各表数组导出对应 model 的全部持久化字段（含敏感字段）

## 架构与组件

### 1. `model/backup.go`（新增）

- `ExportData() (*BackupData, error)`：全量读取五张表
  - Channel 用 `GetAllChannels(0, 0, "all")`（scope=all 含 Key）
  - Option 用 `AllOption()`（不经过滤，含敏感键）
  - User 用全量查询（含 password/access_token，导出是管理员显式操作，属预期）
- `ImportData(data *BackupData) (*ImportResult, error)`：事务导入
  - 校验 schemaVersion
  - `DB.Transaction` 包裹，任一步失败整体回滚
  - 顺序：Channel → User → Token → Redemption → Option

### 2. `controller/data.go`（新增）

- `GetExportData(c)`：调用 `ExportData`，返回 JSON（Content-Disposition 建议附件名 `one-api-backup-<date>.json`，前端亦可前端生成文件名）
- `ImportData(c)`：从 multipart 文件读取 JSON（`c.FormFile("file")`），解析 → `ImportData` → 返回统计
- 均注册 `RootAuth`（导出含密钥，仅 root 可见）

### 3. `router/api.go`（修改）

```go
apiRouter.GET("/data/export", RootAuth(), controller.GetExportData)
apiRouter.POST("/data/import", RootAuth(), controller.ImportData)
```

### 4. 前端 `web/default/src/pages/Setting/OperationSetting.js`（修改，Root 可见）

- "数据迁移"区块：
  - 导出按钮：调用 `API.get('/api/data/export')` 获取 JSON，用现有 `downloadTextAsFile(text, 'one-api-backup-<date>.json')` 落盘
  - 导入按钮：`<input type="file" accept=".json">` 选文件 → `FormData` POST `/api/data/import` → 展示返回统计（新增/跳过/失败）
- 无第三方依赖

## 导入逻辑细节

### 判重键（合并导入，重复跳过，不覆盖不删除）

| 表 | 判重键 |
|---|---|
| Channel | `(type, key, base_url)` 组合（表无唯一约束） |
| User | `username`（唯一索引） |
| Token | `key`（唯一索引） |
| Redemption | `key`（唯一索引） |
| Option | `key`（主键） |

### id 重建与引用映射

- 导入时**不保留原 id**，插入后由数据库分配新 id
- 维护两个映射：`oldChannelId → newChannelId`、`oldUserId → newUserId`
- Token 导入时 `UserId` 用映射转换；映射缺失（导出数据被手工裁剪）则跳过该 Token 并计入 failed
- Channel 导入后立即重建 Ability（`AddAbilities`，与 `BatchInsertChannels` 一致），不再依赖导出数据中的 Ability

### Option 导入

- 逐键 `model.UpdateOption(key, value)`（含敏感键），已存在则跳过
- `UpdateOption` 内部同步内存 `OptionMap` 与对应 config 变量，无需重启

### 导入后刷新

- 导入完成后调用 `model.InitChannelCache()` 重建内存渠道缓存（否则新渠道不生效）
- Option 已由 `UpdateOption` 同步，无需额外处理

## 错误处理

| 场景 | 行为 |
|---|---|
| JSON 解析失败 / 非 BackupData 结构 | 400，消息 `无法解析备份文件` |
| schemaVersion 非 1 | 400，消息 `不支持的备份文件版本` |
| 任一表写入失败 | 事务整体回滚，500 |
| 上传文件过大 | 复用现有上传限流 |
| 映射缺失导致引用无法还原 | 跳过该行并计入 failed（不中断整体导入） |

## 返回统计

```json
{
  "inserted": { "channels": 3, "users": 5, "tokens": 12, "redemptions": 0, "options": 20 },
  "skipped":  { "channels": 1, "users": 1, "tokens": 0, "redemptions": 0, "options": 5 },
  "failed": []
}
```

## 测试

1. **model 层**（`model/backup_test.go`，临时 sqlite 库）：
   - 导出→导入空库往返：各表行数一致；Token.UserId 经映射后指向新 User；Channel 的 Ability 已重建
   - 重复导入：第二次全部 skipped
   - id 冲突：目标库已有用户 id=1（root）时，导入的 root 按 username 跳过，其余数据正确插入且引用映射正确
2. **controller 层**（`controller/data_test.go`，httptest）：
   - 导出返回 schemaVersion 与五张表
   - 导入返回统计结构
   - 非法 JSON → 400
3. 全量 `go test ./...` 无回归

## 明确不做（YAGNI）

- 不导出/导入 Log
- 不做按表拆分端点、差异同步、定时自动备份
- 不做密码解密/重加密（password 哈希原样还原）
- 不保留原 id（避免目标库 id 冲突，改用重建 + 映射）
