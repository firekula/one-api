# 数据导入导出功能 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 one-api 增加管理面板 + 管理 API 的 JSON 快照导出/导入，方便部署升级时迁移核心业务数据。

**Architecture:** 新增 `model/backup.go`（导出读取 + 事务导入）与 `controller/data.go`（两个 RootAuth 端点）；前端在设置页 `OperationSetting` 组件加"数据迁移"区块。导入按 Channel→User→Token→Redemption→Option 顺序，业务唯一键判重跳过，id 重建并维护映射还原引用。

**Tech Stack:** Go 1.20+ / GORM（sqlite/mysql/postgres）/ gin / React(CRA) + semantic-ui-react。

## Global Constraints

- 模块路径：`github.com/songquanpeng/one-api/...`，所有 import 用该前缀
- 不新增任何第三方依赖（只用现有 gorm/gin/标准库）
- 日志统一 `common/logger`（`SysError`/`SysLogf`），不直接 `fmt.Println` 打日志
- 错误消息用中文（与现有风格一致，如 `"无效的令牌"`）
- TDD：每个任务先写失败测试，验证失败后再实现，再验证通过
- 测试执行环境：本机 Windows 无 mingw gcc，`go test` 必须在 `golang:1.22-alpine` 容器内跑（打包源码 → `docker cp` → 容器内 `go test`；`//go:embed web/build/*` 要求打包时含 `web/build`，否则 main 包编译失败）
- 提交信息风格：`feat: 中文描述`（本项目禁止 Co-Authored-By 行）
- 前端改动后必须重建 `web/build`（`//go:embed` 才会收录新产物），交付时三个主题都要构建

---

### Task 1: model 层导出（BackupData + ExportData）

**Files:**
- Create: `model/backup.go`
- Test: `model/backup_test.go`

**Interfaces:**
- Produces（供 Task 2/3 使用）:
  - `type BackupData struct { SchemaVersion int \`json:"schemaVersion"\`; ExportedAt int64 \`json:"exportedAt"\`; Data BackupTables \`json:"data"\` }`
  - `type BackupTables struct { Channels []*Channel \`json:"channels"\`; Users []*User \`json:"users"\`; Tokens []*Token \`json:"tokens"\`; Redemptions []*Redemption \`json:"redemptions"\`; Options []*Option \`json:"options"\` }`
  - `func ExportData() (*BackupData, error)`
  - `func setupTestDB(t *testing.T)`（测试辅助，`model/backup_test.go` 包内函数：初始化临时 sqlite 文件库 + AutoMigrate + 替换全局 `model.DB`，`t.Cleanup` 恢复）

- [ ] **Step 1: 写失败测试**

`model/backup_test.go`：

