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
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	pricingAdminDefaultModel = "zz-pricing-admin-default"
	pricingAdminVipModel     = "zz-pricing-admin-vip"
	pricingAdminYeeboModel   = "zz-pricing-admin-yeebo"
	pricingAdminNoRatioModel = "zz-pricing-admin-no-ratio"
)

type pricingAdminViewResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Pricings []model.Pricing `json:"pricings"`
	} `json:"data"`
	UsableGroup map[string]string  `json:"usable_group"`
	GroupRatio  map[string]float64 `json:"group_ratio"`
}

// seedPricingAdminViewFixture 提供四个分组各一个模型：default/vip 是全局可用分组，
// yeebo-group 只在价目表里出现且配置了倍率，no-ratio-group 出现但不配置倍率。
func seedPricingAdminViewFixture(t *testing.T) (db *gorm.DB, admin, commonUser model.User) {
	t.Helper()
	db = setupPricingModelAccessTestDB(t)

	originalRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(
		`{"default":1,"vip":1,"yeebo-group":0.5}`))

	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: pricingAdminDefaultModel, ChannelId: 1, Enabled: true},
		{Group: "vip", Model: pricingAdminVipModel, ChannelId: 1, Enabled: true},
		{Group: "yeebo-group", Model: pricingAdminYeeboModel, ChannelId: 1, Enabled: true},
		{Group: "no-ratio-group", Model: pricingAdminNoRatioModel, ChannelId: 1, Enabled: true},
	}).Error)

	admin = createMemberModelAccessTestUser(t, db, "pricing-admin-view-admin", common.RoleAdminUser)
	commonUser = createMemberModelAccessTestUser(t, db, "pricing-admin-view-common", common.RoleCommonUser)
	return db, admin, commonUser
}

// performGetPricingRequestAsRole 直调 handler：补齐路由鉴权中间件会写入的 id/role。
func performGetPricingRequestAsRole(t *testing.T, viewerId, role int) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	if viewerId > 0 {
		context.Set("id", viewerId)
	}
	context.Set("role", role)
	GetPricing(context)
	return recorder
}

func decodePricingAdminViewResponse(t *testing.T, recorder *httptest.ResponseRecorder) pricingAdminViewResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload pricingAdminViewResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	return payload
}

func TestGetPricingAdminSeesAllSiteGroups(t *testing.T) {
	_, admin, commonUser := seedPricingAdminViewFixture(t)

	t.Run("admin sees every group in the price table", func(t *testing.T) {
		payload := decodePricingAdminViewResponse(t,
			performGetPricingRequestAsRole(t, admin.Id, admin.Role))
		byName := pricingByModelName(payload.Data.Pricings)
		for _, modelName := range []string{
			pricingAdminDefaultModel,
			pricingAdminVipModel,
			pricingAdminYeeboModel,
			pricingAdminNoRatioModel,
		} {
			require.Contains(t, byName, modelName)
		}
		require.Contains(t, payload.UsableGroup, "yeebo-group")
		require.Contains(t, payload.UsableGroup, "no-ratio-group")
		require.Equal(t, 0.5, payload.GroupRatio["yeebo-group"])
		require.NotContains(t, payload.GroupRatio, "no-ratio-group")
	})

	t.Run("common user keeps the usable-group view", func(t *testing.T) {
		payload := decodePricingAdminViewResponse(t,
			performGetPricingRequestAsRole(t, commonUser.Id, commonUser.Role))
		byName := pricingByModelName(payload.Data.Pricings)
		require.Contains(t, byName, pricingAdminDefaultModel)
		require.Contains(t, byName, pricingAdminVipModel)
		require.NotContains(t, byName, pricingAdminYeeboModel)
		require.NotContains(t, byName, pricingAdminNoRatioModel)
		require.NotContains(t, payload.UsableGroup, "yeebo-group")
		require.NotContains(t, payload.UsableGroup, "no-ratio-group")
	})

	t.Run("anonymous keeps the usable-group view", func(t *testing.T) {
		payload := decodePricingAdminViewResponse(t,
			performGetPricingRequestAsRole(t, 0, common.RoleGuestUser))
		byName := pricingByModelName(payload.Data.Pricings)
		require.Contains(t, byName, pricingAdminDefaultModel)
		require.NotContains(t, byName, pricingAdminYeeboModel)
		require.NotContains(t, payload.UsableGroup, "yeebo-group")
	})
}

func TestGetPricingAdminInOrganizationContextSeesAllSiteGroups(t *testing.T) {
	db, admin, _ := seedPricingAdminViewFixture(t)
	organization := createPricingTestOrganization(t, db, admin.Id, "pricing-admin-org", "org-group")
	require.NoError(t, db.Create(&model.UserAccountContext{
		UserId:      admin.Id,
		ContextType: model.AccountContextTypeOrganization,
		ContextId:   organization.Id,
	}).Error)

	payload := decodePricingAdminViewResponse(t,
		performGetPricingRequestAsRole(t, admin.Id, admin.Role))
	byName := pricingByModelName(payload.Data.Pricings)
	for _, modelName := range []string{
		pricingAdminDefaultModel,
		pricingAdminVipModel,
		pricingAdminYeeboModel,
		pricingAdminNoRatioModel,
	} {
		require.Contains(t, byName, modelName)
	}
}

func TestGetPricingAdminViewDoesNotPollutePricingCache(t *testing.T) {
	_, admin, _ := seedPricingAdminViewFixture(t)

	cachedBefore := append([]model.Pricing(nil), model.GetPricing()...)
	require.Len(t, cachedBefore, 4)

	payload := decodePricingAdminViewResponse(t,
		performGetPricingRequestAsRole(t, admin.Id, admin.Role))
	require.Len(t, payload.Data.Pricings, 4)

	cachedAfter := model.GetPricing()
	require.Equal(t, cachedBefore, cachedAfter, "管理员视图不得改写共享价目缓存")
}

func TestCollectPricingGroupsExpandsAdminView(t *testing.T) {
	groups := collectPricingGroups([]model.Pricing{
		{ModelName: "all-group-model", EnableGroup: []string{"all"}},
		{ModelName: "default-model", EnableGroup: []string{"default"}},
		{ModelName: "yeebo-model", EnableGroup: []string{"yeebo", "default"}},
		{ModelName: "blank-model", EnableGroup: []string{""}},
	})
	require.Len(t, groups, 2)
	require.NotEmpty(t, groups["default"])
	require.Equal(t, "yeebo", groups["yeebo"])
	require.NotContains(t, groups, "all")
	require.Empty(t, collectPricingGroups(nil))
}
