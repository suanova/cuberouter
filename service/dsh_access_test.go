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
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/require"
)

func setDshAllowedOrganizationsForTest(t *testing.T, organizationIds []int) {
	t.Helper()
	encoded := "[]"
	if len(organizationIds) > 0 {
		raw, err := common.Marshal(organizationIds)
		require.NoError(t, err)
		encoded = string(raw)
	}

	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	previous, existed := common.OptionMap[setting.DshAllowedOrganizationIdsKey]
	common.OptionMap[setting.DshAllowedOrganizationIdsKey] = encoded
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if existed {
			common.OptionMap[setting.DshAllowedOrganizationIdsKey] = previous
			return
		}
		delete(common.OptionMap, setting.DshAllowedOrganizationIdsKey)
	})
}

func TestCanAccessDsh(t *testing.T) {
	setupServiceTestDB(t)

	owner := createServiceTestUser(t, "dsh-access-owner", common.RoleCommonUser)
	admin := createServiceTestUser(t, "dsh-access-admin", common.RoleAdminUser)
	member := createServiceTestUser(t, "dsh-access-member", common.RoleCommonUser)
	exitedMember := createServiceTestUser(t, "dsh-access-exited", common.RoleCommonUser)
	outsider := createServiceTestUser(t, "dsh-access-outsider", common.RoleCommonUser)

	allowedOrganization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "DSH Allowed Organization"})
	require.NoError(t, err)
	otherOrganization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "DSH Other Organization"})
	require.NoError(t, err)
	disabledOrganization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "DSH Disabled Organization"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).
		Where("id = ?", disabledOrganization.Id).
		Update("status", model.OrganizationStatusDisabled).Error)

	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: allowedOrganization.Id,
		UserId:         member.Id,
		Role:           model.OrganizationRoleMember,
		Status:         model.OrganizationMemberStatusActive,
	}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: disabledOrganization.Id,
		UserId:         member.Id,
		Role:           model.OrganizationRoleMember,
		Status:         model.OrganizationMemberStatusActive,
	}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: allowedOrganization.Id,
		UserId:         exitedMember.Id,
		Role:           model.OrganizationRoleMember,
		Status:         model.OrganizationMemberStatusExited,
	}).Error)

	t.Run("empty allowlist denies a user without an allowlisted organization", func(t *testing.T) {
		setDshAllowedOrganizationsForTest(t, nil)
		require.False(t, CanAccessDsh(member.Id))
	})

	t.Run("platform admin is allowed even when the allowlist is empty", func(t *testing.T) {
		setDshAllowedOrganizationsForTest(t, nil)
		require.True(t, CanAccessDsh(admin.Id))
	})

	t.Run("active membership in an allowlisted organization grants access", func(t *testing.T) {
		setDshAllowedOrganizationsForTest(t, []int{allowedOrganization.Id})
		require.True(t, CanAccessDsh(member.Id))
	})

	t.Run("membership in another organization does not grant access", func(t *testing.T) {
		setDshAllowedOrganizationsForTest(t, []int{otherOrganization.Id})
		require.False(t, CanAccessDsh(member.Id))
	})

	t.Run("a disabled allowlisted organization does not grant access", func(t *testing.T) {
		setDshAllowedOrganizationsForTest(t, []int{disabledOrganization.Id})
		require.False(t, CanAccessDsh(member.Id))
	})

	t.Run("an exited membership does not grant access", func(t *testing.T) {
		setDshAllowedOrganizationsForTest(t, []int{allowedOrganization.Id})
		require.False(t, CanAccessDsh(exitedMember.Id))
	})

	t.Run("a user without any organization is denied", func(t *testing.T) {
		setDshAllowedOrganizationsForTest(t, []int{allowedOrganization.Id})
		require.False(t, CanAccessDsh(outsider.Id))
	})

	t.Run("any matching organization is enough", func(t *testing.T) {
		setDshAllowedOrganizationsForTest(t, []int{otherOrganization.Id, allowedOrganization.Id})
		require.True(t, CanAccessDsh(member.Id))
	})
}
