package model

import (
	"testing"

	"github.com/songquanpeng/one-api/common/config"
)

func ptr(v int64) *int64 { return &v }

func TestResolveDailyLimit(t *testing.T) {
	cases := []struct {
		name          string
		override      *int64
		globalDefault int64
		want          int64
	}{
		{"未设置且无全局默认", nil, 0, 0},
		{"未设置跟随全局", nil, 5000, 5000},
		{"显式 0 跟随全局", ptr(0), 5000, 5000},
		{"显式 0 且全局为 0", ptr(0), 0, 0},
		{"豁免覆盖全局", ptr(-1), 5000, 0},
		{"豁免且全局为 0", ptr(-1), 0, 0},
		{"单独上限覆盖全局", ptr(100), 5000, 100},
		{"单独上限小于全局", ptr(10), 5000, 10},
		{"非法的更小负数按豁免处理", ptr(-5), 5000, 0},
		{"全局默认被写成负数按不限处理", nil, -3, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveDailyLimit(tc.override, tc.globalDefault); got != tc.want {
				t.Fatalf("resolveDailyLimit(%v, %d) = %d，期望 %d", tc.override, tc.globalDefault, got, tc.want)
			}
		})
	}
}

func TestUserEffectiveDailyLimits(t *testing.T) {
	oldToken, oldQuota := config.DailyTokenLimitDefault, config.DailyQuotaLimitDefault
	config.DailyTokenLimitDefault = 1000
	config.DailyQuotaLimitDefault = 2000
	t.Cleanup(func() {
		config.DailyTokenLimitDefault = oldToken
		config.DailyQuotaLimitDefault = oldQuota
	})

	user := &User{}
	if user.EffectiveDailyTokenLimit() != 1000 {
		t.Fatalf("token 上限应跟随全局 1000，实际 %d", user.EffectiveDailyTokenLimit())
	}
	if user.EffectiveDailyQuotaLimit() != 2000 {
		t.Fatalf("额度上限应跟随全局 2000，实际 %d", user.EffectiveDailyQuotaLimit())
	}

	user = &User{DailyTokenLimit: ptr(-1), DailyQuotaLimit: ptr(50)}
	if user.EffectiveDailyTokenLimit() != 0 {
		t.Fatalf("token 应被豁免（0=不限），实际 %d", user.EffectiveDailyTokenLimit())
	}
	if user.EffectiveDailyQuotaLimit() != 50 {
		t.Fatalf("额度应使用单独上限 50，实际 %d", user.EffectiveDailyQuotaLimit())
	}
}
