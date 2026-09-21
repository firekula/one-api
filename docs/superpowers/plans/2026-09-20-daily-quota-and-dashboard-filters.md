# 单日用量上限与总览/日志筛选 实现计划

> **给自动化执行者：** 必须使用子技能 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务执行本计划；步骤用 `- [ ]` 复选框跟踪。

**目标：** 给所有用户加单日 token/额度双维度上限（可按用户豁免或覆盖），并把总览改造成"管理员默认全站、可选时间段/粒度、可按用户/令牌/模型筛选"的复盘视图，同时把三个主题日志页的用户/令牌筛选从手打文本框改成下拉。

**架构：** 后端新增一张按本地日期分行的 `daily_usage` 累计表作为单日用量的唯一事实来源（不读 `logs`，因为 `RecordConsumeLog` 受 `config.LogConsumeEnabled` 门控）；准入判定用估算值、结算写入真实值，判定点前移到额度预扣之前。总览不再新增接口，而是扩展 `/api/user/dashboard` 的参数与权限；另新增 `/api/log/filters` 返回所选区间内日志里实际出现过的用户/令牌/模型候选值，供日志页与总览共用。

**技术栈：** Go 1.20 + gin + GORM（sqlite/mysql/postgres）+ Redis（可选）；前端 React/CRA，三主题 `web/default`（semantic-ui + recharts）、`web/berry`（MUI + ApexCharts）、`web/air`（Semi UI + 命令式 `@vchart`）。三套主题互不复用代码。

**设计依据：** [2026-09-20-daily-quota-and-dashboard-filters-design.md](../specs/2026-09-20-daily-quota-and-dashboard-filters-design.md)（已评审并提交，接口契约已冻结；实现中不要重新设计或再向用户征询已问过的问题）。

## 全局约束

以下约束每个任务都适用，值逐字取自设计文档：

- 模块路径 `github.com/songquanpeng/one-api`；仓库内文档一律中文。
- 日志一律走 `common/logger`（`SysLogf`/`Errorf`/`Info`），禁止 `fmt.Println`。
- 新表名固定 `daily_usage`（模型 `DailyUsage`，必须显式实现 `TableName()`，否则 GORM 默认取名 `daily_usages`）。
- 用户字段固定 `daily_token_limit`、`daily_quota_limit`，类型 `*int64`。语义：`nil`/`0` = 跟随全局默认；`-1` = 豁免；`> 0` = 单独上限。前端切回"跟随全局"必须**显式发送 `0`**（发 `null` 会被 GORM 结构体 `Updates` 跳过，静默不生效）。
- 全局选项固定 `DailyTokenLimitDefault`、`DailyQuotaLimitDefault`，`int64`，`0` = 不限，**不加环境变量**（运行时以 DB options 表为准，设置页是唯一入口）。
- 全局默认选项的 `UpdateOption` 必须拒绝负值。
- 错误码固定 `insufficient_user_daily_tokens` / `insufficient_user_daily_quota`，均为 HTTP 403，消息带具体数字，不走 i18n（与现有 relay 错误一致）。
- 时间口径统一**服务器本地时区**：Go 侧用 `time.Local` 计算当天零点；SQL 分桶三方言都按本地时间，SQLite 必须带 `'localtime'` 修饰符。
- 查询区间上限：`hour` 粒度 90 天，`day` 粒度 366 天，超限返回 400。
- 候选值接口每个列表 `LIMIT 500`。
- `logs` 表不新增 `token_id` 列；令牌筛选只能按 `token_name`。
- 不改 `model.User` 既有字段语义，不改备份导出格式（因此用户列表的今日用量用 DTO 拼装，不给 `model.User` 加字段）。
- 图片与音频请求不累加 token，只受额度上限约束。
- 前端三主题各自构建；`web/build/` 被 gitignore，不入库。
- **Windows 测试陷阱**：所有用 `t.TempDir()` 建 sqlite 库的测试辅助函数，都必须在 `t.Cleanup` 里关闭连接（`db.DB()` → `Close()`），且注册顺序要在 `t.TempDir()` 之后——`t.Cleanup` 是 LIFO，这样关闭会先于 TempDir 的目录删除执行。否则 Windows 报 `The process cannot access the file because it is being used by another process`，会把整个包判为 FAIL。计划里给出的测试辅助函数已包含这段清理。
- **已知基线失败（与本次改动无关，验证时排除）**：`common/image` 的 `TestDecode` 会联网从 wikimedia 下载测试图片，本机网络下报 `image: unknown format` 并 panic。因此 `go test ./...` 的验收口径是「除 `common/image` 外全部 ok」；跑全量时用 `go test $(go list ./... | grep -v common/image)`。
- 每个任务结束必须 `go build ./...` 或对应主题 `npm run build` 通过后再提交。

## 实施顺序与依赖

| 阶段 | 内容 | 依赖 |
|---|---|---|
| Part A | 后端：单日用量上限（设计文档阶段 1） | 无 |
| Part B | 后端：总览查询与候选值接口（设计文档阶段 2） | 无（与 A 无共同改动面，可并行） |
| Part C | 前端：default 主题（设计文档阶段 3） | A、B 的接口契约 |
| Part D | 前端：berry 与 air 主题（设计文档阶段 4） | C 确认过的交互 |

TDD 说明：A1–A4、B1–B2、B3–B5 有真实逻辑，走"先写失败测试"的完整流程。A5（接入四个模态）、A6 之后的界面任务属于接线与 UI，仓库没有对应的测试脚手架（relay 路径需要真实上游，前端三主题无测试基建），因此这些任务用"实现 → 明确命令验证 → 提交"的步骤，并在 A7 用端到端手工验收兜底。这是本计划唯一偏离严格 TDD 的地方。

---

# Part A：单日用量上限（后端）

### Task A1: `daily_usage` 表与用量记账

**Files:**
- Create: `model/daily_usage.go`
- Modify: `model/main.go`（`migrateDB`，约 140-167 行）
- Test: `model/daily_usage_test.go`

**Interfaces:**
- Consumes: `model.DB`（`*gorm.DB` 包级变量）、`common.LogQuota(quota int64) string`
- Produces:
  - `type DailyUsage struct{ Id int; UserId int; Day string; PromptTokens, CompletionTokens, Quota int64 }`
  - `func Today() string`
  - `func GetDailyUsage(userId int, day string) (*DailyUsage, error)`
  - `func RecordDailyUsage(userId int, promptTokens, completionTokens, quota int64) error`
  - `func GetDailyUsages(userIds []int, day string) (map[int]*DailyUsage, error)`

- [ ] **Step 1: 写失败的测试**

创建 `model/daily_usage_test.go`：

```go
package model

import (
	"fmt"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupModelTestDB 用临时 sqlite 替换包级 DB，并在测试结束还原。
func setupModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	dsn := fmt.Sprintf("%s?_busy_timeout=5000&_journal_mode=WAL", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &DailyUsage{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })
	// Windows 上必须先释放 sqlite 文件句柄，否则 t.TempDir 的清理会报
	// "The process cannot access the file because it is being used by another process"
	// 而把整个包判为 FAIL。t.Cleanup 是 LIFO，这里在 TempDir 之后注册，因此先执行。
	if sqlDB, err := db.DB(); err == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	return db
}

func TestRecordDailyUsageAccumulates(t *testing.T) {
	setupModelTestDB(t)

	if err := RecordDailyUsage(7, 100, 50, 300); err != nil {
		t.Fatalf("第一次记账失败: %v", err)
	}
	if err := RecordDailyUsage(7, 10, 5, 30); err != nil {
		t.Fatalf("第二次记账失败: %v", err)
	}

	usage, err := GetDailyUsage(7, Today())
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if usage.PromptTokens != 110 || usage.CompletionTokens != 55 || usage.Quota != 330 {
		t.Fatalf("累加结果错误: prompt=%d completion=%d quota=%d，期望 110/55/330",
			usage.PromptTokens, usage.CompletionTokens, usage.Quota)
	}
}

func TestRecordDailyUsageKeepsDaysSeparate(t *testing.T) {
	setupModelTestDB(t)

	if err := RecordDailyUsage(7, 100, 0, 100); err != nil {
		t.Fatalf("今天记账失败: %v", err)
	}
	// 直接写一条昨天的记录，验证不会与今天合并
	yesterday := DailyUsage{UserId: 7, Day: "2020-01-01", PromptTokens: 999}
	if err := DB.Create(&yesterday).Error; err != nil {
		t.Fatalf("写入历史记录失败: %v", err)
	}
	if err := RecordDailyUsage(7, 1, 0, 1); err != nil {
		t.Fatalf("今天再记账失败: %v", err)
	}

	var count int64
	if err := DB.Model(&DailyUsage{}).Where("user_id = ?", 7).Count(&count).Error; err != nil {
		t.Fatalf("统计行数失败: %v", err)
	}
	if count != 2 {
		t.Fatalf("应有两个日期的记录，实际 %d 行", count)
	}
	todayUsage, err := GetDailyUsage(7, Today())
	if err != nil {
		t.Fatalf("读取今天失败: %v", err)
	}
	if todayUsage.PromptTokens != 101 {
		t.Fatalf("今天的记录被污染: prompt=%d，期望 101", todayUsage.PromptTokens)
	}
	oldUsage, err := GetDailyUsage(7, "2020-01-01")
	if err != nil {
		t.Fatalf("读取历史日期失败: %v", err)
	}
	if oldUsage.PromptTokens != 999 {
		t.Fatalf("历史记录被污染: prompt=%d，期望 999", oldUsage.PromptTokens)
	}
}

func TestGetDailyUsageMissingRowReturnsZero(t *testing.T) {
	setupModelTestDB(t)

	usage, err := GetDailyUsage(404, Today())
	if err != nil {
		t.Fatalf("无记录时不应报错: %v", err)
	}
	if usage.UserId != 404 || usage.PromptTokens != 0 || usage.Quota != 0 {
		t.Fatalf("无记录时应返回零值且带 userId，实际: %+v", usage)
	}
}

func TestGetDailyUsagesBulk(t *testing.T) {
	setupModelTestDB(t)

	if err := RecordDailyUsage(1, 10, 0, 10); err != nil {
		t.Fatalf("记账失败: %v", err)
	}
	if err := RecordDailyUsage(2, 20, 0, 20); err != nil {
		t.Fatalf("记账失败: %v", err)
	}

	usages, err := GetDailyUsages([]int{1, 2, 3}, Today())
	if err != nil {
		t.Fatalf("批量读取失败: %v", err)
	}
	if len(usages) != 2 {
		t.Fatalf("应返回 2 个用户的记录，实际 %d", len(usages))
	}
	if usages[1].PromptTokens != 10 || usages[2].PromptTokens != 20 {
		t.Fatalf("批量读取数值错误: %+v", usages)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./model/ -run 'TestRecordDailyUsage|TestGetDailyUsage' -v`
Expected: 编译失败，报 `undefined: RecordDailyUsage`、`undefined: DailyUsage`、`undefined: Today`

- [ ] **Step 3: 实现**

创建 `model/daily_usage.go`：

```go
package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// daily_usage 按服务器本地日期累计每个用户的当日用量。
// 它是单日上限的唯一事实来源，刻意不读 logs 表：RecordConsumeLog 受
// config.LogConsumeEnabled 门控，关掉日志会让上限静默失效。
type DailyUsage struct {
	Id               int    `json:"id" gorm:"primaryKey"`
	UserId           int    `json:"user_id" gorm:"uniqueIndex:idx_daily_usage_user_day,priority:1"`
	Day              string `json:"day" gorm:"type:varchar(10);uniqueIndex:idx_daily_usage_user_day,priority:2"`
	PromptTokens     int64  `json:"prompt_tokens" gorm:"bigint;default:0"`
	CompletionTokens int64  `json:"completion_tokens" gorm:"bigint;default:0"`
	Quota            int64  `json:"quota" gorm:"bigint;default:0"`
}

// TableName 显式指定表名，避免 GORM 复数化成 daily_usages。
func (DailyUsage) TableName() string {
	return "daily_usage"
}

// Today 返回服务器本地时区的当日日期，格式 YYYY-MM-DD。
func Today() string {
	return time.Now().Format("2006-01-02")
}

// GetDailyUsage 读取某用户某天的累计用量；无记录时返回带 userId/day 的零值，不报错。
func GetDailyUsage(userId int, day string) (*DailyUsage, error) {
	usage := &DailyUsage{UserId: userId, Day: day}
	err := DB.Where("user_id = ? AND day = ?", userId, day).First(usage).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return usage, nil
}

// RecordDailyUsage 原子累加某用户今天的用量。与 model.RecordConsumeLog 并列调用，
// 但不受 config.LogConsumeEnabled 影响。
func RecordDailyUsage(userId int, promptTokens, completionTokens, quota int64) error {
	if userId == 0 {
		return errors.New("userId 不能为 0")
	}
	if promptTokens == 0 && completionTokens == 0 && quota == 0 {
		return nil
	}
	usage := DailyUsage{
		UserId:           userId,
		Day:              Today(),
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		Quota:            quota,
	}
	// SET 右边用未限定列名：MySQL 的 ON DUPLICATE KEY UPDATE、SQLite/PostgreSQL 的
	// ON CONFLICT DO UPDATE 都把它解析为已存在行的当前值，三方言通用。
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "day"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"prompt_tokens":     gorm.Expr("prompt_tokens + ?", promptTokens),
			"completion_tokens": gorm.Expr("completion_tokens + ?", completionTokens),
			"quota":             gorm.Expr("quota + ?", quota),
		}),
	}).Create(&usage).Error
}

// GetDailyUsages 按用户 id 批量读取某天的用量，供管理列表一次取整页。
func GetDailyUsages(userIds []int, day string) (map[int]*DailyUsage, error) {
	result := make(map[int]*DailyUsage, len(userIds))
	if len(userIds) == 0 {
		return result, nil
	}
	var usages []*DailyUsage
	if err := DB.Where("day = ? AND user_id IN ?", day, userIds).Find(&usages).Error; err != nil {
		return nil, err
	}
	for _, usage := range usages {
		result[usage.UserId] = usage
	}
	return result, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./model/ -run 'TestRecordDailyUsage|TestGetDailyUsage' -v`
Expected: 4 个测试全部 PASS

- [ ] **Step 5: 接入自动迁移**

在 `model/main.go` 的 `migrateDB()` 中，`DB.AutoMigrate(&Log{})` 之后、第二处 `DB.AutoMigrate(&Channel{})` 之前插入：

```go
	if err = DB.AutoMigrate(&DailyUsage{}); err != nil {
		return err
	}
```

- [ ] **Step 6: 提交**

```bash
go build ./... && go test ./model/ -run 'TestRecordDailyUsage|TestGetDailyUsage'
git add model/daily_usage.go model/daily_usage_test.go model/main.go
git commit -m "feat: 新增 daily_usage 单日用量累计表与记账"
```

---

### Task A2: 用户三态字段与有效上限解析

**Files:**
- Modify: `model/user.go`（`User` 结构体，约 34-54 行）
- Test: `model/user_daily_limit_test.go`

**Interfaces:**
- Produces:
  - `func (user *User) EffectiveDailyTokenLimit() int64`
  - `func (user *User) EffectiveDailyQuotaLimit() int64`
  - `func resolveDailyLimit(override *int64, globalDefault int64) int64`（包内私有）
- Consumes: `config.DailyTokenLimitDefault`、`config.DailyQuotaLimitDefault`（Task A3 添加；本任务先引用，A3 落地前 `go build` 会失败，故 A2 与 A3 必须一起提交或用 `0` 常量占位——按 Step 顺序先做 A3 也可以，见 Step 5 说明）

- [ ] **Step 1: 写失败的测试**

创建 `model/user_daily_limit_test.go`：

```go
package model

import (
	"testing"

	"github.com/songquanpeng/one-api/common/config"
)

func ptr(v int64) *int64 { return &v }

func TestResolveDailyLimit(t *testing.T) {
	cases := []struct {
		name          string
		override      *int64
		globalDefault int64
		want          int64
	}{
		{"未设置且无全局默认", nil, 0, 0},
		{"未设置跟随全局", nil, 5000, 5000},
		{"显式 0 跟随全局", ptr(0), 5000, 5000},
		{"显式 0 且全局为 0", ptr(0), 0, 0},
		{"豁免覆盖全局", ptr(-1), 5000, 0},
		{"豁免且全局为 0", ptr(-1), 0, 0},
		{"单独上限覆盖全局", ptr(100), 5000, 100},
		{"单独上限小于全局", ptr(10), 5000, 10},
		{"非法的更小负数按豁免处理", ptr(-5), 5000, 0},
		{"全局默认被写成负数按不限处理", nil, -3, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveDailyLimit(tc.override, tc.globalDefault); got != tc.want {
				t.Fatalf("resolveDailyLimit(%v, %d) = %d，期望 %d", tc.override, tc.globalDefault, got, tc.want)
			}
		})
	}
}

func TestUserEffectiveDailyLimits(t *testing.T) {
	oldToken, oldQuota := config.DailyTokenLimitDefault, config.DailyQuotaLimitDefault
	config.DailyTokenLimitDefault = 1000
	config.DailyQuotaLimitDefault = 2000
	t.Cleanup(func() {
		config.DailyTokenLimitDefault = oldToken
		config.DailyQuotaLimitDefault = oldQuota
	})

	user := &User{}
	if user.EffectiveDailyTokenLimit() != 1000 {
		t.Fatalf("token 上限应跟随全局 1000，实际 %d", user.EffectiveDailyTokenLimit())
	}
	if user.EffectiveDailyQuotaLimit() != 2000 {
		t.Fatalf("额度上限应跟随全局 2000，实际 %d", user.EffectiveDailyQuotaLimit())
	}

	user = &User{DailyTokenLimit: ptr(-1), DailyQuotaLimit: ptr(50)}
	if user.EffectiveDailyTokenLimit() != 0 {
		t.Fatalf("token 应被豁免（0=不限），实际 %d", user.EffectiveDailyTokenLimit())
	}
	if user.EffectiveDailyQuotaLimit() != 50 {
		t.Fatalf("额度应使用单独上限 50，实际 %d", user.EffectiveDailyQuotaLimit())
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./model/ -run 'TestResolveDailyLimit|TestUserEffectiveDailyLimits' -v`
Expected: 编译失败，报 `undefined: resolveDailyLimit`、`unknown field DailyTokenLimit`、`config.DailyTokenLimitDefault undefined`

- [ ] **Step 3: 实现**

在 `model/user.go` 的 `User` 结构体末尾（`InviterId` 字段之后）加两个字段：

```go
	// 单日用量上限，三态：nil/0 = 跟随全局默认，-1 = 豁免（不限），>0 = 该用户的单独上限。
	// 用指针而非 int64 是必须的：User.Update 是结构体形式的 Updates，GORM 会跳过零值字段，
	// 非指针字段一旦被设过就无法再改回 0（跟随全局）。
	DailyTokenLimit *int64 `json:"daily_token_limit" gorm:"bigint;default:0"`
	DailyQuotaLimit *int64 `json:"daily_quota_limit" gorm:"bigint;default:0"`
```

在 `model/user.go` 末尾加：

```go
// resolveDailyLimit 把三态覆盖值与全局默认解析成"有效上限"，返回 0 表示不限。
func resolveDailyLimit(override *int64, globalDefault int64) int64 {
	if override != nil && *override < 0 {
		return 0 // 豁免
	}
	if override != nil && *override > 0 {
		return *override
	}
	if globalDefault > 0 {
		return globalDefault
	}
	return 0 // 全局默认非正数一律按不限处理
}

// EffectiveDailyTokenLimit 返回该用户当前生效的单日 token 上限，0 表示不限。
func (user *User) EffectiveDailyTokenLimit() int64 {
	return resolveDailyLimit(user.DailyTokenLimit, config.DailyTokenLimitDefault)
}

// EffectiveDailyQuotaLimit 返回该用户当前生效的单日额度上限，0 表示不限。
func (user *User) EffectiveDailyQuotaLimit() int64 {
	return resolveDailyLimit(user.DailyQuotaLimit, config.DailyQuotaLimitDefault)
}
```

