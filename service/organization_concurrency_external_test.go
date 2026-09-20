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
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestConcurrentUpdateOrganizationReturnsTypedNormalizedNameConflictExternalDatabase(t *testing.T) {
	deadline := organizationNameConcurrencyTestDeadline(t)
	db, _ := setupOrganizationExternalConcurrencyDB(t)
	// UpdateOrganization 走组织策略加载（loadOrganizationPolicyDisableState），
	// PostgreSQL 下缺失 organization_disable_records 表会使事务中止（SQLSTATE
	// 25P02），因此本测试必须自建该表，不能依赖其他测试遗留的表状态。
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.OrganizationDisableRecord{},
		&model.OrganizationAuditLog{},
	))

	suffix := strings.ReplaceAll(common.GetUUID(), "-", "")[:8]
	owner := model.User{Username: "external-name-update-" + suffix, Password: "password", DisplayName: "external-name-update", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "enu-" + suffix}
	require.NoError(t, db.Create(&owner).Error)
	organizations := make([]*model.Organization, 0, 2)
	for _, name := range []string{"External First " + suffix, "External Second " + suffix} {
		organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: name})
		require.NoError(t, err)
		organizations = append(organizations, organization)
	}
	t.Cleanup(func() {
		organizationIds := []int{organizations[0].Id, organizations[1].Id}
		require.NoError(t, db.Where("organization_id IN ?", organizationIds).Delete(&model.OrganizationAuditLog{}).Error)
		require.NoError(t, db.Where("organization_id IN ?", organizationIds).Delete(&model.OrganizationMember{}).Error)
		require.NoError(t, db.Where("id IN ?", organizationIds).Delete(&model.Organization{}).Error)
		require.NoError(t, db.Unscoped().Delete(&model.User{}, owner.Id).Error)
	})

	targetNormalized := "external concurrent target " + suffix
	updateReady := make(chan struct{}, len(organizations))
	releaseUpdates := make(chan struct{})
	callbackName := "test:concurrent-organization-name-update-barrier"
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != "organizations" {
			return
		}
		organization, ok := tx.Statement.Dest.(*model.Organization)
		if !ok || organization.NameNormalized != targetNormalized {
			return
		}
		updateReady <- struct{}{}
		<-releaseUpdates
	}))
	t.Cleanup(func() { _ = db.Callback().Update().Remove(callbackName) })

	start := make(chan struct{})
	results := make(chan error, len(organizations))
	for index, organization := range organizations {
		index, organization := index, organization
		go func() {
			<-start
			_, err := UpdateOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, UpdateOrganizationRequest{
				Name: []string{"External Concurrent Target " + suffix, "  external concurrent target " + suffix + "  "}[index],
			})
			results <- err
		}()
	}
	close(start)
	waitForOrganizationNameConcurrencyBarrier(t, updateReady, results, len(organizations), releaseUpdates, deadline)

	var successCount, conflictCount int
	for _, err := range waitForOrganizationNameConcurrencyResults(t, results, len(organizations), deadline) {
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

func TestOrganizationNameWriteConflictMappingExternalDatabase(t *testing.T) {
	db, _ := setupOrganizationExternalConcurrencyDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.OrganizationAuditLog{},
	))

	suffix := strings.ReplaceAll(common.GetUUID(), "-", "")[:8]
	owner := model.User{Username: "external-name-mapping-" + suffix, Password: "password", DisplayName: "external-name-mapping", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "enm-" + suffix}
	require.NoError(t, db.Create(&owner).Error)
	existing, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "External Mapping Existing " + suffix})
	require.NoError(t, err)
	target, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "External Mapping Target " + suffix})
	require.NoError(t, err)
	t.Cleanup(func() {
		organizationIds := []int{existing.Id, target.Id}
		require.NoError(t, db.Where("organization_id IN ?", organizationIds).Delete(&model.OrganizationAuditLog{}).Error)
		require.NoError(t, db.Where("organization_id IN ?", organizationIds).Delete(&model.OrganizationMember{}).Error)
		require.NoError(t, db.Where("id IN ?", organizationIds).Delete(&model.Organization{}).Error)
		require.NoError(t, db.Unscoped().Delete(&model.User{}, owner.Id).Error)
	})

	target.Name = "External Mapping Existing " + suffix
	target.NameNormalized = "external mapping existing " + suffix
	err = db.Model(target).Select("name", "name_normalized").Updates(target).Error
	require.Error(t, err)
	require.True(t, model.IsDuplicateKeyError(db, err))
	require.ErrorIs(t, mapOrganizationNameWriteError(err), ErrOrganizationNameConflict)
}

