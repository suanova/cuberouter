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

func TestOrganizationManagementListRejectsNonAdmin(t *testing.T) {
	setupServiceTestDB(t)
	user := createServiceTestUser(t, "mgmt-list-non-admin", common.RoleCommonUser)
	pageInfo := &common.PageInfo{Page: 1, PageSize: 20}

	_, _, err := ListOrganizationsForManagement(user.Id, OrganizationManagementListRequest{}, pageInfo)

	require.ErrorContains(t, err, "permission denied")
}

func TestOrganizationManagementListDefaultsToAllStatuses(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "mgmt-list-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "mgmt-list-creator", common.RoleCommonUser)
	activeOrg, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Active Managed Org"})
	require.NoError(t, err)
	disabledOrg, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Disabled Managed Org"})
	require.NoError(t, err)
	dissolvedOrg, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Dissolved Managed Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", disabledOrg.Id).Update("status", model.OrganizationStatusDisabled).Error)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", dissolvedOrg.Id).Update("status", model.OrganizationStatusDissolved).Error)
	pageInfo := &common.PageInfo{Page: 1, PageSize: 20}

	views, total, err := ListOrganizationsForManagement(platformAdmin.Id, OrganizationManagementListRequest{}, pageInfo)

	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	ids := map[int]bool{}
	for _, view := range views {
		ids[view.Id] = true
	}
	require.True(t, ids[activeOrg.Id])
	require.True(t, ids[disabledOrg.Id])
	require.True(t, ids[dissolvedOrg.Id])
}

func TestOrganizationManagementListInvalidStatusDefaultsToAllStatuses(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "mgmt-list-invalid-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "mgmt-list-invalid-creator", common.RoleCommonUser)
	activeOrg, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Invalid Active Managed Org"})
	require.NoError(t, err)
	disabledOrg, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Invalid Disabled Managed Org"})
	require.NoError(t, err)
	dissolvedOrg, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Invalid Dissolved Managed Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", disabledOrg.Id).Update("status", model.OrganizationStatusDisabled).Error)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", dissolvedOrg.Id).Update("status", model.OrganizationStatusDissolved).Error)

	views, total, err := ListOrganizationsForManagement(platformAdmin.Id, OrganizationManagementListRequest{Status: "unknown"}, &common.PageInfo{Page: 1, PageSize: 20})

	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, views, 3)
	ids := map[int]bool{}
	for _, view := range views {
		ids[view.Id] = true
	}
	require.True(t, ids[activeOrg.Id])
	require.True(t, ids[disabledOrg.Id])
	require.True(t, ids[dissolvedOrg.Id])
}

func TestOrganizationManagementListCanFilterDissolved(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "mgmt-list-dissolved-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "mgmt-list-dissolved-creator", common.RoleCommonUser)
	dissolvedOrg, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Dissolved Filter Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", dissolvedOrg.Id).Update("status", model.OrganizationStatusDissolved).Error)
	pageInfo := &common.PageInfo{Page: 1, PageSize: 20}

	views, total, err := ListOrganizationsForManagement(platformAdmin.Id, OrganizationManagementListRequest{Status: model.OrganizationStatusDissolved}, pageInfo)

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, views, 1)
	require.Equal(t, dissolvedOrg.Id, views[0].Id)
}

