package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/model"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// 修复回归测试：sqlite 等瞬态基础设施错误（database is locked）导致的
// 令牌验证失败必须返回 5xx（可重试），绝不能伪装成 401 认证失败。
// 三个认证中间件共享同一错误处理模式，逐一覆盖防止复制粘贴代码漂移。
func TestAuthTransientErrorReturnsServiceUnavailable(t *testing.T) {
	tests := []struct {
		name       string
		middleware gin.HandlerFunc
		authHeader string
	}{
		{"TokenAuth", TokenAuth(), "Authorization"},
		{"FlexibleTokenAuth", FlexibleTokenAuth(), "x-api-key"},
		{"AnthropicTokenAuth", AnthropicTokenAuth(), "x-api-key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := validateUserToken
			validateUserToken = func(key string) (*model.Token, error) {
				return nil, fmt.Errorf("%w: 令牌验证失败", model.ErrTransientTokenError)
			}
			defer func() { validateUserToken = old }()

			r := gin.New()
			r.Use(tt.middleware)
			r.GET("/v1/chat/completions", func(c *gin.Context) { c.Status(http.StatusOK) })

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
			req.Header.Set(tt.authHeader, "sk-test-token")
			r.ServeHTTP(w, req)

			if w.Code != http.StatusServiceUnavailable {
				t.Fatalf("瞬态令牌验证错误应返回 %d，实际 %d", http.StatusServiceUnavailable, w.Code)
			}
		})
	}
}

func TestAuthInvalidTokenReturnsUnauthorized(t *testing.T) {
	tests := []struct {
		name       string
		middleware gin.HandlerFunc
		authHeader string
	}{
		{"TokenAuth", TokenAuth(), "Authorization"},
		{"FlexibleTokenAuth", FlexibleTokenAuth(), "x-api-key"},
		{"AnthropicTokenAuth", AnthropicTokenAuth(), "x-api-key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := validateUserToken
			validateUserToken = func(key string) (*model.Token, error) {
				return nil, errors.New("无效的令牌")
			}
			defer func() { validateUserToken = old }()

			r := gin.New()
			r.Use(tt.middleware)
			r.GET("/v1/chat/completions", func(c *gin.Context) { c.Status(http.StatusOK) })

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
			req.Header.Set(tt.authHeader, "sk-test-token")
			r.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("无效令牌应返回 %d，实际 %d", http.StatusUnauthorized, w.Code)
			}
		})
	}
}
