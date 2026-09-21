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
