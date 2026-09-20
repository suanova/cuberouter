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
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// legacyOrganizationMemberWithoutDisabledSource 是 disabled_source 列之前的表结构，
// 与 OrganizationMember 列集一致但缺少 DisabledSource。
type legacyOrganizationMemberWithoutDisabledSource struct {
	Id             int    `gorm:"primaryKey"`
	OrganizationId int    `gorm:"uniqueIndex:idx_org_user;index;not null"`
	UserId         int    `gorm:"uniqueIndex:idx_org_user;index;not null"`
	Role           string `gorm:"type:varchar(16);not null;index"`
	Status         string `gorm:"type:varchar(16);not null;index;default:'active'"`
	InvitedBy      int    `gorm:"index;default:0"`
	JoinedAt       int64  `gorm:"bigint;index;default:0"`
	LastActiveAt   int64  `gorm:"bigint;index;default:0"`
	CreatedAt      int64  `gorm:"bigint;index"`
	UpdatedAt      int64  `gorm:"bigint;index"`
	DisabledAt     int64  `gorm:"bigint;default:0"`
	ExitedAt       int64  `gorm:"bigint;default:0"`
	RemovedAt      int64  `gorm:"bigint;default:0"`
}

func (legacyOrganizationMemberWithoutDisabledSource) TableName() string {
	return "organization_members"
}

func TestPrepareOrganizationMemberDisableSourceMigrationBackfillsAuditAuthority(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/member-source.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacyOrganizationMemberWithoutDisabledSource{}, &OrganizationAuditLog{}))
	members := []legacyOrganizationMemberWithoutDisabledSource{
		{OrganizationId: 1, UserId: 11, Role: OrganizationRoleMember, Status: OrganizationMemberStatusDisabled},
		{OrganizationId: 1, UserId: 12, Role: OrganizationRoleMember, Status: OrganizationMemberStatusDisabled},
		{OrganizationId: 1, UserId: 13, Role: OrganizationRoleMember, Status: OrganizationMemberStatusDisabled},
		{OrganizationId: 1, UserId: 14, Role: OrganizationRoleMember, Status: OrganizationMemberStatusDisabled},
		{OrganizationId: 1, UserId: 15, Role: OrganizationRoleMember, Status: OrganizationMemberStatusActive},
	}
	require.NoError(t, db.Create(&members).Error)

	snapshot := func(status string) string {
		data, marshalErr := common.Marshal(map[string]any{"status": status})
		require.NoError(t, marshalErr)
		return string(data)
	}
	logs := []OrganizationAuditLog{
		// 组织 owner 停用：归属组织。
		{OrganizationId: 1, OperatorUserId: 1, OperatorRole: OrganizationRoleOwner, ActionType: "organization.member.update", TargetType: "member", TargetId: members[0].Id, BeforeData: snapshot(OrganizationMemberStatusActive), AfterData: snapshot(OrganizationMemberStatusDisabled), CreatedAt: 1},
		// 平台管理员停用：归属平台。
		{OrganizationId: 1, OperatorUserId: 2, OperatorRole: "platform_admin", ActionType: "organization.member.update", TargetType: "member", TargetId: members[1].Id, BeforeData: snapshot(OrganizationMemberStatusActive), AfterData: snapshot(OrganizationMemberStatusDisabled), CreatedAt: 2},
		// 平台停用 → 平台恢复 → 组织停用：最后一次停用来自组织。
		{OrganizationId: 1, OperatorUserId: 2, OperatorRole: "platform_admin", ActionType: "organization.member.update", TargetType: "member", TargetId: members[2].Id, BeforeData: snapshot(OrganizationMemberStatusActive), AfterData: snapshot(OrganizationMemberStatusDisabled), CreatedAt: 3},
		{OrganizationId: 1, OperatorUserId: 2, OperatorRole: "platform_admin", ActionType: "organization.member.update", TargetType: "member", TargetId: members[2].Id, BeforeData: snapshot(OrganizationMemberStatusDisabled), AfterData: snapshot(OrganizationMemberStatusActive), CreatedAt: 4},
		{OrganizationId: 1, OperatorUserId: 1, OperatorRole: OrganizationRoleOwner, ActionType: "organization.member.update", TargetType: "member", TargetId: members[2].Id, BeforeData: snapshot(OrganizationMemberStatusActive), AfterData: snapshot(OrganizationMemberStatusDisabled), CreatedAt: 5},
		// members[3] 没有任何审计记录：唯一安全的归因是组织。
	}
	require.NoError(t, db.Create(&logs).Error)

	require.NoError(t, prepareOrganizationMemberDisableSourceMigration(db))
	require.NoError(t, prepareOrganizationMemberDisableSourceMigration(db), "migration must be safe to resume")

	var stored []OrganizationMember
	require.NoError(t, db.Order("id asc").Find(&stored).Error)
	require.Len(t, stored, len(members))
	assert.Equal(t, OrganizationMemberDisableSourceOrganization, stored[0].DisabledSource)
	assert.Equal(t, OrganizationMemberDisableSourcePlatform, stored[1].DisabledSource)
	assert.Equal(t, OrganizationMemberDisableSourceOrganization, stored[2].DisabledSource)
	assert.Equal(t, OrganizationMemberDisableSourceOrganization, stored[3].DisabledSource)
	assert.Empty(t, stored[4].DisabledSource, "an active member must not be attributed a disable source")
}

