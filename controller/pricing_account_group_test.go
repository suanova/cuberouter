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
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupPricingAccountGroupTestDB 只需要账户上下文那条链路用到的表：价目表分组
// 解析会读用户、组织、成员关系与失效记录。表给多了反而掩盖依赖。
func setupPricingAccountGroupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Organization{}, &model.OrganizationMember{},
		&model.UserAccountContext{}, &model.OrganizationDisableRecord{},
	))

	t.Cleanup(func() {
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

func createPricingTestUser(t *testing.T, db *gorm.DB, username string, group string) model.User {
	t.Helper()
	user := model.User{
		Username: username, Password: "password", AffCode: username,
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: group,
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func createPricingTestOrganization(t *testing.T, db *gorm.DB, ownerId int, slug string, group string) model.Organization {
	t.Helper()
	organization := model.Organization{
		Name:      "Pricing " + slug,
		Slug:      slug,
		Group:     group,
		Status:    model.OrganizationStatusActive,
		CreatedBy: ownerId,
	}
	require.NoError(t, db.Create(&organization).Error)
	require.NoError(t, db.Create(&model.OrganizationMember{
		OrganizationId: organization.Id, UserId: ownerId,
		Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive,
	}).Error)
	return organization
}

// TestPricingAccountGroupUsesOrganizationGroupInOrganizationContext 覆盖组织上下文下的
// 价目表分组：组织密钥按组织的分组倍率计费，价目表也必须按组织的分组过滤，
// 否则用户在组织里看到的是自己个人分组的价钱。
func TestPricingAccountGroupUsesOrganizationGroupInOrganizationContext(t *testing.T) {
	db := setupPricingAccountGroupTestDB(t)
	user := createPricingTestUser(t, db, "pricing-org-context", "personal-group")
	organization := createPricingTestOrganization(t, db, user.Id, "pricing-org-context", "org-group")
	require.NoError(t, db.Create(&model.UserAccountContext{
		UserId: user.Id, ContextType: model.AccountContextTypeOrganization, ContextId: organization.Id,
	}).Error)

	require.Equal(t, "org-group", pricingAccountGroup(user.Id))
}

// TestPricingAccountGroupFallsBackToPersonalGroup 覆盖三种退回个人的情况：
// 没有上下文记录、上下文是个人、以及上下文指向的组织已经不可用（被解散/退出）。
// 退回个人分组比让人看不到价目表好。
func TestPricingAccountGroupFallsBackToPersonalGroup(t *testing.T) {
	db := setupPricingAccountGroupTestDB(t)
	user := createPricingTestUser(t, db, "pricing-fallback", "personal-group")

	require.Equal(t, "personal-group", pricingAccountGroup(user.Id), "no context row defaults to personal")

	require.NoError(t, db.Create(&model.UserAccountContext{
		UserId: user.Id, ContextType: model.AccountContextTypePersonal, ContextId: user.Id,
	}).Error)
	require.Equal(t, "personal-group", pricingAccountGroup(user.Id))

	organization := createPricingTestOrganization(t, db, user.Id, "pricing-dissolved", "org-group")
	require.NoError(t, db.Model(&model.Organization{}).Where("id = ?", organization.Id).
		Update("status", model.OrganizationStatusDissolved).Error)
	require.NoError(t, db.Model(&model.UserAccountContext{}).Where("user_id = ?", user.Id).
		Updates(map[string]any{"context_type": model.AccountContextTypeOrganization, "context_id": organization.Id}).Error)

	require.Equal(t, "personal-group", pricingAccountGroup(user.Id), "a dissolved organization falls back to personal")
}

// TestPricingAccountGroupNormalizesEmptyOrganizationGroup 组织分组为空时按默认分组算，
// 与 NormalizeOrganizationGroup 一致：不能返回空串，那会让 GetAccountUsableGroups
// 落到「没有任何可用分组」，价目表直接空掉。
func TestPricingAccountGroupNormalizesEmptyOrganizationGroup(t *testing.T) {
	db := setupPricingAccountGroupTestDB(t)
	user := createPricingTestUser(t, db, "pricing-empty-org-group", "personal-group")
	organization := createPricingTestOrganization(t, db, user.Id, "pricing-empty-org-group", "")
	require.NoError(t, db.Create(&model.UserAccountContext{
		UserId: user.Id, ContextType: model.AccountContextTypeOrganization, ContextId: organization.Id,
	}).Error)

	require.Equal(t, service.DefaultOrganizationGroup, pricingAccountGroup(user.Id))
}

// TestFilterPricingByUsableGroupsKeepsAllGroup 守住这次移植没有改动的过滤语义：
// 标了 all 的模型对任何分组可见，交集为空时返回空集而不是 nil。
func TestFilterPricingByUsableGroupsKeepsAllGroup(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "all-group-model", EnableGroup: []string{"all"}},
		{ModelName: "matching-model", EnableGroup: []string{"vip", "default"}},
		{ModelName: "unavailable-model", EnableGroup: []string{"internal"}},
	}

	filtered := filterPricingByUsableGroups(pricing, map[string]string{"default": "default"})
	require.Len(t, filtered, 2)
	require.Equal(t, "all-group-model", filtered[0].ModelName)
	require.Equal(t, "matching-model", filtered[1].ModelName)

	require.Empty(t, filterPricingByUsableGroups(pricing, map[string]string{}))
	require.Empty(t, filterPricingByUsableGroups([]model.Pricing{}, map[string]string{"default": "default"}))
}

// TestFilterPricingByGroupKeepsOnlyOwnGroup 价目表只描述本人分组能用的模型：
// 其他分组的模型不出现，标了 all 的模型对任何分组可见，空分组只保留公共条目。
func TestFilterPricingByGroupKeepsOnlyOwnGroup(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "all-group-model", EnableGroup: []string{"all"}},
		{ModelName: "yeebo-model", EnableGroup: []string{"yeebo"}},
		{ModelName: "default-model", EnableGroup: []string{"default", "vip"}},
	}

	filtered := filterPricingByGroup(pricing, "yeebo")
	require.Len(t, filtered, 2)
	require.Equal(t, "all-group-model", filtered[0].ModelName)
	require.Equal(t, "yeebo-model", filtered[1].ModelName)

	assert.Equal(t, pricing[:1], filterPricingByGroup(pricing, ""), "解析不出本人分组时只保留公共条目")
	onlyAll := filterPricingByGroup(pricing, "internal")
	require.Len(t, onlyAll, 1)
	require.Equal(t, "all-group-model", onlyAll[0].ModelName)
}

