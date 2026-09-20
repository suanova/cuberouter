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
