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
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// createMemberQuotaTestOrg 建一个 owner（组织创建者）+ admin + member 三档成员的组织。
// owner 的 owner 角色由 organization.owner_user_id 决定，不靠成员行里的 role 值。
func createMemberQuotaTestOrg(t *testing.T) (model.User, model.User, model.User, *model.Organization) {
	t.Helper()
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "member-quota-owner", common.RoleCommonUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Member Quota Org"})
	require.NoError(t, err)
	admin := createServiceTestUser(t, "member-quota-admin", common.RoleCommonUser)
	member := createServiceTestUser(t, "member-quota-member", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	return owner, admin, member, organization
}

// setMemberQuotaTestOrganizationQuota 模拟平台调整组织总额度：配置动作读的是库里的值。
func setMemberQuotaTestOrganizationQuota(t *testing.T, organization *model.Organization, quota int) {
	t.Helper()
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("quota", quota).Error)
	organization.Quota = quota
}

// createMemberQuotaTestMember 建一个带配额与已用值的成员行，状态由调用方指定。
func createMemberQuotaTestMember(t *testing.T, organizationId int, username string, status string, quotaLimit int, usedQuota int) model.User {
	t.Helper()
	user := createServiceTestUser(t, username, common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: organizationId,
		UserId:         user.Id,
		Role:           model.OrganizationRoleMember,
		Status:         status,
		QuotaLimit:     quotaLimit,
		UsedQuota:      usedQuota,
	}).Error)
	return user
}

// storedMemberQuota 读成员行 quota_limit / used_quota 列的原值。
func storedMemberQuota(t *testing.T, organizationId, userId int) (int, int) {
	t.Helper()
	var stored model.OrganizationMember
	require.NoError(t, model.DB.Select("id", "quota_limit", "used_quota").Where("organization_id = ? AND user_id = ?", organizationId, userId).First(&stored).Error)
	return stored.QuotaLimit, stored.UsedQuota
}

func TestOrganizationMemberQuotaCapabilityMapping(t *testing.T) {
	owner, admin, member, organization := createMemberQuotaTestOrg(t)

	ownerActor, err := GetOrganizationActorContextForAccessMode(owner.Id, organization.Id, OrganizationAccessModeManagement, true)
	require.NoError(t, err)
	require.True(t, ownerActor.Capabilities.CanManageMemberQuotas)

	adminActor, err := GetOrganizationActorContextForAccessMode(admin.Id, organization.Id, OrganizationAccessModeManagement, true)
	require.NoError(t, err)
	require.True(t, adminActor.Capabilities.CanManageMemberQuotas)

	memberActor, err := GetOrganizationActorContextForAccessMode(member.Id, organization.Id, OrganizationAccessModeWorkspace, true)
	require.NoError(t, err)
	require.False(t, memberActor.Capabilities.CanManageMemberQuotas)
}

func TestOrganizationMemberQuotaGetDefault(t *testing.T) {
	owner, _, member, organization := createMemberQuotaTestOrg(t)

	limit, used, organizationQuota, allocated, overAllocated, err := GetOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id)
	require.NoError(t, err)
	require.Zero(t, limit)
	require.Zero(t, used)
	require.Equal(t, organization.Quota, organizationQuota)
	require.Zero(t, allocated)
	require.False(t, overAllocated)
}

