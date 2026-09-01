package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetVideoRouter(router *gin.Engine) {
	// OpenAI 兼容的 /v1/videos* 路径（POST /v1/videos、GET /v1/videos/:task_id、
	// GET /v1/videos/:task_id/content）由 SetTaskPluginProtocolRouter 注册的
	// openai_video host protocol 承载：其 create 处理链在未 pin 到 JS 插件时
	// 回退到 controller.RelayTask（RelayTaskPluginEndpoint 委托），retrieve /
	// content 与这里曾注册的 RelayTaskFetch / VideoProxy 是同一个 controller。
	// 因此这里不再重复注册，避免 gin 启动期 duplicate route panic。
	videoV1Router := router.Group("/v1")
	videoV1Router.Use(middleware.RouteTag("relay"))
	videoV1Router.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		videoV1Router.POST("/video/generations", controller.RelayTask)
		videoV1Router.GET("/video/generations/:task_id", controller.RelayTaskFetch)
		videoV1Router.POST("/videos/:video_id/remix", controller.RelayTask)
		// Ark 风格视频端点（Seedance），仅 doubao/astraflow 渠道开放，渠道类型
		// 在 RelayTaskSubmit / videoFetchByIDRespBodyBuilder 中校验。
		videoV1Router.POST("/videos/generations/tasks", controller.RelayTask)
		videoV1Router.GET("/videos/generations/tasks/:task_id", controller.RelayTaskFetch)
	}

	klingV1Router := router.Group("/kling/v1")
	klingV1Router.Use(middleware.RouteTag("relay"))
	klingV1Router.Use(middleware.KlingRequestConvert(), middleware.TokenAuth(), middleware.Distribute())
	{
		klingV1Router.POST("/videos/text2video", controller.RelayTask)
		klingV1Router.POST("/videos/image2video", controller.RelayTask)
		klingV1Router.GET("/videos/text2video/:task_id", controller.RelayTaskFetch)
		klingV1Router.GET("/videos/image2video/:task_id", controller.RelayTaskFetch)
	}

	// Jimeng official API routes - direct mapping to official API format
	jimengOfficialGroup := router.Group("jimeng")
	jimengOfficialGroup.Use(middleware.RouteTag("relay"))
	jimengOfficialGroup.Use(middleware.JimengRequestConvert(), middleware.TokenAuth(), middleware.Distribute())
	{
		// Maps to: /?Action=CVSync2AsyncSubmitTask&Version=2022-08-31 and /?Action=CVSync2AsyncGetResult&Version=2022-08-31
		jimengOfficialGroup.POST("/", controller.RelayTask)
	}
}
