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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateOrganizationDefaultMemberTokenQuotaSetsKeepsAndClears(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "default-token-quota-owner", common.RoleCommonUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Default Token Quota Org"})
	require.NoError(t, err)
	require.Zero(t, organization.DefaultMemberTokenQuota)

	value := 5000
	updated, err := UpdateOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, UpdateOrganizationRequest{
		Name: "Default Token Quota Org", DefaultMemberTokenQuota: &value,
	})
	require.NoError(t, err)
	require.Equal(t, 5000, updated.DefaultMemberTokenQuota)

	// 缺省字段不修改已存值。
	updated, err = UpdateOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, UpdateOrganizationRequest{Name: "Renamed"})
	require.NoError(t, err)
	require.Equal(t, 5000, updated.DefaultMemberTokenQuota)

	// 0 表示清除（无默认 = 不限）。
	zero := 0
	updated, err = UpdateOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, UpdateOrganizationRequest{Name: "Renamed", DefaultMemberTokenQuota: &zero})
	require.NoError(t, err)
	require.Zero(t, updated.DefaultMemberTokenQuota)

	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	assert.Zero(t, stored.DefaultMemberTokenQuota)
}

func TestUpdateOrganizationDefaultMemberTokenQuotaRejectsOutOfRange(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "default-token-quota-range-owner", common.RoleCommonUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Default Token Quota Range Org"})
	require.NoError(t, err)

	negative := -1
	_, err = UpdateOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, UpdateOrganizationRequest{Name: "Range Org", DefaultMemberTokenQuota: &negative})
	require.Error(t, err)

	tooLarge := maxOrganizationTokenQuotaValue() + 1
	_, err = UpdateOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, UpdateOrganizationRequest{Name: "Range Org", DefaultMemberTokenQuota: &tooLarge})
	require.Error(t, err)

	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Zero(t, stored.DefaultMemberTokenQuota)
}