func TestOrganizationMemberQuotaUpdateSetAndClear(t *testing.T) {
	owner, admin, member, organization := createMemberQuotaTestOrg(t)
	setMemberQuotaTestOrganizationQuota(t, organization, 300)
	memberRow := memberRowId(t, organization.Id, member.Id)

	require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, 100))

	limit, used, organizationQuota, allocated, overAllocated, err := GetOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id)
	require.NoError(t, err)
	require.Equal(t, 100, limit)
	require.Zero(t, used)
	require.Equal(t, 300, organizationQuota)
	require.Equal(t, 100, allocated)
	require.False(t, overAllocated)

	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND target_id = ? AND action_type = ?", organization.Id, memberRow, organizationAuditActionMemberQuotaUpdate).First(&audit).Error)
	require.Equal(t, "member", audit.TargetType)
	require.Equal(t, member.Username, audit.TargetName)
	require.Equal(t, owner.Id, audit.OperatorUserId)
	require.Equal(t, model.OrganizationRoleOwner, audit.OperatorRole)
	require.Contains(t, audit.Reason, "cumulative organization consumption cap")
	require.Contains(t, audit.Reason, "zero means no cap")
	require.Equal(t, `{"limit":0,"used":0}`, audit.BeforeData)
	require.Equal(t, `{"limit":100,"used":0}`, audit.AfterData)

	// 已用值属于计费口径，配置动作不能重置它。
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("id = ?", memberRow).Update("used_quota", 42).Error)

	// 管理员也可以清空限制：0 = 不限制。
	require.NoError(t, UpdateOrganizationMemberQuota(admin.Id, organization.Id, OrganizationAccessModeManagement, member.Id, 0))

	limit, used, _, allocated, overAllocated, err = GetOrganizationMemberQuota(admin.Id, organization.Id, OrganizationAccessModeManagement, member.Id)
	require.NoError(t, err)
	require.Zero(t, limit)
	require.Equal(t, 42, used)
	require.Zero(t, allocated)
	require.False(t, overAllocated)

	var cleared model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND target_id = ? AND action_type = ?", organization.Id, memberRow, organizationAuditActionMemberQuotaUpdate).Order("id desc").First(&cleared).Error)
	require.Equal(t, model.OrganizationRoleAdmin, cleared.OperatorRole)
	require.Equal(t, `{"limit":100,"used":42}`, cleared.BeforeData)
	require.Equal(t, `{"limit":0,"used":42}`, cleared.AfterData)

	storedLimit, storedUsed := storedMemberQuota(t, organization.Id, member.Id)
	require.Zero(t, storedLimit)
	require.Equal(t, 42, storedUsed)
}

