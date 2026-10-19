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
	"fmt"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupPersonalTokenModelAccessTest 建立个人令牌用例的 sqlite 环境与用户的组织成员行。
// modelAccess 为空串表示该用户没有受限成员行 —— 成员白名单为 nil（不受限）。
func setupPersonalTokenModelAccessTest(t *testing.T, modelAccess string) *model.User {
	t.Helper()
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Organization{}, &model.OrganizationMember{}))
	user := &model.User{
		Id:       101,
		Username: "token-model-access",
		Password: "password",
		Role:     common.RoleCommonUser,
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)
	if modelAccess != "" {
		// 成员行只在组织 active 时才构成限制：受限用例必须有一条真实的组织行，
		// 否则限制会因组织缺失而失效，用例就不是在测它宣称的东西了。
		require.NoError(t, db.Create(&model.Organization{
			Id:        1,
			Name:      "token-model-access-org",
			Slug:      "token-model-access-org",
			Status:    model.OrganizationStatusActive,
			CreatedBy: user.Id,
		}).Error)
		require.NoError(t, db.Create(&model.OrganizationMember{
			OrganizationId: 1,
			UserId:         user.Id,
			Role:           model.OrganizationRoleMember,
			Status:         model.OrganizationMemberStatusActive,
			ModelAccess:    modelAccess,
		}).Error)
	}
	return user
}

// tokenModelAccessTranslation 是 common.TranslateMessage 的记录型替身。controller 包不能
// 调用 i18n.Init()（那个 sync.Once 会把 common.TranslateMessage 永久换成 i18n.T，破坏本包
// 后续依赖原始 key 兜底的用例，见 user_export_test.go），因此改用局部替身：观察处理器送到
// 翻译层的键与参数（用例结束立即还原），并按生产模板的形态把模型名渲染进消息。
type tokenModelAccessTranslation struct {
	key  string
	args map[string]any
}

func (translation *tokenModelAccessTranslation) translate(_ *gin.Context, key string, args ...map[string]any) string {
	translation.key = key
	if len(args) > 0 {
		translation.args = args[0]
	}
	return fmt.Sprintf("%s: %v", key, translation.args["Model"])
}

func installTokenModelAccessTranslation(t *testing.T) *tokenModelAccessTranslation {
	t.Helper()
	translation := &tokenModelAccessTranslation{}
	previous := common.TranslateMessage
	common.TranslateMessage = translation.translate
	t.Cleanup(func() { common.TranslateMessage = previous })
	return translation
}

// assertTokenModelAccessRejection 断言请求被成员白名单拦下：命中专用 i18n 键、消息点名
// 越界模型。副作用由各自用例单独断言。
func assertTokenModelAccessRejection(t *testing.T, translation *tokenModelAccessTranslation, response tokenAPIResponse, deniedModel string) {
	t.Helper()
	assert.False(t, response.Success)
	assert.Equal(t, i18n.MsgTokenModelOutsideMemberAccess, translation.key, "rejection must use the member access key")
	assert.Equal(t, deniedModel, translation.args["Model"], "the translated message must carry the offending model")
	assert.Contains(t, response.Message, deniedModel, "the response message must name the offending model")
}

