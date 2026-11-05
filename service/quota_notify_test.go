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
	"fmt"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func quotaNotifyKeyPrefix(userId int) string {
	return fmt.Sprintf("%d:%s:", userId, dto.NotifyTypeQuotaExceed)
}

func clearQuotaNotifyAttempts(userId int) {
	prefix := quotaNotifyKeyPrefix(userId)
	notifyLimitStore.Range(func(key, _ interface{}) bool {
		if strings.HasPrefix(fmt.Sprint(key), prefix) {
			notifyLimitStore.Delete(key)
		}
		return true
	})
}

func hasQuotaNotifyAttempt(userId int) bool {
	prefix := quotaNotifyKeyPrefix(userId)
	found := false
	notifyLimitStore.Range(func(key, _ interface{}) bool {
		if strings.HasPrefix(fmt.Sprint(key), prefix) {
			found = true
			return false
		}
		return true
	})
	return found
}

// 组织计费不动个人钱包，个人余额低于阈值不能作为发预警的依据。
func TestOrganizationBillingDoesNotSendPersonalQuotaWarning(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := insertOrganizationTokenForTest(t, organization, member, OrganizationTokenRequest{Name: "org-quota-notify", ExpiredTime: -1, RemainQuota: 1000})
	require.NoError(t, err)
	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "org-quota-notify"
	relayInfo.UserQuota = 0
	relayInfo.UserEmail = "org-owner@example.com"
	relayInfo.UserSetting = dto.UserSetting{
		NotifyType:            dto.NotifyTypeEmail,
		QuotaWarningThreshold: 500000,
	}

	clearQuotaNotifyAttempts(relayInfo.UserId)
	require.Nil(t, PreConsumeBilling(&gin.Context{}, 100, relayInfo))
	require.NoError(t, SettleBilling(&gin.Context{}, relayInfo, 135))
	WaitForBackgroundWork()

	require.False(t, hasQuotaNotifyAttempt(relayInfo.UserId), "organization billing must not send personal quota warnings")
}

// 个人钱包计费的预警必须保留，防止修复把整条通知路径关掉。
func TestPersonalBillingStillSendsQuotaWarning(t *testing.T) {
	setupServiceTestDB(t)
	relayInfo := &relaycommon.RelayInfo{
		UserId:    424242,
		UserQuota: 0,
		UserEmail: "personal-user@example.com",
		UserSetting: dto.UserSetting{
			NotifyType:            dto.NotifyTypeEmail,
			QuotaWarningThreshold: 500000,
		},
	}

	clearQuotaNotifyAttempts(relayInfo.UserId)
	checkAndSendQuotaNotify(relayInfo, 100, 0)
	WaitForBackgroundWork()

	require.True(t, hasQuotaNotifyAttempt(relayInfo.UserId), "personal billing must keep sending quota warnings")
}
