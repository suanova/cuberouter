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

package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func seedOrganizationUsage(t *testing.T) int64 {
	t.Helper()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{model.LogUsageBackfillDoneOption: "true"}
	t.Cleanup(func() { common.OptionMap = originalOptions })
	db := model.DB
	require.NoError(t, db.AutoMigrate(&model.Model{}, &model.Ability{}))
	ts := time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&[]model.User{
		{Id: 101, Username: "alice", DisplayName: "Alice Chen", AffCode: "alice", Status: common.UserStatusEnabled, Role: common.RoleCommonUser},
		{Id: 102, Username: "bob", AffCode: "bob", Status: common.UserStatusEnabled, Role: common.RoleCommonUser},
	}).Error)
	require.NoError(t, db.Create(&model.Organization{
		Id: 9, Name: "Acme", Slug: "acme", Group: "acme-group", Status: model.OrganizationStatusActive, OwnerUserId: 101,
	}).Error)
	require.NoError(t, db.Create(&[]model.OrganizationMember{
		{OrganizationId: 9, UserId: 101, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive},
		{OrganizationId: 9, UserId: 102, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive},
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "acme-group", Model: "gpt-4o", ChannelId: 1, Enabled: true},
		{Group: "acme-group", Model: "claude", ChannelId: 1, Enabled: true},
		{Group: "acme-group", Model: "wan3.0-video", ChannelId: 1, Enabled: true},
		{Group: "acme-group", Model: "hidden-model", ChannelId: 1, Enabled: false},
	}).Error)
	require.NoError(t, db.Create(&[]model.Model{
		{ModelName: "gpt-4o", Tags: "工具"},
		{ModelName: "claude", Tags: "工具"},
		{ModelName: "wan3.0-video", Tags: "视频生成"},
	}).Error)
	require.NoError(t, db.Create(&[]model.Token{
		{Id: 1, UserId: 101, Name: "team-key", Key: "sk-team", ScopeType: model.TokenScopeOrganization, OrganizationId: 9},
		{Id: 2, UserId: 101, Name: "ops-key", Key: "sk-ops", ScopeType: model.TokenScopeOrganization, OrganizationId: 9, DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
		{Id: 3, UserId: 101, Name: "foreign-key", Key: "sk-foreign", ScopeType: model.TokenScopeOrganization, OrganizationId: 10},
		{Id: 4, UserId: 101, Name: "personal-key", Key: "sk-personal", ScopeType: model.TokenScopePersonal},
	}).Error)
	require.NoError(t, db.Create(&[]model.Log{
		{UserId: 101, Username: "alice", ResponsibleUserId: 101, ResponsibleName: "Alice Chen", Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: ts, BillingAccountType: model.AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, OrganizationName: "Acme", TokenId: 1, TokenName: "team-key", ModelName: "gpt-4o", RequestId: "o1", PromptTokens: 100, CompletionTokens: 20, CacheTokens: 10, Quota: 100},
		{UserId: 101, Username: "alice", ResponsibleUserId: 101, ResponsibleName: "Alice Chen", Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: ts + 30*60, BillingAccountType: model.AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, OrganizationName: "Acme", TokenId: 1, TokenName: "team-key", ModelName: "gpt-4o", RequestId: "o2", PromptTokens: 50, CompletionTokens: 10, Quota: 50},
		{UserId: 102, Username: "bob", ResponsibleUserId: 102, ResponsibleName: "bob", Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: ts, BillingAccountType: model.AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, OrganizationName: "Acme", TokenId: 1, TokenName: "team-key", ModelName: "gpt-4o", RequestId: "o3", PromptTokens: 70, CompletionTokens: 5, Quota: 70},
		{UserId: 101, Username: "alice", ResponsibleUserId: 101, ResponsibleName: "Alice Chen", Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: ts, BillingAccountType: model.AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, OrganizationName: "Acme", TokenId: 2, TokenName: "ops-key", ModelName: "claude", RequestId: "o4", PromptTokens: 10, Quota: 10},
		{UserId: 101, Username: "alice", ResponsibleUserId: 101, ResponsibleName: "Alice Chen", Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceFee, CreatedAt: ts + 60*60, BillingAccountType: model.AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, OrganizationName: "Acme", TokenId: 1, TokenName: "team-key", ModelName: "gpt-4o", RequestId: "o5", Quota: 25},
		{UserId: 101, Username: "alice", ResponsibleUserId: 101, ResponsibleName: "Alice Chen", Type: model.LogTypeRefund, UsageSource: model.LogUsageSourceText, CreatedAt: ts + 90*60, BillingAccountType: model.AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, OrganizationName: "Acme", TokenId: 1, TokenName: "team-key", ModelName: "gpt-4o", RequestId: "o6", Quota: 5},
		{UserId: 101, Username: "alice", ResponsibleUserId: 101, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: ts, BillingAccountType: model.AccountContextTypePersonal, TokenId: 4, TokenName: "personal-key", ModelName: "gpt-4o", RequestId: "p1", Quota: 999},
		{UserId: 103, Username: "carol", ResponsibleUserId: 103, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: ts, BillingAccountType: model.AccountContextTypeOrganization, BillingAccountId: 10, OrganizationId: 10, OrganizationName: "Other", TokenId: 3, TokenName: "foreign-key", ModelName: "gpt-4o", RequestId: "o7", Quota: 999},
	}).Error)
	require.NoError(t, db.Create(&[]model.Task{
		{TaskID: "task-a", UserId: 101, ActorUserId: 101, ResponsibleUserId: 101, ResponsibleName: "Alice Chen", BillingAccountType: model.AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, OrganizationName: "Acme", TokenId: 1, TokenName: "team-key", Status: model.TaskStatusSuccess, Quota: 40, SubmitTime: ts, Properties: model.Properties{OriginModelName: "wan3.0-video"}},
	}).Error)
	require.NoError(t, db.Create(&[]model.Midjourney{
		{UserId: 102, ActorUserId: 102, ResponsibleUserId: 102, ResponsibleName: "bob", BillingAccountType: model.AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, OrganizationName: "Acme", TokenId: 1, TokenName: "team-key", Action: "IMAGINE", Status: "SUCCESS", Quota: 80, SubmitTime: ts * 1000},
	}).Error)
	return ts
}

