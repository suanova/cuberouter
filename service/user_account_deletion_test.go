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
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gorm.io/gorm"
)

func createDeletionTestOrganization(t *testing.T, status string, ownerUserId int) model.Organization {
	t.Helper()
	organization := model.Organization{
		Name:        "Del Org " + common.GetUUID(),
		Slug:        "del-org-" + common.GetUUID(),
		Status:      status,
		OwnerUserId: ownerUserId,
		CreatedBy:   ownerUserId,
	}
	require.NoError(t, model.DB.Create(&organization).Error)
	return organization
}

func addDeletionTestMember(t *testing.T, organizationId, userId int, role, status string) model.OrganizationMember {
	t.Helper()
	now := common.GetTimestamp()
	member := model.OrganizationMember{
		OrganizationId: organizationId, UserId: userId, Role: role, Status: status,
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, model.DB.Create(&member).Error)
	return member
}

// 普通成员的账号被删除：成员行与账号必须在同一事务里一起消失，并留下组织审计。
func TestDeleteUserAccountPurgesMembershipWithAccount(t *testing.T) {
	setupServiceTestDB(t)
	operator := createServiceTestUser(t, "del-op-"+common.GetUUID(), common.RoleRootUser)
	owner := createServiceTestUser(t, "del-owner-"+common.GetUUID(), common.RoleCommonUser)
	target := createServiceTestUser(t, "del-target-"+common.GetUUID(), common.RoleCommonUser)
	organization := createDeletionTestOrganization(t, model.OrganizationStatusActive, owner.Id)
	member := addDeletionTestMember(t, organization.Id, target.Id, model.OrganizationRoleMember, model.OrganizationMemberStatusActive)

	require.NoError(t, DeleteUserAccount(operator.Id, target.Id))

	var memberCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("id = ?", member.Id).Count(&memberCount).Error)
	assert.Zero(t, memberCount, "成员行必须随账号一起收口")

	var userCount int64
	require.NoError(t, model.DB.Unscoped().Model(&model.User{}).Where("id = ?", target.Id).Count(&userCount).Error)
	assert.Zero(t, userCount)

	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionMemberAccountDeleted).First(&audit).Error)
	assert.Equal(t, operator.Id, audit.OperatorUserId)
	assert.Equal(t, "account_deleted:"+strconv.Itoa(target.Id), audit.Reason)
	assert.Equal(t, member.Id, audit.TargetId)
}

// removed 墓碑同样随账号收口：留着它，下一个复用该 user id 的账号会被
// "already belongs" 判定挡住自动加入。
func TestDeleteUserAccountPurgesRemovedTombstone(t *testing.T) {
	setupServiceTestDB(t)
	operator := createServiceTestUser(t, "del-op-"+common.GetUUID(), common.RoleRootUser)
	owner := createServiceTestUser(t, "del-owner-"+common.GetUUID(), common.RoleCommonUser)
	target := createServiceTestUser(t, "del-target-"+common.GetUUID(), common.RoleCommonUser)
	organization := createDeletionTestOrganization(t, model.OrganizationStatusActive, owner.Id)
	addDeletionTestMember(t, organization.Id, target.Id, model.OrganizationRoleMember, model.OrganizationMemberStatusRemoved)

	require.NoError(t, DeleteUserAccount(operator.Id, target.Id))

	var count int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND user_id = ?", organization.Id, target.Id).Count(&count).Error)
	assert.Zero(t, count)

	// 收口之后，同一个邮箱再次注册时自动加入必须恢复（D2 回归）。
	require.NoError(t, model.DB.Create(&model.OrganizationJoinRule{
		OrganizationId: organization.Id, MatchType: model.OrganizationJoinRuleMatchTypeDomain,
		Pattern: "rejoin.example.com", PatternNormalized: "rejoin.example.com",
		CreatedBy: operator.Id, CreatedAt: common.GetTimestamp(), UpdatedAt: common.GetTimestamp(),
	}).Error)
	newcomer := createJoinTestUser(t, "rejoin-"+common.GetUUID()+"@rejoin.example.com")
	require.NoError(t, model.DB.Transaction(func(tx *gorm.DB) error {
		return JoinOrganizationByJoinRuleWithTx(tx, &newcomer, newcomer.Email)
	}))
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, newcomer.Id).First(&model.OrganizationMember{}).Error)
}