func TestOrganizationManagementListSearchesOrganizationAndOwnerAndHydratesCounts(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "mgmt-search-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "creator-searchable", common.RoleCommonUser)
	owner := createServiceTestUser(t, "owner-searchable", common.RoleCommonUser)
	member := createServiceTestUser(t, "counted-member", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Hydrated Counts Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: org.Id, UserId: owner.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, TransferOrganizationOwner(creator.Id, org.Id, OrganizationAccessModeManagement, TransferOrganizationOwnerRequest{OwnerUserId: owner.Id, Reason: "new owner", IdempotencyKey: testOrganizationIdempotencyKey(t, "transfer-owner")}))
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: org.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusDisabled}).Error)
	require.NoError(t, model.DB.Create(&model.Token{UserId: creator.Id, Key: "101010101010101010101010101010101010101010101010", Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: org.Id, OrganizationId: org.Id, ResponsibleUserId: creator.Id}).Error)
	require.NoError(t, model.DB.Create(&model.Token{UserId: member.Id, Key: "202020202020202020202020202020202020202020202020", Status: common.TokenStatusDisabled, ScopeType: model.TokenScopeOrganization, ScopeId: org.Id, OrganizationId: org.Id, ResponsibleUserId: member.Id}).Error)
	pageInfo := &common.PageInfo{Page: 1, PageSize: 20}

	views, total, err := ListOrganizationsForManagement(platformAdmin.Id, OrganizationManagementListRequest{Keyword: "owner-searchable"}, pageInfo)

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, views, 1)
	require.Equal(t, org.Id, views[0].Id)
	require.Equal(t, owner.Username, views[0].OwnerUsername)
	require.Equal(t, owner.DisplayName, views[0].OwnerDisplayName)
	require.Equal(t, owner.Email, views[0].OwnerEmail)
	require.EqualValues(t, 2, views[0].ActiveMemberCount)
	require.EqualValues(t, 1, views[0].DisabledMemberCount)
	require.EqualValues(t, 3, views[0].TotalMemberCount)
	require.EqualValues(t, 1, views[0].EnabledTokenCount)
	require.EqualValues(t, 1, views[0].DisabledTokenCount)
	require.EqualValues(t, 2, views[0].TotalTokenCount)
}

func TestOrganizationDetailAllowsPlatformAdminWithoutMembershipAndReturnsNilMember(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "mgmt-detail-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "mgmt-detail-creator", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Managed Detail Org"})
	require.NoError(t, err)

	detail, err := GetOrganizationDetailForUserWithAccessMode(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, true)

	require.NoError(t, err)
	require.Equal(t, org.Id, detail.Organization.Id)
	require.Nil(t, detail.Member)
	require.Equal(t, OrganizationPolicyRolePlatformAdmin, detail.Actor.Role)
	require.Equal(t, OrganizationAccessModeAdmin, detail.Actor.AccessMode)
}

func TestOrganizationManagementUpdateAllowsPlatformAdminToEditGroupWithoutMembershipAndAuditsRole(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "mgmt-update-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "mgmt-update-creator", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Managed Update Org"})
	require.NoError(t, err)

	updated, err := UpdateOrganization(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, UpdateOrganizationRequest{Name: "Managed Updated Org", Description: "updated", Group: stringPtr("vip"), Reason: "metadata correction"})

	require.NoError(t, err)
	require.Equal(t, "Managed Updated Org", updated.Name)
	require.Equal(t, "updated", updated.Description)
	require.Equal(t, "vip", updated.Group)
	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", org.Id, organizationAuditActionUpdate).Order("id desc").First(&audit).Error)
	require.Equal(t, "platform_admin", audit.OperatorRole)
	require.Equal(t, "metadata correction", audit.Reason)
}

func TestOrganizationManagementStatusAllowsPlatformAdminWithoutMembership(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "mgmt-status-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "mgmt-status-creator", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Managed Status Org"})
	require.NoError(t, err)
	token := model.Token{UserId: creator.Id, Key: "303030303030303030303030303030303030303030303030", Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: org.Id, OrganizationId: org.Id, ResponsibleUserId: creator.Id}
	require.NoError(t, model.DB.Create(&token).Error)

	require.NoError(t, DisableOrganizationByPlatform(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, org.Slug, "platform disable"))
	var disabledToken model.Token
	require.NoError(t, model.DB.First(&disabledToken, token.Id).Error)
	require.Equal(t, common.TokenStatusDisabled, disabledToken.Status)
	require.True(t, disabledToken.DisabledBySystems)
	visibleToken, err := GetOrganizationToken(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, token.Id)
	require.NoError(t, err)
	require.True(t, visibleToken.DisabledBySystems)
	require.Equal(t, organizationTokenBlockerReasonOrganizationPlatform, visibleToken.SystemDisabledReason)
	require.NoError(t, EnableOrganizationByPlatform(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, org.Slug, "platform enable"))
	require.NoError(t, model.DB.First(&disabledToken, token.Id).Error)
	require.Equal(t, common.TokenStatusEnabled, disabledToken.Status)
	require.False(t, disabledToken.DisabledBySystems)
	require.ErrorContains(t, DissolveOrganization(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, DissolveOrganizationRequest{ConfirmName: org.Name, Reason: "platform dissolve", IdempotencyKey: testOrganizationIdempotencyKey(t, "dissolve")}), "permission denied")
	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, org.Id).Error)
	require.Equal(t, model.OrganizationStatusActive, stored.Status)
}

