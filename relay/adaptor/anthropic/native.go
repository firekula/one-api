package anthropic

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/common/render"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/meta"
	"github.com/songquanpeng/one-api/relay/model"
)

// NativeDoResponse 原生 Anthropic 响应处理分发：根据 stream 模式选择对应处理函数
func NativeDoResponse(c *gin.Context, resp *http.Response, meta *meta.Meta) (*model.Usage, *model.ErrorWithStatusCode) {
	if meta.IsStream {
		err, usage := NativeStreamHandler(c, resp)
		return usage, err
	}
	err, usage := NativeHandler(c, resp)
	return usage, err
}

// NativeHandler 非流模式：解析 usage → 原样透传 Anthropic JSON 响应
func NativeHandler(c *gin.Context, resp *http.Response) (*model.ErrorWithStatusCode, *model.Usage) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return openai.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError), nil
	}
	_ = resp.Body.Close()

	var claudeResponse Response
	if err = json.Unmarshal(responseBody, &claudeResponse); err != nil {
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Writer.WriteHeader(resp.StatusCode)
		fallback := map[string]any{
			"type": "error",
			"error": map[string]string{
				"type":    "upstream_error",
				"message": string(responseBody),
			},
		}
		_ = json.NewEncoder(c.Writer).Encode(fallback)
		return &model.ErrorWithStatusCode{
			Error:      model.Error{Message: "upstream returned non-Anthropic response", Code: "upstream_error"},
			StatusCode: resp.StatusCode,
		}, nil
	}
	if claudeResponse.Error.Type != "" {
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Writer.WriteHeader(resp.StatusCode)
		_, _ = c.Writer.Write(responseBody)
		return &model.ErrorWithStatusCode{
			Error:      model.Error{Message: claudeResponse.Error.Message, Code: claudeResponse.Error.Type},
			StatusCode: resp.StatusCode,
		}, nil
	}
	usage := model.Usage{
		PromptTokens:     claudeResponse.Usage.InputTokens,
		CompletionTokens: claudeResponse.Usage.OutputTokens,
		TotalTokens:      claudeResponse.Usage.InputTokens + claudeResponse.Usage.OutputTokens,
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(responseBody)
	return nil, &usage
}

// NativeStreamHandler 流模式：SSE 事件原样转发，从 message_delta 静默提取 usage
func NativeStreamHandler(c *gin.Context, resp *http.Response) (*model.ErrorWithStatusCode, *model.Usage) {
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := strings.Index(string(data), "\n"); i >= 0 {
			return i + 1, data[0:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	})

	common.SetEventStreamHeaders(c)
	var usage model.Usage

	for scanner.Scan() {
		data := scanner.Text()
		if len(data) < 6 || !strings.HasPrefix(data, "data:") {
			continue
		}
		render.StringData(c, strings.TrimPrefix(data, "data: "))

		payload := strings.TrimSpace(strings.TrimPrefix(data, "data:"))
		var streamResp StreamResponse
		if json.Unmarshal([]byte(payload), &streamResp) == nil {
			if streamResp.Type == "message_delta" && streamResp.Usage != nil {
				usage.PromptTokens += streamResp.Usage.InputTokens
				usage.CompletionTokens += streamResp.Usage.OutputTokens
				usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			}
		}
	}
	if err := scanner.Err(); err != nil {
		logger.SysError("error reading stream: " + err.Error())
	}
	render.Done(c)
	_ = resp.Body.Close()
	return nil, &usage
}
