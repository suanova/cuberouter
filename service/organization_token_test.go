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
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func createOrganizationTokenTestOrg(t *testing.T) (model.User, model.User, *model.Organization) {
	t.Helper()
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "org-token-admin", common.RoleCommonUser)
	member := createServiceTestUser(t, "org-token-member", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Token Org"})
	require.NoError(t, err)
	// 测试组织预置可消费额度，模拟平台已为组织配置额度后的计费场景。
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("quota", 2000).Error)
	organization.Quota = 2000
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	return admin, member, organization
}

func createOrganizationTokenPlatformMember(t *testing.T, username string) (model.User, model.User, *model.Organization) {
	t.Helper()
	owner, _, organization := createOrganizationTokenTestOrg(t)
	platformMember := createServiceTestUser(t, username, common.RoleAdminUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: platformMember.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	return owner, platformMember, organization
}

func TestOrganizationTokenPlatformRoleMemberWorkspaceCanCreateOwnPrivateToken(t *testing.T) {
	_, platformMember, organization := createOrganizationTokenPlatformMember(t, "org-token-platform-member-create")

	token, err := CreateOrganizationToken(platformMember.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "workspace-create", ResponsibleUserId: platformMember.Id, Visibility: model.TokenVisibilityPrivate, ExpiredTime: -1})

	require.NoError(t, err)
	require.Equal(t, platformMember.Id, token.ResponsibleUserId)
	require.Equal(t, model.TokenVisibilityPrivate, token.Visibility)
}

func TestOrganizationTokenPlatformRoleMemberWorkspaceCanBatchCreateOwnPrivateTokens(t *testing.T) {
	_, platformMember, organization := createOrganizationTokenPlatformMember(t, "org-token-platform-member-batch-create")
	req := OrganizationTokenBatchCreateRequest{
		TokenCount:     2,
		IdempotencyKey: testOrganizationIdempotencyKey(t, "platform-member-workspace-batch-create"),
		Token:          OrganizationTokenRequest{Name: "workspace-batch", ResponsibleUserId: platformMember.Id, Visibility: model.TokenVisibilityPrivate, ExpiredTime: -1},
	}

	result, err := BatchCreateOrganizationTokens(platformMember.Id, organization.Id, OrganizationAccessModeWorkspace, req)

	require.NoError(t, err)
	require.Len(t, result.Tokens, 2)
	for _, token := range result.Tokens {
		require.Equal(t, platformMember.Id, token.ResponsibleUserId)
		require.Equal(t, model.TokenVisibilityPrivate, token.Visibility)
	}
}

func TestOrganizationTokenPlatformRoleMemberWorkspaceCreatesMemberClearanceBlocker(t *testing.T) {
	owner, platformMember, organization := createOrganizationTokenPlatformMember(t, "org-token-platform-member-disable")
	token, err := CreateOrganizationToken(owner.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "workspace-disable", ResponsibleUserId: platformMember.Id, Visibility: model.TokenVisibilityPrivate, ExpiredTime: -1})
	require.NoError(t, err)

	_, err = UpdateOrganizationToken(platformMember.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusDisabled, ResponsibleUserId: platformMember.Id, Visibility: model.TokenVisibilityPrivate, ExpiredTime: -1})

	require.NoError(t, err)
	var blocker model.OrganizationTokenSystemBlocker
	require.NoError(t, model.DB.Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonManualDisabled, model.OrganizationTokenBlockerStatusActive).First(&blocker).Error)
	require.Equal(t, organizationTokenBlockerClearanceMember, blocker.ClearanceLevel)
}

func TestOrganizationTokenPlatformRoleMemberWorkspaceCannotClearPlatformBlocker(t *testing.T) {
	owner, platformMember, organization := createOrganizationTokenPlatformMember(t, "org-token-platform-member-enable")
	token, err := CreateOrganizationToken(owner.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "workspace-enable", ResponsibleUserId: platformMember.Id, Visibility: model.TokenVisibilityPrivate, ExpiredTime: -1})
	require.NoError(t, err)
	now := common.GetTimestamp()
	require.NoError(t, ensureOrganizationTokenSystemBlockerWithTx(model.DB, token, organizationTokenBlockerReasonManualDisabled, organizationTokenBlockerRefTypeOperator, owner.Id, owner.Id, organizationTokenBlockerClearancePlatform, now))
	require.NoError(t, reconcileOrganizationTokenSystemBlockerWithTx(model.DB, token, now))

	_, err = UpdateOrganizationToken(platformMember.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusEnabled, ResponsibleUserId: platformMember.Id, Visibility: model.TokenVisibilityPrivate, ExpiredTime: -1})

	require.ErrorIs(t, err, ErrOrganizationTokenEnableForbidden)
	var activeBlockerCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonManualDisabled, model.OrganizationTokenBlockerStatusActive).Count(&activeBlockerCount).Error)
	require.EqualValues(t, 1, activeBlockerCount)
}

func TestOrganizationTokenPlatformRoleMemberAdminModeKeepsPlatformRestrictionsAndClearance(t *testing.T) {
	owner, platformMember, organization := createOrganizationTokenPlatformMember(t, "org-token-platform-member-admin-mode")

	_, err := CreateOrganizationToken(platformMember.Id, organization.Id, OrganizationAccessModeAdmin, OrganizationTokenRequest{Name: "admin-create", ResponsibleUserId: platformMember.Id, Visibility: model.TokenVisibilityPrivate, ExpiredTime: -1})
	require.ErrorContains(t, err, "permission denied")

	token, err := CreateOrganizationToken(owner.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "admin-disable", ResponsibleUserId: platformMember.Id, Visibility: model.TokenVisibilityPrivate, ExpiredTime: -1})
	require.NoError(t, err)
	_, err = UpdateOrganizationToken(platformMember.Id, organization.Id, OrganizationAccessModeAdmin, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusDisabled, ResponsibleUserId: platformMember.Id, Visibility: model.TokenVisibilityPrivate, ExpiredTime: -1})
	require.NoError(t, err)
	var blocker model.OrganizationTokenSystemBlocker
	require.NoError(t, model.DB.Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonManualDisabled, model.OrganizationTokenBlockerStatusActive).First(&blocker).Error)
	require.Equal(t, organizationTokenBlockerClearancePlatform, blocker.ClearanceLevel)

	_, err = UpdateOrganizationToken(platformMember.Id, organization.Id, OrganizationAccessModeAdmin, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusEnabled, ResponsibleUserId: platformMember.Id, Visibility: model.TokenVisibilityPrivate, ExpiredTime: -1})
	require.NoError(t, err)
	var activeBlockerCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonManualDisabled, model.OrganizationTokenBlockerStatusActive).Count(&activeBlockerCount).Error)
	require.Zero(t, activeBlockerCount)
}

func TestOrganizationTokenPlatformNonMemberWorkspaceRemainsDenied(t *testing.T) {
	_, _, organization := createOrganizationTokenTestOrg(t)
	platformNonMember := createServiceTestUser(t, "org-token-platform-non-member-workspace", common.RoleAdminUser)

	_, err := CreateOrganizationToken(platformNonMember.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "denied", ResponsibleUserId: platformNonMember.Id, Visibility: model.TokenVisibilityPrivate, ExpiredTime: -1})

	require.Error(t, err)
}

