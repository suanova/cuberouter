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
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// OrganizationFunding 是 FundingSource 的组织实现。
//
// 它和其他两个实现的关键差别在于「一次请求动两本账」：
// 组织账户的 used_quota 和令牌的 remain_quota 要一起变，
// 所以预扣/结算/退款都落在 organization_billing.go 的事务里，
// 由账本会话兜住重放与崩溃恢复。这里只做接口适配。
type OrganizationFunding struct {
	relayInfo *relaycommon.RelayInfo
	// session 是本次扣费对应的账本凭据，预扣后填充，结算/退款靠它定位。
	session *model.OrganizationBillingSession
	// tokenConsumed 是当前仍然压在令牌上的额度，退款按它退。
	tokenConsumed int
}

func (f *OrganizationFunding) Source() string {
	return BillingSourceOrganization
}

func (f *OrganizationFunding) PreConsume(amount int) error {
	return f.PreConsumeWithToken(amount)
}

func (f *OrganizationFunding) Settle(delta int) error {
	return f.SettleWithToken(delta)
}

func (f *OrganizationFunding) Refund() error {
	return f.RefundWithToken()
}
