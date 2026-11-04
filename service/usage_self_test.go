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
	"math"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestOrganizationRemainingQuota(t *testing.T) {
	org := &model.Organization{Quota: 1000, UsedQuota: 400, OwnerUserId: 7}
	assert.Equal(t, 600, organizationRemainingQuota(org, &model.OrganizationMember{QuotaLimit: 0, UsedQuota: 0}, true))
	assert.Equal(t, 600, organizationRemainingQuota(org, &model.OrganizationMember{QuotaLimit: 0, UsedQuota: 0}, false))
	assert.Equal(t, 300, organizationRemainingQuota(org, &model.OrganizationMember{QuotaLimit: 500, UsedQuota: 200}, false))
	assert.Equal(t, 50, organizationRemainingQuota(org, &model.OrganizationMember{QuotaLimit: 900, UsedQuota: 850}, false))
	assert.Equal(t, 0, organizationRemainingQuota(org, &model.OrganizationMember{QuotaLimit: 100, UsedQuota: 200}, false))
}

func TestMergeSelfUsageRowsKeepsCacheSeparateAndTotalsAllRows(t *testing.T) {
	rows := mergeSelfUsageRows(
		[]model.SelfUsageLogRow{{
			DayKey: 0, BillingAccountType: "personal", TokenID: 1, TokenName: "k", ModelName: "gpt-4o",
			RequestCount: 2, PromptTokens: 100, CompletionTokens: 40, CacheTokens: 30, Quota: 10,
		}},
		[]model.SelfUsageAsyncRow{{
			DayKey: 0, BillingAccountType: "personal", TokenID: 1, TokenName: "k", ModelName: "wan3.0-video",
			RequestCount: 1, SuccessCount: 1, Quota: 20,
		}},
		nil,
		&ModelTypeResolver{byName: map[string]ModelUsageType{"gpt-4o": ModelUsageTypeText, "wan3.0-video": ModelUsageTypeVideo}},
	)
	assert.Len(t, rows, 2)
	totalInput, totalCache, totalQuota, totalRequests := 0, 0, 0, 0
	for _, row := range rows {
		totalInput += row.PromptTokens
		totalCache += row.CacheTokens
		totalQuota += row.Quota
		totalRequests += row.RequestCount
	}
	assert.Equal(t, 100, totalInput)
	assert.Equal(t, 30, totalCache)
	assert.Equal(t, 30, totalQuota)
	assert.Equal(t, 3, totalRequests)
}

func newSelfUsageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	origDB, origLog := model.DB, model.LOG_DB
	origOptions := common.OptionMap
	origRedis := common.RedisEnabled
	model.DB, model.LOG_DB = db, db
	common.OptionMap = map[string]string{model.LogUsageBackfillDoneOption: "true"}
	common.RedisEnabled = false
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Log{},
		&model.Task{},
		&model.Midjourney{},
		&model.Token{},
		&model.Ability{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.Model{},
	))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = origDB, origLog
		common.OptionMap = origOptions
		common.RedisEnabled = origRedis
	})
	return db
}

func TestGetSelfUsageRejectsForeignRows(t *testing.T) {
	db := newSelfUsageTestDB(t)
	ts := time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&model.User{Id: 7, Username: "u7", Quota: 1000}).Error)
	require.NoError(t, db.Create(&model.Model{ModelName: "gpt-4o", Tags: "工具"}).Error)
	require.NoError(t, db.Create(&model.Model{ModelName: "wan3.0-video", Tags: "视频生成"}).Error)
	require.NoError(t, db.Create(&model.Organization{Id: 9, Name: "Acme", Slug: "acme", OwnerUserId: 99, Quota: 1000, UsedQuota: 400}).Error)
	require.NoError(t, db.Create(&model.OrganizationMember{
		OrganizationId: 9, UserId: 7, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive,
		QuotaLimit: 500, UsedQuota: 200,
	}).Error)
	require.NoError(t, db.Create(&[]model.Log{
		{UserId: 7, ActorUserId: 7, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: ts, ModelName: "gpt-4o", BillingAccountType: "personal", RequestId: "r1", Quota: 100},
		{UserId: 0, ActorUserId: 0, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: ts, ModelName: "gpt-4o", BillingAccountType: "personal", RequestId: "r2", Quota: 50},
		{UserId: 8, ActorUserId: 8, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: ts, ModelName: "gpt-4o", BillingAccountType: "personal", RequestId: "r3", Quota: 70},
		{UserId: 7, ActorUserId: 7, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: ts, ModelName: "gpt-4o", BillingAccountType: "organization", OrganizationId: 9, OrganizationName: "Acme", RequestId: "r4", Quota: 30},
	}).Error)
	require.NoError(t, db.Create(&[]model.Task{
		{TaskID: "t7", UserId: 7, ActorUserId: 7, BillingAccountType: "personal", Status: model.TaskStatusSuccess, Quota: 40, SubmitTime: ts, Properties: model.Properties{OriginModelName: "wan3.0-video"}},
		{TaskID: "t8", UserId: 8, ActorUserId: 8, BillingAccountType: "personal", Status: model.TaskStatusSuccess, Quota: 99, SubmitTime: ts, Properties: model.Properties{OriginModelName: "wan3.0-video"}},
	}).Error)

	got, err := GetSelfUsage(SelfUsageQuery{
		ActorUserId:            7,
		StartTs:                ts - 3600,
		EndTs:                  ts + 3600,
		OverviewOrganizationID: 9,
		Page:                   1,
		PageSize:               20,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, got.Overview.Personal.RequestCount)
	assert.Equal(t, 140, got.Overview.Personal.Quota)
	require.NotNil(t, got.Overview.Organization)
	assert.Equal(t, 1, got.Overview.Organization.RequestCount)
	assert.Equal(t, 30, got.Overview.Organization.Quota)
	assert.Equal(t, 300, got.Overview.Organization.RemainingQuota)
	assert.Equal(t, 3, got.Rows.Total)
	assert.Equal(t, 3, got.Totals.RequestCount)
	assert.Equal(t, 170, got.Totals.Quota)
}

