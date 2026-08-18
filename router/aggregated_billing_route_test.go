package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestAggregatedAndBillingRoutesRegistered verifies that the aggregated API
// (9 user routes + plans) is registered under all three prefixes
// (/api, /api/v1, /api/v2 — see SetApiRouter) and that the billing report
// routes are mirrored under the same three prefixes. This guards against
// prefix drift and route typos.
func TestAggregatedAndBillingRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}

	wantByPrefix := []struct{ method, path string }{
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
	}
	for _, prefix := range []string{"/api", "/api/v1", "/api/v2"} {
		for _, r := range wantByPrefix {
			assert.Containsf(t, routes, r.method+" "+prefix+r.path,
				"聚合 API 路由缺失: %s %s", r.method, prefix+r.path)
		}
	}

	// billing 报表路由同样在 registerApiRoutes 内，随三前缀镜像
	billingByPrefix := []struct{ method, path string }{
		{http.MethodGet, "/data/billing"},
		{http.MethodGet, "/data/reconciliation"},
		{http.MethodGet, "/ops/data/billing"},
	}
	for _, prefix := range []string{"/api", "/api/v1", "/api/v2"} {
		for _, r := range billingByPrefix {
			assert.Containsf(t, routes, r.method+" "+prefix+r.path,
				"billing 路由缺失: %s %s", r.method, prefix+r.path)
		}
	}
}