// TestCollectPricingGroupsExpandsAdminView 管理员的可用分组来自价目表本身：
// all 是特殊标记不算业务分组，已配置描述的分组沿用配置，未配置的退回分组名。
func TestCollectPricingGroupsExpandsAdminView(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "all-group-model", EnableGroup: []string{"all"}},
		{ModelName: "default-model", EnableGroup: []string{"default"}},
		{ModelName: "yeebo-model", EnableGroup: []string{"yeebo", "default"}},
	}

	groups := collectPricingGroups(pricing)
	require.Len(t, groups, 2)
	require.NotEmpty(t, groups["default"])
	require.Equal(t, "yeebo", groups["yeebo"], "未配置描述时退回分组名")
	require.NotContains(t, groups, "all")

	require.Empty(t, collectPricingGroups(nil))
}

// A successful personal-context lookup must not turn a later user-cache failure
// into anonymous pricing access.
func TestGetPricingRestrictsModelsWhenFinalUserLookupFails(t *testing.T) {
	db := setupPricingModelAccessTestDB(t)
	user := createPricingTestUser(t, db, "pricing-lookup-failure", "yeebo")
	previousGroups := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousGroups))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","yeebo":"Yeebo"}`))
	for i, group := range []string{"default", "yeebo", "all"} {
		require.NoError(t, db.Create(&model.Ability{
			Group: group, Model: "zz-pricing-" + group, ChannelId: i + 1, Enabled: true,
		}).Error)
	}
	require.Len(t, model.GetPricing(), 3)

	userLookups := 0
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("pricing_second_user_lookup_failure", func(tx *gorm.DB) {
		if tx.Statement.Table != "users" {
			return
		}
		userLookups++
		if userLookups == 2 {
			tx.AddError(errors.New("user lookup unavailable"))
		}
	}))
	t.Cleanup(func() {
		require.NoError(t, db.Callback().Query().Remove("pricing_second_user_lookup_failure"))
	})

	payload := decodePricingPageResponse(t, performGetPricingRequest(t, user.Id))

	require.GreaterOrEqual(t, userLookups, 2, "the final group lookup must have failed")
	require.Len(t, payload.Data.Pricings, 1)
	assert.Equal(t, "zz-pricing-all", payload.Data.Pricings[0].ModelName)
}