func TestConcurrentOrganizationQuotaAdjustment(t *testing.T) {
	db, _ := setupOrganizationExternalConcurrencyDB(t)

	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.OrganizationDisableRecord{},
		&model.OrganizationQuotaAdjustment{},
		&model.OrganizationBillingRecord{},
		&model.OrganizationAuditLog{},
	))

	suffix := strings.ReplaceAll(common.GetUUID(), "-", "")[:8]
	platformAdmin := model.User{Username: "quota-admin-" + suffix, Password: "password", DisplayName: "quota-admin", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, AffCode: "qa-" + suffix}
	creator := model.User{Username: "quota-user-" + suffix, Password: "password", DisplayName: "quota-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "qu-" + suffix}
	organization := model.Organization{Name: "External Quota " + suffix, Slug: "external-quota-" + suffix, Status: model.OrganizationStatusActive, Quota: 1000}
	keys := []string{"quota-external-100-" + suffix, "quota-external-200-" + suffix}
	t.Cleanup(func() {
		require.NoError(t, db.Where("record_key IN ?", []string{"adjust:" + keys[0], "adjust:" + keys[1]}).Delete(&model.OrganizationBillingRecord{}).Error)
		require.NoError(t, db.Where("idempotency_key IN ?", keys).Delete(&model.OrganizationQuotaAdjustment{}).Error)
		if organization.Id > 0 {
			require.NoError(t, db.Where("organization_id = ?", organization.Id).Delete(&model.OrganizationAuditLog{}).Error)
			require.NoError(t, db.Where("organization_id = ?", organization.Id).Delete(&model.OrganizationDisableRecord{}).Error)
			require.NoError(t, db.Where("organization_id = ?", organization.Id).Delete(&model.OrganizationMember{}).Error)
			require.NoError(t, db.Delete(&model.Organization{}, organization.Id).Error)
		}
		userIds := make([]int, 0, 2)
		if platformAdmin.Id > 0 {
			userIds = append(userIds, platformAdmin.Id)
		}
		if creator.Id > 0 {
			userIds = append(userIds, creator.Id)
		}
		if len(userIds) > 0 {
			require.NoError(t, db.Unscoped().Where("id IN ?", userIds).Delete(&model.User{}).Error)
		}
	})

	require.NoError(t, db.Create(&platformAdmin).Error)
	require.NoError(t, db.Create(&creator).Error)
	organization.CreatedBy = creator.Id
	organization.OwnerUserId = creator.Id
	require.NoError(t, db.Create(&organization).Error)

	type adjustmentRequest struct {
		delta int
		key   string
	}
	requests := []adjustmentRequest{{delta: 100, key: keys[0]}, {delta: 200, key: keys[1]}}
	start := make(chan struct{})
	results := make(chan error, len(requests))
	for _, request := range requests {
		request := request
		go func() {
			<-start
			_, err := AdjustOrganizationQuota(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, OrganizationQuotaAdjustmentRequest{
				QuotaDelta:     request.delta,
				Reason:         "external concurrency test",
				IdempotencyKey: request.key,
			})

			results <- err
		}()
	}
	close(start)
	resultErrors := make([]error, 0, len(requests))
	for range requests {
		resultErrors = append(resultErrors, <-results)
	}
	for _, resultErr := range resultErrors {
		require.NoError(t, resultErr)
	}

	var stored model.Organization
	require.NoError(t, db.First(&stored, organization.Id).Error)
	require.Equal(t, organization.Quota+300, stored.Quota)
	var adjustments []model.OrganizationQuotaAdjustment
	require.NoError(t, db.Where("idempotency_key IN ?", keys).Find(&adjustments).Error)
	require.Len(t, adjustments, 2)
}

