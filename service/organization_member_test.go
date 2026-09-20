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
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func createOrganizationMemberTestOrg(t *testing.T) (model.User, *model.Organization) {
	t.Helper()
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "org-member-admin", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Member Org"})
	require.NoError(t, err)
	return admin, organization
}

func TestAddOrganizationMemberLocksOrganizationThenOperatorAndTargetMember(t *testing.T) {
	_, organization := createOrganizationMemberTestOrg(t)
	platformAdmin := createServiceTestUser(t, "member-add-lock-platform-admin", common.RoleAdminUser)
	target := createServiceTestUser(t, "member-add-lock-target", common.RoleCommonUser)
	lockOrder := make([]string, 0, 3)
	callbackName := "test:organization-member-add-lock-order"
	require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		forClause, ok := tx.Statement.Clauses["FOR"]
		if !ok {
			return
		}
		locking, ok := forClause.Expression.(clause.Locking)
		if !ok || locking.Strength != "UPDATE" {
			return
		}
		switch tx.Statement.Table {
		case "organizations":
			lockOrder = append(lockOrder, "organization")
		case "organization_members":
			lockOrder = append(lockOrder, "member")
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Query().Remove(callbackName) })

	_, err := AddOrganizationMember(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, AddMemberRequest{UserId: target.Id, Role: model.OrganizationRoleMember, Reason: "add"})

	require.NoError(t, err)
	requireOrganizationRowLockOrder(t, lockOrder, []string{"organization", "member", "member"})
}

func TestOrganizationMemberAdminCanDisableMember(t *testing.T) {
	admin, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "member-disable", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	err := UpdateOrganizationMember(admin.Id, organization.Id, OrganizationAccessModeManagement, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusDisabled, Reason: "inactive", IdempotencyKey: testOrganizationIdempotencyKey(t, "disable-member")})
	require.NoError(t, err)

	var stored model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).First(&stored).Error)
	require.Equal(t, model.OrganizationMemberStatusDisabled, stored.Status)
	require.Equal(t, model.OrganizationMemberDisableSourceOrganization, stored.DisabledSource)
	require.NotZero(t, stored.DisabledAt)
}

func TestOrganizationMemberPlatformDisableUsesPlatformClearance(t *testing.T) {
	_, organization := createOrganizationMemberTestOrg(t)
	platformAdmin := createServiceTestUser(t, "member-platform-disable-admin", common.RoleAdminUser)
	member := createServiceTestUser(t, "member-platform-disable-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	token := model.Token{UserId: member.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPrivate, ResponsibleUserId: member.Id}
	require.NoError(t, model.DB.Create(&token).Error)

	require.NoError(t, UpdateOrganizationMember(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusDisabled, Reason: "platform disable", IdempotencyKey: testOrganizationIdempotencyKey(t, "platform-disable-member")}))

	var blocker model.OrganizationTokenSystemBlocker
	require.NoError(t, model.DB.Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonMemberDisabled, model.OrganizationTokenBlockerStatusActive).First(&blocker).Error)
	require.Equal(t, organizationTokenBlockerClearancePlatform, blocker.ClearanceLevel)
	var stored model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).First(&stored).Error)
	require.Equal(t, model.OrganizationMemberDisableSourcePlatform, stored.DisabledSource)
}

func TestOrganizationMemberPlatformDisableCannotBeClearedByOrganizationOwner(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	platformAdmin := createServiceTestUser(t, "member-platform-priority-admin", common.RoleAdminUser)
	member := createServiceTestUser(t, "member-platform-priority-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	require.NoError(t, UpdateOrganizationMember(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusDisabled, Reason: "platform disable", IdempotencyKey: testOrganizationIdempotencyKey(t, "platform-disable-member")}))
	err := UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusActive, Reason: "organization enable"})

	require.ErrorContains(t, err, "organization member disabled by platform")
	var stored model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).First(&stored).Error)
	require.Equal(t, model.OrganizationMemberStatusDisabled, stored.Status)
	require.NoError(t, UpdateOrganizationMember(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusActive, Reason: "platform enable"}))
	require.NoError(t, model.DB.First(&stored, stored.Id).Error)
	require.Equal(t, model.OrganizationMemberStatusActive, stored.Status)
	require.Empty(t, stored.DisabledSource)
}

func TestOrganizationMemberPlatformDisableCannotBeRemovedByOrganizationOwner(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	platformAdmin := createServiceTestUser(t, "member-platform-remove-admin", common.RoleAdminUser)
	member := createServiceTestUser(t, "member-platform-remove-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	require.NoError(t, UpdateOrganizationMember(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusDisabled, Reason: "platform disable", IdempotencyKey: testOrganizationIdempotencyKey(t, "platform-disable-member")}))
	err := RemoveOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, RemoveMemberRequest{Reason: "organization remove", IdempotencyKey: testOrganizationIdempotencyKey(t, "organization-remove-member")})

	require.ErrorContains(t, err, "organization member disabled by platform")
	var stored model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).First(&stored).Error)
	require.Equal(t, model.OrganizationMemberStatusDisabled, stored.Status)
}

func TestOrganizationMemberPlatformDisableUpgradesOrganizationBlocker(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	platformAdmin := createServiceTestUser(t, "member-platform-upgrade-admin", common.RoleAdminUser)
	member := createServiceTestUser(t, "member-platform-upgrade-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	token := model.Token{UserId: member.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPrivate, ResponsibleUserId: member.Id}
	require.NoError(t, model.DB.Create(&token).Error)

	require.NoError(t, UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusDisabled, Reason: "organization disable", IdempotencyKey: testOrganizationIdempotencyKey(t, "organization-disable-member")}))
	require.NoError(t, UpdateOrganizationMember(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusDisabled, Reason: "platform takeover", IdempotencyKey: testOrganizationIdempotencyKey(t, "platform-disable-member")}))

	var blocker model.OrganizationTokenSystemBlocker
	require.NoError(t, model.DB.Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonMemberDisabled, model.OrganizationTokenBlockerStatusActive).First(&blocker).Error)
	require.Equal(t, organizationTokenBlockerClearancePlatform, blocker.ClearanceLevel)
	var stored model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).First(&stored).Error)
	require.Equal(t, model.OrganizationMemberDisableSourcePlatform, stored.DisabledSource)
}

func TestOrganizationMemberDisablePreservesAndDisablesAllOwnedKeys(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "member-disable-preserve-keys", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	tokens := []model.Token{
		{UserId: member.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: member.Id},
		{UserId: member.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPrivate, ResponsibleUserId: member.Id},
	}
	require.NoError(t, model.DB.Create(&tokens).Error)

	err := UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusDisabled, TransferToUserId: owner.Id, Reason: "inactive", IdempotencyKey: testOrganizationIdempotencyKey(t, "disable-member")})

	require.NoError(t, err)
	for _, token := range tokens {
		var stored model.Token
		require.NoError(t, model.DB.First(&stored, token.Id).Error)
		require.Equal(t, member.Id, stored.UserId)
		require.Equal(t, member.Id, stored.ResponsibleUserId)
		require.Equal(t, common.TokenStatusDisabled, stored.Status)
		var activeCount int64
		require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonMemberDisabled, model.OrganizationTokenBlockerStatusActive).Count(&activeCount).Error)
		require.EqualValues(t, 1, activeCount)
	}
}

func TestOrganizationMemberDisableLocksResponsibleKeysBeforeCreatingBlockers(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "member-disable-lock-keys", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	token := model.Token{UserId: member.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPrivate, ResponsibleUserId: member.Id}
	require.NoError(t, model.DB.Create(&token).Error)
	queriedTokens := false
	lockedTokens := false
	callbackName := "test:organization-member-disable-lock-keys"
	require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != "tokens" || queriedTokens {
			return
		}
		queriedTokens = true
		forClause, ok := tx.Statement.Clauses["FOR"]
		if !ok {
			return
		}
		locking, ok := forClause.Expression.(clause.Locking)
		lockedTokens = ok && locking.Strength == "UPDATE"
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Query().Remove(callbackName) })

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.OrganizationMember{}).Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).Update("status", model.OrganizationMemberStatusDisabled).Error; err != nil {
			return err
		}
		_, err := createOrganizationMemberDisabledTokenBlockersWithTx(tx, organization.Id, member.Id, owner.Id, model.OrganizationMemberDisableSourceOrganization, common.GetTimestamp())
		return err
	})

	require.NoError(t, err)
	require.True(t, queriedTokens)
	requireOrganizationRowLock(t, lockedTokens, "disabling a member must lock the member's keys before creating blockers")
}

func TestOrganizationMemberBlockerHelpersReturnOnlyAffectedTokenKeys(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "member-exact-cache-keys", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusDisabled}).Error)
	memberToken := model.Token{UserId: member.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPrivate, ResponsibleUserId: member.Id}
	unrelatedToken := model.Token{UserId: owner.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: owner.Id}
	require.NoError(t, model.DB.Create(&memberToken).Error)
	require.NoError(t, model.DB.Create(&unrelatedToken).Error)

	var disabledKeys []string
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		disabledKeys, err = createOrganizationMemberDisabledTokenBlockersWithTx(tx, organization.Id, member.Id, owner.Id, model.OrganizationMemberDisableSourceOrganization, common.GetTimestamp())
		return err
	})
	require.NoError(t, err)
	require.Equal(t, []string{memberToken.Key}, disabledKeys)

	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).
		Update("status", model.OrganizationMemberStatusActive).Error)
	var enabledKeys []string
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		enabledKeys, err = clearOrganizationMemberDisabledTokenBlockersWithTx(tx, organization.Id, member.Id, common.GetTimestamp())
		return err
	})
	require.NoError(t, err)
	require.Equal(t, []string{memberToken.Key}, enabledKeys)
}

