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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newOrganizationUsageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newUsageAsyncTestDB(t)
	require.NoError(t, db.AutoMigrate(&Task{}, &Midjourney{}, &Token{}, &Organization{}, &User{}))
	return db
}

func TestGetOrganizationUsageLogRowsGroupsByResponsibleUser(t *testing.T) {
	db := newOrganizationUsageTestDB(t)
	ts := time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&[]Log{
		{UserId: 101, Username: "alice", ResponsibleUserId: 101, ResponsibleName: "Alice", Type: LogTypeConsume, UsageSource: LogUsageSourceText, CreatedAt: ts, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, OrganizationName: "Acme", TokenId: 1, TokenName: "team-key", ModelName: "gpt-4o", RequestId: "o1", PromptTokens: 100, CompletionTokens: 20, CacheTokens: 10, Quota: 100},
		{UserId: 101, Username: "alice", ResponsibleUserId: 101, ResponsibleName: "Alice", Type: LogTypeConsume, UsageSource: LogUsageSourceText, CreatedAt: ts + 30*60, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, OrganizationName: "Acme", TokenId: 1, TokenName: "team-key", ModelName: "gpt-4o", RequestId: "o2", PromptTokens: 50, CompletionTokens: 10, Quota: 50},
		{UserId: 102, Username: "bob", ResponsibleUserId: 102, ResponsibleName: "Bob", Type: LogTypeConsume, UsageSource: LogUsageSourceText, CreatedAt: ts, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, OrganizationName: "Acme", TokenId: 1, TokenName: "team-key", ModelName: "gpt-4o", RequestId: "o3", PromptTokens: 70, CompletionTokens: 5, Quota: 70},
		{UserId: 101, Username: "alice", ResponsibleUserId: 101, ResponsibleName: "Alice", Type: LogTypeConsume, UsageSource: LogUsageSourceFee, CreatedAt: ts + 60*60, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, OrganizationName: "Acme", TokenId: 1, TokenName: "team-key", ModelName: "gpt-4o", RequestId: "o4", Quota: 25},
		{UserId: 101, Username: "alice", ResponsibleUserId: 101, ResponsibleName: "Alice", Type: LogTypeRefund, UsageSource: LogUsageSourceText, CreatedAt: ts + 90*60, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, OrganizationName: "Acme", TokenId: 1, TokenName: "team-key", ModelName: "gpt-4o", RequestId: "o5", Quota: 5},
		{UserId: 101, Username: "alice", ResponsibleUserId: 101, Type: LogTypeConsume, UsageSource: LogUsageSourceText, CreatedAt: ts, BillingAccountType: AccountContextTypePersonal, TokenId: 1, TokenName: "team-key", ModelName: "gpt-4o", RequestId: "p1", Quota: 999},
		{UserId: 103, Username: "carol", ResponsibleUserId: 103, Type: LogTypeConsume, UsageSource: LogUsageSourceText, CreatedAt: ts, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 10, OrganizationId: 10, OrganizationName: "Other", TokenId: 2, TokenName: "other-key", ModelName: "gpt-4o", RequestId: "o6", Quota: 999},
	}).Error)

	rows, err := GetOrganizationUsageLogRowsByBucket(9, ts-60, ts+3*60*60, SelfUsageDaySeconds, 0)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	byResponsible := map[int]SelfUsageLogRow{}
	for _, row := range rows {
		byResponsible[row.ResponsibleUserID] = row
	}
	alice := byResponsible[101]
	assert.Equal(t, 2, alice.RequestCount)
	assert.Equal(t, 170, alice.Quota)
	assert.Equal(t, 150, alice.PromptTokens)
	assert.Equal(t, 30, alice.CompletionTokens)
	assert.Equal(t, 10, alice.CacheTokens)
	assert.Equal(t, "Alice", alice.ResponsibleName)
	assert.Equal(t, "team-key", alice.TokenName)

	bob := byResponsible[102]
	assert.Equal(t, 1, bob.RequestCount)
	assert.Equal(t, 70, bob.Quota)
	assert.Equal(t, "Bob", bob.ResponsibleName)

	filtered, err := GetOrganizationUsageLogRowsByBucket(9, ts-60, ts+3*60*60, SelfUsageDaySeconds, 102)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	assert.Equal(t, 102, filtered[0].ResponsibleUserID)
}