func TestOrganizationMemberQuotaValidateBudget(t *testing.T) {
	tests := []struct {
		name              string
		currentSum        int
		currentLimit      int
		newLimit          int
		organizationQuota int
		wantErr           bool
	}{
		{name: "increase within budget", currentSum: 200, currentLimit: 0, newLimit: 50, organizationQuota: 300},
		{name: "increase exactly to budget", currentSum: 250, currentLimit: 0, newLimit: 50, organizationQuota: 300},
		{name: "increase one above budget", currentSum: 250, currentLimit: 0, newLimit: 51, organizationQuota: 300, wantErr: true},
		{name: "replacing own limit at budget", currentSum: 300, currentLimit: 100, newLimit: 100, organizationQuota: 300},
		{name: "lowering frees budget", currentSum: 300, currentLimit: 100, newLimit: 50, organizationQuota: 300},
		{name: "over-allocated unchanged limit", currentSum: 350, currentLimit: 100, newLimit: 100, organizationQuota: 300},
		{name: "over-allocated increase rejected", currentSum: 350, currentLimit: 100, newLimit: 101, organizationQuota: 300, wantErr: true},
		{name: "over-allocated lowering allowed", currentSum: 350, currentLimit: 100, newLimit: 50, organizationQuota: 300},
		{name: "over-allocated clear always allowed", currentSum: 350, currentLimit: 100, newLimit: 0, organizationQuota: 300},
		{name: "clear within budget allowed", currentSum: 100, currentLimit: 100, newLimit: 0, organizationQuota: 300},
		{name: "zero organization quota blocks first allocation", currentSum: 0, currentLimit: 0, newLimit: 1, organizationQuota: 0, wantErr: true},
		{name: "zero organization quota clear allowed", currentSum: 0, currentLimit: 0, newLimit: 0, organizationQuota: 0},
		{name: "zero-limit members are not exempt from the budget", currentSum: 300, currentLimit: 0, newLimit: 1, organizationQuota: 300, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMemberQuotaBudget(tc.currentSum, tc.currentLimit, tc.newLimit, tc.organizationQuota)
			if tc.wantErr {
				require.ErrorContains(t, err, "budget")
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestOrganizationMemberQuotaBudgetSum(t *testing.T) {
	// 组织总额度 300；active 100 + active 100 + disabled 50 = 250 已分配。
	createBudgetFixture := func(t *testing.T, organization *model.Organization) (model.User, model.User, model.User) {
		t.Helper()
		memberA := createMemberQuotaTestMember(t, organization.Id, "member-quota-budget-a", model.OrganizationMemberStatusActive, 100, 0)
		memberB := createMemberQuotaTestMember(t, organization.Id, "member-quota-budget-b", model.OrganizationMemberStatusActive, 100, 0)
		memberC := createMemberQuotaTestMember(t, organization.Id, "member-quota-budget-c", model.OrganizationMemberStatusDisabled, 50, 0)
		return memberA, memberB, memberC
	}

	t.Run("fills the remaining budget up to organization quota", func(t *testing.T) {
		owner, _, _, organization := createMemberQuotaTestOrg(t)
		setMemberQuotaTestOrganizationQuota(t, organization, 300)
		createBudgetFixture(t, organization)
		target := createMemberQuotaTestMember(t, organization.Id, "member-quota-budget-target", model.OrganizationMemberStatusActive, 0, 0)

		require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id, 50))

		limit, _, _, allocated, overAllocated, err := GetOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id)
		require.NoError(t, err)
		require.Equal(t, 50, limit)
		require.Equal(t, 300, allocated)
		require.False(t, overAllocated)
	})

	t.Run("rejects allocation above organization quota", func(t *testing.T) {
		owner, _, _, organization := createMemberQuotaTestOrg(t)
		setMemberQuotaTestOrganizationQuota(t, organization, 300)
		createBudgetFixture(t, organization)
		target := createMemberQuotaTestMember(t, organization.Id, "member-quota-budget-target", model.OrganizationMemberStatusActive, 0, 0)

		require.ErrorContains(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id, 51), "budget")
		storedLimit, _ := storedMemberQuota(t, organization.Id, target.Id)
		require.Zero(t, storedLimit)
		require.Zero(t, countOrganizationAuditRows(t, organization.Id, organizationAuditActionMemberQuotaUpdate))
	})

	t.Run("zero limit members do not consume budget", func(t *testing.T) {
		owner, _, _, organization := createMemberQuotaTestOrg(t)
		setMemberQuotaTestOrganizationQuota(t, organization, 300)
		createBudgetFixture(t, organization)
		createMemberQuotaTestMember(t, organization.Id, "member-quota-budget-unlimited", model.OrganizationMemberStatusActive, 0, 0)
		target := createMemberQuotaTestMember(t, organization.Id, "member-quota-budget-target", model.OrganizationMemberStatusActive, 0, 0)

		require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id, 50))
		_, _, _, allocated, _, err := GetOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id)
		require.NoError(t, err)
		require.Equal(t, 300, allocated)
	})

	t.Run("lowering an existing limit frees budget", func(t *testing.T) {
		owner, _, _, organization := createMemberQuotaTestOrg(t)
		setMemberQuotaTestOrganizationQuota(t, organization, 300)
		memberA, _, _ := createBudgetFixture(t, organization)
		target := createMemberQuotaTestMember(t, organization.Id, "member-quota-budget-target", model.OrganizationMemberStatusActive, 0, 0)

		require.ErrorContains(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id, 51), "budget")
		require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, memberA.Id, 49))
		require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id, 101))
	})

	t.Run("clearing an existing limit frees budget", func(t *testing.T) {
		owner, _, _, organization := createMemberQuotaTestOrg(t)
		setMemberQuotaTestOrganizationQuota(t, organization, 300)
		memberA, _, _ := createBudgetFixture(t, organization)
		target := createMemberQuotaTestMember(t, organization.Id, "member-quota-budget-target", model.OrganizationMemberStatusActive, 0, 0)

		require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, memberA.Id, 0))
		require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id, 150))
	})
}

func TestOrganizationMemberQuotaOverAllocated(t *testing.T) {
	owner, _, _, organization := createMemberQuotaTestOrg(t)
	setMemberQuotaTestOrganizationQuota(t, organization, 200)
	memberA := createMemberQuotaTestMember(t, organization.Id, "member-quota-over-a", model.OrganizationMemberStatusActive, 100, 0)
	memberB := createMemberQuotaTestMember(t, organization.Id, "member-quota-over-b", model.OrganizationMemberStatusActive, 100, 0)
	memberC := createMemberQuotaTestMember(t, organization.Id, "member-quota-over-c", model.OrganizationMemberStatusDisabled, 50, 0)
	target := createMemberQuotaTestMember(t, organization.Id, "member-quota-over-target", model.OrganizationMemberStatusActive, 0, 0)

	_, _, organizationQuota, allocated, overAllocated, err := GetOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id)
	require.NoError(t, err)
	require.Equal(t, 200, organizationQuota)
	require.Equal(t, 250, allocated)
	require.True(t, overAllocated)

	// 已超额：任何增加总和的修改都拒绝，降低/清除/不变都允许。
	require.ErrorContains(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id, 1), "budget")
	require.ErrorContains(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, memberA.Id, 101), "budget")
	require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, memberA.Id, 50))
	require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, memberB.Id, 0))
	require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, memberC.Id, 50))
}

