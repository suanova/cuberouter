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
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestCreateOrganizationCreatesCreatorAsOnlyActiveOwner(t *testing.T) {
	setupServiceTestDB(t)
	user := createServiceTestUser(t, "org-admin", common.RoleCommonUser)

	organization, err := CreateOrganization(user.Id, CreateOrganizationRequest{Name: "Test Org", Description: "desc"})

	require.NoError(t, err)
	require.NotZero(t, organization.Id)
	var member model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, user.Id).First(&member).Error)
	require.Equal(t, user.Id, organization.OwnerUserId)
	require.Equal(t, model.OrganizationRoleOwner, member.Role)
	require.Equal(t, model.OrganizationMemberStatusActive, member.Status)
	var activeOwnerCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND role = ? AND status = ?", organization.Id, model.OrganizationRoleOwner, model.OrganizationMemberStatusActive).Count(&activeOwnerCount).Error)
	require.EqualValues(t, 1, activeOwnerCount)
}

func TestOrganizationWriteAuditStoresRequestMetadata(t *testing.T) {
	setupServiceTestDB(t)
	user := createServiceTestUser(t, "org-audit-metadata", common.RoleCommonUser)

	organization, err := CreateOrganization(user.Id, CreateOrganizationRequest{Name: "Audit Metadata Org"}, OrganizationAuditRequestMetadata{IP: "198.51.100.17", UserAgent: "searouter-audit-test"})
	require.NoError(t, err)

	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionCreate).First(&audit).Error)
	require.Equal(t, "198.51.100.17", audit.Ip)
	require.Equal(t, "searouter-audit-test", audit.UserAgent)
}

func TestCreateOrganizationUsesDefaultQuotaZero(t *testing.T) {
	setupServiceTestDB(t)
	user := createServiceTestUser(t, "quota-default-user", common.RoleCommonUser)

	organization, err := CreateOrganization(user.Id, CreateOrganizationRequest{Name: "Quota Default Org"})

	require.NoError(t, err)
	require.Equal(t, model.OrganizationDefaultQuota, organization.Quota)
	require.Equal(t, 0, organization.Quota)
	require.Equal(t, 0, organization.UsedQuota)
}

func TestCreateOrganizationRequestCannotSetQuotaOrUsedQuota(t *testing.T) {
	setupServiceTestDB(t)
	user := createServiceTestUser(t, "quota-forbidden-user", common.RoleCommonUser)

	organization, err := CreateOrganization(user.Id, CreateOrganizationRequest{Name: "Quota Forbidden Org"})

	require.NoError(t, err)
	require.Equal(t, model.OrganizationDefaultQuota, organization.Quota)
	require.Equal(t, 0, organization.Quota)
	require.Equal(t, 0, organization.UsedQuota)
}

func TestCreateOrganizationRejectsGlobalNormalizedNameConflict(t *testing.T) {
	setupServiceTestDB(t)
	first := createServiceTestUser(t, "name-owner-1", common.RoleCommonUser)
	second := createServiceTestUser(t, "name-owner-2", common.RoleCommonUser)
	organization, err := CreateOrganization(first.Id, CreateOrganizationRequest{Name: "Design Group"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).
		Where("id = ?", organization.Id).
		Update("status", model.OrganizationStatusDissolved).Error)

	_, err = CreateOrganization(second.Id, CreateOrganizationRequest{Name: "  design group  "})
	require.ErrorIs(t, err, ErrOrganizationNameConflict)
}

func TestConcurrentCreateOrganizationReturnsTypedNormalizedNameConflict(t *testing.T) {
	deadline := organizationNameConcurrencyTestDeadline(t)
	db, _ := setupOrganizationExternalConcurrencyDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.OrganizationAuditLog{},
	))

	suffix := strings.ReplaceAll(common.GetUUID(), "-", "")[:8]
	first := model.User{Username: "concurrent-name-owner-1-" + suffix, Password: "password", DisplayName: "concurrent-name-owner-1", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "cno1-" + suffix}
	second := model.User{Username: "concurrent-name-owner-2-" + suffix, Password: "password", DisplayName: "concurrent-name-owner-2", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "cno2-" + suffix}
	require.NoError(t, db.Create(&first).Error)
	require.NoError(t, db.Create(&second).Error)
	normalized := "concurrent design group " + suffix
	insertReady := make(chan struct{}, 2)
	releaseInserts := make(chan struct{})
	callbackName := "test:concurrent-organization-name-insert-barrier"
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != "organizations" {
			return
		}
		organization, ok := tx.Statement.Dest.(*model.Organization)
		if !ok || organization.NameNormalized != normalized {
			return
		}
		insertReady <- struct{}{}
		<-releaseInserts
	}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callbackName) })
	t.Cleanup(func() {
		var organizations []model.Organization
		require.NoError(t, db.Where("name_normalized = ?", normalized).Find(&organizations).Error)
		for _, organization := range organizations {
			require.NoError(t, db.Where("organization_id = ?", organization.Id).Delete(&model.OrganizationAuditLog{}).Error)
			require.NoError(t, db.Where("organization_id = ?", organization.Id).Delete(&model.OrganizationMember{}).Error)
			require.NoError(t, db.Delete(&model.Organization{}, organization.Id).Error)
		}
		require.NoError(t, db.Unscoped().Where("id IN ?", []int{first.Id, second.Id}).Delete(&model.User{}).Error)
	})

	start := make(chan struct{})
	results := make(chan error, 2)
	requests := []struct {
		userId int
		name   string
	}{
		{userId: first.Id, name: "Concurrent Design Group " + suffix},
		{userId: second.Id, name: "  concurrent design group " + suffix + "  "},
	}
	for _, request := range requests {
		request := request
		go func() {
			<-start
			_, err := CreateOrganization(request.userId, CreateOrganizationRequest{Name: request.name})
			results <- err
		}()
	}
	close(start)
	waitForOrganizationNameConcurrencyBarrier(t, insertReady, results, len(requests), releaseInserts, deadline)

	var successCount, conflictCount int
	for _, err := range waitForOrganizationNameConcurrencyResults(t, results, len(requests), deadline) {
		switch {
		case err == nil:
			successCount++
		case errors.Is(err, ErrOrganizationNameConflict):
			conflictCount++
		default:
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, successCount)
	require.Equal(t, 1, conflictCount)
}