```go
package model

import (
	"fmt"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB 用临时 sqlite 文件库初始化全局 DB（避免 :memory: 多连接问题），
// 并在测试结束后恢复原 DB。
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	dsn := fmt.Sprintf("%s?_busy_timeout=5000&_journal_mode=WAL", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &Token{}, &Channel{}, &Redemption{}, &Option{}, &Ability{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })
	return db
}

func seedTestData(t *testing.T, db *gorm.DB) {
	t.Helper()
	users := []User{
		{Username: "root", Password: "$2a$10$hash", Role: 100, Status: 1, Quota: 500000, AccessToken: "at-root", AffCode: "aff-root"},
		{Username: "alice", Password: "$2a$10$hash2", Role: 1, Status: 1, Quota: 1000, AccessToken: "at-alice", AffCode: "aff-alice"},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	channels := []Channel{
		{Type: 1, Key: "sk-channel-1", Name: "ch1", BaseURL: "https://api.example.com", Models: "gpt-3.5-turbo", Group: "default", Status: 1},
	}
	if err := db.Create(&channels).Error; err != nil {
		t.Fatalf("seed channels: %v", err)
	}
	if err := channels[0].AddAbilities(); err != nil {
		t.Fatalf("seed abilities: %v", err)
	}
	tokens := []Token{
		{UserId: users[0].Id, Key: "token-key-1", Status: TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 500, UnlimitedQuota: false, Name: "t1"},
		{UserId: users[1].Id, Key: "token-key-2", Status: TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 100, UnlimitedQuota: false, Name: "t2"},
	}
	if err := db.Create(&tokens).Error; err != nil {
		t.Fatalf("seed tokens: %v", err)
	}
	redemptions := []Redemption{
		{Key: "red-key-1", Status: 1, Name: "r1", Quota: 1000, CreatedTime: 1},
	}
	if err := db.Create(&redemptions).Error; err != nil {
		t.Fatalf("seed redemptions: %v", err)
	}
	options := []Option{
		{Key: "SystemName", Value: "One API"},
		{Key: "GitHubClientSecret", Value: "secret-value"}, // 敏感键也必须导出
	}
	if err := db.Create(&options).Error; err != nil {
		t.Fatalf("seed options: %v", err)
	}
}

func TestExportData(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	data, err := ExportData()
	if err != nil {
		t.Fatalf("ExportData: %v", err)
	}
	if data.SchemaVersion != 1 {
		t.Fatalf("schemaVersion = %d, want 1", data.SchemaVersion)
	}
	if data.ExportedAt == 0 {
		t.Fatal("exportedAt 不应为 0")
	}
	if len(data.Data.Users) != 2 {
		t.Fatalf("users = %d, want 2", len(data.Data.Users))
	}
	if len(data.Data.Channels) != 1 {
		t.Fatalf("channels = %d, want 1", len(data.Data.Channels))
	}
	if len(data.Data.Tokens) != 2 {
		t.Fatalf("tokens = %d, want 2", len(data.Data.Tokens))
	}
	if len(data.Data.Redemptions) != 1 {
		t.Fatalf("redemptions = %d, want 1", len(data.Data.Redemptions))
	}
	if len(data.Data.Options) != 2 {
		t.Fatalf("options = %d, want 2", len(data.Data.Options))
	}
	// 渠道 Key 必须导出（迁移需要密钥）
	if data.Data.Channels[0].Key != "sk-channel-1" {
		t.Fatalf("channel key 未导出: %q", data.Data.Channels[0].Key)
	}
	// 敏感 option 必须导出
	foundSecret := false
	for _, o := range data.Data.Options {
		if o.Key == "GitHubClientSecret" {
			foundSecret = true
		}
	}
	if !foundSecret {
		t.Fatal("敏感 option 键未导出")
	}
	// 用户 password 哈希必须导出（导入时原样还原）
	if data.Data.Users[0].Password == "" {
		t.Fatal("user password 哈希未导出")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./model/ -run TestExportData`
Expected: 编译失败，`undefined: ExportData`、`undefined: BackupData`

- [ ] **Step 3: 实现导出**

`model/backup.go`：

```go
package model

import (
	"github.com/songquanpeng/one-api/common/helper"
)

const backupSchemaVersion = 1

type BackupTables struct {
	Channels    []*Channel    `json:"channels"`
	Users       []*User       `json:"users"`
	Tokens      []*Token      `json:"tokens"`
	Redemptions []*Redemption `json:"redemptions"`
	Options     []*Option     `json:"options"`
}

type BackupData struct {
	SchemaVersion int          `json:"schemaVersion"`
	ExportedAt    int64        `json:"exportedAt"`
	Data          BackupTables `json:"data"`
}

// ExportData 导出核心业务数据。注意：这是管理员显式备份操作，
// 渠道 Key、用户密码哈希、敏感 option 均按原样导出。
func ExportData() (*BackupData, error) {
	data := &BackupData{
		SchemaVersion: backupSchemaVersion,
		ExportedAt:    helper.GetTimestamp(),
	}
	var err error
	if data.Data.Channels, err = GetAllChannels(0, 0, "all"); err != nil {
		return nil, err
	}
	if data.Data.Options, err = AllOption(); err != nil {
		return nil, err
	}
	if err = DB.Find(&data.Data.Users).Error; err != nil {
		return nil, err
	}
	if err = DB.Find(&data.Data.Tokens).Error; err != nil {
		return nil, err
	}
	if err = DB.Find(&data.Data.Redemptions).Error; err != nil {
		return nil, err
	}
	return data, nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./model/ -run TestExportData`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add model/backup.go model/backup_test.go
