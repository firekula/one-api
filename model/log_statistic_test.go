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

// 测试钉住的"本地时区"：+08。
// TZ 用 POSIX 记法而不是 IANA 名："UTC-8" 表示 8 小时以东（= UTC+8），glibc 与
// Windows CRT 都按同一语义解析；"Asia/Shanghai" 这种 IANA 名 Windows 的 CRT 会
// 解析成一个带美式夏令时的垃圾时区（实测 3 月偏移 +1h），两边就对不上了。
const (
	testTZEnv        = "UTC-8"
	testTZOffsetSecs = 8 * 3600
	testTZOffsetName = "UTC+8(test)"
)

// pinNonUTCLocalZone 把「本地时间」同时钉在进程与 SQLite 两边。
// 两边都要设：SQLite 的 'localtime' 走 C 库的本地时区（认 TZ），而 Go 会缓存
// time.Local，改 TZ 不会影响它（Windows 上更完全忽略 TZ）。
// 为什么必须非 UTC：ubuntu-latest 上 TZ 未设置时，Go 的 time.Local 与 SQLite 的
// 'localtime' 都是 UTC，"23:59 与次日 00:01 落在不同日桶"会无条件成立，
// 去掉 'localtime' 修饰符测试照样通过——那个守卫就是空的。
func pinNonUTCLocalZone(t *testing.T, db *gorm.DB) {
	t.Helper()
	oldLocal := time.Local
	t.Setenv("TZ", testTZEnv)
	time.Local = time.FixedZone(testTZOffsetName, testTZOffsetSecs)
	t.Cleanup(func() { time.Local = oldLocal })

	// 再问 SQLite 一次它实际用的偏移：若为 0，说明 TZ 没被 C 库采纳（或进程更早
	// 已经缓存过别的时区），本地日与 UTC 日重合，断言会退化成恒真。宁可红，不要空守卫。
	var offset int64
	if err := db.Raw(
		`SELECT CAST(strftime('%s', datetime(0, 'unixepoch', 'localtime')) AS INTEGER)`,
	).Row().Scan(&offset); err != nil {
		t.Fatalf("读取 SQLite 本地时区偏移失败: %v", err)
	}
	if offset != testTZOffsetSecs {
		t.Fatalf("SQLite 的 'localtime' 偏移 = %d 秒，期望 %d 秒（TZ=%q 未被 C 库采纳，本断言会失去区分力）",
			offset, testTZOffsetSecs, testTZEnv)
	}
}

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
	// 只在非 UTC 时区下这个断言才有区分力：钉 +08 之后，
	// 2026-03-10 23:59 本地 = 2026-03-10 15:59 UTC，2026-03-11 00:01 本地 = 2026-03-10 16:01 UTC，
	// 去掉 'localtime' 后两条都会落进 UTC 的 2026-03-10 → 只剩 1 个桶 → 下面的 len(got) != 2 失败。
	pinNonUTCLocalZone(t, db)

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

	// 按模型收窄：alice 与 bob 各有一条 gpt-3.5-turbo，但二者同日同模型，
	// 聚合后只应剩一个桶，且 token 合计 3（1+2），claude-3 的 4+8 不得混入。
	// （简报原文此处期望 2 行，与「按 day×model_name 分组」的语义矛盾，故按真实语义修正。）
	byModel := window
	byModel.ModelName = "gpt-3.5-turbo"
	got, err = SearchLogsByDayAndModel(byModel)
	if err != nil {
		t.Fatalf("按模型查询失败: %v", err)
	}
	if len(got) != 1 || got[0].PromptTokens != 3 {
		t.Fatalf("gpt-3.5-turbo 应聚合成一个桶且 token 合计 3，实际 %+v", got)
	}
}