func TestGetOrganizationUsageLogRowsGroupsByHour(t *testing.T) {
	db := newOrganizationUsageTestDB(t)
	ts := time.Date(2026, 11, 3, 10, 5, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&[]Log{
		{UserId: 101, ResponsibleUserId: 101, Type: LogTypeConsume, UsageSource: LogUsageSourceText, CreatedAt: ts, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, ModelName: "gpt-4o", RequestId: "h1", Quota: 10},
		{UserId: 101, ResponsibleUserId: 101, Type: LogTypeConsume, UsageSource: LogUsageSourceText, CreatedAt: ts + 50*60, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, ModelName: "gpt-4o", RequestId: "h2", Quota: 20},
		{UserId: 101, ResponsibleUserId: 101, Type: LogTypeConsume, UsageSource: LogUsageSourceText, CreatedAt: ts + 60*60, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, ModelName: "gpt-4o", RequestId: "h3", Quota: 30},
	}).Error)

	rows, err := GetOrganizationUsageLogRowsByBucket(9, ts-60, ts+2*60*60, SelfUsageHourSeconds, 0)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.NotEqual(t, rows[0].BucketKey, rows[1].BucketKey)
	assert.Equal(t, 60, rows[0].Quota+rows[1].Quota)
}

func TestGetOrganizationUsageTaskAndMidjourneyRowsKeepResponsibleUser(t *testing.T) {
	db := newOrganizationUsageTestDB(t)
	ts := time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&[]Task{
		{TaskID: "task-a", UserId: 101, ActorUserId: 101, ResponsibleUserId: 101, ResponsibleName: "Alice", BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, OrganizationName: "Acme", TokenId: 1, TokenName: "team-key", Status: TaskStatusSuccess, Quota: 40, SubmitTime: ts, Properties: Properties{OriginModelName: "wan3.0-video"}},
		{TaskID: "task-b", UserId: 102, ActorUserId: 102, ResponsibleUserId: 102, ResponsibleName: "Bob", BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, OrganizationName: "Acme", TokenId: 1, TokenName: "team-key", Status: TaskStatusFailure, Quota: 0, SubmitTime: ts, Properties: Properties{OriginModelName: "wan3.0-video"}},
		{TaskID: "task-personal", UserId: 101, ActorUserId: 101, ResponsibleUserId: 101, BillingAccountType: AccountContextTypePersonal, Status: TaskStatusSuccess, Quota: 999, SubmitTime: ts, Properties: Properties{OriginModelName: "wan3.0-video"}},
	}).Error)
	require.NoError(t, db.Create(&[]Midjourney{
		{UserId: 102, ActorUserId: 102, ResponsibleUserId: 102, ResponsibleName: "Bob", BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9, OrganizationName: "Acme", TokenId: 1, TokenName: "team-key", Action: "IMAGINE", Status: "SUCCESS", Quota: 80, SubmitTime: ts * 1000},
	}).Error)

	tasks, err := GetOrganizationUsageTaskRowsByBucket(9, ts-60, ts+60, SelfUsageDaySeconds, 0)
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	byResponsible := map[int]SelfUsageAsyncRow{}
	for _, row := range tasks {
		byResponsible[row.ResponsibleUserID] = row
	}
	assert.Equal(t, 1, byResponsible[101].SuccessCount)
	assert.Equal(t, 40, byResponsible[101].Quota)
	assert.Equal(t, "Alice", byResponsible[101].ResponsibleName)
	assert.Equal(t, 1, byResponsible[102].FailureCount)

	journeys, err := GetOrganizationUsageMidjourneyRowsByBucket(9, ts-60, ts+60, SelfUsageDaySeconds, 102)
	require.NoError(t, err)
	require.Len(t, journeys, 1)
	assert.Equal(t, "mj_imagine", journeys[0].ModelName)
	assert.Equal(t, 102, journeys[0].ResponsibleUserID)
	assert.Equal(t, 80, journeys[0].Quota)
}