git commit -m "feat: 数据导出（BackupData/ExportData）"
```

---

### Task 2: model 层导入（ImportData）

**Files:**
- Modify: `model/backup.go`
- Test: `model/backup_test.go`

**Interfaces:**
- Consumes: `ExportData` / `BackupData` / `setupTestDB` / `seedTestData`（Task 1）
- Produces（供 Task 3 使用）:
  - `type ImportResult struct { Inserted map[string]int \`json:"inserted"\`; Skipped map[string]int \`json:"skipped"\`; Failed []string \`json:"failed"\` }`
  - `func ImportData(data *BackupData) (*ImportResult, error)`

- [ ] **Step 1: 写失败测试**

追加到 `model/backup_test.go`：

```go
func TestImportDataRoundTrip(t *testing.T) {
	// 库 A：造数据并导出
	dbA := setupTestDB(t)
	seedTestData(t, dbA)
	exported, err := ExportData()
	if err != nil {
		t.Fatalf("ExportData: %v", err)
	}

	// 库 B：全新空库，导入
	setupTestDB(t) // 替换为第二个临时库
	result, err := ImportData(exported)
	if err != nil {
		t.Fatalf("ImportData: %v", err)
	}
	if result.Inserted["users"] != 2 || result.Inserted["tokens"] != 2 ||
		result.Inserted["channels"] != 1 || result.Inserted["redemptions"] != 1 ||
		result.Inserted["options"] != 2 {
		t.Fatalf("inserted 统计不正确: %+v", result.Inserted)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("failed 应为空: %+v", result.Failed)
	}

	// 校验引用映射：Token.UserId 必须指向导入后的新用户
	var tk Token
	if err := DB.Where("`key` = ?", "token-key-1").First(&tk).Error; err != nil {
		t.Fatalf("find imported token: %v", err)
	}
	var u User
	if err := DB.First(&u, tk.UserId).Error; err != nil {
		t.Fatalf("imported token 引用的用户不存在: %v", err)
	}
	if u.Username != "root" {
		t.Fatalf("token.user_id 映射错误，指向 %q", u.Username)
	}
	// Channel 的 Ability 已重建
	var abilityCount int64
	DB.Model(&Ability{}).Where("channel_id = ?", 1).Count(&abilityCount)
	if abilityCount == 0 {
		t.Fatal("channel 导入后 Ability 未重建")
	}
}

func TestImportDataSkipDuplicates(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	exported, err := ExportData()
	if err != nil {
		t.Fatalf("ExportData: %v", err)
	}
	// 同一实例上二次导入：全部按唯一键跳过
	result, err := ImportData(exported)
	if err != nil {
		t.Fatalf("ImportData: %v", err)
	}
	if result.Inserted["users"] != 0 || result.Inserted["tokens"] != 0 ||
		result.Inserted["channels"] != 0 || result.Inserted["redemptions"] != 0 ||
		result.Inserted["options"] != 0 {
		t.Fatalf("二次导入应全部 skipped: %+v", result.Inserted)
	}
	for _, table := range []string{"users", "tokens", "channels", "redemptions", "options"} {
		if result.Skipped[table] == 0 {
			t.Fatalf("%s 应有 skipped 记录", table)
		}
	}
}

func TestImportDataIDConflictWithExistingRoot(t *testing.T) {
	// 目标库已有 root（id=1），导入数据里也有 root（username 相同）
	db := setupTestDB(t)
	seedTestData(t, db) // 库 A 数据（含 root, id=1）
	exported, err := ExportData()
	if err != nil {
		t.Fatalf("ExportData: %v", err)
	}

	// 目标库：也先建 root（模拟真实新部署，id=1 被占用）
	setupTestDB(t)
	db2 := DB
	root := User{Username: "root", Password: "$2a$10$newhash", Role: 100, Status: 1, Quota: 999, AccessToken: "at-new", AffCode: "aff-new"}
	if err := db2.Create(&root).Error; err != nil {
		t.Fatalf("create existing root: %v", err)
	}

	result, err := ImportData(exported)
	if err != nil {
		t.Fatalf("ImportData: %v", err)
	}
	// 目标库已有的 root 按 username 跳过，alice 插入
	if result.Skipped["users"] != 1 || result.Inserted["users"] != 1 {
		t.Fatalf("users 导入统计错误: inserted=%d skipped=%d", result.Inserted["users"], result.Skipped["users"])
	}
	// alice 的 token 应正确映射到 alice 的新 id
	var alice User
	if err := DB.Where("username = ?", "alice").First(&alice).Error; err != nil {
		t.Fatalf("find alice: %v", err)
	}
	var tk Token
	if err := DB.Where("`key` = ?", "token-key-2").First(&tk).Error; err != nil {
		t.Fatalf("find token-key-2: %v", err)
	}
	if tk.UserId != alice.Id {
		t.Fatalf("token-key-2 user_id = %d, want alice id %d", tk.UserId, alice.Id)
	}
}

func TestImportDataRejectsBadSchemaVersion(t *testing.T) {
	setupTestDB(t)
	data := &BackupData{SchemaVersion: 99}
	if _, err := ImportData(data); err == nil {
		t.Fatal("schemaVersion 99 应被拒绝")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./model/ -run TestImportData`
Expected: 编译失败，`undefined: ImportData`、`undefined: ImportResult`

- [ ] **Step 3: 实现导入**

追加到 `model/backup.go`：

```go
import (
	"errors"
	"fmt"

	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/helper"
)

type ImportResult struct {
	Inserted map[string]int `json:"inserted"`
	Skipped  map[string]int `json:"skipped"`
	Failed   []string       `json:"failed"`
}

func keyColumn() string {
	if common.UsingPostgreSQL {
		return `"key"`
	}
	return "`key`"
}

// ImportData 合并导入：按业务唯一键判重，已存在则跳过；id 重建并维护
// 旧 id→新 id 映射还原引用。任一表写入失败则整体回滚。
func ImportData(data *BackupData) (*ImportResult, error) {
	if data.SchemaVersion != backupSchemaVersion {
		return nil, errors.New("不支持的备份文件版本")
	}
	result := &ImportResult{
		Inserted: map[string]int{},
		Skipped:  map[string]int{},
		Failed:   []string{},
	}
	keyCol := keyColumn()
	err := DB.Transaction(func(tx *gorm.DB) error {
		channelIDMap := map[int]int{}
		userIDMap := map[int]int{}

		// 1. Channel（含 Ability 重建；AddAbilities 内部用全局 DB，事务内改为直接 tx.Create）
		for _, ch := range data.Data.Channels {
			var count int64
			if err := tx.Model(&Channel{}).Where("type = ? AND `key` = ? AND base_url = ?", ch.Type, ch.Key, ch.BaseURL).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				result.Skipped["channels"]++
				continue
			}
			oldID := ch.Id
			ch.Id = 0
			if err := tx.Create(ch).Error; err != nil {
				return err
			}
			channelIDMap[oldID] = ch.Id
			// 重建 Ability（与 AddAbilities 逻辑一致，但使用事务连接）
			models := utils.DeDuplication(strings.Split(ch.Models, ","))
			groups := strings.Split(ch.Group, ",")
			abilities := make([]Ability, 0, len(models)*len(groups))
			for _, model := range models {
				for _, group := range groups {
					abilities = append(abilities, Ability{
						Group:     group,
						Model:     model,
						ChannelId: ch.Id,
						Enabled:   ch.Status == ChannelStatusEnabled,
						Priority:  ch.Priority,
					})
				}
			}
			if len(abilities) > 0 {
				if err := tx.Create(&abilities).Error; err != nil {
					return err
				}
			}
			result.Inserted["channels"]++
		}

		// 2. User（原生 Create 保留 password 哈希/access_token/aff_code 原值）
		for _, u := range data.Data.Users {
			var count int64
			if err := tx.Model(&User{}).Where("username = ?", u.Username).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				result.Skipped["users"]++
				continue
			}
			oldID := u.Id
			u.Id = 0
			if err := tx.Create(u).Error; err != nil {
				return err
			}
			userIDMap[oldID] = u.Id
			result.Inserted["users"]++
		}

		// 3. Token（UserId 经映射转换）
		for _, tk := range data.Data.Tokens {
			var count int64
			if err := tx.Model(&Token{}).Where(keyCol+" = ?", tk.Key).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				result.Skipped["tokens"]++
				continue
			}
			newUserID, ok := userIDMap[tk.UserId]
			if !ok {
				result.Failed = append(result.Failed, fmt.Sprintf("token %s: 引用的用户不存在，已跳过", tk.Key))
				continue
			}
			tk.Id = 0
			tk.UserId = newUserID
			if err := tx.Create(tk).Error; err != nil {
				return err
			}
			result.Inserted["tokens"]++
		}

		// 4. Redemption
		for _, r := range data.Data.Redemptions {
			var count int64
			if err := tx.Model(&Redemption{}).Where(keyCol+" = ?", r.Key).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				result.Skipped["redemptions"]++
				continue
			}
			r.Id = 0
			if err := tx.Create(r).Error; err != nil {
				return err
			}
			result.Inserted["redemptions"]++
		}

		// 5. Option（含敏感键；已存在跳过）
		for _, o := range data.Data.Options {
			var count int64
			if err := tx.Model(&Option{}).Where("`key` = ?", o.Key).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				result.Skipped["options"]++
				continue
			}
			if err := UpdateOption(o.Key, o.Value); err != nil {
				return err
			}
			result.Inserted["options"]++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 刷新内存渠道缓存，让新渠道立即生效
	InitChannelCache()
	return result, nil
}
```

注意：`model/backup.go` 需补充 import：`errors`、`fmt`、`strings`、`gorm.io/gorm`、`github.com/songquanpeng/one-api/common/utils`。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./model/ -run TestImportData`
Expected: 4 个测试全部 PASS

- [ ] **Step 5: 提交**

```bash
git add model/backup.go model/backup_test.go
git commit -m "feat: 数据导入（合并导入/判重跳过/id 映射/事务）"
```

---

### Task 3: controller 端点 + 路由

**Files:**
- Create: `controller/data.go`
- Modify: `router/api.go`（在 apiRouter 注册两行）
- Test: `controller/data_test.go`

**Interfaces:**
- Consumes: `model.ExportData()`、`model.ImportData(data)`、`model.BackupData`、`model.ImportResult`（Task 1/2）
- Produces: `func GetExportData(c *gin.Context)`、`func ImportData(c *gin.Context)`（router/api.go 注册）

- [ ] **Step 1: 写失败测试**

`controller/data_test.go`：

```go
package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/songquanpeng/one-api/model"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupControllerTestDB(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	dsn := fmt.Sprintf("%s?_busy_timeout=5000&_journal_mode=WAL", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Token{}, &model.Channel{}, &model.Redemption{}, &model.Option{}, &model.Ability{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB })
}

