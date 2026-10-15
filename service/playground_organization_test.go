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
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func createPlaygroundOrganizationFixture(t *testing.T, name, group string, quota, usedQuota int) *model.Organization {
	t.Helper()
	organization := &model.Organization{
		Name:      name,
		Slug:      "pg-" + strings.ToLower(strings.ReplaceAll(name, " ", "-")),
		Group:     group,
		Status:    model.OrganizationStatusActive,
		Quota:     quota,
		UsedQuota: usedQuota,
	}
	require.NoError(t, model.DB.Create(organization).Error)
	return organization
}

func addPlaygroundOrganizationMember(t *testing.T, userId, organizationId int, status string) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: organizationId,
		UserId:         userId,
		Role:           model.OrganizationRoleMember,
		Status:         status,
	}).Error)
}

func setPlaygroundLastOrganization(t *testing.T, userId, organizationId int) {
	t.Helper()
	userSetting, err := model.GetUserSetting(userId, false)
	require.NoError(t, err)
	userSetting.PlaygroundLastOrganizationId = organizationId
	require.NoError(t, model.UpdateUserSetting(userId, userSetting))
}

func TestSelectPlaygroundOrganizationPrefersLastUsed(t *testing.T) {
	setupServiceTestDB(t)
	user := createServiceTestUser(t, "pg-select-last-used", common.RoleCommonUser)
	lastUsed := createPlaygroundOrganizationFixture(t, "last used", "org-a", 50000, 0)
	richer := createPlaygroundOrganizationFixture(t, "richer", "org-b", 200000, 0)
	addPlaygroundOrganizationMember(t, user.Id, lastUsed.Id, model.OrganizationMemberStatusActive)
	addPlaygroundOrganizationMember(t, user.Id, richer.Id, model.OrganizationMemberStatusActive)
	setPlaygroundLastOrganization(t, user.Id, lastUsed.Id)

	selected, ok, err := SelectPlaygroundOrganization(user.Id)

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, lastUsed.Id, selected.Id)
}

func TestSelectPlaygroundOrganizationFallsBackToMaxRemaining(t *testing.T) {
	setupServiceTestDB(t)
	user := createServiceTestUser(t, "pg-select-max-remaining", common.RoleCommonUser)
	small := createPlaygroundOrganizationFixture(t, "small", "org-small", 10000, 0)
	big := createPlaygroundOrganizationFixture(t, "big", "org-big", 90000, 10000)
	addPlaygroundOrganizationMember(t, user.Id, small.Id, model.OrganizationMemberStatusActive)
	addPlaygroundOrganizationMember(t, user.Id, big.Id, model.OrganizationMemberStatusActive)
	// last-used 指向一个用户不是成员的组织 → 失效，退化到剩余最多
	setPlaygroundLastOrganization(t, user.Id, 999999)

	selected, ok, err := SelectPlaygroundOrganization(user.Id)

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, big.Id, selected.Id)
}

func TestSelectPlaygroundOrganizationTieBreaksByLowestId(t *testing.T) {
	setupServiceTestDB(t)
	user := createServiceTestUser(t, "pg-select-tie", common.RoleCommonUser)
	first := createPlaygroundOrganizationFixture(t, "tie first", "org-tie-a", 50000, 0)
	second := createPlaygroundOrganizationFixture(t, "tie second", "org-tie-b", 80000, 30000)
	addPlaygroundOrganizationMember(t, user.Id, second.Id, model.OrganizationMemberStatusActive)
	addPlaygroundOrganizationMember(t, user.Id, first.Id, model.OrganizationMemberStatusActive)

	selected, ok, err := SelectPlaygroundOrganization(user.Id)

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, first.Id, selected.Id)
}

func TestSelectPlaygroundOrganizationExcludesInvalidCandidates(t *testing.T) {
	setupServiceTestDB(t)
	user := createServiceTestUser(t, "pg-select-excluded", common.RoleCommonUser)

	disabledOrg := createPlaygroundOrganizationFixture(t, "disabled org", "org-disabled", 50000, 0)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", disabledOrg.Id).Update("status", model.OrganizationStatusDisabled).Error)
	addPlaygroundOrganizationMember(t, user.Id, disabledOrg.Id, model.OrganizationMemberStatusActive)

	emptyOrg := createPlaygroundOrganizationFixture(t, "empty org", "org-empty", 50000, 50000)
	addPlaygroundOrganizationMember(t, user.Id, emptyOrg.Id, model.OrganizationMemberStatusActive)

	disabledMemberOrg := createPlaygroundOrganizationFixture(t, "disabled member org", "org-disabled-member", 50000, 0)
	addPlaygroundOrganizationMember(t, user.Id, disabledMemberOrg.Id, model.OrganizationMemberStatusDisabled)

	selected, ok, err := SelectPlaygroundOrganization(user.Id)

	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, selected)
}

func TestSelectPlaygroundOrganizationExcludesDisabledUser(t *testing.T) {
	setupServiceTestDB(t)
	user := createServiceTestUser(t, "pg-select-disabled-user", common.RoleCommonUser)
	organization := createPlaygroundOrganizationFixture(t, "user disabled", "org-user-disabled", 50000, 0)
	addPlaygroundOrganizationMember(t, user.Id, organization.Id, model.OrganizationMemberStatusActive)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("status", common.UserStatusDisabled).Error)

	selected, ok, err := SelectPlaygroundOrganization(user.Id)

	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, selected)
}

func TestBuildPlaygroundOrganizationScope(t *testing.T) {
	organization := &model.Organization{Id: 7, Group: "", Quota: 200000, UsedQuota: 50000}

	scope := BuildPlaygroundOrganizationScope(organization, 42)

	require.NotNil(t, scope)
	require.Equal(t, model.AccountContextTypeOrganization, scope.ScopeType)
	require.Equal(t, 7, scope.ScopeId)
	require.Equal(t, model.AccountContextTypeOrganization, scope.BillingAccountType)
	require.Equal(t, 7, scope.BillingAccountId)
	require.Equal(t, 7, scope.OrganizationId)
	require.Equal(t, 42, scope.ActorUserId)
	require.Equal(t, 42, scope.CreatorUserId)
	require.Equal(t, 42, scope.ResponsibleUserId)
	require.Equal(t, 150000, scope.OrganizationQuota)
	require.Equal(t, "default", scope.AccountGroup)

	organization.Group = "org-x"
	require.Equal(t, "org-x", BuildPlaygroundOrganizationScope(organization, 42).AccountGroup)
	require.Nil(t, BuildPlaygroundOrganizationScope(nil, 42))
}
