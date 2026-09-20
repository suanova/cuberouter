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
	"github.com/stretchr/testify/require"
)

func TestOrganizationActorContextMemberCapabilities(t *testing.T) {
	setupServiceTestDB(t)
	creator := createServiceTestUser(t, "actor-member-creator", common.RoleCommonUser)
	memberUser := createServiceTestUser(t, "actor-member-user", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Actor Member Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: org.Id, UserId: memberUser.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	actor, err := GetOrganizationActorContextForAccessMode(memberUser.Id, org.Id, OrganizationAccessModeWorkspace, true)

	require.NoError(t, err)
	require.Equal(t, memberUser.Id, actor.UserId)
	require.Equal(t, org.Id, actor.OrganizationId)
	require.True(t, actor.IsOrganizationMember)
	require.False(t, actor.IsOrganizationAdmin)
	require.False(t, actor.IsPlatformAdmin)
	require.False(t, actor.IsPlatformRoot)
	require.Equal(t, model.OrganizationRoleMember, actor.OperatorRoleForAudit)
	require.True(t, actor.Capabilities.CanViewOrganization)
	require.False(t, actor.Capabilities.CanViewOrganizationWideData)
	require.True(t, actor.Capabilities.CanViewMembersLimited)
	require.False(t, actor.Capabilities.CanManageMembers)
	require.True(t, actor.Capabilities.CanViewOrganizationTokens)
	require.True(t, actor.Capabilities.CanViewOrganizationLogs)
	require.True(t, actor.Capabilities.CanViewOrganizationUsage)
	require.False(t, actor.Capabilities.CanUpdateOrganization)
	require.False(t, actor.Capabilities.CanDisableOrganization)
	require.False(t, actor.Capabilities.CanEnableOrganization)
	require.False(t, actor.Capabilities.CanAddMembersDirectly)
	require.True(t, actor.Capabilities.CanExitOrganization)
	require.False(t, actor.Capabilities.CanViewInvites)
	require.False(t, actor.Capabilities.CanCreateInvites)
	require.False(t, actor.Capabilities.CanRevokeInvites)
	require.False(t, actor.Capabilities.CanManageAllTokens)
	require.False(t, actor.Capabilities.CanModifyOrganizationGroup)
	require.False(t, actor.Capabilities.CanDissolveOrganization)
	require.False(t, actor.Capabilities.CanViewAudit)
	require.False(t, actor.Capabilities.CanViewOrganizationBillingSummary)
	require.True(t, actor.Capabilities.ShowReturnOrganizationCenter)
	require.True(t, actor.Capabilities.ShowReturnPersonalCenter)
}

func TestOrganizationActorContextOrganizationAdminCapabilities(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "actor-org-admin-owner", common.RoleCommonUser)
	admin := createServiceTestUser(t, "actor-org-admin", common.RoleCommonUser)
	org, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Actor Org Admin Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: org.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)

	actor, err := GetOrganizationActorContextForAccessMode(admin.Id, org.Id, OrganizationAccessModeWorkspace, true)

	require.NoError(t, err)
	require.True(t, actor.IsOrganizationMember)
	require.True(t, actor.IsOrganizationAdmin)
	require.False(t, actor.IsPlatformAdmin)
	require.False(t, actor.IsPlatformRoot)
	require.Equal(t, model.OrganizationRoleAdmin, actor.OperatorRoleForAudit)
	require.True(t, actor.Capabilities.CanViewOrganizationWideData)
	require.True(t, actor.Capabilities.CanViewOrganizationTokens)
	require.True(t, actor.Capabilities.CanViewOrganizationLogs)
	require.True(t, actor.Capabilities.CanViewOrganizationUsage)
	require.True(t, actor.Capabilities.CanManageMembers)
	require.True(t, actor.Capabilities.CanUpdateOrganization)
	require.True(t, actor.Capabilities.CanDisableOrganization)
	require.False(t, actor.Capabilities.CanEnableOrganization)
	require.False(t, actor.Capabilities.CanAddMembersDirectly)
	require.True(t, actor.Capabilities.CanExitOrganization)
	require.True(t, actor.Capabilities.CanViewInvites)
	require.True(t, actor.Capabilities.CanCreateInvites)
	require.True(t, actor.Capabilities.CanRevokeInvites)
	require.True(t, actor.Capabilities.CanManageAllTokens)
	require.False(t, actor.Capabilities.CanModifyOrganizationGroup)
	require.False(t, actor.Capabilities.CanDissolveOrganization)
	require.True(t, actor.Capabilities.CanViewAudit)
	require.True(t, actor.Capabilities.CanViewOrganizationBillingSummary)
	require.True(t, actor.Capabilities.ShowReturnOrganizationCenter)
	require.True(t, actor.Capabilities.ShowReturnPersonalCenter)
}

