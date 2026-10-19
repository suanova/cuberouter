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
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setMemberQuotaBillingLimit 直接改成员行 quota_limit：计费测试只关心读到的值，
// 不走配置接口（配置语义由 Task 2 的测试覆盖）。
func setMemberQuotaBillingLimit(t *testing.T, organizationId, userId, limit int) {
	t.Helper()
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", organizationId, userId).
		Update("quota_limit", limit).Error)
}

// createMemberQuotaBillingToken 建一把组织 Key，创建人和责任人由调用方指定。
func createMemberQuotaBillingToken(t *testing.T, organization *model.Organization, creator, responsible model.User) *model.Token {
	t.Helper()
	token, err := insertOrganizationTokenAsForTest(t, organization, creator, responsible, OrganizationTokenRequest{
		Name: "member-quota-billing-key", ExpiredTime: -1, RemainQuota: 1000,
	})
	require.NoError(t, err)
	return token
}

func TestOrganizationMemberQuotaBillingFundingIsStrict(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	setMemberQuotaBillingLimit(t, organization.Id, member.Id, 100)
	token := createMemberQuotaBillingToken(t, organization, member, member)

	first := organizationBillingRelayInfo(token, organization.Id)
	first.RequestId = "member-quota-funding-first"
	require.NoError(t, (&OrganizationFunding{relayInfo: first}).PreConsumeWithToken(100))
	_, used := storedMemberQuota(t, organization.Id, member.Id)
	require.Equal(t, 100, used)

	second := organizationBillingRelayInfo(token, organization.Id)
	second.RequestId = "member-quota-funding-second"
	err := (&OrganizationFunding{relayInfo: second}).PreConsumeWithToken(1)
	require.ErrorContains(t, err, "organization member quota is not enough")
	require.ErrorIs(t, err, ErrOrganizationMemberQuotaExceeded)

	// 事务回滚后不能留下任何账本痕迹，成员计数和组织计数都不动。
	_, used = storedMemberQuota(t, organization.Id, member.Id)
	require.Equal(t, 100, used)
	var storedOrganization model.Organization
	require.NoError(t, model.DB.First(&storedOrganization, organization.Id).Error)
	require.Equal(t, 100, storedOrganization.UsedQuota)
	var sessionCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationBillingSession{}).Where("request_id = ?", second.RequestId).Count(&sessionCount).Error)
	require.Zero(t, sessionCount)
	var recordCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationBillingRecord{}).Where("request_id = ?", second.RequestId).Count(&recordCount).Error)
	require.Zero(t, recordCount)
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.Equal(t, 900, storedToken.RemainQuota)
}

func TestOrganizationMemberQuotaBillingSettlementMayExceedCap(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	setMemberQuotaBillingLimit(t, organization.Id, member.Id, 100)
	token := createMemberQuotaBillingToken(t, organization, member, member)

	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "member-quota-settle-session"
	funding := &OrganizationFunding{relayInfo: relayInfo}
	require.NoError(t, funding.PreConsumeWithToken(80))

	// 结算的增差额允许越过成员上限（I5），之后的新请求才被拒绝。
	require.NoError(t, funding.SettleWithTokenActual(120))
	_, used := storedMemberQuota(t, organization.Id, member.Id)
	require.Equal(t, 120, used)

	next := organizationBillingRelayInfo(token, organization.Id)
	next.RequestId = "member-quota-settle-next"
	err := (&OrganizationFunding{relayInfo: next}).PreConsumeWithToken(1)
	require.ErrorContains(t, err, "organization member quota is not enough")
	require.ErrorIs(t, err, ErrOrganizationMemberQuotaExceeded)
}

func TestOrganizationMemberQuotaBillingReserveMoreMayExceedCap(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	setMemberQuotaBillingLimit(t, organization.Id, member.Id, 100)
	token := createMemberQuotaBillingToken(t, organization, member, member)

	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "member-quota-reserve-session"
	funding := &OrganizationFunding{relayInfo: relayInfo}
	require.NoError(t, funding.PreConsumeWithToken(80))

	require.NoError(t, funding.ReserveMoreWithToken(30))
	_, used := storedMemberQuota(t, organization.Id, member.Id)
	require.Equal(t, 110, used)

	next := organizationBillingRelayInfo(token, organization.Id)
	next.RequestId = "member-quota-reserve-next"
	err := (&OrganizationFunding{relayInfo: next}).PreConsumeWithToken(1)
	require.ErrorContains(t, err, "organization member quota is not enough")
	require.ErrorIs(t, err, ErrOrganizationMemberQuotaExceeded)
}