func TestOrganizationTokenMemberSeesOwnResponsibleAndPublicKeys(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	own, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "own", ExpiredTime: -1})
	require.NoError(t, err)
	adminPrivateToken, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "admin-private", ExpiredTime: -1, Visibility: model.TokenVisibilityPrivate})
	require.NoError(t, err)
	adminPublicToken, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "admin-public", ExpiredTime: -1, Visibility: model.TokenVisibilityPublic})
	require.NoError(t, err)

	tokens, total, err := ListOrganizationTokens(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenListRequest{Limit: 20})

	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, tokens, 2)
	ids := []int{tokens[0].Id, tokens[1].Id}
	require.Contains(t, ids, own.Id)
	require.Contains(t, ids, adminPublicToken.Id)
	require.NotContains(t, ids, adminPrivateToken.Id)

	publicToken, err := GetOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, adminPublicToken.Id)
	require.NoError(t, err)
	require.Equal(t, adminPublicToken.Id, publicToken.Id)

	_, err = GetOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, adminPrivateToken.Id)
	require.Error(t, err)
}

func TestOrganizationTokenMemberSeesDisabledPublicKeyWithUnavailableReason(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	own, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "own", ExpiredTime: -1})
	require.NoError(t, err)
	disabledPublic, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "disabled-public", ExpiredTime: -1, Visibility: model.TokenVisibilityPublic})
	require.NoError(t, err)
	_, err = UpdateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, disabledPublic.Id, OrganizationTokenRequest{Name: disabledPublic.Name, Status: common.TokenStatusDisabled, ExpiredTime: -1, UnlimitedQuota: true, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: admin.Id})
	require.NoError(t, err)

	tokens, total, err := ListOrganizationTokens(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenListRequest{Limit: 20})

	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, tokens, 2)
	ids := []int{tokens[0].Id, tokens[1].Id}
	require.Contains(t, ids, own.Id)
	require.Contains(t, ids, disabledPublic.Id)
	memberPublic, err := GetOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, disabledPublic.Id)
	require.NoError(t, err)
	require.Contains(t, memberPublic.UnavailableReasons, organizationTokenBlockerReasonManualDisabled)

	memberFiltered, memberFilteredTotal, err := ListOrganizationTokens(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenListRequest{Limit: 20, Keyword: "disabled-public", Status: common.TokenStatusDisabled, Visibility: model.TokenVisibilityPublic})
	require.NoError(t, err)
	require.EqualValues(t, 1, memberFilteredTotal)
	require.Len(t, memberFiltered, 1)
	require.Equal(t, disabledPublic.Id, memberFiltered[0].Id)
	require.Contains(t, memberFiltered[0].UnavailableReasons, organizationTokenBlockerReasonManualDisabled)

	adminFiltered, adminFilteredTotal, err := ListOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenListRequest{Limit: 20, Keyword: "disabled-public", Status: common.TokenStatusDisabled, Visibility: model.TokenVisibilityPublic})
	require.NoError(t, err)
	require.EqualValues(t, 1, adminFilteredTotal)
	require.Len(t, adminFiltered, 1)
	require.Equal(t, disabledPublic.Id, adminFiltered[0].Id)
	require.Contains(t, adminFiltered[0].UnavailableReasons, organizationTokenBlockerReasonManualDisabled)
}

func TestOrganizationTokenMemberSeesUnavailablePublicKeyFromDisabledResponsibleUser(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	otherAdmin := createServiceTestUser(t, "public-disabled-responsible-admin", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: otherAdmin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	token, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "public-disabled-responsible-key", ExpiredTime: -1, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: otherAdmin.Id})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", otherAdmin.Id).Update("status", common.UserStatusDisabled).Error)
	require.NoError(t, UpdateOrganizationTokenBlockersForUserStatus(otherAdmin.Id, common.UserStatusDisabled, admin.Id))

	tokens, total, err := ListOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenListRequest{Limit: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, tokens, 1)
	require.Equal(t, token.Id, tokens[0].Id)
	require.Contains(t, tokens[0].UnavailableReasons, organizationTokenBlockerReasonUserPlatformDisabled)
	require.True(t, tokens[0].DisabledBySystems)

	memberTokens, memberTotal, err := ListOrganizationTokens(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenListRequest{Limit: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, memberTotal)
	require.Len(t, memberTokens, 1)
	require.Equal(t, token.Id, memberTokens[0].Id)
	require.Contains(t, memberTokens[0].UnavailableReasons, organizationTokenBlockerReasonUserPlatformDisabled)
	require.True(t, memberTokens[0].DisabledBySystems)
}

func TestOrganizationTokenDisabledActorCannotReadOrganizationTokens(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", member.Id).Update("status", common.UserStatusDisabled).Error)

	tokens, total, err := ListOrganizationTokens(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenListRequest{Limit: 20})

	require.ErrorContains(t, err, "permission denied")
	require.Zero(t, total)
	require.Nil(t, tokens)
}

func TestOrganizationTokenAdminSeesAllKeys(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	_, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "member-key", ExpiredTime: -1})
	require.NoError(t, err)
	_, err = CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "admin-key", ExpiredTime: -1})
	require.NoError(t, err)

	tokens, total, err := ListOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenListRequest{Limit: 20})

	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, tokens, 2)
}

func TestOrganizationTokenListIncludesResponsibleUsernameButNotEmail(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", member.Id).Updates(map[string]any{
		"username":     "token-owner",
		"display_name": "Token Owner",
		"email":        "owner@example.com",
	}).Error)
	_, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "member-key", ExpiredTime: -1})
	require.NoError(t, err)

	tokens, total, err := ListOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenListRequest{Limit: 20})

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, tokens, 1)
	require.Equal(t, "token-owner", tokens[0].ResponsibleUsername)
	require.Equal(t, "Token Owner", tokens[0].ResponsibleDisplayName)
	require.Empty(t, tokens[0].ResponsibleEmail)
}

func TestOrganizationTokenCreateSetsScopeFieldsAndPersonalIsolation(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)

	token, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "scoped-key", ExpiredTime: -1, ResponsibleUserId: member.Id})

	require.NoError(t, err)
	require.Equal(t, model.TokenScopeOrganization, token.ScopeType)
	require.Equal(t, organization.Id, token.ScopeId)
	require.Equal(t, organization.Id, token.OrganizationId)
	require.Equal(t, admin.Id, token.CreatorUserId)
	require.Equal(t, member.Id, token.ResponsibleUserId)
	require.Equal(t, member.Id, token.UserId)
	personalTokens, err := model.GetAllUserTokens(member.Id, 0, 20)
	require.NoError(t, err)
	for _, personalToken := range personalTokens {
		require.NotEqual(t, token.Id, personalToken.Id)
		require.Equal(t, model.TokenScopePersonal, personalToken.ScopeType)
	}
}

