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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupErrorLogScopeTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/error-log-scope.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldErrorLogEnabled := constant.ErrorLogEnabled
	common.RedisEnabled = false
	constant.ErrorLogEnabled = true
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.RedisEnabled = oldRedisEnabled
		constant.ErrorLogEnabled = oldErrorLogEnabled
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Organization{}, &model.Log{}))
}

// TestProcessChannelErrorRecordsRelayScopeOnErrorLog 锁定失败日志的作用域归属：
// 组织 Key 的失败日志必须带上 RelayInfo 上的组织计费字段。漏掉这一步时
// NormalizeRecordConsumeLogParams 会把空作用域默认成个人，组织的失败请求在
// 组织日志里就看不到了。
func TestProcessChannelErrorRecordsRelayScopeOnErrorLog(t *testing.T) {
	setupErrorLogScopeTestDB(t)

	tests := []struct {
		name      string
		relayInfo *relaycommon.RelayInfo
		assertLog func(t *testing.T, log model.Log)
	}{
		{
			name: "organization token keeps organization scope",
			relayInfo: &relaycommon.RelayInfo{
				UserId:                        42,
				TokenId:                       442,
				ScopeType:                     model.AccountContextTypeOrganization,
				ScopeId:                       2,
				BillingAccountType:            model.AccountContextTypeOrganization,
				BillingAccountId:              2,
				OrganizationId:                2,
				ActorUserId:                   42,
				CreatorUserId:                 10199,
				ResponsibleUserId:             10199,
				OrganizationBillingSessionId:  77,
				OrganizationBillingSessionKey: "billing-session-key",
			},
			assertLog: func(t *testing.T, log model.Log) {
				require.Equal(t, model.LogTypeError, log.Type)
				require.Zero(t, log.Quota)
				require.Equal(t, model.AccountContextTypeOrganization, log.ScopeType)
				require.Equal(t, 2, log.ScopeId)
				require.Equal(t, model.AccountContextTypeOrganization, log.BillingAccountType)
				require.Equal(t, 2, log.BillingAccountId)
				require.Equal(t, 2, log.OrganizationId)
				require.Equal(t, 42, log.ActorUserId)
				require.Equal(t, 10199, log.CreatorUserId)
				require.Equal(t, 10199, log.ResponsibleUserId)
				require.Equal(t, 77, log.OrganizationBillingSessionId)
				require.Equal(t, "billing-session-key", log.OrganizationBillingSessionKey)
			},
		},
		{
			name:      "missing relay info falls back to personal",
			relayInfo: nil,
			assertLog: func(t *testing.T, log model.Log) {
				require.Equal(t, model.AccountContextTypePersonal, log.ScopeType)
				require.Equal(t, 42, log.ScopeId)
				require.Equal(t, model.AccountContextTypePersonal, log.BillingAccountType)
				require.Equal(t, 42, log.BillingAccountId)
				require.Zero(t, log.OrganizationId)
			},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestId := fmt.Sprintf("req-error-log-scope-%d", i)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			c.Set("id", 42)
			c.Set("username", "frank")
			c.Set("token_name", "POC")
			c.Set("original_model", "gpt-6.1-terra-b")
			c.Set("token_id", 442)
			c.Set("group", "default")
			c.Set("channel_id", 30)
			c.Set("channel_name", "baidu-gpt")
			c.Set("channel_type", 1)
			c.Set(common.RequestIdKey, requestId)
			common.SetContextKey(c, constant.ContextKeyIsStream, true)
			common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())

			channelError := types.NewChannelError(30, 1, "baidu-gpt", false, "", false)
			apiErr := types.NewOpenAIError(
				errors.New("context canceled"),
				types.ErrorCodeStreamClientClosed,
				499,
				types.ErrOptionWithSkipRetry(),
			)

			processChannelError(c, *channelError, apiErr, tt.relayInfo)

			var log model.Log
			require.NoError(t, model.LOG_DB.Where("request_id = ?", requestId).First(&log).Error)
			tt.assertLog(t, log)
		})
	}
}