func TestOrganizationMemberQuotaBudgetExclusions(t *testing.T) {
	owner, _, _, organization := createMemberQuotaTestOrg(t)
	setMemberQuotaTestOrganizationQuota(t, organization, 300)

	// owner 存量配额是历史残留（例如被降级前写入）：不占预算，也不可再编辑。
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND user_id = ?", organization.Id, owner.Id).Update("quota_limit", 300).Error)
	createMemberQuotaTestMember(t, organization.Id, "member-quota-exclusion-exited", model.OrganizationMemberStatusExited, 100, 0)
	createMemberQuotaTestMember(t, organization.Id, "member-quota-exclusion-removed", model.OrganizationMemberStatusRemoved, 100, 0)
	createMemberQuotaTestMember(t, organization.Id, "member-quota-exclusion-a", model.OrganizationMemberStatusActive, 100, 0)
	createMemberQuotaTestMember(t, organization.Id, "member-quota-exclusion-b", model.OrganizationMemberStatusActive, 100, 0)
	target := createMemberQuotaTestMember(t, organization.Id, "member-quota-exclusion-target", model.OrganizationMemberStatusActive, 0, 0)

	// 只有 active + active 占预算：100 + 100 = 200。
	_, _, _, allocated, overAllocated, err := GetOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id)
	require.NoError(t, err)
	require.Equal(t, 200, allocated)
	require.False(t, overAllocated)

	require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id, 100))
	require.ErrorContains(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id, 101), "budget")

	require.ErrorContains(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, owner.Id, 10), "organization member operation forbidden")
	ownerLimit, _ := storedMemberQuota(t, organization.Id, owner.Id)
	require.Equal(t, 300, ownerLimit)
}

// TestOrganizationMemberQuotaOwnerTransferSwapsExemption 覆盖 owner 豁免的另一半：
// 上限跟着成员行走，豁免跟着 organization.owner_user_id 动态走。转交后，新 owner
// 的存量上限退出预算并停止拦截准入，旧 owner 的存量上限立刻生效并重新占用预算。
func TestOrganizationMemberQuotaOwnerTransferSwapsExemption(t *testing.T) {
	owner, _, member, organization := createMemberQuotaTestOrg(t)
	setMemberQuotaTestOrganizationQuota(t, organization, 300)

	// A（owner）行上的 100 是在位期间的历史残留：不计入预算，也不拦截准入。
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", organization.Id, owner.Id).
		Update("quota_limit", 100).Error)
	// B 在 A 在位时配置 150，预算是 150 而不是 250。
	require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, 150))
	_, _, _, allocated, overAllocated, err := GetOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id)
	require.NoError(t, err)
	require.Equal(t, 150, allocated)
	require.False(t, overAllocated)

	// 转交后角色互换：B 成为 owner，A 降为 admin 且保留自己的 100。
	require.NoError(t, TransferOrganizationOwner(owner.Id, organization.Id, OrganizationAccessModeManagement, TransferOrganizationOwnerRequest{
		OwnerUserId:    member.Id,
		Reason:         "handoff",
		IdempotencyKey: testOrganizationIdempotencyKey(t, "member-quota-owner-transfer"),
	}))
	var transferred model.Organization
	require.NoError(t, model.DB.First(&transferred, organization.Id).Error)
	require.Equal(t, member.Id, transferred.OwnerUserId)

	// 预算随 owner 身份翻转：B 的 150 退出，A 的 100 重新计入。
	limit, _, _, allocated, overAllocated, err := GetOrganizationMemberQuota(member.Id, organization.Id, OrganizationAccessModeManagement, owner.Id)
	require.NoError(t, err)
	require.Equal(t, 100, limit)
	require.Equal(t, 100, allocated)
	require.False(t, overAllocated)

	// 新 owner B 豁免准入：一次远超其存量上限的严格计费仍通过，并如实累计。
	require.NoError(t, runOrganizationBillingTransaction(func(tx *gorm.DB) error {
		return updateOrganizationMemberUsedQuotaTx(tx, organization.Id, member.Id, transferred.OwnerUserId, 200, true)
	}))
	_, memberUsed := storedMemberQuota(t, organization.Id, member.Id)
	require.Equal(t, 200, memberUsed)

	// 旧 owner A 的上限立即生效：越过 100 的严格计费被拒，事务回滚不留计数。
	err = runOrganizationBillingTransaction(func(tx *gorm.DB) error {
		return updateOrganizationMemberUsedQuotaTx(tx, organization.Id, owner.Id, transferred.OwnerUserId, 101, true)
	})
	require.ErrorIs(t, err, ErrOrganizationMemberQuotaExceeded)
	_, ownerUsed := storedMemberQuota(t, organization.Id, owner.Id)
	require.Zero(t, ownerUsed)
}