func TestGetExportData(t *testing.T) {
	setupControllerTestDB(t)
	// 插入一条数据
	u := model.User{Username: "root", Password: "hash", Role: 100, Status: 1, Quota: 100, AccessToken: "at", AffCode: "aff"}
	if err := model.DB.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	r := gin.New()
	r.GET("/api/data/export", GetExportData)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/data/export", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp struct {
		Data struct {
			SchemaVersion int `json:"schemaVersion"`
			Data          struct {
				Users []*model.User `json:"users"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Data.SchemaVersion != 1 {
		t.Fatalf("schemaVersion = %d, want 1", resp.Data.SchemaVersion)
	}
	if len(resp.Data.Data.Users) != 1 {
		t.Fatalf("users = %d, want 1", len(resp.Data.Data.Users))
	}
}

func TestImportDataEndpoint(t *testing.T) {
	setupControllerTestDB(t)
	backup := &model.BackupData{
		SchemaVersion: 1,
		ExportedAt:    123,
		Data: model.BackupTables{
			Users: []*model.User{{Username: "bob", Password: "hash", Role: 1, Status: 1, Quota: 100, AccessToken: "at-bob", AffCode: "aff-bob"}},
		},
	}
	bodyBytes, _ := json.Marshal(backup)

	// 构造 multipart 请求
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "backup.json")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(bodyBytes); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	mw.Close()

	r := gin.New()
	r.POST("/api/data/import", ImportData)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/data/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Inserted map[string]int `json:"inserted"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("success = false, body=%s", w.Body.String())
	}
	if resp.Data.Inserted["users"] != 1 {
		t.Fatalf("inserted.users = %d, want 1", resp.Data.Inserted["users"])
	}
}

func TestImportDataEndpointBadJSON(t *testing.T) {
	setupControllerTestDB(t)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "bad.json")
	fw.Write([]byte("not json"))
	mw.Close()

	r := gin.New()
	r.POST("/api/data/import", ImportData)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/data/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./controller/ -run "TestGetExportData|TestImportDataEndpoint"`
Expected: 编译失败，`undefined: GetExportData`、`undefined: ImportData`

- [ ] **Step 3: 实现 controller 与路由**

`controller/data.go`：

```go
package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/songquanpeng/one-api/model"
)

