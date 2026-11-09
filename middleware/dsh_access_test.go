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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setDshAllowedOrganizationsForMiddlewareTest(t *testing.T, organizationIds []int) {
	t.Helper()
	encoded := "[]"
	if len(organizationIds) > 0 {
		raw, err := common.Marshal(organizationIds)
		require.NoError(t, err)
		encoded = string(raw)
	}

	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap[setting.DshAllowedOrganizationIdsKey] = encoded
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		delete(common.OptionMap, setting.DshAllowedOrganizationIdsKey)
	})
}

func setDshOrganizationBindingsForMiddlewareTest(t *testing.T, bindings map[int]int) {
	t.Helper()
	encoded := "{}"
	if len(bindings) > 0 {
		raw, err := common.Marshal(bindings)
		require.NoError(t, err)
		encoded = string(raw)
	}

	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap[setting.DshOrganizationChannelsKey] = encoded
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		delete(common.OptionMap, setting.DshOrganizationChannelsKey)
	})
}

func createDshMiddlewareTestUser(t *testing.T, username string, role int) model.User {
	t.Helper()
	user := model.User{Username: username, Password: "password", DisplayName: username, Role: role, Status: common.UserStatusEnabled, AffCode: username}
	require.NoError(t, model.DB.Create(&user).Error)
	return user
}

func createDshMiddlewareTestChannel(t *testing.T, name string, status int) model.Channel {
	t.Helper()
	settings := `{"task_plugin_key":"dsh"}`
	baseURL := "http://" + name + ":3080"
	channel := model.Channel{
		Type:    constant.ChannelTypeTaskPlugin,
		Key:     "dsh-middleware-key",
		Status:  status,
		Name:    name,
		BaseURL: &baseURL,
		Group:   "default",
		Models:  "dsh-model",
		Setting: &settings,
	}
	require.NoError(t, model.DB.Create(&channel).Error)
	return channel
}

type dshProbeOptions struct {
	userId         int
	actorUserId    int
	organizationId int
	taskPluginKey  string
	presetTokenPin int
}