func TestOrganizationMemberEnableRestoresOwnedKeyStatuses(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "member-enable-restore-status", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	statuses := []int{common.TokenStatusEnabled, common.TokenStatusDisabled, common.TokenStatusExpired, common.TokenStatusExhausted}
	tokens := make([]model.Token, 0, len(statuses))
	for _, status := range statuses {
		tokens = append(tokens, model.Token{UserId: member.Id, Key: common.GetUUID(), Status: status, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: member.Id})
	}
	require.NoError(t, model.DB.Create(&tokens).Error)

	require.NoError(t, UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusDisabled, Reason: "inactive", IdempotencyKey: testOrganizationIdempotencyKey(t, "disable-member")}))
	require.NoError(t, UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusActive, Reason: "reactivate"}))

	var stored []model.Token
	require.NoError(t, model.DB.Where("organization_id = ? AND responsible_user_id = ?", organization.Id, member.Id).Order("id asc").Find(&stored).Error)
	require.Len(t, stored, len(statuses))
	for i := range stored {
		require.Equal(t, member.Id, stored[i].UserId)
		require.Equal(t, member.Id, stored[i].ResponsibleUserId)
		require.Equal(t, statuses[i], stored[i].Status)
	}
}

func TestReconcileOrganizationTokenBlockersClearsStaleSystemBlockers(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "stale-system-blocker-member", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	now := common.GetTimestamp()
	memberToken := model.Token{UserId: member.Id, Key: common.GetUUID(), Status: common.TokenStatusDisabled, PreviousStatus: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, ResponsibleUserId: member.Id}
	userToken := model.Token{UserId: member.Id, Key: common.GetUUID(), Status: common.TokenStatusDisabled, PreviousStatus: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, ResponsibleUserId: member.Id}
	manualToken := model.Token{UserId: owner.Id, Key: common.GetUUID(), Status: common.TokenStatusDisabled, PreviousStatus: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, ResponsibleUserId: owner.Id}
	require.NoError(t, model.DB.Create(&memberToken).Error)
	require.NoError(t, model.DB.Create(&userToken).Error)
	require.NoError(t, model.DB.Create(&manualToken).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationTokenSystemBlocker{TokenId: memberToken.Id, OrganizationId: organization.Id, Reason: organizationTokenBlockerReasonMemberDisabled, RefType: organizationTokenBlockerRefTypeMember, RefId: member.Id, Status: model.OrganizationTokenBlockerStatusActive, PreviousStatus: common.TokenStatusEnabled, DisabledAt: now, CreatedAt: now, UpdatedAt: now}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationTokenSystemBlocker{TokenId: userToken.Id, OrganizationId: organization.Id, Reason: organizationTokenBlockerReasonUserPlatformDisabled, RefType: organizationTokenBlockerRefTypeUser, RefId: member.Id, Status: model.OrganizationTokenBlockerStatusActive, PreviousStatus: common.TokenStatusEnabled, DisabledAt: now, CreatedAt: now, UpdatedAt: now}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationTokenSystemBlocker{TokenId: manualToken.Id, OrganizationId: organization.Id, Reason: organizationTokenBlockerReasonManualDisabled, RefType: organizationTokenBlockerRefTypeOperator, RefId: owner.Id, Status: model.OrganizationTokenBlockerStatusActive, PreviousStatus: common.TokenStatusEnabled, DisabledAt: now, CreatedAt: now, UpdatedAt: now}).Error)

	require.NoError(t, ReconcileOrganizationTokenBlockers())

	var activeSystemBlockers int64
	require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).Where("token_id IN ? AND status = ?", []int{memberToken.Id, userToken.Id}, model.OrganizationTokenBlockerStatusActive).Count(&activeSystemBlockers).Error)
	require.EqualValues(t, 0, activeSystemBlockers)
	var activeManualBlockers int64
	require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).Where("token_id = ? AND status = ?", manualToken.Id, model.OrganizationTokenBlockerStatusActive).Count(&activeManualBlockers).Error)
	require.EqualValues(t, 1, activeManualBlockers)
	require.NoError(t, model.DB.First(&memberToken, memberToken.Id).Error)
	require.NoError(t, model.DB.First(&userToken, userToken.Id).Error)
	require.NoError(t, model.DB.First(&manualToken, manualToken.Id).Error)
	require.Equal(t, common.TokenStatusEnabled, memberToken.Status)
	require.Equal(t, common.TokenStatusEnabled, userToken.Status)
	require.Equal(t, common.TokenStatusDisabled, manualToken.Status)
}

func TestUpdateUserStatusAndOrganizationTokenBlockersUpdatesUserAndTokenTogether(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "atomic-user-status-member", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	token := model.Token{UserId: member.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 10, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, ResponsibleUserId: member.Id}
	require.NoError(t, model.DB.Create(&token).Error)

	require.NoError(t, UpdateUserStatusAndOrganizationTokenBlockers(member.Id, common.UserStatusDisabled, owner.Id))

	var storedUser model.User
	require.NoError(t, model.DB.First(&storedUser, member.Id).Error)
	require.Equal(t, common.UserStatusDisabled, storedUser.Status)
	require.NoError(t, model.DB.First(&token, token.Id).Error)
	require.Equal(t, common.TokenStatusDisabled, token.Status)
	var activeBlocker model.OrganizationTokenSystemBlocker
	require.NoError(t, model.DB.Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonUserPlatformDisabled, model.OrganizationTokenBlockerStatusActive).First(&activeBlocker).Error)

	require.NoError(t, UpdateUserStatusAndOrganizationTokenBlockers(member.Id, common.UserStatusEnabled, owner.Id))

	require.NoError(t, model.DB.First(&storedUser, member.Id).Error)
	require.Equal(t, common.UserStatusEnabled, storedUser.Status)
	require.NoError(t, model.DB.First(&token, token.Id).Error)
	require.Equal(t, common.TokenStatusEnabled, token.Status)
	var activeCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonUserPlatformDisabled, model.OrganizationTokenBlockerStatusActive).Count(&activeCount).Error)
	require.EqualValues(t, 0, activeCount)
}

func TestOrganizationMemberDisabledMemberCannotUseActivePermission(t *testing.T) {
	_, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "disabled-permission", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusDisabled}).Error)

	err := UpdateOrganizationMember(member.Id, organization.Id, OrganizationAccessModeManagement, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusActive})
	require.Error(t, err)
	require.Contains(t, err.Error(), "permission denied")
}

func TestOrganizationMemberCannotUpdateMember(t *testing.T) {
	_, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "member-cannot-update", common.RoleCommonUser)
	target := createServiceTestUser(t, "member-cannot-update-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	err := UpdateOrganizationMember(member.Id, organization.Id, OrganizationAccessModeManagement, target.Id, UpdateMemberRequest{Role: model.OrganizationRoleAdmin})

	require.ErrorContains(t, err, "permission denied")
}

func TestOrganizationMemberCannotRemoveMember(t *testing.T) {
	_, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "member-cannot-remove", common.RoleCommonUser)
	target := createServiceTestUser(t, "member-cannot-remove-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	err := RemoveOrganizationMember(member.Id, organization.Id, OrganizationAccessModeManagement, target.Id, RemoveMemberRequest{Reason: "not allowed", IdempotencyKey: testOrganizationIdempotencyKey(t, "remove-member")})

	require.ErrorContains(t, err, "permission denied")
}

func TestOrganizationAdminCanPromoteMember(t *testing.T) {
	_, organization := createOrganizationMemberTestOrg(t)
	admin := createServiceTestUser(t, "member-promote-admin", common.RoleCommonUser)
	target := createServiceTestUser(t, "member-promote-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	targetMember := model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}
	require.NoError(t, model.DB.Create(&targetMember).Error)

	err := UpdateOrganizationMember(admin.Id, organization.Id, OrganizationAccessModeManagement, target.Id, UpdateMemberRequest{Role: model.OrganizationRoleAdmin, Reason: "delegate admin"})

	require.NoError(t, err)
	var stored model.OrganizationMember
	require.NoError(t, model.DB.First(&stored, targetMember.Id).Error)
	require.Equal(t, model.OrganizationRoleAdmin, stored.Role)
	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND target_id = ? AND action_type = ?", organization.Id, targetMember.Id, organizationAuditActionMemberUpdate).First(&audit).Error)
	require.Equal(t, model.OrganizationRoleAdmin, audit.OperatorRole)
	require.Contains(t, audit.AfterData, `"role":"admin"`)
}

func TestOrganizationAdminPromotionDisableIdempotencyReplay(t *testing.T) {
	_, organization := createOrganizationMemberTestOrg(t)
	admin := createServiceTestUser(t, "member-promote-disable-admin", common.RoleCommonUser)
	target := createServiceTestUser(t, "member-promote-disable-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	targetMember := model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}
	require.NoError(t, model.DB.Create(&targetMember).Error)
	req := UpdateMemberRequest{
		Role:           model.OrganizationRoleAdmin,
		Status:         model.OrganizationMemberStatusDisabled,
		Reason:         "promote and suspend",
		IdempotencyKey: testOrganizationIdempotencyKey(t, "promote-disable"),
	}

	require.NoError(t, UpdateOrganizationMember(admin.Id, organization.Id, OrganizationAccessModeManagement, target.Id, req))
	require.NoError(t, UpdateOrganizationMember(admin.Id, organization.Id, OrganizationAccessModeManagement, target.Id, req))

	var stored model.OrganizationMember
	require.NoError(t, model.DB.First(&stored, targetMember.Id).Error)
	require.Equal(t, model.OrganizationRoleAdmin, stored.Role)
	require.Equal(t, model.OrganizationMemberStatusDisabled, stored.Status)
	var auditCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationAuditLog{}).Where("organization_id = ? AND target_id = ? AND action_type = ?", organization.Id, targetMember.Id, organizationAuditActionMemberUpdate).Count(&auditCount).Error)
	require.EqualValues(t, 1, auditCount)
}

