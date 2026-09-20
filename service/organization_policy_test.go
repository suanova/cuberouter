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
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestEvaluateOrganizationPolicyCapabilityMatrix(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		input            OrganizationPolicyInput
		wantRole         string
		wantCapabilities []string
		denyCapabilities []string
	}{
		{
			name:     "owner workspace can manage lifecycle and owner transfer",
			input:    baseOrganizationPolicyInput(1, common.RoleCommonUser, model.OrganizationRoleMember, OrganizationAccessModeWorkspace),
			wantRole: model.OrganizationRoleOwner,
			wantCapabilities: []string{
				OrganizationCapabilityViewOrganization,
				OrganizationCapabilityUpdateOrganization,
				OrganizationCapabilityDisableOrganization,
				OrganizationCapabilityDissolveOrganization,
				OrganizationCapabilityManageMembers,
				OrganizationCapabilityTransferOwner,
				OrganizationCapabilityManageInvitations,
				OrganizationCapabilityManageOrganizationTokens,
				OrganizationCapabilityViewOrganizationAuditLogs,
			},
			denyCapabilities: []string{OrganizationCapabilityAdjustOrganizationQuota},
		},
		{
			name:     "organization admin workspace cannot dissolve or transfer owner",
			input:    baseOrganizationPolicyInput(2, common.RoleCommonUser, model.OrganizationRoleAdmin, OrganizationAccessModeWorkspace),
			wantRole: model.OrganizationRoleAdmin,
			wantCapabilities: []string{
				OrganizationCapabilityViewOrganization,
				OrganizationCapabilityUpdateOrganization,
				OrganizationCapabilityManageMembers,
				OrganizationCapabilityManageInvitations,
				OrganizationCapabilityManageOrganizationTokens,
			},
			denyCapabilities: []string{
				OrganizationCapabilityDissolveOrganization,
				OrganizationCapabilityTransferOwner,
				OrganizationCapabilityAdjustOrganizationQuota,
			},
		},
		{
			name:     "member workspace is limited",
			input:    baseOrganizationPolicyInput(3, common.RoleCommonUser, model.OrganizationRoleMember, OrganizationAccessModeWorkspace),
			wantRole: model.OrganizationRoleMember,
			wantCapabilities: []string{
				OrganizationCapabilityViewOrganization,
				OrganizationCapabilityViewMembersLimited,
				OrganizationCapabilityViewOrganizationTokens,
				OrganizationCapabilityViewOrganizationLogs,
				OrganizationCapabilityViewOrganizationUsage,
			},
			denyCapabilities: []string{
				OrganizationCapabilityUpdateOrganization,
				OrganizationCapabilityManageMembers,
				OrganizationCapabilityManageInvitations,
				OrganizationCapabilityManageOrganizationTokens,
				OrganizationCapabilityViewOrganizationAuditLogs,
			},
		},
		{
			name:     "platform admin admin mode cannot manage owner or dissolve",
			input:    baseOrganizationPolicyInput(4, common.RoleAdminUser, "", OrganizationAccessModeAdmin),
			wantRole: OrganizationPolicyRolePlatformAdmin,
			wantCapabilities: []string{
				OrganizationCapabilityViewOrganization,
				OrganizationCapabilityUpdateOrganization,
				OrganizationCapabilityDisableOrganization,
				OrganizationCapabilityEnableOrganization,
				OrganizationCapabilityManageMembers,
				OrganizationCapabilityAdjustOrganizationQuota,
			},
			denyCapabilities: []string{
				OrganizationCapabilityTransferOwner,
				OrganizationCapabilityDissolveOrganization,
			},
		},
		{
			name:     "platform root admin mode can transfer owner and dissolve",
			input:    baseOrganizationPolicyInput(5, common.RoleRootUser, "", OrganizationAccessModeAdmin),
			wantRole: OrganizationPolicyRolePlatformRoot,
			wantCapabilities: []string{
				OrganizationCapabilityViewOrganization,
				OrganizationCapabilityManageMembers,
				OrganizationCapabilityTransferOwner,
				OrganizationCapabilityDissolveOrganization,
				OrganizationCapabilityAdjustOrganizationQuota,
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			decision := EvaluateOrganizationPolicy(tc.input)

			require.True(t, decision.Allowed)
			require.Equal(t, tc.wantRole, decision.Role)
			for _, capability := range tc.wantCapabilities {
				require.Truef(t, decision.HasCapability(capability), "expected capability %s", capability)
			}
			for _, capability := range tc.denyCapabilities {
				require.Falsef(t, decision.HasCapability(capability), "unexpected capability %s", capability)
			}
		})
	}
}

