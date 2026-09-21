package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/songquanpeng/one-api/common"
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

// decodeDashboardBuckets 把 200 响应的桶按模型名汇总成 "模型 -> prompt_tokens 合计"，
// 用来断言"这一份响应里到底有没有别人的数据"。
func decodeDashboardBuckets(t *testing.T, w *httptest.ResponseRecorder) map[string]int {
	t.Helper()
	var resp struct {
		Success bool `json:"success"`
		Data    []struct {
			Day          string `json:"Day"`
			ModelName    string `json:"ModelName"`
			PromptTokens int    `json:"PromptTokens"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v，body=%s", err, w.Body.String())
	}
	if !resp.Success {
		t.Fatalf("响应 success=false: %s", w.Body.String())
	}
	out := map[string]int{}
	for _, row := range resp.Data {
		out[row.ModelName] += row.PromptTokens
	}
	return out
}

// TestDashboardAllScopeReturnsOtherUsersRows 覆盖这条特性真正的授权属性：
// 管理员 scope=all 必须**看到别人的数据**，默认（self）与按用户名收窄又必须收得回去。
// 已提交的用例只有 403/400 两条负路径，没有 200 路径——把 query.UserId 无条件写成 0
// （人人可看全站）或把 scope=all 忽略掉（管理员永远只看自己）都能让它们保持全绿，
// 所以这里必须有一条能红的正路径。
func TestDashboardAllScopeReturnsOtherUsersRows(t *testing.T) {
	setupControllerTestDB(t)
	// 分桶 SQL 按方言分支：测试库是 sqlite，必须显式声明，否则走 MySQL 分支
	// （DATE_FORMAT/FROM_UNIXTIME 在 sqlite 里不存在，查询直接报 no such function）。
	oldSQLite, oldPG, oldMySQL := common.UsingSQLite, common.UsingPostgreSQL, common.UsingMySQL
	common.UsingSQLite, common.UsingPostgreSQL, common.UsingMySQL = true, false, false
	t.Cleanup(func() {
		common.UsingSQLite, common.UsingPostgreSQL, common.UsingMySQL = oldSQLite, oldPG, oldMySQL
	})

	now := time.Now().Unix()
	rows := []model.Log{
		{UserId: 1, Username: "root", TokenName: "t1", ModelName: "admin-model", Type: model.LogTypeConsume,
			CreatedAt: now - 600, PromptTokens: 10, Quota: 100},
		{UserId: 2, Username: "bob", TokenName: "t2", ModelName: "bob-model", Type: model.LogTypeConsume,
			CreatedAt: now - 600, PromptTokens: 20, Quota: 200},
		// 充值日志不参与统计，用来确认过滤条件没被放宽
		{UserId: 2, Username: "bob", TokenName: "t2", ModelName: "bob-topup", Type: model.LogTypeTopup,
			CreatedAt: now - 600, PromptTokens: 999, Quota: 999},
	}
	if err := model.LOG_DB.Create(&rows).Error; err != nil {
		t.Fatalf("写入日志失败: %v", err)
	}
	window := "?start_timestamp=" + itoa(now-3600) + "&end_timestamp=" + itoa(now+3600) + "&granularity=day"

	// 1) scope=all：两个用户的桶都必须在
	w := getDashboard(t, 1, model.RoleAdminUser, window+"&scope=all")
	if w.Code != http.StatusOK {
		t.Fatalf("管理员 scope=all 应 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	all := decodeDashboardBuckets(t, w)
	if all["admin-model"] != 10 || all["bob-model"] != 20 {
		t.Fatalf("全站视图必须同时包含两个用户的桶，实际 %+v，body=%s", all, w.Body.String())
	}
	if all["bob-topup"] != 0 {
		t.Fatalf("充值日志不得进入统计，实际 %+v，body=%s", all, w.Body.String())
	}

	// 2) 默认（不传 scope，即 self）：只能看到自己的桶
	w = getDashboard(t, 1, model.RoleAdminUser, window)
	if w.Code != http.StatusOK {
		t.Fatalf("默认 self 应 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	self := decodeDashboardBuckets(t, w)
	if self["admin-model"] != 10 {
		t.Fatalf("self 视图应包含自己的桶，实际 %+v，body=%s", self, w.Body.String())
	}
	if self["bob-model"] != 0 {
		t.Fatalf("self 视图不得包含他人数据，实际 %+v，body=%s", self, w.Body.String())
	}

	// 3) scope=all + username 收窄：只剩被选中用户
	w = getDashboard(t, 1, model.RoleAdminUser, window+"&scope=all&username=bob")
	if w.Code != http.StatusOK {
		t.Fatalf("按用户名收窄应 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	byUser := decodeDashboardBuckets(t, w)
	if byUser["bob-model"] != 20 {
		t.Fatalf("按用户名收窄应只含 bob 的桶，实际 %+v，body=%s", byUser, w.Body.String())
	}
	if byUser["admin-model"] != 0 {
		t.Fatalf("按用户名收窄不得混入其他用户，实际 %+v，body=%s", byUser, w.Body.String())
	}
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