若 `model/user.go` 尚未导入 `config`，在 import 块加入 `"github.com/songquanpeng/one-api/common/config"`。

- [ ] **Step 4: 先落地 Task A3 的配置变量，再运行测试**

本步骤与 Task A3 的 Step 3 共用：先在 `common/config/config.go` 加上两个变量（内容见 Task A3），然后：

Run: `go test ./model/ -run 'TestResolveDailyLimit|TestUserEffectiveDailyLimits' -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交**（与 A3 一起提交，因为编译互锁）

见 Task A3 的提交步骤。

---

### Task A3: 两个全局默认选项

**Files:**
- Modify: `common/config/config.go`（约 94-103 行的额度相关变量区）
- Modify: `model/option.go`（`InitOptionMap` 约 24-81 行；`updateOptionMap` 约 120-247 行）
- Modify: `controller/option.go`（`UpdateOption` 校验 switch，约 47-100 行）
- Test: `model/daily_option_test.go`

**Interfaces:**
- Produces: `config.DailyTokenLimitDefault int64`、`config.DailyQuotaLimitDefault int64`；选项键 `"DailyTokenLimitDefault"`、`"DailyQuotaLimitDefault"`

- [ ] **Step 1: 写失败的测试**

创建 `model/daily_option_test.go`：

```go
package model

import (
	"testing"

	"github.com/songquanpeng/one-api/common/config"
)

func TestUpdateOptionDailyLimitDefaults(t *testing.T) {
	setupModelTestDB(t)
	InitOptionMap()

	oldToken, oldQuota := config.DailyTokenLimitDefault, config.DailyQuotaLimitDefault
	t.Cleanup(func() {
		config.DailyTokenLimitDefault = oldToken
		config.DailyQuotaLimitDefault = oldQuota
	})

	if err := UpdateOption("DailyTokenLimitDefault", "123456"); err != nil {
		t.Fatalf("写入 token 默认上限失败: %v", err)
	}
	if config.DailyTokenLimitDefault != 123456 {
		t.Fatalf("DailyTokenLimitDefault = %d，期望 123456", config.DailyTokenLimitDefault)
	}

	if err := UpdateOption("DailyQuotaLimitDefault", "789"); err != nil {
		t.Fatalf("写入额度默认上限失败: %v", err)
	}
	if config.DailyQuotaLimitDefault != 789 {
		t.Fatalf("DailyQuotaLimitDefault = %d，期望 789", config.DailyQuotaLimitDefault)
	}

	// 默认值必须是 0（不限），否则新装实例会意外限制所有人
	InitOptionMap()
	if config.OptionMap["DailyTokenLimitDefault"] == "" || config.OptionMap["DailyQuotaLimitDefault"] == "" {
		t.Fatal("两个新选项必须出现在 OptionMap 中，否则设置页读不到")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./model/ -run TestUpdateOptionDailyLimitDefaults -v`
Expected: FAIL，`config.DailyTokenLimitDefault` 仍为 0 / 或编译失败

- [ ] **Step 3: 实现**

`common/config/config.go`，在 `var PreConsumedQuota int64 = 500` 之后插入：

```go
// 单日用量上限的全局默认值，0 表示不限制。仅通过设置页（options 表）配置，不提供环境变量。
var DailyTokenLimitDefault int64 = 0
var DailyQuotaLimitDefault int64 = 0
```

`model/option.go` 的 `InitOptionMap()` 中，紧跟 `config.OptionMap["PreConsumedQuota"] = ...` 之后插入：

```go
	config.OptionMap["DailyTokenLimitDefault"] = strconv.FormatInt(config.DailyTokenLimitDefault, 10)
	config.OptionMap["DailyQuotaLimitDefault"] = strconv.FormatInt(config.DailyQuotaLimitDefault, 10)
```

`model/option.go` 的 `updateOptionMap` 里，紧跟 `case "PreConsumedQuota":` 分支之后插入：

```go
	case "DailyTokenLimitDefault":
		config.DailyTokenLimitDefault, _ = strconv.ParseInt(value, 10, 64)
	case "DailyQuotaLimitDefault":
		config.DailyQuotaLimitDefault, _ = strconv.ParseInt(value, 10, 64)
```

`controller/option.go` 的 `UpdateOption` 校验 switch 中，在最后一个 case 之后插入：

```go
	case "DailyTokenLimitDefault", "DailyQuotaLimitDefault":
		if v, err := strconv.ParseInt(option.Value, 10, 64); err != nil || v < 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "单日上限必须是不小于 0 的整数（0 表示不限制）",
			})
			return
		}
```

若 `controller/option.go` 未导入 `strconv`，加入 import。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./model/ -run 'TestUpdateOptionDailyLimitDefaults|TestResolveDailyLimit|TestUserEffectiveDailyLimits' -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交**（Task A2 + A3 一起）

```bash
go build ./... && go test ./model/ -run 'DailyLimit|DailyOption|EffectiveDaily|ResolveDaily'
git add common/config/config.go model/option.go model/user.go controller/option.go model/user_daily_limit_test.go model/daily_option_test.go
git commit -m "feat: 新增单日上限的三态用户字段与全局默认选项"
```

---

### Task A4: 判定函数与错误类型

**Files:**
- Modify: `model/daily_usage.go`
- Test: `model/daily_limit_check_test.go`

**Interfaces:**
- Produces:
  - `type DailyLimitState struct{ DailyTokenLimit *int64; DailyQuotaLimit *int64; UsedTokens int64; UsedQuota int64 }`
  - `func GetDailyLimitState(userId int, day string) (*DailyLimitState, error)`
  - `type DailyLimitError struct{ Dimension string; Used, Limit, Estimated int64 }`（实现 `error`）
  - `func AsDailyLimitError(err error) (*DailyLimitError, bool)`
  - `func CheckUserDailyLimit(userId int, addTokens int64, addQuota int64) error`
- Consumes: `config.DailyTokenLimitDefault`、`config.DailyQuotaLimitDefault`、`resolveDailyLimit`（A2）、`Today`（A1）

- [ ] **Step 1: 写失败的测试**

创建 `model/daily_limit_check_test.go`：

```go
package model

import (
	"testing"

	"github.com/songquanpeng/one-api/common/config"
)

func withGlobalDailyLimits(t *testing.T, token, quota int64) {
	t.Helper()
	oldToken, oldQuota := config.DailyTokenLimitDefault, config.DailyQuotaLimitDefault
	config.DailyTokenLimitDefault = token
	config.DailyQuotaLimitDefault = quota
	t.Cleanup(func() {
		config.DailyTokenLimitDefault = oldToken
		config.DailyQuotaLimitDefault = oldQuota
	})
}

func createUserWithDailyLimits(t *testing.T, token, quota *int64) *User {
	t.Helper()
	user := &User{
		Username:        "u_daily_" + Today(),
		Password:        "hash",
		Role:            RoleCommonUser,
		Status:          UserStatusEnabled,
		AccessToken:     "at-" + Today(),
		AffCode:         "aff-" + Today(),
		DailyTokenLimit: token,
		DailyQuotaLimit: quota,
	}
	if err := DB.Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	return user
}

func TestCheckUserDailyLimitTokenDimension(t *testing.T) {
	setupModelTestDB(t)
	withGlobalDailyLimits(t, 1000, 0)
	user := createUserWithDailyLimits(t, nil, nil)

	// 未超限
	if err := CheckUserDailyLimit(user.Id, 400, 0); err != nil {
		t.Fatalf("未超限不应拒绝: %v", err)
	}
	// 累加真实用量 900
	if err := RecordDailyUsage(user.Id, 900, 0, 0); err != nil {
		t.Fatalf("记账失败: %v", err)
	}
	// 恰好等于上限：放行
	if err := CheckUserDailyLimit(user.Id, 100, 0); err != nil {
		t.Fatalf("恰好等于上限应放行: %v", err)
	}
	// 超出 1 个 token：拒绝
	err := CheckUserDailyLimit(user.Id, 101, 0)
	if err == nil {
		t.Fatal("超出上限应拒绝")
	}
	limitErr, ok := AsDailyLimitError(err)
	if !ok {
		t.Fatalf("应为 *DailyLimitError，实际 %T: %v", err, err)
	}
	if limitErr.Dimension != "token" || limitErr.Used != 900 || limitErr.Limit != 1000 || limitErr.Estimated != 101 {
		t.Fatalf("错误详情不对: %+v", limitErr)
	}
}

func TestCheckUserDailyLimitQuotaDimension(t *testing.T) {
	setupModelTestDB(t)
	withGlobalDailyLimits(t, 0, 500)
	user := createUserWithDailyLimits(t, nil, nil)

	if err := RecordDailyUsage(user.Id, 999, 999, 480); err != nil {
		t.Fatalf("记账失败: %v", err)
	}
	// token 维度不限，即使 token 早已远超也不该拒绝
	if err := CheckUserDailyLimit(user.Id, 100000, 20); err != nil {
		t.Fatalf("token 维度不限时不应拒绝: %v", err)
	}
	err := CheckUserDailyLimit(user.Id, 0, 21)
	if err == nil {
		t.Fatal("额度超出应拒绝")
	}
	limitErr, _ := AsDailyLimitError(err)
	if limitErr.Dimension != "quota" {
		t.Fatalf("维度应为 quota，实际 %s", limitErr.Dimension)
	}
}

func TestCheckUserDailyLimitExemptAndCustom(t *testing.T) {
	setupModelTestDB(t)
	withGlobalDailyLimits(t, 100, 100)

	exempt := createUserWithDailyLimits(t, ptr(-1), ptr(-1))
	if err := RecordDailyUsage(exempt.Id, 100000, 100000, 100000); err != nil {
		t.Fatalf("记账失败: %v", err)
	}
	if err := CheckUserDailyLimit(exempt.Id, 100000, 100000); err != nil {
		t.Fatalf("豁免用户不应被拒绝: %v", err)
	}

	custom := createUserWithDailyLimits(t, ptr(50), ptr(0))
	if err := CheckUserDailyLimit(custom.Id, 40, 100); err != nil {
		t.Fatalf("token 走单独上限 50 时 40 应放行: %v", err)
	}
	if err := CheckUserDailyLimit(custom.Id, 60, 100); err == nil {
		t.Fatal("token 超过单独上限 50 应拒绝（即使低于全局 100）")
	}
	// 额度维度显式 0 = 跟随全局 100
	if err := CheckUserDailyLimit(custom.Id, 10, 90); err != nil {
		t.Fatalf("额度跟随全局 100 时 90 应放行: %v", err)
	}
	if err := CheckUserDailyLimit(custom.Id, 10, 110); err == nil {
		t.Fatal("额度超过全局 100 应拒绝")
	}
}

