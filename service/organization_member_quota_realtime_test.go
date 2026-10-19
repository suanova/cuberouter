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

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/require"
)

// memberQuotaRealtimeRelayInfo 构造 /v1/realtime 会话开始检查所需的计费上下文：
// 组织计费账户 + 责任成员。长连接没有预扣，准入只看这两个字段。
func memberQuotaRealtimeRelayInfo(organizationId, responsibleUserId int) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		BillingAccountType: model.AccountContextTypeOrganization,
		BillingAccountId:   organizationId,
		OrganizationId:     organizationId,
		ResponsibleUserId:  responsibleUserId,
	}
}

// setMemberQuotaRealtimeUsage 直接改成员行的上限与已用值：会话开始检查只读这两列，
// 不走配置接口（配置语义由 Task 2 的测试覆盖）。
func setMemberQuotaRealtimeUsage(t *testing.T, organizationId, userId, limit, used int) {
	t.Helper()
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", organizationId, userId).
		Updates(map[string]any{"quota_limit": limit, "used_quota": used}).Error)
}

func TestCheckOrganizationMemberQuotaAtSessionStart(t *testing.T) {
	t.Run("unlimited member is admitted", func(t *testing.T) {
		_, _, member, organization := createMemberQuotaTestOrg(t)
		setMemberQuotaRealtimeUsage(t, organization.Id, member.Id, 0, 5000)

		require.NoError(t, CheckOrganizationMemberQuotaAtSessionStart(memberQuotaRealtimeRelayInfo(organization.Id, member.Id)))
	})

	t.Run("member below the limit is admitted", func(t *testing.T) {
		_, _, member, organization := createMemberQuotaTestOrg(t)
		setMemberQuotaRealtimeUsage(t, organization.Id, member.Id, 100, 60)

		require.NoError(t, CheckOrganizationMemberQuotaAtSessionStart(memberQuotaRealtimeRelayInfo(organization.Id, member.Id)))
	})

	t.Run("member at the limit is rejected", func(t *testing.T) {
		_, _, member, organization := createMemberQuotaTestOrg(t)
		setMemberQuotaRealtimeUsage(t, organization.Id, member.Id, 100, 100)

		err := CheckOrganizationMemberQuotaAtSessionStart(memberQuotaRealtimeRelayInfo(organization.Id, member.Id))
		require.ErrorIs(t, err, ErrOrganizationMemberQuotaExceeded)

		// 准入检查只读：拒绝时也不能改成员计数（没有预留）。
		limit, used := storedMemberQuota(t, organization.Id, member.Id)
		require.Equal(t, 100, limit)
		require.Equal(t, 100, used)
	})

	t.Run("owner is exempt even with a stored limit", func(t *testing.T) {
		owner, _, _, organization := createMemberQuotaTestOrg(t)
		// owner 豁免按 organization.owner_user_id 动态判定：存量配额（例如被降级前配置）
		// 不能拦截 owner 建连。
		setMemberQuotaRealtimeUsage(t, organization.Id, owner.Id, 10, 500)

		require.NoError(t, CheckOrganizationMemberQuotaAtSessionStart(memberQuotaRealtimeRelayInfo(organization.Id, owner.Id)))
	})

	t.Run("non-organization billing is ignored", func(t *testing.T) {
		_, _, member, organization := createMemberQuotaTestOrg(t)
		setMemberQuotaRealtimeUsage(t, organization.Id, member.Id, 100, 100)
		relayInfo := memberQuotaRealtimeRelayInfo(organization.Id, member.Id)
		relayInfo.BillingAccountType = model.AccountContextTypePersonal
		relayInfo.BillingAccountId = member.Id

		require.NoError(t, CheckOrganizationMemberQuotaAtSessionStart(relayInfo))
	})

	t.Run("request without a responsible user is ignored", func(t *testing.T) {
		_, _, _, organization := createMemberQuotaTestOrg(t)

		require.NoError(t, CheckOrganizationMemberQuotaAtSessionStart(memberQuotaRealtimeRelayInfo(organization.Id, 0)))
	})

	t.Run("missing member row is admitted", func(t *testing.T) {
		_, _, _, organization := createMemberQuotaTestOrg(t)
		outsider := createServiceTestUser(t, "member-quota-realtime-outsider", common.RoleCommonUser)

		require.NoError(t, CheckOrganizationMemberQuotaAtSessionStart(memberQuotaRealtimeRelayInfo(organization.Id, outsider.Id)))
	})
}
