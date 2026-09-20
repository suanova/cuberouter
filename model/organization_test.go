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
package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupModelTestDB 建一个测试专属的 SQLite 并把组织相关模型建表。
//
// 这些用例走包级全局 DB / LOG_DB，而 model 包里还有别的用例共用同一个测试二进制，
// 所以必须在 cleanup 里把全局和一个数据库方言标志原样还原。
// initCol() 要显式调用：它填的是 `key` / `group` 这两个保留字的引用格式，
// 不走 InitDB 的路径就必须自己补上。
func setupModelTestDB(t *testing.T) {
	t.Helper()
	originalDB, originalLogDB := DB, LOG_DB
	originalMainType, originalLogType := common.MainDatabaseType(), common.LogDatabaseType()
	originalRedisEnabled := common.RedisEnabled

	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/test.db"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	LOG_DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	initCol()
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		DB, LOG_DB = originalDB, originalLogDB
		common.SetDatabaseTypes(originalMainType, originalLogType)
		common.RedisEnabled = originalRedisEnabled
		initCol()
	})
	require.NoError(t, DB.AutoMigrate(
		&User{},
		&Token{},
		&Task{},
		&Midjourney{},
		&Log{},
		&QuotaData{},
		&Organization{},
		&OrganizationMember{},
		&OrganizationInvite{},
		&UserAccountContext{},
		&OrganizationDisableRecord{},
		&OrganizationTokenSystemBlocker{},
		&OrganizationIdempotencyRecord{},
		&OrganizationBillingSession{},
		&OrganizationBillingRecord{},
		&OrganizationAuditLog{},
		&OrganizationQuotaAdjustment{},
	))
}

type legacyOrganizationQuotaAdjustment struct {
	Id             int    `json:"id"`
	OrganizationId int    `json:"organization_id" gorm:"index;not null"`
	OperatorUserId int    `json:"operator_user_id" gorm:"index;not null"`
	QuotaDelta     int    `json:"quota_delta" gorm:"not null"`
	QuotaBefore    int    `json:"quota_before" gorm:"not null"`
	QuotaAfter     int    `json:"quota_after" gorm:"not null"`
	Reason         string `json:"reason" gorm:"type:varchar(255);default:''"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint;index"`
}

func (legacyOrganizationQuotaAdjustment) TableName() string {
	return "organization_quota_adjustments"
}

func TestOrganizationFoundationAutoMigrateCreatesRequiredTables(t *testing.T) {
	setupModelTestDB(t)

	for _, table := range []string{
		"organizations",
		"organization_members",
		"organization_invitations",
		"user_account_contexts",
		"organization_disable_records",
		"organization_token_system_blockers",
		"organization_idempotency_records",
		"organization_billing_sessions",
		"organization_billing_records",
		"organization_quota_adjustments",
		"organization_audit_logs",
	} {
		require.True(t, DB.Migrator().HasTable(table), "missing table %s", table)
	}
}

func TestOrganizationFoundationAutoMigrateCreatesRequiredColumns(t *testing.T) {
	setupModelTestDB(t)

	require.True(t, DB.Migrator().HasColumn(&Organization{}, "name_normalized"))
	require.True(t, DB.Migrator().HasIndex(&Organization{}, "idx_organizations_name_normalized"))
	require.True(t, DB.Migrator().HasColumn(&Organization{}, "owner_user_id"))
	require.True(t, DB.Migrator().HasColumn(&OrganizationMember{}, "joined_at"))
	require.True(t, DB.Migrator().HasColumn(&OrganizationMember{}, "last_active_at"))
	require.True(t, DB.Migrator().HasColumn(&OrganizationMember{}, "disabled_source"))
	require.True(t, DB.Migrator().HasColumn(&OrganizationInvite{}, "token_hash"))
	require.False(t, DB.Migrator().HasColumn(&OrganizationInvite{}, "token"), "raw invitation token must not be a model column")
	for _, column := range []string{"delivery_status", "delivery_attempts", "last_delivery_error", "delivered_at"} {
		require.True(t, DB.Migrator().HasColumn(&OrganizationInvite{}, column), "missing invite column %s", column)
	}

	for _, column := range []string{"system_disabled_reason", "system_disabled_ref_id", "system_disabled_at", "previous_status"} {
		require.True(t, DB.Migrator().HasColumn(&Token{}, column), "missing token column %s", column)
	}

	for _, column := range []string{"token_name", "group", "request_id", "organization_name", "actor_user_id", "creator_name", "responsible_name", "organization_billing_session_id", "organization_billing_session_key"} {
		require.True(t, DB.Migrator().HasColumn(&Log{}, column), "missing log column %s", column)
	}

	for _, column := range []string{"token_id", "token_name", "group", "request_id", "organization_name", "actor_user_id", "creator_name", "responsible_name", "organization_billing_session_id", "organization_billing_session_key"} {
		require.True(t, DB.Migrator().HasColumn(&Task{}, column), "missing task column %s", column)
		require.True(t, DB.Migrator().HasColumn(&Midjourney{}, column), "missing midjourney column %s", column)
	}
}

func TestOrganizationInviteJSONHidesDeliveryErrorAndToken(t *testing.T) {
	invite := OrganizationInvite{
		Id:                7,
		TargetEmail:       "target@example.com",
		Token:             "raw-token",
		TokenHash:         "token-hash",
		DeliveryStatus:    OrganizationInviteDeliveryFailed,
		LastDeliveryError: "recipient:550 user not found",
	}

	payload, err := common.Marshal(invite)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "raw-token")
	require.NotContains(t, string(payload), "token-hash")
	require.NotContains(t, string(payload), "last_delivery_error")
	require.NotContains(t, string(payload), "user not found")
}

