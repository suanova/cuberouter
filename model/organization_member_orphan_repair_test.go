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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createRepairTestUser(t *testing.T) User {
	t.Helper()
	user := User{
		Username: "repair-" + common.GetUUID(),
		Password: "password",
		AffCode:  common.GetUUID(),
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, DB.Create(&user).Error)
	return user
}

func TestOrganizationMemberOrphanRepairPurgesOnlyAccountlessRows(t *testing.T) {
	setupModelTestDB(t)

	live := createRepairTestUser(t)
	orphan := createRepairTestUser(t)
	softDeleted := createRepairTestUser(t)
	require.NoError(t, DB.Delete(&softDeleted).Error) // 软删：行仍在，id 不可复用
	var softDeletedRow User
	require.NoError(t, DB.Unscoped().Where("id = ?", softDeleted.Id).First(&softDeletedRow).Error)
	require.True(t, softDeletedRow.DeletedAt.Valid)
	require.NoError(t, DB.Unscoped().Delete(&User{Id: orphan.Id}).Error) // 硬删：账号真的不存在

	liveRow := OrganizationMember{OrganizationId: 1, UserId: live.Id, Role: OrganizationRoleMember, Status: OrganizationMemberStatusActive, CreatedAt: common.GetTimestamp()}
	orphanRow := OrganizationMember{OrganizationId: 1, UserId: orphan.Id, Role: OrganizationRoleMember, Status: OrganizationMemberStatusRemoved, CreatedAt: common.GetTimestamp()}
	softDeletedMemberRow := OrganizationMember{OrganizationId: 1, UserId: softDeleted.Id, Role: OrganizationRoleMember, Status: OrganizationMemberStatusActive, CreatedAt: common.GetTimestamp()}
	for _, row := range []*OrganizationMember{&liveRow, &orphanRow, &softDeletedMemberRow} {
		require.NoError(t, DB.Create(row).Error)
	}

	require.NoError(t, prepareOrganizationMemberOrphanRepair(DB))

	var storedLive, storedSoftDeleted OrganizationMember
	require.NoError(t, DB.First(&storedLive, liveRow.Id).Error, "账号还在，成员行不得被删")
	require.NoError(t, DB.First(&storedSoftDeleted, softDeletedMemberRow.Id).Error, "软删账号仍占用 id，其成员行不是孤儿")
	var orphanCount int64
	require.NoError(t, DB.Model(&OrganizationMember{}).Where("id = ?", orphanRow.Id).Count(&orphanCount).Error)
	assert.Zero(t, orphanCount, "账号不存在的成员行必须被清理")
}

// 幂等：第二次运行不得再删任何行。
func TestOrganizationMemberOrphanRepairIsIdempotent(t *testing.T) {
	setupModelTestDB(t)

	orphan := createRepairTestUser(t)
	require.NoError(t, DB.Unscoped().Delete(&User{Id: orphan.Id}).Error)
	row := OrganizationMember{OrganizationId: 1, UserId: orphan.Id, Role: OrganizationRoleMember, Status: OrganizationMemberStatusRemoved, CreatedAt: common.GetTimestamp()}
	require.NoError(t, DB.Create(&row).Error)

	require.NoError(t, prepareOrganizationMemberOrphanRepair(DB))
	require.NoError(t, prepareOrganizationMemberOrphanRepair(DB))

	var count int64
	require.NoError(t, DB.Model(&OrganizationMember{}).Where("id = ?", row.Id).Count(&count).Error)
	assert.Zero(t, count)
}

// 成员行 created_at 早于账号 created_at 只是“可疑”，不是“可证明”：多节点部署下
// 管理员在 B 节点给 A 节点创建的账号加成员，两个时间戳来自不同墙钟。这类行必须
// 原样保留（只记日志），否则会误删合法成员。访问路径不受影响这一条由
// service 包的 TestOrganizationMembershipWithOlderTimestampStillResolves 钉住
// （本包不能调 service.GetOrganizationForMember）。
func TestOrganizationMemberOrphanRepairKeepsMismatchedCreatedAt(t *testing.T) {
	setupModelTestDB(t)

	user := createRepairTestUser(t)
	now := common.GetTimestamp()
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("created_at", now).Error)
	row := OrganizationMember{OrganizationId: 1, UserId: user.Id, Role: OrganizationRoleMember, Status: OrganizationMemberStatusActive, CreatedAt: now - 3600}
	require.NoError(t, DB.Create(&row).Error)

	require.NoError(t, prepareOrganizationMemberOrphanRepair(DB))

	var stored OrganizationMember
	require.NoError(t, DB.First(&stored, row.Id).Error, "时钟偏差类行不得被自动删除")
}