func TestOrganizationTokenUpdatePersistsOrganizationFields(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	other := createServiceTestUser(t, "org-token-update-org-fields", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: other.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	token, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "update-org-fields", ExpiredTime: -1, ResponsibleUserId: member.Id})
	require.NoError(t, err)

	token.UserId = other.Id
	token.ResponsibleUserId = other.Id
	token.CreatorUserId = admin.Id
	token.ScopeType = model.TokenScopeOrganization
	token.ScopeId = organization.Id
	token.OrganizationId = organization.Id
	token.Visibility = model.TokenVisibilityPublic
	token.TransferReason = "model update transfer"
	require.NoError(t, token.Update())

	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	require.Equal(t, model.TokenScopeOrganization, stored.ScopeType)
	require.Equal(t, organization.Id, stored.ScopeId)
	require.Equal(t, organization.Id, stored.OrganizationId)
	require.Equal(t, admin.Id, stored.CreatorUserId)
	require.Equal(t, other.Id, stored.ResponsibleUserId)
	require.Equal(t, other.Id, stored.UserId)
	require.Equal(t, model.TokenVisibilityPublic, stored.Visibility)
	require.Equal(t, "model update transfer", stored.TransferReason)
}

func TestOrganizationTokenMemberCannotTransferResponsibility(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	other := createServiceTestUser(t, "org-token-other", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: other.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "member-key", ExpiredTime: -1})
	require.NoError(t, err)

	_, err = UpdateOrganizationTokenResponsibility(member.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, UpdateOrganizationTokenResponsibilityRequest{ResponsibleUserId: other.Id, Reason: "handoff"})

	require.ErrorContains(t, err, "permission denied")
}

func TestOrganizationTokenAdminCanTransferResponsibility(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	other := createServiceTestUser(t, "org-token-transfer-to", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: other.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "member-key", ExpiredTime: -1})
	require.NoError(t, err)

	updated, err := UpdateOrganizationTokenResponsibility(admin.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, UpdateOrganizationTokenResponsibilityRequest{ResponsibleUserId: other.Id, Reason: "handoff"})

	require.NoError(t, err)
	require.Equal(t, other.Id, updated.UserId)
	require.Equal(t, other.Id, updated.ResponsibleUserId)
	require.Equal(t, "handoff", updated.TransferReason)
	// 组织 Key 完整 secret 每次返回，同时补全 masked 预览用于展示。
	require.NotEmpty(t, updated.Key)
	require.NotEmpty(t, updated.KeyPreview)

	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	require.Equal(t, other.Id, stored.UserId)
	require.Equal(t, other.Id, stored.ResponsibleUserId)
}

func TestOrganizationTokenResponsibilityTransferReconcilesOldResponsibilityBlockers(t *testing.T) {
	owner, _, organization := createOrganizationTokenTestOrg(t)
	source := createServiceTestUser(t, "responsibility-transfer-blocked-source", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", source.Id).Update("status", common.UserStatusDisabled).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: source.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusDisabled}).Error)
	now := common.GetTimestamp()
	token := model.Token{UserId: source.Id, Key: common.GetUUID(), Status: common.TokenStatusDisabled, DisabledBySystems: true, SystemDisabledReason: organizationTokenBlockerReasonMemberDisabled, SystemDisabledRefId: source.Id, SystemDisabledAt: now, PreviousStatus: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: source.Id}
	require.NoError(t, model.DB.Create(&token).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationTokenSystemBlocker{TokenId: token.Id, OrganizationId: organization.Id, Reason: organizationTokenBlockerReasonMemberDisabled, RefType: organizationTokenBlockerRefTypeMember, RefId: source.Id, Status: model.OrganizationTokenBlockerStatusActive, PreviousStatus: common.TokenStatusEnabled, DisabledAt: now, CreatedAt: now, UpdatedAt: now}).Error)

	updated, err := UpdateOrganizationTokenResponsibility(owner.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, UpdateOrganizationTokenResponsibilityRequest{ResponsibleUserId: owner.Id, Reason: "repair stale responsibility"})

	require.NoError(t, err)
	require.Equal(t, owner.Id, updated.ResponsibleUserId)
	require.Equal(t, common.TokenStatusEnabled, updated.Status)
	require.False(t, updated.DisabledBySystems)
	var activeBlockers int64
	require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).Where("token_id = ? AND status = ?", token.Id, model.OrganizationTokenBlockerStatusActive).Count(&activeBlockers).Error)
	require.Zero(t, activeBlockers)
}

func TestOrganizationTokenAdminCannotTransferPublicResponsibilityToMember(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "public-key", ExpiredTime: -1, Visibility: model.TokenVisibilityPublic})
	require.NoError(t, err)

	_, err = UpdateOrganizationTokenResponsibility(admin.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, UpdateOrganizationTokenResponsibilityRequest{ResponsibleUserId: member.Id, Reason: "handoff"})

	require.ErrorContains(t, err, "public token responsible user must be organization owner or admin")
	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	require.Equal(t, admin.Id, stored.ResponsibleUserId)
}

func TestOrganizationTokenUpdateCannotTransferPublicResponsibilityToMember(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "public-update-key", ExpiredTime: -1, Visibility: model.TokenVisibilityPublic})
	require.NoError(t, err)

	_, err = UpdateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusEnabled, ExpiredTime: -1, Visibility: model.TokenVisibilityPublic, UnlimitedQuota: true, ResponsibleUserId: member.Id})

	require.ErrorContains(t, err, "public token responsible user must be organization owner or admin")
	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	require.Equal(t, admin.Id, stored.ResponsibleUserId)
}

func TestOrganizationTokenUpdateWithoutVisibilityCannotTransferPublicResponsibilityToMember(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "public-update-implicit-key", ExpiredTime: -1, Visibility: model.TokenVisibilityPublic})
	require.NoError(t, err)

	_, err = UpdateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true, ResponsibleUserId: member.Id})

	require.ErrorContains(t, err, "public token responsible user must be organization owner or admin")
	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	require.Equal(t, admin.Id, stored.ResponsibleUserId)
	require.Equal(t, model.TokenVisibilityPublic, stored.Visibility)
}