// TestAddTokenMemberModelAccess 覆盖建个人令牌时按归属用户的成员白名单校验 model limits：
// 集内放行、任一条集外拒绝、不启用/为空放行、白名单 nil 全部放行。
func TestAddTokenMemberModelAccess(t *testing.T) {
	tests := []struct {
		name               string
		modelAccess        string // "" = 用户没有受限成员行（白名单 nil）
		modelLimitsEnabled bool
		modelLimits        string
		wantDeniedModel    string // 空串 = 期望成功
	}{
		{
			name: "limits within the member allow list", modelAccess: `["allowed-model","second-model"]`,
			modelLimitsEnabled: true, modelLimits: "allowed-model",
		},
		{
			name: "limits outside the member allow list", modelAccess: `["allowed-model"]`,
			modelLimitsEnabled: true, modelLimits: "denied-model", wantDeniedModel: "denied-model",
		},
		{
			name: "one outside entry rejects the whole list", modelAccess: `["allowed-model"]`,
			modelLimitsEnabled: true, modelLimits: "allowed-model,denied-model", wantDeniedModel: "denied-model",
		},
		{
			name: "disabled limits pass regardless", modelAccess: `["allowed-model"]`,
			modelLimitsEnabled: false, modelLimits: "denied-model",
		},
		{
			name: "empty limits count as unset", modelAccess: `["allowed-model"]`,
			modelLimitsEnabled: true, modelLimits: "",
		},
		{
			name: "unrestricted owner passes any limits", modelAccess: "",
			modelLimitsEnabled: true, modelLimits: "denied-model",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			user := setupPersonalTokenModelAccessTest(t, tc.modelAccess)
			translation := installTokenModelAccessTranslation(t)
			request := map[string]any{
				"name":                 "member-access-add",
				"expired_time":         -1,
				"remain_quota":         0,
				"unlimited_quota":      true,
				"model_limits_enabled": tc.modelLimitsEnabled,
				"model_limits":         tc.modelLimits,
				"group":                "default",
			}

			ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", request, user.Id)
			AddToken(ctx)

			response := decodeAPIResponse(t, recorder)
			var storedTokens []model.Token
			require.NoError(t, model.DB.Find(&storedTokens).Error)
			if tc.wantDeniedModel != "" {
				assertTokenModelAccessRejection(t, translation, response, tc.wantDeniedModel)
				assert.Empty(t, storedTokens, "a rejected create must not store a key")
				return
			}
			require.True(t, response.Success, response.Message)
			require.Len(t, storedTokens, 1)
			assert.Equal(t, tc.modelLimitsEnabled, storedTokens[0].ModelLimitsEnabled)
			assert.Equal(t, tc.modelLimits, storedTokens[0].ModelLimits)
		})
	}
}

// TestUpdateTokenMemberModelAccess 覆盖改个人令牌时同样的校验，并断言越界被拒的写入
// 不留任何痕迹（存量 model limits 保持原样）。
func TestUpdateTokenMemberModelAccess(t *testing.T) {
	tests := []struct {
		name               string
		modelAccess        string
		modelLimitsEnabled bool
		modelLimits        string
		wantDeniedModel    string
	}{
		{
			name: "limits within the member allow list", modelAccess: `["allowed-model","second-model"]`,
			modelLimitsEnabled: true, modelLimits: "second-model",
		},
		{
			name: "limits outside the member allow list", modelAccess: `["allowed-model"]`,
			modelLimitsEnabled: true, modelLimits: "denied-model", wantDeniedModel: "denied-model",
		},
		{
			name: "one outside entry rejects the whole list", modelAccess: `["allowed-model"]`,
			modelLimitsEnabled: true, modelLimits: "allowed-model,denied-model", wantDeniedModel: "denied-model",
		},
		{
			name: "disabled limits pass regardless", modelAccess: `["allowed-model"]`,
			modelLimitsEnabled: false, modelLimits: "denied-model",
		},
		{
			name: "empty limits count as unset", modelAccess: `["allowed-model"]`,
			modelLimitsEnabled: true, modelLimits: "",
		},
		{
			name: "unrestricted owner passes any limits", modelAccess: "",
			modelLimitsEnabled: true, modelLimits: "denied-model",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			user := setupPersonalTokenModelAccessTest(t, tc.modelAccess)
			translation := installTokenModelAccessTranslation(t)
			token := seedToken(t, model.DB, user.Id, "member-access-update", "member-access-update-key")
			require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).
				Updates(map[string]any{"model_limits_enabled": true, "model_limits": "allowed-model"}).Error)

			request := map[string]any{
				"id":                   token.Id,
				"name":                 token.Name,
				"status":               common.TokenStatusEnabled,
				"expired_time":         -1,
				"remain_quota":         100,
				"unlimited_quota":      true,
				"model_limits_enabled": tc.modelLimitsEnabled,
				"model_limits":         tc.modelLimits,
				"group":                "default",
			}

			ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", request, user.Id)
			UpdateToken(ctx)

			response := decodeAPIResponse(t, recorder)
			var stored model.Token
			require.NoError(t, model.DB.First(&stored, token.Id).Error)
			if tc.wantDeniedModel != "" {
				assertTokenModelAccessRejection(t, translation, response, tc.wantDeniedModel)
				assert.True(t, stored.ModelLimitsEnabled, "a rejected update must leave the stored limits enabled")
				assert.Equal(t, "allowed-model", stored.ModelLimits, "a rejected update must leave the stored limits untouched")
				return
			}
			require.True(t, response.Success, response.Message)
			assert.Equal(t, tc.modelLimitsEnabled, stored.ModelLimitsEnabled)
			assert.Equal(t, tc.modelLimits, stored.ModelLimits)
		})
	}
}

