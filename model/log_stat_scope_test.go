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

	"github.com/stretchr/testify/require"
)

// TestPersonalLogStatFiltersOutOrganizationBilling 锁定个人看板顶部那三个数字。
//
// 组织请求把这个人记为责任人，日志里 user_id/username 都是他的，因此只按
// 用户名统计会把组织花的 20 也算进他的个人消耗；按账单归属收窄后只应剩 10。
func TestPersonalLogStatFiltersOutOrganizationBilling(t *testing.T) {
	setupModelTestDB(t)
	user := User{Username: "stat-user", Password: "password", AffCode: "stat-user"}
	require.NoError(t, DB.Create(&user).Error)

	require.NoError(t, LOG_DB.Create(&Log{UserId: user.Id, Username: user.Username, Type: LogTypeConsume, ModelName: "personal", Quota: 10, PromptTokens: 4, CompletionTokens: 6, ScopeType: AccountContextTypePersonal, ScopeId: user.Id, BillingAccountType: AccountContextTypePersonal, BillingAccountId: user.Id}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: user.Id, Username: user.Username, Type: LogTypeConsume, ModelName: "org", Quota: 20, PromptTokens: 7, CompletionTokens: 8, ScopeType: AccountContextTypeOrganization, ScopeId: 99, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 99, OrganizationId: 99, ResponsibleUserId: user.Id}).Error)

	stat, err := SumUsedQuotaForBillingAccount(LogTypeConsume, 0, 0, "", user.Username, "", 0, "", AccountContextTypePersonal, user.Id)
	require.NoError(t, err)
	require.Equal(t, 10, stat.Quota)
}
