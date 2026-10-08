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
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

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