func TestCheckUserDailyLimitDisabledByDefault(t *testing.T) {
	setupModelTestDB(t)
	withGlobalDailyLimits(t, 0, 0)
	user := createUserWithDailyLimits(t, nil, nil)

	if err := RecordDailyUsage(user.Id, 999999999, 999999999, 999999999); err != nil {
		t.Fatalf("记账失败: %v", err)
	}
	if err := CheckUserDailyLimit(user.Id, 999999999, 999999999); err != nil {
		t.Fatalf("全局与用户都不限时不应拒绝: %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./model/ -run TestCheckUserDailyLimit -v`
Expected: 编译失败，报 `undefined: CheckUserDailyLimit`、`undefined: AsDailyLimitError`

- [ ] **Step 3: 实现**

在 `model/daily_usage.go` 追加（import 需补 `fmt`、`github.com/songquanpeng/one-api/common`、`github.com/songquanpeng/one-api/common/config`）：

```go
// DailyLimitState 是判定一次请求是否超限所需的全部状态，用一条 SQL 取回。
type DailyLimitState struct {
	DailyTokenLimit *int64
	DailyQuotaLimit *int64
	UsedTokens      int64
	UsedQuota       int64
}

// GetDailyLimitState 一次查询同时取出用户的两个日限设置与今天的累计用量（LEFT JOIN，最多一行）。
func GetDailyLimitState(userId int, day string) (*DailyLimitState, error) {
	state := &DailyLimitState{}
	err := DB.Raw(`
		SELECT u.daily_token_limit,
		       u.daily_quota_limit,
		       COALESCE(d.prompt_tokens, 0) + COALESCE(d.completion_tokens, 0) AS used_tokens,
		       COALESCE(d.quota, 0) AS used_quota
		FROM users u
		LEFT JOIN daily_usage d ON d.user_id = u.id AND d.day = ?
		WHERE u.id = ?
		LIMIT 1
	`, day, userId).Row().Scan(
		&state.DailyTokenLimit,
		&state.DailyQuotaLimit,
		&state.UsedTokens,
		&state.UsedQuota,
	)
	if err != nil {
		return nil, err
	}
	return state, nil
}

// DailyLimitError 表示单日用量已达上限。Dimension 取 "token" 或 "quota"。
type DailyLimitError struct {
	Dimension string
	Used      int64
	Limit     int64
	Estimated int64
}

func (e *DailyLimitError) Error() string {
	if e.Dimension == "quota" {
		return fmt.Sprintf("今日额度用量已达上限（已用 %s / 上限 %s），请明日再试或联系管理员调整",
			common.LogQuota(e.Used), common.LogQuota(e.Limit))
	}
	return fmt.Sprintf("今日 token 用量已达上限（已用 %d / 上限 %d），请明日再试或联系管理员调整",
		e.Used, e.Limit)
}

// AsDailyLimitError 判断 err 是否为单日上限错误，并取出详情。
func AsDailyLimitError(err error) (*DailyLimitError, bool) {
	var target *DailyLimitError
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}

// CheckUserDailyLimit 判断"今日已用 + 本次预估"是否超过生效上限。返回 0 表示不限时不拦截。
// 调用方必须在额度预扣之前调用，且传短路前的原始估算值。
func CheckUserDailyLimit(userId int, addTokens int64, addQuota int64) error {
	state, err := GetDailyLimitState(userId, Today())
	if err != nil {
		return err
	}
	tokenLimit := resolveDailyLimit(state.DailyTokenLimit, config.DailyTokenLimitDefault)
	quotaLimit := resolveDailyLimit(state.DailyQuotaLimit, config.DailyQuotaLimitDefault)
	if tokenLimit > 0 && state.UsedTokens+addTokens > tokenLimit {
		return &DailyLimitError{
			Dimension: "token",
			Used:      state.UsedTokens,
			Limit:     tokenLimit,
			Estimated: addTokens,
		}
	}
	if quotaLimit > 0 && state.UsedQuota+addQuota > quotaLimit {
		return &DailyLimitError{
			Dimension: "quota",
			Used:      state.UsedQuota,
			Limit:     quotaLimit,
			Estimated: addQuota,
		}
	}
	return nil
}
```

注意：这里刻意**不做**"两个全局默认都是 0 就跳过查询"的短路优化——用户级覆盖可能存在，跳过会漏判。代价是每个转发请求恒定多一次点查，已在设计文档「已知取舍」中记录。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./model/ -run TestCheckUserDailyLimit -v`
Expected: 4 个测试全部 PASS

- [ ] **Step 5: 提交**

```bash
go build ./... && go test ./model/
git add model/daily_usage.go model/daily_limit_check_test.go
git commit -m "feat: 新增单日上限判定与带具体数字的错误类型"
```

---

### Task A5: 接入四个模态的准入判定与结算记账

**Files:**
- Modify: `relay/controller/helper.go`（`preConsumeQuota` 68-95 行；`postConsumeQuota` 97-141 行）
- Modify: `relay/controller/anthropic.go`（`preConsumeQuotaAnthropic` 约 101-122 行；`postConsumeQuotaAnthropic` 约 124-140 行）
- Modify: `relay/controller/audio.go`（约 60-95 行）
- Modify: `relay/controller/image.go`（约 186-190 行与 defer 内 `if quota != 0` 块）
- Modify: `relay/billing/billing.go`（`PostConsumeQuota`）

**Interfaces:**
- Consumes: `model.CheckUserDailyLimit`、`model.AsDailyLimitError`、`model.RecordDailyUsage`（A1/A4）
- Produces: `func dailyLimitError(err error) *relaymodel.ErrorWithStatusCode`（`relay/controller` 包内共享）

- [ ] **Step 1: 加错误映射辅助函数并接入文本路径**

在 `relay/controller/helper.go` 的 `getPreConsumedQuota` 之后插入：

```go
// dailyLimitError 把 model 层的单日上限错误映射成 relay 错误：维度决定错误码，统一 403。
func dailyLimitError(err error) *relaymodel.ErrorWithStatusCode {
	limitErr, ok := model.AsDailyLimitError(err)
	if !ok {
		return openai.ErrorWrapper(err, "check_daily_limit_failed", http.StatusInternalServerError)
	}
	code := "insufficient_user_daily_tokens"
	if limitErr.Dimension == "quota" {
		code = "insufficient_user_daily_quota"
	}
	return openai.ErrorWrapper(limitErr, code, http.StatusForbidden)
}
```

修改 `preConsumeQuota`，在 `preConsumedQuota := getPreConsumedQuota(...)` 之后、`userQuota, err := model.CacheGetUserQuota(...)` 之前插入：

```go
	// 日限判定必须在额度预扣之前，也必须用短路前的原始估算值：
	// 下面的 "userQuota > 100*preConsumedQuota" 分支会把 preConsumedQuota 置 0，
	// 拿置 0 后的值做判定会让额度充裕的用户永远不触发上限。
	estimatedTokens := int64(promptTokens) + int64(textRequest.MaxTokens)
	if err := model.CheckUserDailyLimit(meta.UserId, estimatedTokens, preConsumedQuota); err != nil {
		return preConsumedQuota, dailyLimitError(err)
	}
```

修改 `postConsumeQuota`，在 `model.UpdateChannelUsedQuota(meta.ChannelId, quota)` 之后追加：

```go
	// 单日用量记账：与日志开关无关，必须无条件写入
	if err := model.RecordDailyUsage(meta.UserId, int64(promptTokens), int64(completionTokens), quota); err != nil {
		logger.Error(ctx, "error recording daily usage: "+err.Error())
	}
```

- [ ] **Step 2: 接入 Anthropic 路径**

`relay/controller/anthropic.go` 的 `preConsumeQuotaAnthropic`，在 `preConsumedQuota := int64(float64(preConsumedTokens) * ratio)` 之后插入：

```go
	estimatedTokens := int64(promptTokens) + int64(maxTokens)
	if err := model.CheckUserDailyLimit(meta.UserId, estimatedTokens, preConsumedQuota); err != nil {
		return preConsumedQuota, dailyLimitError(err)
	}
```

`postConsumeQuotaAnthropic` 中，在 `model.UpdateChannelUsedQuota(meta.ChannelId, quota)` 之后追加：

```go
	if err := model.RecordDailyUsage(meta.UserId, int64(usage.PromptTokens), int64(usage.CompletionTokens), quota); err != nil {
		logger.Error(ctx, "error recording daily usage: "+err.Error())
	}
```

（该文件已导入 `logger`；若未被引用请确认 import 存在，`postConsumeQuotaAnthropic` 里其它地方可能已使用。）

- [ ] **Step 3: 接入 Audio 路径**

`relay/controller/audio.go` 中，`switch relayMode { ... }` 计算完 `preConsumedQuota` 之后、`userQuota, err := model.CacheGetUserQuota(ctx, userId)` 之前插入：

```go
	// 音频不返回真实 token，只受额度上限约束
	if err := model.CheckUserDailyLimit(userId, 0, preConsumedQuota); err != nil {
		return dailyLimitError(err)
	}
```

`relay/billing/billing.go` 的 `PostConsumeQuota` 中，在 `if totalQuota != 0 {` 块内的 `model.UpdateChannelUsedQuota(channelId, totalQuota)` 之后追加：

```go
		// 音频的 token 字段不可信（billing 把 quota 写进了 PromptTokens），只累计额度
		if err := model.RecordDailyUsage(userId, 0, 0, totalQuota); err != nil {
			logger.SysError("error recording daily usage: " + err.Error())
		}
```

- [ ] **Step 4: 接入 Image 路径**

`relay/controller/image.go` 中，`switch meta.ChannelType { ... }` 计算完 `quota` 之后、`if userQuota-quota < 0 {` 之前插入：

```go
	// 图片不返回真实 token，只受额度上限约束
	if err := model.CheckUserDailyLimit(meta.UserId, 0, quota); err != nil {
		return dailyLimitError(err)
	}
```

同文件 defer 内 `if quota != 0 {` 块的 `model.UpdateChannelUsedQuota(channelId, quota)` 之后追加：

```go
			if err := model.RecordDailyUsage(meta.UserId, 0, 0, quota); err != nil {
				logger.SysError("error recording daily usage: " + err.Error())
			}
```

（该文件 defer 内已有 `ctx` 与 `logger` 可用；若作用域内没有 `logger`，用 `logger.SysError` 需要确认 import 已存在。）

- [ ] **Step 5: 编译并确认调用点齐全**

Run:
```bash
go build ./... && go vet ./relay/...
grep -c "CheckUserDailyLimit" relay/controller/helper.go relay/controller/anthropic.go relay/controller/audio.go relay/controller/image.go
grep -c "RecordDailyUsage" relay/controller/helper.go relay/controller/anthropic.go relay/billing/billing.go relay/controller/image.go
```
Expected: 构建与 vet 通过；`CheckUserDailyLimit` 在四个文件各出现 ≥1 次；`RecordDailyUsage` 在四个结算点各出现 ≥1 次。

- [ ] **Step 6: 提交**

```bash
git add relay/controller/helper.go relay/controller/anthropic.go relay/controller/audio.go relay/controller/image.go relay/billing/billing.go
git commit -m "feat: 四个模态接入单日上限的准入判定与结算记账"
```

行为验证放在 Task A7 的端到端验收（relay 路径需要真实上游，仓库没有对应测试脚手架）。

---

### Task A6: 管理 API 的新字段校验与"改回 0"

**Files:**
- Modify: `controller/user.go`（`UpdateUser`，367-430 行）
- Test: `controller/user_daily_limit_test.go`

**Interfaces:**
- Consumes: `setupControllerTestDB(t)`（已存在于 `controller/data_test.go`，需要把 `&model.DailyUsage{}` 加进它的 AutoMigrate 列表）
- Produces: 无新导出符号；`PUT /api/user/` 接受 `daily_token_limit`/`daily_quota_limit`，拒绝 `< -1`

- [ ] **Step 1: 扩展测试库迁移列表**

修改 `controller/data_test.go` 的 `setupControllerTestDB`，把 AutoMigrate 调用改为：

```go
	if err := db.AutoMigrate(&model.User{}, &model.Token{}, &model.Channel{}, &model.Redemption{}, &model.Option{}, &model.Ability{}, &model.DailyUsage{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
```

- [ ] **Step 2: 写失败的测试**

创建 `controller/user_daily_limit_test.go`：

```go
package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/model"
)

// 用中间件注入会话身份，绕开真的登录流程。
func withIdentity(id int, role int) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ctxkey.Id, id)
		c.Set(ctxkey.Role, role)
	}
}

func putUser(t *testing.T, id int, role int, body map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	r := gin.New()
	r.PUT("/api/user/", withIdentity(id, role), UpdateUser)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/user/", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestUpdateUserDailyLimits(t *testing.T) {
	setupControllerTestDB(t)
	root := &model.User{Username: "root", Password: "hash", Role: model.RoleRootUser, Status: 1, Quota: 100, AccessToken: "at-root", AffCode: "aff-root"}
	if err := model.DB.Create(root).Error; err != nil {
		t.Fatalf("创建 root 失败: %v", err)
	}
	target := &model.User{Username: "bob", Password: "hash", Role: model.RoleCommonUser, Status: 1, Quota: 100, AccessToken: "at-bob", AffCode: "aff-bob"}
	if err := model.DB.Create(target).Error; err != nil {
		t.Fatalf("创建目标用户失败: %v", err)
	}

	// 设置单独上限
	putUser(t, root.Id, root.Role, map[string]interface{}{"id": target.Id, "daily_token_limit": 5000, "daily_quota_limit": -1})
	var got model.User
	if err := model.DB.First(&got, target.Id).Error; err != nil {
		t.Fatalf("重新读取失败: %v", err)
	}
	if got.DailyTokenLimit == nil || *got.DailyTokenLimit != 5000 {
		t.Fatalf("daily_token_limit 未写入: %v", got.DailyTokenLimit)
	}
	if got.DailyQuotaLimit == nil || *got.DailyQuotaLimit != -1 {
		t.Fatalf("daily_quota_limit 未写入: %v", got.DailyQuotaLimit)
	}

	// 改回 0（跟随全局）——这是必须能生效的路径，GORM 结构体 Updates 会跳过零值
	putUser(t, root.Id, root.Role, map[string]interface{}{"id": target.Id, "daily_token_limit": 0})
	got = model.User{}
	if err := model.DB.First(&got, target.Id).Error; err != nil {
		t.Fatalf("重新读取失败: %v", err)
	}
	if got.DailyTokenLimit == nil || *got.DailyTokenLimit != 0 {
		t.Fatalf("daily_token_limit 改回 0 未生效: %v", got.DailyTokenLimit)
	}

	// 非法值被拒
	w := putUser(t, root.Id, root.Role, map[string]interface{}{"id": target.Id, "daily_token_limit": -2})
	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Success {
		t.Fatalf("daily_token_limit = -2 应被拒绝，响应: %s", w.Body.String())
	}
	got = model.User{}
	if err := model.DB.First(&got, target.Id).Error; err != nil {
		t.Fatalf("重新读取失败: %v", err)
	}
	if got.DailyTokenLimit == nil || *got.DailyTokenLimit != 0 {
		t.Fatalf("非法请求不应改动数据: %v", got.DailyTokenLimit)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./controller/ -run TestUpdateUserDailyLimits -v`
Expected: FAIL——非法值 `-2` 被接受（`resp.Success == true`），因为还没有校验

- [ ] **Step 4: 实现校验**

在 `controller/user.go` 的 `UpdateUser` 中，紧接角色校验（`if myRole <= updatedUser.Role && ...` 之后、`if updatedUser.Password == "$I_LOVE_U"` 之前）插入：

```go
	if (updatedUser.DailyTokenLimit != nil && *updatedUser.DailyTokenLimit < -1) ||
		(updatedUser.DailyQuotaLimit != nil && *updatedUser.DailyQuotaLimit < -1) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "单日上限只能是 -1（豁免）、0（跟随全局默认）或不小于 1 的整数",
		})
		return
	}
```

说明：`0` 能被写入是因为字段是 `*int64`——GORM 结构体 `Updates` 会跳过零值字段，但非 nil 指针本身不是零值，因此显式传 `0` 会正确落库。这条路径已由 Step 2 的第二个断言覆盖。

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./controller/ -run TestUpdateUserDailyLimits -v`
Expected: PASS

- [ ] **Step 6: 全量测试并提交**

```bash
go test ./...
git add controller/user.go controller/user_daily_limit_test.go controller/data_test.go
git commit -m "feat: 管理 API 支持单日上限三态字段并拒绝非法值"
```

---

### Task A7: 单日上限的端到端手工验收

**Files:** 无代码改动（发现问题时回到对应任务修复）

- [ ] **Step 1: 构建并启动**

```bash
CGO_ENABLED=1 go build -o one-api.exe .
./one-api.exe
```
（Windows Git Bash 下 SQLite 需要 CGO；若报驱动缺失，用 `go env -w CGO_ENABLED=1` 后重试。）

- [ ] **Step 2: 准备数据**

浏览器打开 `http://localhost:3000`，用 root 登录，确认：
- 渠道页有至少一个可用渠道（否则加一个上游渠道）
- 令牌页有一个令牌，复制其 `sk-` 开头的 key
- 用户页新建一个测试用户（如 `dailytest`），并给它充值少量额度
- 用新用户登录，令牌页复制它的令牌 key

- [ ] **Step 3: 给测试用户设一个极小的上限**

以 root 身份在「用户 → 编辑 dailytest」把"单日 Token 上限"设为自定义值 `100`，保存。

如果此时 default 主题的用户编辑页还没改（Part C 未执行），改用 API 验证：

```bash
# 系统访问令牌在 个人设置 → 生成系统访问令牌
curl -s -X PUT http://localhost:3000/api/user/ \
  -H "Authorization: Bearer <系统访问令牌>" -H "Content-Type: application/json" \
  -d '{"id":<测试用户 id>,"daily_token_limit":100}'
```

- [ ] **Step 4: 验证拦截**

```bash
curl -s -i http://localhost:3000/v1/chat/completions \
  -H "Authorization: Bearer <测试用户的令牌 key>" -H "Content-Type: application/json" \
  -d '{"model":"<可用模型名>","messages":[{"role":"user","content":"你好"}],"max_tokens":50}'
```
预期：
- 第一次请求正常返回（或上游报错，但不是日限错误）
- 累计用量超过 100 后，后续请求返回 HTTP 403，响应体 `error.code` 为 `insufficient_user_daily_tokens`，`error.message` 形如 `今日 token 用量已达上限（已用 128 / 上限 100），请明日再试或联系管理员调整`

- [ ] **Step 5: 验证豁免**

把该用户的"单日 Token 上限"设为豁免（`-1`）：
```bash
curl -s -X PUT http://localhost:3000/api/user/ \
  -H "Authorization: Bearer <系统访问令牌>" -H "Content-Type: application/json" \
  -d '{"id":<测试用户 id>,"daily_token_limit":-1}'
```
再次发送请求，预期不再被日限拦截。

- [ ] **Step 6: 验证记账数据**

```bash
# 有 sqlite3 命令时（注意表名是 daily_usage，不是 daily_usages）
sqlite3 one-api.db "select * from daily_usage;"
```
本地没有 `sqlite3` 命令时，用任意 SQLite 客户端（DB Browser for SQLite / DBeaver）打开 `one-api.db` 执行同一条查询；或在 Part C 完成后直接看 用户 页面的「今日用量」列——它读的就是同一张表。

预期：测试用户有一行今天的记录，`prompt_tokens + completion_tokens` 与日志页该用户今天消耗的 token 数一致，`quota` 与日志页该用户今天的额度消耗一致。

- [ ] **Step 7: 记录结果**

在本计划文件末尾追加一行验收记录（日期 + 结果 + 遇到的偏差），然后：

```bash
git add docs/superpowers/plans/2026-09-20-daily-quota-and-dashboard-filters.md
git commit -m "docs: 记录单日上限的端到端验收结果"
```

---

# Part B：总览查询与候选值接口（后端）

### Task B1: 聚合查询支持区间、粒度与三维筛选

**Files:**
- Modify: `model/log.go`（`SearchLogsByDayAndModel`，225-251 行；`LogStatistic` 结构体保持不变）
- Test: `model/log_statistic_test.go`

**Interfaces:**
- Produces:
  - `const LogGranularityDay = "day"` / `const LogGranularityHour = "hour"`
  - `type LogStatisticQuery struct{ UserId int; Username, TokenName, ModelName, Granularity string; StartTimestamp, EndTimestamp int64 }`
  - `func SearchLogsByDayAndModel(query LogStatisticQuery) ([]*LogStatistic, error)`（替换原三参版本，唯一调用点在 `controller/user.go:268`，由 Task B3 更新）
- Consumes: `LOG_DB`、`common.UsingSQLite/UsingPostgreSQL`、`LogTypeConsume`

- [ ] **Step 1: 写失败的测试**

创建 `model/log_statistic_test.go`：

```go
package model

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/songquanpeng/one-api/common"
)

// setupLogTestDB 用临时 sqlite 作为 LOG_DB，并强制 UsingSQLite 走本地时间分桶分支。
func setupLogTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "logs.db")
	dsn := fmt.Sprintf("%s?_busy_timeout=5000&_journal_mode=WAL", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开日志测试库失败: %v", err)
	}
	if err := db.AutoMigrate(&Log{}); err != nil {
		t.Fatalf("迁移日志表失败: %v", err)
	}
	oldLogDB, oldSQLite, oldPG, oldMySQL := LOG_DB, common.UsingSQLite, common.UsingPostgreSQL, common.UsingMySQL
	LOG_DB = db
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.UsingMySQL = false
	t.Cleanup(func() {
		LOG_DB = oldLogDB
		common.UsingSQLite = oldSQLite
		common.UsingPostgreSQL = oldPG
		common.UsingMySQL = oldMySQL
	})
	// Windows 上必须先释放 sqlite 文件句柄，否则 t.TempDir 的清理会报
	// "The process cannot access the file because it is being used by another process"。
	if sqlDB, err := db.DB(); err == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	return db
}

// localTime 返回本地时区指定时刻的时间戳。
func localTime(y int, mo time.Month, d, h, mi, s int) int64 {
	return time.Date(y, mo, d, h, mi, s, 0, time.Local).Unix()
}

func TestSearchLogsByDayAndModelSplitsLocalDayBoundary(t *testing.T) {
	db := setupLogTestDB(t)

	rows := []Log{
		{UserId: 1, Username: "alice", TokenName: "t1", ModelName: "gpt-3.5-turbo", Type: LogTypeConsume,
			CreatedAt: localTime(2026, 3, 10, 23, 59, 0), PromptTokens: 10, CompletionTokens: 5, Quota: 100},
		{UserId: 1, Username: "alice", TokenName: "t1", ModelName: "gpt-3.5-turbo", Type: LogTypeConsume,
			CreatedAt: localTime(2026, 3, 11, 0, 1, 0), PromptTokens: 20, CompletionTokens: 7, Quota: 200},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("写入日志失败: %v", err)
	}

	got, err := SearchLogsByDayAndModel(LogStatisticQuery{
		UserId:         1,
		StartTimestamp: localTime(2026, 3, 10, 0, 0, 0),
		EndTimestamp:   localTime(2026, 3, 11, 23, 59, 59),
		Granularity:    LogGranularityDay,
	})
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("应分成两个日桶，实际 %d 行: %+v", len(got), got)
	}
	// 23:59 必须落在 3-10，00:01 必须落在 3-11 —— 这是本地时区分桶的核心断言
	if got[0].Day != "2026-03-10" || got[0].PromptTokens != 10 {
		t.Fatalf("第一个桶错误: %+v", got[0])
	}
	if got[1].Day != "2026-03-11" || got[1].PromptTokens != 20 {
		t.Fatalf("第二个桶错误: %+v", got[1])
	}
}

func TestSearchLogsByDayAndModelHourGranularity(t *testing.T) {
	db := setupLogTestDB(t)

	rows := []Log{
		{UserId: 1, Username: "alice", TokenName: "t1", ModelName: "gpt-3.5-turbo", Type: LogTypeConsume,
			CreatedAt: localTime(2026, 3, 10, 14, 5, 0), PromptTokens: 1, Quota: 1},
		{UserId: 1, Username: "alice", TokenName: "t1", ModelName: "gpt-3.5-turbo", Type: LogTypeConsume,
			CreatedAt: localTime(2026, 3, 10, 14, 55, 0), PromptTokens: 2, Quota: 2},
		{UserId: 1, Username: "alice", TokenName: "t1", ModelName: "gpt-3.5-turbo", Type: LogTypeConsume,
			CreatedAt: localTime(2026, 3, 10, 15, 0, 0), PromptTokens: 4, Quota: 4},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("写入日志失败: %v", err)
	}

	got, err := SearchLogsByDayAndModel(LogStatisticQuery{
		UserId:         1,
		StartTimestamp: localTime(2026, 3, 10, 0, 0, 0),
		EndTimestamp:   localTime(2026, 3, 10, 23, 59, 59),
		Granularity:    LogGranularityHour,
	})
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("应分成两个小时桶，实际 %d 行: %+v", len(got), got)
	}
	if got[0].Day != "2026-03-10 14:00" || got[0].PromptTokens != 3 {
		t.Fatalf("14 点桶错误: %+v", got[0])
	}
	if got[1].Day != "2026-03-10 15:00" || got[1].PromptTokens != 4 {
		t.Fatalf("15 点桶错误: %+v", got[1])
	}
}

func TestSearchLogsByDayAndModelFilters(t *testing.T) {
	db := setupLogTestDB(t)

	rows := []Log{
		{UserId: 1, Username: "alice", TokenName: "t1", ModelName: "gpt-3.5-turbo", Type: LogTypeConsume, CreatedAt: localTime(2026, 3, 10, 10, 0, 0), PromptTokens: 1},
		{UserId: 2, Username: "bob", TokenName: "t2", ModelName: "gpt-3.5-turbo", Type: LogTypeConsume, CreatedAt: localTime(2026, 3, 10, 10, 0, 0), PromptTokens: 2},
		{UserId: 2, Username: "bob", TokenName: "t2", ModelName: "claude-3", Type: LogTypeConsume, CreatedAt: localTime(2026, 3, 10, 10, 0, 0), PromptTokens: 4},
		{UserId: 2, Username: "bob", TokenName: "t3", ModelName: "claude-3", Type: LogTypeConsume, CreatedAt: localTime(2026, 3, 10, 10, 0, 0), PromptTokens: 8},
		{UserId: 2, Username: "bob", TokenName: "t3", ModelName: "claude-3", Type: LogTypeTopup, CreatedAt: localTime(2026, 3, 10, 10, 0, 0), PromptTokens: 999},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("写入日志失败: %v", err)
	}
	window := LogStatisticQuery{
		StartTimestamp: localTime(2026, 3, 10, 0, 0, 0),
		EndTimestamp:   localTime(2026, 3, 10, 23, 59, 59),
		Granularity:    LogGranularityDay,
	}

	// 全站（UserId=0）只统计消费类日志，充值日志不参与
	all, err := SearchLogsByDayAndModel(window)
	if err != nil {
		t.Fatalf("全站查询失败: %v", err)
	}
	total := 0
	for _, row := range all {
		total += row.PromptTokens
	}
	if total != 15 {
		t.Fatalf("全站 token 合计 = %d，期望 15（1+2+4+8，不含充值 999）", total)
	}

	// 按用户收窄
	byUser := window
	byUser.UserId = 1
	got, err := SearchLogsByDayAndModel(byUser)
	if err != nil {
		t.Fatalf("按用户查询失败: %v", err)
	}
	if len(got) != 1 || got[0].PromptTokens != 1 {
		t.Fatalf("按用户收窄结果错误: %+v", got)
	}

	// 按用户名收窄
	byName := window
	byName.Username = "bob"
	got, err = SearchLogsByDayAndModel(byName)
	if err != nil {
		t.Fatalf("按用户名查询失败: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("bob 应有两个模型的桶，实际 %+v", got)
	}

	// 按令牌收窄
	byToken := window
	byToken.TokenName = "t3"
	got, err = SearchLogsByDayAndModel(byToken)
	if err != nil {
		t.Fatalf("按令牌查询失败: %v", err)
	}
	if len(got) != 1 || got[0].PromptTokens != 8 {
		t.Fatalf("按令牌收窄结果错误: %+v", got)
	}

	// 按模型收窄
	byModel := window
	byModel.ModelName = "gpt-3.5-turbo"
	got, err = SearchLogsByDayAndModel(byModel)
	if err != nil {
		t.Fatalf("按模型查询失败: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("gpt-3.5-turbo 应有两条记录，实际 %+v", got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./model/ -run TestSearchLogsByDayAndModel -v`
Expected: 编译失败，报 `unknown field Granularity`、`too many arguments`

- [ ] **Step 3: 实现**

替换 `model/log.go` 中现有的 `SearchLogsByDayAndModel`（225-251 行）为：

```go
const (
	LogGranularityDay  = "day"
	LogGranularityHour = "hour"
)

// LogStatisticQuery 描述一次日志聚合查询。UserId 为 0 表示不限用户（调用方必须先完成权限校验）。
type LogStatisticQuery struct {
	UserId         int
	Username       string
	TokenName      string
	ModelName      string
	StartTimestamp int64
	EndTimestamp   int64
	Granularity    string
}

// logBucketSelect 返回按服务器本地时间分桶的 SQL 表达式。
// 三个方言都按本地时间分桶：MySQL 的 FROM_UNIXTIME 走会话时区，PostgreSQL 的
// date_trunc 走会话 TimeZone，SQLite 默认是 UTC，必须显式加 'localtime' 修饰符，
// 否则同一份数据在 sqlite 与 mysql 下会落到不同桶里。
func logBucketSelect(granularity string) string {
	hour := granularity == LogGranularityHour
	switch {
	case common.UsingPostgreSQL:
		if hour {
			return "TO_CHAR(date_trunc('hour', to_timestamp(created_at)), 'YYYY-MM-DD HH24:00')"
		}
		return "TO_CHAR(date_trunc('day', to_timestamp(created_at)), 'YYYY-MM-DD')"
	case common.UsingSQLite:
		if hour {
			return "strftime('%Y-%m-%d %H:00', datetime(created_at, 'unixepoch', 'localtime'))"
		}
		return "strftime('%Y-%m-%d', datetime(created_at, 'unixepoch', 'localtime'))"
	default:
		if hour {
			return "DATE_FORMAT(FROM_UNIXTIME(created_at), '%Y-%m-%d %H:00')"
		}
		return "DATE_FORMAT(FROM_UNIXTIME(created_at), '%Y-%m-%d')"
	}
}

// SearchLogsByDayAndModel 按桶（天或小时）× 模型聚合消费日志。
// 桶标签仍放在 LogStatistic.Day 字段里，前端不必区分粒度。
func SearchLogsByDayAndModel(query LogStatisticQuery) (LogStatistics []*LogStatistic, err error) {
	sql := `
		SELECT ` + logBucketSelect(query.Granularity) + ` AS day,
		model_name, count(1) as request_count,
		sum(quota) as quota,
		sum(prompt_tokens) as prompt_tokens,
		sum(completion_tokens) as completion_tokens
		FROM logs
		WHERE type = ?`
	args := []interface{}{LogTypeConsume}
	if query.UserId != 0 {
		sql += ` AND user_id = ?`
		args = append(args, query.UserId)
	}
	if query.Username != "" {
		sql += ` AND username = ?`
		args = append(args, query.Username)
	}
	if query.TokenName != "" {
		sql += ` AND token_name = ?`
		args = append(args, query.TokenName)
	}
	if query.ModelName != "" {
		sql += ` AND model_name = ?`
		args = append(args, query.ModelName)
	}
	sql += ` AND created_at BETWEEN ? AND ?
		GROUP BY day, model_name
		ORDER BY day, model_name`
	args = append(args, query.StartTimestamp, query.EndTimestamp)

	err = LOG_DB.Raw(sql, args...).Scan(&LogStatistics).Error
	return LogStatistics, err
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./model/ -run TestSearchLogsByDayAndModel -v`
Expected: 3 个测试全部 PASS

注意：此时 `controller/user.go:268` 仍在调用旧的三参版本，`go build ./...` 会失败。这是预期的中间状态，Task B3 修复；提交前先完成 B3，或本任务只提交 model 层改动并接受 `go vet ./model/` 单独通过。

- [ ] **Step 5: 提交**（与 Task B3 一起，见 B3 Step 6）

---

### Task B2: 候选值查询

**Files:**
- Modify: `model/log.go`
- Test: `model/log_distinct_test.go`

**Interfaces:**
- Produces: `func SearchLogDistinctValues(userId int, username string, startTimestamp, endTimestamp int64, field string, limit int) ([]string, error)`
- Consumes: `LOG_DB`

- [ ] **Step 1: 写失败的测试**

创建 `model/log_distinct_test.go`（复用 B1 的 `setupLogTestDB` 与 `localTime`）：

```go
package model

import (
	"testing"
)

func TestSearchLogDistinctValues(t *testing.T) {
	db := setupLogTestDB(t)

	rows := []Log{
		{UserId: 1, Username: "alice", TokenName: "t1", ModelName: "gpt-3.5-turbo", Type: LogTypeConsume, CreatedAt: localTime(2026, 3, 10, 10, 0, 0)},
		{UserId: 2, Username: "bob", TokenName: "t2", ModelName: "claude-3", Type: LogTypeConsume, CreatedAt: localTime(2026, 3, 10, 11, 0, 0)},
		{UserId: 2, Username: "bob", TokenName: "t2", ModelName: "claude-3", Type: LogTypeConsume, CreatedAt: localTime(2026, 3, 10, 12, 0, 0)},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("写入日志失败: %v", err)
	}
	start, end := localTime(2026, 3, 10, 0, 0, 0), localTime(2026, 3, 10, 23, 59, 59)

	// 全站用户名去重
	users, err := SearchLogDistinctValues(0, "", start, end, "username", 500)
	if err != nil {
		t.Fatalf("查询用户名失败: %v", err)
	}
	if len(users) != 2 || users[0] != "alice" || users[1] != "bob" {
		t.Fatalf("用户名去重结果错误: %v", users)
	}

	// 按用户收窄
	tokens, err := SearchLogDistinctValues(1, "", start, end, "token_name", 500)
	if err != nil {
		t.Fatalf("查询令牌名失败: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "t1" {
		t.Fatalf("用户 1 的令牌候选错误: %v", tokens)
	}

	// 按用户名收窄令牌
	tokens, err = SearchLogDistinctValues(0, "bob", start, end, "token_name", 500)
	if err != nil {
		t.Fatalf("按用户名收窄令牌失败: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "t2" {
		t.Fatalf("bob 的令牌候选错误: %v", tokens)
	}

	// 非白名单字段必须被拒绝
	if _, err := SearchLogDistinctValues(0, "", start, end, "password", 500); err == nil {
		t.Fatal("非白名单字段应报错")
	}

	// 时间区间之外的值不应出现
	otherUsers, err := SearchLogDistinctValues(0, "", localTime(2020, 1, 1, 0, 0, 0), localTime(2020, 1, 2, 0, 0, 0), "username", 500)
	if err != nil {
		t.Fatalf("区间外查询失败: %v", err)
	}
	if len(otherUsers) != 0 {
		t.Fatalf("区间外应无候选值，实际 %v", otherUsers)
	}
}

func TestSearchLogDistinctValuesRespectsLimit(t *testing.T) {
	db := setupLogTestDB(t)

	for i := 0; i < 5; i++ {
		row := Log{UserId: 1, Username: "u" + string(rune('a'+i)), ModelName: "m", Type: LogTypeConsume, CreatedAt: localTime(2026, 3, 10, 10, i, 0)}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("写入日志失败: %v", err)
		}
	}
	got, err := SearchLogDistinctValues(0, "", localTime(2026, 3, 10, 0, 0, 0), localTime(2026, 3, 10, 23, 59, 59), "username", 3)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("应被 LIMIT 3 截断，实际 %d 条", len(got))
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./model/ -run TestSearchLogDistinctValues -v`
Expected: 编译失败，`undefined: SearchLogDistinctValues`

- [ ] **Step 3: 实现**

在 `model/log.go` 追加：

```go
// logDistinctFields 是允许作为候选值维度查询的列白名单，避免调用方拼接任意列名。
var logDistinctFields = map[string]bool{
	"username":   true,
	"token_name": true,
	"model_name": true,
}

// SearchLogDistinctValues 返回某时间区间内日志里实际出现过的去重取值，用于筛选下拉候选值。
// userId 为 0 表示不限用户；username 非空时会进一步收窄（用于列出某用户的令牌）。
// 不按 type 过滤：候选值应当与日志列表默认（全部类型）的视图一致。
func SearchLogDistinctValues(userId int, username string, startTimestamp, endTimestamp int64, field string, limit int) ([]string, error) {
	if !logDistinctFields[field] {
		return nil, fmt.Errorf("不支持的候选值字段: %s", field)
	}
	if limit <= 0 {
		limit = 500
	}
	tx := LOG_DB.Table("logs").Distinct(field)
	if userId != 0 {
		tx = tx.Where("user_id = ?", userId)
	}
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	values := make([]string, 0)
	err := tx.Order(field + " asc").Limit(limit).Pluck(field, &values).Error
	return values, err
}
```

若 `model/log.go` 未导入 `fmt`，加入 import。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./model/ -run TestSearchLogDistinctValues -v`
Expected: 2 个测试 PASS

- [ ] **Step 5: 提交**（与 B1、B3 一起，见 B3 Step 6）

---

### Task B3: `/api/user/dashboard` 参数、权限与区间校验

**Files:**
- Modify: `controller/user.go`（`GetUserDashboard`，262-283 行）
- Test: `controller/dashboard_test.go`

**Interfaces:**
- Consumes: `model.LogStatisticQuery`、`model.SearchLogsByDayAndModel`（B1）、`ctxkey.Id`/`ctxkey.Role`、`model.RoleAdminUser`
- Produces: 处理 `start_timestamp`、`end_timestamp`、`granularity`、`scope`、`username`、`token_name`、`model_name` 参数

- [ ] **Step 1: 写失败的测试**

创建 `controller/dashboard_test.go`（复用 `withIdentity`，来自 Task A6）：

```go
package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func getDashboard(t *testing.T, id, role int, query string) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.GET("/api/user/dashboard", withIdentity(id, role), GetUserDashboard)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/user/dashboard"+query, nil)
	r.ServeHTTP(w, req)
	return w
}

func decodeDashboard(t *testing.T, w *httptest.ResponseRecorder) (bool, string) {
	t.Helper()
	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v，body=%s", err, w.Body.String())
	}
	return resp.Success, resp.Message
}

func TestDashboardRejectsAllScopeForNonAdmin(t *testing.T) {
	setupControllerTestDB(t)

	w := getDashboard(t, 1, model.RoleCommonUser, "?scope=all")
	if w.Code != http.StatusForbidden {
		t.Fatalf("非管理员请求 scope=all 应 403，实际 %d，body=%s", w.Code, w.Body.String())
	}
}

func TestDashboardRejectsInvalidRange(t *testing.T) {
	setupControllerTestDB(t)

	w := getDashboard(t, 1, model.RoleCommonUser, "?start_timestamp=2000&end_timestamp=1000")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("起始晚于结束应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	success, message := decodeDashboard(t, w)
	if success || message != "无效的时间范围" {
		t.Fatalf("响应不符: success=%v message=%q", success, message)
	}
}

func TestDashboardRejectsTooLongHourRange(t *testing.T) {
	setupControllerTestDB(t)

	start := int64(1000000)
	end := start + 100*86400 // 100 天，超过 hour 粒度 90 天上限
	w := getDashboard(t, 1, model.RoleCommonUser, "?start_timestamp="+itoa(start)+"&end_timestamp="+itoa(end)+"&granularity=hour")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("小时粒度超长区间应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
```

在文件 import 中加入 `"strconv"` 与 `"github.com/songquanpeng/one-api/model"`。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./controller/ -run TestDashboard -v`
Expected: FAIL——非管理员 `scope=all` 目前返回 200（当前 handler 完全不看 role，也不看参数），超长区间也不会被拒

- [ ] **Step 3: 实现**

把 `controller/user.go` 的 `GetUserDashboard` 整体替换为：

```go
const (
	dashboardDefaultDays = 7
	dashboardMaxDays     = 366
	dashboardMaxHourDays = 90
)

func GetUserDashboard(c *gin.Context) {
	now := time.Now()
	// 默认区间用本地时区的"今天"边界，与 SQL 的本地时区分桶保持一致
	defaultEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.Local).Unix()
	defaultStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).
		AddDate(0, 0, -(dashboardDefaultDays - 1)).Unix()

	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	if startTimestamp <= 0 {
		startTimestamp = defaultStart
	}
	if endTimestamp <= 0 {
		endTimestamp = defaultEnd
	}
	if startTimestamp > endTimestamp {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的时间范围",
			"data":    nil,
		})
		return
	}

	granularity := c.DefaultQuery("granularity", model.LogGranularityDay)
	if granularity != model.LogGranularityDay && granularity != model.LogGranularityHour {
		granularity = model.LogGranularityDay
	}
	maxDays := int64(dashboardMaxDays)
	if granularity == model.LogGranularityHour {
		maxDays = dashboardMaxHourDays
	}
	if endTimestamp-startTimestamp > maxDays*86400 {
		message := "天粒度最多查询 366 天"
		if granularity == model.LogGranularityHour {
			message = "小时粒度最多查询 90 天"
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": message,
			"data":    nil,
		})
		return
	}

	query := model.LogStatisticQuery{
		UserId:         c.GetInt(ctxkey.Id),
		TokenName:      c.Query("token_name"),
		ModelName:      c.Query("model_name"),
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		Granularity:    granularity,
	}

	// 只有管理员能请求全站视图；非管理员传了 username 也会被忽略（只能看自己）
	if c.DefaultQuery("scope", "self") == "all" {
		if c.GetInt(ctxkey.Role) < model.RoleAdminUser {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "无权查看全站统计",
				"data":    nil,
			})
			return
		}
		query.UserId = 0
		query.Username = c.Query("username")
	}

	dashboards, err := model.SearchLogsByDayAndModel(query)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无法获取统计信息",
			"data":    nil,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    dashboards,
	})
}
```

要确认 `controller/user.go` 已导入 `time`、`strconv`、`model`、`ctxkey`（原有代码已用到其中大部分）。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./controller/ -run TestDashboard -v`
Expected: 3 个测试 PASS

- [ ] **Step 5: 确认整体可编译**

Run: `go build ./... && go test ./model/ ./controller/`
Expected: 全部通过（此前的三参调用点已随本任务一起更新）

- [ ] **Step 6: 提交**

```bash
git add model/log.go model/log_statistic_test.go model/log_distinct_test.go controller/user.go controller/dashboard_test.go
git commit -m "feat: 总览支持时间段/粒度/全站范围与用户令牌模型筛选"
```

---

### Task B4: `/api/log/filters` 候选值接口

**Files:**
- Modify: `controller/log.go`
- Modify: `router/api.go`（log 路由组，约 109-116 行）
- Test: `controller/log_filters_test.go`

**Interfaces:**
- Consumes: `model.SearchLogDistinctValues`（B2）
- Produces: `func GetLogFilters(c *gin.Context)`；路由 `GET /api/log/filters`（`middleware.UserAuth()`）

- [ ] **Step 1: 写失败的测试**

创建 `controller/log_filters_test.go`。注意：本接口需要同时替换 `model.LOG_DB` 与用户身份，参考 `model/log_statistic_test.go` 的 `setupLogTestDB` 思路在 controller 包内建一个 `setupLogDBForController(t)`：

```go
package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/songquanpeng/one-api/model"
)

func setupLogDBForController(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "logs.db")
	dsn := fmt.Sprintf("%s?_busy_timeout=5000&_journal_mode=WAL", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开日志库失败: %v", err)
	}
	if err := db.AutoMigrate(&model.Log{}); err != nil {
		t.Fatalf("迁移日志表失败: %v", err)
	}
	old := model.LOG_DB
	model.LOG_DB = db
	t.Cleanup(func() { model.LOG_DB = old })
	// Windows 上必须先释放 sqlite 文件句柄，否则 t.TempDir 的清理会报
	// "The process cannot access the file because it is being used by another process"。
	if sqlDB, err := db.DB(); err == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
}

func getLogFilters(t *testing.T, id, role int, query string) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.GET("/api/log/filters", withIdentity(id, role), GetLogFilters)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/log/filters"+query, nil)
	r.ServeHTTP(w, req)
	return w
}

type filterResp struct {
	Success bool `json:"success"`
	Data    struct {
		Users  []string `json:"users"`
		Tokens []string `json:"tokens"`
		Models []string `json:"models"`
	} `json:"data"`
}

func TestGetLogFiltersScopesByRole(t *testing.T) {
	setupControllerTestDB(t)
	setupLogDBForController(t)

	rows := []model.Log{
		{UserId: 1, Username: "alice", TokenName: "t1", ModelName: "gpt-3.5-turbo", Type: model.LogTypeConsume, CreatedAt: 1773000000},
		{UserId: 2, Username: "bob", TokenName: "t2", ModelName: "claude-3", Type: model.LogTypeConsume, CreatedAt: 1773000001},
	}
	if err := model.LOG_DB.Create(&rows).Error; err != nil {
		t.Fatalf("写入日志失败: %v", err)
	}

	// 普通用户：没有 users，只有自己的 tokens/models
	w := getLogFilters(t, 1, model.RoleCommonUser, "")
	var nonAdmin filterResp
	if err := json.Unmarshal(w.Body.Bytes(), &nonAdmin); err != nil {
		t.Fatalf("解析失败: %v，body=%s", err, w.Body.String())
	}
	if len(nonAdmin.Data.Users) != 0 {
		t.Fatalf("非管理员不应拿到 users，实际 %v", nonAdmin.Data.Users)
	}
	if len(nonAdmin.Data.Tokens) != 1 || nonAdmin.Data.Tokens[0] != "t1" {
		t.Fatalf("非管理员令牌候选错误: %v", nonAdmin.Data.Tokens)
	}

	// 管理员：拿到全站 users
	w = getLogFilters(t, 99, model.RoleRootUser, "")
	var admin filterResp
	if err := json.Unmarshal(w.Body.Bytes(), &admin); err != nil {
		t.Fatalf("解析失败: %v，body=%s", err, w.Body.String())
	}
	if len(admin.Data.Users) != 2 {
		t.Fatalf("管理员应拿到两个用户，实际 %v", admin.Data.Users)
	}
	if len(admin.Data.Tokens) != 2 {
		t.Fatalf("管理员应拿到两个令牌，实际 %v", admin.Data.Tokens)
	}

	// 管理员传 username 收窄令牌
	w = getLogFilters(t, 99, model.RoleRootUser, "?username=bob")
	var narrowed filterResp
	if err := json.Unmarshal(w.Body.Bytes(), &narrowed); err != nil {
		t.Fatalf("解析失败: %v，body=%s", err, w.Body.String())
	}
	if len(narrowed.Data.Tokens) != 1 || narrowed.Data.Tokens[0] != "t2" {
		t.Fatalf("按用户收窄令牌失败: %v", narrowed.Data.Tokens)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./controller/ -run TestGetLogFiltersScopesByRole -v`
Expected: 编译失败，`undefined: GetLogFilters`

- [ ] **Step 3: 实现**

在 `controller/log.go` 追加：

```go
// GetLogFilters 返回所选区间内日志里实际出现过的用户/令牌/模型候选值，供筛选下拉使用。
// 非管理员按自己收窄，且不返回 users（该筛选框对非管理员隐藏）。
func GetLogFilters(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	userId := c.GetInt(ctxkey.Id)
	isAdmin := c.GetInt(ctxkey.Role) >= model.RoleAdminUser
	usernameFilter := ""
	if isAdmin {
		// 管理员看全站；带 username 时用它收窄令牌候选值
		userId = 0
		usernameFilter = c.Query("username")
	}
	const candidateLimit = 500

	users := make([]string, 0)
	if isAdmin {
		values, err := model.SearchLogDistinctValues(0, "", startTimestamp, endTimestamp, "username", candidateLimit)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error(), "data": nil})
			return
		}
		users = values
	}

	tokens, err := model.SearchLogDistinctValues(userId, usernameFilter, startTimestamp, endTimestamp, "token_name", candidateLimit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error(), "data": nil})
		return
	}
	models, err := model.SearchLogDistinctValues(userId, usernameFilter, startTimestamp, endTimestamp, "model_name", candidateLimit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error(), "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"users":  users,
			"tokens": tokens,
			"models": models,
		},
	})
}
```

在 `router/api.go` 的 log 路由组中，紧跟 `logRoute.GET("/search", ...)` 之后插入：

```go
		logRoute.GET("/filters", middleware.UserAuth(), controller.GetLogFilters)
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./controller/ -run TestGetLogFiltersScopesByRole -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
go test ./...
git add controller/log.go controller/log_filters_test.go router/api.go
git commit -m "feat: 新增日志候选值接口 /api/log/filters"
```

---

### Task B5: 用户列表返回今日用量

**Files:**
- Modify: `controller/user.go`（`GetAllUsers`，187-209 行）
- Test: `controller/users_today_usage_test.go`

**Interfaces:**
- Consumes: `model.GetDailyUsages`（A1）、`model.User.EffectiveDailyTokenLimit`/`EffectiveDailyQuotaLimit`（A2）
- Produces: `type UserListItem struct { *model.User; TodayTokens int64; TodayQuota int64; EffectiveDailyTokenLimit int64; EffectiveDailyQuotaLimit int64 }`

- [ ] **Step 1: 写失败的测试**

创建 `controller/users_today_usage_test.go`：

```go
package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/songquanpeng/one-api/model"
)