func TestOrganizationMemberQuotaReactivationOverAllocates(t *testing.T) {
	owner, _, _, organization := createMemberQuotaTestOrg(t)
	platformAdmin := createServiceTestUser(t, "member-quota-platform-admin", common.RoleAdminUser)
	setMemberQuotaTestOrganizationQuota(t, organization, 300)
	memberA := createMemberQuotaTestMember(t, organization.Id, "member-quota-reactivation-a", model.OrganizationMemberStatusActive, 100, 0)
	createMemberQuotaTestMember(t, organization.Id, "member-quota-reactivation-b", model.OrganizationMemberStatusActive, 100, 0)
	createMemberQuotaTestMember(t, organization.Id, "member-quota-reactivation-c", model.OrganizationMemberStatusActive, 100, 0)
	exited := createMemberQuotaTestMember(t, organization.Id, "member-quota-reactivation-exited", model.OrganizationMemberStatusExited, 50, 0)

	// 平台把总额度调低到低于已分配之和：允许，读取显示超额分配。
	setMemberQuotaTestOrganizationQuota(t, organization, 250)
	_, _, organizationQuota, allocated, overAllocated, err := GetOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, memberA.Id)
	require.NoError(t, err)
	require.Equal(t, 250, organizationQuota)
	require.Equal(t, 300, allocated)
	require.True(t, overAllocated)

	// 重新加入不是配置动作，不校验预算：exited 行恢复后其原配额重新计入，超额分配扩大。
	_, err = AddOrganizationMember(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, AddMemberRequest{UserId: exited.Id, Role: model.OrganizationRoleMember, Reason: "re-add"})
	require.NoError(t, err)
	reactivatedLimit, _ := storedMemberQuota(t, organization.Id, exited.Id)
	require.Equal(t, 50, reactivatedLimit)

	_, _, _, allocated, overAllocated, err = GetOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, memberA.Id)
	require.NoError(t, err)
	require.Equal(t, 350, allocated)
	require.True(t, overAllocated)

	// 之后的配置动作按「不增加总和」校验：任何增加都拒绝，降低仍允许。
	require.ErrorContains(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, memberA.Id, 101), "budget")
	require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, memberA.Id, 50))
}

func TestOrganizationMemberQuotaTargetStatuses(t *testing.T) {
	owner, admin, _, organization := createMemberQuotaTestOrg(t)
	setMemberQuotaTestOrganizationQuota(t, organization, 300)
	disabled := createMemberQuotaTestMember(t, organization.Id, "member-quota-status-disabled", model.OrganizationMemberStatusDisabled, 0, 0)

	// disabled 成员仍占预算、需要可读可改。
	require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, disabled.Id, 40))
	limit, _, _, allocated, _, err := GetOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, disabled.Id)
	require.NoError(t, err)
	require.Equal(t, 40, limit)
	require.Equal(t, 40, allocated)

	for _, status := range []string{model.OrganizationMemberStatusExited, model.OrganizationMemberStatusRemoved} {
		t.Run(status, func(t *testing.T) {
			target := createMemberQuotaTestMember(t, organization.Id, "member-quota-status-"+status, status, 10, 0)

			require.ErrorContains(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id, 20), "active or disabled organization member required")
			storedLimit, _ := storedMemberQuota(t, organization.Id, target.Id)
			require.Equal(t, 10, storedLimit)

			_, _, _, _, _, err := GetOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, target.Id)
			require.ErrorContains(t, err, "active or disabled organization member required")
		})
	}

	// 同级 admin 不能改自己（ensureOrganizationMemberTargetAllowed 只允许 admin 改 member）。
	require.ErrorContains(t, UpdateOrganizationMemberQuota(admin.Id, organization.Id, OrganizationAccessModeManagement, admin.Id, 10), "organization member operation forbidden")

	// 被拒绝的写不落审计：只有 disabled 成员那一次成功写入。
	require.Equal(t, int64(1), countOrganizationAuditRows(t, organization.Id, organizationAuditActionMemberQuotaUpdate))
}

