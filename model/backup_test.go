package model

import (
	"fmt"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB 用临时 sqlite 文件库初始化全局 DB（避免 :memory: 多连接问题），
// 并在测试结束后恢复原 DB。
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	dsn := fmt.Sprintf("%s?_busy_timeout=5000&_journal_mode=WAL", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &Token{}, &Channel{}, &Redemption{}, &Option{}, &Ability{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })
	return db
}

func seedTestData(t *testing.T, db *gorm.DB) {
	t.Helper()
	users := []User{
		{Username: "root", Password: "$2a$10$hash", Role: 100, Status: 1, Quota: 500000, AccessToken: "at-root", AffCode: "aff-root"},
		{Username: "alice", Password: "$2a$10$hash2", Role: 1, Status: 1, Quota: 1000, AccessToken: "at-alice", AffCode: "aff-alice"},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	baseURL := "https://api.example.com" // Channel.BaseURL 是 *string，必须指针赋值
	channels := []Channel{
		{Type: 1, Key: "sk-channel-1", Name: "ch1", BaseURL: &baseURL, Models: "gpt-3.5-turbo", Group: "default", Status: 1},
	}
	if err := db.Create(&channels).Error; err != nil {
		t.Fatalf("seed channels: %v", err)
	}
	if err := channels[0].AddAbilities(); err != nil {
		t.Fatalf("seed abilities: %v", err)
	}
	tokens := []Token{
		{UserId: users[0].Id, Key: "token-key-1", Status: TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 500, UnlimitedQuota: false, Name: "t1"},
		{UserId: users[1].Id, Key: "token-key-2", Status: TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 100, UnlimitedQuota: false, Name: "t2"},
	}
	if err := db.Create(&tokens).Error; err != nil {
		t.Fatalf("seed tokens: %v", err)
	}
	redemptions := []Redemption{
		{Key: "red-key-1", Status: 1, Name: "r1", Quota: 1000, CreatedTime: 1},
	}
	if err := db.Create(&redemptions).Error; err != nil {
		t.Fatalf("seed redemptions: %v", err)
	}
	options := []Option{
		{Key: "SystemName", Value: "One API"},
		{Key: "GitHubClientSecret", Value: "secret-value"}, // 敏感键也必须导出
	}
	if err := db.Create(&options).Error; err != nil {
		t.Fatalf("seed options: %v", err)
	}
}

func TestExportData(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	data, err := ExportData()
	if err != nil {
		t.Fatalf("ExportData: %v", err)
	}
	if data.SchemaVersion != 1 {
		t.Fatalf("schemaVersion = %d, want 1", data.SchemaVersion)
	}
	if data.ExportedAt == 0 {
		t.Fatal("exportedAt 不应为 0")
	}
	if len(data.Data.Users) != 2 {
		t.Fatalf("users = %d, want 2", len(data.Data.Users))
	}
	if len(data.Data.Channels) != 1 {
		t.Fatalf("channels = %d, want 1", len(data.Data.Channels))
	}
	if len(data.Data.Tokens) != 2 {
		t.Fatalf("tokens = %d, want 2", len(data.Data.Tokens))
	}
	if len(data.Data.Redemptions) != 1 {
		t.Fatalf("redemptions = %d, want 1", len(data.Data.Redemptions))
	}
	if len(data.Data.Options) != 2 {
		t.Fatalf("options = %d, want 2", len(data.Data.Options))
	}
	// 渠道 Key 必须导出（迁移需要密钥）
	if data.Data.Channels[0].Key != "sk-channel-1" {
		t.Fatalf("channel key 未导出: %q", data.Data.Channels[0].Key)
	}
	// 敏感 option 必须导出
	foundSecret := false
	for _, o := range data.Data.Options {
		if o.Key == "GitHubClientSecret" {
			foundSecret = true
		}
	}
	if !foundSecret {
		t.Fatal("敏感 option 键未导出")
	}
	// 用户 password 哈希必须导出（导入时原样还原）
	if data.Data.Users[0].Password == "" {
		t.Fatal("user password 哈希未导出")
	}
}
