package model

import (
	"testing"

	"github.com/songquanpeng/one-api/common/config"
)

func TestUpdateOptionDailyLimitDefaults(t *testing.T) {
	setupModelTestDB(t)
	InitOptionMap()

	oldToken, oldQuota := config.DailyTokenLimitDefault, config.DailyQuotaLimitDefault
	t.Cleanup(func() {
		config.DailyTokenLimitDefault = oldToken
		config.DailyQuotaLimitDefault = oldQuota
	})

	if err := UpdateOption("DailyTokenLimitDefault", "123456"); err != nil {
		t.Fatalf("写入 token 默认上限失败: %v", err)
	}
	if config.DailyTokenLimitDefault != 123456 {
		t.Fatalf("DailyTokenLimitDefault = %d，期望 123456", config.DailyTokenLimitDefault)
	}

	if err := UpdateOption("DailyQuotaLimitDefault", "789"); err != nil {
		t.Fatalf("写入额度默认上限失败: %v", err)
	}
	if config.DailyQuotaLimitDefault != 789 {
		t.Fatalf("DailyQuotaLimitDefault = %d，期望 789", config.DailyQuotaLimitDefault)
	}

	// 默认值必须是 0（不限），否则新装实例会意外限制所有人
	InitOptionMap()
	if config.OptionMap["DailyTokenLimitDefault"] == "" || config.OptionMap["DailyQuotaLimitDefault"] == "" {
		t.Fatal("两个新选项必须出现在 OptionMap 中，否则设置页读不到")
	}
}