func TestOrganizationActorContextPlatformAdminCapabilities(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "actor-platform-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "actor-platform-admin-creator", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Actor Platform Admin Org"})
	require.NoError(t, err)

	actor, err := GetOrganizationActorContextForAccessMode(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, true)

	require.NoError(t, err)
	require.False(t, actor.IsOrganizationMember)
	require.False(t, actor.IsOrganizationAdmin)
	require.True(t, actor.IsPlatformAdmin)
	require.False(t, actor.IsPlatformRoot)
	require.Equal(t, organizationAuditOperatorRolePlatformAdmin, actor.OperatorRoleForAudit)
	require.True(t, actor.Capabilities.CanViewOrganizationWideData)
	require.True(t, actor.Capabilities.CanViewOrganizationTokens)
	require.True(t, actor.Capabilities.CanViewOrganizationLogs)
	require.True(t, actor.Capabilities.CanViewOrganizationUsage)
	require.True(t, actor.Capabilities.CanManageMembers)
	require.True(t, actor.Capabilities.CanUpdateOrganization)
	require.True(t, actor.Capabilities.CanDisableOrganization)
	require.True(t, actor.Capabilities.CanEnableOrganization)
	require.True(t, actor.Capabilities.CanAddMembersDirectly)
	require.False(t, actor.Capabilities.CanExitOrganization)
	require.True(t, actor.Capabilities.CanViewInvites)
	require.False(t, actor.Capabilities.CanCreateInvites)
	require.True(t, actor.Capabilities.CanRevokeInvites)
	require.True(t, actor.Capabilities.CanManageAllTokens)
	require.True(t, actor.Capabilities.CanModifyOrganizationGroup)
	require.False(t, actor.Capabilities.CanDissolveOrganization)
	require.True(t, actor.Capabilities.CanViewAudit)
	require.True(t, actor.Capabilities.CanViewOrganizationBillingSummary)
	require.False(t, actor.Capabilities.ShowReturnOrganizationCenter)
	require.False(t, actor.Capabilities.ShowReturnPersonalCenter)
}

func TestOrganizationActorContextPlatformRootCapabilities(t *testing.T) {
	setupServiceTestDB(t)
	root := createServiceTestUser(t, "actor-platform-root", common.RoleRootUser)
	creator := createServiceTestUser(t, "actor-platform-root-creator", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Actor Platform Root Org"})
	require.NoError(t, err)

	actor, err := GetOrganizationActorContextForAccessMode(root.Id, org.Id, OrganizationAccessModeAdmin, true)

	require.NoError(t, err)
	require.False(t, actor.IsOrganizationMember)
	require.False(t, actor.IsOrganizationAdmin)
	require.True(t, actor.IsPlatformAdmin)
	require.True(t, actor.IsPlatformRoot)
	require.Equal(t, organizationAuditOperatorRolePlatformRoot, actor.OperatorRoleForAudit)
	require.True(t, actor.Capabilities.CanViewOrganizationWideData)
	require.True(t, actor.Capabilities.CanViewOrganizationTokens)
	require.True(t, actor.Capabilities.CanViewOrganizationLogs)
	require.True(t, actor.Capabilities.CanViewOrganizationUsage)
	require.True(t, actor.Capabilities.CanManageMembers)
	require.True(t, actor.Capabilities.CanUpdateOrganization)
	require.True(t, actor.Capabilities.CanDisableOrganization)
	require.True(t, actor.Capabilities.CanEnableOrganization)
	require.True(t, actor.Capabilities.CanAddMembersDirectly)
	require.False(t, actor.Capabilities.CanExitOrganization)
	require.True(t, actor.Capabilities.CanViewInvites)
	require.False(t, actor.Capabilities.CanCreateInvites)
	require.True(t, actor.Capabilities.CanRevokeInvites)
	require.True(t, actor.Capabilities.CanManageAllTokens)
	require.True(t, actor.Capabilities.CanModifyOrganizationGroup)
	require.True(t, actor.Capabilities.CanDissolveOrganization)
	require.True(t, actor.Capabilities.CanViewAudit)
	require.True(t, actor.Capabilities.CanViewOrganizationBillingSummary)
	require.False(t, actor.Capabilities.ShowReturnOrganizationCenter)
	require.False(t, actor.Capabilities.ShowReturnPersonalCenter)
}

func TestOrganizationActorContextPlatformMemberUsesExplicitAccessMode(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "actor-platform-member-owner", common.RoleCommonUser)
	platformAdmin := createServiceTestUser(t, "actor-platform-member-admin", common.RoleAdminUser)
	org, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Actor Platform Member Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: org.Id, UserId: platformAdmin.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	workspaceActor, err := GetOrganizationActorContextForAccessMode(platformAdmin.Id, org.Id, OrganizationAccessModeWorkspace, true)
	require.NoError(t, err)
	require.Equal(t, OrganizationAccessModeWorkspace, workspaceActor.AccessMode)
	require.Equal(t, model.OrganizationRoleMember, workspaceActor.Role)
	require.Equal(t, OrganizationPolicyRolePlatformAdmin, workspaceActor.PlatformRole)
	require.False(t, workspaceActor.Capabilities.CanManageMembers)

	adminActor, err := GetOrganizationActorContextForAccessMode(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, true)
	require.NoError(t, err)
	require.Equal(t, OrganizationAccessModeAdmin, adminActor.AccessMode)
	require.Equal(t, OrganizationPolicyRolePlatformAdmin, adminActor.Role)
	require.Equal(t, OrganizationPolicyRolePlatformAdmin, adminActor.PlatformRole)
	require.True(t, adminActor.Capabilities.CanManageMembers)
}