func TestOrganizationTokenResponsibleMemberWritesLockTargetMember(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	other := createServiceTestUser(t, "org-token-lock-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: other.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	lockedMemberChecks := 0
	callbackName := "test:organization-token-responsible-member-lock"
	require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != "organization_members" {
			return
		}
		forClause, ok := tx.Statement.Clauses["FOR"]
		if !ok {
			return
		}
		locking, ok := forClause.Expression.(clause.Locking)
		if ok && locking.Strength == "UPDATE" {
			lockedMemberChecks++
		}
	}))
	t.Cleanup(func() {
		_ = model.DB.Callback().Query().Remove(callbackName)
	})

	created, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "locked-create", ExpiredTime: -1, ResponsibleUserId: member.Id})
	require.NoError(t, err)
	memberToken, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "locked-self-create", ExpiredTime: -1})
	require.NoError(t, err)
	memberBatchDeleteToken, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "locked-self-batch-delete", ExpiredTime: -1})
	require.NoError(t, err)
	_, err = BatchCreateOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenBatchCreateRequest{TokenCount: 1, IdempotencyKey: testOrganizationIdempotencyKey(t, "batch-create"), Token: OrganizationTokenRequest{Name: "locked-batch", ExpiredTime: -1, ResponsibleUserId: member.Id}})
	require.NoError(t, err)
	_, err = UpdateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, memberToken.Id, OrganizationTokenRequest{Name: memberToken.Name, Status: common.TokenStatusEnabled, ExpiredTime: -1, Visibility: model.TokenVisibilityPrivate, UnlimitedQuota: true})
	require.NoError(t, err)
	_, err = UpdateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, created.Id, OrganizationTokenRequest{Name: created.Name, Status: common.TokenStatusEnabled, ExpiredTime: -1, Visibility: model.TokenVisibilityPrivate, UnlimitedQuota: true, ResponsibleUserId: other.Id})
	require.NoError(t, err)
	_, err = UpdateOrganizationTokenResponsibility(admin.Id, organization.Id, OrganizationAccessModeWorkspace, created.Id, UpdateOrganizationTokenResponsibilityRequest{ResponsibleUserId: member.Id, Reason: "lock target"})
	require.NoError(t, err)
	require.NoError(t, DeleteOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, memberToken.Id))
	_, err = BatchDeleteOrganizationTokens(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenBatchDeleteRequest{Ids: []int{memberBatchDeleteToken.Id}, IdempotencyKey: testOrganizationIdempotencyKey(t, "batch-delete")})
	require.NoError(t, err)

	requireOrganizationRowLockCount(t, lockedMemberChecks, 9)
}

func TestOrganizationTokenWritesLockOrganizationRow(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	other := createServiceTestUser(t, "org-token-lock-org-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: other.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	lockedOrganizationChecks := 0
	callbackName := "test:organization-token-write-org-lock"
	require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != "organizations" {
			return
		}
		forClause, ok := tx.Statement.Clauses["FOR"]
		if !ok {
			return
		}
		locking, ok := forClause.Expression.(clause.Locking)
		if ok && locking.Strength == "UPDATE" {
			lockedOrganizationChecks++
		}
	}))
	t.Cleanup(func() {
		_ = model.DB.Callback().Query().Remove(callbackName)
	})

	created, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "org-lock-create", ExpiredTime: -1, ResponsibleUserId: member.Id})
	require.NoError(t, err)
	batchResult, err := BatchCreateOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenBatchCreateRequest{TokenCount: 1, IdempotencyKey: testOrganizationIdempotencyKey(t, "batch-create"), Token: OrganizationTokenRequest{Name: "org-lock-batch", ExpiredTime: -1, ResponsibleUserId: member.Id}})
	require.NoError(t, err)
	batchTokens := batchResult.Tokens
	require.Len(t, batchTokens, 1)
	_, err = UpdateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, created.Id, OrganizationTokenRequest{Name: created.Name, Status: common.TokenStatusEnabled, ExpiredTime: -1, Visibility: model.TokenVisibilityPrivate, UnlimitedQuota: true, ResponsibleUserId: other.Id})
	require.NoError(t, err)
	_, err = UpdateOrganizationTokenResponsibility(admin.Id, organization.Id, OrganizationAccessModeWorkspace, created.Id, UpdateOrganizationTokenResponsibilityRequest{ResponsibleUserId: member.Id, Reason: "lock organization"})
	require.NoError(t, err)
	require.NoError(t, DeleteOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, created.Id))
	_, err = BatchDeleteOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenBatchDeleteRequest{Ids: []int{batchTokens[0].Id}, IdempotencyKey: testOrganizationIdempotencyKey(t, "batch-delete")})
	require.NoError(t, err)

	requireOrganizationRowLockCount(t, lockedOrganizationChecks, 6)
}

func TestOrganizationTokenAdminCanCreatePublicKey(t *testing.T) {
	admin, _, organization := createOrganizationTokenTestOrg(t)

	token, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "admin-public", ExpiredTime: -1, Visibility: model.TokenVisibilityPublic})

	require.NoError(t, err)
	require.Equal(t, model.TokenVisibilityPublic, token.Visibility)
}

func TestOrganizationTokenAdminCannotCreatePublicKeyForMember(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)

	_, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "member-public-owner", ExpiredTime: -1, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: member.Id})

	require.ErrorContains(t, err, "public token responsible user must be organization owner or admin")
	var count int64
	require.NoError(t, model.DB.Model(&model.Token{}).Where("organization_id = ? AND name = ?", organization.Id, "member-public-owner").Count(&count).Error)
	require.EqualValues(t, 0, count)
}

func TestOrganizationTokenAdminCannotBatchCreatePublicKeysForMember(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)

	_, err := BatchCreateOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenBatchCreateRequest{TokenCount: 2, IdempotencyKey: testOrganizationIdempotencyKey(t, "batch-create"), Token: OrganizationTokenRequest{Name: "member-public-owner-batch", ExpiredTime: -1, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: member.Id}})

	require.ErrorContains(t, err, "public token responsible user must be organization owner or admin")
	var count int64
	require.NoError(t, model.DB.Model(&model.Token{}).Where("organization_id = ? AND name LIKE ?", organization.Id, "member-public-owner-batch%").Count(&count).Error)
	require.EqualValues(t, 0, count)
}

func TestOrganizationTokenMemberCannotCreatePublicKey(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)

	_, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "member-public", ExpiredTime: -1, Visibility: model.TokenVisibilityPublic})

	require.ErrorContains(t, err, "permission denied")
	var count int64
	require.NoError(t, model.DB.Model(&model.Token{}).Where("organization_id = ? AND name = ?", organization.Id, "member-public").Count(&count).Error)
	require.EqualValues(t, 0, count)
}

func TestOrganizationTokenMemberCannotBatchCreatePublicKeys(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)

	_, err := BatchCreateOrganizationTokens(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenBatchCreateRequest{TokenCount: 2, IdempotencyKey: testOrganizationIdempotencyKey(t, "batch-create"), Token: OrganizationTokenRequest{Name: "member-public-batch", ExpiredTime: -1, Visibility: model.TokenVisibilityPublic}})

	require.ErrorContains(t, err, "permission denied")
	var count int64
	require.NoError(t, model.DB.Model(&model.Token{}).Where("organization_id = ? AND name LIKE ?", organization.Id, "member-public-batch%").Count(&count).Error)
	require.EqualValues(t, 0, count)
}

func TestOrganizationTokenMemberCannotPromotePrivateKeyToPublic(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "member-private", ExpiredTime: -1, Visibility: model.TokenVisibilityPrivate})
	require.NoError(t, err)

	_, err = UpdateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusEnabled, ExpiredTime: -1, Visibility: model.TokenVisibilityPublic, UnlimitedQuota: true})

	require.ErrorContains(t, err, "permission denied")
	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	require.Equal(t, model.TokenVisibilityPrivate, stored.Visibility)
}