// 收口不区分状态：active / disabled / removed / exited 都是账号级关系的残留，
// 账号不存在时一律不成立（active 会变成"继承"，墓碑会变成"误判跳过"）。
func TestDeleteUserAccountPurgesEveryMembershipStatus(t *testing.T) {
	statuses := []string{
		model.OrganizationMemberStatusActive,
		model.OrganizationMemberStatusDisabled,
		model.OrganizationMemberStatusRemoved,
		model.OrganizationMemberStatusExited,
	}
	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			setupServiceTestDB(t)
			operator := createServiceTestUser(t, "del-op-"+common.GetUUID(), common.RoleRootUser)
			owner := createServiceTestUser(t, "del-owner-"+common.GetUUID(), common.RoleCommonUser)
			target := createServiceTestUser(t, "del-target-"+common.GetUUID(), common.RoleCommonUser)
			organization := createDeletionTestOrganization(t, model.OrganizationStatusActive, owner.Id)
			addDeletionTestMember(t, organization.Id, target.Id, model.OrganizationRoleMember, status)

			require.NoError(t, DeleteUserAccount(operator.Id, target.Id))

			var count int64
			require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND user_id = ?", organization.Id, target.Id).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

// 被拒绝时组织必须留痕：组织需要知道有人试图删除一个还挂着它 owner / key 的账号，
// 以及被什么挡住。这条审计写在回滚之后（独立事务），所以成员行还在。
func TestDeleteUserAccountRefusalIsAudited(t *testing.T) {
	setupServiceTestDB(t)
	operator := createServiceTestUser(t, "del-op-"+common.GetUUID(), common.RoleRootUser)
	target := createServiceTestUser(t, "del-owner-"+common.GetUUID(), common.RoleCommonUser)
	organization := createDeletionTestOrganization(t, model.OrganizationStatusActive, target.Id)
	addDeletionTestMember(t, organization.Id, target.Id, model.OrganizationRoleOwner, model.OrganizationMemberStatusActive)

	require.Error(t, DeleteUserAccount(operator.Id, target.Id))

	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionMemberKeyTransferBlocked).First(&audit).Error)
	assert.Contains(t, audit.AfterData, "active_owner")
	// 被什么挡住：blockers 与"这是一次账号删除"的语境都落在 AfterData（operation_reason），
	// reason 字段保持被拦截错误本身（与既有三处调用点一致）。
	assert.Contains(t, audit.AfterData, "account deletion blocked")
	assert.Contains(t, audit.Reason, organization.Name)
}

// 收口的锁序必须是 users → organizations → organization_members：注册路径是
// user → member（InsertWithTx 落库后插成员行），管理员加人同样先读 users 再写成员行，
// 收口若反过来先把成员行/组织行握在手里再碰 users，就会与它们形成反向锁序（spec §9）。
// SQLite 不下发 FOR UPDATE（单写者模型），因此锁断言在该方言上自动跳过，真正的
// 锁序断言由 user_account_deletion_concurrency_test.go 的 PostgreSQL 变体承担。
func TestDeleteUserAccountLocksUserRowBeforeOrganization(t *testing.T) {
	setupServiceTestDB(t)
	runDeleteUserAccountLockOrder(t, model.DB)
}

// 成员行 created_at 早于账号时，访问路径必须照旧放行：多节点部署下管理员跨节点加人
// 会让两个时间戳来自不同墙钟，据此拒绝会误伤合法成员（spec I6）。
func TestOrganizationMembershipWithOlderTimestampStillResolves(t *testing.T) {
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "skew-owner-"+common.GetUUID(), common.RoleCommonUser)
	member := createServiceTestUser(t, "skew-member-"+common.GetUUID(), common.RoleCommonUser)
	organization := createDeletionTestOrganization(t, model.OrganizationStatusActive, owner.Id)
	now := common.GetTimestamp()
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", member.Id).Update("created_at", now).Error)
	row := addDeletionTestMember(t, organization.Id, member.Id, model.OrganizationRoleMember, model.OrganizationMemberStatusActive)
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("id = ?", row.Id).Update("created_at", now-3600).Error)

	resolvedOrganization, resolvedMember, err := GetOrganizationForMember(member.Id, organization.Id, false)
	require.NoError(t, err)
	assert.Equal(t, organization.Id, resolvedOrganization.Id)
	assert.Equal(t, model.OrganizationMemberStatusActive, resolvedMember.Status)
}

