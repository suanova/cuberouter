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
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestNormalizeLogScopeDefaultsLegacyLogToPersonal(t *testing.T) {
	log := &Log{UserId: 42}

	NormalizeLogScope(log)

	require.Equal(t, AccountContextTypePersonal, log.ScopeType)
	require.Equal(t, 42, log.ScopeId)
	require.Equal(t, AccountContextTypePersonal, log.BillingAccountType)
	require.Equal(t, 42, log.BillingAccountId)
	require.Equal(t, 42, log.CreatorUserId)
	require.Equal(t, 42, log.ResponsibleUserId)
}

func TestNormalizeRecordConsumeLogParamsDefaultsMissingScopeToPersonal(t *testing.T) {
	params := RecordConsumeLogParams{}

	NormalizeRecordConsumeLogParams(42, &params)

	require.Equal(t, AccountContextTypePersonal, params.ScopeType)
	require.Equal(t, 42, params.ScopeId)
	require.Equal(t, AccountContextTypePersonal, params.BillingAccountType)
	require.Equal(t, 42, params.BillingAccountId)
	require.Equal(t, 42, params.CreatorUserId)
	require.Equal(t, 42, params.ResponsibleUserId)
}

func TestCreateConsumeLogBillingEventDuplicateIsReplay(t *testing.T) {
	setupModelTestDB(t)
	eventKey := "violation_fee:1"

	inserted, err := createConsumeLog(&Log{Type: LogTypeConsume, BillingEventKey: &eventKey})
	require.NoError(t, err)
	require.True(t, inserted)
	inserted, err = createConsumeLog(&Log{Type: LogTypeConsume, BillingEventKey: &eventKey})
	require.NoError(t, err)
	require.False(t, inserted)

	var count int64
	require.NoError(t, LOG_DB.Model(&Log{}).Where("billing_event_key = ?", eventKey).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

// TestCreateConsumeLogBillingEventNonDuplicateErrorIsReturned 确认幂等键的唯一索引
// 只吞掉「重复」这一类错误。把别的写入失败也当成重放，会让真正没记上的账静默消失。
func TestCreateConsumeLogBillingEventNonDuplicateErrorIsReturned(t *testing.T) {
	setupModelTestDB(t)
	expected := errors.New("consume log insert failed")
	require.NoError(t, LOG_DB.Callback().Create().Before("gorm:create").Register("test:consume-log-insert-failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "logs" {
			tx.AddError(expected)
		}
	}))
	eventKey := "violation_fee:2"

	inserted, err := createConsumeLog(&Log{Type: LogTypeConsume, BillingEventKey: &eventKey})

	require.False(t, inserted)
	require.ErrorIs(t, err, expected)
}

// TestRecordConsumeLogBillingEventReplayExportsOnce 覆盖「重放不进看板」这一条：
// 幂等键挡住重复日志的同时，导出到 quota_data 也必须只发生一次，否则看板上的
// 用量会被同一次请求记两遍。
//
// 日志库与主库同库、分库两种部署都要成立，所以两种都跑。
func TestRecordConsumeLogBillingEventReplayExportsOnce(t *testing.T) {
	for _, separateLogDB := range []bool{false, true} {
		name := "shared_log_db"
		if separateLogDB {
			name = "separate_log_db"
		}
		t.Run(name, func(t *testing.T) {
			setupModelTestDB(t)
			if separateLogDB {
				logDB, err := gorm.Open(sqlite.Open(t.TempDir()+"/logs.db"), &gorm.Config{})
				require.NoError(t, err)
				LOG_DB = logDB
				require.NoError(t, LOG_DB.AutoMigrate(&Log{}))
				// 分库部署下日志库是另一套 schema，幂等键的唯一索引同样要显式补上
				// （见 ensureOrganizationBillingLogIndexes），否则这个子用例证明不了什么。
				require.NoError(t, prepareLogBillingEventKeyMigration(LOG_DB))
				require.NoError(t, ensureOrganizationBillingLogIndexes(LOG_DB))
			}
			previousLogConsumeEnabled := common.LogConsumeEnabled
			previousDataExportEnabled := common.DataExportEnabled
			common.LogConsumeEnabled = true
			common.DataExportEnabled = true
			t.Cleanup(func() {
				common.LogConsumeEnabled = previousLogConsumeEnabled
				common.DataExportEnabled = previousDataExportEnabled
			})
			CacheQuotaDataLock.Lock()
			previousCache := CacheQuotaData
			CacheQuotaData = make(map[string]*QuotaData)
			CacheQuotaDataLock.Unlock()
			t.Cleanup(func() {
				CacheQuotaDataLock.Lock()
				CacheQuotaData = previousCache
				CacheQuotaDataLock.Unlock()
			})
			ctx := &gin.Context{}
			ctx.Set("username", "billing-event-user")
			ctx.Set(common.RequestIdKey, "billing-event-request")
			params := RecordConsumeLogParams{
				BillingEventKey:    "violation_fee:3",
				Quota:              25,
				PromptTokens:       2,
				CompletionTokens:   3,
				BillingAccountType: AccountContextTypeOrganization,
				BillingAccountId:   9,
				ScopeType:          AccountContextTypeOrganization,
				ScopeId:            9,
				OrganizationId:     9,
				ResponsibleUserId:  4,
			}
			RecordConsumeLog(ctx, 4, params)
			require.Eventually(t, func() bool { return cachedQuotaDataCount() == 1 }, time.Second, 10*time.Millisecond)
			RecordConsumeLog(ctx, 4, params)
			require.Never(t, func() bool { return cachedQuotaDataCount() != 1 }, 200*time.Millisecond, 10*time.Millisecond)

			var logs []Log
			require.NoError(t, LOG_DB.Where("billing_event_key = ?", params.BillingEventKey).Find(&logs).Error)
			require.Len(t, logs, 1)
			require.NotNil(t, logs[0].BillingEventKey)
			require.Equal(t, params.BillingEventKey, *logs[0].BillingEventKey)
		})
	}
}