func TestGetOrganizationUsageAggregatesByDayResponsibleUserAndKey(t *testing.T) {
	setupServiceTestDB(t)
	ts := seedOrganizationUsage(t)

	data, err := GetOrganizationUsage(OrganizationUsageQuery{
		OperatorUserId: 101,
		OrganizationId: 9,
		AccessMode:     OrganizationAccessModeManagement,
		StartTs:        ts - 3600,
		EndTs:          ts + 3*3600,
		Page:           1,
		PageSize:       20,
	})
	require.NoError(t, err)
	assert.True(t, data.BackfillComplete)
	assert.Equal(t, 5, data.Rows.Total)
	assert.Equal(t, 6, data.Totals.RequestCount)
	assert.Equal(t, 370, data.Totals.Quota)
	assert.Equal(t, 230, data.Totals.PromptTokens)
	assert.Equal(t, 35, data.Totals.CompletionTokens)
	assert.Equal(t, 10, data.Totals.CacheTokens)
	assert.Equal(t, []string{"claude", "gpt-4o", "mj_imagine", "wan3.0-video"}, data.FilterOptions.Models)
	assert.Equal(t, []string{"ops-key", "team-key"}, []string{data.FilterOptions.Tokens[0].Name, data.FilterOptions.Tokens[1].Name})
	require.Len(t, data.FilterOptions.Users, 2)
	assert.Equal(t, "Alice Chen", data.FilterOptions.Users[0].Name)
	assert.Equal(t, "bob", data.FilterOptions.Users[1].Name)

	byKey := map[string]modelUsageRowView{}
	for _, row := range data.Rows.Items {
		byKey[row.ModelName+"|"+row.ResponsibleName+"|"+row.TokenName] = modelUsageRowView{
			requests: row.RequestCount,
			quota:    row.Quota,
			typeName: row.Type,
		}
	}
	assert.Equal(t, modelUsageRowView{requests: 2, quota: 170, typeName: string(ModelUsageTypeText)}, byKey["gpt-4o|Alice Chen|team-key"])
	assert.Equal(t, modelUsageRowView{requests: 1, quota: 70, typeName: string(ModelUsageTypeText)}, byKey["gpt-4o|bob|team-key"])
	assert.Equal(t, modelUsageRowView{requests: 1, quota: 10, typeName: string(ModelUsageTypeText)}, byKey["claude|Alice Chen|ops-key"])
	assert.Equal(t, modelUsageRowView{requests: 1, quota: 40, typeName: string(ModelUsageTypeVideo)}, byKey["wan3.0-video|Alice Chen|team-key"])
	assert.Equal(t, modelUsageRowView{requests: 1, quota: 80, typeName: string(ModelUsageTypeImage)}, byKey["mj_imagine|bob|team-key"])

	bobOnly, err := GetOrganizationUsage(OrganizationUsageQuery{
		OperatorUserId:    101,
		OrganizationId:    9,
		AccessMode:        OrganizationAccessModeManagement,
		StartTs:           ts - 3600,
		EndTs:             ts + 3*3600,
		ResponsibleUserId: 102,
		Page:              1,
		PageSize:          20,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, bobOnly.Rows.Total)
	assert.Equal(t, 150, bobOnly.Totals.Quota)
	// Filtering by one member must not remove the other members from the
	// dropdown that produced the filter.
	assert.Len(t, bobOnly.FilterOptions.Users, 2)

	tokenOnly, err := GetOrganizationUsage(OrganizationUsageQuery{
		OperatorUserId: 101,
		OrganizationId: 9,
		AccessMode:     OrganizationAccessModeManagement,
		StartTs:        ts - 3600,
		EndTs:          ts + 3*3600,
		TokenId:        2,
		Page:           1,
		PageSize:       20,
	})
	require.NoError(t, err)
	require.Len(t, tokenOnly.Rows.Items, 1)
	assert.Equal(t, "claude", tokenOnly.Rows.Items[0].ModelName)

	modelOnly, err := GetOrganizationUsage(OrganizationUsageQuery{
		OperatorUserId: 101,
		OrganizationId: 9,
		AccessMode:     OrganizationAccessModeManagement,
		StartTs:        ts - 3600,
		EndTs:          ts + 3*3600,
		ModelName:      "mj_imagine",
		Page:           1,
		PageSize:       20,
	})
	require.NoError(t, err)
	require.Len(t, modelOnly.Rows.Items, 1)
	assert.Equal(t, "mj_imagine", modelOnly.Rows.Items[0].ModelName)

	prefixOnly, err := GetOrganizationUsage(OrganizationUsageQuery{
		OperatorUserId: 101,
		OrganizationId: 9,
		AccessMode:     OrganizationAccessModeManagement,
		StartTs:        ts - 3600,
		EndTs:          ts + 3*3600,
		ModelName:      "gpt",
		Page:           1,
		PageSize:       20,
	})
	require.NoError(t, err)
	assert.Zero(t, prefixOnly.Rows.Total)
}

// modelUsageRowView keeps the three assertions above readable without repeating
// the full DTO field list.
type modelUsageRowView struct {
	requests int
	quota    int
	typeName string
}

func TestHydrateOrganizationUsageResponsibleNamesFallsBackToID(t *testing.T) {
	setupServiceTestDB(t)

	rows := []usageRow{{ResponsibleUserID: 999}}
	hydrateOrganizationUsageResponsibleNames(rows)
	assert.Equal(t, "#999", rows[0].ResponsibleName)
}

func TestGetOrganizationUsagePinsMembersWithoutWideData(t *testing.T) {
	setupServiceTestDB(t)
	ts := seedOrganizationUsage(t)

	data, err := GetOrganizationUsage(OrganizationUsageQuery{
		OperatorUserId: 102,
		OrganizationId: 9,
		AccessMode:     OrganizationAccessModeWorkspace,
		StartTs:        ts - 3600,
		EndTs:          ts + 3*3600,
		Page:           1,
		PageSize:       20,
	})
	require.NoError(t, err)
	require.Len(t, data.Rows.Items, 2)
	for _, row := range data.Rows.Items {
		assert.Equal(t, 102, row.ResponsibleUserId)
	}
	require.Len(t, data.FilterOptions.Users, 1)
	assert.Equal(t, 102, data.FilterOptions.Users[0].Id)
}

func TestExportOrganizationUsageGroupsByHourAndKeepsResponsibleUser(t *testing.T) {
	setupServiceTestDB(t)
	ts := seedOrganizationUsage(t)

	data, err := ExportOrganizationUsage(OrganizationUsageQuery{
		OperatorUserId: 101,
		OrganizationId: 9,
		AccessMode:     OrganizationAccessModeManagement,
		StartTs:        ts - 3600,
		EndTs:          ts + 3*3600,
	}, SelfUsageGranularityHour)
	require.NoError(t, err)
	assert.Equal(t, SelfUsageGranularityHour, data.Granularity)
	assert.Equal(t, 6, data.Total)
	require.Len(t, data.Rows, 6)
	dates := map[string]bool{}
	for _, row := range data.Rows {
		dates[row.Date] = true
		assert.NotZero(t, row.ResponsibleUserId)
	}
	assert.True(t, dates["2026-11-03 10:00"])
	assert.True(t, dates["2026-11-03 11:00"])
}