func TestOrganizationMemberQuotaBillingRefundClampsAtZero(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	setMemberQuotaBillingLimit(t, organization.Id, member.Id, 100)
	token := createMemberQuotaBillingToken(t, organization, member, member)

	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "member-quota-refund-session"
	funding := &OrganizationFunding{relayInfo: relayInfo}
	require.NoError(t, funding.PreConsumeWithToken(100))

	// 模拟计数漂移：已用值低于本次退款额时，退款必须夹在 0 而不是记成负消费。
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).
		Update("used_quota", 20).Error)

	require.NoError(t, funding.RefundWithToken())
	_, used := storedMemberQuota(t, organization.Id, member.Id)
	require.Zero(t, used)

	// 重复退款保持幂等，计数仍是 0。
	require.NoError(t, funding.RefundWithToken())
	_, used = storedMemberQuota(t, organization.Id, member.Id)
	require.Zero(t, used)
}

func TestOrganizationMemberQuotaBillingOwnerIsExempt(t *testing.T) {
	owner, _, organization := createOrganizationTokenTestOrg(t)
	// owner 行上存一个很小的上限：owner 豁免按 organization.owner_user_id 动态判定，
	// 不看 quota_limit 是否清零。
	setMemberQuotaBillingLimit(t, organization.Id, owner.Id, 10)
	token := createMemberQuotaBillingToken(t, organization, owner, owner)

	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "member-quota-owner-session"
	require.NoError(t, (&OrganizationFunding{relayInfo: relayInfo}).PreConsumeWithToken(100))

	// 豁免的是准入检查，累计值仍要记录，便于展示/审计。
	_, used := storedMemberQuota(t, organization.Id, owner.Id)
	require.Equal(t, 100, used)
}

func TestOrganizationMemberQuotaBillingCountsResponsibleUserOnly(t *testing.T) {
	owner, admin, member, organization := createMemberModelAccessTestOrg(t)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("quota", 2000).Error)
	setMemberQuotaBillingLimit(t, organization.Id, member.Id, 100)

	// 操作人/创建人是 admin，责任成员是 member：计数只跟责任成员走。
	token := createMemberQuotaBillingToken(t, organization, admin, member)

	relayInfo := organizationBillingRelayInfo(token, organization.Id)
	relayInfo.RequestId = "member-quota-attribution-session"
	require.NoError(t, (&OrganizationFunding{relayInfo: relayInfo}).PreConsumeWithToken(60))

	_, memberUsed := storedMemberQuota(t, organization.Id, member.Id)
	require.Equal(t, 60, memberUsed)
	_, adminUsed := storedMemberQuota(t, organization.Id, admin.Id)
	require.Zero(t, adminUsed)
	_, ownerUsed := storedMemberQuota(t, organization.Id, owner.Id)
	require.Zero(t, ownerUsed)
}

func TestOrganizationMemberQuotaBillingConcurrentFundingDoesNotExceedCap(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	setMemberQuotaBillingLimit(t, organization.Id, member.Id, 100)
	token := createMemberQuotaBillingToken(t, organization, member, member)

	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			relayInfo := organizationBillingRelayInfo(token, organization.Id)
			relayInfo.RequestId = fmt.Sprintf("member-quota-concurrent-%d", index)
			<-start
			errs[index] = (&OrganizationFunding{relayInfo: relayInfo}).PreConsumeWithToken(60)
		}(i)
	}
	close(start)
	wg.Wait()

	successes := 0
	failures := 0
	for _, err := range errs {
		if err == nil {
			successes++
			continue
		}
		require.ErrorContains(t, err, "organization member quota is not enough")
		require.ErrorIs(t, err, ErrOrganizationMemberQuotaExceeded)
		failures++
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, failures)
	_, used := storedMemberQuota(t, organization.Id, member.Id)
	require.Equal(t, 60, used)
}