func TestOrganizationPolicyAccessModeMatrix(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		platformRole int
		memberRole   string
		accessMode   string
		status       string
		wantAllowed  bool
		wantRole     string
		wantReadOnly bool
		wantCode     types.ErrorCode
	}{
		{name: "workspace rejects platform admin non-member", platformRole: common.RoleAdminUser, accessMode: OrganizationAccessModeWorkspace, status: model.OrganizationStatusActive, wantRole: "", wantCode: types.ErrorCodeOrganizationAccessDenied},
		{name: "workspace uses member role for platform admin member", platformRole: common.RoleAdminUser, memberRole: model.OrganizationRoleMember, accessMode: OrganizationAccessModeWorkspace, status: model.OrganizationStatusActive, wantAllowed: true, wantRole: model.OrganizationRoleMember},
		{name: "workspace uses admin role for platform root organization admin", platformRole: common.RoleRootUser, memberRole: model.OrganizationRoleAdmin, accessMode: OrganizationAccessModeWorkspace, status: model.OrganizationStatusActive, wantAllowed: true, wantRole: model.OrganizationRoleAdmin},
		{name: "workspace uses owner role for platform admin owner", platformRole: common.RoleAdminUser, memberRole: model.OrganizationRoleOwner, accessMode: OrganizationAccessModeWorkspace, status: model.OrganizationStatusActive, wantAllowed: true, wantRole: model.OrganizationRoleOwner},
		{name: "management rejects platform root non-member", platformRole: common.RoleRootUser, accessMode: OrganizationAccessModeManagement, status: model.OrganizationStatusActive, wantRole: "", wantCode: types.ErrorCodeOrganizationAccessDenied},
		{name: "management rejects platform admin member", platformRole: common.RoleAdminUser, memberRole: model.OrganizationRoleMember, accessMode: OrganizationAccessModeManagement, status: model.OrganizationStatusActive, wantRole: model.OrganizationRoleMember, wantCode: types.ErrorCodeOrganizationAccessDenied},
		{name: "management allows platform root organization admin as organization admin", platformRole: common.RoleRootUser, memberRole: model.OrganizationRoleAdmin, accessMode: OrganizationAccessModeManagement, status: model.OrganizationStatusActive, wantAllowed: true, wantRole: model.OrganizationRoleAdmin},
		{name: "management allows platform admin owner as organization owner", platformRole: common.RoleAdminUser, memberRole: model.OrganizationRoleOwner, accessMode: OrganizationAccessModeManagement, status: model.OrganizationStatusActive, wantAllowed: true, wantRole: model.OrganizationRoleOwner},
		{name: "read only rejects disabled platform admin non-member", platformRole: common.RoleAdminUser, accessMode: OrganizationAccessModeReadOnly, status: model.OrganizationStatusDisabled, wantRole: "", wantCode: types.ErrorCodeOrganizationDisabled},
		{name: "read only rejects disabled platform root member", platformRole: common.RoleRootUser, memberRole: model.OrganizationRoleMember, accessMode: OrganizationAccessModeReadOnly, status: model.OrganizationStatusDisabled, wantRole: model.OrganizationRoleMember, wantCode: types.ErrorCodeOrganizationDisabled},
		{name: "read only allows disabled platform admin organization admin", platformRole: common.RoleAdminUser, memberRole: model.OrganizationRoleAdmin, accessMode: OrganizationAccessModeReadOnly, status: model.OrganizationStatusDisabled, wantAllowed: true, wantRole: model.OrganizationRoleAdmin, wantReadOnly: true},
		{name: "read only allows disabled platform root owner", platformRole: common.RoleRootUser, memberRole: model.OrganizationRoleOwner, accessMode: OrganizationAccessModeReadOnly, status: model.OrganizationStatusDisabled, wantAllowed: true, wantRole: model.OrganizationRoleOwner, wantReadOnly: true},
		{name: "read only rejects active member", platformRole: common.RoleCommonUser, memberRole: model.OrganizationRoleMember, accessMode: OrganizationAccessModeReadOnly, status: model.OrganizationStatusActive, wantRole: model.OrganizationRoleMember, wantCode: types.ErrorCodeOrganizationAccessDenied},
		{name: "read only rejects active organization admin", platformRole: common.RoleCommonUser, memberRole: model.OrganizationRoleAdmin, accessMode: OrganizationAccessModeReadOnly, status: model.OrganizationStatusActive, wantRole: model.OrganizationRoleAdmin, wantCode: types.ErrorCodeOrganizationAccessDenied},
		{name: "read only rejects active owner", platformRole: common.RoleCommonUser, memberRole: model.OrganizationRoleOwner, accessMode: OrganizationAccessModeReadOnly, status: model.OrganizationStatusActive, wantRole: model.OrganizationRoleOwner, wantCode: types.ErrorCodeOrganizationAccessDenied},
		{name: "empty access mode is rejected without workspace fallback", platformRole: common.RoleAdminUser, memberRole: model.OrganizationRoleOwner, accessMode: "", status: model.OrganizationStatusActive, wantRole: model.OrganizationRoleOwner, wantCode: types.ErrorCodeOrganizationAccessDenied},
		{name: "admin rejects ordinary organization owner", platformRole: common.RoleCommonUser, memberRole: model.OrganizationRoleOwner, accessMode: OrganizationAccessModeAdmin, status: model.OrganizationStatusActive, wantRole: "", wantCode: types.ErrorCodeOrganizationAccessDenied},
		{name: "admin allows platform admin non-member", platformRole: common.RoleAdminUser, accessMode: OrganizationAccessModeAdmin, status: model.OrganizationStatusActive, wantAllowed: true, wantRole: OrganizationPolicyRolePlatformAdmin},
		{name: "admin keeps platform root role for member", platformRole: common.RoleRootUser, memberRole: model.OrganizationRoleMember, accessMode: OrganizationAccessModeAdmin, status: model.OrganizationStatusActive, wantAllowed: true, wantRole: OrganizationPolicyRolePlatformRoot},
		{name: "admin keeps platform admin role for organization admin", platformRole: common.RoleAdminUser, memberRole: model.OrganizationRoleAdmin, accessMode: OrganizationAccessModeAdmin, status: model.OrganizationStatusActive, wantAllowed: true, wantRole: OrganizationPolicyRolePlatformAdmin},
		{name: "admin keeps platform root role for owner", platformRole: common.RoleRootUser, memberRole: model.OrganizationRoleOwner, accessMode: OrganizationAccessModeAdmin, status: model.OrganizationStatusActive, wantAllowed: true, wantRole: OrganizationPolicyRolePlatformRoot},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := baseOrganizationPolicyInput(100+i, tc.platformRole, tc.memberRole, tc.accessMode)
			input.Organization.Status = tc.status

			decision := EvaluateOrganizationPolicy(input)

			require.Equal(t, tc.wantAllowed, decision.Allowed)
			require.Equal(t, tc.wantRole, decision.Role)
			require.Equal(t, tc.wantReadOnly, decision.ReadOnly)
			require.Equal(t, tc.wantCode, decision.Code)
		})
	}
}