func TestOrganizationActorContextDisabledOrganizationIsReadOnly(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "actor-disabled-owner", common.RoleCommonUser)
	admin := createServiceTestUser(t, "actor-disabled-admin", common.RoleCommonUser)
	org, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Actor Disabled Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: org.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", org.Id).Update("status", model.OrganizationStatusDisabled).Error)

	actor, err := GetOrganizationActorContextForAccessMode(admin.Id, org.Id, OrganizationAccessModeReadOnly, true)

	require.NoError(t, err)
	require.True(t, actor.ReadOnly)
	require.False(t, actor.Capabilities.CanExitOrganization)
	require.False(t, actor.Capabilities.CanManageMembers)
	require.False(t, actor.Capabilities.CanManageAllTokens)
}

func TestOrganizationActorContextDissolvedPlatformAdminIsReadOnly(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "actor-dissolved-platform-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "actor-dissolved-creator", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Actor Dissolved Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", org.Id).Update("status", model.OrganizationStatusDissolved).Error)

	actor, err := GetOrganizationActorContextForAccessMode(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, true)

	require.NoError(t, err)
	require.True(t, actor.ReadOnly)
	require.True(t, actor.Capabilities.CanViewOrganization)
	require.False(t, actor.Capabilities.CanManageMembers)
	require.False(t, actor.Capabilities.CanDissolveOrganization)
}

func TestOrganizationActorContextDisabledPlatformAdminUsesExplicitAccessMode(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "actor-disabled-platform-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "actor-disabled-platform-creator", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Actor Disabled Platform Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", org.Id).Update("status", model.OrganizationStatusDisabled).Error)

	_, err = GetOrganizationActorContextForAccessMode(platformAdmin.Id, org.Id, OrganizationAccessModeReadOnly, true)
	require.ErrorContains(t, err, "organization disabled")

	actor, err := GetOrganizationActorContextForAccessMode(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, true)
	require.NoError(t, err)
	require.Equal(t, OrganizationAccessModeAdmin, actor.AccessMode)
	require.False(t, actor.ReadOnly)
	require.True(t, actor.Capabilities.CanViewOrganization)
	require.True(t, actor.Capabilities.CanManageMembers)
	require.False(t, actor.Capabilities.CanDissolveOrganization)
}

func TestOrganizationActorContextPlatformAdminMemberAdminAccessModeAcrossStates(t *testing.T) {
	testCases := []struct {
		name             string
		status           string
		readOnly         bool
		canManageMembers bool
	}{
		{name: "disabled", status: model.OrganizationStatusDisabled, readOnly: false, canManageMembers: true},
		{name: "dissolved", status: model.OrganizationStatusDissolved, readOnly: true, canManageMembers: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setupServiceTestDB(t)
			owner := createServiceTestUser(t, "actor-platform-state-owner-"+tc.name, common.RoleCommonUser)
			platformAdmin := createServiceTestUser(t, "actor-platform-state-member-"+tc.name, common.RoleAdminUser)
			org, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Actor Platform State " + tc.name})
			require.NoError(t, err)
			require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: org.Id, UserId: platformAdmin.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
			require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", org.Id).Update("status", tc.status).Error)

			actor, err := GetOrganizationActorContextForAccessMode(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, true)

			require.NoError(t, err)
			require.NotNil(t, actor.Member)
			require.Equal(t, model.OrganizationRoleMember, actor.OrganizationRole)
			require.Equal(t, OrganizationPolicyRolePlatformAdmin, actor.Role)
			require.Equal(t, OrganizationPolicyRolePlatformAdmin, actor.PlatformRole)
			require.Equal(t, OrganizationPolicyRolePlatformAdmin, actor.OperatorRoleForAudit)
			require.True(t, actor.IsOrganizationMember)
			require.True(t, actor.IsPlatformAdmin)
			require.Equal(t, tc.readOnly, actor.ReadOnly)
			require.Equal(t, tc.canManageMembers, actor.Capabilities.CanManageMembers)
		})
	}
}

func TestOrganizationActorContextRejectsUnknownAccessMode(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "actor-unknown-mode-owner", common.RoleCommonUser)
	org, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Actor Unknown Mode Org"})
	require.NoError(t, err)

	for _, accessMode := range []string{"", "unknown"} {
		_, err = GetOrganizationActorContextForAccessMode(owner.Id, org.Id, accessMode, true)
		require.ErrorContains(t, err, "organization access denied")
	}
}
