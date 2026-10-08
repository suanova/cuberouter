package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskArtifactAccessIsRedactedAndVerifiedBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousSecret := common.CryptoSecret
	common.CryptoSecret = "task-artifact-middleware-secret"
	t.Cleanup(func() { common.CryptoSecret = previousSecret })

	access, err := service.IssueTaskArtifactAccess("task-1", "video-main")
	require.NoError(t, err)

	router := gin.New()
	router.Use(redactTaskArtifactAccessQuery())
	router.GET(
		"/v1/tasks/:key/artifacts/:artifact_key/content",
		TokenOrTaskArtifactAccessAuth("key", "artifact_key"),
		func(c *gin.Context) {
			assert.True(t, IsTaskArtifactAccess(c))
			assert.NotContains(t, c.Request.URL.RawQuery, service.TaskArtifactAccessQueryParameter)
			assert.Equal(t, "kept", c.Query("keep"))
			c.Status(http.StatusNoContent)
		},
	)
	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/tasks/task-1/artifacts/video-main/content?access="+urlQueryEscape(access)+"&keep=kept",
		nil,
	)
	request.RemoteAddr = "192.0.2.1:1234"
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestTaskArtifactAccessRejectsTamperedAndEmptyCapabilitiesAsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousSecret := common.CryptoSecret
	common.CryptoSecret = "task-artifact-middleware-reject-secret"
	t.Cleanup(func() { common.CryptoSecret = previousSecret })

	router := gin.New()
	router.Use(redactTaskArtifactAccessQuery())
	router.GET(
		"/v1/tasks/:key/artifacts/:artifact_key/content",
		TokenOrTaskArtifactAccessAuth("key", "artifact_key"),
		func(c *gin.Context) { c.Status(http.StatusNoContent) },
	)

	for _, query := range []string{
		"?access=",
		"?access=invalid",
		"?access=first&access=second",
		"?access=" + strings.Repeat("x", 1024),
		"?access=%20" + strings.Repeat("A", 43) + "%20",
	} {
		request := httptest.NewRequest(
			http.MethodGet,
			"/v1/tasks/task-1/artifacts/video-main/content"+query,
			nil,
		)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusNotFound, recorder.Code)
	}
}

func TestTaskArtifactAccessLimiterDefaults(t *testing.T) {
	limits := system_setting.TaskArtifactAccessLimits{
		InvalidRatePerMinute: system_setting.DefaultTaskArtifactInvalidRateLimitPerMinute,
		GlobalConcurrency:    system_setting.DefaultTaskArtifactGlobalConcurrency,
		IPConcurrency:        system_setting.DefaultTaskArtifactIPConcurrency,
		ObjectConcurrency:    system_setting.DefaultTaskArtifactObjectConcurrency,
	}
	limiter := newTaskArtifactAccessLimiter(limits)
	now := time.Unix(1000, 0)
	releases := make([]func(), 0, limits.ObjectConcurrency)
	for i := 0; i < limits.ObjectConcurrency; i++ {
		release, ok := limiter.acquire("192.0.2.1", "task-1", "video")
		require.True(t, ok)
		releases = append(releases, release)
	}
	_, ok := limiter.acquire("192.0.2.2", "task-1", "video")
	assert.False(t, ok, "task+key concurrency is shared across IPs")
	for _, release := range releases {
		release()
	}

	rateLimiter := newTaskArtifactAccessLimiter(limits)
	for i := 0; i < limits.InvalidRatePerMinute; i++ {
		assert.True(t, rateLimiter.invalidAttempt(now, "192.0.2.10"))
	}
	assert.False(t, rateLimiter.invalidAttempt(now, "192.0.2.10"))
	assert.True(t, rateLimiter.invalidAttempt(now.Add(time.Minute), "192.0.2.10"))
}

func TestRedactTaskArtifactAccessAlsoCoversLegacyVideoRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(redactTaskArtifactAccessQuery())
	router.GET("/v1/videos/:task_id/content", func(c *gin.Context) {
		assert.NotContains(t, c.Request.URL.RawQuery, "access")
		assert.NotContains(t, c.Request.RequestURI, "secret-capability")
		assert.Equal(t, "ok", c.Query("keep"))
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/videos/task-1/content?access=secret-capability&keep=ok",
		nil,
	)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestSetUpLoggerNeverWritesTaskArtifactAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousWriter := gin.DefaultWriter
	var output bytes.Buffer
	gin.DefaultWriter = &output
	t.Cleanup(func() { gin.DefaultWriter = previousWriter })

	router := gin.New()
	SetUpLogger(router)
	router.GET("/v1/tasks/:key/artifacts/:artifact_key/content", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/tasks/task-1/artifacts/video/content?access=never-log-this&keep=ok",
		nil,
	)
	router.ServeHTTP(httptest.NewRecorder(), request)

	assert.False(t, strings.Contains(output.String(), "never-log-this"))
}