func TestOrganizationManagementMemberAndTokenOperationsAllowPlatformAdminWithoutMembership(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "mgmt-member-token-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "mgmt-member-token-creator", common.RoleCommonUser)
	member := createServiceTestUser(t, "mgmt-member-token-member", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Managed Member Token Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: org.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	newMember := createServiceTestUser(t, "mgmt-member-token-added", common.RoleCommonUser)

	added, err := AddOrganizationMember(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, AddMemberRequest{UserId: newMember.Id, Role: model.OrganizationRoleMember, Reason: "platform add member"})
	require.NoError(t, err)
	require.Equal(t, newMember.Id, added.UserId)
	require.Equal(t, model.OrganizationMemberStatusActive, added.Status)

	members, total, err := ListOrganizationMembersPaged(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, OrganizationMemberListRequest{Limit: 20})
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, members, 3)

	token, err := CreateOrganizationToken(creator.Id, org.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "owner-created", ResponsibleUserId: creator.Id, Visibility: model.TokenVisibilityPublic, UnlimitedQuota: true, ExpiredTime: -1})
	require.NoError(t, err)
	require.Equal(t, creator.Id, token.ResponsibleUserId)
	tokens, totalTokens, err := ListOrganizationTokens(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, OrganizationTokenListRequest{Limit: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, totalTokens)
	require.Len(t, tokens, 1)
	var addAudit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", org.Id, organizationAuditActionMemberAdd).Order("id desc").First(&addAudit).Error)
	require.Equal(t, "platform_admin", addAudit.OperatorRole)
	require.Equal(t, "platform add member", addAudit.Reason)

	require.NoError(t, UpdateOrganizationMember(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusDisabled, Reason: "platform disable member", IdempotencyKey: testOrganizationIdempotencyKey(t, "disable-member")}))
	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", org.Id, organizationAuditActionMemberUpdate).Order("id desc").First(&audit).Error)
	require.Equal(t, "platform_admin", audit.OperatorRole)
}
func TestOrganizationManagementUpdateBlankGroupFallsBackDefaultForPlatformAdmin(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "mgmt-blank-group-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "mgmt-blank-group-creator", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Managed Blank Group Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", org.Id).Update("group", "vip").Error)

	updated, err := UpdateOrganization(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, UpdateOrganizationRequest{Name: "Managed Blank Group Org", Description: "", Group: stringPtr("   "), Reason: "normalize group"})

	require.NoError(t, err)
	require.Equal(t, "default", updated.Group)
}

func TestOrganizationManagementPlatformAdminCannotCreateToken(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "mgmt-token-no-resp-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "mgmt-token-no-resp-creator", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Managed Token No Resp Org"})
	require.NoError(t, err)

	_, err = CreateOrganizationToken(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, OrganizationTokenRequest{Name: "missing-responsible", Visibility: model.TokenVisibilityPrivate, UnlimitedQuota: true, ExpiredTime: -1})

	require.ErrorContains(t, err, "permission denied")
}

func TestOrganizationManagementPlatformAdminCanTransferAndDeleteTokensWithoutMembership(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "mgmt-token-transfer-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "mgmt-token-transfer-creator", common.RoleCommonUser)
	member := createServiceTestUser(t, "mgmt-token-transfer-member", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Managed Token Transfer Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: org.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	token, err := CreateOrganizationToken(creator.Id, org.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "transfer-me", ExpiredTime: -1})
	require.NoError(t, err)

	updated, err := UpdateOrganizationToken(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true, ResponsibleUserId: member.Id, Visibility: model.TokenVisibilityPrivate})

	require.NoError(t, err)
	require.Equal(t, member.Id, updated.ResponsibleUserId)
	require.NoError(t, DeleteOrganizationToken(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, token.Id))
	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", org.Id, organizationAuditActionTokenDelete).Order("id desc").First(&audit).Error)
	require.Equal(t, "platform_admin", audit.OperatorRole)
}