// GetExportData 导出全部核心业务数据（RootAuth）。返回的 JSON 可直接下载保存。
func GetExportData(c *gin.Context) {
	data, err := model.ExportData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "导出失败：" + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

// ImportData 从上传的 JSON 备份文件导入数据（RootAuth）。
func ImportData(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "未找到上传的备份文件",
		})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无法读取上传的备份文件",
		})
		return
	}
	defer f.Close()
	var data model.BackupData
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无法解析备份文件",
		})
		return
	}
	result, err := model.ImportData(&data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
	})
}
```

`router/api.go`（在 `apiRouter` 内追加，与其它 RootAuth 路由并列）：

```go
	apiRouter.GET("/data/export", RootAuth(), controller.GetExportData)
	apiRouter.POST("/data/import", RootAuth(), controller.ImportData)
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./controller/ -run "TestGetExportData|TestImportDataEndpoint"`
Expected: 3 个测试全部 PASS

- [ ] **Step 5: 提交**

```bash
git add controller/data.go controller/data_test.go router/api.go
git commit -m "feat: 数据导出/导入管理 API（RootAuth）"
```

---

### Task 4: 前端数据迁移区块

**Files:**
- Modify: `web/default/src/components/OperationSetting.js`
- Test: 前端构建验证（无单测框架，用 build 验证）

**Interfaces:**
- Consumes: 后端 `GET /api/data/export`、`POST /api/data/import`（Task 3）；现有 `API`（`helpers/api.js`）、`downloadTextAsFile`（`helpers/utils.js`，经 `../helpers` 导出）、`showSuccess`/`showError`

- [ ] **Step 1: 写失败测试（无测试框架，改为先确认现状可构建）**

Run: `cd web/default && npm install --legacy-peer-deps && DISABLE_ESLINT_PLUGIN=true npm run build`
Expected: 构建成功（若本机 node 版本导致失败，改用容器 `node:16` 执行同样命令）

- [ ] **Step 2: 实现前端区块**

在 `web/default/src/components/OperationSetting.js`：

1. import 增加 `downloadTextAsFile`：

```js
import {
  API,
  showError,
  showSuccess,
  timestamp2string,
  verifyJSON,
  downloadTextAsFile,
} from '../helpers';
```

2. 组件内新增两个 handler（放在 `deleteHistoryLogs` 之后）：

```js
  const exportData = async () => {
    const res = await API.get('/api/data/export');
    const { success, message, data } = res.data;
    if (!success) {
      showError('导出失败：' + message);
      return;
    }
    const date = new Date().toISOString().slice(0, 10);
    downloadTextAsFile(JSON.stringify(data, null, 2), `one-api-backup-${date}.json`);
    showSuccess('数据已导出，请妥善保存备份文件！');
  };

  const importData = async (e) => {
    const file = e.target.files[0];
    if (!file) {
      return;
    }
    const formData = new FormData();
    formData.append('file', file);
    const res = await API.post('/api/data/import', formData);
    const { success, message, data } = res.data;
    if (!success) {
      showError('导入失败：' + message);
      return;
    }
    const fmt = (m) => Object.entries(m || {}).map(([k, v]) => `${k}: ${v}`).join('，');
    showSuccess(`导入完成！新增 ${fmt(data.inserted)}；跳过 ${fmt(data.skipped)}${data.failed && data.failed.length ? `；失败 ${data.failed.length} 条` : ''}`);
    e.target.value = '';
  };