func TestMapOrganizationNameWriteErrorReturnsTypedNameConflict(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "name-mapper-owner", common.RoleCommonUser)
	_, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Mapper Name Conflict"})
	require.NoError(t, err)
	target, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Mapper Update Target"})
	require.NoError(t, err)

	target.Name = "  mapper name conflict  "
	target.NameNormalized = "mapper name conflict"
	err = model.DB.Model(target).Select("name", "name_normalized").Updates(target).Error
	require.Error(t, err)
	require.True(t, model.IsDuplicateKeyError(model.DB, err))

	mapped := mapOrganizationNameWriteError(err)
	require.ErrorIs(t, mapped, ErrOrganizationNameConflict)
}

func TestMapOrganizationNameWriteErrorDoesNotMislabelSlugCollision(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "slug-collision-owner", common.RoleCommonUser)
	first, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Original Slug Name"})
	require.NoError(t, err)

	duplicateSlug := model.Organization{
		Name:      "Distinct Organization Name",
		Slug:      first.Slug,
		Status:    model.OrganizationStatusActive,
		CreatedBy: owner.Id,
	}
	err = model.DB.Create(&duplicateSlug).Error
	require.Error(t, err)
	require.True(t, model.IsDuplicateKeyError(model.DB, err))

	mapped := mapOrganizationNameWriteError(err)
	require.Error(t, mapped)
	require.NotErrorIs(t, mapped, ErrOrganizationNameConflict)
	require.Same(t, err, mapped)
}

func TestCreateOrganizationLimitCountsDisabledOrganizations(t *testing.T) {
	setupServiceTestDB(t)
	user := createServiceTestUser(t, "disabled-limit", common.RoleCommonUser)
	for i := 0; i < maxActiveOrganizationsPerUser; i++ {
		organization, err := CreateOrganization(user.Id, CreateOrganizationRequest{Name: fmt.Sprintf("Disabled Limit Org %d", i)})
		require.NoError(t, err)
		require.NoError(t, DisableOrganization(user.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "disabled but not dissolved"))
	}

	_, err := CreateOrganization(user.Id, CreateOrganizationRequest{Name: "Should Hit Disabled Limit"})

	require.ErrorContains(t, err, "organization limit exceeded")
}

func TestCreateOrganizationLimitUsesCreatedByNotMemberships(t *testing.T) {
	setupServiceTestDB(t)
	member := createServiceTestUser(t, "membership-limit-member", common.RoleCommonUser)
	for i := 0; i < maxActiveOrganizationsPerUser; i++ {
		creator := createServiceTestUser(t, fmt.Sprintf("membership-limit-creator-%d", i), common.RoleCommonUser)
		organization, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: fmt.Sprintf("Membership Limit Org %d", i)})
		require.NoError(t, err)
		require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	}

	organization, err := CreateOrganization(member.Id, CreateOrganizationRequest{Name: "Created By Count Org"})

	require.NoError(t, err)
	require.Equal(t, member.Id, organization.CreatedBy)
}

func TestGetOrganizationForMemberRejectsNonMember(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "detail-admin", common.RoleCommonUser)
	nonMember := createServiceTestUser(t, "detail-non-member", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Detail Org"})
	require.NoError(t, err)

	_, _, err = GetOrganizationForMember(nonMember.Id, organization.Id, true)

	require.ErrorContains(t, err, "permission denied")
}

func TestGetOrganizationForMemberAllowsActiveMember(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "active-admin", common.RoleCommonUser)
	member := createServiceTestUser(t, "active-member", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Active Member Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	readOrg, readMember, err := GetOrganizationForMember(member.Id, organization.Id, true)

	require.NoError(t, err)
	require.Equal(t, organization.Id, readOrg.Id)
	require.Equal(t, model.OrganizationRoleMember, readMember.Role)
}

func TestUpdateOrganizationAllowsActiveAdmin(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "update-admin", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Update Org"})
	require.NoError(t, err)

	updated, err := UpdateOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, UpdateOrganizationRequest{Name: "Updated Org", Description: "new"})

	require.NoError(t, err)
	require.Equal(t, "Updated Org", updated.Name)
	require.Equal(t, "new", updated.Description)
}

func TestUpdateOrganizationRejectsAnotherNormalizedNameButAllowsOwnName(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "name-update-owner", common.RoleCommonUser)
	first, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "First Org"})
	require.NoError(t, err)
	second, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Second Org"})
	require.NoError(t, err)

	_, err = UpdateOrganization(owner.Id, second.Id, OrganizationAccessModeManagement, UpdateOrganizationRequest{Name: " first org "})
	require.ErrorIs(t, err, ErrOrganizationNameConflict)

	updated, err := UpdateOrganization(owner.Id, first.Id, OrganizationAccessModeManagement, UpdateOrganizationRequest{Name: " FIRST ORG "})
	require.NoError(t, err)
	require.Equal(t, "FIRST ORG", updated.Name)
}

