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
