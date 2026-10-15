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
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

// PlaygroundOrganizationFallback 是 /pg 生成入口的组织额度兜底。
//
// 触发条件：钱包是该用户的有效资金来源（不会被优先消费的活跃订阅兜住）且钱包
// 余量低于阈值（setting.PlaygroundOrgFallbackThreshold）。命中后选择组织并把
// 组织作用域写入上下文：Distribute 按组织分组选渠道，计费走组织账本。
//
// 必须注册在 UserAuth 之后（需要会话用户）、Distribute 之前（分组影响渠道选择）。
// 任一步失败都 fail-open 降级为个人路径：兜底是增强，不该挡住正常请求。
func PlaygroundOrganizationFallback() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost ||
			c.GetBool("use_access_token") ||
			common.GetContextKeyString(c, constant.ContextKeyBillingAccountType) == model.AccountContextTypeOrganization {
			c.Next()
			return
		}
		userId := c.GetInt("id")
		if userId <= 0 || !shouldSwitchToOrganization(userId) {
			c.Next()
			return
		}
		organization, ok, err := service.SelectPlaygroundOrganization(userId)
		if err != nil {
			common.SysLog(fmt.Sprintf("playground organization fallback: select organization failed (userId=%d): %s", userId, err.Error()))
			c.Next()
			return
		}
		if !ok || organization == nil {
			c.Next()
			return
		}
		group := service.NormalizeOrganizationGroup(organization.Group)
		tempToken := &model.Token{
			UserId:         userId,
			Name:           "playground-org-" + group,
			Group:          group,
			ScopeType:      model.TokenScopeOrganization,
			OrganizationId: organization.Id,
			Visibility:     model.TokenVisibilityPrivate,
		}
		scopeContext := service.BuildPlaygroundOrganizationScope(organization, userId)
		if err := SetupContextForTokenWithScopeContext(c, tempToken, scopeContext); err != nil {
			common.SysLog(fmt.Sprintf("playground organization fallback: setup context failed (userId=%d, organizationId=%d): %s", userId, organization.Id, err.Error()))
			c.Next()
			return
		}
		// 与 TokenAuth 的组织分支一致（middleware/auth.go:474-498）：
		// user_group 决定「这个账户能用哪些分组」，using_group 决定「本次用哪个分组」。
		common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
		common.SetContextKey(c, constant.ContextKeyUserGroup, group)
		rememberPlaygroundOrganization(userId, organization.Id)
		c.Next()
	}
}

// shouldSwitchToOrganization 判断该用户这次请求是否会由钱包承担。
// 会先消费订阅的偏好（subscription_first / subscription_only）与钱包之后仍有
// 订阅兜底的 wallet_first 都不切换：组织额度不该越过用户自己的订阅。
// wallet_only 明确排除订阅，仍按钱包判定。
func shouldSwitchToOrganization(userId int) bool {
	userSetting, err := model.GetUserSetting(userId, false)
	if err != nil {
		common.SysLog(fmt.Sprintf("playground organization fallback: load setting failed (userId=%d): %s", userId, err.Error()))
		return false
	}
	preference := common.NormalizeBillingPreference(userSetting.BillingPreference)
	if preference == "subscription_first" || preference == "subscription_only" || preference == "wallet_first" {
		hasSubscription, err := model.HasActiveUserSubscription(userId)
		if err != nil {
			common.SysLog(fmt.Sprintf("playground organization fallback: load subscription state failed (userId=%d): %s", userId, err.Error()))
			return false
		}
		if hasSubscription {
			return false
		}
	}
	quota, err := model.GetUserQuota(userId, false)
	if err != nil {
		common.SysLog(fmt.Sprintf("playground organization fallback: load quota failed (userId=%d): %s", userId, err.Error()))
		return false
	}
	return quota < setting.PlaygroundOrgFallbackThreshold()
}

// rememberPlaygroundOrganization 记住本次兜底选中的组织（选择规则的下一次输入）。
// 仅值变化时写入：稳态下同一组织不会每请求都写用户设置。
func rememberPlaygroundOrganization(userId int, organizationId int) {
	userSetting, err := model.GetUserSetting(userId, false)
	if err != nil {
		common.SysLog(fmt.Sprintf("playground organization fallback: load setting for remember failed (userId=%d): %s", userId, err.Error()))
		return
	}
	if userSetting.PlaygroundLastOrganizationId == organizationId {
		return
	}
	userSetting.PlaygroundLastOrganizationId = organizationId
	if err := model.UpdateUserSetting(userId, userSetting); err != nil {
		common.SysLog(fmt.Sprintf("playground organization fallback: remember organization failed (userId=%d, organizationId=%d): %s", userId, organizationId, err.Error()))
	}
}
