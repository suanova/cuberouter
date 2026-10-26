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

	"github.com/glebarez/sqlite"
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