func TestOrganizationMemberDisableIdempotencyConflictsOnChangedRequest(t *testing.T) {
	_, organization := createOrganizationMemberTestOrg(t)
	admin := createServiceTestUser(t, "member-disable-conflict-admin", common.RoleCommonUser)
	target := createServiceTestUser(t, "member-disable-conflict-target", common.RoleCommonUser)
	otherTarget := createServiceTestUser(t, "member-disable-conflict-other-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: otherTarget.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	idempotencyKey := testOrganizationIdempotencyKey(t, "disable-conflict")
	require.NoError(t, UpdateOrganizationMember(admin.Id, organization.Id, OrganizationAccessModeManagement, target.Id, UpdateMemberRequest{
		Role:           model.OrganizationRoleAdmin,
		Status:         model.OrganizationMemberStatusDisabled,
		Reason:         "original",
		IdempotencyKey: idempotencyKey,
	}))

	testCases := []struct {
		name         string
		targetUserId int
		role         string
		reason       string
	}{
		{name: "reason", targetUserId: target.Id, role: model.OrganizationRoleAdmin, reason: "changed"},
		{name: "role", targetUserId: target.Id, role: model.OrganizationRoleMember, reason: "original"},
		{name: "target", targetUserId: otherTarget.Id, role: model.OrganizationRoleAdmin, reason: "original"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := UpdateOrganizationMember(admin.Id, organization.Id, OrganizationAccessModeManagement, tc.targetUserId, UpdateMemberRequest{
				Role:           tc.role,
				Status:         model.OrganizationMemberStatusDisabled,
				Reason:         tc.reason,
				IdempotencyKey: idempotencyKey,
			})
			require.ErrorContains(t, err, "organization idempotency conflict")
		})
	}
}

func TestOrganizationMemberDisableDeniedRequestRollsBackIdempotency(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	admin := createServiceTestUser(t, "member-disable-rollback-admin", common.RoleCommonUser)
	target := createServiceTestUser(t, "member-disable-rollback-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	idempotencyKey := testOrganizationIdempotencyKey(t, "disable-rollback")
	req := UpdateMemberRequest{
		Status:         model.OrganizationMemberStatusDisabled,
		Reason:         "disable after demotion",
		IdempotencyKey: idempotencyKey,
	}

	err := UpdateOrganizationMember(admin.Id, organization.Id, OrganizationAccessModeManagement, target.Id, req)
	require.ErrorContains(t, err, "organization member operation forbidden")
	var idempotencyCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationIdempotencyRecord{}).Where("idempotency_key = ?", idempotencyKey).Count(&idempotencyCount).Error)
	require.Zero(t, idempotencyCount)

	require.NoError(t, UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id, UpdateMemberRequest{Role: model.OrganizationRoleMember}))
	require.NoError(t, UpdateOrganizationMember(admin.Id, organization.Id, OrganizationAccessModeManagement, target.Id, req))
}

func TestOrganizationAdminCannotManageExistingAdmin(t *testing.T) {
	_, organization := createOrganizationMemberTestOrg(t)
	admin := createServiceTestUser(t, "peer-admin-operator", common.RoleCommonUser)
	peer := createServiceTestUser(t, "peer-admin-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	peerMember := model.OrganizationMember{OrganizationId: organization.Id, UserId: peer.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}
	require.NoError(t, model.DB.Create(&peerMember).Error)

	testCases := []struct {
		name string
		run  func() error
	}{
		{name: "demote", run: func() error {
			return UpdateOrganizationMember(admin.Id, organization.Id, OrganizationAccessModeManagement, peer.Id, UpdateMemberRequest{Role: model.OrganizationRoleMember})
		}},
		{name: "disable", run: func() error {
			return UpdateOrganizationMember(admin.Id, organization.Id, OrganizationAccessModeManagement, peer.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusDisabled, IdempotencyKey: testOrganizationIdempotencyKey(t, "disable-peer-admin")})
		}},
		{name: "remove", run: func() error {
			return RemoveOrganizationMember(admin.Id, organization.Id, OrganizationAccessModeManagement, peer.Id, RemoveMemberRequest{Reason: "peer management forbidden", IdempotencyKey: testOrganizationIdempotencyKey(t, "remove-peer-admin")})
		}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorContains(t, tc.run(), "organization member operation forbidden")
		})
	}

	var stored model.OrganizationMember
	require.NoError(t, model.DB.First(&stored, peerMember.Id).Error)
	require.Equal(t, model.OrganizationRoleAdmin, stored.Role)
	require.Equal(t, model.OrganizationMemberStatusActive, stored.Status)
}

func TestOrganizationOwnerCanPromoteAndDemoteMember(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	target := createServiceTestUser(t, "owner-role-target", common.RoleCommonUser)
	targetMember := model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}
	require.NoError(t, model.DB.Create(&targetMember).Error)

	require.NoError(t, UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id, UpdateMemberRequest{Role: model.OrganizationRoleAdmin}))
	require.NoError(t, UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id, UpdateMemberRequest{Role: model.OrganizationRoleMember}))

	var stored model.OrganizationMember
	require.NoError(t, model.DB.First(&stored, targetMember.Id).Error)
	require.Equal(t, model.OrganizationRoleMember, stored.Role)
}

func TestOrganizationOwnerDemotionTransfersPublicKeysToOwnerAndPreservesPrivateKeys(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	admin := createServiceTestUser(t, "owner-demotion-key-admin", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: organization.Id,
		UserId:         admin.Id,
		Role:           model.OrganizationRoleAdmin,
		Status:         model.OrganizationMemberStatusActive,
	}).Error)
	publicToken := model.Token{
		UserId:            admin.Id,
		Key:               common.GetUUID(),
		Status:            common.TokenStatusEnabled,
		ScopeType:         model.TokenScopeOrganization,
		ScopeId:           organization.Id,
		OrganizationId:    organization.Id,
		Visibility:        model.TokenVisibilityPublic,
		ResponsibleUserId: admin.Id,
	}
	privateToken := publicToken
	privateToken.Id = 0
	privateToken.Key = common.GetUUID()
	privateToken.Visibility = model.TokenVisibilityPrivate
	require.NoError(t, model.DB.Create(&publicToken).Error)
	require.NoError(t, model.DB.Create(&privateToken).Error)

	err := UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, admin.Id, UpdateMemberRequest{
		Role:   model.OrganizationRoleMember,
		Reason: "least privilege",
	})

	require.NoError(t, err)
	var storedPublic model.Token
	require.NoError(t, model.DB.First(&storedPublic, publicToken.Id).Error)
	require.Equal(t, owner.Id, storedPublic.UserId)
	require.Equal(t, owner.Id, storedPublic.ResponsibleUserId)
	require.Equal(t, "least privilege", storedPublic.TransferReason)
	var storedPrivate model.Token
	require.NoError(t, model.DB.First(&storedPrivate, privateToken.Id).Error)
	require.Equal(t, admin.Id, storedPrivate.UserId)
	require.Equal(t, admin.Id, storedPrivate.ResponsibleUserId)
}

func TestOrganizationOwnerDemotionTransfersPublicKeysToSelectedAdmin(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	source := createServiceTestUser(t, "owner-demotion-explicit-source", common.RoleCommonUser)
	target := createServiceTestUser(t, "owner-demotion-explicit-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&[]model.OrganizationMember{
		{OrganizationId: organization.Id, UserId: source.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive},
		{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive},
	}).Error)
	token := model.Token{UserId: source.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: source.Id}
	require.NoError(t, model.DB.Create(&token).Error)

	err := UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, source.Id, UpdateMemberRequest{
		Role:             model.OrganizationRoleMember,
		TransferToUserId: target.Id,
		Reason:           "handoff",
	})

	require.NoError(t, err)
	require.NoError(t, model.DB.First(&token, token.Id).Error)
	require.Equal(t, target.Id, token.UserId)
	require.Equal(t, target.Id, token.ResponsibleUserId)
	require.Equal(t, "handoff", token.TransferReason)
}