func TestGetAllUsersReturnsTodayUsage(t *testing.T) {
	setupControllerTestDB(t)
	u := &model.User{Username: "bob", Password: "hash", Role: model.RoleCommonUser, Status: 1, Quota: 100, AccessToken: "at-bob", AffCode: "aff-bob"}
	if err := model.DB.Create(u).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	limit := int64(5000)
	if err := model.DB.Model(&model.User{}).Where("id = ?", u.Id).Update("daily_token_limit", limit).Error; err != nil {
		t.Fatalf("设置上限失败: %v", err)
	}
	if err := model.RecordDailyUsage(u.Id, 120, 30, 45); err != nil {
		t.Fatalf("记账失败: %v", err)
	}

	r := gin.New()
	r.GET("/api/user/", GetAllUsers)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/user/?p=0", nil)
	r.ServeHTTP(w, req)

	var resp struct {
		Success bool `json:"success"`
		Data    []struct {
			Id                       int   `json:"id"`
			TodayTokens              int64 `json:"today_tokens"`
			TodayQuota               int64 `json:"today_quota"`
			EffectiveDailyTokenLimit int64 `json:"effective_daily_token_limit"`
			EffectiveDailyQuotaLimit int64 `json:"effective_daily_quota_limit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析失败: %v，body=%s", err, w.Body.String())
	}
	if !resp.Success || len(resp.Data) != 1 {
		t.Fatalf("响应不符: %s", w.Body.String())
	}
	if resp.Data[0].TodayTokens != 150 || resp.Data[0].TodayQuota != 45 {
		t.Fatalf("今日用量错误: %+v", resp.Data[0])
	}
	if resp.Data[0].EffectiveDailyTokenLimit != 5000 {
		t.Fatalf("生效 token 上限应取用户单独上限 5000，实际 %d", resp.Data[0].EffectiveDailyTokenLimit)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./controller/ -run TestGetAllUsersReturnsTodayUsage -v`
Expected: FAIL——响应里没有 `today_tokens` 等字段（解析后全为 0）

- [ ] **Step 3: 实现**

在 `controller/user.go` 中，`GetAllUsers` 之前加 DTO 定义，并替换 `GetAllUsers` 的响应构建：

```go
// UserListItem 在用户记录上附加今日用量与生效上限。
// 刻意不把这些字段加到 model.User 上：那会改变备份导出的 JSON 结构，
// 而 model/backup_test.go 对导出/导入做往返比较。
type UserListItem struct {
	*model.User
	TodayTokens              int64 `json:"today_tokens"`
	TodayQuota               int64 `json:"today_quota"`
	EffectiveDailyTokenLimit int64 `json:"effective_daily_token_limit"`
	EffectiveDailyQuotaLimit int64 `json:"effective_daily_quota_limit"`
}

func GetAllUsers(c *gin.Context) {
	p, _ := strconv.Atoi(c.Query("p"))
	if p < 0 {
		p = 0
	}

	order := c.DefaultQuery("order", "")
	users, err := model.GetAllUsers(p*config.ItemsPerPage, config.ItemsPerPage, order)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	userIds := make([]int, 0, len(users))
	for _, user := range users {
		userIds = append(userIds, user.Id)
	}
	// 每页只查一次今日用量，不是每行一次
	usages, err := model.GetDailyUsages(userIds, model.Today())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	items := make([]*UserListItem, 0, len(users))
	for _, user := range users {
		item := &UserListItem{
			User:                     user,
			EffectiveDailyTokenLimit: user.EffectiveDailyTokenLimit(),
			EffectiveDailyQuotaLimit: user.EffectiveDailyQuotaLimit(),
		}
		if usage, ok := usages[user.Id]; ok {
			item.TodayTokens = usage.PromptTokens + usage.CompletionTokens
			item.TodayQuota = usage.Quota
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    items,
	})
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./controller/ -run TestGetAllUsersReturnsTodayUsage -v`
Expected: PASS

- [ ] **Step 5: 全量测试并提交**

```bash
go test ./... && go build -o one-api.exe .
git add controller/user.go controller/users_today_usage_test.go
git commit -m "feat: 用户列表返回今日用量与生效上限"
```

特别注意 `model/backup_test.go` 必须仍然通过——本任务没有给 `model.User` 加字段，往返测试不应受影响。

---

### Task B6: 后端接口手工验收

**Files:** 无代码改动

- [ ] **Step 1: 启动并准备日志数据**

```bash
go build -o one-api.exe . && ./one-api.exe
```
用两个不同用户各发几次请求（至少跨两个模型），让 `logs` 里有可筛选的数据。

- [ ] **Step 2: 验证总览全站与筛选**

```bash
TOKEN=<系统访问令牌>
curl -s "http://localhost:3000/api/user/dashboard?scope=all&start_timestamp=$(date -d '-6 days' +%s)&end_timestamp=$(date +%s)" -H "Authorization: Bearer $TOKEN" | head -c 600
echo
curl -s "http://localhost:3000/api/user/dashboard?scope=all&username=<某个用户名>" -H "Authorization: Bearer $TOKEN" | head -c 400
echo
curl -s "http://localhost:3000/api/user/dashboard?scope=all&granularity=hour" -H "Authorization: Bearer $TOKEN" | head -c 400
```
预期：第一次返回所有用户的记录；第二次只剩该用户；第三次桶标签形如 `2026-09-20 14:00`。

- [ ] **Step 3: 验证权限与校验**

```bash
# 用普通用户的令牌调 scope=all，应 403
curl -s -o /dev/null -w "%{http_code}\n" "http://localhost:3000/api/user/dashboard?scope=all" -H "Authorization: Bearer $USER_TOKEN"
# 起始晚于结束，应 400
curl -s -o /dev/null -w "%{http_code}\n" "http://localhost:3000/api/user/dashboard?start_timestamp=2000&end_timestamp=1000" -H "Authorization: Bearer $TOKEN"
```
预期：`403` 与 `400`。

- [ ] **Step 4: 验证候选值接口**

```bash
curl -s "http://localhost:3000/api/log/filters" -H "Authorization: Bearer $TOKEN"
curl -s "http://localhost:3000/api/log/filters?username=<某个用户名>" -H "Authorization: Bearer $TOKEN"
curl -s "http://localhost:3000/api/log/filters" -H "Authorization: Bearer $USER_TOKEN"
```
预期：管理员第一次拿到 users/tokens/models 三组值；第二次 tokens 收窄到该用户；普通用户返回的 `users` 为空数组。

- [ ] **Step 5: 记录结果并提交**

在计划末尾追加验收记录，然后：
```bash
git add docs/superpowers/plans/2026-09-20-daily-quota-and-dashboard-filters.md
git commit -m "docs: 记录后端接口验收结果"
```

---

# Part C：default 主题前端

前置：Part A、B 已合并，接口契约按设计文档冻结。

### Task C1: 总览筛选栏与拉取重写

**Files:**
- Modify: `web/default/src/pages/Dashboard/index.js`
- Modify: `web/default/src/pages/Dashboard/Dashboard.css`

**Interfaces:**
- Consumes: `GET /api/user/dashboard`、`GET /api/log/filters`、`helpers/api.js` 的 `API`
- Produces: 供 C5 复用的筛选交互约定（快捷区间、`all`/`self` 切换、三个下拉 + 「全部」选项）

- [ ] **Step 1: 替换导入与状态**

把 `import axios from 'axios';` 换成 `import { API } from '../../helpers/api';`，并新增 `Form, Button, Select` 等所需组件（从 `semantic-ui-react` 一起导入，与文件既有导入合并）。

在文件顶部 `chartConfig` 之后、组件之外先加两个解析辅助函数（`YYYY-MM-DD HH:mm:ss` 这种带空格的写法在 Safari 里无法被 `Date` 解析）：

```jsx
const parseLocalDateTime = (value) => new Date(String(value).replace(' ', 'T'));
const toUnixSeconds = (value) =>
  Math.floor(parseLocalDateTime(value).getTime() / 1000);
```

然后在组件内 `const [summaryData, setSummaryData] = useState(...)` 之后加入：

```jsx
  const isAdminUser = isAdmin();
  const [filters, setFilters] = useState({
    start: timestamp2string(Date.now() / 1000 - 86400 * 6),
    end: timestamp2string(Date.now() / 1000),
    granularity: 'day',
    scope: 'self',
    username: '',
    token_name: '',
    model_name: '',
  });
  const [candidates, setCandidates] = useState({
    users: [],
    tokens: [],
    models: [],
  });
```

`isAdmin` 从 `../../helpers/utils` 导入（与 `LogsTable.js:16` 的用法一致）；`timestamp2string` 已在该 helpers 中。

- [ ] **Step 2: 重写拉取与候选值加载**

替换 `useEffect` 与 `fetchDashboardData`：

```jsx
  useEffect(() => {
    fetchDashboardData();
  }, [filters]);

  useEffect(() => {
    fetchCandidates();
    // 用户名变化时令牌候选值要跟着收窄
  }, [filters.start, filters.end, filters.username, filters.scope]);

  const buildQuery = (params) => {
    const search = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => {
      if (value !== '' && value !== undefined && value !== null) {
        search.append(key, value);
      }
    });
    return search.toString();
  };

  const fetchCandidates = async () => {
    try {
      const query = buildQuery({
        start_timestamp: toUnixSeconds(filters.start),
        end_timestamp: toUnixSeconds(filters.end),
        username: filters.scope === 'all' ? filters.username : '',
      });
      const response = await API.get(`/api/log/filters?${query}`);
      if (response.data.success) {
        setCandidates(response.data.data || { users: [], tokens: [], models: [] });
      }
    } catch (error) {
      // 候选值拉取失败不阻塞总览本身，退化为只保留「全部」
      setCandidates({ users: [], tokens: [], models: [] });
    }
  };

  const fetchDashboardData = async () => {
    try {
      const query = buildQuery({
        start_timestamp: toUnixSeconds(filters.start),
        end_timestamp: toUnixSeconds(filters.end),
        granularity: filters.granularity,
        scope: filters.scope,
        username: filters.scope === 'all' ? filters.username : '',
        token_name: filters.token_name,
        model_name: filters.model_name,
      });
      const response = await API.get(`/api/user/dashboard?${query}`);
      if (response.data.success) {
        const dashboardData = response.data.data || [];
        setData(dashboardData);
        calculateSummary(dashboardData);
      }
    } catch (error) {
      setData([]);
      calculateSummary([]);
    }
  };
```

- [ ] **Step 3: 加桶生成函数并重写两个 process 函数**

替换 `processTimeSeriesData` 与 `processModelData`（原 114-195 行）为：

```jsx
  // 按请求区间与粒度生成桶标签，本地时区，与后端分桶口径一致。
  // 用 Date 加法而非固定毫秒步长，避免夏令时导致偏移。
  const buildBuckets = () => {
    const pad = (n) => String(n).padStart(2, '0');
    const format = (d) =>
      filters.granularity === 'hour'
        ? `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(
            d.getDate()
          )} ${pad(d.getHours())}:00`
        : `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;

    const buckets = [];
    const cursor = parseLocalDateTime(filters.start);
    if (filters.granularity === 'hour') {
      cursor.setMinutes(0, 0, 0);
    } else {
      cursor.setHours(0, 0, 0, 0);
    }
    const end = parseLocalDateTime(filters.end);
    let guard = 0;
    while (cursor <= end && guard < 4000) {
      buckets.push(format(cursor));
      if (filters.granularity === 'hour') {
        cursor.setHours(cursor.getHours() + 1);
      } else {
        cursor.setDate(cursor.getDate() + 1);
      }
      guard += 1;
    }
    return buckets;
  };

  const processTimeSeriesData = () => {
    const dailyData = {};
    buildBuckets().forEach((bucket) => {
      dailyData[bucket] = { date: bucket, requests: 0, quota: 0, tokens: 0 };
    });
    data.forEach((item) => {
      // 后端返回的桶理论上都在范围内，越界时跳过而不是抛异常
      if (!dailyData[item.Day]) {
        return;
      }
      dailyData[item.Day].requests += item.RequestCount;
      dailyData[item.Day].quota += item.Quota / 1000000;
      dailyData[item.Day].tokens += item.PromptTokens + item.CompletionTokens;
    });
    return Object.values(dailyData).sort((a, b) =>
      a.date.localeCompare(b.date)
    );
  };

  const processModelData = () => {
    const buckets = buildBuckets();
    const models = [...new Set(data.map((item) => item.ModelName))];
    const timeData = {};
    buckets.forEach((bucket) => {
      timeData[bucket] = { date: bucket };
      models.forEach((model) => {
        timeData[bucket][model] = 0;
      });
    });
    data.forEach((item) => {
      if (!timeData[item.Day]) {
        return;
      }
      timeData[item.Day][item.ModelName] =
        item.PromptTokens + item.CompletionTokens;
    });
    return Object.values(timeData).sort((a, b) =>
      a.date.localeCompare(b.date)
    );
  };
```

- [ ] **Step 4: 修掉「今天」的 UTC 判断并支持小时标签**

替换 `calculateSummary` 里的今天判断：

```jsx
    const pad = (n) => String(n).padStart(2, '0');
    const now = new Date();
    const today = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(
      now.getDate()
    )}`;
```

