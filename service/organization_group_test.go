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
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestOrganizationBeforeCreateDefaultsGroup(t *testing.T) {
	setupServiceTestDB(t)
	organization := model.Organization{Name: "Default Group Org", Slug: "default-group-org", CreatedBy: 1}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.Equal(t, DefaultOrganizationGroup, organization.Group)
}

func TestNormalizeOrganizationGroupFallsBackToDefault(t *testing.T) {
	require.Equal(t, DefaultOrganizationGroup, NormalizeOrganizationGroup(""))
	require.Equal(t, DefaultOrganizationGroup, NormalizeOrganizationGroup("   "))
	require.Equal(t, "vip", NormalizeOrganizationGroup(" vip "))
}

func TestAccountUsableGroupsIncludesOwnGroupWhenMissing(t *testing.T) {
	groups := GetAccountUsableGroups("exclusive")
	require.Equal(t, "用户分组", groups["exclusive"])
}

func TestOrganizationUsableGroupsUsesDefaultWhenEmpty(t *testing.T) {
	groups := GetOrganizationUsableGroups("")
	require.Contains(t, groups, DefaultOrganizationGroup)
}

func TestOrganizationGroupsActiveMemberCanRead(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	groups, err := GetOrganizationGroups(member.Id, organization.Id, OrganizationAccessModeWorkspace)
	require.NoError(t, err)
	require.Contains(t, groups, DefaultOrganizationGroup)
	_ = admin
}

func TestOrganizationGroupsDisabledOwnerCanRead(t *testing.T) {
	admin, _, organization := createOrganizationTokenTestOrg(t)
	require.NoError(t, DisableOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "maintenance"))

	groups, err := GetOrganizationGroups(admin.Id, organization.Id, OrganizationAccessModeReadOnly)

	require.NoError(t, err)
	require.Contains(t, groups, DefaultOrganizationGroup)
}

func TestOrganizationGroupsDisabledMemberDenied(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	require.NoError(t, DisableOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "maintenance"))

	_, err := GetOrganizationGroups(member.Id, organization.Id, OrganizationAccessModeReadOnly)

	require.ErrorContains(t, err, "organization disabled")
}

func TestOrganizationGroupsNonMemberDenied(t *testing.T) {
	_, _, organization := createOrganizationTokenTestOrg(t)
	nonMember := createServiceTestUser(t, "org-groups-non-member", common.RoleCommonUser)
	_, err := GetOrganizationGroups(nonMember.Id, organization.Id, OrganizationAccessModeWorkspace)
	require.ErrorContains(t, err, "organization access denied")
}

func TestOrganizationGroupsUsesOrganizationGroupNotUserGroup(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("group", "vip").Error)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", member.Id).Update("group", "member-only").Error)
	groups, err := GetOrganizationGroups(member.Id, organization.Id, OrganizationAccessModeWorkspace)
	require.NoError(t, err)
	require.Contains(t, groups, "vip")
	require.NotContains(t, groups, "member-only")
}

func TestOrganizationTokenGroupValidationUsesOrganizationGroupNotResponsibleUserGroup(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("group", "vip").Error)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", member.Id).Update("group", "member-only").Error)
	_, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "org-group", ExpiredTime: -1, ResponsibleUserId: member.Id, Group: "vip"})
	require.NoError(t, err)
	_, err = CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "member-group", ExpiredTime: -1, ResponsibleUserId: member.Id, Group: "member-only"})
	require.ErrorContains(t, err, "token group is not usable by organization")
}

func TestOrganizationTokenAuthUsesOrganizationGroupForEmptyTokenGroup(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("group", "vip").Error)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "auth-group", ExpiredTime: -1})
	require.NoError(t, err)
	ctx, err := ValidateTokenScopeForRelay(token)
	require.NoError(t, err)
	require.Equal(t, "vip", ctx.AccountGroup)
}

func TestOrganizationTokenCreateAllowsAutoWhenUsable(t *testing.T) {
	oldUserUsableGroups := setting.UserUsableGroups2JSONString()
	oldAutoGroups := setting.AutoGroups2JsonString()
	oldGroupRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUserUsableGroups))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(oldAutoGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroupRatio))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组","auto":"自动分组"}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["default"]`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
	admin, _, organization := createOrganizationTokenTestOrg(t)
	_, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "auto-key", ExpiredTime: -1, Group: "auto"})
	require.NoError(t, err)
}
