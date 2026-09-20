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

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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

// legacyQuotaDataTableDDL 是移植之前 quota_data 的建表语句：只有分析维度，没有作用域
// 与账单归属列。升级时必须能从这个形状的原表接上去，所以这里照着老结构先建出来。
const legacyQuotaDataTableDDL = "CREATE TABLE `quota_data` (" +
	"`id` integer PRIMARY KEY AUTOINCREMENT," +
	"`user_id` integer, `username` varchar(64) DEFAULT '', `model_name` varchar(64) DEFAULT ''," +
	"`created_at` bigint, `use_group` varchar(64) DEFAULT '', `token_id` integer DEFAULT 0," +
	"`channel_id` integer DEFAULT 0, `node_name` varchar(64) DEFAULT ''," +
	"`token_used` integer DEFAULT 0, `count` integer DEFAULT 0, `quota` integer DEFAULT 0)"

// TestEnsureQuotaDataAccountContextIndexMergesLegacyDuplicateRows 覆盖真实升级里最危险
// 的一步：老库上带着重复行时，唯一索引建不出来，而 ensureOrganizationBillingIndexes 的
// 错误会拦住 master 启动——上线即不可用。老写路径是「先查后插」，多节点同时刷同一个
// 时间桶就会各插一行，所以这不是理论情况。合并必须保住各节点用量的总和。
func TestEnsureQuotaDataAccountContextIndexMergesLegacyDuplicateRows(t *testing.T) {
	previousDB, previousLogDB := DB, LOG_DB
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/legacy-quota-data.db"), &gorm.Config{})
	require.NoError(t, err)
	DB, LOG_DB = db, db
	t.Cleanup(func() { DB, LOG_DB = previousDB, previousLogDB })
	require.NoError(t, db.Exec(legacyQuotaDataTableDDL).Error)

	// 同一个时间桶被两个节点各插了一行，第三行是不同的 token，不该被并进来。
	require.NoError(t, db.Exec("INSERT INTO quota_data (user_id, username, model_name, created_at, use_group, token_id, channel_id, node_name, token_used, count, quota) VALUES (7, 'legacy', 'gpt-legacy', 3600, 'default', 11, 22, 'node-a', 30, 1, 100)").Error)
	require.NoError(t, db.Exec("INSERT INTO quota_data (user_id, username, model_name, created_at, use_group, token_id, channel_id, node_name, token_used, count, quota) VALUES (7, 'legacy', 'gpt-legacy', 3600, 'default', 11, 22, 'node-a', 40, 2, 200)").Error)
	require.NoError(t, db.Exec("INSERT INTO quota_data (user_id, username, model_name, created_at, use_group, token_id, channel_id, node_name, token_used, count, quota) VALUES (7, 'legacy', 'gpt-legacy', 3600, 'default', 12, 22, 'node-a', 5, 1, 50)").Error)

	// 升级顺序与 model/main.go 一致：先补作用域列，再建唯一索引。
	require.NoError(t, db.AutoMigrate(&QuotaData{}))
	require.NoError(t, prepareOrganizationScopeMigration(db))
	require.NoError(t, ensureQuotaDataAccountContextIndex(db))
	require.True(t, db.Migrator().HasIndex("quota_data", quotaDataAccountContextIndexName))

	// 看板按 user/模型/时间聚合，两个 token 的用量都算进来：合并只动重复行，
	// 不同 token 的桶各自独立。
	group, err := GetQuotaDataByUserId(7, 0, 7200)
	require.NoError(t, err)
	require.Len(t, group, 1)
	require.Equal(t, 4, group[0].Count)
	require.Equal(t, 350, group[0].Quota)
	require.Equal(t, 75, group[0].TokenUsed)

	var rows []QuotaData
	require.NoError(t, db.Order("token_id").Find(&rows).Error)
	require.Len(t, rows, 2)
	require.Equal(t, 300, rows[0].Quota)
	require.Equal(t, 50, rows[1].Quota)

	// 合并过的库再跑一次必须是无操作，并且索引真的开始拦重复写入了。
	require.NoError(t, ensureQuotaDataAccountContextIndex(db))
	require.Error(t, db.Create(&QuotaData{
		UserID: 7, Username: "legacy", ModelName: "gpt-legacy", CreatedAt: 3600,
		UseGroup: "default", TokenID: 11, ChannelID: 22, NodeName: "node-a",
		ScopeType: AccountContextTypePersonal, ScopeId: 7,
		BillingAccountType: AccountContextTypePersonal, BillingAccountId: 7,
		ResponsibleUserId: 7,
	}).Error)
}

// TestQuotaDataDuplicateMergePredicateCoversIndexColumns 守住合并用的 WHERE 与索引列
// 不能各写各的：漏一列就可能把本该保留的两行并成一行，或者让合并循环匹配不到自己
// 选出来的组。列名清单是唯一来源，这里逐列核对。
func TestQuotaDataDuplicateMergePredicateCoversIndexColumns(t *testing.T) {
	conditions := quotaDataGroupConditions()
	for _, column := range quotaDataAccountContextColumnNames {
		require.Contains(t, conditions, column+" = ?", "合并条件缺少索引列 %s", column)
	}
	require.Len(t, quotaDataGroupArguments(quotaDataAccountContextKey{}), len(quotaDataAccountContextColumnNames),
		"占位符与绑定值必须一一对应")
}
