package router

import (
	"fmt"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

func SetTaskPluginProtocolRouter(router *gin.Engine) {
	for _, protocol := range pluginruntime.HostProtocols() {
		for _, operation := range protocol.Operations {
			for _, method := range operation.Methods {
				handlers, err := taskPluginProtocolHandlers(protocol.Name, operation.Name)
				if err != nil {
					panic(err)
				}
				router.Handle(method, operation.Path, handlers...)
			}
		}
	}
}

func taskPluginProtocolHandlers(protocol, operation string) ([]gin.HandlerFunc, error) {
	switch protocol + "." + operation {
	case "openai_responses.create":
		return []gin.HandlerFunc{
			middleware.RouteTag("relay"), middleware.SystemPerformanceCheck(), middleware.TokenAuth(),
			middleware.ModelRequestRateLimit(), middleware.PinTaskPluginEndpoint(), middleware.PrepareTaskPluginEndpoint(), middleware.Distribute(),
			func(c *gin.Context) {
				controller.RelayTaskPluginEndpoint(c, func(c *gin.Context) { controller.Relay(c, types.RelayFormatOpenAIResponses) })
			},
		}, nil
	case "openai_video.create":
		return []gin.HandlerFunc{
			middleware.RouteTag("relay"), middleware.TokenAuth(), middleware.SystemPerformanceCheck(),
			middleware.PinTaskPluginEndpoint(), middleware.TaskPluginEndpointOnly(middleware.ModelRequestRateLimit()), middleware.PrepareTaskPluginEndpoint(), middleware.Distribute(),
			func(c *gin.Context) { controller.RelayTaskPluginEndpoint(c, controller.RelayTask) },
		}, nil
	case "openai_responses.retrieve":
		return []gin.HandlerFunc{middleware.RouteTag("relay"), middleware.TokenAuth(), controller.RetrieveTaskPluginResponse}, nil
	case "openai_video.retrieve":
		return []gin.HandlerFunc{middleware.RouteTag("relay"), middleware.TokenAuth(), middleware.Distribute(), controller.RelayTaskFetch}, nil
	case "openai_video.content":
		// fork 原 video-router 对 /v1/videos/:task_id/content 使用 TokenOrUserAuth，
		// 允许 dashboard 会话身份（session / PAT）与 relay API 令牌访问视频内容，
		// 这里沿用该行为以保留本地功能。
		return []gin.HandlerFunc{middleware.RouteTag("relay"), middleware.TokenOrUserAuth(), controller.VideoProxy}, nil
	default:
		return nil, fmt.Errorf("host protocol registry operation %s.%s has no handler", protocol, operation)
	}
}