替换 `formatDate`：

```jsx
  const formatDate = (dateStr) => {
    // 兼容 day（YYYY-MM-DD）与 hour（YYYY-MM-DD HH:00）两种桶标签
    if (dateStr.length > 10) {
      return dateStr.slice(5, 16);
    }
    const date = new Date(dateStr);
    return date.toLocaleDateString('zh-CN', {
      month: 'numeric',
      day: 'numeric',
    });
  };
```

- [ ] **Step 5: 加筛选栏 JSX**

在 `return (` 的 `<div className='dashboard-container'>` 之内、第一个 `<Grid columns={3} ...>` 之前插入：

```jsx
      <Card fluid className='chart-card'>
        <Card.Content>
          <Form size='small'>
            <Form.Group inline>
              <Button
                size='small'
                basic
                onClick={() => setQuickRange(0)}
                content={t('dashboard.filters.today')}
              />
              <Button
                size='small'
                basic
                onClick={() => setQuickRange(6)}
                content={t('dashboard.filters.last_7_days')}
              />
              <Button
                size='small'
                basic
                onClick={() => setQuickRange(29)}
                content={t('dashboard.filters.last_30_days')}
              />
              <Form.Input
                label={t('dashboard.filters.start')}
                type='date'
                value={filters.start.slice(0, 10)}
                onChange={(e) =>
                  setFilters({ ...filters, start: `${e.target.value} 00:00:00` })
                }
              />
              <Form.Input
                label={t('dashboard.filters.end')}
                type='date'
                value={filters.end.slice(0, 10)}
                onChange={(e) =>
                  setFilters({ ...filters, end: `${e.target.value} 23:59:59` })
                }
              />
              <Form.Select
                label={t('dashboard.filters.granularity')}
                options={[
                  { key: 'day', text: t('dashboard.filters.granularity_day'), value: 'day' },
                  { key: 'hour', text: t('dashboard.filters.granularity_hour'), value: 'hour' },
                ]}
                value={filters.granularity}
                onChange={(e, { value }) =>
                  setFilters({ ...filters, granularity: value })
                }
              />
            </Form.Group>
            <Form.Group inline>
              {isAdminUser && (
                <Form.Select
                  label={t('dashboard.filters.scope')}
                  options={[
                    { key: 'self', text: t('dashboard.filters.scope_self'), value: 'self' },
                    { key: 'all', text: t('dashboard.filters.scope_all'), value: 'all' },
                  ]}
                  value={filters.scope}
                  onChange={(e, { value }) =>
                    setFilters({ ...filters, scope: value, username: '', token_name: '' })
                  }
                />
              )}
              {isAdminUser && filters.scope === 'all' && (
                <Form.Select
                  label={t('dashboard.filters.username')}
                  options={[
                    { key: '', text: t('dashboard.filters.all'), value: '' },
                    ...candidates.users.map((name) => ({ key: name, text: name, value: name })),
                  ]}
                  value={filters.username}
                  onChange={(e, { value }) =>
                    setFilters({ ...filters, username: value, token_name: '' })
                  }
                />
              )}
              <Form.Select
                label={t('dashboard.filters.token_name')}
                options={[
                  { key: '', text: t('dashboard.filters.all'), value: '' },
                  ...candidates.tokens.map((name) => ({ key: name, text: name, value: name })),
                ]}
                value={filters.token_name}
                onChange={(e, { value }) =>
                  setFilters({ ...filters, token_name: value })
                }
              />
              <Form.Select
                label={t('dashboard.filters.model_name')}
                options={[
                  { key: '', text: t('dashboard.filters.all'), value: '' },
                  ...candidates.models.map((name) => ({ key: name, text: name, value: name })),
                ]}
                value={filters.model_name}
                onChange={(e, { value }) =>
                  setFilters({ ...filters, model_name: value })
                }
              />
            </Form.Group>
          </Form>
        </Card.Content>
      </Card>
```

在同文件加 `setQuickRange`：

```jsx
  const setQuickRange = (daysAgo) => {
    const pad = (n) => String(n).padStart(2, '0');
    const formatDay = (d) =>
      `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
    const end = new Date();
    const start = new Date();
    start.setDate(start.getDate() - daysAgo);
    setFilters({
      ...filters,
      start: `${formatDay(start)} 00:00:00`,
      end: `${formatDay(end)} 23:59:59`,
    });
  };