// 重复的停用记录（自身 before 已是停用）通常来自幂等重放，不能把先前的平台归因改写成组织归因。
func TestOrganizationMemberDisableSourceIgnoresRedundantDisableRecords(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/member-platform.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacyOrganizationMemberWithoutDisabledSource{}, &OrganizationAuditLog{}))
	member := legacyOrganizationMemberWithoutDisabledSource{
		OrganizationId: 1, UserId: 21, Role: OrganizationRoleMember, Status: OrganizationMemberStatusDisabled,
	}
	require.NoError(t, db.Create(&member).Error)

	snapshot := func(status string) string {
		data, marshalErr := common.Marshal(map[string]any{"status": status})
		require.NoError(t, marshalErr)
		return string(data)
	}
	logs := []OrganizationAuditLog{
		{OrganizationId: 1, OperatorUserId: 2, OperatorRole: "platform_admin", ActionType: "organization.member.update", TargetType: "member", TargetId: member.Id, BeforeData: snapshot(OrganizationMemberStatusActive), AfterData: snapshot(OrganizationMemberStatusDisabled), CreatedAt: 1},
		{OrganizationId: 1, OperatorUserId: 1, OperatorRole: OrganizationRoleOwner, ActionType: "organization.member.update", TargetType: "member", TargetId: member.Id, BeforeData: snapshot(OrganizationMemberStatusDisabled), AfterData: snapshot(OrganizationMemberStatusDisabled), CreatedAt: 2},
	}
	require.NoError(t, db.Create(&logs).Error)

	require.NoError(t, prepareOrganizationMemberDisableSourceMigration(db))

	var stored OrganizationMember
	require.NoError(t, db.First(&stored, member.Id).Error)
	assert.Equal(t, OrganizationMemberDisableSourcePlatform, stored.DisabledSource)
}

// 已经带 disabled_source 的行不得被重写，否则运维的显式归因会被迁移覆盖。
func TestPrepareOrganizationMemberDisableSourceMigrationLeavesAttributedRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/member-attributed.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&OrganizationMember{}, &OrganizationAuditLog{}))
	member := OrganizationMember{
		OrganizationId: 1, UserId: 31, Role: OrganizationRoleMember,
		Status: OrganizationMemberStatusDisabled, DisabledSource: OrganizationMemberDisableSourcePlatform,
	}
	require.NoError(t, db.Create(&member).Error)
	require.NoError(t, db.Create(&OrganizationAuditLog{
		OrganizationId: 1, OperatorUserId: 1, OperatorRole: OrganizationRoleOwner,
		ActionType: "organization.member.update", TargetType: "member", TargetId: member.Id,
		AfterData: `{"status":"disabled"}`, CreatedAt: 1,
	}).Error)

	require.NoError(t, prepareOrganizationMemberDisableSourceMigration(db))

	var stored OrganizationMember
	require.NoError(t, db.First(&stored, member.Id).Error)
	assert.Equal(t, OrganizationMemberDisableSourcePlatform, stored.DisabledSource)
}