func TestOrganizationPolicyAccessModeCapabilities(t *testing.T) {
	t.Parallel()

	t.Run("workspace requires active organization", func(t *testing.T) {
		t.Parallel()
		input := baseOrganizationPolicyInput(11, common.RoleCommonUser, model.OrganizationRoleOwner, OrganizationAccessModeWorkspace)
		input.Organization.Status = model.OrganizationStatusDisabled

		decision := EvaluateOrganizationPolicy(input)

		require.False(t, decision.Allowed)
		require.Equal(t, types.ErrorCodeOrganizationDisabled, decision.Code)
	})

	t.Run("disabled read only allows owner view without write capabilities", func(t *testing.T) {
		t.Parallel()
		input := baseOrganizationPolicyInput(12, common.RoleCommonUser, model.OrganizationRoleOwner, OrganizationAccessModeReadOnly)
		input.Organization.Status = model.OrganizationStatusDisabled

		decision := EvaluateOrganizationPolicy(input)

		require.True(t, decision.Allowed)
		require.True(t, decision.ReadOnly)
		require.True(t, decision.HasCapability(OrganizationCapabilityViewReadOnlyOrganization))
		require.True(t, decision.HasCapability(OrganizationCapabilityViewOrganization))
		require.False(t, decision.HasCapability(OrganizationCapabilityUpdateOrganization))
		require.False(t, decision.HasCapability(OrganizationCapabilityManageMembers))
	})

	t.Run("dissolved admin mode returns platform read only capabilities", func(t *testing.T) {
		t.Parallel()
		input := baseOrganizationPolicyInput(15, common.RoleAdminUser, "", OrganizationAccessModeAdmin)
		input.Organization.Status = model.OrganizationStatusDissolved

		decision := EvaluateOrganizationPolicy(input)

		require.True(t, decision.Allowed)
		require.True(t, decision.ReadOnly)
		require.Equal(t, types.ErrorCodeOrganizationDissolved, decision.Code)
		require.Equal(t, "organization dissolved", decision.Message)
		require.True(t, decision.HasCapability(OrganizationCapabilityViewReadOnlyOrganization))
		require.False(t, decision.HasCapability(OrganizationCapabilityUpdateOrganization))
		require.False(t, decision.HasCapability(OrganizationCapabilityAdjustOrganizationQuota))
	})

	t.Run("platform disable source blocks owner self enable", func(t *testing.T) {
		t.Parallel()
		input := baseOrganizationPolicyInput(16, common.RoleCommonUser, model.OrganizationRoleOwner, OrganizationAccessModeManagement)
		input.Organization.Status = model.OrganizationStatusDisabled
		input.DisableState.ActiveSources = []string{model.OrganizationDisableSourcePlatform}
		input.DisableState.CanSelfEnable = false

		decision := EvaluateOrganizationPolicy(input)

		require.True(t, decision.Allowed)
		require.False(t, decision.HasCapability(OrganizationCapabilityEnableOrganization))
	})
}

