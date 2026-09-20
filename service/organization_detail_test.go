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

func TestGetOrganizationDetailMemberActorCapabilities(t *testing.T) {
	setupServiceTestDB(t)
	member := createServiceTestUser(t, "detail-member", common.RoleCommonUser)
	org, err := CreateOrganization(member.Id, CreateOrganizationRequest{Name: "Detail Member Org"})
	require.NoError(t, err)

	detail, err := GetOrganizationDetailForUserWithAccessMode(member.Id, org.Id, OrganizationAccessModeWorkspace, true)

	require.NoError(t, err)
	require.Equal(t, org.Id, detail.Organization.Id)
	require.NotNil(t, detail.Member)
	require.NotNil(t, detail.Actor)
	require.Equal(t, "owner", detail.Actor.OrganizationRole)
	require.True(t, detail.Actor.Capabilities.CanCreateInvites)
	require.True(t, detail.Actor.Capabilities.CanDissolveOrganization)
	require.True(t, detail.Actor.Capabilities.ShowReturnOrganizationCenter)
}

func TestGetOrganizationDetailPlatformAdminActorCapabilities(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "detail-platform-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "detail-platform-creator", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Detail Platform Org"})
	require.NoError(t, err)

	detail, err := GetOrganizationDetailForUserWithAccessMode(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, true)

	require.NoError(t, err)
	require.Equal(t, org.Id, detail.Organization.Id)
	require.Nil(t, detail.Member)
	require.NotNil(t, detail.Actor)
	require.Equal(t, "platform_admin", detail.Actor.Role)
	require.True(t, detail.Actor.Capabilities.CanManageMembers)
	require.True(t, detail.Actor.Capabilities.CanAddMembersDirectly)
	require.False(t, detail.Actor.Capabilities.CanDissolveOrganization)
	require.False(t, detail.Actor.Capabilities.ShowReturnOrganizationCenter)
	require.False(t, detail.Actor.Capabilities.ShowReturnPersonalCenter)
}

func TestGetOrganizationDetailPlatformRootActorCapabilities(t *testing.T) {
	setupServiceTestDB(t)
	root := createServiceTestUser(t, "detail-platform-root", common.RoleRootUser)
	creator := createServiceTestUser(t, "detail-root-creator", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Detail Root Org"})
	require.NoError(t, err)

	detail, err := GetOrganizationDetailForUserWithAccessMode(root.Id, org.Id, OrganizationAccessModeAdmin, true)

	require.NoError(t, err)
	require.Equal(t, org.Id, detail.Organization.Id)
	require.Nil(t, detail.Member)
	require.NotNil(t, detail.Actor)
	require.Equal(t, "platform_root", detail.Actor.Role)
	require.True(t, detail.Actor.Capabilities.CanDissolveOrganization)
}

func TestGetOrganizationDetailPlatformRootMemberUsesExplicitAccessMode(t *testing.T) {
	setupServiceTestDB(t)
	root := createServiceTestUser(t, "detail-root-member", common.RoleRootUser)
	org, err := CreateOrganization(root.Id, CreateOrganizationRequest{Name: "Detail Root Member Org"})
	require.NoError(t, err)

	workspaceDetail, err := GetOrganizationDetailForUserWithAccessMode(root.Id, org.Id, OrganizationAccessModeWorkspace, true)

	require.NoError(t, err)
	require.NotNil(t, workspaceDetail.Member)
	require.Equal(t, OrganizationAccessModeWorkspace, workspaceDetail.Actor.AccessMode)
	require.Equal(t, model.OrganizationRoleOwner, workspaceDetail.Actor.Role)
	require.True(t, workspaceDetail.Actor.Capabilities.ShowReturnOrganizationCenter)

	adminDetail, err := GetOrganizationDetailForUserWithAccessMode(root.Id, org.Id, OrganizationAccessModeAdmin, true)
	require.NoError(t, err)
	require.Equal(t, OrganizationAccessModeAdmin, adminDetail.Actor.AccessMode)
	require.Equal(t, OrganizationPolicyRolePlatformRoot, adminDetail.Actor.Role)
	require.False(t, adminDetail.Actor.Capabilities.ShowReturnOrganizationCenter)
}