func TestOrganizationOwnerDemotionRejectsInvalidPublicKeyTransferTarget(t *testing.T) {
	testCases := []struct {
		name         string
		createTarget func(t *testing.T, organization *model.Organization, source model.User) int
	}{
		{
			name: "member",
			createTarget: func(t *testing.T, organization *model.Organization, _ model.User) int {
				target := createServiceTestUser(t, "demotion-target-member", common.RoleCommonUser)
				require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
				return target.Id
			},
		},
		{
			name: "disabled admin",
			createTarget: func(t *testing.T, organization *model.Organization, _ model.User) int {
				target := createServiceTestUser(t, "demotion-target-disabled-admin", common.RoleCommonUser)
				require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusDisabled}).Error)
				return target.Id
			},
		},
		{
			name: "platform-disabled admin",
			createTarget: func(t *testing.T, organization *model.Organization, _ model.User) int {
				target := createServiceTestUser(t, "demotion-target-platform-disabled", common.RoleCommonUser)
				require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", target.Id).Update("status", common.UserStatusDisabled).Error)
				require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
				return target.Id
			},
		},
		{
			name: "foreign organization admin",
			createTarget: func(t *testing.T, _ *model.Organization, _ model.User) int {
				target := createServiceTestUser(t, "demotion-target-foreign-admin", common.RoleCommonUser)
				foreignOwner := createServiceTestUser(t, "demotion-target-foreign-owner", common.RoleCommonUser)
				foreignOrganization, err := CreateOrganization(foreignOwner.Id, CreateOrganizationRequest{Name: "Foreign Demotion Org"})
				require.NoError(t, err)
				require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: foreignOrganization.Id, UserId: target.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
				return target.Id
			},
		},
		{
			name: "source admin",
			createTarget: func(_ *testing.T, _ *model.Organization, source model.User) int {
				return source.Id
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			owner, organization := createOrganizationMemberTestOrg(t)
			source := createServiceTestUser(t, "demotion-invalid-source", common.RoleCommonUser)
			require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: source.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
			token := model.Token{UserId: source.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: source.Id}
			require.NoError(t, model.DB.Create(&token).Error)
			targetUserId := testCase.createTarget(t, organization, source)

			err := UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, source.Id, UpdateMemberRequest{Role: model.OrganizationRoleMember, TransferToUserId: targetUserId})

			var blockedErr *OrganizationOperationBlockedError
			require.ErrorAs(t, err, &blockedErr)
			var member model.OrganizationMember
			require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, source.Id).First(&member).Error)
			require.Equal(t, model.OrganizationRoleAdmin, member.Role)
			require.NoError(t, model.DB.First(&token, token.Id).Error)
			require.Equal(t, source.Id, token.UserId)
			require.Equal(t, source.Id, token.ResponsibleUserId)
		})
	}
}

func TestOrganizationOwnerDemotionTransferBlockedWritesAudit(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	source := createServiceTestUser(t, "demotion-blocked-audit-source", common.RoleCommonUser)
	target := createServiceTestUser(t, "demotion-blocked-audit-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&[]model.OrganizationMember{
		{OrganizationId: organization.Id, UserId: source.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive},
		{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive},
	}).Error)
	token := model.Token{UserId: source.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: source.Id}
	require.NoError(t, model.DB.Create(&token).Error)

	err := UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, source.Id, UpdateMemberRequest{Role: model.OrganizationRoleMember, TransferToUserId: target.Id, Reason: "blocked demotion"})

	var blockedErr *OrganizationOperationBlockedError
	require.ErrorAs(t, err, &blockedErr)
	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionMemberKeyTransferBlocked).First(&audit).Error)
	require.Equal(t, model.OrganizationRoleOwner, audit.OperatorRole)
	require.Contains(t, audit.Reason, "active transfer target required")
	require.Contains(t, audit.AfterData, "blocked demotion")
}

func TestOrganizationOwnerDemotionWithoutPublicKeysDoesNotValidateTransferTarget(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	admin := createServiceTestUser(t, "owner-demotion-private-only", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	token := model.Token{UserId: admin.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPrivate, ResponsibleUserId: admin.Id}
	require.NoError(t, model.DB.Create(&token).Error)

	err := UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, admin.Id, UpdateMemberRequest{Role: model.OrganizationRoleMember, TransferToUserId: 999999})

	require.NoError(t, err)
	var member model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, admin.Id).First(&member).Error)
	require.Equal(t, model.OrganizationRoleMember, member.Role)
	require.NoError(t, model.DB.First(&token, token.Id).Error)
	require.Equal(t, admin.Id, token.UserId)
	require.Equal(t, admin.Id, token.ResponsibleUserId)
}

func TestPlatformOrganizationAdminDemotionTransfersPublicKeysToOwner(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	platformAdmin := createServiceTestUser(t, "platform-demotion-operator", common.RoleAdminUser)
	admin := createServiceTestUser(t, "platform-demotion-source", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	token := model.Token{UserId: admin.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: admin.Id}
	require.NoError(t, model.DB.Create(&token).Error)

	err := UpdateOrganizationMember(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, admin.Id, UpdateMemberRequest{Role: model.OrganizationRoleMember})

	require.NoError(t, err)
	require.NoError(t, model.DB.First(&token, token.Id).Error)
	require.Equal(t, owner.Id, token.UserId)
	require.Equal(t, owner.Id, token.ResponsibleUserId)
}

func TestOrganizationOwnerDemotionClearsStaleResponsibilityBlockersOnly(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	admin := createServiceTestUser(t, "demotion-stale-blocker-admin", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", admin.Id).Update("status", common.UserStatusDisabled).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusDisabled}).Error)
	now := common.GetTimestamp()
	token := model.Token{
		UserId:               admin.Id,
		Key:                  common.GetUUID(),
		Status:               common.TokenStatusDisabled,
		DisabledBySystems:    true,
		SystemDisabledReason: organizationTokenBlockerReasonUserPlatformDisabled,
		SystemDisabledRefId:  admin.Id,
		SystemDisabledAt:     now,
		PreviousStatus:       common.TokenStatusEnabled,
		ScopeType:            model.TokenScopeOrganization,
		ScopeId:              organization.Id,
		OrganizationId:       organization.Id,
		Visibility:           model.TokenVisibilityPublic,
		ResponsibleUserId:    admin.Id,
	}
	require.NoError(t, model.DB.Create(&token).Error)
	blockers := []model.OrganizationTokenSystemBlocker{
		{TokenId: token.Id, OrganizationId: organization.Id, Reason: organizationTokenBlockerReasonMemberDisabled, RefType: organizationTokenBlockerRefTypeMember, RefId: admin.Id, Status: model.OrganizationTokenBlockerStatusActive, PreviousStatus: common.TokenStatusEnabled, DisabledAt: now, CreatedAt: now, UpdatedAt: now},
		{TokenId: token.Id, OrganizationId: organization.Id, Reason: organizationTokenBlockerReasonUserPlatformDisabled, RefType: organizationTokenBlockerRefTypeUser, RefId: admin.Id, Status: model.OrganizationTokenBlockerStatusActive, PreviousStatus: common.TokenStatusEnabled, DisabledAt: now, CreatedAt: now, UpdatedAt: now},
		{TokenId: token.Id, OrganizationId: organization.Id, Reason: organizationTokenBlockerReasonManualDisabled, RefType: organizationTokenBlockerRefTypeOperator, RefId: owner.Id, Status: model.OrganizationTokenBlockerStatusActive, PreviousStatus: common.TokenStatusEnabled, DisabledAt: now, CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, model.DB.Create(&blockers).Error)

	err := UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, admin.Id, UpdateMemberRequest{Role: model.OrganizationRoleMember})

	require.NoError(t, err)
	var activeResponsibilityBlockers int64
	require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).
		Where("token_id = ? AND reason IN ? AND status = ?", token.Id, []string{organizationTokenBlockerReasonMemberDisabled, organizationTokenBlockerReasonUserPlatformDisabled}, model.OrganizationTokenBlockerStatusActive).
		Count(&activeResponsibilityBlockers).Error)
	require.Zero(t, activeResponsibilityBlockers)
	var activeManualBlockers int64
	require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).
		Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonManualDisabled, model.OrganizationTokenBlockerStatusActive).
		Count(&activeManualBlockers).Error)
	require.EqualValues(t, 1, activeManualBlockers)
	require.NoError(t, model.DB.First(&token, token.Id).Error)
	require.Equal(t, common.TokenStatusDisabled, token.Status)
	require.False(t, token.DisabledBySystems)
	require.Empty(t, token.SystemDisabledReason)
	var transferAudit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionMemberKeyTransfer).First(&transferAudit).Error)
	require.Contains(t, transferAudit.AfterData, organizationTokenBlockerReasonMemberDisabled)
	require.Contains(t, transferAudit.AfterData, organizationTokenBlockerReasonUserPlatformDisabled)
}

func TestOrganizationOwnerDemotionRestoresPublicKeyWhenOnlyOldResponsibilityBlockersRemain(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	admin := createServiceTestUser(t, "demotion-restore-blocker-admin", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", admin.Id).Update("status", common.UserStatusDisabled).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusDisabled}).Error)
	now := common.GetTimestamp()
	token := model.Token{UserId: admin.Id, Key: common.GetUUID(), Status: common.TokenStatusDisabled, DisabledBySystems: true, SystemDisabledReason: organizationTokenBlockerReasonMemberDisabled, SystemDisabledRefId: admin.Id, SystemDisabledAt: now, PreviousStatus: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: admin.Id}
	require.NoError(t, model.DB.Create(&token).Error)
	require.NoError(t, model.DB.Create(&[]model.OrganizationTokenSystemBlocker{
		{TokenId: token.Id, OrganizationId: organization.Id, Reason: organizationTokenBlockerReasonMemberDisabled, RefType: organizationTokenBlockerRefTypeMember, RefId: admin.Id, Status: model.OrganizationTokenBlockerStatusActive, PreviousStatus: common.TokenStatusEnabled, DisabledAt: now, CreatedAt: now, UpdatedAt: now},
		{TokenId: token.Id, OrganizationId: organization.Id, Reason: organizationTokenBlockerReasonUserPlatformDisabled, RefType: organizationTokenBlockerRefTypeUser, RefId: admin.Id, Status: model.OrganizationTokenBlockerStatusActive, PreviousStatus: common.TokenStatusEnabled, DisabledAt: now, CreatedAt: now, UpdatedAt: now},
	}).Error)

	require.NoError(t, UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, admin.Id, UpdateMemberRequest{Role: model.OrganizationRoleMember}))

	require.NoError(t, model.DB.First(&token, token.Id).Error)
	require.Equal(t, common.TokenStatusEnabled, token.Status)
	require.False(t, token.DisabledBySystems)
	require.Zero(t, token.PreviousStatus)
}

