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
package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPreparePluginRoundContextScope 覆盖插件轮次子上下文的作用域选择：
// 外层请求命中游乐场组织兜底时必须复制组织作用域（钱包已见底的用户否则会在
// 每一轮被个人作用域 403），否则保持个人临时令牌路径不变。
func TestPreparePluginRoundContextScope(t *testing.T) {
	setupPluginTestDB(t)
	personalGroup := "pg-personal"
	user := model.User{
		Username: "pg-plugin-org",
		Status:   common.UserStatusEnabled,
		Group:    personalGroup,
		Quota:    0,
		AffCode:  "pg-plugin-org",
	}
	require.NoError(t, model.DB.Create(&user).Error)

	const (
		outerGroup     = "pg-org-group"
		organizationId = 4711
	)
	// 组织分支只复制外层作用域、不重新查库校验：下面的用例不建组织行，
	// 走 ValidateTokenScopeForRelay 就会失败。

	tests := []struct {
		name               string
		organizationScoped bool
		wantBillingType    string
		wantBillingId      int
		wantTokenName      string
		wantUserGroup      string
	}{
		{
			name:               "组织兜底复制外层作用域",
			organizationScoped: true,
			wantBillingType:    model.AccountContextTypeOrganization,
			wantBillingId:      organizationId,
			wantTokenName:      "playground-plugin-org-" + outerGroup,
			wantUserGroup:      outerGroup,
		},
		{
			name:               "个人路径保持个人令牌",
			organizationScoped: false,
			wantBillingType:    model.AccountContextTypePersonal,
			wantBillingId:      user.Id,
			wantTokenName:      "playground-plugin-" + outerGroup,
			wantUserGroup:      personalGroup,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outer := playgroundControllerTestContext(http.MethodPost, "/pg/chat/completions")
			outer.Set("id", user.Id)
			if test.organizationScoped {
				// 与 middleware.SetupContextForTokenWithScopeContext 写进外层请求的
				// 键保持一致（PlaygroundOrganizationFallback 走的就是那条路径）。
				common.SetContextKey(outer, constant.ContextKeyScopeType, model.AccountContextTypeOrganization)
				common.SetContextKey(outer, constant.ContextKeyScopeId, organizationId)
				common.SetContextKey(outer, constant.ContextKeyBillingAccountType, model.AccountContextTypeOrganization)
				common.SetContextKey(outer, constant.ContextKeyBillingAccountId, organizationId)
				common.SetContextKey(outer, constant.ContextKeyOrganizationId, organizationId)
				common.SetContextKey(outer, constant.ContextKeyActorUserId, user.Id)
				common.SetContextKey(outer, constant.ContextKeyCreatorUserId, user.Id)
				common.SetContextKey(outer, constant.ContextKeyResponsibleUserId, user.Id)
				common.SetContextKey(outer, constant.ContextKeyOrganizationQuota, 123456)
			}
			subCtx := playgroundControllerTestContext(http.MethodPost, "/pg/chat/completions")

			require.Nil(t, preparePluginRoundContext(outer, subCtx, user.Id, outerGroup))

			assert.Equal(t, test.wantBillingType, common.GetContextKeyString(subCtx, constant.ContextKeyBillingAccountType))
			assert.Equal(t, test.wantBillingId, common.GetContextKeyInt(subCtx, constant.ContextKeyBillingAccountId))
			assert.Equal(t, outerGroup, common.GetContextKeyString(subCtx, constant.ContextKeyUsingGroup))
			assert.Equal(t, test.wantUserGroup, common.GetContextKeyString(subCtx, constant.ContextKeyUserGroup))
			assert.Equal(t, test.wantTokenName, subCtx.GetString("token_name"))
			assert.Equal(t, user.Id, subCtx.GetInt("id"))
			if test.organizationScoped {
				assert.Equal(t, organizationId, common.GetContextKeyInt(subCtx, constant.ContextKeyOrganizationId))
				assert.Equal(t, model.AccountContextTypeOrganization, common.GetContextKeyString(subCtx, constant.ContextKeyScopeType))
				assert.Equal(t, user.Id, common.GetContextKeyInt(subCtx, constant.ContextKeyActorUserId))
				assert.Equal(t, user.Id, common.GetContextKeyInt(subCtx, constant.ContextKeyCreatorUserId))
				assert.Equal(t, user.Id, common.GetContextKeyInt(subCtx, constant.ContextKeyResponsibleUserId))
				assert.Equal(t, 123456, common.GetContextKeyInt(subCtx, constant.ContextKeyOrganizationQuota))
			}
		})
	}
}
