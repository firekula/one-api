package model

import (
	"fmt"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/songquanpeng/one-api/common/config"
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

func TestImportDataRoundTrip(t *testing.T) {
	// 库 A：造数据并导出
	dbA := setupTestDB(t)
	seedTestData(t, dbA)
	exported, err := ExportData()
	if err != nil {
		t.Fatalf("ExportData: %v", err)
	}

	// 库 B：全新空库，导入
	setupTestDB(t) // 替换为第二个临时库
	result, err := ImportData(exported)
	if err != nil {
		t.Fatalf("ImportData: %v", err)
	}
	if result.Inserted["users"] != 2 || result.Inserted["tokens"] != 2 ||
		result.Inserted["channels"] != 1 || result.Inserted["redemptions"] != 1 ||
		result.Inserted["options"] != 2 {
		t.Fatalf("inserted 统计不正确: %+v", result.Inserted)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("failed 应为空: %+v", result.Failed)
	}

	// 校验引用映射：Token.UserId 必须指向导入后的新用户
	var tk Token
	if err := DB.Where("`key` = ?", "token-key-1").First(&tk).Error; err != nil {
		t.Fatalf("find imported token: %v", err)
	}
	var u User
	if err := DB.First(&u, tk.UserId).Error; err != nil {
		t.Fatalf("imported token 引用的用户不存在: %v", err)
	}
	if u.Username != "root" {
		t.Fatalf("token.user_id 映射错误，指向 %q", u.Username)
	}
	// Channel 的 Ability 已重建
	var abilityCount int64
	DB.Model(&Ability{}).Where("channel_id = ?", 1).Count(&abilityCount)
	if abilityCount == 0 {
		t.Fatal("channel 导入后 Ability 未重建")
	}
	// 导入的 option 应同步到内存 config（ImportData 提交后调用 InitOptionMap）
	// 注：SystemName 默认值同为 "One API"，属弱断言；GitHubClientSecret 默认
	// 为空串，能真正证明 loadOptionsFromDatabase 已把导入的 option 同步到内存。
	if config.SystemName != "One API" {
		t.Fatalf("SystemName 未同步到内存 config: %q", config.SystemName)
	}
	if config.GitHubClientSecret != "secret-value" {
		t.Fatalf("GitHubClientSecret 未同步到内存 config: %q", config.GitHubClientSecret)
	}
}

func TestImportDataSkipDuplicates(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	exported, err := ExportData()
	if err != nil {
		t.Fatalf("ExportData: %v", err)
	}
	// 同一实例上二次导入：全部按唯一键跳过
	result, err := ImportData(exported)
	if err != nil {
		t.Fatalf("ImportData: %v", err)
	}
	if result.Inserted["users"] != 0 || result.Inserted["tokens"] != 0 ||
		result.Inserted["channels"] != 0 || result.Inserted["redemptions"] != 0 ||
		result.Inserted["options"] != 0 {
		t.Fatalf("二次导入应全部 skipped: %+v", result.Inserted)
	}
	for _, table := range []string{"users", "tokens", "channels", "redemptions", "options"} {
		if result.Skipped[table] == 0 {
			t.Fatalf("%s 应有 skipped 记录", table)
		}
	}
}

func TestImportDataIDConflictWithExistingRoot(t *testing.T) {
	// 目标库已有 root（id=1），导入数据里也有 root（username 相同）
	db := setupTestDB(t)
	seedTestData(t, db) // 库 A 数据（含 root, id=1）
	exported, err := ExportData()
	if err != nil {
		t.Fatalf("ExportData: %v", err)
	}

	// 目标库：也先建 root（模拟真实新部署，id=1 被占用）
	setupTestDB(t)
	db2 := DB
	root := User{Username: "root", Password: "$2a$10$newhash", Role: 100, Status: 1, Quota: 999, AccessToken: "at-new", AffCode: "aff-new"}
	if err := db2.Create(&root).Error; err != nil {
		t.Fatalf("create existing root: %v", err)
	}

	result, err := ImportData(exported)
	if err != nil {
		t.Fatalf("ImportData: %v", err)
	}
	// 目标库已有的 root 按 username 跳过，alice 插入
	if result.Skipped["users"] != 1 || result.Inserted["users"] != 1 {
		t.Fatalf("users 导入统计错误: inserted=%d skipped=%d", result.Inserted["users"], result.Skipped["users"])
	}
	// alice 的 token 应正确映射到 alice 的新 id
	var alice User
	if err := DB.Where("username = ?", "alice").First(&alice).Error; err != nil {
		t.Fatalf("find alice: %v", err)
	}
	var tk Token
	if err := DB.Where("`key` = ?", "token-key-2").First(&tk).Error; err != nil {
		t.Fatalf("find token-key-2: %v", err)
	}
	if tk.UserId != alice.Id {
		t.Fatalf("token-key-2 user_id = %d, want alice id %d", tk.UserId, alice.Id)
	}
}

func TestImportDataRejectsBadSchemaVersion(t *testing.T) {
	setupTestDB(t)
	data := &BackupData{SchemaVersion: 99}
	if _, err := ImportData(data); err == nil {
		t.Fatal("schemaVersion 99 应被拒绝")
	}
}