// TestConcurrentOrganizationTokenDeleteAndBillingSettle 锁定「删键」与「结算」两个
// 并发路径的最终状态：无论谁先执行，账本会话都必须结算、令牌额度必须落账，
// 而且被软删的令牌仍然要能收到结算写入——结算读的是未删除行会拿不到记录。
func TestConcurrentOrganizationTokenDeleteAndBillingSettle(t *testing.T) {
	db, _ := setupOrganizationExternalConcurrencyDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.OrganizationDisableRecord{},
		&model.OrganizationBillingSession{},
		&model.OrganizationBillingRecord{},
		&model.OrganizationAuditLog{},
	))

	type deleteSettleFixture struct {
		owner            model.User
		organization     model.Organization
		token            model.Token
		session          *BillingSession
		billingSessionId int
	}
	newFixture := func(t *testing.T) *deleteSettleFixture {
		t.Helper()
		suffix := strings.ReplaceAll(common.GetUUID(), "-", "")[:8]
		fixture := &deleteSettleFixture{
			owner:        model.User{Username: "delete-settle-owner-" + suffix, Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "ds-" + suffix},
			organization: model.Organization{Name: "Delete Settle " + suffix, Slug: "delete-settle-" + suffix, Status: model.OrganizationStatusActive, Quota: 2000},
		}
		t.Cleanup(func() {
			if fixture.organization.Id > 0 {
				require.NoError(t, db.Where("organization_id = ?", fixture.organization.Id).Delete(&model.OrganizationBillingRecord{}).Error)
				require.NoError(t, db.Where("organization_id = ?", fixture.organization.Id).Delete(&model.OrganizationBillingSession{}).Error)
				require.NoError(t, db.Where("organization_id = ?", fixture.organization.Id).Delete(&model.OrganizationAuditLog{}).Error)
				require.NoError(t, db.Where("organization_id = ?", fixture.organization.Id).Delete(&model.OrganizationDisableRecord{}).Error)
			}
			if fixture.token.Id > 0 {
				require.NoError(t, db.Unscoped().Where("id = ?", fixture.token.Id).Delete(&model.Token{}).Error)
			}
			if fixture.organization.Id > 0 {
				require.NoError(t, db.Where("organization_id = ?", fixture.organization.Id).Delete(&model.OrganizationMember{}).Error)
				require.NoError(t, db.Delete(&model.Organization{}, fixture.organization.Id).Error)
			}
			if fixture.owner.Id > 0 {
				require.NoError(t, db.Unscoped().Delete(&model.User{}, fixture.owner.Id).Error)
			}
		})

		require.NoError(t, db.Create(&fixture.owner).Error)
		fixture.organization.CreatedBy = fixture.owner.Id
		fixture.organization.OwnerUserId = fixture.owner.Id
		require.NoError(t, db.Create(&fixture.organization).Error)
		require.NoError(t, db.Create(&model.OrganizationMember{OrganizationId: fixture.organization.Id, UserId: fixture.owner.Id, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive}).Error)
		fixture.token = model.Token{UserId: fixture.owner.Id, Key: common.GetRandomString(48), Name: "delete-settle-key", Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 1000, ScopeType: model.TokenScopeOrganization, ScopeId: fixture.organization.Id, OrganizationId: fixture.organization.Id, CreatorUserId: fixture.owner.Id, ResponsibleUserId: fixture.owner.Id, Visibility: model.TokenVisibilityPrivate}
		require.NoError(t, db.Create(&fixture.token).Error)

		relayInfo := organizationBillingRelayInfo(&fixture.token, fixture.organization.Id)
		relayInfo.RequestId = "delete-settle-" + suffix
		session, apiErr := NewBillingSession(&gin.Context{}, relayInfo, 100)
		require.Nil(t, apiErr)
		fixture.session = session
		fixture.billingSessionId = relayInfo.OrganizationBillingSessionId
		return fixture
	}
	assertFinalState := func(t *testing.T, fixture *deleteSettleFixture) {
		t.Helper()
		var storedSession model.OrganizationBillingSession
		require.NoError(t, db.First(&storedSession, fixture.billingSessionId).Error)
		require.Equal(t, model.OrganizationBillingSessionStatusSettled, storedSession.Status)
		var deletedToken model.Token
		require.NoError(t, db.Unscoped().First(&deletedToken, fixture.token.Id).Error)
		require.True(t, deletedToken.DeletedAt.Valid)
		require.Equal(t, 860, deletedToken.RemainQuota)
		require.Equal(t, 140, deletedToken.UsedQuota)
		var storedOrganization model.Organization
		require.NoError(t, db.First(&storedOrganization, fixture.organization.Id).Error)
		require.Equal(t, 140, storedOrganization.UsedQuota)
	}

	t.Run("delete_then_settle", func(t *testing.T) {
		fixture := newFixture(t)
		require.NoError(t, DeleteOrganizationToken(fixture.owner.Id, fixture.organization.Id, OrganizationAccessModeWorkspace, fixture.token.Id))
		require.NoError(t, fixture.session.Settle(140))
		assertFinalState(t, fixture)
	})
	t.Run("settle_then_delete", func(t *testing.T) {
		fixture := newFixture(t)
		require.NoError(t, fixture.session.Settle(140))
		require.NoError(t, DeleteOrganizationToken(fixture.owner.Id, fixture.organization.Id, OrganizationAccessModeWorkspace, fixture.token.Id))
		assertFinalState(t, fixture)
	})
	t.Run("concurrent_smoke", func(t *testing.T) {
		fixture := newFixture(t)
		start := make(chan struct{})
		results := make(chan error, 2)
		go func() {
			<-start
			results <- DeleteOrganizationToken(fixture.owner.Id, fixture.organization.Id, OrganizationAccessModeWorkspace, fixture.token.Id)
		}()
		go func() { <-start; results <- fixture.session.Settle(140) }()
		close(start)
		resultErrors := []error{<-results, <-results}
		for _, resultErr := range resultErrors {
			require.NoError(t, resultErr)
		}
		assertFinalState(t, fixture)
	})
}