func TestOrganizationOwnerDemotionAndDisableBlocksOnlyRemainingPrivateKeys(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	admin := createServiceTestUser(t, "demotion-disable-admin", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	publicToken := model.Token{UserId: admin.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: admin.Id}
	privateToken := publicToken
	privateToken.Id = 0
	privateToken.Key = common.GetUUID()
	privateToken.Visibility = model.TokenVisibilityPrivate
	require.NoError(t, model.DB.Create(&publicToken).Error)
	require.NoError(t, model.DB.Create(&privateToken).Error)

	require.NoError(t, UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, admin.Id, UpdateMemberRequest{
		Role:           model.OrganizationRoleMember,
		Status:         model.OrganizationMemberStatusDisabled,
		Reason:         "demote and disable",
		IdempotencyKey: testOrganizationIdempotencyKey(t, "demote-disable"),
	}))

	require.NoError(t, model.DB.First(&publicToken, publicToken.Id).Error)
	require.Equal(t, owner.Id, publicToken.ResponsibleUserId)
	require.Equal(t, common.TokenStatusEnabled, publicToken.Status)
	var publicBlockers int64
	require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).Where("token_id = ? AND reason = ? AND status = ?", publicToken.Id, organizationTokenBlockerReasonMemberDisabled, model.OrganizationTokenBlockerStatusActive).Count(&publicBlockers).Error)
	require.Zero(t, publicBlockers)
	require.NoError(t, model.DB.First(&privateToken, privateToken.Id).Error)
	require.Equal(t, admin.Id, privateToken.ResponsibleUserId)
	require.Equal(t, common.TokenStatusDisabled, privateToken.Status)
	var privateBlockers int64
	require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).Where("token_id = ? AND reason = ? AND status = ?", privateToken.Id, organizationTokenBlockerReasonMemberDisabled, model.OrganizationTokenBlockerStatusActive).Count(&privateBlockers).Error)
	require.EqualValues(t, 1, privateBlockers)
}

func TestOrganizationOwnerDemotionDisableIdempotencyIncludesTransferTarget(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	source := createServiceTestUser(t, "demotion-idempotency-source", common.RoleCommonUser)
	firstTarget := createServiceTestUser(t, "demotion-idempotency-target-a", common.RoleCommonUser)
	secondTarget := createServiceTestUser(t, "demotion-idempotency-target-b", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&[]model.OrganizationMember{
		{OrganizationId: organization.Id, UserId: source.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive},
		{OrganizationId: organization.Id, UserId: firstTarget.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive},
		{OrganizationId: organization.Id, UserId: secondTarget.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive},
	}).Error)
	token := model.Token{UserId: source.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: source.Id}
	require.NoError(t, model.DB.Create(&token).Error)
	idempotencyKey := testOrganizationIdempotencyKey(t, "demote-disable-transfer-target")

	require.NoError(t, UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, source.Id, UpdateMemberRequest{Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusDisabled, TransferToUserId: firstTarget.Id, IdempotencyKey: idempotencyKey}))
	err := UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, source.Id, UpdateMemberRequest{Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusDisabled, TransferToUserId: secondTarget.Id, IdempotencyKey: idempotencyKey})

	require.ErrorContains(t, err, "organization idempotency conflict")
	require.NoError(t, model.DB.First(&token, token.Id).Error)
	require.Equal(t, firstTarget.Id, token.ResponsibleUserId)
}

func TestOrganizationOwnerDemotionThenPlatformDisableDoesNotBlockTransferredPublicKey(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	admin := createServiceTestUser(t, "demotion-platform-disable-admin", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	publicToken := model.Token{UserId: admin.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: admin.Id}
	privateToken := publicToken
	privateToken.Id = 0
	privateToken.Key = common.GetUUID()
	privateToken.Visibility = model.TokenVisibilityPrivate
	require.NoError(t, model.DB.Create(&publicToken).Error)
	require.NoError(t, model.DB.Create(&privateToken).Error)
	require.NoError(t, UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, admin.Id, UpdateMemberRequest{Role: model.OrganizationRoleMember}))

	require.NoError(t, UpdateUserStatusAndOrganizationTokenBlockers(admin.Id, common.UserStatusDisabled, owner.Id))

	require.NoError(t, model.DB.First(&publicToken, publicToken.Id).Error)
	require.Equal(t, owner.Id, publicToken.ResponsibleUserId)
	require.Equal(t, common.TokenStatusEnabled, publicToken.Status)
	require.NoError(t, model.DB.First(&privateToken, privateToken.Id).Error)
	require.Equal(t, admin.Id, privateToken.ResponsibleUserId)
	require.Equal(t, common.TokenStatusDisabled, privateToken.Status)
}

func TestOrganizationOwnerDemotionPreservesOrganizationBlocker(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	admin := createServiceTestUser(t, "demotion-organization-blocker-admin", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	now := common.GetTimestamp()
	token := model.Token{UserId: admin.Id, Key: common.GetUUID(), Status: common.TokenStatusDisabled, DisabledBySystems: true, SystemDisabledReason: organizationTokenBlockerReasonOrganizationDissolved, SystemDisabledRefId: organization.Id, SystemDisabledAt: now, PreviousStatus: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: admin.Id}
	require.NoError(t, model.DB.Create(&token).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationTokenSystemBlocker{TokenId: token.Id, OrganizationId: organization.Id, Reason: organizationTokenBlockerReasonOrganizationDissolved, RefType: organizationTokenBlockerRefTypeOrganization, RefId: organization.Id, Status: model.OrganizationTokenBlockerStatusActive, PreviousStatus: common.TokenStatusEnabled, DisabledAt: now, CreatedAt: now, UpdatedAt: now}).Error)

	require.NoError(t, UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, admin.Id, UpdateMemberRequest{Role: model.OrganizationRoleMember}))

	var activeOrganizationBlockers int64
	require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonOrganizationDissolved, model.OrganizationTokenBlockerStatusActive).Count(&activeOrganizationBlockers).Error)
	require.EqualValues(t, 1, activeOrganizationBlockers)
	require.NoError(t, model.DB.First(&token, token.Id).Error)
	require.Equal(t, common.TokenStatusDisabled, token.Status)
	require.True(t, token.DisabledBySystems)
	require.Equal(t, organizationTokenBlockerReasonOrganizationDissolved, token.SystemDisabledReason)
}

func TestOrganizationMemberCanExit(t *testing.T) {
	_, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "member-can-exit", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	err := ExitOrganization(member.Id, organization.Id, OrganizationAccessModeWorkspace, ExitMemberRequest{Reason: "leave", IdempotencyKey: testOrganizationIdempotencyKey(t, "exit-member")})

	require.NoError(t, err)
	var stored model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).First(&stored).Error)
	require.Equal(t, model.OrganizationMemberStatusExited, stored.Status)
}

func TestOrganizationMemberActiveMemberCanListMembers(t *testing.T) {
	admin, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "member-list-active-reader", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	members, err := ListOrganizationMembers(member.Id, organization.Id, OrganizationAccessModeWorkspace)
	require.NoError(t, err)
	visibleByUserId := map[int]model.OrganizationMember{}
	for _, member := range members {
		visibleByUserId[member.UserId] = member.OrganizationMember
	}
	require.Equal(t, model.OrganizationMemberStatusActive, visibleByUserId[admin.Id].Status)
	require.Equal(t, model.OrganizationMemberStatusActive, visibleByUserId[member.Id].Status)
}

