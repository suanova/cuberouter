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
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// SelectPlaygroundOrganization 为游乐场组织额度兜底选择计费组织。
//
// 候选 = 调用者的活跃成员身份（且用户 enabled）∩ 组织 active ∩ 组织剩余额度 > 0。
// 规则：上次兜底用过的组织命中候选则沿用；否则取剩余额度最多，并列取 id 最小。
// 未命中返回 (nil, false, nil)；last-used 读取失败时退化为「剩余额度最多」。
func SelectPlaygroundOrganization(userId int) (*model.Organization, bool, error) {
	if userId <= 0 {
		return nil, false, nil
	}
	// 成员子查询的条件与 activeOrganizationMemberWithEnabledUserQuery 一致
	// （成员 active + 用户 enabled）；不直接复用该 helper 是因为它自带
	// Select("organization_members.*")，叠加 Select 后的 IN 子查询会返回多列，
	// MySQL 会直接报错。
	memberOrgIds := model.DB.Table("organization_members").
		Select("organization_members.organization_id").
		Joins("JOIN users ON users.id = organization_members.user_id AND users.deleted_at IS NULL AND users.status = ?", common.UserStatusEnabled).
		Where("organization_members.user_id = ? AND organization_members.status = ?", userId, model.OrganizationMemberStatusActive)

	var candidates []model.Organization
	if err := model.DB.
		Where("id IN (?) AND status = ? AND quota - used_quota > 0", memberOrgIds, model.OrganizationStatusActive).
		Find(&candidates).Error; err != nil {
		return nil, false, err
	}
	if len(candidates) == 0 {
		return nil, false, nil
	}
	if userSetting, err := model.GetUserSetting(userId, false); err == nil && userSetting.PlaygroundLastOrganizationId > 0 {
		for i := range candidates {
			if candidates[i].Id == userSetting.PlaygroundLastOrganizationId {
				return &candidates[i], true, nil
			}
		}
	}
	best := &candidates[0]
	for i := 1; i < len(candidates); i++ {
		candidate := &candidates[i]
		bestRemaining := best.Quota - best.UsedQuota
		candidateRemaining := candidate.Quota - candidate.UsedQuota
		if candidateRemaining > bestRemaining ||
			(candidateRemaining == bestRemaining && candidate.Id < best.Id) {
			best = candidate
		}
	}
	return best, true, nil
}

// BuildPlaygroundOrganizationScope 构造游乐场组织兜底的计费作用域。
// 发起人即 actor/creator/responsible：账本与日志因此能落到具体的人。
func BuildPlaygroundOrganizationScope(organization *model.Organization, actorUserId int) *TokenScopeContext {
	if organization == nil {
		return nil
	}
	return &TokenScopeContext{
		ScopeType:          model.AccountContextTypeOrganization,
		ScopeId:            organization.Id,
		BillingAccountType: model.AccountContextTypeOrganization,
		BillingAccountId:   organization.Id,
		OrganizationId:     organization.Id,
		CreatorUserId:      actorUserId,
		ResponsibleUserId:  actorUserId,
		ActorUserId:        actorUserId,
		OrganizationQuota:  organization.Quota - organization.UsedQuota,
		AccountGroup:       normalizeOrganizationGroup(organization.Group),
	}
}
