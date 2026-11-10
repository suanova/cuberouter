package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/setting/security_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDocumentedAPIRoutesOnlyOnV2 verifies that every endpoint described by the
// Swagger spec (docs/swagger.json) is registered only under /api/v2, which
// carries the IP whitelist. /api and /api/v1 must not expose the same paths,
// otherwise callers could bypass the whitelist. This guards against prefix
// drift and route typos.
func TestDocumentedAPIRoutesOnlyOnV2(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}

	documented := []struct{ method, path string }{
		{http.MethodPost, "/users"},
		{http.MethodPost, "/users/"},
		{http.MethodPost, "/users/:user_id/suspend"},
		{http.MethodPost, "/users/:user_id/reactivate"},
		{http.MethodPost, "/users/:user_id/reset-password"},
		{http.MethodPost, "/users/:user_id/adjust-quota"},
		{http.MethodGet, "/users/:user_id/status"},
		{http.MethodPost, "/users/:user_id/bind-subscription"},
		{http.MethodPost, "/users/:user_id/delete"},
		{http.MethodPost, "/plans"},
		{http.MethodPost, "/plans/"},
		{http.MethodGet, "/data/billing"},
		{http.MethodGet, "/data/reconciliation"},
		{http.MethodGet, "/ops/data/billing"},
		{http.MethodPost, "/channel/:id/import_models_csv"},
	}
	for _, r := range documented {
		assert.Containsf(t, routes, r.method+" /api/v2"+r.path,
			"文档接口缺失: %s /api/v2%s", r.method, r.path)
		for _, prefix := range []string{"/api", "/api/v1"} {
			assert.NotContainsf(t, routes, r.method+" "+prefix+r.path,
				"文档接口不应出现在 %s: %s %s", prefix, r.method, prefix+r.path)
		}
	}
}

// TestV2PrefixUsesIPWhitelist verifies the whitelist is wired onto /api/v2 only:
// a non-listed client is rejected there, while /api keeps answering (the
// dashboard has no whitelist).
func TestV2PrefixUsesIPWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)

	setting := security_setting.GetSecuritySetting()
	original := setting.SwaggerAPIIPWhitelist
	t.Cleanup(func() { setting.SwaggerAPIIPWhitelist = original })
	setting.SwaggerAPIIPWhitelist = []string{"203.0.113.7"}

	engine := gin.New()
	require.NoError(t, engine.SetTrustedProxies(nil))
	SetApiRouter(engine)

	probe := func(path, remoteAddr string) int {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.RemoteAddr = remoteAddr
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		return recorder.Code
	}

	assert.Equal(t, http.StatusForbidden, probe("/api/v2/status", "198.51.100.9:1234"))
	assert.NotEqual(t, http.StatusForbidden, probe("/api/v2/status", "203.0.113.7:1234"))
	assert.NotEqual(t, http.StatusForbidden, probe("/api/status", "198.51.100.9:1234"))
}