func TestEvaluateOrganizationPolicyResourceVisibility(t *testing.T) {
	t.Parallel()

	t.Run("member can view and manage own private token", func(t *testing.T) {
		t.Parallel()
		input := baseOrganizationPolicyInput(21, common.RoleCommonUser, model.OrganizationRoleMember, OrganizationAccessModeWorkspace)
		input.Resource = OrganizationPolicyResource{Type: OrganizationPolicyResourceToken, ResponsibleUserId: 21, Visibility: model.TokenVisibilityPrivate}

		decision := EvaluateOrganizationPolicy(input)

		require.True(t, decision.Allowed)
		require.True(t, decision.HasCapability(OrganizationCapabilityViewResource))
		require.True(t, decision.HasCapability(OrganizationCapabilityManageResource))
	})

	t.Run("member cannot view another member private token", func(t *testing.T) {
		t.Parallel()
		input := baseOrganizationPolicyInput(22, common.RoleCommonUser, model.OrganizationRoleMember, OrganizationAccessModeWorkspace)
		input.Resource = OrganizationPolicyResource{Type: OrganizationPolicyResourceToken, ResponsibleUserId: 23, Visibility: model.TokenVisibilityPrivate}

		decision := EvaluateOrganizationPolicy(input)

		require.True(t, decision.Allowed)
		require.False(t, decision.HasCapability(OrganizationCapabilityViewResource))
		require.False(t, decision.HasCapability(OrganizationCapabilityManageResource))
	})

	t.Run("member can view public token regardless of availability", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			name    string
			status  int
			canView bool
		}{
			{"enabled", common.TokenStatusEnabled, true},
			{"disabled", common.TokenStatusDisabled, true},
			{"expired", common.TokenStatusExpired, true},
			{"exhausted", common.TokenStatusExhausted, true},
		}
		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				input := baseOrganizationPolicyInput(24, common.RoleCommonUser, model.OrganizationRoleMember, OrganizationAccessModeWorkspace)
				input.Resource = OrganizationPolicyResource{Type: OrganizationPolicyResourceToken, ResponsibleUserId: 25, Visibility: model.TokenVisibilityPublic, Status: tc.status}

				decision := EvaluateOrganizationPolicy(input)

				require.True(t, decision.Allowed)
				require.Equal(t, tc.canView, decision.HasCapability(OrganizationCapabilityViewResource))
				require.False(t, decision.HasCapability(OrganizationCapabilityManageResource))
			})
		}
	})
}

func TestEvaluateOrganizationResourcePolicyForPlatformMemberDoesNotBypassReadOnlyPolicy(t *testing.T) {
	t.Parallel()

	for _, status := range []string{model.OrganizationStatusDisabled, model.OrganizationStatusDissolved} {
		status := status
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			organization := &model.Organization{Id: 100, Status: status, OwnerUserId: 1}
			member := &model.OrganizationMember{OrganizationId: organization.Id, UserId: 30, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}
			actor := &OrganizationActorContext{
				UserId:               member.UserId,
				OrganizationId:       organization.Id,
				Organization:         organization,
				Member:               member,
				AccessMode:           OrganizationAccessModeReadOnly,
				Role:                 OrganizationPolicyRolePlatformAdmin,
				OrganizationRole:     model.OrganizationRoleMember,
				PlatformRole:         OrganizationPolicyRolePlatformAdmin,
				IsOrganizationMember: true,
				IsPlatformAdmin:      true,
				ReadOnly:             true,
			}

			decision := EvaluateOrganizationResourcePolicyForActor(actor, OrganizationPolicyResource{
				Type:              OrganizationPolicyResourceToken,
				ResponsibleUserId: 31,
				Visibility:        model.TokenVisibilityPrivate,
				Status:            common.TokenStatusDisabled,
			})

			require.False(t, decision.Allowed)
			require.Equal(t, model.OrganizationRoleMember, decision.Role)
			require.False(t, decision.HasCapability(OrganizationCapabilityViewResource))
			require.False(t, decision.HasCapability(OrganizationCapabilityManageResource))
		})
	}
}