func TestOrganizationTokenAdminCannotPromoteMemberResponsiblePrivateKeyToPublic(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "member-private-to-public", ExpiredTime: -1, Visibility: model.TokenVisibilityPrivate, ResponsibleUserId: member.Id})
	require.NoError(t, err)

	_, err = UpdateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusEnabled, ExpiredTime: -1, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: member.Id, UnlimitedQuota: true})

	require.ErrorContains(t, err, "public token responsible user must be organization owner or admin")
	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	require.Equal(t, model.TokenVisibilityPrivate, stored.Visibility)
	require.Equal(t, member.Id, stored.ResponsibleUserId)
}

func TestOrganizationTokenWritesRejectDisabledReadOnlyOrganization(t *testing.T) {
	admin, _, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "disabled-write", ExpiredTime: -1, Visibility: model.TokenVisibilityPrivate})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("status", model.OrganizationStatusDisabled).Error)

	_, err = CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "blocked-create", ExpiredTime: -1, Visibility: model.TokenVisibilityPrivate})
	require.ErrorContains(t, err, "organization disabled")
	_, err = BatchCreateOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenBatchCreateRequest{TokenCount: 2, IdempotencyKey: testOrganizationIdempotencyKey(t, "batch-create"), Token: OrganizationTokenRequest{Name: "blocked-batch", ExpiredTime: -1, Visibility: model.TokenVisibilityPrivate}})
	require.ErrorContains(t, err, "organization disabled")
	_, err = UpdateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusEnabled, ExpiredTime: -1, Visibility: model.TokenVisibilityPrivate, UnlimitedQuota: true})
	require.ErrorContains(t, err, "organization disabled")
	require.ErrorContains(t, DeleteOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id), "organization disabled")
	_, err = BatchDeleteOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenBatchDeleteRequest{Ids: []int{token.Id}, IdempotencyKey: testOrganizationIdempotencyKey(t, "batch-delete")})
	require.ErrorContains(t, err, "organization disabled")
	_, err = UpdateOrganizationTokenResponsibility(admin.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, UpdateOrganizationTokenResponsibilityRequest{ResponsibleUserId: admin.Id})
	require.ErrorContains(t, err, "organization disabled")
}

func TestOrganizationTokenResponsibleUserMustBeActiveMember(t *testing.T) {
	admin, _, organization := createOrganizationTokenTestOrg(t)
	nonMember := createServiceTestUser(t, "org-token-non-member", common.RoleCommonUser)

	_, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "bad-key", ExpiredTime: -1, ResponsibleUserId: nonMember.Id})

	require.ErrorContains(t, err, "responsible user must be active member")
}

func TestOrganizationTokenDisabledMemberCannotBeResponsible(t *testing.T) {
	admin, _, organization := createOrganizationTokenTestOrg(t)
	disabled := createServiceTestUser(t, "org-token-disabled", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: disabled.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusDisabled}).Error)

	_, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "bad-key", ExpiredTime: -1, ResponsibleUserId: disabled.Id})

	require.ErrorContains(t, err, "responsible user must be active member")
}

func TestOrganizationTokenDisabledUserCannotBeResponsible(t *testing.T) {
	admin, _, organization := createOrganizationTokenTestOrg(t)
	disabledUser := createServiceTestUser(t, "org-token-disabled-user", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", disabledUser.Id).Update("status", common.UserStatusDisabled).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: disabledUser.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	_, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "bad-key", ExpiredTime: -1, ResponsibleUserId: disabledUser.Id})

	require.ErrorContains(t, err, "responsible user must be active member")
}

func TestOrganizationTokenDeletedUserCannotBeResponsible(t *testing.T) {
	admin, _, organization := createOrganizationTokenTestOrg(t)
	deletedUser := createServiceTestUser(t, "org-token-deleted-user", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: deletedUser.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Delete(&deletedUser).Error)

	_, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "bad-key", ExpiredTime: -1, ResponsibleUserId: deletedUser.Id})

	require.ErrorContains(t, err, "responsible user must be active member")
}

func TestOrganizationTokenAuditLogWritten(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "audit-key", ExpiredTime: -1})
	require.NoError(t, err)

	var createLog model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ? AND target_id = ?", organization.Id, organizationAuditActionTokenCreate, token.Id).First(&createLog).Error)
	require.Equal(t, organization.Id, createLog.OrganizationId)

	_, err = TransferOrganizationTokenResponsibility(admin.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, admin.Id, "audit-transfer")
	require.NoError(t, err)
	var transferLog model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ? AND target_id = ?", organization.Id, organizationAuditActionTokenResponsibilityUpdate, token.Id).First(&transferLog).Error)
	require.Equal(t, "audit-transfer", transferLog.Reason)
}

func TestOrganizationTokenMemberCanUpdateOwnResponsibleKeyButCannotTransfer(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "before", ExpiredTime: -1})
	require.NoError(t, err)

	updated, err := UpdateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{Name: "after", Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 10})

	require.NoError(t, err)
	require.Equal(t, "after", updated.Name)
	require.Equal(t, member.Id, updated.ResponsibleUserId)
	require.NotZero(t, updated.UpdatedAt)
}

func TestOrganizationTokenMemberCannotUpdateResponsiblePublicKey(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "member-public", ExpiredTime: -1, Visibility: model.TokenVisibilityPublic})
	require.NoError(t, err)

	_, err = UpdateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusDisabled, ExpiredTime: -1, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: admin.Id, UnlimitedQuota: true})

	require.ErrorContains(t, err, "permission denied")
	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	require.Equal(t, common.TokenStatusEnabled, stored.Status)
	require.Equal(t, model.TokenVisibilityPublic, stored.Visibility)
}

func TestOrganizationTokenMemberCannotEnableKeyDisabledByOwnerOrAdmin(t *testing.T) {
	for _, tc := range []struct {
		name         string
		orgAdmin     bool
		platformRole int
		username     string
	}{
		{name: "owner", username: "owner-disabled-key"},
		{name: "organization admin", orgAdmin: true, username: "admin-disabled-key"},
		{name: "platform admin", platformRole: common.RoleAdminUser, username: "platform-admin-disabled-key"},
		{name: "platform root", platformRole: common.RoleRootUser, username: "platform-root-disabled-key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner, member, organization := createOrganizationTokenTestOrg(t)
			disabler := owner
			if tc.orgAdmin {
				disabler = createServiceTestUser(t, tc.username, common.RoleCommonUser)
				require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: disabler.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
			}
			if tc.platformRole != 0 {
				disabler = createServiceTestUser(t, tc.username, tc.platformRole)
			}
			disablerAccessMode := OrganizationAccessModeWorkspace
			if tc.platformRole != 0 {
				disablerAccessMode = OrganizationAccessModeAdmin
			}
			token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: tc.username, ExpiredTime: -1, RemainQuota: 10})
			require.NoError(t, err)
			_, err = UpdateOrganizationToken(disabler.Id, organization.Id, disablerAccessMode, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusDisabled, ExpiredTime: -1, RemainQuota: 10})
			require.NoError(t, err)
			var blocker model.OrganizationTokenSystemBlocker
			require.NoError(t, model.DB.Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonManualDisabled, model.OrganizationTokenBlockerStatusActive).First(&blocker).Error)
			require.Equal(t, disabler.Id, blocker.OperatorUserId)
			if tc.platformRole != 0 {
				require.Equal(t, organizationTokenBlockerClearancePlatform, blocker.ClearanceLevel)
			} else {
				require.Equal(t, organizationTokenBlockerClearanceOrganizationAdmin, blocker.ClearanceLevel)
			}

			_, err = UpdateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 10})

			require.ErrorIs(t, err, ErrOrganizationTokenEnableForbidden)
			if tc.platformRole != 0 {
				_, err = UpdateOrganizationToken(owner.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 10})
				require.ErrorIs(t, err, ErrOrganizationTokenEnableForbidden)
			}
			var stored model.Token
			require.NoError(t, model.DB.First(&stored, token.Id).Error)
			require.Equal(t, common.TokenStatusDisabled, stored.Status)
			require.Equal(t, member.Id, stored.ResponsibleUserId)

			_, err = UpdateOrganizationToken(disabler.Id, organization.Id, disablerAccessMode, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 10})
			require.NoError(t, err)
			require.NoError(t, model.DB.First(&stored, token.Id).Error)
			require.Equal(t, common.TokenStatusEnabled, stored.Status)
			var activeBlockerCount int64
			require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonManualDisabled, model.OrganizationTokenBlockerStatusActive).Count(&activeBlockerCount).Error)
			require.EqualValues(t, 0, activeBlockerCount)
		})
	}
}

