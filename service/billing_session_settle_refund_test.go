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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	hosttypes "github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBillingSessionRefundAfterSettleIsNoop 固定 I1：部分结算完成后，
// controller 的退款 defer 必须是 no-op——钱包只能扣差额，不得整笔退回预扣。
func TestBillingSessionRefundAfterSettleIsNoop(t *testing.T) {
	setupServiceTestDB(t)
	gin.SetMode(gin.TestMode)

	user := createServiceTestUser(t, "refund_noop", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", 1000).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{UserId: user.Id}
	session := &BillingSession{relayInfo: info, funding: &WalletFunding{userId: user.Id}}
	require.NoError(t, session.funding.PreConsume(500))
	session.preConsumedQuota = 500

	require.NoError(t, session.Settle(200)) // 差额 -300 → 退回 300
	session.Refund(ctx)                     // 已结算 → 必须是 no-op
	WaitForBackgroundWork()
	model.WaitForQuotaCacheWorkers()

	var after model.User
	require.NoError(t, model.DB.First(&after, user.Id).Error)
	// 1000 - 500 预扣 + 300 差额退还 = 800；若 Refund 未短路会变成 1300。
	assert.Equal(t, 800, after.Quota)
}

// failingSettleFunding 让 BillingSession.Settle 在提交资金来源时失败，
// 模拟结算期间的数据库故障。
type failingSettleFunding struct{}

func (failingSettleFunding) Source() string       { return BillingSourceWallet }
func (failingSettleFunding) PreConsume(int) error { return nil }
func (failingSettleFunding) Settle(int) error {
	return errors.New("simulated funding settle failure")
}
func (failingSettleFunding) Refund() error { return nil }

func newSettleLogTestContext(t *testing.T, user model.User) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		UserId:          user.Id,
		OriginModelName: "gpt-4o",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o"},
		PriceData: hosttypes.PriceData{
			ModelRatio:     1,
			GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1},
		},
	}
	// ChannelId 是 ChannelMeta 上的提升字段，字面量里不能直接赋值。
	info.ChannelId = 1
	info.StartTime = time.Now()
	info.FirstResponseTime = info.StartTime
	return ctx, info
}

// TestPostTextConsumeQuotaMarksSettleFailure 结算失败时资金来源没有提交，controller
// 的退款 defer 会把预扣全额退回（钱没收到），而消费日志仍如实记录本次请求——必须留下
// 标记，否则对账会把没收到的钱算成收入。
func TestPostTextConsumeQuotaMarksSettleFailure(t *testing.T) {
	setupServiceTestDB(t)
	gin.SetMode(gin.TestMode)

	user := createServiceTestUser(t, "settle_failed_log", common.RoleCommonUser)
	ctx, info := newSettleLogTestContext(t, user)
	// preConsumedQuota=0 且实际额度>0 → delta>0，Settle 必须提交资金来源并失败。
	info.Billing = &BillingSession{relayInfo: info, funding: failingSettleFunding{}}

	PostTextConsumeQuota(ctx, info, &dto.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}, nil)

	var entry model.Log
	require.NoError(t, model.LOG_DB.Where("type = ?", model.LogTypeConsume).Order("id desc").First(&entry).Error)
	assert.Greater(t, entry.Quota, 0, "日志仍如实记录应扣额度")
	assert.Contains(t, entry.Other, `"settle_failed":true`)
	assert.Contains(t, entry.Content, "结算失败")
}

// TestPostTextConsumeQuotaOmitsSettleFailureMarkerOnSuccess 结算成功时不得出现该标记。
func TestPostTextConsumeQuotaOmitsSettleFailureMarkerOnSuccess(t *testing.T) {
	setupServiceTestDB(t)
	gin.SetMode(gin.TestMode)

	user := createServiceTestUser(t, "settle_ok_log", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", 1000).Error)
	ctx, info := newSettleLogTestContext(t, user)
	info.Billing = &BillingSession{relayInfo: info, funding: &WalletFunding{userId: user.Id}}

	PostTextConsumeQuota(ctx, info, &dto.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}, nil)

	var entry model.Log
	require.NoError(t, model.LOG_DB.Where("type = ?", model.LogTypeConsume).Order("id desc").First(&entry).Error)
	assert.NotContains(t, entry.Other, "settle_failed")
}