func TestOrganizationMemberQuotaRejectsNegativeLimit(t *testing.T) {
	owner, _, member, organization := createMemberQuotaTestOrg(t)

	require.ErrorContains(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, -1), "must not be negative")
	storedLimit, _ := storedMemberQuota(t, organization.Id, member.Id)
	require.Zero(t, storedLimit)
	require.Zero(t, countOrganizationAuditRows(t, organization.Id, organizationAuditActionMemberQuotaUpdate))
}

func TestOrganizationMemberQuotaPermissionMatrix(t *testing.T) {
	owner, admin, member, organization := createMemberQuotaTestOrg(t)
	setMemberQuotaTestOrganizationQuota(t, organization, 300)
	outsider := createServiceTestUser(t, "member-quota-outsider", common.RoleCommonUser)
	crossOrgOwner := createServiceTestUser(t, "member-quota-cross-owner", common.RoleCommonUser)
	_, err := CreateOrganization(crossOrgOwner.Id, CreateOrganizationRequest{Name: "Member Quota Cross Org"})
	require.NoError(t, err)

	allowedUpdates := []struct {
		name       string
		operator   model.User
		accessMode string
	}{
		{name: "owner", operator: owner, accessMode: OrganizationAccessModeManagement},
		{name: "admin", operator: admin, accessMode: OrganizationAccessModeManagement},
	}
	for _, tc := range allowedUpdates {
		t.Run("update "+tc.name, func(t *testing.T) {
			require.NoError(t, UpdateOrganizationMemberQuota(tc.operator.Id, organization.Id, tc.accessMode, member.Id, 10))
			limit, _ := storedMemberQuota(t, organization.Id, member.Id)
			require.Equal(t, 10, limit)
			require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, 0))
		})
	}

	deniedUpdates := []struct {
		name       string
		operator   model.User
		accessMode string
		wantErr    string
	}{
		{name: "member", operator: member, accessMode: OrganizationAccessModeManagement, wantErr: "permission denied"},
		{name: "outsider", operator: outsider, accessMode: OrganizationAccessModeManagement, wantErr: "permission denied"},
		{name: "cross organization owner", operator: crossOrgOwner, accessMode: OrganizationAccessModeManagement, wantErr: "permission denied"},
	}
	auditsBefore := countOrganizationAuditRows(t, organization.Id, organizationAuditActionMemberQuotaUpdate)
	for _, tc := range deniedUpdates {
		t.Run("denied update "+tc.name, func(t *testing.T) {
			require.ErrorContains(t, UpdateOrganizationMemberQuota(tc.operator.Id, organization.Id, tc.accessMode, member.Id, 100), tc.wantErr)
			storedLimit, _ := storedMemberQuota(t, organization.Id, member.Id)
			require.Zero(t, storedLimit)
		})
	}
	require.Equal(t, auditsBefore, countOrganizationAuditRows(t, organization.Id, organizationAuditActionMemberQuotaUpdate))

	allowedReads := []struct {
		name       string
		operator   model.User
		accessMode string
	}{
		{name: "owner", operator: owner, accessMode: OrganizationAccessModeWorkspace},
		{name: "admin", operator: admin, accessMode: OrganizationAccessModeManagement},
	}
	for _, tc := range allowedReads {
		t.Run("read "+tc.name, func(t *testing.T) {
			limit, _, _, _, _, err := GetOrganizationMemberQuota(tc.operator.Id, organization.Id, tc.accessMode, member.Id)
			require.NoError(t, err)
			require.Zero(t, limit)
		})
	}

	deniedReads := []struct {
		name       string
		operator   model.User
		accessMode string
		wantErr    string
	}{
		{name: "member", operator: member, accessMode: OrganizationAccessModeWorkspace, wantErr: "permission denied"},
		{name: "outsider", operator: outsider, accessMode: OrganizationAccessModeWorkspace, wantErr: "organization access denied"},
	}
	for _, tc := range deniedReads {
		t.Run("denied read "+tc.name, func(t *testing.T) {
			limit, used, organizationQuota, allocated, overAllocated, err := GetOrganizationMemberQuota(tc.operator.Id, organization.Id, tc.accessMode, member.Id)
			require.ErrorContains(t, err, tc.wantErr)
			require.Zero(t, limit)
			require.Zero(t, used)
			require.Zero(t, organizationQuota)
			require.Zero(t, allocated)
			require.False(t, overAllocated)
		})
	}
}