func TestOrganizationTokenEnableReturnsActionableBlockerError(t *testing.T) {
	testCases := []struct {
		name       string
		addBlocker func(t *testing.T, owner model.User, member model.User, organization *model.Organization, token *model.Token)
		operator   func(owner model.User, member model.User) model.User
		wantErr    error
	}{
		{
			name: "responsible member disabled",
			addBlocker: func(t *testing.T, owner model.User, member model.User, organization *model.Organization, _ *model.Token) {
				require.NoError(t, UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeWorkspace, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusDisabled, Reason: "inactive", IdempotencyKey: testOrganizationIdempotencyKey(t, "disable-member-for-key-enable")}))
			},
			operator: func(owner model.User, _ model.User) model.User { return owner },
			wantErr:  ErrOrganizationTokenResponsibleMemberDisabled,
		},
		{
			name: "responsible platform user disabled",
			addBlocker: func(t *testing.T, owner model.User, member model.User, _ *model.Organization, _ *model.Token) {
				require.NoError(t, UpdateUserStatusAndOrganizationTokenBlockers(member.Id, common.UserStatusDisabled, owner.Id))
			},
			operator: func(owner model.User, _ model.User) model.User { return owner },
			wantErr:  ErrOrganizationTokenResponsibleUserDisabled,
		},
		{
			name: "manual blocker requires higher clearance",
			addBlocker: func(t *testing.T, owner model.User, _ model.User, _ *model.Organization, token *model.Token) {
				now := common.GetTimestamp()
				require.NoError(t, ensureOrganizationTokenSystemBlockerWithTx(model.DB, token, organizationTokenBlockerReasonManualDisabled, organizationTokenBlockerRefTypeOperator, owner.Id, owner.Id, organizationTokenBlockerClearanceOrganizationAdmin, now))
				require.NoError(t, reconcileOrganizationTokenSystemBlockerWithTx(model.DB, token, now))
			},
			operator: func(_ model.User, member model.User) model.User { return member },
			wantErr:  ErrOrganizationTokenEnableForbidden,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			owner, member, organization := createOrganizationTokenTestOrg(t)
			token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "blocked-enable", ExpiredTime: -1, RemainQuota: 10})
			require.NoError(t, err)
			tc.addBlocker(t, owner, member, organization, token)

			operator := tc.operator(owner, member)
			_, err = UpdateOrganizationToken(operator.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 10})

			require.ErrorIs(t, err, tc.wantErr)
			var stored model.Token
			require.NoError(t, model.DB.First(&stored, token.Id).Error)
			require.Equal(t, common.TokenStatusDisabled, stored.Status)
			var activeBlockerCount int64
			require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).Where("token_id = ? AND status = ?", token.Id, model.OrganizationTokenBlockerStatusActive).Count(&activeBlockerCount).Error)
			require.EqualValues(t, 1, activeBlockerCount)
		})
	}
}

func TestOrganizationTokenMemberCanEnableKeyDisabledBySelf(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "member-self-disabled-key", ExpiredTime: -1, RemainQuota: 10})
	require.NoError(t, err)

	_, err = UpdateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusDisabled, ExpiredTime: -1, RemainQuota: 10})
	require.NoError(t, err)
	var blocker model.OrganizationTokenSystemBlocker
	require.NoError(t, model.DB.Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonManualDisabled, model.OrganizationTokenBlockerStatusActive).First(&blocker).Error)
	require.Equal(t, member.Id, blocker.OperatorUserId)
	require.Equal(t, organizationTokenBlockerClearanceMember, blocker.ClearanceLevel)

	_, err = UpdateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 10})

	require.NoError(t, err)
	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	require.Equal(t, common.TokenStatusEnabled, stored.Status)
	var activeBlockerCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonManualDisabled, model.OrganizationTokenBlockerStatusActive).Count(&activeBlockerCount).Error)
	require.EqualValues(t, 0, activeBlockerCount)
}

func TestOrganizationTokenManualDisableWhileSystemBlockedKeepsKeyDisabledAfterSystemRestore(t *testing.T) {
	admin, _, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "manual-while-system-blocked", ExpiredTime: -1, RemainQuota: 10})
	require.NoError(t, err)
	now := common.GetTimestamp()
	require.NoError(t, ensureOrganizationTokenSystemBlockerWithTx(model.DB, token, organizationTokenBlockerReasonOrganizationPlatform, organizationTokenBlockerRefTypeDisableRecord, 1, admin.Id, organizationTokenBlockerClearancePlatform, now))
	require.NoError(t, reconcileOrganizationTokenSystemBlockerWithTx(model.DB, token, now))

	_, err = UpdateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{Name: token.Name, Status: common.TokenStatusDisabled, ExpiredTime: -1, RemainQuota: 10})
	require.NoError(t, err)

	var manualBlocker model.OrganizationTokenSystemBlocker
	require.NoError(t, model.DB.Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonManualDisabled, model.OrganizationTokenBlockerStatusActive).First(&manualBlocker).Error)
	require.Equal(t, common.TokenStatusEnabled, manualBlocker.PreviousStatus)

	require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonOrganizationPlatform, model.OrganizationTokenBlockerStatusActive).Updates(map[string]any{"status": model.OrganizationTokenBlockerStatusCleared, "cleared_at": now, "updated_at": now}).Error)
	require.NoError(t, reconcileOrganizationTokenSystemBlockerWithTx(model.DB, token, now))
	stored, err := GetOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id)
	require.NoError(t, err)
	require.Equal(t, common.TokenStatusDisabled, stored.Status)
	require.False(t, stored.DisabledBySystems)
	require.Contains(t, stored.UnavailableReasons, organizationTokenBlockerReasonManualDisabled)
}

