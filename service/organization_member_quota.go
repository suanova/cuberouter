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
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"gorm.io/gorm"
)

// ErrOrganizationMemberQuotaExceeded 表示责任成员的累计消费已到达组织内配置的
// 上限。它只在预扣（funding）的准入检查里返回，抛出后组织计费事务整体回滚。
var ErrOrganizationMemberQuotaExceeded = errors.New("organization member quota is not enough")

// organizationMemberQuotaStatuses 是占用成员配额预算的成员状态：active 与 disabled。
// exited/removed 的配额退出预算总和，重新加入时恢复（可能造成超额分配）。
func organizationMemberQuotaStatuses() []string {
	return []string{model.OrganizationMemberStatusActive, model.OrganizationMemberStatusDisabled}
}

// GetOrganizationMemberQuota 读取成员配额与当前预算占用情况。与成员名单同权限：
// 只有能看到全组织数据的人读得到。disabled 成员仍占预算，因此也可读。
func GetOrganizationMemberQuota(operatorUserId, organizationId int, accessMode string, targetUserId int) (limit, used, organizationQuota, allocated int, overAllocated bool, err error) {
	if operatorUserId <= 0 || organizationId <= 0 || targetUserId <= 0 {
		err = errors.New("invalid organization member request")
		return
	}
	actor, err := GetOrganizationActorContextForAccessMode(operatorUserId, organizationId, accessMode, true)
	if err != nil {
		return
	}
	if !actor.Capabilities.CanViewOrganizationWideData {
		err = errors.New("permission denied")
		return
	}
	var member model.OrganizationMember
	err = model.DB.Where("organization_id = ? AND user_id = ? AND status IN ?", organizationId, targetUserId, organizationMemberQuotaStatuses()).First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = errors.New("active or disabled organization member required")
		}
		return
	}
	allocated, err = sumOrganizationMemberQuotaLimits(model.DB, actor.Organization)
	if err != nil {
		return
	}
	return member.QuotaLimit, member.UsedQuota, actor.Organization.Quota, allocated, allocated > actor.Organization.Quota, nil
}

// UpdateOrganizationMemberQuota 配置成员在组织内的累计实际消费上限（0 = 不限制）。
// 预算校验在组织行锁内完成：同一组织的配置动作串行化，总和不会被并发写穿。
// 平台调低总额度、成员重新加入等非配置动作允许超额；之后的配置动作只允许不增加总和。
func UpdateOrganizationMemberQuota(operatorUserId, organizationId int, accessMode string, targetUserId int, limit int, auditMetadata ...OrganizationAuditRequestMetadata) error {
	if operatorUserId <= 0 || organizationId <= 0 || targetUserId <= 0 {
		return errors.New("invalid organization member request")
	}
	if limit < 0 {
		return errors.New("organization member quota limit must not be negative")
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		organization, decision, err := lockedOrganizationManagementDecisionWithTx(tx, operatorUserId, organizationId, accessMode, OrganizationCapabilityManageMemberQuotas)
		if err != nil {
			return err
		}
		var target model.OrganizationMember
		if err := model.LockForUpdate(tx).
			Where("organization_id = ? AND user_id = ? AND status IN ?", organizationId, targetUserId, organizationMemberQuotaStatuses()).
			First(&target).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("active or disabled organization member required")
			}
			return err
		}
		if err := ensureOrganizationMemberTargetAllowed(decision, organization, &target); err != nil {
			return err
		}
		allocated, err := sumOrganizationMemberQuotaLimits(tx, organization)
		if err != nil {
			return err
		}
		if err := validateMemberQuotaBudget(allocated, target.QuotaLimit, limit, organization.Quota); err != nil {
			return err
		}
		before := map[string]any{"limit": target.QuotaLimit, "used": target.UsedQuota}
		target.QuotaLimit = limit
		target.UpdatedAt = common.GetTimestamp()
		if err := tx.Model(&target).Select("quota_limit", "updated_at").Updates(&target).Error; err != nil {
			return err
		}
		after := map[string]any{"limit": target.QuotaLimit, "used": target.UsedQuota}
		return recordOrganizationAudit(tx, organization, operatorUserId, decision.Role, organizationAuditActionMemberQuotaUpdate, "member", target.Id, before, after, organizationAuditReasonMemberQuotaUpdate, auditMetadata...)
	})
}

// CheckOrganizationMemberQuotaAtSessionStart 是 /v1/realtime 的会话开始准入检查。
//
// realtime 请求同样先经 Relay 的 PreConsumeBilling 按估算预扣，再进入这里（设计 §5.4）；
// 检查因此运行在预扣之后：配额已用尽的成员不再允许建立上游连接，本次预扣随请求失败退回。
// 由于预扣已计入 used_quota，预扣后恰好到达上限的会话会被拒绝，比 I4 的等号规则更保守。
// 检查只读、不做预留；并发会话可能同时通过并各自超额，与「上限是准入闸门」的口径一致。
//
// 与计费计数同口径：owner 按 organization.owner_user_id 动态豁免；成员行缺失
// （用户被彻底删除等）不阻断准入。
func CheckOrganizationMemberQuotaAtSessionStart(relayInfo *relaycommon.RelayInfo) error {
	if !IsOrganizationBilling(relayInfo) || relayInfo.ResponsibleUserId <= 0 {
		return nil
	}
	var organization model.Organization
	if err := model.DB.Select("id", "owner_user_id").Where("id = ?", relayInfo.BillingAccountId).First(&organization).Error; err != nil {
		return err
	}
	if organization.OwnerUserId == relayInfo.ResponsibleUserId {
		return nil
	}
	var member model.OrganizationMember
	err := model.DB.Select("id", "quota_limit", "used_quota").
		Where("organization_id = ? AND user_id = ?", relayInfo.BillingAccountId, relayInfo.ResponsibleUserId).
		First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if member.QuotaLimit > 0 && member.UsedQuota >= member.QuotaLimit {
		return ErrOrganizationMemberQuotaExceeded
	}
	return nil
}