func TestOrganizationOwnerIdentityAllowsLifecycleWrites(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "owner-identity-lifecycle", common.RoleCommonUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Owner Identity Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND user_id = ?", organization.Id, owner.Id).Update("role", model.OrganizationRoleOwner).Error)

	updated, err := UpdateOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, UpdateOrganizationRequest{Name: "Owner Identity Updated"})
	require.NoError(t, err)
	require.Equal(t, "Owner Identity Updated", updated.Name)
	require.NoError(t, DisableOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "maintenance"))
	require.NoError(t, EnableOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "resume"))
	require.NoError(t, dissolveOrganizationForTest(t, owner.Id, organization.Id, "done"))
}

func TestOrganizationLifecycleWritesLockOrganizationRow(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "lifecycle-lock-owner", common.RoleCommonUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Lifecycle Lock Org"})
	require.NoError(t, err)

	lockedOrganizationChecks := 0
	callbackName := "test:organization-lifecycle-lock"
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

	_, err = UpdateOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, UpdateOrganizationRequest{Name: "Lifecycle Lock Updated"})
	require.NoError(t, err)
	require.NoError(t, DisableOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "maintenance"))
	require.NoError(t, EnableOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "resume"))
	require.NoError(t, dissolveOrganizationForTest(t, owner.Id, organization.Id, "done"))

	requireOrganizationRowLockCount(t, lockedOrganizationChecks, 4)
}

func TestUpdateOrganizationRejectsMember(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "reject-update-admin", common.RoleCommonUser)
	member := createServiceTestUser(t, "reject-update-member", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Reject Update Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	_, err = UpdateOrganization(member.Id, organization.Id, OrganizationAccessModeManagement, UpdateOrganizationRequest{Name: "Denied"})

	require.ErrorContains(t, err, "permission denied")
}

func TestDissolveOrganizationSetsDissolvedStatus(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "dissolve-status-admin", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Dissolve Status Org"})
	require.NoError(t, err)

	require.NoError(t, dissolveOrganizationForTest(t, admin.Id, organization.Id, "done"))

	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, model.OrganizationStatusDissolved, stored.Status)
	require.NotZero(t, stored.DissolvedAt)
}

func TestPlatformAdminCannotDissolveOrganization(t *testing.T) {
	setupServiceTestDB(t)
	creator := createServiceTestUser(t, "platform-dissolve-creator", common.RoleCommonUser)
	platformAdmin := createServiceTestUser(t, "platform-dissolve-admin", common.RoleAdminUser)
	organization, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Platform Dissolve Denied Org"})
	require.NoError(t, err)

	err = DissolveOrganization(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, DissolveOrganizationRequest{ConfirmName: organization.Name, Reason: "denied", IdempotencyKey: testOrganizationIdempotencyKey(t, "dissolve")})

	require.ErrorContains(t, err, "permission denied")
	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, model.OrganizationStatusActive, stored.Status)
}

func TestRootCanDissolveOrganization(t *testing.T) {
	setupServiceTestDB(t)
	creator := createServiceTestUser(t, "root-dissolve-creator", common.RoleCommonUser)
	root := createServiceTestUser(t, "root-dissolve-user", common.RoleRootUser)
	organization, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Root Dissolve Org"})
	require.NoError(t, err)

	require.NoError(t, DissolveOrganization(root.Id, organization.Id, OrganizationAccessModeAdmin, DissolveOrganizationRequest{ConfirmName: organization.Name, Reason: "root", IdempotencyKey: testOrganizationIdempotencyKey(t, "dissolve")}))
	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, model.OrganizationStatusDissolved, stored.Status)

	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionDissolve).First(&audit).Error)
	require.Equal(t, organizationAuditOperatorRolePlatformRoot, audit.OperatorRole)
}

func TestUpdateOrganizationRejectsDissolvedOrganization(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "dissolved-update-admin", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Dissolved Update Org"})
	require.NoError(t, err)
	require.NoError(t, dissolveOrganizationForTest(t, admin.Id, organization.Id, "done"))

	_, err = UpdateOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, UpdateOrganizationRequest{Name: "After Dissolve"})

	require.ErrorContains(t, err, "organization dissolved")
}

func TestGetOrganizationForMemberAllowsDissolvedHistoricalReadForOwner(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "history-admin", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "History Org"})
	require.NoError(t, err)
	require.NoError(t, dissolveOrganizationForTest(t, admin.Id, organization.Id, "history"))

	readOrg, readMember, err := GetOrganizationForMember(admin.Id, organization.Id, true)

	require.NoError(t, err)
	require.Equal(t, model.OrganizationStatusDissolved, readOrg.Status)
	require.Equal(t, model.OrganizationRoleOwner, readMember.Role)
}

func TestOrganizationCreateUpdateAndDissolveWriteAuditLogs(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "audit-admin", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Audit Org"})
	require.NoError(t, err)
	_, err = UpdateOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, UpdateOrganizationRequest{Name: "Audit Org Updated"})
	require.NoError(t, err)
	require.NoError(t, dissolveOrganizationForTest(t, admin.Id, organization.Id, "audit"))

	var auditCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationAuditLog{}).Where("organization_id = ?", organization.Id).Count(&auditCount).Error)
	require.EqualValues(t, 3, auditCount)
}

func TestDissolveOrganizationDisablesOrganizationTokensAndRevokesPendingInvites(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "dissolve-side-effect-admin", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Dissolve Side Effect Org"})
	require.NoError(t, err)
	token := model.Token{UserId: admin.Id, Key: "444444444444444444444444444444444444444444444444", Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id}
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "a@example.com", Role: model.OrganizationRoleMember, Token: "invite-token", Status: model.OrganizationInviteStatusPending, InviterUserId: admin.Id}
	require.NoError(t, model.DB.Create(&token).Error)
	require.NoError(t, model.DB.Create(&invite).Error)

	require.NoError(t, dissolveOrganizationForTest(t, admin.Id, organization.Id, "closing"))

	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.Equal(t, common.TokenStatusDisabled, storedToken.Status)
	var activeBlockers int64
	require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).Where("token_id = ? AND reason = ? AND status = ?", token.Id, organizationTokenBlockerReasonOrganizationDissolved, model.OrganizationTokenBlockerStatusActive).Count(&activeBlockers).Error)
	require.EqualValues(t, 1, activeBlockers)
	var storedInvite model.OrganizationInvite
	require.NoError(t, model.DB.First(&storedInvite, invite.Id).Error)
	require.Equal(t, model.OrganizationInviteStatusRevoked, storedInvite.Status)
}

