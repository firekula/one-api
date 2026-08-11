package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/songquanpeng/one-api/model"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupControllerTestDB(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	dsn := fmt.Sprintf("%s?_busy_timeout=5000&_journal_mode=WAL", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Token{}, &model.Channel{}, &model.Redemption{}, &model.Option{}, &model.Ability{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB })
}

func TestGetExportData(t *testing.T) {
	setupControllerTestDB(t)
	// 插入一条数据
	u := model.User{Username: "root", Password: "hash", Role: 100, Status: 1, Quota: 100, AccessToken: "at", AffCode: "aff"}
	if err := model.DB.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	r := gin.New()
	r.GET("/api/data/export", GetExportData)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/data/export", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp struct {
		Data struct {
			SchemaVersion int `json:"schemaVersion"`
			Data          struct {
				Users []*model.User `json:"users"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Data.SchemaVersion != 1 {
		t.Fatalf("schemaVersion = %d, want 1", resp.Data.SchemaVersion)
	}
	if len(resp.Data.Data.Users) != 1 {
		t.Fatalf("users = %d, want 1", len(resp.Data.Data.Users))
	}
}

func TestImportDataEndpoint(t *testing.T) {
	setupControllerTestDB(t)
	backup := &model.BackupData{
		SchemaVersion: 1,
		ExportedAt:    123,
		Data: model.BackupTables{
			Users: []*model.User{{Username: "bob", Password: "hash", Role: 1, Status: 1, Quota: 100, AccessToken: "at-bob", AffCode: "aff-bob"}},
		},
	}
	bodyBytes, _ := json.Marshal(backup)

	// 构造 multipart 请求
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "backup.json")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(bodyBytes); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	mw.Close()

	r := gin.New()
	r.POST("/api/data/import", ImportData)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/data/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Inserted map[string]int `json:"inserted"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("success = false, body=%s", w.Body.String())
	}
	if resp.Data.Inserted["users"] != 1 {
		t.Fatalf("inserted.users = %d, want 1", resp.Data.Inserted["users"])
	}
}

func TestImportDataEndpointBadJSON(t *testing.T) {
	setupControllerTestDB(t)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "bad.json")
	fw.Write([]byte("not json"))
	mw.Close()

	r := gin.New()
	r.POST("/api/data/import", ImportData)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/data/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