func TestEvaluateOrganizationResourcePolicyForActivePlatformMemberUsesActorAccessMode(t *testing.T) {
	t.Parallel()
	organization := &model.Organization{Id: 101, Status: model.OrganizationStatusActive, OwnerUserId: 1}
	member := &model.OrganizationMember{OrganizationId: organization.Id, UserId: 30, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}
	workspaceActor := &OrganizationActorContext{
		UserId:               member.UserId,
		OrganizationId:       organization.Id,
		Organization:         organization,
		Member:               member,
		AccessMode:           OrganizationAccessModeWorkspace,
		Role:                 model.OrganizationRoleMember,
		OrganizationRole:     model.OrganizationRoleMember,
		PlatformRole:         OrganizationPolicyRolePlatformAdmin,
		IsOrganizationMember: true,
		IsPlatformAdmin:      true,
	}
	resource := OrganizationPolicyResource{
		Type:              OrganizationPolicyResourceToken,
		OwnerUserId:       31,
		ResponsibleUserId: 31,
		Visibility:        model.TokenVisibilityPrivate,
		Status:            common.TokenStatusEnabled,
	}

	workspaceDecision := EvaluateOrganizationResourcePolicyForActor(workspaceActor, resource)
	require.True(t, workspaceDecision.Allowed)
	require.Equal(t, OrganizationAccessModeWorkspace, workspaceDecision.AccessMode)
	require.Equal(t, model.OrganizationRoleMember, workspaceDecision.Role)
	require.False(t, workspaceDecision.HasCapability(OrganizationCapabilityViewResource))
	require.False(t, workspaceDecision.HasCapability(OrganizationCapabilityManageResource))

	adminActor := *workspaceActor
	adminActor.AccessMode = OrganizationAccessModeAdmin
	adminDecision := EvaluateOrganizationResourcePolicyForActor(&adminActor, resource)
	require.True(t, adminDecision.Allowed)
	require.Equal(t, OrganizationAccessModeAdmin, adminDecision.AccessMode)
	require.Equal(t, OrganizationPolicyRolePlatformAdmin, adminDecision.Role)
	require.True(t, adminDecision.HasCapability(OrganizationCapabilityViewResource))
	require.True(t, adminDecision.HasCapability(OrganizationCapabilityManageResource))
}

func TestEvaluateOrganizationPolicyUsesOwnerUserIdAsOwnerSourceOfTruth(t *testing.T) {
	t.Parallel()
	input := baseOrganizationPolicyInput(2, common.RoleCommonUser, model.OrganizationRoleOwner, OrganizationAccessModeWorkspace)
	input.Organization.OwnerUserId = 1

	decision := EvaluateOrganizationPolicy(input)

	require.True(t, decision.Allowed)
	require.Equal(t, model.OrganizationRoleMember, decision.Role)
	require.False(t, decision.HasCapability(OrganizationCapabilityTransferOwner))
	require.False(t, decision.HasCapability(OrganizationCapabilityDissolveOrganization))
}

func baseOrganizationPolicyInput(userId int, platformRole int, memberRole string, accessMode string) OrganizationPolicyInput {
	input := OrganizationPolicyInput{
		CurrentUser: OrganizationPolicyUser{Id: userId, PlatformRole: platformRole},
		Organization: OrganizationPolicyOrganization{
			Id:          100,
			Status:      model.OrganizationStatusActive,
			OwnerUserId: 1,
		},
		DisableState: OrganizationPolicyDisableState{CanSelfEnable: true},
		AccessMode:   accessMode,
	}
	if memberRole != "" {
		input.Member = &OrganizationPolicyMember{UserId: userId, Role: memberRole, Status: model.OrganizationMemberStatusActive}
		if memberRole == model.OrganizationRoleOwner {
			input.Organization.OwnerUserId = userId
		}
	}
	return input
}