func TestOrganizationQuotaAdjustmentRejectsCommonUser(t *testing.T) {
	setupServiceTestDB(t)
	commonUser := createServiceTestUser(t, "quota-common-user", common.RoleCommonUser)
	organization, err := CreateOrganization(commonUser.Id, CreateOrganizationRequest{Name: "Quota Denied Org"})
	require.NoError(t, err)

	_, err = AdjustOrganizationQuota(commonUser.Id, organization.Id, OrganizationAccessModeAdmin, OrganizationQuotaAdjustmentRequest{QuotaDelta: 100, Reason: "denied", IdempotencyKey: testOrganizationIdempotencyKey(t, "quota-adjust")})

	require.ErrorContains(t, err, "permission denied")
}

func TestOrganizationQuotaAdjustmentAllowsPlatformAdmin(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "quota-platform-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "quota-creator", common.RoleCommonUser)
	organization, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Quota Org"})
	require.NoError(t, err)

	adjustment, err := AdjustOrganizationQuota(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, OrganizationQuotaAdjustmentRequest{QuotaDelta: 700, Reason: "increase", IdempotencyKey: testOrganizationIdempotencyKey(t, "quota-adjust")})

	require.NoError(t, err)
	require.Equal(t, model.OrganizationDefaultQuota, adjustment.QuotaBefore)
	require.Equal(t, 0, adjustment.QuotaBefore)
	require.Equal(t, 700, adjustment.QuotaAfter)
	require.Equal(t, 700, adjustment.QuotaDelta)
}

func TestAdjustOrganizationQuotaLocksOrganizationRow(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "quota-lock-platform-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "quota-lock-creator", common.RoleCommonUser)
	organization, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Quota Lock Org"})
	require.NoError(t, err)

	organizationLocked := false
	callbackName := "test:organization-quota-lock"
	require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != "organizations" {
			return
		}
		forClause, ok := tx.Statement.Clauses["FOR"]
		if !ok {
			return
		}
		locking, ok := forClause.Expression.(clause.Locking)
		organizationLocked = ok && locking.Strength == "UPDATE"
	}))
	t.Cleanup(func() {
		_ = model.DB.Callback().Query().Remove(callbackName)
	})

	_, err = AdjustOrganizationQuota(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, OrganizationQuotaAdjustmentRequest{
		QuotaDelta:     100,
		Reason:         "lock",
		IdempotencyKey: testOrganizationIdempotencyKey(t, "quota-lock"),
	})

	require.NoError(t, err)
	requireOrganizationRowLock(t, organizationLocked, "quota adjustment must lock the organization row before calculating quota")
}

func TestOrganizationQuotaAdjustmentRejectsQuotaBelowUsedQuota(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "quota-low-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "quota-low-creator", common.RoleCommonUser)
	organization, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Quota Low Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("used_quota", 1500).Error)

	_, err = AdjustOrganizationQuota(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, OrganizationQuotaAdjustmentRequest{QuotaDelta: -1000, Reason: "too low", IdempotencyKey: testOrganizationIdempotencyKey(t, "quota-adjust")})

	require.ErrorContains(t, err, "quota cannot be less than used quota")
}

func TestOrganizationQuotaAdjustmentWritesAdjustmentAndAudit(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "quota-audit-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "quota-audit-creator", common.RoleCommonUser)
	organization, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Quota Audit Org"})
	require.NoError(t, err)

	adjustment, err := AdjustOrganizationQuota(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, OrganizationQuotaAdjustmentRequest{QuotaDelta: 100, Reason: "audit", IdempotencyKey: testOrganizationIdempotencyKey(t, "quota-adjust")})

	require.NoError(t, err)
	require.NotZero(t, adjustment.Id)
	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, model.OrganizationDefaultQuota+100, stored.Quota)
	var auditCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationAuditLog{}).Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionQuotaAdjust).Count(&auditCount).Error)
	require.EqualValues(t, 1, auditCount)
}

func TestOrganizationQuotaAdjustmentIdempotencyReplaysWithoutDoubleApplying(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "quota-idempotent-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "quota-idempotent-creator", common.RoleCommonUser)
	organization, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Quota Idempotent Org"})
	require.NoError(t, err)

	first, err := AdjustOrganizationQuota(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, OrganizationQuotaAdjustmentRequest{QuotaDelta: 120, Reason: "same request", IdempotencyKey: "quota-idempotent-key"})
	require.NoError(t, err)
	second, err := AdjustOrganizationQuota(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, OrganizationQuotaAdjustmentRequest{QuotaDelta: 120, Reason: "same request", IdempotencyKey: "quota-idempotent-key"})

	require.NoError(t, err)
	require.Equal(t, first.Id, second.Id)
	require.Equal(t, first.QuotaAfter, second.QuotaAfter)
	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, organization.Quota+120, stored.Quota)
	var adjustmentCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationQuotaAdjustment{}).Where("idempotency_key = ?", "quota-idempotent-key").Count(&adjustmentCount).Error)
	require.EqualValues(t, 1, adjustmentCount)
	var billingRecordCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationBillingRecord{}).Where("record_key = ?", "adjust:quota-idempotent-key").Count(&billingRecordCount).Error)
	require.EqualValues(t, 1, billingRecordCount)
	var auditCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationAuditLog{}).Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionQuotaAdjust).Count(&auditCount).Error)
	require.EqualValues(t, 1, auditCount)
}

