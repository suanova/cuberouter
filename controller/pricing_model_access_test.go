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
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 价目表白名单用例的三个模型：白名单放行前两个，第三个应当被过滤掉。zz- 前缀
// 避免与真实模型重名，也不会命中默认供应商规则。
const (
	pricingMemberAccessModelOne    = "zz-pricing-member-access-a"
	pricingMemberAccessModelTwo    = "zz-pricing-member-access-b"
	pricingMemberAccessModelHidden = "zz-pricing-member-access-c"
)

// setupPricingModelAccessTestDB 建好 GetPricing 整条链路要用的表：价目表重建要
// 能力/渠道/模型/供应商，分组解析要用户/组织上下文。SQL 日志静音：分组解析会
// 故意查不存在的账号上下文行，那是正常分支，不该混进测试输出。
func setupPricingModelAccessTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Organization{}, &model.OrganizationMember{},
		&model.UserAccountContext{}, &model.OrganizationDisableRecord{},
		&model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{},
	))
	// 价目表缓存是包级全局对象：先失效一次，保证本次按本测试自己的库重建；
	// 测试结束再失效，免得把测试模型留给后面的用例。
	model.InvalidatePricingCache()

	t.Cleanup(func() {
		model.InvalidatePricingCache()
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// seedPricingMemberAccessModels 让三个测试模型出现在 default 分组的价目表里。
func seedPricingMemberAccessModels(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedMemberModelAccessGroupAbilities(t, db,
		pricingMemberAccessModelOne, pricingMemberAccessModelTwo, pricingMemberAccessModelHidden)
}

// pricingMemberAccessAllowList 组装白名单 JSON（organization_members.model_access
// 的原始格式）。
func pricingMemberAccessAllowList(models ...string) string {
	return fmt.Sprintf(`["%s"]`, strings.Join(models, `","`))
}

// TestGetPricingFiltersByMemberModelAccess 登录用户带白名单 [a,b]：价目表只剩
// 白名单内的 a/b；过滤只发生在本次响应上，其余响应字段原样存在。
func TestGetPricingFiltersByMemberModelAccess(t *testing.T) {
	db := setupPricingModelAccessTestDB(t)
	seedPricingMemberAccessModels(t, db)
	viewer := createMemberModelAccessTestUser(t, db, "pricing-member-access-viewer", common.RoleCommonUser)
	attachMemberModelAccess(t, db, viewer.Id, "pricing-member-access-viewer",
		pricingMemberAccessAllowList(pricingMemberAccessModelOne, pricingMemberAccessModelTwo))

	recorder := performGetPricingRequest(t, viewer.Id)

	byName := pricingByModelName(decodePricingPageResponse(t, recorder).Data.Pricings)
	require.Len(t, byName, 2)
	require.Contains(t, byName, pricingMemberAccessModelOne)
	require.Contains(t, byName, pricingMemberAccessModelTwo)

	// 白名单只收窄 pricings：其余字段与过滤无关，必须原样存在。
	var top map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &top))
	for _, key := range []string{
		"success", "data", "vendors", "group_ratio", "user_model_discounts",
		"usable_group", "supported_endpoint", "auto_groups", "pricing_version",
	} {
		require.Contains(t, top, key, "价目表响应字段 %s 缺失", key)
	}
	data, ok := top["data"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, data, "pricings")
	require.Contains(t, data, "off_peak_window")
}

// TestGetPricingMemberModelAccessAnonymousResponseUnchanged 匿名请求没有登录身份，
// 白名单不参与过滤：价目表与过滤前的可用分组视图逐元素（含顺序）一致——库里存在
// 受限用户也不影响匿名视图。
func TestGetPricingMemberModelAccessAnonymousResponseUnchanged(t *testing.T) {
	db := setupPricingModelAccessTestDB(t)
	seedPricingMemberAccessModels(t, db)
	restricted := createMemberModelAccessTestUser(t, db, "pricing-member-access-anonymous", common.RoleCommonUser)
	attachMemberModelAccess(t, db, restricted.Id, "pricing-member-access-anonymous",
		pricingMemberAccessAllowList(pricingMemberAccessModelOne))

	// 过滤前的匿名视图：可用分组过滤后的共享价目切片原样下发。
	expected := filterPricingByUsableGroups(model.GetPricing(), service.GetAccountUsableGroups(""))
	require.Len(t, expected, 3, "fixture 必须真的提供了价目条目，否则这条回归退化为空对空")

	payload := decodePricingPageResponse(t, performGetPricingRequest(t, 0))

	require.Equal(t, expected, payload.Data.Pricings)
}

// TestGetPricingMemberModelAccessDoesNotPollutePricingCache 按请求过滤不得改写共享
// 价目缓存：受限用户请求过后，缓存仍含被隐藏的模型，元素内容（ModelRatio 等）不变。
func TestGetPricingMemberModelAccessDoesNotPollutePricingCache(t *testing.T) {
	db := setupPricingModelAccessTestDB(t)
	seedPricingMemberAccessModels(t, db)
	viewer := createMemberModelAccessTestUser(t, db, "pricing-member-access-pollution", common.RoleCommonUser)
	attachMemberModelAccess(t, db, viewer.Id, "pricing-member-access-pollution",
		pricingMemberAccessAllowList(pricingMemberAccessModelOne, pricingMemberAccessModelTwo))

	cachedBefore := append([]model.Pricing(nil), model.GetPricing()...)
	require.Len(t, cachedBefore, 3)

	payload := decodePricingPageResponse(t, performGetPricingRequest(t, viewer.Id))
	require.Len(t, payload.Data.Pricings, 2)
	cachedByName := pricingByModelName(cachedBefore)
	for _, item := range payload.Data.Pricings {
		cached, ok := cachedByName[item.ModelName]
		require.True(t, ok, "响应里的模型必须来自价目缓存")
		require.Equal(t, cached.ModelRatio, item.ModelRatio, "响应里的元素必须还是缓存原值")
	}

	cachedAfter := model.GetPricing()
	require.Equal(t, cachedBefore, cachedAfter, "共享价目缓存不能被按请求过滤改写")
	hidden, ok := pricingByModelName(cachedAfter)[pricingMemberAccessModelHidden]
	require.True(t, ok, "被隐藏的模型必须仍在缓存里")
	require.Equal(t, cachedByName[pricingMemberAccessModelHidden].ModelRatio, hidden.ModelRatio)
}