func TestOrganizationTokenMemberCannotDeleteResponsiblePublicKey(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "member-public-delete", ExpiredTime: -1, Visibility: model.TokenVisibilityPublic})
	require.NoError(t, err)

	err = DeleteOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id)

	require.ErrorContains(t, err, "permission denied")
	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	require.Equal(t, token.Id, stored.Id)
}

func TestOrganizationTokenMemberCannotBatchDeleteResponsiblePublicKey(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	privateToken, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "member-private-batch", ExpiredTime: -1, Visibility: model.TokenVisibilityPrivate})
	require.NoError(t, err)
	publicToken, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "member-public-batch-delete", ExpiredTime: -1, Visibility: model.TokenVisibilityPublic})
	require.NoError(t, err)

	_, err = BatchDeleteOrganizationTokens(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenBatchDeleteRequest{Ids: []int{privateToken.Id, publicToken.Id}, IdempotencyKey: testOrganizationIdempotencyKey(t, "batch-delete")})

	require.ErrorContains(t, err, "permission denied")
	var count int64
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id IN ?", []int{privateToken.Id, publicToken.Id}).Count(&count).Error)
	require.EqualValues(t, 2, count)
}

func TestOrganizationTokenManualStatusUpdateClearsDisabledBySystems(t *testing.T) {
	admin, _, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "system-disabled", ExpiredTime: -1})
	require.NoError(t, err)
	require.NoError(t, DisableOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "maintenance"))

	stored, err := GetOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeReadOnly, token.Id)
	require.NoError(t, err)
	require.True(t, stored.DisabledBySystems)

	require.NoError(t, EnableOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "resume"))
	stored, err = GetOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id)
	require.NoError(t, err)
	require.False(t, stored.DisabledBySystems)

	updated, err := UpdateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{
		Name:               stored.Name,
		Status:             common.TokenStatusDisabled,
		ExpiredTime:        stored.ExpiredTime,
		RemainQuota:        stored.RemainQuota,
		UnlimitedQuota:     stored.UnlimitedQuota,
		ModelLimitsEnabled: stored.ModelLimitsEnabled,
		ModelLimits:        stored.ModelLimits,
		AllowIps:           stored.AllowIps,
		Group:              stored.Group,
		CrossGroupRetry:    stored.CrossGroupRetry,
		Visibility:         stored.Visibility,
		ResponsibleUserId:  stored.ResponsibleUserId,
	})

	require.NoError(t, err)
	require.False(t, updated.DisabledBySystems)

	var reloaded model.Token
	require.NoError(t, model.DB.First(&reloaded, token.Id).Error)
	require.Equal(t, common.TokenStatusDisabled, reloaded.Status)
	require.False(t, reloaded.DisabledBySystems)
}

func TestOrganizationTokenUpdateRenamesExistingKey(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "original-name", ExpiredTime: -1})
	require.NoError(t, err)

	updated, err := UpdateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{Name: "attempted-rename", Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 10})

	require.NoError(t, err)
	require.Equal(t, "attempted-rename", updated.Name)
	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	require.Equal(t, "attempted-rename", stored.Name)
}

func TestOrganizationTokenMemberCannotUpdateOtherResponsibleKey(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "admin-key", ExpiredTime: -1})
	require.NoError(t, err)

	_, err = UpdateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{Name: "try", Status: common.TokenStatusEnabled, ExpiredTime: -1})

	require.ErrorContains(t, err, "permission denied")
}

func TestOrganizationTokenDeleteRemovesKeyForAdminAndResponsibleMember(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	memberToken, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "delete-member", ExpiredTime: -1})
	require.NoError(t, err)
	adminToken, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "delete-admin", ExpiredTime: -1})
	require.NoError(t, err)

	require.ErrorContains(t, DeleteOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, adminToken.Id), "permission denied")
	require.NoError(t, DeleteOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, memberToken.Id))
	require.NoError(t, DeleteOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, adminToken.Id))

	_, err = GetOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, memberToken.Id)
	require.Error(t, err)
	_, err = GetOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, adminToken.Id)
	require.Error(t, err)
}

func TestOrganizationTokenListFiltersByKeywordStatusVisibilityGroupAndResponsible(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	allowIps := "127.0.0.1"
	memberToken, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "alpha-key", ExpiredTime: -1, Status: common.TokenStatusEnabled, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: admin.Id, Group: "default", AllowIps: &allowIps})
	require.NoError(t, err)
	adminToken, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "beta-key", ExpiredTime: -1, Visibility: model.TokenVisibilityPrivate})
	require.NoError(t, err)
	otherOrg, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Other Token Org"})
	require.NoError(t, err)
	_, err = CreateOrganizationToken(admin.Id, otherOrg.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "alpha-key", ExpiredTime: -1})
	require.NoError(t, err)

	tokens, total, err := ListOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenListRequest{Limit: 20, Keyword: "alpha-key", Status: common.TokenStatusEnabled, Visibility: model.TokenVisibilityPublic, Group: "default", ResponsibleUserId: admin.Id})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, tokens, 1)
	require.Equal(t, memberToken.Id, tokens[0].Id)

	tokens, total, err = ListOrganizationTokens(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenListRequest{Limit: 20, ResponsibleUserId: admin.Id})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, tokens, 1)
	require.Equal(t, memberToken.Id, tokens[0].Id)
	require.NotEqual(t, adminToken.Id, tokens[0].Id)
}

func TestOrganizationTokenListGroupFilterMatchesImplicitOrganizationGroup(t *testing.T) {
	admin, _, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "implicit-default-key", ExpiredTime: -1})
	require.NoError(t, err)
	require.Empty(t, token.Group)

	tokens, total, err := ListOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenListRequest{Limit: 20, Group: "default"})

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, tokens, 1)
	require.Equal(t, token.Id, tokens[0].Id)
}

func TestOrganizationTokenBatchCreateCreatesMultipleKeysAndAuditLogs(t *testing.T) {
	admin, _, organization := createOrganizationTokenTestOrg(t)
	allowIps := "10.0.0.1"
	result, err := BatchCreateOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenBatchCreateRequest{TokenCount: 3, IdempotencyKey: testOrganizationIdempotencyKey(t, "batch-create"), Token: OrganizationTokenRequest{Name: "batch", ExpiredTime: -1, UnlimitedQuota: true, ModelLimitsEnabled: true, ModelLimits: "gpt-4o", AllowIps: &allowIps, Group: "default", Visibility: model.TokenVisibilityPublic, ResponsibleUserId: admin.Id}})
	require.NoError(t, err)
	require.True(t, result.SecretAvailable)
	require.Equal(t, 3, result.TokenCount)
	tokens := result.Tokens
	require.Len(t, tokens, 3)
	for _, token := range tokens {
		require.Contains(t, token.Name, "batch-")
		require.NotEmpty(t, token.Key)
		require.Equal(t, admin.Id, token.ResponsibleUserId)
		require.True(t, token.UnlimitedQuota)
		require.Equal(t, "gpt-4o", token.ModelLimits)
		require.NotNil(t, token.AllowIps)
		require.Equal(t, allowIps, *token.AllowIps)
	}
	var auditCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationAuditLog{}).Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionTokenCreate).Count(&auditCount).Error)
	require.EqualValues(t, 3, auditCount)
}