func TestOrganizationQuotaAdjustmentIdempotencyConflictRejectsDifferentRequest(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "quota-idempotent-conflict-admin", common.RoleAdminUser)
	creator := createServiceTestUser(t, "quota-idempotent-conflict-creator", common.RoleCommonUser)
	organization, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Quota Idempotent Conflict Org"})
	require.NoError(t, err)
	_, err = AdjustOrganizationQuota(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, OrganizationQuotaAdjustmentRequest{QuotaDelta: 120, Reason: "original", IdempotencyKey: "quota-idempotent-conflict-key"})
	require.NoError(t, err)

	_, err = AdjustOrganizationQuota(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, OrganizationQuotaAdjustmentRequest{QuotaDelta: 121, Reason: "original", IdempotencyKey: "quota-idempotent-conflict-key"})

	require.ErrorContains(t, err, "organization idempotency conflict")
	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, organization.Quota+120, stored.Quota)
}

func TestDisableOrganizationSetsInactiveStatusAndDisablesTokens(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "disable-status-admin", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Disable Status Org"})
	require.NoError(t, err)
	token := model.Token{UserId: admin.Id, Key: "555555555555555555555555555555555555555555555555", Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id}
	require.NoError(t, model.DB.Create(&token).Error)

	require.NoError(t, DisableOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "maintenance"))

	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, model.OrganizationStatusDisabled, stored.Status)
	require.Zero(t, stored.DissolvedAt)
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.Equal(t, common.TokenStatusDisabled, storedToken.Status)
	visibleToken, err := GetOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeReadOnly, token.Id)
	require.NoError(t, err)
	require.True(t, visibleToken.DisabledBySystems)
	require.Equal(t, organizationTokenBlockerReasonOrganizationSelf, visibleToken.SystemDisabledReason)
}

func TestOrganizationStatusRequiresSlugConfirmation(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "status-confirmation-owner", common.RoleCommonUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Status Confirmation Org"})
	require.NoError(t, err)

	for _, confirmation := range []string{"", organization.Name, "wrong-slug"} {
		err = DisableOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, confirmation, "maintenance")
		require.ErrorContains(t, err, "organization confirmation mismatch")
		var stored model.Organization
		require.NoError(t, model.DB.First(&stored, organization.Id).Error)
		require.Equal(t, model.OrganizationStatusActive, stored.Status)
		var recordCount int64
		require.NoError(t, model.DB.Model(&model.OrganizationDisableRecord{}).Where("organization_id = ?", organization.Id).Count(&recordCount).Error)
		require.Zero(t, recordCount)
	}
	var auditCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationAuditLog{}).Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionDisable).Count(&auditCount).Error)
	require.Zero(t, auditCount)

	require.NoError(t, DisableOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, "  "+organization.Slug+"  ", "maintenance"))
	for _, confirmation := range []string{"", organization.Name, "wrong-slug"} {
		err = EnableOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, confirmation, "resume")
		require.ErrorContains(t, err, "organization confirmation mismatch")
		var stored model.Organization
		require.NoError(t, model.DB.First(&stored, organization.Id).Error)
		require.Equal(t, model.OrganizationStatusDisabled, stored.Status)
		var activeRecordCount int64
		require.NoError(t, model.DB.Model(&model.OrganizationDisableRecord{}).Where("organization_id = ? AND status = ?", organization.Id, model.OrganizationRecordStatusActive).Count(&activeRecordCount).Error)
		require.EqualValues(t, 1, activeRecordCount)
	}
	require.NoError(t, model.DB.Model(&model.OrganizationAuditLog{}).Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionEnable).Count(&auditCount).Error)
	require.Zero(t, auditCount)
	require.NoError(t, EnableOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "resume"))
}

func TestPlatformOrganizationStatusRequiresSlugConfirmation(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "platform-status-confirmation-admin", common.RoleAdminUser)
	owner := createServiceTestUser(t, "platform-status-confirmation-owner", common.RoleCommonUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Platform Status Confirmation Org"})
	require.NoError(t, err)

	for _, confirmation := range []string{"", organization.Name, "wrong-slug"} {
		err = DisableOrganizationByPlatform(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, confirmation, "maintenance")
		require.ErrorContains(t, err, "organization confirmation mismatch")
	}
	var recordCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationDisableRecord{}).Where("organization_id = ?", organization.Id).Count(&recordCount).Error)
	require.Zero(t, recordCount)
	var auditCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationAuditLog{}).Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionDisable).Count(&auditCount).Error)
	require.Zero(t, auditCount)

	require.NoError(t, DisableOrganizationByPlatform(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, organization.Slug, "maintenance"))
	for _, confirmation := range []string{"", organization.Name, "wrong-slug"} {
		err = EnableOrganizationByPlatform(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, confirmation, "resume")
		require.ErrorContains(t, err, "organization confirmation mismatch")
	}
	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, model.OrganizationStatusDisabled, stored.Status)
	require.NoError(t, model.DB.Model(&model.OrganizationDisableRecord{}).Where("organization_id = ? AND status = ?", organization.Id, model.OrganizationRecordStatusActive).Count(&recordCount).Error)
	require.EqualValues(t, 1, recordCount)
	require.NoError(t, model.DB.Model(&model.OrganizationAuditLog{}).Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionEnable).Count(&auditCount).Error)
	require.Zero(t, auditCount)
	require.NoError(t, EnableOrganizationByPlatform(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, organization.Slug, "resume"))
}

func TestEnableOrganizationRestoresDisabledOrganization(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "enable-status-admin", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Enable Status Org"})
	require.NoError(t, err)
	require.NoError(t, DisableOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "maintenance"))

	require.NoError(t, EnableOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "resume"))

	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, model.OrganizationStatusActive, stored.Status)
}