```

- [ ] **Step 6: 加样式**

在 `web/default/src/pages/Dashboard/Dashboard.css` 追加：

```css
/* 筛选栏：小屏时换行，避免下拉被压缩到不可读 */
.chart-card .ui.form .inline.fields {
  flex-wrap: wrap;
  row-gap: 8px;
}
```

- [ ] **Step 7: 构建验证**

Run: `cd web/default && npm install && DISABLE_ESLINT_PLUGIN='true' npm run build`
Expected: 构建成功，无 ESLint 阻断

- [ ] **Step 8: 浏览器验证**

用 root 登录后打开 `/dashboard`：
- 默认显示最近 7 天，管理员能看到「全站 / 仅自己」切换
- 切到「全站」后出现用户名下拉，选一个用户后令牌下拉只剩该用户的令牌
- 切「天 / 小时」粒度，X 轴标签随之变化（小时标签形如 `09-20 14:00`）
- 切「近 30 天」快捷按钮，区间与图表同步

- [ ] **Step 9: 提交**

```bash
git add web/default/src/pages/Dashboard/index.js web/default/src/pages/Dashboard/Dashboard.css
git commit -m "feat(default): 总览支持时间段/粒度/全站范围与用户令牌模型筛选"
```

---

### Task C2: 用户编辑页三态限额控件

**Files:**
- Modify: `web/default/src/pages/User/EditUser.js`

**Interfaces:**
- Consumes: `PUT /api/user/`（A6）
- Produces: 三态选择 + 数值输入的交互约定，供 D 阶段两主题照搬

- [ ] **Step 1: 扩展表单状态**

在 `useState` 的 `originInputs`（13-22 行）中加入：

```jsx
  daily_token_limit: 0,
  daily_quota_limit: 0,
```

并在 load 用户数据处（约 55-70 行）把后端返回值映射成三态模式：

```jsx
        const dailyTokenMode =
          data.daily_token_limit === null || data.daily_token_limit === 0
            ? 'follow'
            : data.daily_token_limit < 0
            ? 'exempt'
            : 'custom';
        const dailyQuotaMode =
          data.daily_quota_limit === null || data.daily_quota_limit === 0
            ? 'follow'
            : data.daily_quota_limit < 0
            ? 'exempt'
            : 'custom';
        setInputs({
          ...data,
          daily_token_limit: Math.max(data.daily_token_limit || 0, 0),
          daily_quota_limit: Math.max(data.daily_quota_limit || 0, 0),
        });
        setDailyTokenMode(dailyTokenMode);
        setDailyQuotaMode(dailyQuotaMode);
```

新增两个状态：

```jsx
  const [dailyTokenMode, setDailyTokenMode] = useState('follow');
  const [dailyQuotaMode, setDailyQuotaMode] = useState('follow');
```

- [ ] **Step 2: 提交时把三态编码成数字**

在 `submit`（78-95 行）里、`let data = { ...inputs, id: parseInt(userId) }` 之后插入：

```jsx
    // 三态编码：follow 必须显式发 0，发 null 会被后端 GORM 的零值跳过而改不回去
    data.daily_token_limit =
      dailyTokenMode === 'follow'
        ? 0
        : dailyTokenMode === 'exempt'
        ? -1
        : Math.max(parseInt(inputs.daily_token_limit) || 0, 1);
    data.daily_quota_limit =
      dailyQuotaMode === 'follow'
        ? 0
        : dailyQuotaMode === 'exempt'
        ? -1
        : Math.max(parseInt(inputs.daily_quota_limit) || 0, 1);
```

- [ ] **Step 3: 加两个字段的 JSX**

在配额输入（`name='quota'`，152-165 行）之后插入：

```jsx
          <Form.Group widths='equal'>
            <Form.Select
              label={t('user.edit.daily_token_limit')}
              options={[
                { key: 'follow', text: t('user.edit.limit_mode_follow_global'), value: 'follow' },
                { key: 'exempt', text: t('user.edit.limit_mode_exempt'), value: 'exempt' },
                { key: 'custom', text: t('user.edit.limit_mode_custom'), value: 'custom' },
              ]}
              value={dailyTokenMode}
              onChange={(e, { value }) => setDailyTokenMode(value)}
            />
            {dailyTokenMode === 'custom' && (
              <Form.Input
                label={t('user.edit.daily_token_limit_value')}
                name='daily_token_limit'
                type='number'
                min='1'
                value={inputs.daily_token_limit}
                onChange={handleInputChange}
              />
            )}
          </Form.Group>
          <Form.Group widths='equal'>
            <Form.Select
              label={t('user.edit.daily_quota_limit')}
              options={[
                { key: 'follow', text: t('user.edit.limit_mode_follow_global'), value: 'follow' },
                { key: 'exempt', text: t('user.edit.limit_mode_exempt'), value: 'exempt' },
                { key: 'custom', text: t('user.edit.limit_mode_custom'), value: 'custom' },
              ]}
              value={dailyQuotaMode}
              onChange={(e, { value }) => setDailyQuotaMode(value)}
            />
            {dailyQuotaMode === 'custom' && (
              <Form.Input
                label={`${t('user.edit.daily_quota_limit_value')}${renderQuotaWithPrompt(
                  inputs.daily_quota_limit
                )}`}
                name='daily_quota_limit'
                type='number'
                min='1'
                value={inputs.daily_quota_limit}
                onChange={handleInputChange}
              />
            )}
          </Form.Group>
```

- [ ] **Step 4: 构建验证**

Run: `cd web/default && DISABLE_ESLINT_PLUGIN='true' npm run build`
Expected: 构建成功

- [ ] **Step 5: 浏览器验证**

打开 用户 → 编辑某用户，依次验证：
- 默认显示「跟随全局」，数值框隐藏
- 选「自定义」出现数值框，填 5000 保存后重进页面，仍显示「自定义 5000」
- 选「跟随全局」保存后重进，显示「跟随全局」（验证 `0` 真的写进去了）
- 选「豁免」保存后重进，显示「豁免」

- [ ] **Step 6: 提交**

```bash
git add web/default/src/pages/User/EditUser.js
git commit -m "feat(default): 用户编辑页支持单日上限三态设置"
```

---

### Task C3: 运营设置的两个全局默认项

**Files:**
- Modify: `web/default/src/components/OperationSetting.js`

- [ ] **Step 1: 加默认值与字段**

在 `inputs` 默认对象中加入：

```jsx
  DailyTokenLimitDefault: 0,
  DailyQuotaLimitDefault: 0,
```

在 `PreConsumedQuota` 输入（约 238-247 行）之后插入：

```jsx
          <Form.Input
            label={t('setting.operation.quota.daily_token_limit_default')}
            name='DailyTokenLimitDefault'
            onChange={handleInputChange}
            autoComplete='new-password'
            value={inputs.DailyTokenLimitDefault}
            type='number'
            min='0'
            placeholder={t('setting.operation.quota.daily_limit_placeholder')}
          />
          <Form.Input
            label={t('setting.operation.quota.daily_quota_limit_default')}
            name='DailyQuotaLimitDefault'
            onChange={handleInputChange}
            autoComplete='new-password'
            value={inputs.DailyQuotaLimitDefault}
            type='number'
            min='0'
            placeholder={t('setting.operation.quota.daily_limit_placeholder')}
          />
```

在保存逻辑（`originInputs` 差异比较，约 143-153 行的 quota 分组）中加入：

```jsx
    if (originInputs['DailyTokenLimitDefault'] !== inputs.DailyTokenLimitDefault) {
      await updateOption('DailyTokenLimitDefault', inputs.DailyTokenLimitDefault);
    }
    if (originInputs['DailyQuotaLimitDefault'] !== inputs.DailyQuotaLimitDefault) {
      await updateOption('DailyQuotaLimitDefault', inputs.DailyQuotaLimitDefault);
    }
```

- [ ] **Step 2: 构建与验证**

Run: `cd web/default && DISABLE_ESLINT_PLUGIN='true' npm run build`
浏览器打开 设置 → 运营设置：两个输入框存在，填 `1000000` 保存后刷新页面仍显示该值；填 `0` 保存后 `daily_usage` 不再触发拦截（可配合 A7 的拦截用例验证）。

- [ ] **Step 3: 提交**

```bash
git add web/default/src/components/OperationSetting.js
git commit -m "feat(default): 运营设置新增单日上限全局默认值"
```

---

### Task C4: 用户列表显示今日用量

**Files:**
- Modify: `web/default/src/components/UsersTable.js`

- [ ] **Step 1: 在「统计信息」列追加今日用量**

在 `renderQuota` / `request_count` 显示所在单元格（约 270-289 行）之后追加（注意该文件的行变量名是 `user`，不是 `record`）：

```jsx
              <div>
                <Popup
                  content={t('user.table.today_usage')}
                  trigger={
                    <Label basic>
                      {renderNumber(user.today_tokens)}
                      {user.effective_daily_token_limit > 0
                        ? ` / ${renderNumber(user.effective_daily_token_limit)}`
                        : ` / ${t('user.table.unlimited')}`}
                    </Label>
                  }
                />
              </div>
```

`renderNumber`、`Label`、`Popup` 在该文件已导入（第 2-20 行），无需新增 import。

- [ ] **Step 2: 构建与验证**

Run: `cd web/default && DISABLE_ESLINT_PLUGIN='true' npm run build`
浏览器打开 用户 页面：每行显示「今日用量」与上限；未设上限的用户显示「不限」；今日没有消耗的用户显示 0。

- [ ] **Step 3: 提交**

```bash
git add web/default/src/components/UsersTable.js
git commit -m "feat(default): 用户列表显示今日用量与上限"
```

---

### Task C5: 日志页用户/令牌/模型下拉

**Files:**
- Modify: `web/default/src/components/LogsTable.js`

- [ ] **Step 1: 加候选值状态与拉取**

在 `inputs` 状态之后加入：

```jsx
  const [candidates, setCandidates] = useState({
    users: [],
    tokens: [],
    models: [],
  });

  const fetchCandidates = async () => {
    try {
      const query = new URLSearchParams({
        start_timestamp: Math.floor(Date.parse(start_timestamp) / 1000),
        end_timestamp: Math.floor(Date.parse(end_timestamp) / 1000),
        username: isAdminUser ? username : '',
      });
      const res = await API.get(`/api/log/filters?${query.toString()}`);
      if (res.data.success) {
        setCandidates(res.data.data || { users: [], tokens: [], models: [] });
      }
    } catch (error) {
      setCandidates({ users: [], tokens: [], models: [] });
    }
  };

  useEffect(() => {
    fetchCandidates();
  }, [start_timestamp, end_timestamp, username]);
```

- [ ] **Step 2: 三个文本框换成下拉**

把令牌输入（330-336 行）、模型输入（338-347 行）与用户名输入（391-400 行）分别替换为：

```jsx
          <Form.Select
            label={t('log.table.token_name')}
            options={[
              { key: '', text: t('log.table.all'), value: '' },
              ...candidates.tokens.map((name) => ({ key: name, text: name, value: name })),
            ]}
            value={token_name}
            name='token_name'
            onChange={(e, { value }) => handleInputChange(e, { name: 'token_name', value })}
          />
```

```jsx
          <Form.Select
            label={t('log.table.model_name')}
            options={[
              { key: '', text: t('log.table.all'), value: '' },
              ...candidates.models.map((name) => ({ key: name, text: name, value: name })),
            ]}
            value={model_name}
            name='model_name'
            onChange={(e, { value }) => handleInputChange(e, { name: 'model_name', value })}
          />
```

```jsx
              <Form.Select
                label={t('log.table.username')}
                options={[
                  { key: '', text: t('log.table.all'), value: '' },
                  ...candidates.users.map((name) => ({ key: name, text: name, value: name })),
                ]}
                value={username}
                name='username'
                onChange={(e, { value }) => handleInputChange(e, { name: 'username', value })}
              />
