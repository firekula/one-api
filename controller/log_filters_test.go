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

	// models 与 tokens 是两次独立查询，非管理员同样必须按自己收窄
	w = getLogFilters(t, 1, model.RoleCommonUser, "")
	var nonAdminModels filterResp
	if err := json.Unmarshal(w.Body.Bytes(), &nonAdminModels); err != nil {
		t.Fatalf("解析失败: %v，body=%s", err, w.Body.String())
	}
	if len(nonAdminModels.Data.Models) != 1 || nonAdminModels.Data.Models[0] != "gpt-3.5-turbo" {
		t.Fatalf("非管理员模型候选应只有自己的: %v", nonAdminModels.Data.Models)
	}

	// 作用域不可被参数反转：非管理员传 ?username=bob 也只能看到自己的令牌/模型，且没有 users
	w = getLogFilters(t, 1, model.RoleCommonUser, "?username=bob")
	var nonAdminSpoof filterResp
	if err := json.Unmarshal(w.Body.Bytes(), &nonAdminSpoof); err != nil {
		t.Fatalf("解析失败: %v，body=%s", err, w.Body.String())
	}
	if len(nonAdminSpoof.Data.Users) != 0 {
		t.Fatalf("非管理员传 username 也不应拿到 users，实际 %v", nonAdminSpoof.Data.Users)
	}
	if len(nonAdminSpoof.Data.Tokens) != 1 || nonAdminSpoof.Data.Tokens[0] != "t1" {
		t.Fatalf("非管理员传 username 不应改变令牌作用域: %v", nonAdminSpoof.Data.Tokens)
	}
	if len(nonAdminSpoof.Data.Models) != 1 || nonAdminSpoof.Data.Models[0] != "gpt-3.5-turbo" {
		t.Fatalf("非管理员传 username 不应改变模型作用域: %v", nonAdminSpoof.Data.Models)
	}

	// 管理员按 username 收窄时，models 必须与 tokens 同步收窄
	w = getLogFilters(t, 99, model.RoleRootUser, "?username=bob")
	var narrowedModels filterResp
	if err := json.Unmarshal(w.Body.Bytes(), &narrowedModels); err != nil {
		t.Fatalf("解析失败: %v，body=%s", err, w.Body.String())
	}
	if len(narrowedModels.Data.Models) != 1 || narrowedModels.Data.Models[0] != "claude-3" {
		t.Fatalf("按用户收窄模型候选失败: %v", narrowedModels.Data.Models)
	}
}
