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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 前端用 /api/status 的 affiliate_enabled 决定是否展示钱包页的推荐计划卡片,
// 该标志必须与后台配置的邀请奖励一致:两侧奖励都为 0 时视为未启用。
func TestGetStatusAdvertisesAffiliateProgramOnlyWhenRewardsConfigured(t *testing.T) {
	previousOptionMap := common.OptionMap
	previousInviter := common.QuotaForInviter
	previousInvitee := common.QuotaForInvitee
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		common.OptionMap = previousOptionMap
		common.QuotaForInviter = previousInviter
		common.QuotaForInvitee = previousInvitee
	})

	tests := []struct {
		name     string
		inviter  int
		invitee  int
		expected bool
	}{
		{name: "两侧奖励都为 0 时未启用", inviter: 0, invitee: 0, expected: false},
		{name: "仅邀请人奖励时启用", inviter: 500000, invitee: 0, expected: true},
		{name: "仅被邀请人奖励时启用", inviter: 0, invitee: 500000, expected: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			common.QuotaForInviter = test.inviter
			common.QuotaForInvitee = test.invitee

			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)

			GetStatus(context)

			var payload struct {
				Success bool           `json:"success"`
				Data    map[string]any `json:"data"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
			require.True(t, payload.Success)
			assert.Equal(t, test.expected, payload.Data["affiliate_enabled"])
		})
	}
}
