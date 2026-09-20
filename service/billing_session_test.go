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

	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBillingSessionTokenQuotaTxHelpersRespectUnlimitedToken(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	limitedToken, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "tx-limited-key", ExpiredTime: -1, RemainQuota: 100})
	require.NoError(t, err)
	unlimitedToken, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "tx-unlimited-key", ExpiredTime: -1, UnlimitedQuota: true})
	require.NoError(t, err)

	limitedRelayInfo := organizationBillingRelayInfo(limitedToken, organization.Id)
	require.NoError(t, model.DB.Transaction(func(tx *gorm.DB) error {
		deducted, err := DecreaseTokenQuotaTx(tx, limitedRelayInfo, 40)
		require.NoError(t, err)
		require.Equal(t, 40, deducted)
		refunded, err := IncreaseTokenQuotaTx(tx, limitedRelayInfo, 15)
		require.NoError(t, err)
		require.Equal(t, 15, refunded)
		return nil
	}))
	storedLimited, err := model.GetTokenById(limitedToken.Id)
	require.NoError(t, err)
	require.Equal(t, 75, storedLimited.RemainQuota)
	require.Equal(t, 25, storedLimited.UsedQuota)

	unlimitedRelayInfo := organizationBillingRelayInfo(unlimitedToken, organization.Id)
	require.NoError(t, model.DB.Transaction(func(tx *gorm.DB) error {
		deducted, err := DecreaseTokenQuotaTx(tx, unlimitedRelayInfo, 40)
		require.NoError(t, err)
		require.Zero(t, deducted)
		refunded, err := IncreaseTokenQuotaTx(tx, unlimitedRelayInfo, 15)
		require.NoError(t, err)
		require.Zero(t, refunded)
		return nil
	}))
	storedUnlimited, err := model.GetTokenById(unlimitedToken.Id)
	require.NoError(t, err)
	require.Zero(t, storedUnlimited.RemainQuota)
	require.Equal(t, 25, storedUnlimited.UsedQuota)
}