// updateOrganizationMemberUsedQuotaTx 在组织计费的同一事务里维护责任成员的累计消费。
//
// 两种口径（设计 I4/I5）：
//   - enforce（预扣准入）：把上限判定塞进 UPDATE 的 WHERE 里由数据库原子裁决，
//     0 行即超限，返回 ErrOrganizationMemberQuotaExceeded 让整个事务回滚；
//   - 非 enforce（结算、提交后加价、退款、事后扣费）：无条件入账，允许结算把
//     已用值推过上限；负差额夹在 0，成员计数永远不会变成负数。
//
// owner 按当前 organization.owner_user_id 动态豁免准入判定，但仍累计 used_quota
// 供展示/审计。成员行缺失（用户被彻底删除等）只记 SysError，不阻断组织计费。
func updateOrganizationMemberUsedQuotaTx(tx *gorm.DB, organizationId, userId, ownerUserId, delta int, enforce bool) error {
	if tx == nil || organizationId <= 0 || userId <= 0 || delta == 0 {
		return nil
	}
	if enforce && delta > 0 && userId != ownerUserId {
		result := tx.Model(&model.OrganizationMember{}).
			Where("organization_id = ? AND user_id = ? AND (quota_limit = 0 OR used_quota + ? <= quota_limit)", organizationId, userId, delta).
			UpdateColumn("used_quota", gorm.Expr("used_quota + ?", delta))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return nil
		}
		exists, err := organizationMemberRowExistsTx(tx, organizationId, userId)
		if err != nil {
			return err
		}
		if exists {
			return ErrOrganizationMemberQuotaExceeded
		}
		common.SysError(fmt.Sprintf("organization member quota counter skipped: organization %d has no member row for user %d", organizationId, userId))
		return nil
	}

	expression := "used_quota + ?"
	args := []any{delta}
	if delta < 0 {
		expression = "CASE WHEN used_quota + ? < 0 THEN 0 ELSE used_quota + ? END"
		args = []any{delta, delta}
	}
	result := tx.Model(&model.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", organizationId, userId).
		UpdateColumn("used_quota", gorm.Expr(expression, args...))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	// MySQL 对「值未变化」的 UPDATE 报告 0 行（例如负差额已夹在 0），
	// 先确认成员行是否真的不存在，避免把正常 no-op 误报成缺行。
	exists, err := organizationMemberRowExistsTx(tx, organizationId, userId)
	if err != nil {
		return err
	}
	if !exists {
		common.SysError(fmt.Sprintf("organization member quota counter skipped: organization %d has no member row for user %d", organizationId, userId))
	}
	return nil
}

// organizationMemberRowExistsTx 判断成员关系是否还在；计费只按 organization_id + user_id
// 定位，disabled/exited 的成员行同样计入（设计 I8）。
func organizationMemberRowExistsTx(tx *gorm.DB, organizationId, userId int) (bool, error) {
	var count int64
	if err := tx.Model(&model.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", organizationId, userId).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// sumOrganizationMemberQuotaLimits 汇总组织当前占用的配额预算：active + disabled 的
// 非 owner 成员 quota_limit 之和。owner 动态豁免；0 = 不限制，天然不占预算。
func sumOrganizationMemberQuotaLimits(tx *gorm.DB, organization *model.Organization) (int, error) {
	if tx == nil || organization == nil {
		return 0, errors.New("invalid organization request")
	}
	var allocated int
	err := tx.Model(&model.OrganizationMember{}).
		Where("organization_id = ? AND user_id <> ? AND status IN ?", organization.Id, organization.OwnerUserId, organizationMemberQuotaStatuses()).
		Select("COALESCE(SUM(quota_limit), 0)").
		Scan(&allocated).Error
	return allocated, err
}

// validateMemberQuotaBudget 校验一次配额修改是否突破预算约束（设计 I6）：
//   - newSum = currentSum - currentLimit + newLimit；
//   - 当前总和已超组织总额度时，只允许不增加总和的修改（降低/清除）；
//   - 未超时要求新总和不超过组织总额度；
//   - 清除（newLimit = 0）任何时候都允许。
func validateMemberQuotaBudget(currentSum, currentLimit, newLimit, organizationQuota int) error {
	if newLimit == 0 {
		return nil
	}
	newSum := currentSum - currentLimit + newLimit
	if currentSum > organizationQuota {
		if newSum > currentSum {
			return errors.New("organization member quota budget exceeded")
		}
		return nil
	}
	if newSum > organizationQuota {
		return errors.New("organization member quota budget exceeded")
	}
	return nil
}
