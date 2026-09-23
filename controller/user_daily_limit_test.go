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

func postManageUser(t *testing.T, id int, role int, body map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	r := gin.New()
	r.POST("/api/user/manage", withIdentity(id, role), ManageUser)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/manage", bytes.NewBuffer(payload))
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
	// 上一次（改回 0）的请求没有带 daily_quota_limit，nil 指针是零值会被 GORM 跳过，
	// 因此这里的 -1 必须原样保留——非法请求更不能动它。
	if got.DailyQuotaLimit == nil || *got.DailyQuotaLimit != -1 {
		t.Fatalf("非法请求不应改动其他字段: %v", got.DailyQuotaLimit)
	}
}

// TestManageUserResetDailyUsage 覆盖管理员重置当日用量的完整链路：POST /api/user/manage
// 必须真的删掉今天的 daily_usage 行（两个维度一起清零），历史日期的行不受影响。
func TestManageUserResetDailyUsage(t *testing.T) {
	setupControllerTestDB(t)
	root := &model.User{Username: "root", Password: "hash", Role: model.RoleRootUser, Status: 1, Quota: 100, AccessToken: "at-root2", AffCode: "aff-root2"}
	if err := model.DB.Create(root).Error; err != nil {
		t.Fatalf("创建 root 失败: %v", err)
	}
	target := &model.User{Username: "bob", Password: "hash", Role: model.RoleCommonUser, Status: 1, Quota: 100, AccessToken: "at-bob2", AffCode: "aff-bob2"}
	if err := model.DB.Create(target).Error; err != nil {
		t.Fatalf("创建目标用户失败: %v", err)
	}
	// 昨天也留一条记录，验证重置只影响今天。
	yesterday := model.DailyUsage{UserId: target.Id, Day: "2020-01-01", PromptTokens: 999, CompletionTokens: 1, Quota: 999}
	if err := model.DB.Create(&yesterday).Error; err != nil {
		t.Fatalf("写入历史记录失败: %v", err)
	}

	if err := model.RecordDailyUsage(target.Id, 10, 5, 7); err != nil {
		t.Fatalf("记账失败: %v", err)
	}
	usage, err := model.GetDailyUsage(target.Id, model.Today())
	if err != nil {
		t.Fatalf("重置前读取失败: %v", err)
	}
	if usage.PromptTokens != 10 || usage.CompletionTokens != 5 || usage.Quota != 7 {
		t.Fatalf("重置前用量应为 10/5/7，实际 %d/%d/%d", usage.PromptTokens, usage.CompletionTokens, usage.Quota)
	}

	w := postManageUser(t, root.Id, root.Role, map[string]interface{}{
		"username": target.Username,
		"action":   "reset_daily_usage",
	})
	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if !resp.Success {
		t.Fatalf("重置当日用量应成功，响应: %s", w.Body.String())
	}

	usage, err = model.GetDailyUsage(target.Id, model.Today())
	if err != nil {
		t.Fatalf("重置后读取失败: %v", err)
	}
	if usage.PromptTokens != 0 || usage.CompletionTokens != 0 || usage.Quota != 0 {
		t.Fatalf("重置后今天的用量应清零，实际 %d/%d/%d", usage.PromptTokens, usage.CompletionTokens, usage.Quota)
	}
	var count int64
	if err := model.DB.Model(&model.DailyUsage{}).Where("user_id = ? AND day = ?", target.Id, model.Today()).Count(&count).Error; err != nil {
		t.Fatalf("统计今日行数失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("今天的 daily_usage 行应被删除，实际仍有 %d 行", count)
	}
	// 历史日期不受影响
	old, err := model.GetDailyUsage(target.Id, "2020-01-01")
	if err != nil {
		t.Fatalf("读取历史日期失败: %v", err)
	}
	if old.PromptTokens != 999 {
		t.Fatalf("历史日期的用量不应被重置，实际 prompt=%d", old.PromptTokens)
	}

	// 今天已无记录时再重置一次也必须成功（无操作而非报错）
	w = postManageUser(t, root.Id, root.Role, map[string]interface{}{
		"username": target.Username,
		"action":   "reset_daily_usage",
	})
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if !resp.Success {
		t.Fatalf("无今日记录时重置也应成功，响应: %s", w.Body.String())
	}
}