func TestOrganizationManagementPlatformAdminCanReadDissolvedOrganizationAuditWithoutMembership(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "mgmt-audit-dissolved-platform-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "mgmt-audit-dissolved-creator", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Managed Audit Dissolved Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", org.Id).Update("status", model.OrganizationStatusDissolved).Error)

	logs, total, err := ListOrganizationAuditLogs(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, OrganizationAuditQueryRequest{Limit: 20})

	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(1))
	require.NotEmpty(t, logs)
}

func TestOrganizationManagementDissolvedOrganizationAllowsPlatformAdminReadOnlyAccess(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "mgmt-dissolved-read-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "mgmt-dissolved-read-creator", common.RoleCommonUser)
	member := createServiceTestUser(t, "mgmt-dissolved-read-member", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Managed Dissolved Read Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: org.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	token := model.Token{UserId: creator.Id, Key: "313131313131313131313131313131313131313131313131", Name: "dissolved-read-token", Status: common.TokenStatusDisabled, ScopeType: model.TokenScopeOrganization, ScopeId: org.Id, OrganizationId: org.Id, ResponsibleUserId: creator.Id, Visibility: model.TokenVisibilityPrivate}
	require.NoError(t, model.DB.Create(&token).Error)
	invite := model.OrganizationInvite{OrganizationId: org.Id, TargetEmail: "dissolved-read@example.com", Role: model.OrganizationRoleMember, Status: model.OrganizationInviteStatusRevoked, InviterUserId: creator.Id, CreatedAt: common.GetTimestamp(), UpdatedAt: common.GetTimestamp()}
	require.NoError(t, model.DB.Create(&invite).Error)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", org.Id).Update("status", model.OrganizationStatusDissolved).Error)

	detail, err := GetOrganizationDetailForUserWithAccessMode(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, true)
	require.NoError(t, err)
	require.Equal(t, model.OrganizationStatusDissolved, detail.Organization.Status)
	require.True(t, detail.Actor.ReadOnly)
	members, memberTotal, err := ListOrganizationMembersPaged(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, OrganizationMemberListRequest{Limit: 20})
	require.NoError(t, err)
	require.EqualValues(t, 2, memberTotal)
	require.Len(t, members, 2)
	invites, inviteTotal, err := ListOrganizationInvitesPaged(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, OrganizationInviteListRequest{Limit: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, inviteTotal)
	require.Len(t, invites, 1)
	tokens, tokenTotal, err := ListOrganizationTokens(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, OrganizationTokenListRequest{Limit: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, tokenTotal)
	require.Len(t, tokens, 1)
	readToken, err := GetOrganizationToken(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, token.Id)
	require.NoError(t, err)
	require.Equal(t, token.Id, readToken.Id)
}

func TestOrganizationWorkspacePlatformRoleMemberDoesNotBypassDissolvedResourceBoundary(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "mgmt-workspace-dissolved-owner", common.RoleCommonUser)
	platformMember := createServiceTestUser(t, "mgmt-workspace-dissolved-platform-member", common.RoleAdminUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Workspace Dissolved Resource Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: platformMember.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	token, err := CreateOrganizationToken(owner.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "workspace-dissolved-token", ExpiredTime: -1, Visibility: model.TokenVisibilityPrivate})
	require.NoError(t, err)
	invite := model.OrganizationInvite{OrganizationId: organization.Id, TargetEmail: "workspace-dissolved@example.com", Role: model.OrganizationRoleMember, Status: model.OrganizationInviteStatusRevoked, InviterUserId: owner.Id, CreatedAt: common.GetTimestamp(), UpdatedAt: common.GetTimestamp()}
	require.NoError(t, model.DB.Create(&invite).Error)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("status", model.OrganizationStatusDissolved).Error)

	_, _, err = ListOrganizationMembersPaged(platformMember.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationMemberListRequest{Limit: 20})
	require.ErrorContains(t, err, "organization dissolved")
	_, _, err = ListOrganizationInvitesPaged(platformMember.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationInviteListRequest{Limit: 20})
	require.ErrorContains(t, err, "organization dissolved")
	_, _, err = ListOrganizationTokens(platformMember.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenListRequest{Limit: 20})
	require.ErrorContains(t, err, "organization dissolved")
	_, err = GetOrganizationToken(platformMember.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id)
	require.ErrorContains(t, err, "organization dissolved")
}

func TestOrganizationManagementDissolvedOrganizationRejectsPlatformAdminWrites(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "mgmt-dissolved-write-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "mgmt-dissolved-write-creator", common.RoleCommonUser)
	member := createServiceTestUser(t, "mgmt-dissolved-write-member", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Managed Dissolved Write Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: org.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	token := model.Token{UserId: creator.Id, Key: "323232323232323232323232323232323232323232323232", Name: "dissolved-write-token", Status: common.TokenStatusDisabled, ScopeType: model.TokenScopeOrganization, ScopeId: org.Id, OrganizationId: org.Id, ResponsibleUserId: creator.Id, Visibility: model.TokenVisibilityPrivate}
	require.NoError(t, model.DB.Create(&token).Error)
	invite := model.OrganizationInvite{OrganizationId: org.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "dissolved-write@example.com", Role: model.OrganizationRoleMember, Token: "dissolved-write-invite", Status: model.OrganizationInviteStatusPending, InviterUserId: creator.Id}
	require.NoError(t, model.DB.Create(&invite).Error)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", org.Id).Updates(map[string]any{"status": model.OrganizationStatusDissolved, "quota": 100}).Error)

	_, err = UpdateOrganization(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, UpdateOrganizationRequest{Name: "Should Not Update", Description: "blocked"})
	require.ErrorContains(t, err, "organization dissolved")
	require.ErrorContains(t, DisableOrganizationByPlatform(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, org.Slug, "blocked"), "organization dissolved")
	require.ErrorContains(t, EnableOrganizationByPlatform(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, org.Slug, "blocked"), "organization dissolved")
	require.ErrorContains(t, DissolveOrganization(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, DissolveOrganizationRequest{ConfirmName: org.Name, Reason: "blocked", IdempotencyKey: testOrganizationIdempotencyKey(t, "dissolve")}), "organization dissolved")
	_, err = AdjustOrganizationQuota(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, OrganizationQuotaAdjustmentRequest{QuotaDelta: 10, Reason: "blocked", IdempotencyKey: testOrganizationIdempotencyKey(t, "quota-adjust")})
	require.ErrorContains(t, err, "organization dissolved")
	newMember := createServiceTestUser(t, "mgmt-dissolved-write-add-member", common.RoleCommonUser)
	_, err = AddOrganizationMember(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, AddMemberRequest{UserId: newMember.Id, Role: model.OrganizationRoleMember})
	require.ErrorContains(t, err, "organization dissolved")
	require.ErrorContains(t, UpdateOrganizationMember(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusDisabled, IdempotencyKey: testOrganizationIdempotencyKey(t, "disable-member")}), "organization dissolved")
	_, err = CreateOrganizationToken(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, OrganizationTokenRequest{Name: "blocked", ResponsibleUserId: member.Id, ExpiredTime: -1})
	require.ErrorContains(t, err, "organization dissolved")
	_, err = BatchCreateOrganizationTokens(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, OrganizationTokenBatchCreateRequest{TokenCount: 1, IdempotencyKey: testOrganizationIdempotencyKey(t, "batch-create"), Token: OrganizationTokenRequest{Name: "blocked-batch", ResponsibleUserId: member.Id, ExpiredTime: -1}})
	require.ErrorContains(t, err, "organization dissolved")
	_, err = UpdateOrganizationToken(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusDisabled, ExpiredTime: -1, UnlimitedQuota: true, ResponsibleUserId: creator.Id, Visibility: model.TokenVisibilityPrivate})
	require.ErrorContains(t, err, "organization dissolved")
	require.ErrorContains(t, DeleteOrganizationToken(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, token.Id), "organization dissolved")
	_, err = BatchDeleteOrganizationTokens(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, OrganizationTokenBatchDeleteRequest{Ids: []int{token.Id}, IdempotencyKey: testOrganizationIdempotencyKey(t, "batch-delete")})
	require.ErrorContains(t, err, "organization dissolved")
	_, err = UpdateOrganizationTokenResponsibility(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, token.Id, UpdateOrganizationTokenResponsibilityRequest{ResponsibleUserId: member.Id, Reason: "blocked"})
	require.ErrorContains(t, err, "organization dissolved")
	require.ErrorContains(t, RevokeInvite(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, invite.Id, "blocked"), "organization dissolved")
}