```

3. 在 JSX 的 `<Form loading={loading}>` 末尾（`general` 区块的保存按钮之后、`</Form>` 之前）追加数据迁移区块：

```jsx
          <Divider />
          <Header as='h3'>数据迁移（导出 / 导入全部数据）</Header>
          <Form.Group inline>
            <Form.Button
              onClick={() => {
                exportData().then();
              }}
            >
              导出数据
            </Form.Button>
            <Form.Button
              content='导入数据'
              as='label'
              htmlFor='data-import-file'
            />
            <input
              id='data-import-file'
              type='file'
              accept='.json,application/json'
              hidden
              onChange={importData}
            />
          </Form.Group>
          <p style={{ color: '#888' }}>
            导出会生成包含用户、令牌、渠道、兑换码与系统设置的 JSON 备份文件；
            导入为合并模式，与现有数据冲突（相同用户名 / 令牌 / 兑换码键）时自动跳过，不会覆盖或删除现有数据。
          </p>
```

- [ ] **Step 3: 验证前端可构建**

Run: `cd web/default && DISABLE_ESLINT_PLUGIN=true npm run build`
Expected: 构建成功，产物含新的操作设置页

- [ ] **Step 4: 提交**

```bash
git add web/default/src/components/OperationSetting.js
git commit -m "feat: 设置页新增数据导出/导入入口"
```

---

### Task 5: 全量验证与交付

**Files:**
- Modify: 无（仅验证与构建产物）

- [ ] **Step 1: model + controller 全量测试（容器内）**

Run（容器内）: `go test ./model/ ./controller/ -count=1`
Expected: 全部 PASS

- [ ] **Step 2: 全仓测试无回归（容器内）**

Run（容器内，打包含 `web/build`）: `go test ./... -count=1`
Expected: 除 `common/image`（依赖外网 wikimedia 下载，环境性失败，与本次无关）外全部 ok

- [ ] **Step 3: 重建三个主题前端产物**

Run: `cd web && sh build.sh`（本机 node 24 失败则改容器 node:16 执行，`DISABLE_ESLINT_PLUGIN=true`）
Expected: `web/build/{default,berry,air}` 更新，`//go:embed web/build/*` 收录新页面