func TestRecordConsumeLogOrdinaryEventsKeepBillingEventKeyNull(t *testing.T) {
	setupModelTestDB(t)
	previousLogConsumeEnabled := common.LogConsumeEnabled
	previousDataExportEnabled := common.DataExportEnabled
	common.LogConsumeEnabled = true
	common.DataExportEnabled = false
	t.Cleanup(func() {
		common.LogConsumeEnabled = previousLogConsumeEnabled
		common.DataExportEnabled = previousDataExportEnabled
	})

	RecordConsumeLog(&gin.Context{}, 4, RecordConsumeLogParams{Content: "ordinary one"})
	RecordConsumeLog(&gin.Context{}, 4, RecordConsumeLogParams{Content: "ordinary two"})

	var count int64
	require.NoError(t, LOG_DB.Model(&Log{}).Where("billing_event_key IS NULL").Count(&count).Error)
	require.EqualValues(t, 2, count)
}

func cachedQuotaDataCount() int {
	CacheQuotaDataLock.Lock()
	defer CacheQuotaDataLock.Unlock()
	total := 0
	for _, quotaData := range CacheQuotaData {
		total += quotaData.Count
	}
	return total
}

func TestRecordErrorLogPersistsOrganizationScope(t *testing.T) {
	setupModelTestDB(t)
	ctx := &gin.Context{}
	ctx.Set("username", "org-error-user")
	ctx.Set(common.RequestIdKey, "req-org-error")

	RecordErrorLog(ctx, 42, RecordConsumeLogParams{
		ChannelId:          7,
		ModelName:          "gpt-error",
		TokenName:          "org-key",
		Content:            "upstream failed",
		TokenId:            11,
		Group:              "vip",
		ScopeType:          AccountContextTypeOrganization,
		ScopeId:            99,
		BillingAccountType: AccountContextTypeOrganization,
		BillingAccountId:   99,
		OrganizationId:     99,
		ActorUserId:        42,
		CreatorUserId:      40,
		ResponsibleUserId:  41,
	})

	var log Log
	require.NoError(t, LOG_DB.Where("request_id = ?", "req-org-error").First(&log).Error)
	require.Equal(t, LogTypeError, log.Type)
	require.Zero(t, log.Quota)
	require.Equal(t, AccountContextTypeOrganization, log.ScopeType)
	require.Equal(t, 99, log.ScopeId)
	require.Equal(t, AccountContextTypeOrganization, log.BillingAccountType)
	require.Equal(t, 99, log.BillingAccountId)
	require.Equal(t, 99, log.OrganizationId)
	require.Equal(t, 42, log.ActorUserId)
	require.Equal(t, 40, log.CreatorUserId)
	require.Equal(t, 41, log.ResponsibleUserId)
	require.Equal(t, "org-key", log.TokenName)
}

// TestPersonalLogQueriesFilterOutOrganizationLogs 锁定个人日志视图的边界。
//
// 组织请求的 user_id 是操作者本人，所以「按 user_id 查」会把组织的消费列进
// 他的个人日志。scope_type 为空的老行必须仍然算个人，否则升级后历史日志消失。
func TestPersonalLogQueriesFilterOutOrganizationLogs(t *testing.T) {
	setupModelTestDB(t)
	user := User{Username: "log-user", Password: "password", AffCode: "log-user"}
	require.NoError(t, DB.Create(&user).Error)
	legacy := Log{UserId: user.Id, Type: LogTypeConsume, Content: "legacy"}
	personal := Log{UserId: user.Id, Type: LogTypeConsume, Content: "personal", ScopeType: AccountContextTypePersonal, ScopeId: user.Id}
	organization := Log{UserId: user.Id, Type: LogTypeConsume, Content: "organization", ScopeType: AccountContextTypeOrganization, ScopeId: 99, OrganizationId: 99}
	require.NoError(t, LOG_DB.Create(&legacy).Error)
	require.NoError(t, LOG_DB.Create(&personal).Error)
	require.NoError(t, LOG_DB.Create(&organization).Error)

	logs, total, err := GetUserLogs(user.Id, LogTypeUnknown, 0, 0, "", "", 0, 20, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, logs, 2)
	for _, log := range logs {
		require.NotEqual(t, "organization", log.Content)
	}
}
