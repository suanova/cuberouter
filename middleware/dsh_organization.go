/*
Copyright (C) 2023-2026 QuantumNous
Copyright (C) 2026 CubeRouter

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package middleware

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// dshOrganizationIdHeader 让前端把一个会话固定在某个组织的 DSH 实例上。
// 解析失败不会被忽略：显式声明了组织却读不出来时静默换组织，等于把会话
// 悄悄路由到另一个容器。
const dshOrganizationIdHeader = "X-DSH-Organization-Id"

// DshOrganization 为会话提交（POST /pg/agent/tasks）建立组织计费上下文，
// 并把选路 pin 到该组织绑定的 DSH 渠道。
//
// 必须注册在 UserAuth 之后、Distribute 之前：它改分组（影响渠道选择）并加 pin。
// 不写 rememberPlaygroundOrganization：DSH 的选择不该影响钱包兜底的历史组织。
func DshOrganization() gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := dshRequesterUserId(c)
		if userId <= 0 {
			abortDshOrganizationError(c, service.ErrDshOrganizationMissing)
			return
		}
		preferredOrganizationId, ctxErr := dshPreferredOrganizationId(c, userId)
		if ctxErr != nil {
			abortAccountContext(c, ctxErr.status, ctxErr.code, ctxErr.message)
			return
		}
		organization, channelId, err := service.ResolveDshOrganizationForUser(userId, preferredOrganizationId)
		if err != nil {
			abortDshOrganizationError(c, err)
			return
		}
		group := service.NormalizeOrganizationGroup(organization.Group)
		tempToken := &model.Token{
			UserId:         userId,
			Name:           "dsh-org-" + group,
			Group:          group,
			ScopeType:      model.TokenScopeOrganization,
			OrganizationId: organization.Id,
			Visibility:     model.TokenVisibilityPrivate,
		}
		scopeContext := service.BuildPlaygroundOrganizationScope(organization, userId)
		if err := SetupContextForTokenWithScopeContext(c, tempToken, scopeContext); err != nil {
			common.SysError("dsh organization: setup context failed for user " + strconv.Itoa(userId) + ": " + err.Error())
			abortDshOrganizationError(c, service.ErrDshOrganizationMissing)
			return
		}
		// 与 TokenAuth 的组织分支一致：user_group 决定可用分组，
		// using_group 决定本次用哪个分组。
		common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
		common.SetContextKey(c, constant.ContextKeyUserGroup, group)
		AddDshOrganizationPin(c, channelId)
		// CORS 没有暴露自定义响应头，前端不依赖它；仅供 curl/排障回显。
		c.Header(dshOrganizationIdHeader, strconv.Itoa(organization.Id))
		c.Next()
	}
}

// DshUploadOrganization 只解析并写入组织 ID：上传不选渠道、不计费，
// 重写计费上下文没有意义。
func DshUploadOrganization() gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := dshRequesterUserId(c)
		if userId <= 0 {
			abortDshOrganizationError(c, service.ErrDshOrganizationMissing)
			return
		}
		preferredOrganizationId, ctxErr := dshPreferredOrganizationId(c, userId)
		if ctxErr != nil {
			abortAccountContext(c, ctxErr.status, ctxErr.code, ctxErr.message)
			return
		}
		organization, _, err := service.ResolveDshOrganizationForUser(userId, preferredOrganizationId)
		if err != nil {
			abortDshOrganizationError(c, err)
			return
		}
		common.SetContextKey(c, constant.ContextKeyOrganizationId, organization.Id)
		if common.GetContextKeyInt(c, constant.ContextKeyActorUserId) <= 0 {
			common.SetContextKey(c, constant.ContextKeyActorUserId, userId)
		}
		c.Next()
	}
}

// AddDshOrganizationPin pins distribution to the organization's bound channel.
// Rank 5 sits between the admin token pin (0) and the origin-task pin (10).
func AddDshOrganizationPin(c *gin.Context, channelId int) {
	service.GetChannelConstraints(c).AddPin(dto.ChannelPin{
		ChannelId: channelId,
		Source:    dto.PinSourceOrganization,
		Rank:      dto.PinRankOrganization,
		RetryMode: dto.PinRetrySingleAttempt,
	})
}

// dshPreferredOrganizationId 解析调用方希望使用的组织：
//  1. X-DSH-Organization-Id（会话级固定）；
//  2. 账号上下文头（/pg/agent 没有 AccountContext 中间件，这里自行解析）；
//  3. 0，交给 service 自动选择。
//
// 第 1 步只做语法校验：成员关系与开通状态由 ResolveDshOrganizationForUser
// 判定（非成员 → 403；是成员但未开通 → 回退到自动选择）。
func dshPreferredOrganizationId(c *gin.Context, userId int) (int, *accountContextError) {
	if userId <= 0 {
		return 0, &accountContextError{status: http.StatusForbidden, code: types.ErrorCodeOrganizationAccessDenied, message: "organization access denied"}
	}
	if raw := strings.TrimSpace(c.GetHeader(dshOrganizationIdHeader)); raw != "" {
		organizationId, err := strconv.Atoi(raw)
		if err != nil || organizationId <= 0 {
			return 0, &accountContextError{status: http.StatusBadRequest, code: types.ErrorCodeOrganizationContextMismatch, message: "invalid DSH organization id"}
		}
		return organizationId, nil
	}
	accountContext, ctxErr := resolveRequestAccountContext(c, userId)
	if ctxErr != nil {
		return 0, ctxErr
	}
	if accountContext.Type == model.AccountContextTypeOrganization {
		return accountContext.Id, nil
	}
	return 0, nil
}

func abortDshOrganizationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrDshOrganizationRequired):
		abortWithOpenAiMessage(c, http.StatusForbidden, common.TranslateMessage(c, i18n.MsgDshOrganizationRequired))
	case errors.Is(err, service.ErrDshChannelUnavailable):
		abortWithOpenAiMessage(c, http.StatusForbidden, common.TranslateMessage(c, i18n.MsgDshOrganizationChannelUnavailable))
	default:
		abortWithOpenAiMessage(c, http.StatusForbidden, common.TranslateMessage(c, i18n.MsgDshOrganizationUnavailable))
	}
}