func TestOrganizationMemberQuotaBillingErrorClassification(t *testing.T) {
	apiErr := organizationBillingAPIError(ErrOrganizationMemberQuotaExceeded)

	require.Equal(t, "insufficient_organization_member_quota", string(apiErr.GetErrorCode()))
	require.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

// TestOrganizationMemberQuotaBillingCounterExternalDatabase 在真实 MySQL/PostgreSQL
// 上验证条件更新、超额入账与退款夹零：RowsAffected 与 CASE 的语义只有真库能证明。
// 沿用组织外部用例的开关（TEST_MYSQL_DSN / TEST_POSTGRES_DSN +
// ORGANIZATION_TEST_ALLOW_DESTRUCTIVE=1），默认跳过。
func TestOrganizationMemberQuotaBillingCounterExternalDatabase(t *testing.T) {
	db, _ := setupOrganizationExternalConcurrencyDB(t)
	require.NoError(t, db.AutoMigrate(&model.Organization{}, &model.OrganizationMember{}))

	suffix := strings.ReplaceAll(common.GetUUID(), "-", "")[:8]
	now := common.GetTimestamp()
	organization := model.Organization{Name: "Member Quota Billing " + suffix, Slug: "mqb-" + suffix, Status: model.OrganizationStatusActive, CreatedBy: 1, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, db.Create(&organization).Error)
	owner := model.OrganizationMember{OrganizationId: organization.Id, UserId: organization.OwnerUserId, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive}
	member := model.OrganizationMember{OrganizationId: organization.Id, UserId: 2, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive, QuotaLimit: 100}
	require.NoError(t, db.Create(&owner).Error)
	require.NoError(t, db.Create(&member).Error)
	t.Cleanup(func() {
		require.NoError(t, db.Where("organization_id = ?", organization.Id).Delete(&model.OrganizationMember{}).Error)
		require.NoError(t, db.Where("id = ?", organization.Id).Delete(&model.Organization{}).Error)
	})

	counter := func(userId, delta int, enforce bool) error {
		return runOrganizationBillingTransaction(func(tx *gorm.DB) error {
			return updateOrganizationMemberUsedQuotaTx(tx, organization.Id, userId, organization.OwnerUserId, delta, enforce)
		})
	}

	require.NoError(t, counter(member.UserId, 60, true))
	require.ErrorIs(t, counter(member.UserId, 60, true), ErrOrganizationMemberQuotaExceeded)
	_, used := storedMemberQuota(t, organization.Id, member.UserId)
	require.Equal(t, 60, used)

	// 结算/提交后加价允许越过上限；退款夹在 0。
	require.NoError(t, counter(member.UserId, 60, false))
	_, used = storedMemberQuota(t, organization.Id, member.UserId)
	require.Equal(t, 120, used)
	require.NoError(t, counter(member.UserId, -500, false))
	_, used = storedMemberQuota(t, organization.Id, member.UserId)
	require.Zero(t, used)

	// owner 行即使存着很小的上限也不拦截，但累计值仍记录。
	require.NoError(t, db.Model(&model.OrganizationMember{}).Where("id = ?", owner.Id).Update("quota_limit", 10).Error)
	require.NoError(t, counter(owner.UserId, 100, true))
	_, ownerUsed := storedMemberQuota(t, organization.Id, owner.UserId)
	require.Equal(t, 100, ownerUsed)
}

// TestOrganizationMemberQuotaConcurrentExternalDatabase 在真实 MySQL/PostgreSQL 上
// 验证两处并发裁决只有真库能证明的部分：
//   - 两个严格预扣同时请求 60（上限 100）：条件更新必须由行锁串行化，只能一个成功；
//   - 两个配置动作同时各申请 200（组织总额度 300）：组织行锁必须让预算校验串行，
//     不能两个都通过。
//
// 沿用组织外部用例的开关（TEST_MYSQL_DSN / TEST_POSTGRES_DSN +
// ORGANIZATION_TEST_ALLOW_DESTRUCTIVE=1），默认跳过。
func TestOrganizationMemberQuotaConcurrentExternalDatabase(t *testing.T) {
	db, _ := setupOrganizationExternalConcurrencyDB(t)
	// 配置动作会读用户、组织禁用记录并写审计；PostgreSQL 下缺表会让事务中止。
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.OrganizationDisableRecord{},
		&model.OrganizationAuditLog{},
	))

	suffix := strings.ReplaceAll(common.GetUUID(), "-", "")[:8]
	owner := model.User{Username: "member-quota-concurrent-owner-" + suffix, Password: "password", DisplayName: "member-quota-concurrent-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "mqc-o-" + suffix}
	counterTarget := model.User{Username: "member-quota-concurrent-counter-" + suffix, Password: "password", DisplayName: "member-quota-concurrent-counter", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "mqc-c-" + suffix}
	budgetTargetA := model.User{Username: "member-quota-concurrent-a-" + suffix, Password: "password", DisplayName: "member-quota-concurrent-a", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "mqc-a-" + suffix}
	budgetTargetB := model.User{Username: "member-quota-concurrent-b-" + suffix, Password: "password", DisplayName: "member-quota-concurrent-b", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "mqc-b-" + suffix}
	for _, user := range []*model.User{&owner, &counterTarget, &budgetTargetA, &budgetTargetB} {
		require.NoError(t, db.Create(user).Error)
	}
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "External Member Quota Concurrent " + suffix})
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("quota", 300).Error)
	require.NoError(t, db.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: counterTarget.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive, QuotaLimit: 100}).Error)
	require.NoError(t, db.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: budgetTargetA.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, db.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: budgetTargetB.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	t.Cleanup(func() {
		require.NoError(t, db.Where("organization_id = ?", organization.Id).Delete(&model.OrganizationAuditLog{}).Error)
		require.NoError(t, db.Where("organization_id = ?", organization.Id).Delete(&model.OrganizationMember{}).Error)
		require.NoError(t, db.Where("id = ?", organization.Id).Delete(&model.Organization{}).Error)
		require.NoError(t, db.Unscoped().Delete(&model.User{}, []int{owner.Id, counterTarget.Id, budgetTargetA.Id, budgetTargetB.Id}).Error)
	})

	t.Run("concurrent strict counters admit exactly one pre-charge", func(t *testing.T) {
		start := make(chan struct{})
		errs := make([]error, 2)
		var wg sync.WaitGroup
		for i := range errs {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				<-start
				errs[index] = runOrganizationBillingTransaction(func(tx *gorm.DB) error {
					return updateOrganizationMemberUsedQuotaTx(tx, organization.Id, counterTarget.Id, organization.OwnerUserId, 60, true)
				})
			}(i)
		}
		close(start)
		wg.Wait()

		successes := 0
		failures := 0
		for _, err := range errs {
			if err == nil {
				successes++
				continue
			}
			require.ErrorIs(t, err, ErrOrganizationMemberQuotaExceeded)
			failures++
		}
		require.Equal(t, 1, successes)
		require.Equal(t, 1, failures)
		_, used := storedMemberQuota(t, organization.Id, counterTarget.Id)
		require.Equal(t, 60, used)
	})

	t.Run("concurrent budget configuration cannot exceed the organization quota", func(t *testing.T) {
		// 上一段给 counterTarget 留了 100 的存量上限；清掉它，这一段才只观察
		// 两个各 200 的申请如何竞争 300 的总预算。
		require.NoError(t, db.Model(&model.OrganizationMember{}).
			Where("organization_id = ? AND user_id = ?", organization.Id, counterTarget.Id).
			Update("quota_limit", 0).Error)
		targets := []int{budgetTargetA.Id, budgetTargetB.Id}
		start := make(chan struct{})
		errs := make([]error, 2)
		var wg sync.WaitGroup
		for i := range errs {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				<-start
				errs[index] = UpdateOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, targets[index], 200)
			}(i)
		}
		close(start)
		wg.Wait()

		successes := 0
		var failure error
		for _, err := range errs {
			if err == nil {
				successes++
				continue
			}
			failure = err
		}
		require.Equal(t, 1, successes)
		require.ErrorContains(t, failure, "budget")
		_, _, _, allocated, overAllocated, err := GetOrganizationMemberQuota(owner.Id, organization.Id, OrganizationAccessModeManagement, budgetTargetA.Id)
		require.NoError(t, err)
		require.Equal(t, 200, allocated)
		require.False(t, overAllocated)
	})
}
