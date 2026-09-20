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
package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

// TestPersonalQuotaDataQueriesFilterOutOrganizationBilling 锁定个人看板的数据边界：
// 组织把这个人记为责任人，行的 user_id 就是他，只有账单归属列分得清这笔钱是谁出的。
func TestPersonalQuotaDataQueriesFilterOutOrganizationBilling(t *testing.T) {
	setupModelTestDB(t)
	common.DataExportEnabled = true
	common.LogConsumeEnabled = true
	t.Cleanup(func() {
		common.DataExportEnabled = false
		common.LogConsumeEnabled = false
		CacheQuotaData = make(map[string]*QuotaData)
	})

	user := User{Username: "quota-personal-user", Password: "password", AffCode: "quota-personal-user"}
	require.NoError(t, DB.Create(&user).Error)
	LogQuotaData(QuotaDataLogParams{
		UserID:             user.Id,
		Username:           user.Username,
		ModelName:          "personal-model",
		CreatedAt:          time.Now().Unix(),
		Quota:              10,
		TokenUsed:          10,
		ScopeType:          AccountContextTypePersonal,
		ScopeId:            user.Id,
		BillingAccountType: AccountContextTypePersonal,
		BillingAccountId:   user.Id,
	})
	LogQuotaData(QuotaDataLogParams{
		UserID:             user.Id,
		Username:           user.Username,
		ModelName:          "organization-model",
		CreatedAt:          time.Now().Unix(),
		Quota:              20,
		TokenUsed:          20,
		ScopeType:          AccountContextTypeOrganization,
		ScopeId:            99,
		BillingAccountType: AccountContextTypeOrganization,
		BillingAccountId:   99,
		OrganizationId:     99,
		ResponsibleUserId:  user.Id,
	})
	SaveQuotaDataCache()

	data, err := GetQuotaDataByUserId(user.Id, 0, common.GetTimestamp()+3600)
	require.NoError(t, err)
	// 只断言「组织那行没进来」。个人看板是按 user_id/username/model_name/created_at
	// 聚合的，作用域列不在 SELECT 里（也不该在：严格 SQL 下它们不在 GROUP BY 中），
	// 过滤本身已经保证结果全是个人账单。
	require.Len(t, data, 1)
	require.Equal(t, "personal-model", data[0].ModelName)

	// 管理端的按用户名查询走同一条边界：这份数据回答的是「这个人自己花了多少」，
	// 组织那 20 是组织出的，不该算进去。
	byName, err := GetQuotaDataByUsername(user.Username, 0, common.GetTimestamp()+3600)
	require.NoError(t, err)
	require.Len(t, byName, 1)
	require.Equal(t, "personal-model", byName[0].ModelName)
}

func TestOrganizationQuotaDataQueriesUseOrganizationBillingAccount(t *testing.T) {
	setupModelTestDB(t)
	now := common.GetTimestamp()

	require.NoError(t, DB.Create(&QuotaData{UserID: 10, Username: "member", ModelName: "personal", CreatedAt: now, Count: 1, Quota: 10, BillingAccountType: AccountContextTypePersonal, BillingAccountId: 10, ScopeType: AccountContextTypePersonal, ScopeId: 10}).Error)
	require.NoError(t, DB.Create(&QuotaData{UserID: 10, Username: "member", ModelName: "org", CreatedAt: now, Count: 2, Quota: 20, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 99, ScopeType: AccountContextTypeOrganization, ScopeId: 99, OrganizationId: 99, ResponsibleUserId: 10}).Error)

	personal, err := GetQuotaDataByUserId(10, 0, now+1)
	require.NoError(t, err)
	require.Len(t, personal, 1)
	require.Equal(t, "personal", personal[0].ModelName)

	organization, err := GetQuotaDataByOrganizationId(99, 0, now+1)
	require.NoError(t, err)
	require.Len(t, organization, 1)
	require.Equal(t, "org", organization[0].ModelName)
}

// TestSaveQuotaDataCacheUpsertsByAccountContextUniqueKey 覆盖多节点刷缓存那一步。
//
// 同一身份的行连续两次落库必须累加成一行，而不是各插一行——旧实现是「先查再插」，
// 两个节点会同时查到「不存在」然后各插一行，同一小时的用量直接翻倍。
func TestSaveQuotaDataCacheUpsertsByAccountContextUniqueKey(t *testing.T) {
	setupModelTestDB(t)
	now := common.GetTimestamp()

	LogQuotaData(QuotaDataLogParams{UserID: 10, Username: "member", ModelName: "org-model", CreatedAt: now, Quota: 10, TokenUsed: 5, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 99, ScopeType: AccountContextTypeOrganization, ScopeId: 99, OrganizationId: 99, ResponsibleUserId: 10})
	SaveQuotaDataCache()
	LogQuotaData(QuotaDataLogParams{UserID: 10, Username: "member", ModelName: "org-model", CreatedAt: now, Quota: 20, TokenUsed: 15, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 99, ScopeType: AccountContextTypeOrganization, ScopeId: 99, OrganizationId: 99, ResponsibleUserId: 10})
	SaveQuotaDataCache()

	organization, err := GetQuotaDataByOrganizationId(99, 0, now+3600)
	require.NoError(t, err)
	require.Len(t, organization, 1)
	require.Equal(t, 2, organization[0].Count)
	require.Equal(t, 30, organization[0].Quota)
	require.Equal(t, 20, organization[0].TokenUsed)
}

func TestQuotaDataAccountContextUniqueIndexRejectsDuplicateRows(t *testing.T) {
	setupModelTestDB(t)
	now := common.GetTimestamp()
	first := QuotaData{UserID: 10, Username: "member", ModelName: "org-model", CreatedAt: now, Count: 1, Quota: 10, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 99, ScopeType: AccountContextTypeOrganization, ScopeId: 99, OrganizationId: 99, ResponsibleUserId: 10}
	second := first
	second.Count = 2
	second.Quota = 20

	require.NoError(t, DB.Create(&first).Error)
	require.Error(t, DB.Create(&second).Error)
}