func urlQueryEscape(value string) string {
	replacer := strings.NewReplacer("+", "%2B", "=", "%3D")
	return replacer.Replace(value)
}

// newVideoContentAccessProbe 复刻生产路由：全局 redact 中间件 + 内容端点鉴权。
func newVideoContentAccessProbe(handler gin.HandlerFunc) *gin.Engine {
	router := gin.New()
	router.Use(redactTaskArtifactAccessQuery())
	router.GET("/v1/videos/:task_id/content", TokenOrVideoContentAccessAuth("task_id"), handler)
	return router
}

// videoContentProbeRequest 用独立的 ClientIP，避免与 artifact 用例共用
// package 级的失败限流计数。
func videoContentProbeRequest(query string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/v1/videos/task-video/content"+query, nil)
	request.RemoteAddr = "198.51.100.9:1234"
	return request
}

func TestVideoContentAccessAuthAcceptsCredentiallessCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousSecret := common.CryptoSecret
	common.CryptoSecret = "video-content-access-test-secret"
	t.Cleanup(func() { common.CryptoSecret = previousSecret })

	access, err := service.IssueTaskArtifactAccess("task-video", service.TaskVideoArtifactKey)
	require.NoError(t, err)

	router := newVideoContentAccessProbe(func(c *gin.Context) {
		assert.True(t, IsTaskArtifactAccess(c))
		assert.NotContains(t, c.Request.URL.RawQuery, service.TaskArtifactAccessQueryParameter)
		assert.NotContains(t, c.Request.RequestURI, access)
		assert.Equal(t, "kept", c.Query("keep"))
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, videoContentProbeRequest("?access="+urlQueryEscape(access)+"&keep=kept"))

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestVideoContentAccessAuthRejectsTamperedCapabilityAsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousSecret := common.CryptoSecret
	common.CryptoSecret = "video-content-access-test-secret"
	t.Cleanup(func() { common.CryptoSecret = previousSecret })

	// 绑定到别的 artifact key 的签名在视频路由上无效。
	otherKeyAccess, err := service.IssueTaskArtifactAccess("task-video", "image")
	require.NoError(t, err)

	handlerRan := false
	router := newVideoContentAccessProbe(func(c *gin.Context) {
		handlerRan = true
		c.Status(http.StatusNoContent)
	})

	for _, query := range []string{
		"?access=",
		"?access=invalid",
		"?access=" + urlQueryEscape(otherKeyAccess),
		"?access=first&access=second",
		"?access=" + strings.Repeat("x", 1024),
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, videoContentProbeRequest(query))
		assert.Equal(t, http.StatusNotFound, recorder.Code, query)
		assert.Contains(t, recorder.Body.String(), "artifact_not_found", query)
	}
	assert.False(t, handlerRan)
}

func TestVideoContentAccessAuthFallsBackToRelayTokenWithoutAccess(t *testing.T) {
	setupTokenAuthMiddlewareTestDB(t)
	gin.SetMode(gin.TestMode)

	user := createTokenAuthMiddlewareUser(t, "video-content-session-user")
	token := model.Token{
		UserId:         user.Id,
		Key:            "videocontentaccessfallback000000000000000001",
		Name:           "video-content-key",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}
	require.NoError(t, model.DB.Create(&token).Error)

	handlerRan := false
	router := newVideoContentAccessProbe(func(c *gin.Context) {
		handlerRan = true
		// 无 capability 的请求走 TokenOrUserAuth，不应被标记为 capability 身份。
		assert.False(t, IsTaskArtifactAccess(c))
		assert.Equal(t, user.Id, c.GetInt("id"))
		c.Status(http.StatusNoContent)
	})

	// 有效的 relay 令牌：行为与改造前的 TokenOrUserAuth 一致。
	authenticated := videoContentProbeRequest("")
	authenticated.Header.Set("Authorization", "Bearer "+token.Key)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authenticated)
	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.True(t, handlerRan)

	// 无效令牌：仍是鉴权失败，不会被 capability 分支改写成 404 artifact_not_found。
	rejected := videoContentProbeRequest("")
	rejected.Header.Set("Authorization", "Bearer not-a-real-key")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, rejected)
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "artifact_not_found")
}
