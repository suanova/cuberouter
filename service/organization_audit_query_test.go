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

func createOrganizationAuditFixture(t *testing.T) (model.User, model.User, model.User, *model.Organization, *model.Organization) {
	t.Helper()
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "audit-platform-admin", common.RoleAdminUser)
	orgAdmin := createServiceTestUser(t, "audit-org-admin", common.RoleCommonUser)
	member := createServiceTestUser(t, "audit-member", common.RoleCommonUser)
	organization, err := CreateOrganization(orgAdmin.Id, CreateOrganizationRequest{Name: "Audit Org"})
	require.NoError(t, err)
	otherOrg, err := CreateOrganization(platformAdmin.Id, CreateOrganizationRequest{Name: "Other Audit Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Where("organization_id = ?", organization.Id).Delete(&model.OrganizationAuditLog{}).Error)
	require.NoError(t, model.DB.Where("organization_id = ?", otherOrg.Id).Delete(&model.OrganizationAuditLog{}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationAuditLog{OrganizationId: organization.Id, OrganizationName: organization.Name, OrganizationSlug: organization.Slug, OperatorUserId: orgAdmin.Id, ActionType: "organization.test", TargetType: "organization", TargetId: organization.Id, CreatedAt: 100}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationAuditLog{OrganizationId: otherOrg.Id, OrganizationName: otherOrg.Name, OrganizationSlug: otherOrg.Slug, OperatorUserId: platformAdmin.Id, ActionType: "organization.other", TargetType: "organization", TargetId: otherOrg.Id, CreatedAt: 200}).Error)
	return platformAdmin, orgAdmin, member, organization, otherOrg
}

func TestOrganizationAuditMemberCannotRead(t *testing.T) {
	_, _, member, organization, _ := createOrganizationAuditFixture(t)

	_, _, err := ListOrganizationAuditLogs(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationAuditQueryRequest{Limit: 20})

	require.ErrorContains(t, err, "permission denied")
}

func TestOrganizationAuditOrgAdminOnlyReadsOwnOrganization(t *testing.T) {
	_, orgAdmin, _, organization, _ := createOrganizationAuditFixture(t)

	logs, total, err := ListOrganizationAuditLogs(orgAdmin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationAuditQueryRequest{Limit: 20})

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, organization.Id, logs[0].OrganizationId)
}

func TestOrganizationAuditQueryKeepsStableMaskedTokenIdentityAfterDeletion(t *testing.T) {
	setupServiceTestDB(t)
	orgAdmin := createServiceTestUser(t, "audit-token-full-key-admin", common.RoleCommonUser)
	responsible := createServiceTestUser(t, "audit-token-full-key-owner", common.RoleCommonUser)
	organization, err := CreateOrganization(orgAdmin.Id, CreateOrganizationRequest{Name: "Audit Token Full Key Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: responsible.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	token, err := CreateOrganizationToken(orgAdmin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "copyable-key", ResponsibleUserId: responsible.Id, ExpiredTime: -1})
	require.NoError(t, err)

	logs, _, err := ListOrganizationAuditLogs(orgAdmin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationAuditQueryRequest{TargetType: "token", TargetId: token.Id, Limit: 20})
	require.NoError(t, err)
	require.NotEmpty(t, logs)
	expectedPreview := "sk-" + token.Key[:6] + "**********" + token.Key[len(token.Key)-6:]
	require.Equal(t, token.Id, logs[0].TargetId)
	require.Contains(t, logs[0].TargetMetadata, "\"api_key\":\""+expectedPreview+"\"")
	require.Contains(t, logs[0].TargetMetadata, "\"name\":\"copyable-key\"")
	require.NotContains(t, logs[0].TargetMetadata, token.Key)

	require.NoError(t, model.DB.Delete(token).Error)
	logs, _, err = ListOrganizationAuditLogs(orgAdmin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationAuditQueryRequest{TargetType: "token", TargetId: token.Id, Limit: 20})
	require.NoError(t, err)
	require.NotEmpty(t, logs)
	require.Equal(t, token.Id, logs[0].TargetId)
	require.Contains(t, logs[0].TargetMetadata, "\"api_key\":\""+expectedPreview+"\"")
	require.NotContains(t, logs[0].TargetMetadata, token.Key)
}

func TestOrganizationAuditPlatformAdminCanFilterAcrossOrganizations(t *testing.T) {
	platformAdmin, _, _, _, otherOrg := createOrganizationAuditFixture(t)

	logs, total, err := ListAllOrganizationAuditLogs(platformAdmin.Id, OrganizationAuditQueryRequest{OrganizationSlug: otherOrg.Slug, ActionType: "organization.other", Limit: 20})

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, otherOrg.Id, logs[0].OrganizationId)
	require.Equal(t, otherOrg.Name, logs[0].OrganizationName)
	require.Equal(t, otherOrg.Slug, logs[0].OrganizationSlug)
}

func TestOrganizationAuditDissolvedOrganizationHistoryReadableByPlatformAdmin(t *testing.T) {
	platformAdmin, orgAdmin, _, organization, _ := createOrganizationAuditFixture(t)
	require.NoError(t, dissolveOrganizationForTest(t, orgAdmin.Id, organization.Id, "done"))

	logs, total, err := ListOrganizationAuditLogs(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, OrganizationAuditQueryRequest{Limit: 20})

	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(1))
	require.NotEmpty(t, logs)
}

func TestOrganizationAuditQueryReturnsMemberKeyTransferBlockedFailure(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "audit-blocked-owner", common.RoleCommonUser)
	member := createServiceTestUser(t, "audit-blocked-member", common.RoleCommonUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Audit Blocked Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	token := model.Token{UserId: member.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: member.Id}
	require.NoError(t, model.DB.Create(&token).Error)

	err = RemoveOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, RemoveMemberRequest{TransferToUserId: member.Id, Reason: "blocked query", IdempotencyKey: testOrganizationIdempotencyKey(t, "remove-member")})
	require.Error(t, err)

	logs, total, err := ListOrganizationAuditLogs(owner.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationAuditQueryRequest{ActionType: organizationAuditActionMemberKeyTransferBlocked, Limit: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, organizationAuditActionMemberKeyTransferBlocked, logs[0].ActionType)
	require.Equal(t, "member", logs[0].TargetType)
	require.Contains(t, logs[0].Reason, "active transfer target required")
}