func TestOrganizationAdminListsCanonicalOwnerRole(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	admin := createServiceTestUser(t, "member-list-canonical-owner-admin", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND user_id = ?", organization.Id, owner.Id).Update("role", model.OrganizationRoleMember).Error)

	members, err := ListOrganizationMembers(admin.Id, organization.Id, OrganizationAccessModeWorkspace)
	require.NoError(t, err)
	var ownerView *MemberView
	for index := range members {
		if members[index].UserId == owner.Id {
			ownerView = &members[index]
			break
		}
	}

	require.NotNil(t, ownerView)
	require.Equal(t, model.OrganizationRoleOwner, ownerView.Role)
}

func TestOrganizationMemberListIncludesUserPlatformStatus(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "member-list-platform-disabled", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", member.Id).Update("status", common.UserStatusDisabled).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)

	members, err := ListOrganizationMembers(owner.Id, organization.Id, OrganizationAccessModeWorkspace)

	require.NoError(t, err)
	payload, err := common.Marshal(members)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"user_status":2`)
}

func TestOrganizationMemberListHidesRelationshipEndedMembersOnly(t *testing.T) {
	admin, organization := createOrganizationMemberTestOrg(t)
	disabled := createServiceTestUser(t, "member-list-disabled", common.RoleCommonUser)
	exited := createServiceTestUser(t, "member-list-exited", common.RoleCommonUser)
	removed := createServiceTestUser(t, "member-list-removed", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: disabled.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusDisabled}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: exited.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusExited}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: removed.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusRemoved}).Error)

	members, err := ListOrganizationMembers(admin.Id, organization.Id, OrganizationAccessModeWorkspace)
	require.NoError(t, err)
	visibleByUserId := map[int]model.OrganizationMember{}
	for _, member := range members {
		visibleByUserId[member.UserId] = member.OrganizationMember
	}
	require.Equal(t, model.OrganizationMemberStatusActive, visibleByUserId[admin.Id].Status)
	require.Equal(t, model.OrganizationMemberStatusDisabled, visibleByUserId[disabled.Id].Status)
	require.NotContains(t, visibleByUserId, exited.Id)
	require.NotContains(t, visibleByUserId, removed.Id)
}

func TestOrganizationOwnerCannotBeDemotedByAdmin(t *testing.T) {
	admin, organization := createOrganizationMemberTestOrg(t)
	otherAdmin := createServiceTestUser(t, "owner-demote-admin", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: otherAdmin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)

	err := UpdateOrganizationMember(otherAdmin.Id, organization.Id, OrganizationAccessModeManagement, admin.Id, UpdateMemberRequest{Role: model.OrganizationRoleMember})
	require.Error(t, err)
	require.Contains(t, err.Error(), "organization member operation forbidden")
}

func TestOrganizationOwnerCannotBeDisabledByAdmin(t *testing.T) {
	admin, organization := createOrganizationMemberTestOrg(t)
	otherAdmin := createServiceTestUser(t, "owner-disable-admin", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: otherAdmin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)

	err := UpdateOrganizationMember(otherAdmin.Id, organization.Id, OrganizationAccessModeManagement, admin.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusDisabled, IdempotencyKey: testOrganizationIdempotencyKey(t, "disable-member")})
	require.Error(t, err)
	require.Contains(t, err.Error(), "organization member operation forbidden")
}

func TestOrganizationOwnerCannotBeRemovedByAdmin(t *testing.T) {
	admin, organization := createOrganizationMemberTestOrg(t)
	otherAdmin := createServiceTestUser(t, "owner-remove-admin", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: otherAdmin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)

	err := RemoveOrganizationMember(otherAdmin.Id, organization.Id, OrganizationAccessModeManagement, admin.Id, RemoveMemberRequest{Reason: "cleanup", IdempotencyKey: testOrganizationIdempotencyKey(t, "remove-member")})
	require.Error(t, err)
	require.Contains(t, err.Error(), "organization member operation forbidden")
}

func TestOrganizationOwnerCannotBeDisabledByPlatformRootThroughMemberUpdate(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	root := createServiceTestUser(t, "owner-disable-root", common.RoleRootUser)

	err := UpdateOrganizationMember(root.Id, organization.Id, OrganizationAccessModeAdmin, owner.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusDisabled, IdempotencyKey: testOrganizationIdempotencyKey(t, "disable-member")})

	require.ErrorContains(t, err, "organization member operation forbidden")
	var stored model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, owner.Id).First(&stored).Error)
	require.Equal(t, model.OrganizationRoleOwner, stored.Role)
	require.Equal(t, model.OrganizationMemberStatusActive, stored.Status)
}

func TestOrganizationOwnerCannotBeRemovedByPlatformRootThroughMemberDelete(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	root := createServiceTestUser(t, "owner-remove-root", common.RoleRootUser)

	err := RemoveOrganizationMember(root.Id, organization.Id, OrganizationAccessModeAdmin, owner.Id, RemoveMemberRequest{Reason: "root should transfer owner", IdempotencyKey: testOrganizationIdempotencyKey(t, "remove-member")})

	require.ErrorContains(t, err, "organization member operation forbidden")
	var stored model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, owner.Id).First(&stored).Error)
	require.Equal(t, model.OrganizationRoleOwner, stored.Role)
	require.Equal(t, model.OrganizationMemberStatusActive, stored.Status)
}

func TestDisabledOrganizationAllowsPlatformMemberUpdateAndRemove(t *testing.T) {
	testCases := []struct {
		name string
		role int
	}{
		{"platform admin", common.RoleAdminUser},
		{"platform root", common.RoleRootUser},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, organization := createOrganizationMemberTestOrg(t)
			actor := createServiceTestUser(t, "disabled-member-mutation-actor", tc.role)
			member := createServiceTestUser(t, "disabled-member-mutation-target", common.RoleCommonUser)
			require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
			require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("status", model.OrganizationStatusDisabled).Error)

			err := UpdateOrganizationMember(actor.Id, organization.Id, OrganizationAccessModeAdmin, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusDisabled, IdempotencyKey: testOrganizationIdempotencyKey(t, "disable-member")})
			require.NoError(t, err)
			err = RemoveOrganizationMember(actor.Id, organization.Id, OrganizationAccessModeAdmin, member.Id, RemoveMemberRequest{Reason: "disabled org", IdempotencyKey: testOrganizationIdempotencyKey(t, "remove-member")})
			require.NoError(t, err)

			var stored model.OrganizationMember
			require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).First(&stored).Error)
			require.Equal(t, model.OrganizationMemberStatusRemoved, stored.Status)
		})
	}
}

func TestOrganizationOwnerCannotExitThroughSelfExit(t *testing.T) {
	admin, organization := createOrganizationMemberTestOrg(t)

	err := ExitOrganization(admin.Id, organization.Id, OrganizationAccessModeWorkspace, ExitMemberRequest{Reason: "leave", IdempotencyKey: testOrganizationIdempotencyKey(t, "exit-member")})
	require.Error(t, err)
	require.Contains(t, err.Error(), "permission denied")
}

func TestTransferOrganizationOwnerUpdatesOwnerAndMemberRoles(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	target := createServiceTestUser(t, "owner-transfer-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	err := TransferOrganizationOwner(owner.Id, organization.Id, OrganizationAccessModeManagement, TransferOrganizationOwnerRequest{OwnerUserId: target.Id, Reason: "handoff", IdempotencyKey: testOrganizationIdempotencyKey(t, "transfer-owner")})
	require.NoError(t, err)

	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, target.Id, stored.OwnerUserId)
	var oldOwner model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, owner.Id).First(&oldOwner).Error)
	require.Equal(t, model.OrganizationRoleAdmin, oldOwner.Role)
	var newOwner model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, target.Id).First(&newOwner).Error)
	require.Equal(t, model.OrganizationRoleOwner, newOwner.Role)
	var activeOwnerCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND role = ? AND status = ?", organization.Id, model.OrganizationRoleOwner, model.OrganizationMemberStatusActive).Count(&activeOwnerCount).Error)
	require.EqualValues(t, 1, activeOwnerCount)
}

func TestTransferOrganizationOwnerUsesOwnerUserIdWhenCurrentOwnerRoleIsStale(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	target := createServiceTestUser(t, "owner-transfer-stale-role-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND user_id = ?", organization.Id, owner.Id).Update("role", model.OrganizationRoleAdmin).Error)

	err := TransferOrganizationOwner(owner.Id, organization.Id, OrganizationAccessModeManagement, TransferOrganizationOwnerRequest{OwnerUserId: target.Id, Reason: "repair stale owner role", IdempotencyKey: testOrganizationIdempotencyKey(t, "transfer-owner")})
	require.NoError(t, err)

	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, target.Id, stored.OwnerUserId)
	var newOwner model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, target.Id).First(&newOwner).Error)
	require.Equal(t, model.OrganizationRoleOwner, newOwner.Role)
}

func TestPlatformRootRepairsStaleOwnerRoleForSameOwnerUserId(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	root := createServiceTestUser(t, "root-repair-same-owner-role", common.RoleRootUser)
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND user_id = ?", organization.Id, owner.Id).Update("role", model.OrganizationRoleAdmin).Error)

	err := TransferOrganizationOwner(root.Id, organization.Id, OrganizationAccessModeAdmin, TransferOrganizationOwnerRequest{OwnerUserId: owner.Id, Reason: "repair same owner", IdempotencyKey: testOrganizationIdempotencyKey(t, "transfer-owner")})

	require.NoError(t, err)
	var storedOwner model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, owner.Id).First(&storedOwner).Error)
	require.Equal(t, model.OrganizationRoleOwner, storedOwner.Role)
	var activeOwnerCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND role = ? AND status = ?", organization.Id, model.OrganizationRoleOwner, model.OrganizationMemberStatusActive).Count(&activeOwnerCount).Error)
	require.EqualValues(t, 1, activeOwnerCount)
}

func TestPlatformRootRepairsOwnerWhenCurrentOwnerMemberIsInactive(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	root := createServiceTestUser(t, "root-repair-inactive-owner", common.RoleRootUser)
	target := createServiceTestUser(t, "root-repair-inactive-owner-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND user_id = ?", organization.Id, owner.Id).Update("status", model.OrganizationMemberStatusDisabled).Error)

	err := TransferOrganizationOwner(root.Id, organization.Id, OrganizationAccessModeAdmin, TransferOrganizationOwnerRequest{OwnerUserId: target.Id, Reason: "repair inactive owner", IdempotencyKey: testOrganizationIdempotencyKey(t, "transfer-owner")})

	require.NoError(t, err)
	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, target.Id, stored.OwnerUserId)
	var newOwner model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, target.Id).First(&newOwner).Error)
	require.Equal(t, model.OrganizationRoleOwner, newOwner.Role)
	var activeOwnerCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND role = ? AND status = ?", organization.Id, model.OrganizationRoleOwner, model.OrganizationMemberStatusActive).Count(&activeOwnerCount).Error)
	require.EqualValues(t, 1, activeOwnerCount)
}

func TestPlatformRootRepairsDuplicateActiveOwnerRoles(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	root := createServiceTestUser(t, "root-repair-duplicate-owner", common.RoleRootUser)
	target := createServiceTestUser(t, "root-repair-duplicate-owner-target", common.RoleCommonUser)
	extraOwner := createServiceTestUser(t, "root-repair-extra-owner", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: extraOwner.Id, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive}).Error)

	err := TransferOrganizationOwner(root.Id, organization.Id, OrganizationAccessModeAdmin, TransferOrganizationOwnerRequest{OwnerUserId: target.Id, Reason: "repair duplicate owner", IdempotencyKey: testOrganizationIdempotencyKey(t, "transfer-owner")})

	require.NoError(t, err)
	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, target.Id, stored.OwnerUserId)
	for _, userId := range []int{owner.Id, extraOwner.Id} {
		var member model.OrganizationMember
		require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, userId).First(&member).Error)
		require.Equal(t, model.OrganizationRoleAdmin, member.Role)
	}
	var activeOwnerCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND role = ? AND status = ?", organization.Id, model.OrganizationRoleOwner, model.OrganizationMemberStatusActive).Count(&activeOwnerCount).Error)
	require.EqualValues(t, 1, activeOwnerCount)
}

func TestStaleOwnerRoleCannotTransferOrganizationOwner(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	target := createServiceTestUser(t, "stale-owner-target", common.RoleCommonUser)
	staleOwner := createServiceTestUser(t, "stale-owner-role", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: staleOwner.Id, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive}).Error)

	err := TransferOrganizationOwner(staleOwner.Id, organization.Id, OrganizationAccessModeManagement, TransferOrganizationOwnerRequest{OwnerUserId: target.Id, Reason: "stale", IdempotencyKey: testOrganizationIdempotencyKey(t, "transfer-owner")})

	require.ErrorContains(t, err, "permission denied")
	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, owner.Id, stored.OwnerUserId)
}

func TestOrganizationMemberReactivationNormalizesStaleOwnerRole(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	staleOwner := createServiceTestUser(t, "reactivate-stale-owner-role", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: staleOwner.Id, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusDisabled}).Error)

	err := UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, staleOwner.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusActive})

	require.NoError(t, err)
	var stored model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, staleOwner.Id).First(&stored).Error)
	require.Equal(t, model.OrganizationRoleMember, stored.Role)
	require.Equal(t, model.OrganizationMemberStatusActive, stored.Status)
	var activeOwnerCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND role = ? AND status = ?", organization.Id, model.OrganizationRoleOwner, model.OrganizationMemberStatusActive).Count(&activeOwnerCount).Error)
	require.EqualValues(t, 1, activeOwnerCount)
}

func TestDisabledOrganizationAllowsPlatformRootOwnerTransfer(t *testing.T) {
	_, organization := createOrganizationMemberTestOrg(t)
	root := createServiceTestUser(t, "disabled-owner-transfer-root", common.RoleRootUser)
	target := createServiceTestUser(t, "disabled-owner-transfer-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("status", model.OrganizationStatusDisabled).Error)

	err := TransferOrganizationOwner(root.Id, organization.Id, OrganizationAccessModeAdmin, TransferOrganizationOwnerRequest{OwnerUserId: target.Id, Reason: "disabled org", IdempotencyKey: testOrganizationIdempotencyKey(t, "transfer-owner")})

	require.NoError(t, err)
	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, target.Id, stored.OwnerUserId)
}

func TestTransferOrganizationOwnerSuccessfulReplayPrecedesCurrentAuthorization(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	target := createServiceTestUser(t, "owner-transfer-replay-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	req := TransferOrganizationOwnerRequest{OwnerUserId: target.Id, Reason: "handoff", IdempotencyKey: "owner-transfer-success-replay"}

	require.NoError(t, TransferOrganizationOwner(owner.Id, organization.Id, OrganizationAccessModeManagement, req))
	require.NoError(t, TransferOrganizationOwner(owner.Id, organization.Id, OrganizationAccessModeManagement, req))

	conflict := req
	conflict.Reason = "different content"
	require.ErrorContains(t, TransferOrganizationOwner(owner.Id, organization.Id, OrganizationAccessModeManagement, conflict), "organization idempotency conflict")
}

func TestExitOrganizationSuccessfulReplayPrecedesCurrentAuthorization(t *testing.T) {
	_, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "member-exit-success-replay", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	req := ExitMemberRequest{Reason: "leave", IdempotencyKey: "member-exit-success-replay"}

	require.NoError(t, ExitOrganization(member.Id, organization.Id, OrganizationAccessModeWorkspace, req))
	require.NoError(t, ExitOrganization(member.Id, organization.Id, OrganizationAccessModeWorkspace, req))

	conflict := req
	conflict.Reason = "different content"
	require.ErrorContains(t, ExitOrganization(member.Id, organization.Id, OrganizationAccessModeWorkspace, conflict), "organization idempotency conflict")
}

func TestOrganizationOwnerRoleCannotExitWhenOwnerUserIdStillPointsToUser(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND user_id = ?", organization.Id, owner.Id).Update("role", model.OrganizationRoleMember).Error)

	err := ExitOrganization(owner.Id, organization.Id, OrganizationAccessModeWorkspace, ExitMemberRequest{Reason: "stale member role", IdempotencyKey: testOrganizationIdempotencyKey(t, "exit-member")})

	require.ErrorContains(t, err, "permission denied")
	var stored model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, owner.Id).First(&stored).Error)
	require.Equal(t, model.OrganizationMemberStatusActive, stored.Status)
}

func TestOrganizationMemberRemovalTransfersPublicKeysAndDeletesPrivateKeys(t *testing.T) {
	admin, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "key-owner", common.RoleCommonUser)
	transferTo := createServiceTestUser(t, "key-new-owner", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: transferTo.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	publicToken := model.Token{UserId: member.Id, Key: "member-owned-public-key-for-transfer-00000000", Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: member.Id}
	privateToken := model.Token{UserId: member.Id, Key: "member-owned-private-key-for-delete-00000000", Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPrivate, ResponsibleUserId: member.Id}
	require.NoError(t, model.DB.Create(&publicToken).Error)
	require.NoError(t, model.DB.Create(&privateToken).Error)

	err := RemoveOrganizationMember(admin.Id, organization.Id, OrganizationAccessModeManagement, member.Id, RemoveMemberRequest{TransferToUserId: transferTo.Id, Reason: "rotate owner", IdempotencyKey: testOrganizationIdempotencyKey(t, "remove-member")})
	require.NoError(t, err)

	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, publicToken.Id).Error)
	require.Equal(t, transferTo.Id, storedToken.UserId)
	require.Equal(t, transferTo.Id, storedToken.ResponsibleUserId)

	require.Error(t, model.DB.First(&model.Token{}, privateToken.Id).Error)
	var deletedToken model.Token
	require.NoError(t, model.DB.Unscoped().First(&deletedToken, privateToken.Id).Error)
	require.True(t, deletedToken.DeletedAt.Valid)

	var auditCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationAuditLog{}).Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionMemberKeyTransfer).Count(&auditCount).Error)
	require.Equal(t, int64(1), auditCount)
	require.NoError(t, model.DB.Model(&model.OrganizationAuditLog{}).Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionTokenDelete).Count(&auditCount).Error)
	require.Equal(t, int64(1), auditCount)

	var transferAudit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionMemberKeyTransfer).First(&transferAudit).Error)
	require.Equal(t, "rotate owner", transferAudit.Reason)
	var deleteAudit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionTokenDelete).First(&deleteAudit).Error)
	require.Equal(t, "rotate owner", deleteAudit.Reason)
}

func TestOrganizationMemberRemovalRequiresReason(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "key-empty-reason-member", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	err := RemoveOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, RemoveMemberRequest{Reason: " ", IdempotencyKey: testOrganizationIdempotencyKey(t, "remove-member")})

	require.ErrorContains(t, err, "organization member remove reason required")
	var storedMember model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).First(&storedMember).Error)
	require.Equal(t, model.OrganizationMemberStatusActive, storedMember.Status)
}

func TestOrganizationMemberRemovalCannotTransferPublicKeysToMember(t *testing.T) {
	admin, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "key-public-transfer-member", common.RoleCommonUser)
	transferTo := createServiceTestUser(t, "key-public-transfer-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: transferTo.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	token := model.Token{UserId: member.Id, Key: "member-public-key-transfer-target-0000000000", Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: member.Id}
	require.NoError(t, model.DB.Create(&token).Error)

	err := RemoveOrganizationMember(admin.Id, organization.Id, OrganizationAccessModeManagement, member.Id, RemoveMemberRequest{TransferToUserId: transferTo.Id, Reason: "handoff", IdempotencyKey: testOrganizationIdempotencyKey(t, "remove-member")})

	var blockedErr *OrganizationOperationBlockedError
	require.ErrorAs(t, err, &blockedErr)
	require.ErrorContains(t, err, "active transfer target required")
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.Equal(t, member.Id, storedToken.ResponsibleUserId)
	var storedMember model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).First(&storedMember).Error)
	require.Equal(t, model.OrganizationMemberStatusActive, storedMember.Status)
}

func TestOrganizationMemberTransferBlockedAudit(t *testing.T) {
	testCases := []struct {
		name   string
		invoke func(t *testing.T, owner model.User, organization *model.Organization, member model.User, transferTarget model.User) error
	}{
		{
			name: "remove",
			invoke: func(t *testing.T, owner model.User, organization *model.Organization, member model.User, transferTarget model.User) error {
				return RemoveOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, RemoveMemberRequest{TransferToUserId: transferTarget.Id, Reason: "blocked remove", IdempotencyKey: testOrganizationIdempotencyKey(t, "remove-member")})
			},
		},
		{
			name: "exit",
			invoke: func(t *testing.T, owner model.User, organization *model.Organization, member model.User, transferTarget model.User) error {
				return ExitOrganization(member.Id, organization.Id, OrganizationAccessModeWorkspace, ExitMemberRequest{TransferToUserId: transferTarget.Id, Reason: "blocked exit", IdempotencyKey: testOrganizationIdempotencyKey(t, "exit-member")})
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			owner, organization := createOrganizationMemberTestOrg(t)
			member := createServiceTestUser(t, "blocked-audit-member", common.RoleCommonUser)
			transferTarget := createServiceTestUser(t, "blocked-audit-target", common.RoleCommonUser)
			require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
			require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: transferTarget.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusDisabled}).Error)
			token := model.Token{UserId: member.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: member.Id}
			require.NoError(t, model.DB.Create(&token).Error)

			err := tc.invoke(t, owner, organization, member, transferTarget)

			var blockedErr *OrganizationOperationBlockedError
			require.ErrorAs(t, err, &blockedErr)
			var storedMember model.OrganizationMember
			require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).First(&storedMember).Error)
			require.Equal(t, model.OrganizationMemberStatusActive, storedMember.Status)
			var storedToken model.Token
			require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
			require.Equal(t, member.Id, storedToken.ResponsibleUserId)
			var audits []model.OrganizationAuditLog
			require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionMemberKeyTransferBlocked).Find(&audits).Error)
			require.Len(t, audits, 1)
			require.Equal(t, "member", audits[0].TargetType)
			require.Equal(t, storedMember.Id, audits[0].TargetId)
			require.Contains(t, audits[0].Reason, "active transfer target required")
		})
	}
}

func TestOrganizationMemberTransferBlockedAuditUsesAdminDecisionRole(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	platformAdmin := createServiceTestUser(t, "blocked-audit-platform-admin", common.RoleAdminUser)
	member := createServiceTestUser(t, "blocked-audit-admin-member", common.RoleCommonUser)
	transferTarget := createServiceTestUser(t, "blocked-audit-admin-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: transferTarget.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusDisabled}).Error)
	require.NoError(t, model.DB.Create(&model.Token{UserId: member.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: member.Id}).Error)

	err := RemoveOrganizationMember(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, member.Id, RemoveMemberRequest{TransferToUserId: transferTarget.Id, Reason: "blocked platform remove", IdempotencyKey: testOrganizationIdempotencyKey(t, "remove-member")})

	var blockedErr *OrganizationOperationBlockedError
	require.ErrorAs(t, err, &blockedErr)
	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionMemberKeyTransferBlocked).First(&audit).Error)
	require.Equal(t, OrganizationPolicyRolePlatformAdmin, audit.OperatorRole)
	require.Equal(t, owner.Id, organization.OwnerUserId)
}

func TestOrganizationMemberTransferBlockedAuditFailurePreservesOriginalError(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "blocked-audit-failure-member", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	token := model.Token{UserId: member.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: member.Id}
	require.NoError(t, model.DB.Create(&token).Error)
	auditAttempted := false
	auditErr := errors.New("forced blocked audit failure")
	callbackName := "test:organization-member-transfer-blocked-audit-failure"
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "organization_audit_logs" {
			auditAttempted = true
			tx.AddError(auditErr)
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Create().Remove(callbackName) })

	err := RemoveOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, RemoveMemberRequest{TransferToUserId: member.Id, Reason: "blocked audit failure", IdempotencyKey: testOrganizationIdempotencyKey(t, "remove-member")})

	var blockedErr *OrganizationOperationBlockedError
	require.ErrorAs(t, err, &blockedErr)
	require.NotErrorIs(t, err, auditErr)
	require.True(t, auditAttempted)
}

func TestOrganizationMemberRemovalTransfersOwnedKeysToOwnerBeforeAdmin(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	admin := createServiceTestUser(t, "key-fallback-admin", common.RoleCommonUser)
	member := createServiceTestUser(t, "key-fallback-member", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	token := model.Token{UserId: member.Id, Key: "member-owned-key-owner-fallback-000000000", Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: member.Id}
	require.NoError(t, model.DB.Create(&token).Error)

	err := RemoveOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, RemoveMemberRequest{Reason: "fallback", IdempotencyKey: testOrganizationIdempotencyKey(t, "remove-member")})
	require.NoError(t, err)

	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.Equal(t, owner.Id, storedToken.UserId)
	require.Equal(t, owner.Id, storedToken.ResponsibleUserId)
}

func TestOrganizationMemberRemovalWithoutActiveOwnerDoesNotFallBackToAdmin(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	root := createServiceTestUser(t, "remove-no-owner-root", common.RoleRootUser)
	admin := createServiceTestUser(t, "remove-no-owner-admin", common.RoleCommonUser)
	member := createServiceTestUser(t, "remove-no-owner-member", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND user_id = ?", organization.Id, owner.Id).Update("status", model.OrganizationMemberStatusDisabled).Error)
	token := model.Token{UserId: member.Id, Key: common.GetUUID(), Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: member.Id}
	require.NoError(t, model.DB.Create(&token).Error)

	err := RemoveOrganizationMember(root.Id, organization.Id, OrganizationAccessModeAdmin, member.Id, RemoveMemberRequest{Reason: "no active owner", IdempotencyKey: testOrganizationIdempotencyKey(t, "remove-member")})

	var blocked *OrganizationOperationBlockedError
	require.ErrorAs(t, err, &blocked)
	var storedMember model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).First(&storedMember).Error)
	require.Equal(t, model.OrganizationMemberStatusActive, storedMember.Status)
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.Equal(t, member.Id, storedToken.UserId)
	require.Equal(t, member.Id, storedToken.ResponsibleUserId)
}

func TestOrganizationMemberRemovalTransfersOwnedKeysToOwnerUserIdWhenOwnerRoleIsStale(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	admin := createServiceTestUser(t, "key-fallback-stale-admin", common.RoleCommonUser)
	member := createServiceTestUser(t, "key-fallback-stale-member", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND user_id = ?", organization.Id, owner.Id).Update("role", model.OrganizationRoleMember).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	token := model.Token{UserId: member.Id, Key: "member-owned-key-stale-owner-fallback", Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: member.Id}
	require.NoError(t, model.DB.Create(&token).Error)

	err := RemoveOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, RemoveMemberRequest{Reason: "fallback stale owner role", IdempotencyKey: testOrganizationIdempotencyKey(t, "remove-member")})
	require.NoError(t, err)

	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.Equal(t, owner.Id, storedToken.UserId)
	require.Equal(t, owner.Id, storedToken.ResponsibleUserId)
}

func TestOrganizationMemberRemovalTransfersAllNonDeletedOrganizationKeys(t *testing.T) {
	owner, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "key-all-status-member", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	statuses := []int{common.TokenStatusEnabled, common.TokenStatusDisabled, common.TokenStatusExpired, common.TokenStatusExhausted}
	tokenIds := make([]int, 0, len(statuses))
	for _, status := range statuses {
		token := model.Token{UserId: member.Id, Key: common.GetRandomString(48), Name: "all-status", Status: status, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, Visibility: model.TokenVisibilityPublic, ResponsibleUserId: member.Id}
		require.NoError(t, model.DB.Create(&token).Error)
		tokenIds = append(tokenIds, token.Id)
	}

	err := RemoveOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, RemoveMemberRequest{Reason: "all keys", IdempotencyKey: testOrganizationIdempotencyKey(t, "remove-member")})
	require.NoError(t, err)

	var tokens []model.Token
	require.NoError(t, model.DB.Where("id IN ?", tokenIds).Find(&tokens).Error)
	require.Len(t, tokens, len(statuses))
	for _, token := range tokens {
		require.Equal(t, owner.Id, token.UserId)
		require.Equal(t, owner.Id, token.ResponsibleUserId)
	}
}

func TestOrganizationMemberReactivateReusesSameRecord(t *testing.T) {
	admin, organization := createOrganizationMemberTestOrg(t)
	member := createServiceTestUser(t, "reactivate-member", common.RoleCommonUser)
	existing := model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusRemoved, RemovedAt: common.GetTimestamp()}
	require.NoError(t, model.DB.Create(&existing).Error)

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		return ReactivateOrganizationMember(tx, organization.Id, member.Id, model.OrganizationRoleAdmin, admin.Id)
	})
	require.NoError(t, err)

	var members []model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).Find(&members).Error)
	require.Len(t, members, 1)
	require.Equal(t, existing.Id, members[0].Id)
	require.Equal(t, model.OrganizationMemberStatusActive, members[0].Status)
	require.Equal(t, model.OrganizationRoleAdmin, members[0].Role)
	require.Equal(t, admin.Id, members[0].InvitedBy)
}
