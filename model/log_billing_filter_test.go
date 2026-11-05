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

	"github.com/stretchr/testify/require"
)

// TestAdminLogBillingFilters locks the admin-wide Billing Account filter.
//
// Rows written before scoped billing existed carry an empty
// billing_account_type. They must count as personal, otherwise selecting
// "personal" would silently hide the installation's older usage.
func TestAdminLogBillingFilters(t *testing.T) {
	setupModelTestDB(t)
	now := time.Now().Unix()
	logs := []Log{
		{UserId: 1, Username: "alice", Type: LogTypeConsume, ModelName: "legacy", Quota: 10, CreatedAt: now},
		{UserId: 1, Username: "alice", Type: LogTypeConsume, ModelName: "personal", Quota: 20, BillingAccountType: AccountContextTypePersonal, BillingAccountId: 1, CreatedAt: now},
		{UserId: 1, Username: "alice", Type: LogTypeConsume, ModelName: "org-99", Quota: 30, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 99, OrganizationId: 99, CreatedAt: now},
		{UserId: 1, Username: "alice", Type: LogTypeConsume, ModelName: "org-100", Quota: 40, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 100, OrganizationId: 100, CreatedAt: now},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	findLogs := func(billingAccountType string, organizationId int) ([]*Log, int64) {
		t.Helper()
		rows, total, err := GetAllLogs(LogTypeConsume, 0, 0, "", "", "", 0, 10, 0, "", "", "", billingAccountType, organizationId)
		require.NoError(t, err)
		return rows, total
	}

	_, total := findLogs("", 0)
	require.Equal(t, int64(4), total)

	_, total = findLogs(AccountContextTypePersonal, 0)
	require.Equal(t, int64(2), total, "legacy empty billing type must count as personal")

	_, total = findLogs(AccountContextTypeOrganization, 0)
	require.Equal(t, int64(2), total)

	rows, total := findLogs(AccountContextTypeOrganization, 99)
	require.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	require.Equal(t, "org-99", rows[0].ModelName)

	stat, err := SumUsedQuota(LogTypeConsume, 0, 0, "", "", "", 0, "", AccountContextTypePersonal, 0)
	require.NoError(t, err)
	require.Equal(t, 30, stat.Quota, "personal stats must include legacy rows and exclude organization rows")

	stat, err = SumUsedQuota(LogTypeConsume, 0, 0, "", "", "", 0, "", AccountContextTypeOrganization, 99)
	require.NoError(t, err)
	require.Equal(t, 30, stat.Quota)

	stat, err = SumUsedQuota(LogTypeConsume, 0, 0, "", "", "", 0, "", "", 0)
	require.NoError(t, err)
	require.Equal(t, 100, stat.Quota)
}