func TestEnableOrganizationByPlatformOverridesSelfDisable(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "platform-enable-self-disabled-owner", common.RoleCommonUser)
	platformAdmin := createServiceTestUser(t, "platform-enable-self-disabled-admin", common.RoleAdminUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Platform Enable Self Disabled Org"})
	require.NoError(t, err)
	token := model.Token{UserId: owner.Id, Key: "616161616161616161616161616161616161616161616161", Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, ResponsibleUserId: owner.Id}
	require.NoError(t, model.DB.Create(&token).Error)
	require.NoError(t, DisableOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "self maintenance"))

	require.NoError(t, EnableOrganizationByPlatform(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, organization.Slug, "platform resume"))

	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, model.OrganizationStatusActive, stored.Status)
	var activeCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationDisableRecord{}).Where("organization_id = ? AND status = ?", organization.Id, model.OrganizationRecordStatusActive).Count(&activeCount).Error)
	require.EqualValues(t, 0, activeCount)
	require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).Where("token_id = ? AND status = ?", token.Id, model.OrganizationTokenBlockerStatusActive).Count(&activeCount).Error)
	require.EqualValues(t, 0, activeCount)
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.Equal(t, common.TokenStatusEnabled, storedToken.Status)
	require.False(t, storedToken.DisabledBySystems)
}

func TestOrganizationDisableRecordsCoexistAndDeriveStatus(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "disable-record-owner", common.RoleCommonUser)
	platformAdmin := createServiceTestUser(t, "disable-record-platform-admin", common.RoleAdminUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Disable Record Org"})
	require.NoError(t, err)
	token := model.Token{UserId: owner.Id, Key: "515151515151515151515151515151515151515151515151", Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, ResponsibleUserId: owner.Id}
	require.NoError(t, model.DB.Create(&token).Error)

	require.NoError(t, DisableOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "self maintenance"))
	require.NoError(t, DisableOrganizationByPlatform(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, organization.Slug, "platform hold"))

	var activeRecords []model.OrganizationDisableRecord
	require.NoError(t, model.DB.Where("organization_id = ? AND status = ?", organization.Id, model.OrganizationRecordStatusActive).Order("source asc").Find(&activeRecords).Error)
	require.Len(t, activeRecords, 2)
	require.Equal(t, model.OrganizationDisableSourcePlatform, activeRecords[0].Source)
	require.Equal(t, model.OrganizationDisableSourceSelf, activeRecords[1].Source)
	var activeBlockers int64
	require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).Where("token_id = ? AND status = ?", token.Id, model.OrganizationTokenBlockerStatusActive).Count(&activeBlockers).Error)
	require.EqualValues(t, 2, activeBlockers)

	require.NoError(t, EnableOrganizationByPlatform(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, organization.Slug, "platform release"))
	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, model.OrganizationStatusActive, stored.Status)
	require.NoError(t, model.DB.Model(&model.OrganizationDisableRecord{}).Where("organization_id = ? AND status = ?", organization.Id, model.OrganizationRecordStatusActive).Count(&activeBlockers).Error)
	require.EqualValues(t, 0, activeBlockers)
	require.NoError(t, model.DB.Model(&model.OrganizationTokenSystemBlocker{}).Where("token_id = ? AND status = ?", token.Id, model.OrganizationTokenBlockerStatusActive).Count(&activeBlockers).Error)
	require.EqualValues(t, 0, activeBlockers)
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.Equal(t, common.TokenStatusEnabled, storedToken.Status)
	require.False(t, storedToken.DisabledBySystems)
}

func TestEnableOrganizationRejectsOwnerWhenPlatformDisableRecordActive(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "enable-platform-disabled-owner", common.RoleCommonUser)
	platformAdmin := createServiceTestUser(t, "enable-platform-disabled-admin", common.RoleAdminUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Platform Disabled Org"})
	require.NoError(t, err)
	require.NoError(t, DisableOrganizationByPlatform(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, organization.Slug, "platform disable"))

	err = EnableOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "owner resume")

	require.ErrorContains(t, err, "permission denied")
	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, model.OrganizationStatusDisabled, stored.Status)
}

func TestPlatformAdminCannotMutateSelfDisableSource(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "self-source-owner", common.RoleCommonUser)
	platformAdmin := createServiceTestUser(t, "self-source-platform-admin", common.RoleAdminUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Self Source Org"})
	require.NoError(t, err)

	err = DisableOrganization(platformAdmin.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "wrong source")

	require.ErrorContains(t, err, "permission denied")
	var recordCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationDisableRecord{}).Where("organization_id = ? AND source = ?", organization.Id, model.OrganizationDisableSourceSelf).Count(&recordCount).Error)
	require.EqualValues(t, 0, recordCount)
	require.NoError(t, DisableOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "self hold"))
	err = EnableOrganization(platformAdmin.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "wrong release")
	require.ErrorContains(t, err, "permission denied")
	require.NoError(t, model.DB.Model(&model.OrganizationDisableRecord{}).Where("organization_id = ? AND source = ? AND status = ?", organization.Id, model.OrganizationDisableSourceSelf, model.OrganizationRecordStatusActive).Count(&recordCount).Error)
	require.EqualValues(t, 1, recordCount)
}