// 组织 owner 不能靠"删账号"绕过唯一 owner 不变式。
func TestDeleteUserAccountRefusesOrganizationOwner(t *testing.T) {
	setupServiceTestDB(t)
	operator := createServiceTestUser(t, "del-op-"+common.GetUUID(), common.RoleRootUser)
	target := createServiceTestUser(t, "del-owner-"+common.GetUUID(), common.RoleCommonUser)
	organization := createDeletionTestOrganization(t, model.OrganizationStatusActive, target.Id)
	member := addDeletionTestMember(t, organization.Id, target.Id, model.OrganizationRoleOwner, model.OrganizationMemberStatusActive)

	err := DeleteUserAccount(operator.Id, target.Id)
	require.Error(t, err)
	var blocked *OrganizationOperationBlockedError
	require.ErrorAs(t, err, &blocked)
	assert.Contains(t, blocked.Message, organization.Name, "拒绝必须点名组织")

	var memberCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("id = ?", member.Id).Count(&memberCount).Error)
	assert.Equal(t, int64(1), memberCount, "拒绝时不得动成员行")
	var userCount int64
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", target.Id).Count(&userCount).Error)
	assert.Equal(t, int64(1), userCount, "拒绝时不得动账号")
}

// 组织已解散：owner 也不再是活概念，必须允许删除，否则"曾拥有过已解散组织"的人
// 永远删不掉。
func TestDeleteUserAccountPurgesDissolvedOrganizationMembership(t *testing.T) {
	setupServiceTestDB(t)
	operator := createServiceTestUser(t, "del-op-"+common.GetUUID(), common.RoleRootUser)
	target := createServiceTestUser(t, "del-target-"+common.GetUUID(), common.RoleCommonUser)
	organization := createDeletionTestOrganization(t, model.OrganizationStatusDissolved, target.Id)
	addDeletionTestMember(t, organization.Id, target.Id, model.OrganizationRoleOwner, model.OrganizationMemberStatusActive)

	require.NoError(t, DeleteUserAccount(operator.Id, target.Id))

	var count int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND user_id = ?", organization.Id, target.Id).Count(&count).Error)
	assert.Zero(t, count)
}

// 持有组织 key（无论 public 与否）都必须拒绝：删账号会按 user_id 静默删掉这些 key，
// 绕过 key 转移与组织审计。
func TestDeleteUserAccountRefusesOrganizationKeyHolder(t *testing.T) {
	cases := []struct {
		name          string
		visibility    string
		ownerIsTarget bool
	}{
		{name: "private token owned by the target", visibility: model.TokenVisibilityPrivate, ownerIsTarget: true},
		{name: "private token whose responsible user is the target", visibility: model.TokenVisibilityPrivate, ownerIsTarget: false},
		{name: "public token whose responsible user is the target", visibility: model.TokenVisibilityPublic, ownerIsTarget: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupServiceTestDB(t)
			operator := createServiceTestUser(t, "del-op-"+common.GetUUID(), common.RoleRootUser)
			owner := createServiceTestUser(t, "del-owner-"+common.GetUUID(), common.RoleCommonUser)
			target := createServiceTestUser(t, "del-target-"+common.GetUUID(), common.RoleCommonUser)
			organization := createDeletionTestOrganization(t, model.OrganizationStatusActive, owner.Id)
			addDeletionTestMember(t, organization.Id, target.Id, model.OrganizationRoleMember, model.OrganizationMemberStatusActive)
			addDeletionTestMember(t, organization.Id, owner.Id, model.OrganizationRoleOwner, model.OrganizationMemberStatusActive)

			tokenUserId := owner.Id
			if tc.ownerIsTarget {
				tokenUserId = target.Id
			}
			require.NoError(t, model.DB.Create(&model.Token{
				UserId: tokenUserId, ResponsibleUserId: target.Id, CreatorUserId: owner.Id,
				OrganizationId: organization.Id, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id,
				Visibility: tc.visibility, Key: "sk-org-key-" + common.GetUUID(), Name: "org key",
				Status: common.TokenStatusEnabled, CreatedTime: common.GetTimestamp(),
			}).Error)

			err := DeleteUserAccount(operator.Id, target.Id)
			require.Error(t, err)
			var blocked *OrganizationOperationBlockedError
			require.ErrorAs(t, err, &blocked)
			assert.Contains(t, blocked.Message, organization.Name)

			var tokenCount int64
			require.NoError(t, model.DB.Model(&model.Token{}).Where("organization_id = ?", organization.Id).Count(&tokenCount).Error)
			assert.Equal(t, int64(1), tokenCount, "拒绝时不得删 key")
		})
	}
}

