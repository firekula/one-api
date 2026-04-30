package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/relay"
	"github.com/songquanpeng/one-api/relay/adaptor/anthropic"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/apitype"
	"github.com/songquanpeng/one-api/relay/billing"
	billingratio "github.com/songquanpeng/one-api/relay/billing/ratio"
	"github.com/songquanpeng/one-api/relay/meta"
	relaymodel "github.com/songquanpeng/one-api/relay/model"
)

// RelayAnthropicHelper 处理入站 Anthropic Messages API 请求
// 核心原则：请求体/响应体完全不转换，仅透传；usage 静默提取用于记账
func RelayAnthropicHelper(c *gin.Context) *relaymodel.ErrorWithStatusCode {
	ctx := c.Request.Context()
	meta := meta.GetByContext(c)

	// 1. 读取原始请求体
	originalBody, err := common.GetRequestBody(c)
	if err != nil {
		return openai.ErrorWrapper(err, "read_request_body_failed", http.StatusBadRequest)
	}

	// 2. 解析 Anthropic 请求获取 model 和 stream
	var anthropicReq anthropic.Request
	if err := json.Unmarshal(originalBody, &anthropicReq); err != nil {
		return openai.ErrorWrapper(err, "invalid_anthropic_request", http.StatusBadRequest)
	}
	meta.IsStream = anthropicReq.Stream
	meta.OriginModelName = anthropicReq.Model

	// 3. 模型映射
	mappedModel, _ := getMappedModelName(anthropicReq.Model, meta.ModelMapping)
	meta.ActualModelName = mappedModel

	// 4. 强制使用 Anthropic adaptor，不依赖 ToAPIType 映射
	meta.APIType = apitype.Anthropic

	// 5. 费率计算
	modelRatio := billingratio.GetModelRatio(mappedModel, meta.ChannelType)
	groupRatio := billingratio.GetGroupRatio(meta.Group)
	ratio := modelRatio * groupRatio

	// 6. 估算 prompt tokens 用于预扣额度
	promptTokens := estimateAnthropicPromptTokens(&anthropicReq)
	meta.PromptTokens = promptTokens

	// 7. 预扣额度
	preConsumedQuota, bizErr := preConsumeQuotaAnthropic(ctx, promptTokens, anthropicReq.MaxTokens, ratio, meta)
	if bizErr != nil {
		return bizErr
	}

	// 8. 获取 Adaptor
	adaptor := relay.GetAdaptor(meta.APIType)
	if adaptor == nil {
		billing.ReturnPreConsumedQuota(ctx, preConsumedQuota, meta.TokenId)
		return openai.ErrorWrapper(fmt.Errorf("invalid api type: %d", meta.APIType), "invalid_api_type", http.StatusBadRequest)
	}
	adaptor.Init(meta)

	// 9. 透传发送：使用原始 Anthropic 请求体，不调 ConvertRequest
	resp, err := adaptor.DoRequest(c, meta, bytes.NewReader(originalBody))
	if err != nil {
		logger.Errorf(ctx, "DoRequest failed: %s", err.Error())
		billing.ReturnPreConsumedQuota(ctx, preConsumedQuota, meta.TokenId)
		return openai.ErrorWrapper(err, "do_request_failed", http.StatusInternalServerError)
	}
	if isErrorHappened(meta, resp) {
		billing.ReturnPreConsumedQuota(ctx, preConsumedQuota, meta.TokenId)
		return RelayErrorHandler(resp)
	}

	// 10. 原生响应处理：透传并提取 usage（由 native.go 实现）
	usage, respErr := anthropic.NativeDoResponse(c, resp, meta)
	if respErr != nil {
		billing.ReturnPreConsumedQuota(ctx, preConsumedQuota, meta.TokenId)
		return respErr
	}

	// 11. 异步结算额度
	go postConsumeQuotaAnthropic(ctx, usage, meta, &anthropicReq, ratio, preConsumedQuota, modelRatio, groupRatio)
	return nil
}

// preConsumeQuotaAnthropic Anthropic 入站的预扣额度
func preConsumeQuotaAnthropic(ctx context.Context, promptTokens int, maxTokens int, ratio float64, meta *meta.Meta) (int64, *relaymodel.ErrorWithStatusCode) {
	preConsumedTokens := config.PreConsumedQuota + int64(promptTokens) + int64(maxTokens)
	preConsumedQuota := int64(float64(preConsumedTokens) * ratio)

	userQuota, err := model.CacheGetUserQuota(ctx, meta.UserId)
	if err != nil {
		return preConsumedQuota, openai.ErrorWrapper(err, "get_user_quota_failed", http.StatusInternalServerError)
	}
	if userQuota-preConsumedQuota < 0 {
		return preConsumedQuota, openai.ErrorWrapper(fmt.Errorf("user quota is not enough"), "insufficient_user_quota", http.StatusForbidden)
	}
	_ = model.CacheDecreaseUserQuota(meta.UserId, preConsumedQuota)
	if userQuota > 100*preConsumedQuota {
		preConsumedQuota = 0
	}
	if preConsumedQuota > 0 {
		_ = model.PreConsumeTokenQuota(meta.TokenId, preConsumedQuota)
	}
	return preConsumedQuota, nil
}

// postConsumeQuotaAnthropic Anthropic 入站的异步额度结算
func postConsumeQuotaAnthropic(ctx context.Context, usage *relaymodel.Usage, meta *meta.Meta, req *anthropic.Request, ratio float64, preConsumedQuota int64, modelRatio float64, groupRatio float64) {
	if usage == nil {
		return
	}
	completionRatio := billingratio.GetCompletionRatio(req.Model, meta.ChannelType)
	quota := int64(math.Ceil((float64(usage.PromptTokens) + float64(usage.CompletionTokens)*completionRatio) * ratio))
	if ratio != 0 && quota <= 0 {
		quota = 1
	}
	if usage.TotalTokens == 0 {
		quota = 0
	}
	quotaDelta := quota - preConsumedQuota
	_ = model.PostConsumeTokenQuota(meta.TokenId, quotaDelta)
	_ = model.CacheUpdateUserQuota(ctx, meta.UserId)
	model.UpdateUserUsedQuotaAndRequestCount(meta.UserId, quota)
	model.UpdateChannelUsedQuota(meta.ChannelId, quota)
}

// estimateAnthropicPromptTokens 粗略估算 Anthropic 请求的 prompt tokens
func estimateAnthropicPromptTokens(req *anthropic.Request) int {
	totalChars := 0
	if len(req.System) > 0 {
		var sysStr string
		if json.Unmarshal(req.System, &sysStr) == nil {
			totalChars += len(sysStr)
		} else {
			var sysBlocks []anthropic.Content
			if json.Unmarshal(req.System, &sysBlocks) == nil {
				for _, block := range sysBlocks {
					totalChars += len(block.Text) + len(block.Content)
				}
			}
		}
	}
	for _, msg := range req.Messages {
		for _, content := range msg.Content {
			totalChars += len(content.Text) + len(content.Content)
		}
	}
	if totalChars == 0 {
		return 0
	}
	return totalChars/4 + 1 // 粗估: ~4 字符 ≈ 1 token
}