// TestOrganizationMemberQuotaExternalDatabase 在真实 MySQL / PostgreSQL 上盖一遍配置写路径：
// SUM(quota_limit) 聚合的方言行为、成员行 FOR UPDATE、预算和校验、审计落库与清除。
// 需要 TEST_MYSQL_DSN 或 TEST_POSTGRES_DSN 且 ORGANIZATION_TEST_ALLOW_DESTRUCTIVE=1。
func TestOrganizationMemberQuotaExternalDatabase(t *testing.T) {
	db, _ := setupOrganizationExternalConcurrencyDB(t)
	// 组织策略加载会读 organization_disable_records：PostgreSQL 下缺表会让事务中止。
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.OrganizationDisableRecord{},
		&model.OrganizationAuditLog{},
	))

	suffix := strings.ReplaceAll(common.GetUUID(), "-", "")[:8]
	owner := model.User{Username: "external-member-quota-owner-" + suffix, Password: "password", DisplayName: "external-member-quota-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "emq-o-" + suffix}
	require.NoError(t, db.Create(&owner).Error)
	member := model.User{Username: "external-member-quota-member-" + suffix, Password: "password", DisplayName: "external-member-quota-member", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "emq-m-" + suffix}
	require.NoError(t, db.Create(&member).Error)
	second := model.User{Username: "external-member-quota-second-" + suffix, Password: "password", DisplayName: "external-member-quota-second", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "emq-s-" + suffix}
	require.NoError(t, db.Create(&second).Error)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "External Member Quota " + suffix})
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("quota", 300).Error)
	require.NoError(t, db.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, db.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: second.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	t.Cleanup(func() {
		require.NoError(t, db.Where("organization_id = ?", organization.Id).Delete(&model.OrganizationAuditLog{}).Error)
		require.NoError(t, db.Where("organization_id = ?", organization.Id).Delete(&model.OrganizationMember{}).Error)
		require.NoError(t, db.Where("id = ?", organization.Id).Delete(&model.Organization{}).Error)
		require.NoError(t, db.Unscoped().Delete(&model.User{}, []int{owner.Id, member.Id, second.Id}).Error)
	})

	require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, 100))
	limit, used, organizationQuota, allocated, overAllocated, err := GetOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id)
	require.NoError(t, err)
	require.Equal(t, 100, limit)
	require.Zero(t, used)
	require.Equal(t, 300, organizationQuota)
	require.Equal(t, 100, allocated)
	require.False(t, overAllocated)

	// 聚合跨方言：第二个成员把总和推到 150。
	require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, second.Id, 50))
	_, _, _, allocated, overAllocated, err = GetOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, second.Id)
	require.NoError(t, err)
	require.Equal(t, 150, allocated)
	require.False(t, overAllocated)

	// 平台调低总额度到已分配之和：允许；之后的增加被拒绝，降低/清除通过。
	require.NoError(t, db.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("quota", 150).Error)
	require.ErrorContains(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, second.Id, 51), "budget")
	require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, second.Id, 25))
	require.NoError(t, UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, 0))

	_, _, _, allocated, overAllocated, err = GetOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id)
	require.NoError(t, err)
	require.Equal(t, 25, allocated)
	require.False(t, overAllocated)

	var audit model.OrganizationAuditLog
	require.NoError(t, db.Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionMemberQuotaUpdate).Order("id desc").First(&audit).Error)
	require.Equal(t, "member", audit.TargetType)
	require.Equal(t, member.Username, audit.TargetName)
	require.Equal(t, `{"limit":100,"used":0}`, audit.BeforeData)
	require.Equal(t, `{"limit":0,"used":0}`, audit.AfterData)
}