func TestOrganizationTokenBatchCreateIdempotentReplayReturnsSameResult(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	req := OrganizationTokenBatchCreateRequest{
		TokenCount:     2,
		IdempotencyKey: "batch-create-idempotency",
		Token:          OrganizationTokenRequest{Name: "idempotent-batch", ExpiredTime: -1, ResponsibleUserId: member.Id},
	}

	first, err := BatchCreateOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, req)
	require.NoError(t, err)
	require.True(t, first.SecretAvailable)
	require.Equal(t, 2, first.TokenCount)
	require.Len(t, first.Tokens, 2)
	for _, token := range first.Tokens {
		require.NotEmpty(t, token.Key)
	}
	second, err := BatchCreateOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, req)
	require.NoError(t, err)
	// 组织 Key 完整 secret 每次返回，幂等重放同样返回完整 secret。
	require.True(t, second.SecretAvailable)
	require.Equal(t, 2, second.TokenCount)
	require.Len(t, second.Tokens, 2)
	for _, token := range second.Tokens {
		require.NotEmpty(t, token.Key)
		require.NotEmpty(t, token.KeyPreview)
		require.Contains(t, token.KeyPreview, "*")
	}

	var record model.OrganizationIdempotencyRecord
	require.NoError(t, model.DB.Where("idempotency_key = ?", req.IdempotencyKey).First(&record).Error)
	require.NotContains(t, record.ResultJson, first.Tokens[0].Key)
	require.NotContains(t, record.ResultJson, first.Tokens[1].Key)
	require.NotContains(t, record.ResultJson, `"key"`)
}

func TestOrganizationTokenBatchCreateReplayRechecksCurrentViewAuthorization(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	req := OrganizationTokenBatchCreateRequest{
		TokenCount:     1,
		IdempotencyKey: "batch-create-replay-auth",
		Token:          OrganizationTokenRequest{Name: "create-replay-auth", ExpiredTime: -1},
	}
	first, err := BatchCreateOrganizationTokens(member.Id, organization.Id, OrganizationAccessModeWorkspace, req)
	require.NoError(t, err)
	require.Len(t, first.Tokens, 1)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", first.Tokens[0].Id).Updates(map[string]any{
		"user_id":             admin.Id,
		"responsible_user_id": admin.Id,
	}).Error)

	_, err = BatchCreateOrganizationTokens(member.Id, organization.Id, OrganizationAccessModeWorkspace, req)

	require.ErrorContains(t, err, "permission denied")
}

func TestOrganizationTokenBatchCreateIdempotencyConflict(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	req := OrganizationTokenBatchCreateRequest{TokenCount: 1, IdempotencyKey: "batch-create-conflict", Token: OrganizationTokenRequest{Name: "batch-conflict", ExpiredTime: -1, ResponsibleUserId: member.Id}}
	_, err := BatchCreateOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, req)
	require.NoError(t, err)

	req.TokenCount = 2
	_, err = BatchCreateOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, req)

	require.ErrorContains(t, err, "organization idempotency conflict")
}

func TestOrganizationTokenBatchDeleteFailsWhenMemberIncludesUnauthorizedKey(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	memberToken, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "member-delete", ExpiredTime: -1})
	require.NoError(t, err)
	adminToken, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "admin-delete", ExpiredTime: -1})
	require.NoError(t, err)

	_, err = BatchDeleteOrganizationTokens(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenBatchDeleteRequest{Ids: []int{memberToken.Id, adminToken.Id}, IdempotencyKey: testOrganizationIdempotencyKey(t, "batch-delete")})
	require.ErrorContains(t, err, "permission denied")

	_, err = GetOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, memberToken.Id)
	require.NoError(t, err)
	_, err = GetOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, adminToken.Id)
	require.NoError(t, err)
}

func TestOrganizationTokenBatchDeleteAdminDeletesAndWritesPerTokenAudit(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	first, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "delete-one", ExpiredTime: -1})
	require.NoError(t, err)
	second, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "delete-two", ExpiredTime: -1})
	require.NoError(t, err)

	count, err := BatchDeleteOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenBatchDeleteRequest{Ids: []int{first.Id, second.Id}, IdempotencyKey: testOrganizationIdempotencyKey(t, "batch-delete")})
	require.NoError(t, err)
	require.Equal(t, 2, count)

	_, err = GetOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, first.Id)
	require.Error(t, err)
	_, err = GetOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, second.Id)
	require.Error(t, err)
	var auditCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationAuditLog{}).Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionTokenDelete).Count(&auditCount).Error)
	require.EqualValues(t, 2, auditCount)
}

func TestOrganizationTokenBatchDeleteIdempotentReplay(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	first, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "delete-idempotent-one", ExpiredTime: -1})
	require.NoError(t, err)
	second, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "delete-idempotent-two", ExpiredTime: -1})
	require.NoError(t, err)
	req := OrganizationTokenBatchDeleteRequest{Ids: []int{first.Id, second.Id}, IdempotencyKey: "batch-delete-idempotency"}

	count, err := BatchDeleteOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, req)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	count, err = BatchDeleteOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, req)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	_, err = GetOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, first.Id)
	require.Error(t, err)
	_, err = GetOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, second.Id)
	require.Error(t, err)
}

func TestOrganizationTokenBatchDeleteReplayRechecksCurrentResourceAuthorization(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "delete-replay-auth", ExpiredTime: -1})
	require.NoError(t, err)
	req := OrganizationTokenBatchDeleteRequest{Ids: []int{token.Id}, IdempotencyKey: "batch-delete-replay-auth"}
	count, err := BatchDeleteOrganizationTokens(member.Id, organization.Id, OrganizationAccessModeWorkspace, req)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.NoError(t, model.DB.Unscoped().Model(&model.Token{}).Where("id = ?", token.Id).Updates(map[string]any{
		"user_id":             admin.Id,
		"responsible_user_id": admin.Id,
	}).Error)

	_, err = BatchDeleteOrganizationTokens(member.Id, organization.Id, OrganizationAccessModeWorkspace, req)

	require.ErrorContains(t, err, "permission denied")
}

func TestOrganizationTokenBatchDeleteIdempotencyConflict(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	first, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "delete-conflict-one", ExpiredTime: -1})
	require.NoError(t, err)
	second, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "delete-conflict-two", ExpiredTime: -1})
	require.NoError(t, err)
	req := OrganizationTokenBatchDeleteRequest{Ids: []int{first.Id}, IdempotencyKey: "batch-delete-conflict"}
	_, err = BatchDeleteOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, req)
	require.NoError(t, err)

	req.Ids = []int{second.Id}
	_, err = BatchDeleteOrganizationTokens(admin.Id, organization.Id, OrganizationAccessModeWorkspace, req)

	require.ErrorContains(t, err, "organization idempotency conflict")
}
