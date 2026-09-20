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

func TestOrganizationTokenAuthPersonalTokenContext(t *testing.T) {
	setupServiceTestDB(t)
	user := createServiceTestUser(t, "org-token-auth-personal", common.RoleCommonUser)
	token := &model.Token{UserId: user.Id, ScopeType: model.TokenScopePersonal}

	ctx, err := ValidateTokenScopeForRelay(token)

	require.NoError(t, err)
	require.Equal(t, model.AccountContextTypePersonal, ctx.ScopeType)
	require.Equal(t, user.Id, ctx.ScopeId)
	require.Equal(t, model.AccountContextTypePersonal, ctx.BillingAccountType)
	require.Equal(t, user.Id, ctx.BillingAccountId)
	require.Equal(t, user.Id, ctx.ActorUserId)
}

func TestOrganizationTokenAuthDissolvedOrganizationIsRejected(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "auth-key", ExpiredTime: -1})
	require.NoError(t, err)
	require.NoError(t, dissolveOrganizationForTest(t, admin.Id, organization.Id, "done"))

	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	require.Equal(t, common.TokenStatusDisabled, stored.Status)
	_, err = ValidateTokenScopeForRelay(&stored)

	require.ErrorContains(t, err, "organization dissolved")
}

func TestOrganizationTokenAuthDisabledResponsiblePrivateKeyIsPreservedAndBlocked(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "auth-key", ExpiredTime: -1})
	require.NoError(t, err)
	require.NoError(t, UpdateOrganizationMember(admin.Id, organization.Id, OrganizationAccessModeManagement, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusDisabled, Reason: "inactive", IdempotencyKey: testOrganizationIdempotencyKey(t, "disable-member")}))

	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	require.False(t, stored.DeletedAt.Valid)
	require.Equal(t, common.TokenStatusDisabled, stored.Status)
	require.True(t, stored.DisabledBySystems)
	require.Equal(t, organizationTokenBlockerReasonMemberDisabled, stored.SystemDisabledReason)
}

func TestOrganizationTokenAuthMissingResponsibleUserIsRejected(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "auth-missing-user-key", ExpiredTime: -1})
	require.NoError(t, err)
	require.NoError(t, model.DB.Delete(&member).Error)

	_, err = ValidateTokenScopeForRelay(token)

	require.ErrorContains(t, err, "organization responsible user disabled")
}

func TestOrganizationTokenAuthActiveBlockerIsRejected(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "auth-blocked-key", ExpiredTime: -1})
	require.NoError(t, err)
	now := common.GetTimestamp()
	require.NoError(t, model.DB.Create(&model.OrganizationTokenSystemBlocker{
		TokenId:        token.Id,
		OrganizationId: organization.Id,
		Reason:         organizationTokenBlockerReasonManualDisabled,
		RefType:        organizationTokenBlockerRefTypeOperator,
		RefId:          member.Id,
		Status:         model.OrganizationTokenBlockerStatusActive,
		PreviousStatus: common.TokenStatusEnabled,
		DisabledAt:     now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}).Error)
	require.NoError(t, reconcileOrganizationTokenSystemBlockerWithTx(model.DB, token, now))

	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	require.Equal(t, common.TokenStatusDisabled, stored.Status)
	_, err = ValidateTokenScopeForRelay(&stored)

	require.ErrorContains(t, err, "organization token is blocked")
}

func TestOrganizationTokenAuthRejectsPublicKeyResponsibleMember(t *testing.T) {
	owner, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(owner.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "auth-public-member-key", ExpiredTime: -1, Visibility: model.TokenVisibilityPublic})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).Updates(map[string]any{"user_id": member.Id, "responsible_user_id": member.Id}).Error)
	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)

	_, err = ValidateTokenScopeForRelay(&stored)

	require.EqualError(t, err, "public token responsible user must be organization owner or admin")
}

func TestOrganizationTokenPlatformUserDisableBlocksResponsibleKeys(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "platform-user-disabled-key", ExpiredTime: -1, ResponsibleUserId: member.Id})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", member.Id).Update("status", common.UserStatusDisabled).Error)

	require.NoError(t, UpdateOrganizationTokenBlockersForUserStatus(member.Id, common.UserStatusDisabled, admin.Id))

	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	require.Equal(t, common.TokenStatusDisabled, stored.Status)
	var blocker model.OrganizationTokenSystemBlocker
	require.NoError(t, model.DB.Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonUserPlatformDisabled, model.OrganizationTokenBlockerStatusActive).First(&blocker).Error)
	require.Equal(t, organizationTokenBlockerClearancePlatform, blocker.ClearanceLevel)

	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", member.Id).Update("status", common.UserStatusEnabled).Error)
	require.NoError(t, UpdateOrganizationTokenBlockersForUserStatus(member.Id, common.UserStatusEnabled, admin.Id))
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	require.Equal(t, common.TokenStatusEnabled, stored.Status)
}

func TestOrganizationTokenAuthWritesCompleteScopeContext(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "auth-key", ExpiredTime: -1, Visibility: model.TokenVisibilityPrivate, ResponsibleUserId: member.Id})
	require.NoError(t, err)

	ctx, err := ValidateTokenScopeForRelay(token)

	require.NoError(t, err)
	require.Equal(t, model.AccountContextTypeOrganization, ctx.ScopeType)
	require.Equal(t, organization.Id, ctx.ScopeId)
	require.Equal(t, model.AccountContextTypeOrganization, ctx.BillingAccountType)
	require.Equal(t, organization.Id, ctx.BillingAccountId)
	require.Equal(t, organization.Id, ctx.OrganizationId)
	require.Equal(t, member.Id, ctx.ResponsibleUserId)
	require.Equal(t, member.Id, ctx.ActorUserId)
	require.Equal(t, admin.Id, ctx.CreatorUserId)
	require.Equal(t, organization.Quota-organization.UsedQuota, ctx.OrganizationQuota)
}
