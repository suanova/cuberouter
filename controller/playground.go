package controller

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func Playground(c *gin.Context) {
	var newAPIError *types.NewAPIError

	defer func() {
		if newAPIError != nil {
			c.JSON(newAPIError.StatusCode, gin.H{
				"error": newAPIError.ToOpenAIError(),
			})
		}
	}()

	if _, newAPIError = preparePlaygroundContext(c, types.RelayFormatOpenAI); newAPIError != nil {
		return
	}

	// Plugin mentions (@slug) trigger the server-side tool-call loop.
	var pgReq dto.GeneralOpenAIRequest
	if err := common.UnmarshalBodyReusable(c, &pgReq); err == nil {
		if lastUser := lastUserMessageText(&pgReq); lastUser != "" {
			mentions := ExtractPluginMentions(lastUser)
			if plugins := ResolveMentionedPlugins(mentions, service.GetEnabledPlugins()); len(plugins) > 0 {
				playgroundWithPlugins(c, &pgReq, plugins)
				return
			}
		}
	}

	Relay(c, types.RelayFormatOpenAI)
}

// PlaygroundImage 是 /pg/images/generations 的会话鉴权图片生成入口
// （多媒体 studio 页面使用）。与 Playground 相同：登录会话 + 临时 token
// 上下文，走标准 OpenAI 图片 relay（同步阻塞，直到上游出图）。
func PlaygroundImage(c *gin.Context) {
	var newAPIError *types.NewAPIError

	defer func() {
		if newAPIError != nil {
			c.JSON(newAPIError.StatusCode, gin.H{
				"error": newAPIError.ToOpenAIError(),
			})
		}
	}()

	if _, newAPIError = preparePlaygroundContext(c, types.RelayFormatOpenAIImage); newAPIError != nil {
		return
	}

	Relay(c, types.RelayFormatOpenAIImage)
}

// preparePlaygroundContext 是 /pg 会话鉴权入口共享的前置处理：拒绝 access token，
// 校验请求体并生成 RelayInfo，写入用户上下文（保证 acceptUnsetRatio 可用），
// 再挂上临时 token。失败时返回的 newAPIError 由调用方按 OpenAI 风格输出。
func preparePlaygroundContext(c *gin.Context, relayFormat types.RelayFormat) (*relaycommon.RelayInfo, *types.NewAPIError) {
	useAccessToken := c.GetBool("use_access_token")
	if useAccessToken {
		return nil, types.NewError(errors.New("暂不支持使用 access token"), types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
	}

	// 组织作用域（兜底中间件已切换，或视频轮询从任务行还原）在上游链路已写全：
	// 个人临时 token 会把作用域覆盖回个人，这里直接返回。
	// 不重复 userCache.WriteContext：它会写 user_group，把中间件覆写的组织分组改回
	// 个人分组（model/user_cache.go WriteContext）；用户上下文 UserAuth 已写入。
	if common.GetContextKeyString(c, constant.ContextKeyBillingAccountType) == model.AccountContextTypeOrganization {
		return nil, nil
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, nil, nil)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	userId := c.GetInt("id")
	userCache, err := model.GetUserCache(userId)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	userCache.WriteContext(c)

	tempToken := &model.Token{
		UserId: userId,
		Name:   fmt.Sprintf("playground-%s", relayInfo.UsingGroup),
		Group:  relayInfo.UsingGroup,
	}
	_ = middleware.SetupContextForToken(c, tempToken)
	return relayInfo, nil
}

// PlaygroundVideo 是 /pg/video/generations 的会话鉴权视频生成提交入口
// （多媒体 studio 视频 tab 使用）。与 PlaygroundImage 相同：登录会话 + 临时 token
// 上下文；请求体是统一任务体，走标准任务 relay（异步：提交即返回 task_id，
// 前端轮询 PlaygroundVideoFetch 直到 SUCCESS/FAILURE）。
func PlaygroundVideo(c *gin.Context) {
	var newAPIError *types.NewAPIError

	defer func() {
		if newAPIError != nil {
			c.JSON(newAPIError.StatusCode, gin.H{
				"error": newAPIError.ToOpenAIError(),
			})
		}
	}()

	if _, newAPIError = preparePlaygroundContext(c, types.RelayFormatTask); newAPIError != nil {
		return
	}

	RelayTask(c)
}

// PlaygroundVideoFetch 是 /pg/video/generations/:task_id 的会话鉴权视频任务查询入口。
// 按 task_id 在本用户作用域内查任务并回传统一 TaskResponse（成功时含 result_url）。
func PlaygroundVideoFetch(c *gin.Context) {
	var newAPIError *types.NewAPIError

	defer func() {
		if newAPIError != nil {
			c.JSON(newAPIError.StatusCode, gin.H{
				"error": newAPIError.ToOpenAIError(),
			})
		}
	}()

	preparePlaygroundVideoFetchScope(c)
	if _, newAPIError = preparePlaygroundContext(c, types.RelayFormatTask); newAPIError != nil {
		return
	}

	RelayTaskFetch(c)
}

// preparePlaygroundVideoFetchScope 在视频轮询前把任务归属者的计费作用域还原到上下文。
//
// /pg/video 的提交可能由组织额度兜底承担（billing_account_type=organization），
// 而任务查询按上下文作用域隔离（AsyncTaskScopeFromContext）；轮询若固定用个人
// 作用域会查不到任务。这里按 task_id + 调用者归属无作用域加载任务，命中则把任务
// 行上的作用域写回上下文；不属于调用者或加载失败时保持现状（后续查询自然 not-found）。
func preparePlaygroundVideoFetchScope(c *gin.Context) {
	task, exist, err := model.GetByTaskIdAndUser(c.Param("task_id"), c.GetInt("id"))
	if err != nil {
		common.SysLog(fmt.Sprintf("playground video fetch: load task failed (taskId=%s): %s", c.Param("task_id"), err.Error()))
		return
	}
	if !exist || task == nil {
		return
	}
	model.NormalizeTaskBillingScope(task)
	common.SetContextKey(c, constant.ContextKeyScopeType, task.ScopeType)
	common.SetContextKey(c, constant.ContextKeyScopeId, task.ScopeId)
	common.SetContextKey(c, constant.ContextKeyBillingAccountType, task.BillingAccountType)
	common.SetContextKey(c, constant.ContextKeyBillingAccountId, task.BillingAccountId)
	common.SetContextKey(c, constant.ContextKeyOrganizationId, task.OrganizationId)
	common.SetContextKey(c, constant.ContextKeyActorUserId, task.ActorUserId)
	common.SetContextKey(c, constant.ContextKeyCreatorUserId, task.CreatorUserId)
	common.SetContextKey(c, constant.ContextKeyResponsibleUserId, task.ResponsibleUserId)
}