func TestOrganizationManagementTokenListGetReturnFullKeyForAuthorizedViewer(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "mgmt-token-full-key-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "mgmt-token-full-key-creator", common.RoleCommonUser)
	org, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Managed Token Full Key Org"})
	require.NoError(t, err)

	created, err := CreateOrganizationToken(creator.Id, org.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "full-key", ResponsibleUserId: creator.Id, ExpiredTime: -1})
	require.NoError(t, err)
	require.NotEmpty(t, created.Key)
	require.NotContains(t, created.Key, "*")
	fullKey := created.Key

	listed, total, err := ListOrganizationTokens(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, OrganizationTokenListRequest{Limit: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, listed, 1)
	require.Equal(t, fullKey, listed[0].Key)

	read, err := GetOrganizationToken(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, created.Id)
	require.NoError(t, err)
	require.Equal(t, fullKey, read.Key)

	updated, err := UpdateOrganizationToken(platformAdmin.Id, org.Id, OrganizationAccessModeAdmin, created.Id, OrganizationTokenRequest{Name: created.Name, Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true, ResponsibleUserId: creator.Id, Visibility: model.TokenVisibilityPrivate})
	require.NoError(t, err)
	// 组织 Key 完整 secret 在更新后每次返回，同时补全 masked 预览用于展示。
	require.Equal(t, fullKey, updated.Key)
	require.NotEmpty(t, updated.KeyPreview)

	var stored model.Token
	require.NoError(t, model.DB.First(&stored, created.Id).Error)
	require.Equal(t, fullKey, stored.Key)
}

