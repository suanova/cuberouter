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
)

func seedSiteUsage(t *testing.T) int64 {
	t.Helper()
	db := newSelfUsageTestDB(t)
	ts := time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&[]model.User{
		{Id: 101, Username: "alice", DisplayName: "Alice Chen", AffCode: "alice-site", Group: "default", Status: common.UserStatusEnabled, Role: common.RoleCommonUser},
		{Id: 102, Username: "bob", AffCode: "bob-site", Group: "default", Status: common.UserStatusEnabled, Role: common.RoleCommonUser},
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "gpt-4o", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "claude", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "wan3.0-video", ChannelId: 1, Enabled: true},
		{Group: "acme-group", Model: "org-only-model", ChannelId: 1, Enabled: true},
	}).Error)
	require.NoError(t, db.Create(&[]model.Model{
		{ModelName: "gpt-4o", Tags: "工具"},
		{ModelName: "claude", Tags: "工具"},
		{ModelName: "wan3.0-video", Tags: "视频生成"},
	}).Error)
	require.NoError(t, db.Create(&[]model.Log{
		{UserId: 101, ActorUserId: 101, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: ts, ModelName: "gpt-4o", TokenId: 1, TokenName: "alice-key", BillingAccountType: "personal", RequestId: "s1", PromptTokens: 100, CompletionTokens: 20, CacheTokens: 10, Quota: 100},
		{UserId: 101, ActorUserId: 101, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: ts, ModelName: "gpt-4o", TokenId: 1, TokenName: "alice-key", BillingAccountType: "personal", RequestId: "s2", PromptTokens: 50, CompletionTokens: 10, CacheTokens: 5, Quota: 50},
		{UserId: 102, ActorUserId: 102, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: ts, ModelName: "claude", TokenId: 2, TokenName: "bob-key", BillingAccountType: "personal", RequestId: "s3", PromptTokens: 70, CompletionTokens: 7, Quota: 70},
		{UserId: 101, ActorUserId: 101, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: ts, ModelName: "gpt-4o", TokenId: 3, TokenName: "team-key", BillingAccountType: "organization", OrganizationId: 9, OrganizationName: "Acme", RequestId: "s4", PromptTokens: 30, CompletionTokens: 3, Quota: 30},
		{UserId: 101, ActorUserId: 101, Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceFee, CreatedAt: ts, ModelName: "gpt-4o", TokenId: 1, TokenName: "alice-key", BillingAccountType: "personal", RequestId: "s5", Quota: 5},
		{UserId: 101, ActorUserId: 101, Type: model.LogTypeRefund, UsageSource: model.LogUsageSourceText, CreatedAt: ts, ModelName: "gpt-4o", TokenId: 1, TokenName: "alice-key", BillingAccountType: "personal", RequestId: "s6", Quota: 5},
	}).Error)
	require.NoError(t, db.Create(&[]model.Task{
		{TaskID: "t1", UserId: 101, ActorUserId: 101, BillingAccountType: "personal", TokenId: 1, TokenName: "alice-key", Status: model.TaskStatusSuccess, Quota: 40, SubmitTime: ts, Properties: model.Properties{OriginModelName: "wan3.0-video"}},
		{TaskID: "t2", UserId: 102, ActorUserId: 102, BillingAccountType: "personal", TokenId: 2, TokenName: "bob-key", Status: model.TaskStatusFailure, Quota: 20, SubmitTime: ts, Properties: model.Properties{OriginModelName: "wan3.0-video"}},
	}).Error)
	require.NoError(t, db.Create(&model.Midjourney{
		UserId: 102, ActorUserId: 102, BillingAccountType: "personal", Action: "IMAGINE", Status: "SUCCESS", Quota: 80, SubmitTime: ts * 1000,
	}).Error)
	return ts
}

func TestGetSiteUsageMatchesUnionOfSelfUsage(t *testing.T) {
	ts := seedSiteUsage(t)
	start, end := ts-3600, ts+3600

	site, err := GetSiteUsage(SiteUsageQuery{
		StartTs:  start,
		EndTs:    end,
		Page:     1,
		PageSize: 20,
	})
	require.NoError(t, err)
	assert.Equal(t, 6, site.Rows.Total)
	assert.Equal(t, 7, site.Totals.RequestCount)
	assert.Equal(t, 6, site.Totals.SuccessCount)
	assert.Equal(t, 1, site.Totals.FailureCount)
	assert.Equal(t, 250, site.Totals.PromptTokens)
	assert.Equal(t, 40, site.Totals.CompletionTokens)
	assert.Equal(t, 15, site.Totals.CacheTokens)
	assert.Equal(t, 390, site.Totals.Quota)

	userNames := map[int]string{}
	for _, row := range site.Rows.Items {
		if row.UserId > 0 {
			userNames[row.UserId] = row.UserName
		}
	}
	assert.Equal(t, "Alice Chen", userNames[101])
	assert.Equal(t, "bob", userNames[102])

	var union dtoSelfUsageTotalsView
	for _, actor := range []int{101, 102} {
		self, err := GetSelfUsage(SelfUsageQuery{
			ActorUserId: actor,
			StartTs:     start,
			EndTs:       end,
			Page:        1,
			PageSize:    20,
		})
		require.NoError(t, err)
		union.RequestCount += self.Totals.RequestCount
		union.SuccessCount += self.Totals.SuccessCount
		union.FailureCount += self.Totals.FailureCount
		union.PromptTokens += self.Totals.PromptTokens
		union.CompletionTokens += self.Totals.CompletionTokens
		union.CacheTokens += self.Totals.CacheTokens
		union.Quota += self.Totals.Quota
	}
	assert.Equal(t, dtoSelfUsageTotalsView{
		RequestCount:     site.Totals.RequestCount,
		SuccessCount:     site.Totals.SuccessCount,
		FailureCount:     site.Totals.FailureCount,
		PromptTokens:     site.Totals.PromptTokens,
		CompletionTokens: site.Totals.CompletionTokens,
		CacheTokens:      site.Totals.CacheTokens,
		Quota:            site.Totals.Quota,
	}, union)
}

type dtoSelfUsageTotalsView struct {
	RequestCount     int
	SuccessCount     int
	FailureCount     int
	PromptTokens     int
	CompletionTokens int
	CacheTokens      int
	Quota            int
}
