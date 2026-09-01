package router

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPluginRoutesRegisterWithoutConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	require.NotPanics(t, func() {
		SetApiRouter(engine)
	})

	var pluginPaths []string
	for _, r := range engine.Routes() {
		if len(r.Path) >= len("/api/plugin") && r.Path[:len("/api/plugin")] == "/api/plugin" {
			pluginPaths = append(pluginPaths, r.Method+" "+r.Path)
		}
	}
	// 同步上游 JS 任务插件系统后，SetApiRouter 在原有 plugin 管理路由之外新增了
	// /api/plugin/task* 的任务插件管理路由（ListTaskPlugins / UploadTaskPlugin /
	// ActivateTaskPlugin 等，见 api-router.go 的 taskPluginRoute 分组）。
	require.ElementsMatch(t, []string{
		"GET /api/plugin/enabled",
		"GET /api/plugin/",
		"POST /api/plugin/",
		"PUT /api/plugin/",
		"DELETE /api/plugin/:id",
		"POST /api/plugin/:id/refresh",
		"POST /api/plugin/test",
		"GET /api/plugin/task",
		"POST /api/plugin/task",
		"PUT /api/plugin/task",
		"GET /api/plugin/task/runtime/status",
		"GET /api/plugin/task/marketplace/sources",
		"PUT /api/plugin/task/marketplace/sources",
		"GET /api/plugin/task/:key",
		"GET /api/plugin/task/:key/versions",
		"POST /api/plugin/task/:key/activate",
		"POST /api/plugin/task/:key/status",
		"POST /api/plugin/task/:key/dryrun",
		"DELETE /api/plugin/task/:key/versions/:version",
	}, pluginPaths)
}