func TestEnableOrganizationRestoresOnlyTokensDisabledBySystems(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "restore-org-token-admin", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Restore Token Org"})
	require.NoError(t, err)

	autoDisabledToken := model.Token{
		UserId:            admin.Id,
		Key:               "777777777777777777777777777777777777777777777777",
		Status:            common.TokenStatusEnabled,
		ScopeType:         model.TokenScopeOrganization,
		ScopeId:           organization.Id,
		OrganizationId:    organization.Id,
		ResponsibleUserId: admin.Id,
	}
	manuallyDisabledToken := model.Token{
		UserId:            admin.Id,
		Key:               "888888888888888888888888888888888888888888888888",
		Status:            common.TokenStatusDisabled,
		ScopeType:         model.TokenScopeOrganization,
		ScopeId:           organization.Id,
		OrganizationId:    organization.Id,
		ResponsibleUserId: admin.Id,
	}
	require.NoError(t, model.DB.Create(&autoDisabledToken).Error)
	require.NoError(t, model.DB.Create(&manuallyDisabledToken).Error)

	require.NoError(t, DisableOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "maintenance"))

	var afterDisableAuto model.Token
	require.NoError(t, model.DB.First(&afterDisableAuto, autoDisabledToken.Id).Error)
	require.Equal(t, common.TokenStatusDisabled, afterDisableAuto.Status)
	require.True(t, afterDisableAuto.DisabledBySystems)
	visibleAuto, err := GetOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeReadOnly, autoDisabledToken.Id)
	require.NoError(t, err)
	require.True(t, visibleAuto.DisabledBySystems)
	require.Equal(t, organizationTokenBlockerReasonOrganizationSelf, visibleAuto.SystemDisabledReason)

	var afterDisableManual model.Token
	require.NoError(t, model.DB.First(&afterDisableManual, manuallyDisabledToken.Id).Error)
	require.Equal(t, common.TokenStatusDisabled, afterDisableManual.Status)
	require.True(t, afterDisableManual.DisabledBySystems)

	require.NoError(t, EnableOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "resume"))

	var afterEnableAuto model.Token
	require.NoError(t, model.DB.First(&afterEnableAuto, autoDisabledToken.Id).Error)
	require.Equal(t, common.TokenStatusEnabled, afterEnableAuto.Status)
	require.False(t, afterEnableAuto.DisabledBySystems)

	var afterEnableManual model.Token
	require.NoError(t, model.DB.First(&afterEnableManual, manuallyDisabledToken.Id).Error)
	require.Equal(t, common.TokenStatusDisabled, afterEnableManual.Status)
	require.False(t, afterEnableManual.DisabledBySystems)
}

func TestDissolveOrganizationCreatesTerminalTokenBlocker(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "dissolve-blocker-owner", common.RoleCommonUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Dissolve Blocker Org"})
	require.NoError(t, err)
	token := model.Token{UserId: owner.Id, Key: "525252525252525252525252525252525252525252525252", Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, ResponsibleUserId: owner.Id}
	require.NoError(t, model.DB.Create(&token).Error)

	require.NoError(t, dissolveOrganizationForTest(t, owner.Id, organization.Id, "closed"))

	var blocker model.OrganizationTokenSystemBlocker
	require.NoError(t, model.DB.Where("token_id = ? AND status = ?", token.Id, model.OrganizationTokenBlockerStatusActive).First(&blocker).Error)
	require.Equal(t, organizationTokenBlockerReasonOrganizationDissolved, blocker.Reason)
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.Equal(t, common.TokenStatusDisabled, storedToken.Status)
	require.True(t, storedToken.DisabledBySystems)
	require.NoError(t, hydrateOrganizationTokenAvailability([]*model.Token{&storedToken}))
	require.True(t, storedToken.DisabledBySystems)
	require.Equal(t, organizationTokenBlockerReasonOrganizationDissolved, storedToken.SystemDisabledReason)
	require.ErrorContains(t, EnableOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "reopen"), "organization dissolved")
}

func TestDisableOrganizationRejectsMember(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "disable-admin", common.RoleCommonUser)
	member := createServiceTestUser(t, "disable-member", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Disable Reject Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	err = DisableOrganization(member.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "denied")

	require.ErrorContains(t, err, "permission denied")
}

func TestDisabledOrganizationHiddenFromSwitchableAccountContextsAndRelayRejected(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "disabled-context-admin", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Disabled Context Org"})
	require.NoError(t, err)
	token := model.Token{UserId: admin.Id, Key: "666666666666666666666666666666666666666666666666", Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, ResponsibleUserId: admin.Id}
	require.NoError(t, model.DB.Create(&token).Error)
	require.NoError(t, DisableOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "maintenance"))

	contexts, err := ListAccountContexts(admin.Id)
	require.NoError(t, err)
	for _, context := range contexts.Contexts {
		if context.Type == model.AccountContextTypeOrganization {
			require.NotEqual(t, organization.Id, context.Id)
		}
	}
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	// 组织被封禁后令牌状态被置为不可用。cuberouter 用 model.ErrTokenInvalid
	// 这个哨兵错误表达，而不是源仓库的中文字符串，所以这里断言哨兵本身。
	require.NotEqual(t, common.TokenStatusEnabled, storedToken.Status)
	_, err = model.ValidateUserToken(storedToken.Key)
	require.ErrorIs(t, err, model.ErrTokenInvalid)
}

func TestListUserOrganizationsIncludesDisabledOrganizationsWithRole(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "disabled-list-admin", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Disabled List Org"})
	require.NoError(t, err)
	require.NoError(t, DisableOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "maintenance"))

	organizations, err := ListUserOrganizations(admin.Id, false)

	require.NoError(t, err)
	require.Len(t, organizations, 1)
	require.Equal(t, organization.Id, organizations[0].Id)
	require.Equal(t, model.OrganizationStatusDisabled, organizations[0].Status)
	require.Equal(t, model.OrganizationRoleOwner, organizations[0].Role)
	require.True(t, organizations[0].CanSelfEnable)
	require.Equal(t, model.OrganizationDisableSourceSelf, organizations[0].DisableState.EffectiveSource)
	require.Equal(t, OrganizationAccessModeManagement, organizations[0].AccessMode)
	require.True(t, organizations[0].Capabilities.CanEnableOrganization)
	require.False(t, organizations[0].Capabilities.CanUpdateOrganization)
	require.False(t, organizations[0].Capabilities.CanDisableOrganization)
	require.True(t, organizations[0].Capabilities.CanViewOrganizationLogs)
}

