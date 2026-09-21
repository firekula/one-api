package billing

import (
	"context"
	"fmt"

	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/model"
)

func ReturnPreConsumedQuota(ctx context.Context, preConsumedQuota int64, tokenId int) {
	if preConsumedQuota != 0 {
		go func(ctx context.Context) {
			// return pre-consumed quota
			err := model.PostConsumeTokenQuota(tokenId, -preConsumedQuota)
			if err != nil {
				logger.Error(ctx, "error return pre-consumed quota: "+err.Error())
			}
		}(ctx)
	}
}

func PostConsumeQuota(ctx context.Context, tokenId int, quotaDelta int64, totalQuota int64, userId int, channelId int, modelRatio float64, groupRatio float64, modelName string, tokenName string) {
	// quotaDelta is remaining quota to be consumed
	err := model.PostConsumeTokenQuota(tokenId, quotaDelta)
	if err != nil {
		logger.SysError("error consuming token remain quota: " + err.Error())
	}
	err = model.CacheUpdateUserQuota(ctx, userId)
	if err != nil {
		logger.SysError("error update user quota cache: " + err.Error())
	}
	// totalQuota is total quota consumed
	if totalQuota != 0 {
		logContent := fmt.Sprintf("倍率：%.2f × %.2f", modelRatio, groupRatio)
		model.RecordConsumeLog(ctx, &model.Log{
			UserId:           userId,
			ChannelId:        channelId,
			PromptTokens:     int(totalQuota),
			CompletionTokens: 0,
			ModelName:        modelName,
			TokenName:        tokenName,
			Quota:            int(totalQuota),
			Content:          logContent,
		})
		model.UpdateUserUsedQuotaAndRequestCount(userId, totalQuota)
		model.UpdateChannelUsedQuota(channelId, totalQuota)
		// 音频的 token 字段不可信（billing 把 quota 写进了 PromptTokens），只累计额度
		if err := model.RecordDailyUsage(userId, 0, 0, totalQuota); err != nil {
			logger.SysError("error recording daily usage: " + err.Error())
		}
	}
	if totalQuota <= 0 {
		logger.Error(ctx, fmt.Sprintf("totalQuota consumed is %d, something is wrong", totalQuota))
	}
}