```

注意保持该文件既有的 `handleInputChange` 签名（它按 `name` 取值），若签名不同则以文件现状为准并保持取值一致。

- [ ] **Step 3: 构建与验证**

Run: `cd web/default && DISABLE_ESLINT_PLUGIN='true' npm run build`
浏览器打开 日志：三个筛选都是下拉且含「全部」；选某个用户后令牌下拉收窄；选令牌后点「查询」，列表与统计总额都跟着变；非管理员看不到用户名下拉。

- [ ] **Step 4: 提交**

```bash
git add web/default/src/components/LogsTable.js
git commit -m "feat(default): 日志页用户/令牌/模型筛选改为下拉"
```

---

### Task C6: default 主题 i18n

**Files:**
- Modify: `web/default/src/locales/zh/translation.json`
- Modify: `web/default/src/locales/en/translation.json`

- [ ] **Step 1: 中文词条**

在 `zh/translation.json` 的 `dashboard` 对象下加入 `filters` 子对象，在 `log.table` 下加入 `all`，在 `user.edit` 与 `user.table`、`setting.operation.quota` 下加入对应键：

```json
"dashboard.filters.today": "今天",
"dashboard.filters.last_7_days": "近 7 天",
"dashboard.filters.last_30_days": "近 30 天",
"dashboard.filters.start": "起始日期",
"dashboard.filters.end": "结束日期",
"dashboard.filters.granularity": "粒度",
"dashboard.filters.granularity_day": "天",
"dashboard.filters.granularity_hour": "小时",
"dashboard.filters.scope": "数据范围",
"dashboard.filters.scope_self": "仅自己",
"dashboard.filters.scope_all": "全站",
"dashboard.filters.username": "用户",
"dashboard.filters.token_name": "令牌",
"dashboard.filters.model_name": "模型",
"dashboard.filters.all": "全部",
"log.table.all": "全部",
"user.edit.daily_token_limit": "单日 Token 上限",
"user.edit.daily_quota_limit": "单日额度上限",
"user.edit.daily_token_limit_value": "Token 上限值",
"user.edit.daily_quota_limit_value": "额度上限值",
"user.edit.limit_mode_follow_global": "跟随全局默认",
"user.edit.limit_mode_exempt": "豁免（不限制）",
"user.edit.limit_mode_custom": "自定义",
"user.table.today_usage": "今日用量",
"user.table.unlimited": "不限",
"setting.operation.quota.daily_token_limit_default": "单日 Token 上限（全局默认）",
"setting.operation.quota.daily_quota_limit_default": "单日额度上限（全局默认）",
"setting.operation.quota.daily_limit_placeholder": "0 表示不限制"
```

注意：该文件是嵌套 JSON，不是扁平键。上面用点号表示层级，落地时必须展开成嵌套对象（例如 `dashboard.filters.today` 写成 `dashboard: { filters: { today: "今天" } }`）。**不要**混用两种风格，否则 `t()` 取不到值。

- [ ] **Step 2: 英文词条**

在 `en/translation.json` 按同样层级加入对应英文：

```json
"dashboard.filters.today": "Today",
"dashboard.filters.last_7_days": "Last 7 days",
"dashboard.filters.last_30_days": "Last 30 days",
"dashboard.filters.start": "Start date",
"dashboard.filters.end": "End date",
"dashboard.filters.granularity": "Granularity",
"dashboard.filters.granularity_day": "Day",
"dashboard.filters.granularity_hour": "Hour",
"dashboard.filters.scope": "Scope",
"dashboard.filters.scope_self": "Mine only",
"dashboard.filters.scope_all": "Whole station",
"dashboard.filters.username": "User",
"dashboard.filters.token_name": "Token",
"dashboard.filters.model_name": "Model",
"dashboard.filters.all": "All",
"log.table.all": "All",
"user.edit.daily_token_limit": "Daily token limit",
"user.edit.daily_quota_limit": "Daily quota limit",
"user.edit.daily_token_limit_value": "Token limit value",
"user.edit.daily_quota_limit_value": "Quota limit value",
"user.edit.limit_mode_follow_global": "Follow global default",
"user.edit.limit_mode_exempt": "Exempt (unlimited)",
"user.edit.limit_mode_custom": "Custom",
"user.table.today_usage": "Today",
"user.table.unlimited": "Unlimited",
"setting.operation.quota.daily_token_limit_default": "Daily token limit (global default)",
"setting.operation.quota.daily_quota_limit_default": "Daily quota limit (global default)",
"setting.operation.quota.daily_limit_placeholder": "0 means unlimited"
```

- [ ] **Step 3: 校验 JSON 合法且键对齐**

```bash
node -e "const zh=require('./web/default/src/locales/zh/translation.json');const en=require('./web/default/src/locales/en/translation.json');console.log('zh ok', !!zh.dashboard.filters, !!zh.user.edit.daily_token_limit);console.log('en ok', !!en.dashboard.filters, !!en.user.edit.daily_token_limit);"
```
Expected: 输出 `zh ok true true` 与 `en ok true true`

- [ ] **Step 4: 构建并提交**

```bash
cd web/default && DISABLE_ESLINT_PLUGIN='true' npm run build
cd ../.. && git add web/default/src/locales/zh/translation.json web/default/src/locales/en/translation.json
git commit -m "feat(default): 补充总览筛选与单日上限的中英文案"
```

---

# Part D：berry 与 air 主题前端

### Task D1: berry 总览筛选

**Files:**
- Modify: `web/berry/src/views/Dashboard/index.js`
- Modify: `web/berry/src/utils/chart.js`

**Interfaces:**
- Consumes: 与 C1 相同的两个接口；`@mui/x-date-pickers`（该主题已在 `views/Log/component/TableToolBar.js` 使用）

- [ ] **Step 1: 把 7 天硬编码换成按区间生成**

`web/berry/src/utils/chart.js` 中新增（保留 `getLastSevenDays` 供其他地方引用）：

```js
// 按请求区间与粒度生成桶标签，本地时区，与后端分桶口径一致
export function getDateRange(startDate, endDate, granularity = 'day') {
  const pad = (n) => String(n).padStart(2, '0');
  const format = (d) =>
    granularity === 'hour'
      ? `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:00`
      : `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
  const dates = [];
  const cursor = new Date(startDate);
  if (granularity === 'hour') {
    cursor.setMinutes(0, 0, 0);
  } else {
    cursor.setHours(0, 0, 0, 0);
  }
  const end = new Date(endDate);
  let guard = 0;
  while (cursor <= end && guard < 4000) {
    dates.push(format(cursor));
    if (granularity === 'hour') {
      cursor.setHours(cursor.getHours() + 1);
    } else {
      cursor.setDate(cursor.getDate() + 1);
    }
    guard += 1;
  }
  return dates;
}
```

- [ ] **Step 2: Dashboard 用筛选状态驱动**

在 `web/berry/src/views/Dashboard/index.js` 中：
- 新增 `filters` 状态（`start`/`end` 用 dayjs 或 Date、`granularity`、`scope`、`username`/`token_name`/`model_name`，默认最近 7 天）
- `userDashboard` 改为 `API.get('/api/user/dashboard', { params: {...} })`（该主题既有 `params` 用法见 `views/Log/index.js:40-51`）
- `getLineDataGroup` / `getBarDataGroup` 里的 `getLastSevenDays()` 换成 `getDateRange(filters.start, filters.end, filters.granularity)`
- `getBarDataGroup` 里 `const index = lastSevenDays.indexOf(item.Day);` 与 `if (i < 7)` 的写法改为直接按桶名取值：`newData.data[dates.indexOf(item.Day)] = value`，并对 `indexOf` 返回 `-1` 的情况跳过
- `useEffect(() => { userDashboard(); }, [filters])`

- [ ] **Step 3: 加筛选栏**

在外层 `<Grid container spacing={gridSpacing}>` 的第一个子项位置加：

```jsx
      <Grid item xs={12}>
        <Grid container spacing={gridSpacing} alignItems="center">
          <Grid item>
            <Button variant="outlined" onClick={() => setQuickRange(0)}>今天</Button>
            <Button variant="outlined" onClick={() => setQuickRange(6)}>近 7 天</Button>
            <Button variant="outlined" onClick={() => setQuickRange(29)}>近 30 天</Button>
          </Grid>
          <Grid item>
            <LocalizationProvider dateAdapter={AdapterDayjs} adapterLocale={'zh-cn'}>
              <DatePicker
                label="起始日期"
                value={dayjs(filters.start)}
                onChange={(v) => v && setFilters({ ...filters, start: v.startOf('day').toDate() })}
              />
            </LocalizationProvider>
          </Grid>
          <Grid item>
            <LocalizationProvider dateAdapter={AdapterDayjs} adapterLocale={'zh-cn'}>
              <DatePicker
                label="结束日期"
                value={dayjs(filters.end)}
                onChange={(v) => v && setFilters({ ...filters, end: v.endOf('day').toDate() })}
              />
            </LocalizationProvider>
          </Grid>
          <Grid item>
            <FormControl size="small">
              <InputLabel>粒度</InputLabel>
              <Select
                label="粒度"
                value={filters.granularity}
                onChange={(e) => setFilters({ ...filters, granularity: e.target.value })}
              >
                <MenuItem value="day">天</MenuItem>
                <MenuItem value="hour">小时</MenuItem>
              </Select>
            </FormControl>
          </Grid>
        </Grid>
      </Grid>
```

并按 C1 相同的规则补「全站/仅自己」（仅管理员可见）、用户/令牌/模型下拉（候选值来自 `/api/log/filters`，选中用户后收窄令牌）。该主题的用户与管理端判断沿用 `views/Log/index.js` 里既有的 `userIsAdmin` 取得方式。

- [ ] **Step 4: 构建与验证**

Run: `cd web/berry && npm install && DISABLE_ESLINT_PLUGIN='true' npm run build`
浏览器把主题切到 berry（设置 → 主题，或 `THEME=berry` 启动）后重复 C1 Step 8 的全部验证项。

- [ ] **Step 5: 提交**

```bash
git add web/berry/src/views/Dashboard/index.js web/berry/src/utils/chart.js
git commit -m "feat(berry): 总览支持时间段/粒度/全站范围与筛选"
```

---

### Task D2: berry 用户编辑、运营设置与用户列表

**Files:**
- Modify: `web/berry/src/views/User/component/EditModal.js`
- Modify: `web/berry/src/views/Setting/component/OperationSetting.js`
- Modify: `web/berry/src/views/User/component/TableRow.js`

- [ ] **Step 1: 用户编辑弹窗的三态控件**

`EditModal.js`：
- `originInputs`（50-57 行）加 `daily_token_limit: 0`、`daily_quota_limit: 0`
- Yup `validationSchema` 加 `daily_token_limit: Yup.number().min(-1, '单日上限不能小于 -1')`、`daily_quota_limit` 同理
- 在额度字段（210-232 行）之后加两组「三态 Select + 条件数字输入」，语义与 C2 完全一致：`follow` 提交 0、`exempt` 提交 -1、`custom` 提交正整数
- 载入用户时把后端值映射成三态（`null`/`0` → follow、`<0` → exempt、`>0` → custom）

- [ ] **Step 2: 运营设置两个新项**

`OperationSetting.js`：
- `inputs` 默认对象加 `DailyTokenLimitDefault: 0`、`DailyQuotaLimitDefault: 0`
- 在 `PreConsumedQuota` 同区块加两个 `FormControl + OutlinedInput type="number"`，标签用中文（该文件现状即硬编码中文）
- 在保存逻辑里加两处 `updateOption('DailyTokenLimitDefault', ...)` / `updateOption('DailyQuotaLimitDefault', ...)` 的差异比较

- [ ] **Step 3: 用户表格显示今日用量**

`TableRow.js` 的统计信息单元格里，在「请求次数」那个 `Tooltip` 之后（约 101 行）追加。注意该文件的行变量名是 `item`（不是 `record`），且没有导入 `Typography`——沿用同一单元格已有的 `Tooltip` + `Label` 组合：

```jsx
            <Tooltip title={'今日用量 / 单日上限'} placement="top">
              <Label color={'primary'} variant="outlined">
                {' '}
                {renderNumber(item.today_tokens)}
                {item.effective_daily_token_limit > 0
                  ? ` / ${renderNumber(item.effective_daily_token_limit)}`
                  : ' / 不限'}{' '}
              </Label>
            </Tooltip>
```

- [ ] **Step 4: 构建与验证**

Run: `cd web/berry && DISABLE_ESLINT_PLUGIN='true' npm run build`
逐项验证：三态设置能保存并回显（含改回「跟随全局」）；运营设置的全局默认值能保存；用户表格显示今日用量与上限。

- [ ] **Step 5: 提交**

```bash
git add web/berry/src/views/User/component/EditModal.js web/berry/src/views/Setting/component/OperationSetting.js web/berry/src/views/User/component/TableRow.js
git commit -m "feat(berry): 用户单日上限设置与今日用量展示"
```

---

### Task D3: berry 日志页下拉

**Files:**
- Modify: `web/berry/src/views/Log/component/TableToolBar.js`
- Modify: `web/berry/src/views/Log/index.js`

- [ ] **Step 1: 候选值状态放在 Log/index.js**

在 `views/Log/index.js` 里新增 `candidates` 状态与拉取函数，条件与 C5 一致（区间、管理员才带 `username`），并把 `candidates` 透传给 `<TableToolBar candidates={candidates} ... />`。

- [ ] **Step 2: 三个输入换成 Select**

`TableToolBar.js` 中把 `token_name`（45-49 行）、`model_name`（64-68 行）与管理员专用 `username`（185-189 行）替换为 MUI `Select`，选项为「全部」+ 候选值，`onChange` 走既有的 `handleFilterName`。

- [ ] **Step 3: 构建与验证**

Run: `cd web/berry && DISABLE_ESLINT_PLUGIN='true' npm run build`
验证与 C5 Step 3 相同的项。

- [ ] **Step 4: 提交**

```bash
git add web/berry/src/views/Log/component/TableToolBar.js web/berry/src/views/Log/index.js
git commit -m "feat(berry): 日志页用户/令牌/模型筛选改为下拉"
```

---

### Task D4: air 死页面改造为总览

**Files:**
- Create: `web/air/src/pages/Dashboard/index.js`（由 `web/air/src/pages/Detail/index.js` 改造而来）
- Delete: `web/air/src/pages/Detail/index.js`
- Modify: `web/air/src/App.js`
- Modify: `web/air/src/components/SiderBar.js`

背景：`pages/Detail/index.js` 请求的 `/api/data/`、`/api/data/self/`（177-179 行）在本仓库不存在，且菜单项被 `localStorage.enable_data_export === 'true'` 门控，而 `/api/status` 从不返回该字段，因此该页面既打不开也没有数据。

- [ ] **Step 1: 迁移文件并换掉数据源**

把 `pages/Detail/index.js` 复制为 `pages/Dashboard/index.js`，然后：
- 删除 `enable_data_export` 相关的隐藏逻辑（本项目不该再依赖它）
- 数据请求改为 `API.get('/api/user/dashboard', { params })`，参数与 C1 相同：`start_timestamp`、`end_timestamp`、`granularity`、`scope`、`username`、`token_name`、`model_name`
- 时间粒度选项从「小时/天/周」改为「小时/天」（不做周粒度）
- 时间粒度、用户名/令牌名/模型名候选值改用 `/api/log/filters`
- 图表：首次 `new VChart(spec, { dom })` 后，筛选变化时调用 `updateSpec` + `reLayout`（既有代码在 277-283 行已有 `updateSpec`/`reLayout` 的用法）
- 删除 `web/air/src/pages/Detail/index.js`

- [ ] **Step 2: 注册路由与菜单**

`web/air/src/App.js`：
- 把 `const Detail = lazy(() => import('./pages/Detail'));` 改为 `const Dashboard = lazy(() => import('./pages/Dashboard'));`
- 把 `/detail` 的 `<Route>` 改为 `/dashboard`，元素改为 `<Dashboard />`

`web/air/src/components/SiderBar.js`：
- `headerButtons` 中把 `数据看板` 项改为 `{ text: '总览', itemKey: 'dashboard', to: '/dashboard', icon: <IconHistogram /> }`，**去掉** `className: localStorage.getItem('enable_data_export') === 'true' ? ... : 'tableHiddle'` 这层门控
- `routerMap` 中把 `detail: '/detail'` 改为 `dashboard: '/dashboard'`
- 移除 `headerButtons` 的 `useMemo` 依赖数组里已不再需要的 `enable_data_export`（保留其余依赖）

- [ ] **Step 3: 构建与验证**

Run: `cd web/air && npm install && DISABLE_ESLINT_PLUGIN='true' npm run build`
浏览器把主题切到 air：
- 侧边栏出现「总览」且可直接打开（不再需要任何 localStorage 开关）
- 默认显示最近 7 天数据；切时间段、粒度、全站/仅自己、用户/令牌/模型筛选，图表随之更新
- 浏览器控制台不再出现 `/api/data/` 的 404

- [ ] **Step 4: 提交**

```bash
git add web/air/src/pages/Dashboard web/air/src/pages/Detail web/air/src/App.js web/air/src/components/SiderBar.js
git commit -m "feat(air): 修复并改造数据看板为总览页"
```

---

### Task D5: air 用户编辑、运营设置与用户列表

**Files:**
- Modify: `web/air/src/pages/User/EditUser.js`
- Modify: `web/air/src/components/OperationSetting.js`
- Modify: `web/air/src/components/UsersTable.js`

- [ ] **Step 1: SideSheet 里的三态限额**

`pages/User/EditUser.js`：
- `inputs` 加 `daily_token_limit: 0`、`daily_quota_limit: 0`
- 在额度字段（145-174 行）之后加两组 Semi `Select`（跟随全局/豁免/自定义）+ 条件 `Input type='number'`
- `submit`（67-88 行）里按 C2 的规则编码三态：follow → 0、exempt → -1、custom → 正整数

- [ ] **Step 2: 运营设置**

`components/OperationSetting.js`：该文件用的是 **semantic-ui-react**，按同文件既有 `Form.Input` 写法加两个 `type='number' min='0'` 输入（`DailyTokenLimitDefault`、`DailyQuotaLimitDefault`），并在保存逻辑里加两处差异比较。

- [ ] **Step 3: 用户列表**

`components/UsersTable.js` 统计信息列（34-48 行）追加今日用量与上限，写法与 C4 相同（Semi 组件，沿用该文件现有渲染方式）。

- [ ] **Step 4: 构建与验证**

Run: `cd web/air && DISABLE_ESLINT_PLUGIN='true' npm run build`
验证项与 D2 Step 4 相同。

- [ ] **Step 5: 提交**

```bash
git add web/air/src/pages/User/EditUser.js web/air/src/components/OperationSetting.js web/air/src/components/UsersTable.js
git commit -m "feat(air): 用户单日上限设置与今日用量展示"
```

---

### Task D6: air 日志页下拉

**Files:**
- Modify: `web/air/src/components/LogsTable.js`

- [ ] **Step 1: 加候选值与三个下拉**

- 新增 `candidates` 状态与拉取（条件与 C5 一致）
- 令牌框（347 行）、模型框（350 行）、管理员专用用户名框（368 行）换成 `Form.Select`，选项为「全部」+ 候选值
- 该文件的 `handleInputChange(value, name)` 签名与 default 不同，替换时按该文件现有签名传参

- [ ] **Step 2: 构建与验证**

Run: `cd web/air && DISABLE_ESLINT_PLUGIN='true' npm run build`
验证项与 C5 Step 3 相同。

- [ ] **Step 3: 提交**

```bash
git add web/air/src/components/LogsTable.js
git commit -m "feat(air): 日志页用户/令牌/模型筛选改为下拉"
```

---

### Task D7: 三主题整体构建与联调验收

**Files:** 无代码改动（发现问题回到对应任务修复）

- [ ] **Step 1: 三主题一起构建**

```bash
cd web && sh build.sh
```
预期：三个主题依次 `npm install` + 构建成功，产物落在 `web/build/default`、`web/build/berry`、`web/build/air`。

- [ ] **Step 2: 重新构建二进制并验证 embed**

```bash
cd .. && go build -o one-api.exe . && ls -la web/build/*/index.html
```
预期：三个主题的 `index.html` 都存在（前端改动必须重新构建才会被 `//go:embed web/build/*` 收录）。

- [ ] **Step 3: 逐主题联调**

`./one-api.exe` 启动后，依次用 设置 → 主题 切到 default / berry / air，每个主题都验证：
1. 总览：默认最近 7 天；管理员可切全站/仅自己；时间段、粒度、三个筛选都生效且数字与日志页对得上
2. 用户编辑：三态上限能保存、能改回「跟随全局」、能豁免
3. 运营设置：两个全局默认值能保存
4. 用户列表：显示今日用量与上限
5. 日志页：用户/令牌/模型为下拉，选择后列表与统计随之变化

- [ ] **Step 4: 记录并提交**

在计划末尾追加验收记录（日期、三个主题各自结果、遗留问题），然后：

```bash
git add docs/superpowers/plans/2026-09-20-daily-quota-and-dashboard-filters.md
git commit -m "docs: 记录三主题联调验收结果"
```

---

### Task D8: 用户手册补充

**Files:**
- Modify: `docs/getting-started/user-manual.md`

- [ ] **Step 1: 补「单日用量上限」小节**

在 `## 额度规则` 下、`### 账户额度与令牌额度` 之后加：

```markdown
### 单日用量上限

除账户总额度外，系统还支持按天限制单个用户的用量，用于防止单账号（或密钥泄露）在一天内耗尽整站额度。

- **两个维度独立生效**：单日 Token 上限（`prompt_tokens + completion_tokens`）与单日额度上限。
- **三态设置**：每个用户的每个维度都可以是「跟随全局默认」「豁免（不限制）」或「单独上限」。全部用户共用的默认值在 设置 → 运营设置 里配置，`0` 表示不限制。
- **计量口径**：请求准入时用估算值判断，请求结束后按真实用量记账，因此单个进行中的请求最多能超出上限一个响应的量。两个维度的估算口径不同：token 维度是 `promptTokens + maxTokens`；quota 维度是 `getPreConsumedQuota(...) = (config.PreConsumedQuota + promptTokens + maxTokens) × 模型倍率`，其中 `config.PreConsumedQuota`（默认 500）**计入**估算。因此 quota 维度的有效下限与模型倍率相关：`daily_quota_limit` 低于约 `500 × 倍率` 时，当天**第一个**请求就会被拒——以本次验收用的 `gpt-3.5-turbo`（倍率 0.25）为例，下限约 `500 × 0.25 = 125`，再叠加本请求的 `(promptTokens + maxTokens) × 倍率`；实测（`max_tokens=50`、估算 promptTokens ≈ 8）估算值 ≈ `(500 + 8 + 50) × 0.25 ≈ 139`，故上限 150 时首个请求 200、累计 50 后的第二次 403，而低于约 139 时首个请求即 403。这是设计要求的从严行为。
- **跨天重置**：按服务器本地日期自然重置，不需要额外操作。
- **只约束 `/v1` 转发流量**：管理后台操作不计入。豁免用户仍受账户余额与令牌额度限制。
- **时间口径**：按服务器本地时区划分自然日，请确保应用进程与数据库会话的时区一致。
- **按用户/令牌筛选的候选值**：日志页与总览的筛选下拉候选值来自日志里实际出现过的名称。用户或令牌改名后，历史日志保留旧名，因此新旧名称会作为两个候选值分别出现，各自对应改名前后那段历史。
```

- [ ] **Step 2: 提交**

```bash
git add docs/getting-started/user-manual.md
git commit -m "docs: 用户手册补充单日用量上限与筛选候选值说明"
```

---

## 自审记录

**规格覆盖检查**（逐节对照设计文档）：

| 设计文档章节 | 对应任务 |
|---|---|
| `daily_usage` 表与记账（含 TableName、OnConflict、不受 LogConsumeEnabled 影响） | A1 |
| 用户三态字段（`*int64`、指针理由、显式 0） | A2 |
| 全局默认选项（config + OptionMap + 拒绝负值、不加环境变量） | A3 |
| 有效上限解析与判定（含 preConsumedQuota 短路陷阱） | A2、A4 |
| 四个模态的准入与结算（文本/Anthropic/Audio/Image，后两者仅 quota） | A5 |
| 错误码与消息（403、带具体数字、不走 i18n） | A4、A5 |
| UpdateUser 校验与"改回 0" | A6 |
| 总览接口契约（区间/粒度/scope/三个筛选/校验与上限） | B1、B3 |
| 时区修复（Go 本地边界 + 三方言本地分桶 + SQLite `localtime`） | B1、B3 |
| 候选值接口（角色作用域、白名单字段、LIMIT 500、不按 type 过滤） | B2、B4 |
| 用户列表增强（DTO + 每页一次查询 + 不改 model.User） | B5 |
| 顺带修复的 6 个既有缺陷 | C1（崩溃/UTC 今天/axios→API）、B1（SQLite 分桶）、B3（Go UTC 边界）、D4（air 死页面） |
| 三主题总览、用户编辑、运营设置、用户列表、日志页下拉 | C1–C5、D1–D3、D4–D6 |
| i18n 与文案 | C6、D2/D5（air、berry 硬编码中文） |
| 文档（用户手册、configuration.md） | D8 |
| 测试与验证方案 | A1–A6 单测、B1–B5 单测、A7/B6/D7 手工验收 |

**遗留确认项**（不阻塞，执行中如遇到按「已验证的偏离」处理并记录）：

1. 设计文档提到"`docs/getting-started/configuration.md` 若列举运营设置项则补充"——该文件目前是环境变量参考，本次不新增环境变量，因此 D8 只改用户手册。若执行时发现该文件确实列举了运营设置，再补两行。
2. `controller/option.go` 的 `UpdateOption` 需要 `strconv`，`controller/log.go` 的 `GetLogFilters` 依赖 `model.RoleAdminUser` 与 `ctxkey`——这些 import 若缺失需在对应任务内补上，任务描述已注明。
3. 三主题前端任务的步骤给出的是关键代码与锚点行号，落地时以文件现状为准保持周边风格一致；行号来自 2026-09-20 的快照，若已有改动请按锚点内容定位而不是死磕行号。

---

## 验收记录：Task A7 单日上限的端到端验收（2026-09-21）

**结果：通过** —— 6 个验收点全部 PASS；另有 2 项附加观察**未单独取证**，仅作为观察记录、不作为 PASS（理由见下）。无代码改动。完整证据（命令、原始输出、stub 形状、API 载荷）见 `.superpowers/sdd/2026-09-20-daily-quota-and-dashboard-filters/task-A7-report.md`。

驱动方式（无浏览器，按本任务允许的替代方案）：root 走 `POST /api/user/login` 取会话 cookie；上游用本地 stub（`POST /v1/chat/completions` 固定返回 `usage` 80/40）；渠道 `POST /api/channel/`（type 1，`base_url=http://127.0.0.1:18081`）；令牌 `POST /api/token/` 创建后从 `GET /api/token/` 读回 key；新建用户 `dailytest` 单独登录后建令牌。每请求结算 `quota=50`、`tokens=120`。

- **1 token 维度拦截** PASS：`daily_token_limit=100`，首请求 200（累计 120），次请求 403 `insufficient_user_daily_tokens`，消息 `已用 120 / 上限 100`；stub 计数未增加（拒绝发生在上游之前），用户额度/已用额度未变（拒绝发生在预扣之前）。
- **2 quota 维度拦截** PASS：`daily_quota_limit=150`，首请求 200（累计 50），次请求 403 `insufficient_user_daily_quota`，消息 `已用 ＄0.000100 额度 / 上限 ＄0.000300 额度`。
- **3 预扣短路陷阱** PASS（该结论只由 quota 维度证明）：用户额度提到 5×10¹⁴（远超 100×预扣 ≈13900），`preConsumedQuota` 会被短路置 0；本次运行的服务器日志确认该分支确实触发：`[preConsumeQuota] user 2 has enough quota 500000000000000, trusted and no need to pre-consume`。**quota 维度是判别性证据**：同一请求形态先 200 后 403（上限 150：首次 `0 + 139 ≤ 150` 通过，累计 50 后再 `50 + 139 = 189 > 150` 被拒），这一对结果把估算值限定在 `(100, 150]` 的非零区间——若用的是置 0 后的估算值，第二次会因 `50 + 0 ≤ 150` 而返回 200。**token 维度不具判别性**：该次运行中已用 token（120）本已超过上限（100），即使 `add` 为 0 也有 `used + add > limit`；且 token 维度的估算（`promptTokens + maxTokens` ≈ 58）根本不涉及 `preConsumedQuota`，故它只能证明日限生效，不能证明估算值取自短路之前。
- **4 豁免 `-1`** PASS：`daily_token_limit=-1, daily_quota_limit=-1` 后连续 3 次请求均 200，且继续记账（120→480 tokens）；随后改回 `0` 成功，验证指针字段可回退到"跟随全局默认"。
- **5 记账可见** PASS：`GET /api/user/?p=0` 尚无 `today_tokens/today_quota`（B5 未落地），按任务允许改为直接读表：清空后 3 次请求的 `daily_usage` 行为 `prompt 240 + completion 120 = 360` tokens、`quota 150`，与同时段 `logs type=2` 聚合完全一致。
- **6 不依赖日志开关** PASS：`LogConsumeEnabled=false`（option 已落库为 false）后 3 次请求仍 200，`daily_usage` 由 360/150 增至 720/300，而 `logs` 的 `type=2` 行数保持 3 不变。另观察到此时把 token 上限设回 `100` 仍 403（`已用 720 / 上限 100`）——**未单独取证**：该结果在「用户字段为 0 回退全局默认」与「用户字段仍为上一次的值」两种情况下都会相同，本次未回读用户行与全局默认选项确认，故不作为独立证据。
- **附加观察（未单独取证）**：`daily_token_limit=0`、`DailyTokenLimitDefault=100` 时报错显示「上限 100」。**未单独取证**：该结果在「用户字段为 0 回退全局默认」与「用户字段仍为上一次的值」两种情况下都会相同（此时已用 720 > 上限 100，无论 `add` 是否为 0 都必然 403），本次未回读用户行与全局默认选项确认，故不作为独立证据；`resolveDailyLimit` 已有单测覆盖，判定逻辑不因此失去覆盖。

**偏差与观察**（均不阻塞，未放宽任何验收点）：

1. 无浏览器/无真人，全部改用管理 API + 本地 stub 上游驱动；验收点本身未改。
2. 本版本的令牌 key 是 48 位裸字符串，不带 `sk-` 前缀（`random.GenerateKey()`），计划里"复制 `sk-` 开头的 key"描述不准。
3. `AddToken` 固定用调用者 id 作为 `user_id`，root 无法为 `dailytest` 建令牌，因此必须用 `dailytest` 自己登录建令牌（后续手工验收脚本需按此顺序）。
4. quota 维度的预估含 `config.PreConsumedQuota`（默认 500），即 `(500 + promptTokens + max_tokens) × 倍率`；对本例（gpt-3.5-turbo、max_tokens=50）约为 139，因此额度上限低于约 139 时当天**首个**请求就会被拒。这是设计要求的从严行为，但建议在用户手册（D8）中写明这层含义。
5. 两个维度的消息呈现不一致：token 维度用原始计数，quota 维度用货币格式——与设计一致，仅记录。
6. 本次未覆盖：流式响应、Anthropic/Audio/Image 三条准入路径（A5 已接线，本次只跑了文本路径）、跨天翻转、并发竞争同一估算值，以及 B5 落地后的用户列表「今日用量」列。

---

## 验收记录：Task B6 后端接口手工验收（2026-09-21）

**结果：通过** —— 计划的 4 个步骤全部 PASS，无产品代码改动。完整证据（逐条命令、关键请求的原始输出，以及每项结论对应的探针判定；权限与校验类探针附状态码）见 `.superpowers/sdd/2026-09-20-daily-quota-and-dashboard-filters/task-B6-report.md`；请求与状态码的独立底稿是本轮 `e2e/server.log` 的 82 条 `[GIN]` 记录。**注：并非每个探针都在报告里附了完整响应体**——报告 §0 列出了哪一档附响应体、哪一档只给逐字段数值或判定（承重的安全探针 S11/S12/S14/S16/S44 属"只给本人基线数值"那一档，判定可由 `server.log` 与 DB 现状核对）。

驱动方式：复用 A7 的 e2e harness，但需 (1) 用当前 HEAD（含 B5 `237142f`）**重建 `one-api.exe`**，(2) **重新登录**（A7 遗留 cookie 已失效），(3) 新建第二个/第三个有日志的用户——A7 遗留库里只有 `dailytest` 一人有消费日志，"全站 vs 本人"无法区分。为满足计划的"至少跨两个模型"，先用 `PUT /api/channel/` 把 stub 渠道扩为 `gpt-3.5-turbo,gpt-4`，并让 root 用自建令牌 `roott` 打 2 次 `gpt-4`；另建 `dashclean`（id=3，role 1）打 3 次 `gpt-3.5-turbo` 作为无污染对照。最终数据：root 4 次/9600、dailytest 5 次/250、dashclean 3 次/150，跨两模型、两小时桶。会话 cookie 与系统访问令牌（`Authorization: Bearer`，`GET /api/user/token`）两种身份各验一遍。

- **Step 2 总览全站与筛选 PASS**：`scope=all` 返回全站两模型（`3次/150` + `2次/4800`）；`scope=all&username=dailytest` 只剩 `gpt-3.5-turbo 3/150`；换成 `username=root` 只剩 `gpt-4 2/4800`（互补结果，证明真收窄）；`scope=all&granularity=hour` 桶标签为 `2026-09-21 10:00` / `2026-09-21 11:00`，与 DB 中 `datetime(...,'localtime')` 的 10:32:53 / 11:03:34 逐一对应。
- **Step 3 权限与校验 PASS**：非管理员（cookie 与 Bearer 两种）请求 `scope=all` 均 **403**（`无权查看全站统计`，`data:null`）；`start_timestamp=2000&end_timestamp=1000` **400**（`无效的时间范围`）；天粒度 400 天 **400**（`天粒度最多查询 366 天`），小时粒度 100 天 **400**（`小时粒度最多查询 90 天`），恰好 366 天为 200（上限是闭区间）。
- **最关键属性 1：非管理员无法通过任何参数组合拿到全站数据 PASS**：报告 §3.1 逐条列出了 12 个探针，全部只给出本人数据或空结果，无一次泄漏。其中总览接口一侧：`scope=all`（dailytest 与 dashclean 两个非管理员）403；`scope=all&username=dailytest` 403；`username=root`（无 `scope`）、`scope=self&username=root`、`scope=ALL`（大写）、无参数这四个返回本人基线（3 次/150），`scope=self&model_name=gpt-4`、`scope=self&username=root&token_name=roott&model_name=gpt-4&granularity=hour` 这两个返回 `data:null`（本人无该模型/该令牌日志），没有一个返回 root 的 4 次/9600；`/api/log/filters?username=root`（cookie 与 Bearer）仍只有本人令牌。相邻接口同样无旁路，但**注意各探针实际发出的 URL**：带 `?username=root` 的是两条 `/api/log/self/stat`（`server.log` 可见 `GET /api/log/self/stat?type=2&username=root&…`），非管理员那条返回本人 150（非全站 10000）；而 `/api/log/self` 那条**没有带 `username`**（`GetUserLogs` 只读 `ctxkey.Id`、不解析该参数，加上去也不是有效探针），它证明的是"返回行的 username 只有自己"，不是"带别人的 username 仍只看自己"；`GetLogsSelfStat` 则取会话里的 `ctxkey.Username`。**未发现 Critical 问题。**
- **最关键属性 2：管理员 `username` 筛选确实收窄 PASS**：同窗口 before（2 行/4950）→ `username=dailytest` 后 1 行/150 → `username=root` 后 1 行/4800；`username=dailytest&token_name=roott` 为 `data:null`（合取过滤，不串用户）。
- **Step 4 候选值接口 PASS**：管理员三组值均非空且含三用户/四令牌/两模型；`?username=dailytest` 把 `tokens` 收窄为 `["","t1"]`、`models` 收窄为 `["","gpt-3.5-turbo"]`（`users` 按设计不随 `username` 收窄）；非管理员 `users` 为**空数组 `[]`**，`tokens`/`models` 只有自己，`?username=root` 不改变结果。
- **Step 4b 用户列表新字段 PASS（1 个由数据解释的例外）**：`GET /api/user/?p=0` 已带 `today_tokens`/`today_quota`/`effective_daily_token_limit`/`effective_daily_quota_limit`。dashclean（360/150）与 root（480/9600）的今日值与同窗口日志统计**逐位相等**；`dailytest` 为 960/400 而日志统计为 600/250，差正好是 A7 阶段 `LogConsumeEnabled=false` 时那 3 次请求的 360 tokens/150 quota（记账独立于日志开关是设计要求），随后"每用户再打 2 次请求"的前后增量（+240 tokens；+100 与 +4800 quota）与 `daily_usage` 增量完全吻合，故判 PASS 而非 FAIL。另验 `effective_*` 三态解析：行内 `1000`→1000、`-1`（豁免）→0（不限）、`0`+全局默认 `123`→123，且全局默认不覆盖用户的显式正数；测后已把默认与用户行恢复为 0。

**偏差与观察（均不阻塞，未放宽任何验收点）**：

1. 无浏览器/无真人，全部走 HTTP API（会话 cookie + 访问令牌双身份），语义与计划的 `curl` 一致。
2. A7 的 cookie 已失效、其 `one-api.exe` 早于 B5，复用 harness 必须先重新登录并重建二进制；这是复用旧 harness 的必然步骤，非计划缺失。
3. **"今日值 == 日志统计"的前提是消费日志开启**：`LogConsumeEnabled=false` 期间的请求照常记账但不写 `type=2` 日志，故两者会有差额（本次 dailytest 即差 360/150，已归因）。将来做自动化断言应保证日志开启或使用无污染用户。
4. `/api/log/filters` 的 `tokens`/`models` 含空字符串 `""`（错误日志的 `token_name`/`model_name` 为空），源于 `SearchLogDistinctValues` 刻意不按 `type` 过滤；前端（Part C/D）需决定是否过滤空白选项，计划与设计文档未规定，不判 FAIL。
5. `effective_daily_*_limit` 无法区分"豁免（-1）"与"不限"（`resolveDailyLimit` 两者都返回 0，与"0 表示不限"的契约自洽）；用户列表若要显示"豁免"需另用行内原值。
6. `scope=ALL`（大写）不报错、按 self 处理（fail-closed，无安全问题），仅记录。
7. 本次未覆盖：三主题前端的下拉与表格渲染（Part C/D）、小时粒度跨天边界的标签衔接、并发下的聚合一致性、MySQL/PostgreSQL 分桶分支（只跑了 sqlite 的 `'localtime'` 路径）；纯逻辑分支已由 `model/log_statistic_test.go`、`model/log_distinct_test.go`、`controller/log_filters_test.go`、`controller/dashboard_test.go` 覆盖。
8. 未发现产品缺陷，无需修复项。

---

## 验收记录：Task D7 三主题整体构建与联调验收（2026-09-21）

**结果：构建与 embed 通过；逐主题联调 15 项中 13 项 PASS（2 项"PASS 但有显示缺口"）；跨任务 a–f 中 a/c/e 三项 FAIL（真实缺陷）、b/d/f 三项 PASS（b 的数值不实、f 只到 HTTP 边界）。无产品代码改动。** 完整证据（每条命令、控件显示文本、请求 URL 与响应体、截图清单、失败注入方法）见 `.superpowers/sdd/2026-09-20-daily-quota-and-dashboard-filters/task-D7-report.md`。

**驱动方式（重要，先说明证据性质）**：本任务以 subagent 身份执行，会话内的 `browser-use` 技能直接返回 `Browser is not available in subagent`，**无法使用官方浏览器工具**。改为用本机 Chrome（153.0.8010.52）以 `--headless=new --remote-debugging-port` 启动，并由自写的极小 CDP 驱动（只用 Node 内建 `WebSocket`/`fetch`，无第三方依赖）完成**真实 DOM 交互**：React 需原生 setter + `input/change`、语义文本定位下拉与选项、`Network.requestWillBeSent`/`getResponseBody` 记录请求与响应体、`Network.setBlockedURLs` 注入失败、逐张截图目视核对。结论一律用"控件显示文本 + 请求参数 + 响应体"三条独立证据交叉验证。驱动与截图在 gitignored 的 `.superpowers/sdd/2026-09-20-daily-quota-and-dashboard-filters/d7/`。

- **Step 1 三主题构建 PASS**：`sh build.sh` 退出码 0，default/berry/air 各自 `Compiled successfully`，无 error/failed 行；产物哈希与体积互不相同（`main.8859206e.js` 1,082,412 B / `main.a55d38fe.js` 551,984 B / `main.dfab458d.js` 3,770,389 B）。
- **Step 2 embed PASS**：`CGO_ENABLED=1 go build` 成功（108,151,602 B）；`/` 逐主题返回**本次构建**的 bundle —— `curl /` 的 `main.<hash>.js` 与磁盘产物哈希一致、`main.8859206e.js` 返回 `200 size=1082412` 字节一致（三个主题都验过），并在每次重启后由 `main.go:65 using theme <t>` 印证。
- **逐主题 5 项联调**：三主题的 **总览 / 用户编辑 / 运营设置 / 用户列表 / 日志页全部 PASS**。要点：默认区间都是最近 7 天（`start_timestamp=1789401600&end_timestamp=1790006399`）；管理员可切「全站/仅自己」且数字随之变化（default：self `gpt-4 4/9600` → all `gpt-3.5-turbo 8/400 + gpt-4 4/9600`；berry 卡：`4/$0.019/480` → `12/$0.020/1440`）；天/小时粒度、用户/令牌/模型三个筛选在请求参数与响应体中逐条核实；**总览与日志页同窗口数字一致**（今日仅自己 = 4 次 / 9600 额度 / 480 tokens，总览与 `/api/log/stat?type=2&username=root` 双向对上）。用户编辑三态往返（default 与 air 走页面、berry 走弹窗）都做了"保存→重开读回"闭环，并单独证明 **"保存「跟随全局默认」后原值真的归零"**（PUT 里 `daily_token_limit` 由 1000 变 `0`，DB 直读为 0）。运营设置两个全局默认保存成功、**刷新后读回 1234/5678**，测后复原。用户列表三主题都在同一列给出 token 与额度两对（`960 / 不限`、`$0.00 / $0.00`），`0` 上限渲染为「不限」。日志页三个下拉都含「全部」且选后列表/统计变化（default 11→5 行、`/api/log/stat` 给 9600；非管理员仅 token/model/type）。
- **跨任务 a–f**：
  - **a FAIL（default 与 berry 都复现）**：先选模型再让候选收窄（切用户），请求仍带 `model_name=gpt-4`（berry 实测 `…username=dailytest&model_name=gpt-4`）而**控件显示空白**（`displayOf('模型').shown === ""`），即"筛选在生效却看不见"。default 与 berry 同形（截图 `default-3a-model-blank.png`、`berry-3a-model-blank.png`）；air 因 Semi 的 `_notExist` 合成 + `withSelected` **不存在**该陷阱。
  - **b PASS（渲染优雅，数值不实）**：default 同一用户搜索行显示 `0 / 不限` 与 `$0.00 / 不限`，而分页行是 `960 / 不限` 与 `$0.00 / $0.00` —— 无 NaN/空白，但"今日 0""上限不限"是**错误的正面断言**；根因是 `/api/user/search` 不返回那四个字段（已知后端缺口）。berry/air 的搜索路径未单独实测。
  - **c FAIL（berry）**：brief 记在 air 名下的"三张头部卡"实测属 berry（air 总览只有两张 VChart 图）。窗口 2026/08/01–08/31（今天在窗口外）+ 小时粒度时，后端返回 `{"data":null}`，页面卡片仍是上一个窗口的 `12 / $0.020 / 1440`，**未按代码注释给 `'-'`** —— 根因 `web/berry/src/views/Dashboard/index.js` 的 `if (data) {…}` 把"成功但空"和"失败"一起当保留旧值处理。另：小时粒度且含今天时卡片给的是**当天所有小时箱合计**（正确）。
  - **d PASS**：air 日志页三个下拉显示选中值（初始「全部」→ 选 t1 显示「t1」→ 清回后仍显示「全部」，DOM `.semi-select-selection-text` 逐次读取）。对照：**default 与 berry 的同类控件在「全部」时是空白的**（筛选正确，仅显示缺失）。
  - **e FAIL（air）**：`setBlockedURLs` 注入失败后，"已有数据再查询"会弹「错误：Network Error」并保留旧图（可接受）；但**首次加载即失败**时页面只显示「**所选区间内暂无数据**」，无任何错误提示 → 把"请求失败"说成"区间无数据"（误导）。真实的越界区间被前端守卫拦下并提示「错误：小时粒度最多查询 90 天」，这条是诚实的。
  - **f PASS（到 HTTP 边界）**：置 `daily_token_limit=1`（已用 960）后在浏览器页面上下文用该用户令牌发起真实请求，返回 **403** 与可读消息 `今日 token 用量已达上限（已用 960 / 上限 1），请明日再试或联系管理员调整 (request id: …)`（`code=insufficient_user_daily_tokens`），`stub /stats` 计数不变（拒绝在上游之前）。**但三主题都没有可用的聊天客户端**（air 的「聊天」是外链 iframe、default 的 `/chat` 无内容），因此"用户在 UI 里看到这句话"未实测。
- **附加发现（浏览器实测，属预存在缺陷，非本特性引入）**：air 日志页点「查询」会丢掉「类型」筛选 —— `onClick={refresh}` 把 click 事件当成 `localLogType` 传下去，请求变成 `…&type=[object%20Object]…`；实测"选消费→首行变消费 ✓，再点查询→管理类行又回来而控件仍显示消费"。`git show 7f7a8dc~1:web/air/src/components/LogsTable.js` 确认 `refresh(localLogType)`/`onClick={refresh}`/`htmlType="submit"` 在特性分支之前就是这样，本分支未触碰该段。
- **非管理员矩阵（三主题全打）**：三主题总览都**没有**「数据范围/用户」控件且请求恒为 `scope=self`；日志页分别只剩 `token_name/model_name/logType`（default）、`令牌名称/模型名称/起始时间/结束时间/类型`（berry）、`令牌名称/模型名称`（air），**都没有用户名称下拉**，请求走 `/api/log/self/`，行数只有自己的日志。
- **遗留问题（按优先级，详见报告 §7）**：①Important/两个主题：模型筛选空白却仍生效；②Important/berry：空响应不更新看板，"今日"卡片显示上个窗口数字；③Important/air：首次请求失败被显示为「所选区间内暂无数据」；④Important/**预存在**/air：点查询丢类型筛选（`type=[object Object]`）；⑤Minor/default+berry：日志页三个下拉在「全部」时显示空白（air 显示「全部」），default 总览的令牌/模型同样如此；⑥Minor：搜索行"今日用量 0 / 不限"不实（后端 `/api/user/search` 缺口，default 已实测）；⑦Minor：无内置聊天客户端，上限提示无法在 UI 端到端观察。
- **过程与环境事实**：`SessionSecret = uuid.New().String()`（`common/config/config.go:27`）导致**每次重启都会话失效**，每轮都要重新登录；**主题在 `SetRouter` 时固定**（`router/web.go:18` 按 `config.Theme` 取静态目录与 index.html），故换主题必须重启（UI 按钮自述「设置主题（重启生效）」，实测一致）。测后已复原：三个用户单日上限 `0/0`、两个全局默认 `0`、`Theme=default`、`LogConsumeEnabled=true`（DB 直读确认），服务端最终以 default 运行。
- **偏差**：官方浏览器工具不可用（subagent 限制），改用本机 Chrome + 自写 CDP 驱动（见上），验收点未放宽、未修改任何产品代码；canvas 图表（air 的 VChart、berry 的 ECharts）只做了截图目视与 API 数据核对，未做像素级判定；深色模式/小屏布局、小时粒度跨天/跨时区衔接、非管理员在用户列表与令牌页的字段可见性未覆盖。