func TestOrganizationAutoMigrateCreatesUniqueOrganizationUserMemberIndex(t *testing.T) {
	setupModelTestDB(t)
	organization := Organization{Name: "Test Org", Slug: "test-org", CreatedBy: 1}
	require.NoError(t, DB.Create(&organization).Error)
	member := OrganizationMember{OrganizationId: organization.Id, UserId: 1, Role: OrganizationRoleAdmin, Status: OrganizationMemberStatusActive}
	require.NoError(t, DB.Create(&member).Error)

	duplicate := OrganizationMember{OrganizationId: organization.Id, UserId: 1, Role: OrganizationRoleMember, Status: OrganizationMemberStatusActive}
	require.Error(t, DB.Create(&duplicate).Error)
}

func TestOrganizationDefaultQuotaIsZeroWhenQuotaIsOmitted(t *testing.T) {
	setupModelTestDB(t)
	organization := Organization{Name: "Default Quota Org", Slug: "default-quota-org", CreatedBy: 1}

	require.NoError(t, DB.Create(&organization).Error)

	require.NotZero(t, organization.Id)
	require.Equal(t, OrganizationDefaultQuota, organization.Quota)
	require.Equal(t, 0, organization.Quota)
	require.Equal(t, OrganizationStatusActive, organization.Status)
}

func TestOrganizationOwnerDefaultsToCreatorWhenOmitted(t *testing.T) {
	setupModelTestDB(t)
	organization := Organization{Name: "Owner Default Org", Slug: "owner-default-org", CreatedBy: 42}

	require.NoError(t, DB.Create(&organization).Error)

	require.Equal(t, organization.CreatedBy, organization.OwnerUserId)
}

func TestOrganizationMemberStatusConstantsIncludeDisabled(t *testing.T) {
	require.Equal(t, "disabled", OrganizationMemberStatusDisabled)
}

func TestOrganizationRoleConstantsIncludeOwner(t *testing.T) {
	require.Equal(t, "owner", OrganizationRoleOwner)
}

func TestOrganizationIdempotencyKeyModelsRejectEmptyKeys(t *testing.T) {
	setupModelTestDB(t)

	require.Error(t, DB.Create(&OrganizationIdempotencyRecord{OrganizationId: 1, OperationType: "batch", Status: "processing", CreatedBy: 1}).Error)
	require.Error(t, DB.Create(&OrganizationBillingSession{OrganizationId: 1, Status: "pre_consumed"}).Error)
	require.Error(t, DB.Create(&OrganizationBillingRecord{OrganizationId: 1, RecordType: "settle"}).Error)
	require.Error(t, DB.Create(&OrganizationQuotaAdjustment{OrganizationId: 1, QuotaDelta: 100, QuotaBefore: 1000, QuotaAfter: 1100, OperatorUserId: 1}).Error)
}

func TestOrganizationQuotaAdjustmentIdempotencyMigrationBackfillsLegacyRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/legacy.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacyOrganizationQuotaAdjustment{}))
	require.NoError(t, db.Create(&legacyOrganizationQuotaAdjustment{OrganizationId: 1, OperatorUserId: 2, QuotaDelta: 10, QuotaBefore: 100, QuotaAfter: 110, Reason: "legacy one", CreatedAt: 1000}).Error)
	require.NoError(t, db.Create(&legacyOrganizationQuotaAdjustment{OrganizationId: 1, OperatorUserId: 2, QuotaDelta: 20, QuotaBefore: 110, QuotaAfter: 130, Reason: "legacy two", CreatedAt: 1001}).Error)

	require.NoError(t, prepareOrganizationQuotaAdjustmentIdempotencyMigration(db))
	require.NoError(t, db.AutoMigrate(&OrganizationQuotaAdjustment{}))

	var adjustments []OrganizationQuotaAdjustment
	require.NoError(t, db.Order("id asc").Find(&adjustments).Error)
	require.Len(t, adjustments, 2)
	require.NotEmpty(t, adjustments[0].IdempotencyKey)
	require.NotEmpty(t, adjustments[1].IdempotencyKey)
	require.NotEqual(t, adjustments[0].IdempotencyKey, adjustments[1].IdempotencyKey)

	duplicate := OrganizationQuotaAdjustment{OrganizationId: 1, OperatorUserId: 2, QuotaDelta: 30, QuotaBefore: 130, QuotaAfter: 160, IdempotencyKey: adjustments[0].IdempotencyKey}
	require.Error(t, db.Create(&duplicate).Error)
}

func TestEnsureOrganizationBillingIndexes(t *testing.T) {
	setupModelTestDB(t)

	require.NoError(t, ensureOrganizationBillingIndexes())
	require.True(t, DB.Migrator().HasIndex(&Log{}, "idx_logs_org_billing_type_time"))
	require.True(t, DB.Migrator().HasIndex(&Log{}, "idx_logs_org_billing_responsible_time"))
	require.True(t, DB.Migrator().HasIndex(&Token{}, "idx_tokens_org_scope_responsible"))
	require.True(t, DB.Migrator().HasIndex(&OrganizationMember{}, "idx_org_members_org_status"))
}