func TestListUserOrganizationsMarksPlatformDisabledOrganizationReadOnly(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "platform-disabled-list-platform-admin", common.RoleAdminUser)
	owner := createServiceTestUser(t, "platform-disabled-list-owner", common.RoleCommonUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Platform Disabled List Org"})
	require.NoError(t, err)
	require.NoError(t, DisableOrganizationByPlatform(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, organization.Slug, "platform hold"))

	organizations, err := ListUserOrganizations(owner.Id, false)

	require.NoError(t, err)
	require.Len(t, organizations, 1)
	require.Equal(t, organization.Id, organizations[0].Id)
	require.Equal(t, model.OrganizationStatusDisabled, organizations[0].Status)
	require.Equal(t, model.OrganizationRoleOwner, organizations[0].Role)
	require.False(t, organizations[0].CanSelfEnable)
	require.Equal(t, OrganizationAccessModeManagement, organizations[0].AccessMode)
	require.Equal(t, model.OrganizationDisableSourcePlatform, organizations[0].DisableState.EffectiveSource)
	require.False(t, organizations[0].Capabilities.CanEnableOrganization)
	require.True(t, organizations[0].Capabilities.CanViewOrganization)
	require.True(t, organizations[0].Capabilities.CanViewOrganizationLogs)
}

func TestListUserOrganizationsPlatformMemberUsesWorkspaceCapabilities(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "platform-list-owner", common.RoleCommonUser)
	platformAdmin := createServiceTestUser(t, "platform-list-member", common.RoleAdminUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Platform Member List Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: platformAdmin.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	organizations, err := ListUserOrganizations(platformAdmin.Id, false)

	require.NoError(t, err)
	require.Len(t, organizations, 1)
	require.Equal(t, model.OrganizationRoleMember, organizations[0].Role)
	require.Equal(t, OrganizationAccessModeWorkspace, organizations[0].AccessMode)
	require.True(t, organizations[0].Capabilities.CanViewOrganization)
	require.False(t, organizations[0].Capabilities.CanManageMembers)
	require.False(t, organizations[0].Capabilities.CanUpdateOrganization)
}

func TestListUserOrganizationsDisabledMemberCannotEnterWhileAdminCanRecover(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "disabled-list-owner", common.RoleCommonUser)
	admin := createServiceTestUser(t, "disabled-list-org-admin", common.RoleCommonUser)
	member := createServiceTestUser(t, "disabled-list-member", common.RoleCommonUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Disabled Membership List Org"})
	require.NoError(t, err)
	for _, membership := range []model.OrganizationMember{
		{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive},
		{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive},
	} {
		require.NoError(t, model.DB.Create(&membership).Error)
	}
	require.NoError(t, DisableOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "maintenance"))

	memberOrganizations, err := ListUserOrganizations(member.Id, false)
	require.NoError(t, err)
	require.Len(t, memberOrganizations, 1)
	require.Equal(t, OrganizationAccessModeManagement, memberOrganizations[0].AccessMode)
	require.False(t, memberOrganizations[0].Capabilities.CanViewOrganization)
	require.False(t, memberOrganizations[0].Capabilities.CanEnableOrganization)

	adminOrganizations, err := ListUserOrganizations(admin.Id, false)
	require.NoError(t, err)
	require.Len(t, adminOrganizations, 1)
	require.Equal(t, OrganizationAccessModeManagement, adminOrganizations[0].AccessMode)
	require.True(t, adminOrganizations[0].Capabilities.CanViewOrganization)
	require.True(t, adminOrganizations[0].Capabilities.CanEnableOrganization)
}

func TestUpdateOrganizationDoesNotResetGroupWhenGroupOmitted(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "update-keep-group-admin", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Keep Group Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("group", "vip").Error)

	updated, err := UpdateOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, UpdateOrganizationRequest{Name: "Keep Group Org Updated", Description: "updated"})

	require.NoError(t, err)
	require.Equal(t, "vip", updated.Group)
	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, "vip", stored.Group)
}

func TestUpdateOrganizationIgnoresGroupForOrganizationAdmin(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "update-ignore-group-admin", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Ignore Group Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("group", "vip").Error)

	updated, err := UpdateOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, UpdateOrganizationRequest{Name: "Ignore Group Org Updated", Description: "updated", Group: stringPtr("default")})

	require.NoError(t, err)
	require.Equal(t, "vip", updated.Group)
	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, "vip", stored.Group)
}

func TestUpdateOrganizationAllowsPlatformRootToChangeGroup(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "update-group-root-owner", common.RoleCommonUser)
	root := createServiceTestUser(t, "update-group-root", common.RoleRootUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Root Group Org"})
	require.NoError(t, err)

	updated, err := UpdateOrganization(root.Id, organization.Id, OrganizationAccessModeAdmin, UpdateOrganizationRequest{
		Name:   organization.Name,
		Group:  stringPtr("vip"),
		Reason: "root correction",
	})

	require.NoError(t, err)
	require.Equal(t, "vip", updated.Group)
}

func TestDissolveOrganizationSuccessfulReplayPrecedesCurrentAuthorization(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "dissolve-replay-owner", common.RoleCommonUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Dissolve Replay Org"})
	require.NoError(t, err)
	req := DissolveOrganizationRequest{ConfirmName: organization.Name, Reason: "closed", IdempotencyKey: "dissolve-success-replay"}

	require.NoError(t, DissolveOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, req))
	require.NoError(t, DissolveOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, req))

	conflict := req
	conflict.Reason = "different content"
	require.ErrorContains(t, DissolveOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, conflict), "organization idempotency conflict")
}
