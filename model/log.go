package model

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/helper"
	"github.com/songquanpeng/one-api/common/logger"
)

type Log struct {
	Id                int    `json:"id"`
	UserId            int    `json:"user_id" gorm:"index"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index:idx_created_at_type"`
	Type              int    `json:"type" gorm:"index:idx_created_at_type"`
	Content           string `json:"content"`
	Username          string `json:"username" gorm:"index:index_username_model_name,priority:2;default:''"`
	TokenName         string `json:"token_name" gorm:"index;default:''"`
	ModelName         string `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota             int    `json:"quota" gorm:"default:0"`
	PromptTokens      int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens  int    `json:"completion_tokens" gorm:"default:0"`
	ChannelId         int    `json:"channel" gorm:"index"`
	RequestId         string `json:"request_id" gorm:"default:''"`
	ElapsedTime       int64  `json:"elapsed_time" gorm:"default:0"` // unit is ms
	IsStream          bool   `json:"is_stream" gorm:"default:false"`
	SystemPromptReset bool   `json:"system_prompt_reset" gorm:"default:false"`
}

const (
	LogTypeUnknown = iota
	LogTypeTopup
	LogTypeConsume
	LogTypeManage
	LogTypeSystem
	LogTypeTest
)

func recordLogHelper(ctx context.Context, log *Log) {
	requestId := helper.GetRequestID(ctx)
	log.RequestId = requestId
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.Error(ctx, "failed to record log: "+err.Error())
		return
	}
	logger.Infof(ctx, "record log: %+v", log)
}

func RecordLog(ctx context.Context, userId int, logType int, content string) {
	if logType == LogTypeConsume && !config.LogConsumeEnabled {
		return
	}
	log := &Log{
		UserId:    userId,
		Username:  GetUsernameById(userId),
		CreatedAt: helper.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	recordLogHelper(ctx, log)
}

func RecordTopupLog(ctx context.Context, userId int, content string, quota int) {
	log := &Log{
		UserId:    userId,
		Username:  GetUsernameById(userId),
		CreatedAt: helper.GetTimestamp(),
		Type:      LogTypeTopup,
		Content:   content,
		Quota:     quota,
	}
	recordLogHelper(ctx, log)
}

func RecordConsumeLog(ctx context.Context, log *Log) {
	if !config.LogConsumeEnabled {
		return
	}
	log.Username = GetUsernameById(log.UserId)
	log.CreatedAt = helper.GetTimestamp()
	log.Type = LogTypeConsume
	recordLogHelper(ctx, log)
}

func RecordTestLog(ctx context.Context, log *Log) {
	log.CreatedAt = helper.GetTimestamp()
	log.Type = LogTypeTest
	recordLogHelper(ctx, log)
}

func GetAllLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int) (logs []*Log, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB
	} else {
		tx = LOG_DB.Where("type = ?", logType)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if channel != 0 {
		tx = tx.Where("channel_id = ?", channel)
	}
	err = tx.Order("id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	return logs, err
}

func GetUserLogs(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int) (logs []*Log, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB.Where("user_id = ?", userId)
	} else {
		tx = LOG_DB.Where("user_id = ? and type = ?", userId, logType)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	err = tx.Order("id desc").Limit(num).Offset(startIdx).Omit("id").Find(&logs).Error
	return logs, err
}

func SearchAllLogs(keyword string) (logs []*Log, err error) {
	err = LOG_DB.Where("type = ? or content LIKE ?", keyword, keyword+"%").Order("id desc").Limit(config.MaxRecentItems).Find(&logs).Error
	return logs, err
}

func SearchUserLogs(userId int, keyword string) (logs []*Log, err error) {
	err = LOG_DB.Where("user_id = ? and type = ?", userId, keyword).Order("id desc").Limit(config.MaxRecentItems).Omit("id").Find(&logs).Error
	return logs, err
}

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int) (quota int64) {
	ifnull := "ifnull"
	if common.UsingPostgreSQL {
		ifnull = "COALESCE"
	}
	tx := LOG_DB.Table("logs").Select(fmt.Sprintf("%s(sum(quota),0)", ifnull))
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	if channel != 0 {
		tx = tx.Where("channel_id = ?", channel)
	}
	tx.Where("type = ?", LogTypeConsume).Scan(&quota)
	return quota
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) (token int) {
	ifnull := "ifnull"
	if common.UsingPostgreSQL {
		ifnull = "COALESCE"
	}
	tx := LOG_DB.Table("logs").Select(fmt.Sprintf("%s(sum(prompt_tokens),0) + %s(sum(completion_tokens),0)", ifnull, ifnull))
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	tx.Where("type = ?", LogTypeConsume).Scan(&token)
	return token
}

func DeleteOldLog(targetTimestamp int64) (int64, error) {
	result := LOG_DB.Where("created_at < ?", targetTimestamp).Delete(&Log{})
	return result.RowsAffected, result.Error
}

type LogStatistic struct {
	Day              string `gorm:"column:day"`
	ModelName        string `gorm:"column:model_name"`
	RequestCount     int    `gorm:"column:request_count"`
	Quota            int    `gorm:"column:quota"`
	PromptTokens     int    `gorm:"column:prompt_tokens"`
	CompletionTokens int    `gorm:"column:completion_tokens"`
}

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
	// 先初始化成空切片：Go 的 nil 切片会被序列化成 JSON 的 null，而响应契约里的
	// data 是数组。前端拿到 null 时（berry 的 `if (data)` 守卫）会整段跳过状态更新，
	// 卡片就留着上一个区间的数字配「今日」标题。空区间返回 [] 让每个消费者都拿到数组。
	LogStatistics = make([]*LogStatistic, 0)
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
