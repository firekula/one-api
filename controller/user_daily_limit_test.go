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
	// 上一次（改回 0）的请求没有带 daily_quota_limit，nil 指针是零值会被 GORM 跳过，
	// 因此这里的 -1 必须原样保留——非法请求更不能动它。
	if got.DailyQuotaLimit == nil || *got.DailyQuotaLimit != -1 {
		t.Fatalf("非法请求不应改动其他字段: %v", got.DailyQuotaLimit)
	}
}