// TestUpdateTokenRevalidatesMemberModelLimitsOnlyWhenChanged 覆盖白名单收窄后的存量个人
// 令牌：请求原样带回存量 limits（改名等无关编辑）时不该被拦，否则这把 Key 会永久无法
// 修复；真正改动 limits 时仍按当前白名单拒绝。
func TestUpdateTokenRevalidatesMemberModelLimitsOnlyWhenChanged(t *testing.T) {
	updateRequest := func(token *model.Token, name string, modelLimits string) map[string]any {
		return map[string]any{
			"id":                   token.Id,
			"name":                 name,
			"status":               common.TokenStatusEnabled,
			"expired_time":         -1,
			"remain_quota":         100,
			"unlimited_quota":      true,
			"model_limits_enabled": true,
			"model_limits":         modelLimits,
			"group":                "default",
		}
	}

	t.Run("unchanged limits pass after the allow list narrowed", func(t *testing.T) {
		user := setupPersonalTokenModelAccessTest(t, `["allowed-model"]`)
		translation := installTokenModelAccessTranslation(t)
		token := seedToken(t, model.DB, user.Id, "stale-limits-rename", "stale-limits-rename-key")
		require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).
			Updates(map[string]any{"model_limits_enabled": true, "model_limits": "stale-model"}).Error)

		ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/",
			updateRequest(token, "renamed-stale-limits", "stale-model"), user.Id)
		UpdateToken(ctx)

		response := decodeAPIResponse(t, recorder)
		require.True(t, response.Success, response.Message)
		assert.Empty(t, translation.key, "an unchanged-limit update must not consult the member allow list")
		var stored model.Token
		require.NoError(t, model.DB.First(&stored, token.Id).Error)
		assert.Equal(t, "renamed-stale-limits", stored.Name)
		assert.True(t, stored.ModelLimitsEnabled)
		assert.Equal(t, "stale-model", stored.ModelLimits)
	})

	t.Run("changed limits outside the allow list are still rejected", func(t *testing.T) {
		user := setupPersonalTokenModelAccessTest(t, `["allowed-model"]`)
		translation := installTokenModelAccessTranslation(t)
		token := seedToken(t, model.DB, user.Id, "stale-limits-change", "stale-limits-change-key")
		require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).
			Updates(map[string]any{"model_limits_enabled": true, "model_limits": "stale-model"}).Error)

		ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/",
			updateRequest(token, "changed-limits", "denied-model"), user.Id)
		UpdateToken(ctx)

		response := decodeAPIResponse(t, recorder)
		assertTokenModelAccessRejection(t, translation, response, "denied-model")
		var stored model.Token
		require.NoError(t, model.DB.First(&stored, token.Id).Error)
		assert.Equal(t, "stale-model", stored.ModelLimits, "a rejected update must leave the stored limits untouched")
	})
}

// TestUpdateTokenStatusOnlySkipsMemberModelAccess 覆盖仅改状态的更新路径：它在请求里带的
// model limits 不会被写入，也就不该被成员白名单拦下（前端开关只发 id + status）。
func TestUpdateTokenStatusOnlySkipsMemberModelAccess(t *testing.T) {
	user := setupPersonalTokenModelAccessTest(t, `["allowed-model"]`)
	token := seedToken(t, model.DB, user.Id, "member-access-status-only", "member-access-status-only-key")
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).
		Updates(map[string]any{"model_limits_enabled": true, "model_limits": "allowed-model"}).Error)

	request := map[string]any{
		"id":                   token.Id,
		"name":                 token.Name,
		"status":               common.TokenStatusDisabled,
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": true,
		"model_limits":         "denied-model",
		"group":                "default",
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/?status_only=true", request, user.Id)
	UpdateToken(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	assert.Equal(t, common.TokenStatusDisabled, stored.Status)
	assert.Equal(t, "allowed-model", stored.ModelLimits, "a status-only update must not write the request model limits")
}