func TestGetSelfUsageTotalsCoverAllPages(t *testing.T) {
	db := newSelfUsageTestDB(t)
	ts := time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&model.User{Id: 7, Username: "u7", Quota: 0}).Error)
	require.NoError(t, db.Create(&[]model.Log{
		{UserId: 7, ActorUserId: 7, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: ts, ModelName: "model-a", BillingAccountType: "personal", RequestId: "p1", Quota: 10},
		{UserId: 7, ActorUserId: 7, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: ts, ModelName: "model-b", BillingAccountType: "personal", RequestId: "p2", Quota: 10},
		{UserId: 7, ActorUserId: 7, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: ts, ModelName: "model-c", BillingAccountType: "personal", RequestId: "p3", Quota: 10},
	}).Error)

	got, err := GetSelfUsage(SelfUsageQuery{
		ActorUserId: 7,
		StartTs:     ts - 3600,
		EndTs:       ts + 3600,
		Page:        1,
		PageSize:    1,
	})
	require.NoError(t, err)
	assert.Len(t, got.Rows.Items, 1)
	assert.Equal(t, 3, got.Rows.Total)
	assert.Equal(t, 3, got.Totals.RequestCount)
	assert.Equal(t, 30, got.Totals.Quota)
}

func TestSortSelfUsageRowsComparesFullKey(t *testing.T) {
	rows := []usageRow{
		{DayKey: 1, Type: ModelUsageTypeText, BillingAccountType: "organization", OrganizationID: 2, OrganizationName: "Beta", ModelName: "gpt", TokenID: 1},
		{DayKey: 1, Type: ModelUsageTypeText, BillingAccountType: "organization", OrganizationID: 1, OrganizationName: "Alpha", ModelName: "gpt", TokenID: 1},
	}
	sortSelfUsageRows(rows)
	assert.Equal(t, 1, rows[0].OrganizationID)
	assert.Equal(t, "Alpha", rows[0].OrganizationName)
}

func TestGetSelfUsageReturnsVisibleModelsAndUsableTokens(t *testing.T) {
	setupServiceTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Model{}, &model.Ability{}))

	const actor = 707
	require.NoError(t, model.DB.Create(&model.User{Id: actor, Username: "u707", Group: "default"}).Error)
	require.NoError(t, model.DB.Create(&[]model.Ability{
		{Group: "default", Model: "gpt-4o", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "wan3.0-video", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "hidden-model", ChannelId: 1, Enabled: false},
		{Group: "acme-group", Model: "org-only-model", ChannelId: 1, Enabled: true},
	}).Error)
	require.NoError(t, model.DB.Create(&model.Organization{Id: 909, Name: "Acme", Slug: "acme", Group: "acme-group", OwnerUserId: 9099}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: 909, UserId: actor, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive,
	}).Error)
	require.NoError(t, model.DB.Create(&[]model.Token{
		{UserId: actor, Name: "personal-key", Key: "sk-personal", ScopeType: model.TokenScopePersonal},
		{UserId: actor, Name: "org-public", Key: "sk-orgpub", ScopeType: model.TokenScopeOrganization, OrganizationId: 909, Visibility: model.TokenVisibilityPublic},
		{UserId: actor, Name: "org-mine", Key: "sk-orgmine", ScopeType: model.TokenScopeOrganization, OrganizationId: 909, Visibility: model.TokenVisibilityPrivate, ResponsibleUserId: actor},
		{UserId: 708, Name: "org-other", Key: "sk-orgother", ScopeType: model.TokenScopeOrganization, OrganizationId: 909, Visibility: model.TokenVisibilityPrivate, ResponsibleUserId: 708},
		{UserId: 709, Name: "foreign-org-key", Key: "sk-foreign", ScopeType: model.TokenScopeOrganization, OrganizationId: 910, Visibility: model.TokenVisibilityPublic},
	}).Error)

	got, err := GetSelfUsage(SelfUsageQuery{
		ActorUserId: actor,
		StartTs:     time.Now().Add(-time.Hour).Unix(),
		EndTs:       time.Now().Add(time.Hour).Unix(),
		Page:        1,
		PageSize:    20,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"gpt-4o", "org-only-model", "wan3.0-video"}, got.FilterOptions.Models)
	assert.Equal(t, []dto.SelfUsageTokenOption{
		{Name: "org-mine", OrganizationId: 909, OrganizationName: "Acme"},
		{Name: "org-public", OrganizationId: 909, OrganizationName: "Acme"},
		{Name: "personal-key"},
	}, got.FilterOptions.Tokens)
}

func TestGetSelfUsageClampsHugePage(t *testing.T) {
	db := newSelfUsageTestDB(t)
	ts := time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&model.User{Id: 7, Username: "u7"}).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId: 7, ActorUserId: 7, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText,
		CreatedAt: ts, ModelName: "gpt-4o", BillingAccountType: "personal", RequestId: "p1", Quota: 10,
	}).Error)

	got, err := GetSelfUsage(SelfUsageQuery{
		ActorUserId: 7,
		StartTs:     ts - 3600,
		EndTs:       ts + 3600,
		Page:        math.MaxInt,
		PageSize:    100,
	})
	require.NoError(t, err)
	assert.Empty(t, got.Rows.Items)
	assert.Equal(t, 1, got.Rows.Total)
}
