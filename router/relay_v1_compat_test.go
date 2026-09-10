package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBareV1RoutePatternsDeriveFromRegisteredV1Routes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetRelayRouter(engine)
	SetVideoRouter(engine)
	SetTaskPluginProtocolRouter(engine)

	patterns := bareV1RoutePatterns(engine.Routes())

	for _, want := range []string{
		"/chat/completions",
		"/completions",
		"/embeddings",
		"/moderations",
		"/rerank",
		"/images/generations",
		"/audio/speech",
		"/messages",
		"/responses",
		"/models",
		"/models/:model",
		"/videos",
	} {
		assert.Contains(t, patterns, want)
	}
	// 去前缀必须按 "/v1/" 匹配：写成 "/v1" 会把 "/v1beta/models" 变成没有前导斜杠的
	// "beta/models"，这类模式永远匹配不到任何请求路径。
	for _, pattern := range patterns {
		assert.True(t, strings.HasPrefix(pattern, "/"), "pattern %q must keep its leading slash", pattern)
	}
}

func TestRelayV1CompatRedirectAddsMissingV1Prefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	routeEngine := gin.New()
	SetRelayRouter(routeEngine)
	SetTaskPluginProtocolRouter(routeEngine)
	patterns := bareV1RoutePatterns(routeEngine.Routes())

	for _, testCase := range []struct {
		name         string
		method       string
		target       string
		accept       string
		upgrade      string
		wantLocation string
	}{
		{
			name:         "chat completions",
			method:       http.MethodPost,
			target:       "/chat/completions",
			accept:       "application/json",
			wantLocation: "/v1/chat/completions",
		},
		{
			name:         "query string is preserved",
			method:       http.MethodPost,
			target:       "/responses?stream=true&model=deepseek-chat",
			accept:       "application/json",
			wantLocation: "/v1/responses?stream=true&model=deepseek-chat",
		},
		{
			name:         "models list without accept header",
			method:       http.MethodGet,
			target:       "/models",
			wantLocation: "/v1/models",
		},
		{
			name:         "model retrieve",
			method:       http.MethodGet,
			target:       "/models/gemini-pro",
			accept:       "application/json",
			wantLocation: "/v1/models/gemini-pro",
		},
		{
			name:         "browser navigation keeps dashboard page",
			method:       http.MethodGet,
			target:       "/models",
			accept:       "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			wantLocation: "",
		},
		{
			name:         "browser navigation keeps dashboard model section",
			method:       http.MethodGet,
			target:       "/models/overview",
			accept:       "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			wantLocation: "",
		},
		{
			name:         "websocket upgrade is not redirected",
			method:       http.MethodGet,
			target:       "/realtime",
			accept:       "application/json",
			upgrade:      "websocket",
			wantLocation: "",
		},
		{
			name:         "unmatched path keeps existing fallback",
			method:       http.MethodPost,
			target:       "/not-a-relay-path",
			accept:       "application/json",
			wantLocation: "",
		},
		{
			name:         "unknown v1 path keeps existing fallback",
			method:       http.MethodPost,
			target:       "/v1/chat/completions/typo",
			accept:       "application/json",
			wantLocation: "",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			engine := gin.New()
			engine.NoRoute(
				relayV1CompatRedirect(patterns),
				func(c *gin.Context) { c.String(http.StatusOK, "fallback") },
			)
			request := httptest.NewRequest(testCase.method, testCase.target, nil)
			if testCase.accept != "" {
				request.Header.Set("Accept", testCase.accept)
			}
			if testCase.upgrade != "" {
				request.Header.Set("Upgrade", testCase.upgrade)
			}
			recorder := httptest.NewRecorder()

			engine.ServeHTTP(recorder, request)

			if testCase.wantLocation == "" {
				require.Equal(t, http.StatusOK, recorder.Code)
				assert.Equal(t, "fallback", recorder.Body.String())
				return
			}
			require.Equal(t, http.StatusTemporaryRedirect, recorder.Code)
			assert.Equal(t, testCase.wantLocation, recorder.Header().Get("Location"))
			// 重定向后必须终止链路，SPA fallback 不能继续写响应体。
			assert.NotContains(t, recorder.Body.String(), "fallback")
		})
	}
}