func TestOrganizationPlatformMemberAccessModeWriteBoundary(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "access-mode-write-owner", common.RoleCommonUser)
	platformMember := createServiceTestUser(t, "access-mode-write-platform-member", common.RoleAdminUser)
	target := createServiceTestUser(t, "access-mode-write-target", common.RoleCommonUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Access Mode Write Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: platformMember.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	ownerToken, err := CreateOrganizationToken(owner.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "owner-private", Visibility: model.TokenVisibilityPrivate, ResponsibleUserId: owner.Id, UnlimitedQuota: true, ExpiredTime: -1})
	require.NoError(t, err)

	_, err = AddOrganizationMember(platformMember.Id, organization.Id, OrganizationAccessModeManagement, AddMemberRequest{UserId: target.Id, Role: model.OrganizationRoleMember})
	require.Error(t, err)
	_, err = UpdateOrganization(platformMember.Id, organization.Id, OrganizationAccessModeManagement, UpdateOrganizationRequest{Name: organization.Name, Group: stringPtr("vip")})
	require.ErrorContains(t, err, "permission denied")
	require.ErrorContains(t, DisableOrganization(platformMember.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "blocked"), "permission denied")
	require.ErrorContains(t, DissolveOrganization(platformMember.Id, organization.Id, OrganizationAccessModeManagement, DissolveOrganizationRequest{ConfirmName: organization.Name, Reason: "blocked", IdempotencyKey: testOrganizationIdempotencyKey(t, "dissolve")}), "permission denied")
	_, err = UpdateOrganizationToken(platformMember.Id, organization.Id, OrganizationAccessModeManagement, ownerToken.Id, OrganizationTokenRequest{Name: ownerToken.Name, Status: common.TokenStatusEnabled, Visibility: model.TokenVisibilityPrivate, ResponsibleUserId: owner.Id, UnlimitedQuota: true, ExpiredTime: -1})
	require.Error(t, err)
	tokens, total, err := ListOrganizationTokens(platformMember.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenListRequest{Limit: 20})
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, tokens)

	updated, err := UpdateOrganization(platformMember.Id, organization.Id, OrganizationAccessModeAdmin, UpdateOrganizationRequest{Name: organization.Name, Group: stringPtr("vip"), Reason: "platform correction"})
	require.NoError(t, err)
	require.Equal(t, "vip", updated.Group)
}