// 多组织：任一组织拒绝则整请求失败，任何组织都不得被收口（I4）。
func TestDeleteUserAccountIsAtomicAcrossOrganizations(t *testing.T) {
	setupServiceTestDB(t)
	operator := createServiceTestUser(t, "del-op-"+common.GetUUID(), common.RoleRootUser)
	ownerA := createServiceTestUser(t, "del-owner-a-"+common.GetUUID(), common.RoleCommonUser)
	target := createServiceTestUser(t, "del-target-"+common.GetUUID(), common.RoleCommonUser)

	plain := createDeletionTestOrganization(t, model.OrganizationStatusActive, ownerA.Id)
	owned := createDeletionTestOrganization(t, model.OrganizationStatusActive, target.Id)
	memberA := addDeletionTestMember(t, plain.Id, target.Id, model.OrganizationRoleMember, model.OrganizationMemberStatusActive)
	memberB := addDeletionTestMember(t, owned.Id, target.Id, model.OrganizationRoleOwner, model.OrganizationMemberStatusActive)

	err := DeleteUserAccount(operator.Id, target.Id)
	require.Error(t, err)
	var blocked *OrganizationOperationBlockedError
	require.ErrorAs(t, err, &blocked)
	assert.Contains(t, blocked.Message, owned.Name)

	for _, memberId := range []int{memberA.Id, memberB.Id} {
		var count int64
		require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("id = ?", memberId).Count(&count).Error)
		assert.Equal(t, int64(1), count, "整请求失败时不允许留下半收口状态")
	}
	var userCount int64
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", target.Id).Count(&userCount).Error)
	assert.Equal(t, int64(1), userCount)
}

// 拒绝路径不得解除该成员的 blocker 行：成员还持着组织 key，收口被整请求拒绝，
// 账号、成员行与 blocker 原样留库，由组织先移交 key 再删。
func TestDeleteUserAccountRefusalKeepsMemberTokenBlockers(t *testing.T) {
	setupServiceTestDB(t)
	operator := createServiceTestUser(t, "del-op-"+common.GetUUID(), common.RoleRootUser)
	owner := createServiceTestUser(t, "del-owner-"+common.GetUUID(), common.RoleCommonUser)
	target := createServiceTestUser(t, "del-target-"+common.GetUUID(), common.RoleCommonUser)
	organization := createDeletionTestOrganization(t, model.OrganizationStatusActive, owner.Id)
	addDeletionTestMember(t, organization.Id, target.Id, model.OrganizationRoleMember, model.OrganizationMemberStatusDisabled)

	token := model.Token{
		UserId: target.Id, OrganizationId: organization.Id, ScopeType: model.TokenScopeOrganization,
		ScopeId: organization.Id, Visibility: model.TokenVisibilityPrivate,
		Key: "sk-blocked-" + common.GetUUID(), Name: "blocked key", Status: common.TokenStatusEnabled,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(&token).Error)
	blocker := model.OrganizationTokenSystemBlocker{
		TokenId: token.Id, OrganizationId: organization.Id, Reason: organizationTokenBlockerReasonMemberDisabled,
		RefType: organizationTokenBlockerRefTypeMember, RefId: target.Id,
		Status: model.OrganizationTokenBlockerStatusActive,
	}
	require.NoError(t, model.DB.Create(&blocker).Error)

	// 该成员持 key，删除会被拒绝——这正是期望：blocker 与 key 一起留着，由组织先移交。
	err := DeleteUserAccount(operator.Id, target.Id)
	require.Error(t, err)

	var stored model.OrganizationTokenSystemBlocker
	require.NoError(t, model.DB.First(&stored, blocker.Id).Error)
	assert.Equal(t, model.OrganizationTokenBlockerStatusActive, stored.Status, "拒绝路径不得解除 blocker")
}
