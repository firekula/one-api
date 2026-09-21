package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/songquanpeng/one-api/common/config"
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
			Id                       int    `json:"id"`
			Username                 string `json:"username"`
			Quota                    int64  `json:"quota"`
			TodayTokens              int64  `json:"today_tokens"`
			TodayQuota               int64  `json:"today_quota"`
			EffectiveDailyTokenLimit int64  `json:"effective_daily_token_limit"`
			EffectiveDailyQuotaLimit int64  `json:"effective_daily_quota_limit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析失败: %v，body=%s", err, w.Body.String())
	}
	if !resp.Success || len(resp.Data) != 1 {
		t.Fatalf("响应不符: %s", w.Body.String())
	}
	// 内嵌 *model.User 必须被展平到同一层：原有字段（id/username/quota）仍在顶层，
	// 而不是被包成一个嵌套对象，否则管理端列表会读不到用户信息。
	if resp.Data[0].Id != u.Id || resp.Data[0].Username != "bob" || resp.Data[0].Quota != 100 {
		t.Fatalf("原有用户字段未保留在顶层: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"User"`) {
		t.Fatalf("内嵌用户对象不应被嵌套输出: %s", w.Body.String())
	}
	if resp.Data[0].TodayTokens != 150 || resp.Data[0].TodayQuota != 45 {
		t.Fatalf("今日用量错误: %+v", resp.Data[0])
	}
	if resp.Data[0].EffectiveDailyTokenLimit != 5000 {
		t.Fatalf("生效 token 上限应取用户单独上限 5000，实际 %d", resp.Data[0].EffectiveDailyTokenLimit)
	}
}

// TestGetAllUsersEffectiveLimitFollowsGlobalDefault 覆盖"用户字段为 0 = 跟随全局默认"
// 这一关键语义：此时 effective_* 必须报出全局默认值而不是 0。同时顺带断言没有今日用量行
// 的用户仍然出现在列表里且今日用量为 0（缺失 map 条目不是错误）。
func TestGetAllUsersEffectiveLimitFollowsGlobalDefault(t *testing.T) {
	setupControllerTestDB(t)
	oldToken, oldQuota := config.DailyTokenLimitDefault, config.DailyQuotaLimitDefault
	config.DailyTokenLimitDefault = 7777
	config.DailyQuotaLimitDefault = 888
	t.Cleanup(func() {
		config.DailyTokenLimitDefault = oldToken
		config.DailyQuotaLimitDefault = oldQuota
	})

	u := &model.User{Username: "carol", Password: "hash", Role: model.RoleCommonUser, Status: 1, Quota: 100, AccessToken: "at-carol", AffCode: "aff-carol"}
	if err := model.DB.Create(u).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	r := gin.New()
	r.GET("/api/user/", GetAllUsers)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/user/?p=0", nil)
	r.ServeHTTP(w, req)

	var resp struct {
		Success bool `json:"success"`
		Data    []struct {
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
		t.Fatalf("无今日用量的用户也必须出现在列表里: %s", w.Body.String())
	}
	if resp.Data[0].TodayTokens != 0 || resp.Data[0].TodayQuota != 0 {
		t.Fatalf("无用量行应渲染为 0: %+v", resp.Data[0])
	}
	if resp.Data[0].EffectiveDailyTokenLimit != 7777 {
		t.Fatalf("用户字段为 0 应跟随全局默认 7777，实际 %d", resp.Data[0].EffectiveDailyTokenLimit)
	}
	if resp.Data[0].EffectiveDailyQuotaLimit != 888 {
		t.Fatalf("用户字段为 0 应跟随全局默认 888，实际 %d", resp.Data[0].EffectiveDailyQuotaLimit)
	}
}

// TestGetAllUsersQueriesTodayUsageOncePerPage 锁定"每页只查一次、不是每行一次"这一
// 约束：整页三个用户，daily_usage 的 SELECT 必须恰好是 1 次。
func TestGetAllUsersQueriesTodayUsageOncePerPage(t *testing.T) {
	setupControllerTestDB(t)
	for _, name := range []string{"u1", "u2", "u3"} {
		u := &model.User{Username: name, Password: "hash", Role: model.RoleCommonUser, Status: 1, Quota: 100,
			AccessToken: "at-" + name, AffCode: "aff-" + name}
		if err := model.DB.Create(u).Error; err != nil {
			t.Fatalf("创建用户失败: %v", err)
		}
		if err := model.RecordDailyUsage(u.Id, 10, 1, 2); err != nil {
			t.Fatalf("记账失败: %v", err)
		}
	}

	var dailyQueries int
	// 回调注册在这条测试独享的 DB 连接上（setupControllerTestDB 每次都新开一条），
	// 因此无需注销；gorm 的 Remove 会打一条 warn 日志，反而污染测试输出。
	if err := model.DB.Callback().Query().After("gorm:query").Register("test_count_daily_usage_queries", func(tx *gorm.DB) {
		if strings.Contains(tx.Statement.SQL.String(), "daily_usage") {
			dailyQueries++
		}
	}); err != nil {
		t.Fatalf("注册回调失败: %v", err)
	}

	r := gin.New()
	r.GET("/api/user/", GetAllUsers)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/user/?p=0", nil)
	r.ServeHTTP(w, req)

	var resp struct {
		Success bool `json:"success"`
		Data    []struct {
			TodayTokens int64 `json:"today_tokens"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析失败: %v，body=%s", err, w.Body.String())
	}
	if len(resp.Data) != 3 {
		t.Fatalf("应有 3 个用户: %s", w.Body.String())
	}
	for _, item := range resp.Data {
		if item.TodayTokens != 11 {
			t.Fatalf("今日 token 应为 11: %s", w.Body.String())
		}
	}
	if dailyQueries != 1 {
		t.Fatalf("daily_usage 查询次数 = %d，整页应恰好 1 次（每页一次，不是每行一次）", dailyQueries)
	}
}
