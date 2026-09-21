package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/songquanpeng/one-api/model"
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
