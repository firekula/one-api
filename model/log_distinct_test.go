package model

import (
	"testing"
)

func TestSearchLogDistinctValues(t *testing.T) {
	db := setupLogTestDB(t)

	rows := []Log{
		{UserId: 1, Username: "alice", TokenName: "t1", ModelName: "gpt-3.5-turbo", Type: LogTypeConsume, CreatedAt: localTime(2026, 3, 10, 10, 0, 0)},
		{UserId: 2, Username: "bob", TokenName: "t2", ModelName: "claude-3", Type: LogTypeConsume, CreatedAt: localTime(2026, 3, 10, 11, 0, 0)},
		{UserId: 2, Username: "bob", TokenName: "t2", ModelName: "claude-3", Type: LogTypeConsume, CreatedAt: localTime(2026, 3, 10, 12, 0, 0)},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("写入日志失败: %v", err)
	}
	start, end := localTime(2026, 3, 10, 0, 0, 0), localTime(2026, 3, 10, 23, 59, 59)

	// 全站用户名去重
	users, err := SearchLogDistinctValues(0, "", start, end, "username", 500)
	if err != nil {
		t.Fatalf("查询用户名失败: %v", err)
	}
	if len(users) != 2 || users[0] != "alice" || users[1] != "bob" {
		t.Fatalf("用户名去重结果错误: %v", users)
	}

	// 按用户收窄
	tokens, err := SearchLogDistinctValues(1, "", start, end, "token_name", 500)
	if err != nil {
		t.Fatalf("查询令牌名失败: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "t1" {
		t.Fatalf("用户 1 的令牌候选错误: %v", tokens)
	}

	// 按用户名收窄令牌
	tokens, err = SearchLogDistinctValues(0, "bob", start, end, "token_name", 500)
	if err != nil {
		t.Fatalf("按用户名收窄令牌失败: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "t2" {
		t.Fatalf("bob 的令牌候选错误: %v", tokens)
	}

	// 非白名单字段必须被拒绝
	if _, err := SearchLogDistinctValues(0, "", start, end, "password", 500); err == nil {
		t.Fatal("非白名单字段应报错")
	}

	// 时间区间之外的值不应出现
	otherUsers, err := SearchLogDistinctValues(0, "", localTime(2020, 1, 1, 0, 0, 0), localTime(2020, 1, 2, 0, 0, 0), "username", 500)
	if err != nil {
		t.Fatalf("区间外查询失败: %v", err)
	}
	if len(otherUsers) != 0 {
		t.Fatalf("区间外应无候选值，实际 %v", otherUsers)
	}
}

func TestSearchLogDistinctValuesRespectsLimit(t *testing.T) {
	db := setupLogTestDB(t)

	for i := 0; i < 5; i++ {
		row := Log{UserId: 1, Username: "u" + string(rune('a'+i)), ModelName: "m", Type: LogTypeConsume, CreatedAt: localTime(2026, 3, 10, 10, i, 0)}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("写入日志失败: %v", err)
		}
	}
	got, err := SearchLogDistinctValues(0, "", localTime(2026, 3, 10, 0, 0, 0), localTime(2026, 3, 10, 23, 59, 59), "username", 3)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("应被 LIMIT 3 截断，实际 %d 条", len(got))
	}
}

// 补充覆盖：limit <= 0 必须回退到默认上限，而不是被当成 LIMIT 0 返回空列表；
// 同时覆盖白名单里第三个维度 model_name（前两个由上面的测试覆盖）。
func TestSearchLogDistinctValuesDefaultsLimitAndModelField(t *testing.T) {
	db := setupLogTestDB(t)

	rows := []Log{
		{UserId: 1, Username: "alice", ModelName: "gpt-4", Type: LogTypeConsume, CreatedAt: localTime(2026, 3, 10, 10, 0, 0)},
		{UserId: 1, Username: "alice", ModelName: "claude-3", Type: LogTypeConsume, CreatedAt: localTime(2026, 3, 10, 11, 0, 0)},
		{UserId: 1, Username: "alice", ModelName: "claude-3", Type: LogTypeConsume, CreatedAt: localTime(2026, 3, 10, 12, 0, 0)},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("写入日志失败: %v", err)
	}
	start, end := localTime(2026, 3, 10, 0, 0, 0), localTime(2026, 3, 10, 23, 59, 59)

	models, err := SearchLogDistinctValues(0, "", start, end, "model_name", 0)
	if err != nil {
		t.Fatalf("查询模型名失败: %v", err)
	}
	if len(models) != 2 || models[0] != "claude-3" || models[1] != "gpt-4" {
		t.Fatalf("模型名去重结果错误（limit<=0 应回退默认上限）: %v", models)
	}

	// 非白名单字段返回错误而不是空列表
	if got, err := SearchLogDistinctValues(0, "", start, end, "content", 0); err == nil || got != nil {
		t.Fatalf("非白名单字段应返回错误且不返回取值: got=%v err=%v", got, err)
	}
}
