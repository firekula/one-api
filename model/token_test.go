package model

import (
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"
)

// 修复回归测试：CacheGetTokenByKey 返回 sqlite 的 "database is locked" 等
// 瞬态基础设施错误时，绝不能把用户当成凭据无效（401），必须标记为
// 服务端瞬态错误（ErrTransientTokenError），让上层返回 5xx 供客户端重试。
func TestClassifyTokenCacheError_RecordNotFound_NotTransient(t *testing.T) {
	err := classifyTokenCacheError(gorm.ErrRecordNotFound)
	if errors.Is(err, ErrTransientTokenError) {
		t.Fatal("record not found 不应被标记为瞬态错误")
	}
	if err.Error() != "无效的令牌" {
		t.Fatalf("期望错误消息 %q，得到 %q", "无效的令牌", err.Error())
	}
}

func TestClassifyTokenCacheError_TransientError(t *testing.T) {
	cases := []string{
		"database is locked",        // sqlite 写锁竞争（本次故障根因）
		"connection refused",        // DB/Redis 连接抖动
		"context deadline exceeded", // 查询超时
	}
	for _, raw := range cases {
		err := classifyTokenCacheError(errors.New(raw))
		if !errors.Is(err, ErrTransientTokenError) {
			t.Fatalf("%q 应被标记为瞬态错误（ErrTransientTokenError）", raw)
		}
	}
}

// sqlite DSN 必须启用 WAL 并携带 busy_timeout，否则读写会互相阻塞，
// 高并发下表现为间歇性 "database is locked"。
func TestSQLiteDSNEnablesWALAndBusyTimeout(t *testing.T) {
	dsn := sqliteDSN()
	if !strings.Contains(dsn, "_journal_mode=WAL") {
		t.Fatalf("sqlite DSN 应启用 WAL 模式，实际: %q", dsn)
	}
	if !strings.Contains(dsn, "_busy_timeout=") {
		t.Fatalf("sqlite DSN 应携带 _busy_timeout，实际: %q", dsn)
	}
}
