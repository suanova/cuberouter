package router

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"

	_ "github.com/QuantumNous/new-api/docs"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetRouter(router *gin.Engine, assets WebAssets) {
	SetApiRouter(router)
	SetDashboardRouter(router)
	SetRelayRouter(router)
	SetTaskPluginProtocolRouter(router)
	SetVideoRouter(router)
	SetTaskRouter(router)
	// The docs routes must exist before SetPluginRouter snapshots the static
	// routes: a plugin route under /docs/{user,admin}/... would otherwise be
	// admitted by the conflict check and then silently shadowed by these
	// routes instead of being reported as intersecting a static route.
	SetDocsRouter(router)
	pluginDispatcher := SetPluginRouter(router)
	// Swagger 默认挂载,不再由 DEBUG 开关控制。API 规格本身不含敏感数据,
	// 接口访问仍由各自的鉴权中间件(AdminAuth/OpsAuth 等)保护。
	// 管理员可在系统设置中配置 IP 白名单,同时限制文档与 /api/v2 对外接口。
	router.GET("/swagger/*any", middleware.SwaggerAPIIPWhitelist(), ginSwagger.WrapHandler(swaggerFiles.Handler))
	frontendBaseUrl := os.Getenv("FRONTEND_BASE_URL")
	if common.IsMasterNode && frontendBaseUrl != "" {
		frontendBaseUrl = ""
		common.SysLog("FRONTEND_BASE_URL is ignored on master node")
	}
	if frontendBaseUrl == "" {
		SetWebRouter(router, assets, pluginDispatcher)
	} else {
		frontendBaseUrl = strings.TrimSuffix(frontendBaseUrl, "/")
		router.NoRoute(
			pluginDispatcher,
			relayV1CompatRedirect(bareV1RoutePatterns(router.Routes())),
			func(c *gin.Context) {
				c.Set(middleware.RouteTagKey, "web")
				c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("%s%s", frontendBaseUrl, c.Request.RequestURI))
			},
		)
	}
}
