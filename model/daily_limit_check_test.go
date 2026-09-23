package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/config"
)

func withGlobalDailyLimits(t *testing.T, token, quota int64) {
	t.Helper()
	oldToken, oldQuota := config.DailyTokenLimitDefault, config.DailyQuotaLimitDefault
	config.DailyTokenLimitDefault = token
	config.DailyQuotaLimitDefault = quota
	t.Cleanup(func() {
		config.DailyTokenLimitDefault = oldToken
		config.DailyQuotaLimitDefault = oldQuota
	})
}

// dailyLimitTestUserSeq 让同一测试内创建的多个用户拿到互不相同的 username /
// access_token / aff_code。User 上这三列都是唯一的，固定值会让第二个用户插入失败。
var dailyLimitTestUserSeq int

func createUserWithDailyLimits(t *testing.T, token, quota *int64) *User {
	t.Helper()
	dailyLimitTestUserSeq++
	suffix := fmt.Sprintf("%s_%d", Today(), dailyLimitTestUserSeq)
	user := &User{
		Username:        "u_daily_" + suffix,
		Password:        "hash",
		Role:            RoleCommonUser,
		Status:          UserStatusEnabled,
		AccessToken:     "at-" + suffix,
		AffCode:         "aff-" + suffix,
		DailyTokenLimit: token,
		DailyQuotaLimit: quota,
	}
	if err := DB.Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	return user
}

func TestCheckUserDailyLimitTokenDimension(t *testing.T) {
	setupModelTestDB(t)
	withGlobalDailyLimits(t, 1000, 0)
	user := createUserWithDailyLimits(t, nil, nil)

	// 未超限
	if err := CheckUserDailyLimit(user.Id, 400, 0); err != nil {
		t.Fatalf("未超限不应拒绝: %v", err)
	}
	// 累加真实用量 900
	if err := RecordDailyUsage(user.Id, 900, 0, 0); err != nil {
		t.Fatalf("记账失败: %v", err)
	}
	// 恰好等于上限：放行
	if err := CheckUserDailyLimit(user.Id, 100, 0); err != nil {
		t.Fatalf("恰好等于上限应放行: %v", err)
	}
	// 超出 1 个 token：拒绝
	err := CheckUserDailyLimit(user.Id, 101, 0)
	if err == nil {
		t.Fatal("超出上限应拒绝")
	}
	limitErr, ok := AsDailyLimitError(err)
	if !ok {
		t.Fatalf("应为 *DailyLimitError，实际 %T: %v", err, err)
	}
	if limitErr.Dimension != "token" || limitErr.Used != 900 || limitErr.Limit != 1000 || limitErr.Estimated != 101 {
		t.Fatalf("错误详情不对: %+v", limitErr)
	}
	// 提示必须把"本次预估"也报出来：判定用的是"已用 + 本次预估"，只报"已用"
	// 时（本次预估单独超限的场景，已用可能为 0）会让人误以为功能失效。
	msg := err.Error()
	for _, want := range []string{"900", "101", "1000", "本次预估"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("提示缺少 %q: %s", want, msg)
		}
	}
}

func TestCheckUserDailyLimitQuotaDimension(t *testing.T) {
	setupModelTestDB(t)
	withGlobalDailyLimits(t, 0, 500)
	user := createUserWithDailyLimits(t, nil, nil)

	if err := RecordDailyUsage(user.Id, 999, 999, 480); err != nil {
		t.Fatalf("记账失败: %v", err)
	}
	// token 维度不限，即使 token 早已远超也不该拒绝
	if err := CheckUserDailyLimit(user.Id, 100000, 20); err != nil {
		t.Fatalf("token 维度不限时不应拒绝: %v", err)
	}
	err := CheckUserDailyLimit(user.Id, 0, 21)
	if err == nil {
		t.Fatal("额度超出应拒绝")
	}
	limitErr, _ := AsDailyLimitError(err)
	if limitErr.Dimension != "quota" {
		t.Fatalf("维度应为 quota，实际 %s", limitErr.Dimension)
	}
	// 额度维度三个数字都必须按 common.LogQuota 渲染，且同样包含"本次预估"
	msg := err.Error()
	for _, want := range []string{common.LogQuota(480), common.LogQuota(21), common.LogQuota(500), "本次预估"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("提示缺少 %q: %s", want, msg)
		}
	}
}

// TestDailyLimitErrorShowsEstimateWhenNothingUsed 复现用户遇到的误导场景：
// 已用 0、本次预估 60010、上限 50000 时，提示里必须同时出现这三个数字。
func TestDailyLimitErrorShowsEstimateWhenNothingUsed(t *testing.T) {
	msg := (&DailyLimitError{Dimension: "token", Used: 0, Limit: 50000, Estimated: 60010}).Error()
	want := "今日 token 用量已达上限（已用 0 + 本次预估 60010 / 上限 50000），请明日再试或联系管理员调整"
	if msg != want {
		t.Fatalf("提示文案不符:\n实际: %s\n期望: %s", msg, want)
	}
}

func TestCheckUserDailyLimitExemptAndCustom(t *testing.T) {
	setupModelTestDB(t)
	withGlobalDailyLimits(t, 100, 100)

	exempt := createUserWithDailyLimits(t, ptr(-1), ptr(-1))
	if err := RecordDailyUsage(exempt.Id, 100000, 100000, 100000); err != nil {
		t.Fatalf("记账失败: %v", err)
	}
	if err := CheckUserDailyLimit(exempt.Id, 100000, 100000); err != nil {
		t.Fatalf("豁免用户不应被拒绝: %v", err)
	}

	custom := createUserWithDailyLimits(t, ptr(50), ptr(0))
	if err := CheckUserDailyLimit(custom.Id, 40, 100); err != nil {
		t.Fatalf("token 走单独上限 50 时 40 应放行: %v", err)
	}
	if err := CheckUserDailyLimit(custom.Id, 60, 100); err == nil {
		t.Fatal("token 超过单独上限 50 应拒绝（即使低于全局 100）")
	}
	// 额度维度显式 0 = 跟随全局 100
	if err := CheckUserDailyLimit(custom.Id, 10, 90); err != nil {
		t.Fatalf("额度跟随全局 100 时 90 应放行: %v", err)
	}
	if err := CheckUserDailyLimit(custom.Id, 10, 110); err == nil {
		t.Fatal("额度超过全局 100 应拒绝")
	}
}

func TestCheckUserDailyLimitDisabledByDefault(t *testing.T) {
	setupModelTestDB(t)
	withGlobalDailyLimits(t, 0, 0)
	user := createUserWithDailyLimits(t, nil, nil)

	if err := RecordDailyUsage(user.Id, 999999999, 999999999, 999999999); err != nil {
		t.Fatalf("记账失败: %v", err)
	}
	if err := CheckUserDailyLimit(user.Id, 999999999, 999999999); err != nil {
		t.Fatalf("全局与用户都不限时不应拒绝: %v", err)
	}
}