// probeDshMiddleware runs handler behind a route that seeds the caller identity
// the way the real auth middlewares do: "id" for the session/token user, the
// actor context key for token-scoped calls, the organization context for
// organization tokens, and the resolved task plugin key.
func probeDshMiddleware(t *testing.T, handler gin.HandlerFunc, options dshProbeOptions) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", options.userId)
		if options.actorUserId > 0 {
			c.Set(string(constant.ContextKeyActorUserId), options.actorUserId)
		}
		if options.organizationId > 0 {
			common.SetContextKey(c, constant.ContextKeyOrganizationId, options.organizationId)
		}
		if options.taskPluginKey != "" {
			c.Set("expected_task_plugin_key", options.taskPluginKey)
		}
		if options.presetTokenPin > 0 {
			// 管理员令牌 key 里带 -<channelId> 后缀时 TokenAuth 会加 rank 0 的 pin。
			service.GetChannelConstraints(c).AddPin(dto.ChannelPin{
				ChannelId: options.presetTokenPin,
				Source:    dto.PinSourceToken,
				Rank:      dto.PinRankToken,
				RetryMode: dto.PinRetrySingleAttempt,
			})
		}
	})
	router.Use(handler)
	router.GET("/probe", func(c *gin.Context) {
		recorder := httptest.NewRecorder()
		_ = recorder
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/probe", nil))
	return recorder
}

func TestDshAccessMiddleware(t *testing.T) {
	setupOrganizationMiddlewareTestDB(t, &model.User{}, &model.Organization{}, &model.OrganizationMember{})

	admin := createDshMiddlewareTestUser(t, "dsh-middleware-admin", common.RoleAdminUser)
	member := createDshMiddlewareTestUser(t, "dsh-middleware-member", common.RoleCommonUser)
	outsider := createDshMiddlewareTestUser(t, "dsh-middleware-outsider", common.RoleCommonUser)

	organization := model.Organization{Name: "DSH Middleware Organization", Status: model.OrganizationStatusActive}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: organization.Id,
		UserId:         member.Id,
		Role:           model.OrganizationRoleMember,
		Status:         model.OrganizationMemberStatusActive,
	}).Error)

	enableOrganization := func(t *testing.T) {
		setDshAllowedOrganizationsForMiddlewareTest(t, []int{organization.Id})
		setDshOrganizationBindingsForMiddlewareTest(t, map[int]int{organization.Id: 340})
	}

	t.Run("denies a user outside the enabled organizations", func(t *testing.T) {
		enableOrganization(t)
		recorder := probeDshMiddleware(t, DshAccess(), dshProbeOptions{userId: outsider.Id})
		require.Equal(t, http.StatusForbidden, recorder.Code)
		require.Contains(t, recorder.Body.String(), translated(i18n.MsgDshAccessDenied))
	})

	t.Run("allows a member of an enabled organization", func(t *testing.T) {
		enableOrganization(t)
		recorder := probeDshMiddleware(t, DshAccess(), dshProbeOptions{userId: member.Id})
		require.Equal(t, http.StatusOK, recorder.Code)
	})

	// DSH 任务按组织额度计费：没有组织成员关系就没有账本可记，管理员也一样。
	t.Run("denies a platform admin without an organization", func(t *testing.T) {
		enableOrganization(t)
		recorder := probeDshMiddleware(t, DshAccess(), dshProbeOptions{userId: admin.Id})
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("denies a member when nothing is enabled", func(t *testing.T) {
		setDshAllowedOrganizationsForMiddlewareTest(t, nil)
		setDshOrganizationBindingsForMiddlewareTest(t, nil)
		recorder := probeDshMiddleware(t, DshAccess(), dshProbeOptions{userId: member.Id})
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("denies a member of an allowlisted but unbound organization", func(t *testing.T) {
		setDshAllowedOrganizationsForMiddlewareTest(t, []int{organization.Id})
		setDshOrganizationBindingsForMiddlewareTest(t, nil)
		recorder := probeDshMiddleware(t, DshAccess(), dshProbeOptions{userId: member.Id})
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("evaluates the acting user rather than the token owner", func(t *testing.T) {
		enableOrganization(t)
		recorder := probeDshMiddleware(t, DshAccess(), dshProbeOptions{userId: outsider.Id, actorUserId: member.Id})
		require.Equal(t, http.StatusOK, recorder.Code)

		recorder = probeDshMiddleware(t, DshAccess(), dshProbeOptions{userId: member.Id, actorUserId: outsider.Id})
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})
}

func TestDshSubmitAccessMiddleware(t *testing.T) {
	setupOrganizationMiddlewareTestDB(t,
		&model.User{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.Channel{},
	)
	originalMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCache })

	member := createDshMiddlewareTestUser(t, "dsh-submit-member", common.RoleCommonUser)
	outsider := createDshMiddlewareTestUser(t, "dsh-submit-outsider", common.RoleCommonUser)

	organization := model.Organization{Name: "DSH Submit Organization", Status: model.OrganizationStatusActive}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: organization.Id,
		UserId:         member.Id,
		Role:           model.OrganizationRoleMember,
		Status:         model.OrganizationMemberStatusActive,
	}).Error)
	channel := createDshMiddlewareTestChannel(t, "dsh-submit-channel", common.ChannelStatusEnabled)
	disabledChannel := createDshMiddlewareTestChannel(t, "dsh-submit-disabled", common.ChannelStatusManuallyDisabled)

	enableOrganization := func(t *testing.T, channelId int) {
		setDshAllowedOrganizationsForMiddlewareTest(t, []int{organization.Id})
		setDshOrganizationBindingsForMiddlewareTest(t, map[int]int{organization.Id: channelId})
	}

	t.Run("passes through for another task plugin", func(t *testing.T) {
		recorder := probeDshMiddleware(t, DshSubmitAccess(), dshProbeOptions{
			userId:        outsider.Id,
			taskPluginKey: "other-plugin",
		})
		require.Equal(t, http.StatusOK, recorder.Code)
	})

	// 个人令牌没有组织作用域：严格拒绝，管理员/root 也不例外。
	t.Run("rejects a personal token", func(t *testing.T) {
		enableOrganization(t, channel.Id)
		recorder := probeDshMiddleware(t, DshSubmitAccess(), dshProbeOptions{
			userId:        member.Id,
			taskPluginKey: setting.DshTaskPluginKey,
		})
		require.Equal(t, http.StatusForbidden, recorder.Code)
		require.Contains(t, recorder.Body.String(), translated(i18n.MsgDshOrganizationRequired))
	})

	t.Run("rejects a token whose organization is not enabled", func(t *testing.T) {
		setDshAllowedOrganizationsForMiddlewareTest(t, nil)
		setDshOrganizationBindingsForMiddlewareTest(t, nil)
		recorder := probeDshMiddleware(t, DshSubmitAccess(), dshProbeOptions{
			userId:         member.Id,
			organizationId: organization.Id,
			taskPluginKey:  setting.DshTaskPluginKey,
		})
		require.Equal(t, http.StatusForbidden, recorder.Code)
		require.Contains(t, recorder.Body.String(), translated(i18n.MsgDshOrganizationUnavailable))
	})

	t.Run("rejects a token whose organization channel is stopped", func(t *testing.T) {
		enableOrganization(t, disabledChannel.Id)
		recorder := probeDshMiddleware(t, DshSubmitAccess(), dshProbeOptions{
			userId:         member.Id,
			organizationId: organization.Id,
			taskPluginKey:  setting.DshTaskPluginKey,
		})
		require.Equal(t, http.StatusForbidden, recorder.Code)
		require.Contains(t, recorder.Body.String(), translated(i18n.MsgDshOrganizationChannelUnavailable))
	})

	t.Run("pins the organization channel for an organization token", func(t *testing.T) {
		enableOrganization(t, channel.Id)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("id", member.Id)
			common.SetContextKey(c, constant.ContextKeyOrganizationId, organization.Id)
			c.Set("expected_task_plugin_key", setting.DshTaskPluginKey)
		})
		router.Use(DshSubmitAccess())
		router.GET("/probe", func(c *gin.Context) {
			pin, found, _ := service.GetChannelConstraints(c).ResolvedPin()
			require.True(t, found)
			require.Equal(t, dto.PinSourceOrganization, pin.Source)
			require.Equal(t, channel.Id, pin.ChannelId)
			c.JSON(http.StatusOK, gin.H{"success": true})
		})

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/probe", nil))
		require.Equal(t, http.StatusOK, recorder.Code)
	})

	// 管理员令牌 key 里的 -<channelId> 后缀会产生 rank 0 的 token pin，
	// 它必须不能让 DSH 任务落到别的实例上。
	t.Run("refuses to be overridden by a token channel pin", func(t *testing.T) {
		enableOrganization(t, channel.Id)
		recorder := probeDshMiddleware(t, DshSubmitAccess(), dshProbeOptions{
			userId:         member.Id,
			organizationId: organization.Id,
			taskPluginKey:  setting.DshTaskPluginKey,
			presetTokenPin: channel.Id + 100,
		})
		require.Equal(t, http.StatusForbidden, recorder.Code)
		require.Contains(t, recorder.Body.String(), translated(i18n.MsgDshOrganizationChannelUnavailable))
	})
}