- [ ] **Step 4: 手动冒烟验证（可选，本机 docker）**

- 按既有 docker cp 流程构建测试镜像（`docker build -t firekula/one-api:data-backup .`）
- 起容器（sqlite 模式，`-p 3800:3000`），登录 root → 设置页看到"数据迁移"区块
- 导出下载 JSON → 新容器导入 → 确认令牌/渠道恢复

- [ ] **Step 5: 提交构建产物与收尾**

```bash
git add web/build
git commit -m "build: 重建前端产物（含数据迁移入口）"
```

---

## Self-Review

**1. Spec coverage：**
- JSON 格式（schemaVersion/exportedAt/五表）→ Task 1 BackupData ✓
- 合并导入、判重键（五表）→ Task 2 ✓
- id 重建 + 映射 → Task 2（channelIDMap/userIDMap + Token 映射、Ability 重建）✓
- Option 含敏感键 + 已存在跳过 → Task 1 测试断言 + Task 2 实现 ✓
- 导入后 InitChannelCache → Task 2 Step 3 末尾 ✓
- 错误处理（400/500/失败统计）→ Task 2（schemaVersion 拒绝、Failed 列表）+ Task 3（400/500）✓
- 管理面板 + 管理 API → Task 3（API）+ Task 4（面板）✓
- 不导出 Log / 不做定时备份等 → 明确未实现，符合 YAGNI ✓
- 测试清单（往返/判重/映射/回滚路径/端点）→ Task 1/2/3 测试 ✓

**2. Placeholder scan：** 无 TBD/TODO；每个代码步骤都含完整代码。✓

**3. Type consistency：** `BackupData`/`BackupTables`/`ImportResult`/`ExportData`/`ImportData` 在 Task 1/2/3 中签名一致；`setupTestDB` 在 model 包内、`setupControllerTestDB` 在 controller 包内（不同包不冲突）；controller 引用 `model.BackupData` 字段名与 Task 1 定义一致。✓

**已知实现细节提示（供执行者）：**
- `Task 2` 中 `utils.DeDuplication` 来自 `github.com/songquanpeng/one-api/common/utils`，`strings` 为 std；若 `tx.Create(&abilities)` 与 `AddAbilities` 的构造存在差异，以 Task 2 代码为准（事务内必须用 tx）。
- `Task 3` 的 `router/api.go` 中 `RootAuth`/`controller` 均已 import，仅追加两行。
- `Task 4` 的 `semantic-ui-react` `Form.Button as='label' htmlFor` 为文件选择的标准写法（项目无第三方上传组件）。
