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

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

// accountMembershipPurgeResult 是一次账号收口的结果：收口失败的组织 id（0 表示没有
// 组织走到失败分支）与真正收口的成员行数（成员行按 organization_id + user_id 唯一，
// 因此它也是被收口的组织数）。
//
// FailedOrganizationId 只记「哪个组织让这次收口失败了」，不记失败性质：blocker 拒绝
// 与判定/写审计本身报错都会落在它上面。
type accountMembershipPurgeResult struct {
	FailedOrganizationId int
	PurgedMemberships    int
}

// DeleteUserAccount 硬删除一个账号，并在同一事务内收口其组织成员关系。
//
// 两个硬删入口（controller/user.go 的 DeleteUser、controller/aggregated_api.go 的
// AggregatedDeleteUser）都必须走这里：硬删会按 user_id 连带删除该用户的全部 token，
// 其中包含组织 token，因此收口与删除必须在同一事务里完成，且必须在删 token 之前
// 判定该用户是否持有组织 key。
func DeleteUserAccount(operatorUserId, targetUserId int, auditMetadata ...OrganizationAuditRequestMetadata) error {
	if operatorUserId <= 0 || targetUserId <= 0 {
		return errors.New("invalid user account deletion request")
	}
	var deleted *model.HardDeletedUser
	purgeResult := accountMembershipPurgeResult{}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		// 先锁 users 行，把本事务的锁序固定成 users → organizations → organization_members。
		// 注册路径是 user → member（InsertWithTx 落库后插成员行），管理员加人也是先读 user
		// 再写成员行；收口若反过来先锁成员行再碰 users，就会与它们形成反向锁序。
		if err := model.LockForUpdate(tx).Where("id = ?", targetUserId).First(&model.User{}).Error; err != nil {
			return err
		}
		var err error
		purgeResult, err = purgeAccountOrganizationMembershipsWithTx(tx, targetUserId, operatorUserId, auditMetadata...)
		if err != nil {
			return err
		}
		deleted, err = model.HardDeleteUserWithTx(tx, targetUserId)
		return err
	})
	if err != nil {
		if purgeResult.FailedOrganizationId > 0 {
			// 事务已经回滚，这条留痕只能写在它自己的事务里。组织需要知道有人试图删除
			// 一个还挂着它 owner / key 的账号，以及被什么挡住（动作常量与组织子系统
			// 记"成员操作被 blocker 拦下"用的是同一条，便于一处检索）。
			if auditErr := recordOrganizationMemberKeyTransferBlockedAudit(purgeResult.FailedOrganizationId, operatorUserId, organizationAuditOperatorRoleSystem, targetUserId, 0, "account deletion blocked", err, auditMetadata...); auditErr != nil {
				common.SysError("failed to record organization blocked audit for account deletion: " + auditErr.Error())
			}
		}
		return err
	}
	if err := deleted.Finalize(); err != nil {
		return err
	}
	if purgeResult.PurgedMemberships > 0 {
		// 收口是跨组织的平台级动作（一个账号可能同时属于多个组织），留一条汇总供平台侧排查。
		common.SysLog(fmt.Sprintf("account deletion purged organization memberships: target_user_id=%d organizations=%d", targetUserId, purgeResult.PurgedMemberships))
	}
	return nil
}

// purgeAccountOrganizationMembershipsWithTx 按组织升序逐个处理该用户的成员行。
//
// 锁序与组织子系统一致（先锁组织行，再锁成员行），并按组织 id 升序，避免与并发的
// 组织操作形成反向锁序。
func purgeAccountOrganizationMembershipsWithTx(tx *gorm.DB, targetUserId, operatorUserId int, auditMetadata ...OrganizationAuditRequestMetadata) (accountMembershipPurgeResult, error) {
	result := accountMembershipPurgeResult{}
	var organizationIds []int
	if err := tx.Model(&model.OrganizationMember{}).
		Where("user_id = ?", targetUserId).
		Order("organization_id asc").
		Pluck("organization_id", &organizationIds).Error; err != nil {
		return result, err
	}
	for _, organizationId := range organizationIds {
		organization, err := lockOrganizationForUpdateWithTx(tx, organizationId)
		if err != nil {
			return result, err
		}
		var member model.OrganizationMember
		if err := model.LockForUpdate(tx).
			Where("organization_id = ? AND user_id = ?", organizationId, targetUserId).
			First(&member).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return result, err
		}
		if err := ensureAccountMayLeaveOrganizationWithTx(tx, organization, targetUserId); err != nil {
			result.FailedOrganizationId = organizationId
			return result, err
		}
		// 审计先写：recordOrganizationAudit 会按 target 补快照，此时成员行必须还在。
		if err := recordOrganizationAudit(tx, organization, operatorUserId, organizationAuditOperatorRoleSystem, organizationAuditActionMemberAccountDeleted, "member", member.Id, member, nil, fmt.Sprintf("account_deleted:%d", targetUserId), auditMetadata...); err != nil {
			return result, err
		}
		if err := tx.Delete(&model.OrganizationMember{}, member.Id).Error; err != nil {
			return result, err
		}
		result.PurgedMemberships++
		// 防御性解除：走到这里说明该成员已经不带 key（无 key 时 blocker 理论上不存在），
		// 但历史脏行或刚被软删的 token 都可能留下 active blocker，清掉不留死锁。
		if _, err := clearOrganizationMemberDisabledTokenBlockersWithTx(tx, organizationId, targetUserId, common.GetTimestamp()); err != nil {
			return result, err
		}
	}
	return result, nil
}

// ensureAccountMayLeaveOrganizationWithTx 判定一个账号是否可以从该组织收口离开。
// 解散组织的成员在 evaluateWorkspaceOrganizationPolicy 下什么都拿不到，key 也已无
// 实际效力，因此直接放行；否则 owner 与持 key 者一律拒绝——两种处置都会破坏组织
// 不变式或绕过既有的 key 移交语义，不能由"删用户"顺手完成。
func ensureAccountMayLeaveOrganizationWithTx(tx *gorm.DB, organization *model.Organization, targetUserId int) error {
	if organization.Status == model.OrganizationStatusDissolved {
		return nil
	}
	if organization.OwnerUserId == targetUserId {
		return organizationOperationBlockedError(
			fmt.Sprintf("organization %s still owns this account as its owner; transfer ownership first", organization.Name),
			"active_owner",
		)
	}
	var keyCount int64
	if err := tx.Model(&model.Token{}).
		Where("scope_type = ? AND organization_id = ? AND (user_id = ? OR responsible_user_id = ?)", model.TokenScopeOrganization, organization.Id, targetUserId, targetUserId).
		Count(&keyCount).Error; err != nil {
		return err
	}
	if keyCount > 0 {
		return organizationOperationBlockedError(
			fmt.Sprintf("organization %s still has keys held by this account; hand the keys over first", organization.Name),
			"organization_keys",
		)
	}
	return nil
}
