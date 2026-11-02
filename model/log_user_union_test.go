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

// TestUserLogUnionCoversResponsibleOrganizationRows locks the personal page's
// definition: my personal-billed rows plus organization-billed rows where I am
// the responsible person. The caller on an organization row may be someone
// else; responsibility is what decides membership.
func TestUserLogUnionCoversResponsibleOrganizationRows(t *testing.T) {
	setupModelTestDB(t)
	me := User{Username: "union-me", Password: "password", AffCode: "union-me"}
	other := User{Username: "union-other", Password: "password", AffCode: "union-other"}
	require.NoError(t, DB.Create(&me).Error)
	require.NoError(t, DB.Create(&other).Error)
	now := time.Now().Unix()

	logs := []Log{
		{UserId: me.Id, Username: me.Username, CreatedAt: now, Type: LogTypeConsume, Content: "legacy", Quota: 10, PromptTokens: 1, CompletionTokens: 1},
		{UserId: me.Id, Username: me.Username, CreatedAt: now, Type: LogTypeConsume, Content: "personal", Quota: 20, PromptTokens: 2, CompletionTokens: 2, ScopeType: AccountContextTypePersonal, ScopeId: me.Id, BillingAccountType: AccountContextTypePersonal, BillingAccountId: me.Id},
		{UserId: me.Id, Username: me.Username, CreatedAt: now, Type: LogTypeConsume, Content: "org-own", Quota: 30, PromptTokens: 3, CompletionTokens: 3, ScopeType: AccountContextTypeOrganization, ScopeId: 7, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 7, OrganizationId: 7, OrganizationName: "Org Seven", ResponsibleUserId: me.Id},
		{UserId: other.Id, Username: other.Username, CreatedAt: now, Type: LogTypeConsume, Content: "org-seven-other", Quota: 1000, ScopeType: AccountContextTypeOrganization, ScopeId: 7, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 7, OrganizationId: 7, ResponsibleUserId: other.Id},
		{UserId: me.Id, Username: me.Username, CreatedAt: now, Type: LogTypeConsume, Content: "org-responsible-other", Quota: 2000, ScopeType: AccountContextTypeOrganization, ScopeId: 7, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 7, OrganizationId: 7, ResponsibleUserId: other.Id},
		{UserId: other.Id, Username: other.Username, CreatedAt: now, Type: LogTypeConsume, Content: "org-eight-mine", Quota: 40, PromptTokens: 4, CompletionTokens: 4, ScopeType: AccountContextTypeOrganization, ScopeId: 8, BillingAccountType: AccountContextTypeOrganization, BillingAccountId: 8, OrganizationId: 8, ResponsibleUserId: me.Id},
		{UserId: other.Id, Username: other.Username, CreatedAt: now, Type: LogTypeConsume, Content: "other-personal", Quota: 500, ScopeType: AccountContextTypePersonal, ScopeId: other.Id, BillingAccountType: AccountContextTypePersonal, BillingAccountId: other.Id},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	find := func(includeOrganization bool, billingAccountType string, organizationId int) ([]*Log, int64) {
		t.Helper()
		rows, total, err := GetUserLogs(me.Id, LogTypeUnknown, 0, 0, "", "", 0, 20, "", "", "", includeOrganization, billingAccountType, organizationId)
		require.NoError(t, err)
		return rows, total
	}

	_, total := find(false, "", 0)
	require.Equal(t, int64(2), total, "without the union only personal-billed rows count")

	rows, total := find(true, "", 0)
	require.Equal(t, int64(4), total)
	contents := make(map[string]bool, len(rows))
	for _, row := range rows {
		contents[row.Content] = true
	}
	require.True(t, contents["legacy"])
	require.True(t, contents["personal"])
	require.True(t, contents["org-own"], "the caller's own organization charge is included")
	require.True(t, contents["org-eight-mine"], "responsibility, not caller, decides the union")
	require.False(t, contents["org-seven-other"])
	require.False(t, contents["org-responsible-other"])
	require.False(t, contents["other-personal"])

	_, total = find(true, AccountContextTypePersonal, 0)
	require.Equal(t, int64(2), total)
	_, total = find(true, AccountContextTypeOrganization, 0)
	require.Equal(t, int64(2), total)
	_, total = find(true, "", 7)
	require.Equal(t, int64(1), total)
	_, total = find(true, "", 8)
	require.Equal(t, int64(1), total)

	stat, err := GetUserLogStats(me.Id, 0, 0, "", "", "", "", "", true, "", 0)
	require.NoError(t, err)
	require.Equal(t, 100, stat.Quota)
	require.Equal(t, 30, stat.PersonalQuota)
	require.Equal(t, 70, stat.OrganizationQuota)
	require.Equal(t, 4, stat.Rpm)
	require.Equal(t, 20, stat.Tpm)

	stat, err = GetUserLogStats(me.Id, 0, 0, "", "", "", "", "", true, AccountContextTypeOrganization, 7)
	require.NoError(t, err)
	require.Equal(t, 30, stat.Quota)
	require.Equal(t, 0, stat.PersonalQuota)
	require.Equal(t, 30, stat.OrganizationQuota)

	stat, err = GetUserLogStats(me.Id, 0, 0, "", "", "", "", "", false, "", 0)
	require.NoError(t, err)
	require.Equal(t, 30, stat.Quota)
	require.Equal(t, 30, stat.PersonalQuota)
	require.Equal(t, 0, stat.OrganizationQuota)
}
