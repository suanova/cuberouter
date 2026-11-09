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
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
)

// DSH 按组织实例化后，每个请求都必须先落到某个组织的上下文：组织决定计费账本、
// 分组以及（通过渠道绑定）具体访问哪个 dsh 容器。这里的解析函数是唯一入口，
// 中间件与上传接口都通过它拿组织 ID 与绑定渠道。
var (
	// ErrDshOrganizationRequired：调用方用个人令牌（没有组织作用域）提交 DSH 任务。
	ErrDshOrganizationRequired = errors.New("DSH requires an organization account context")
	// ErrDshOrganizationMissing：用户不属于任何已开通（白名单 + 已绑定渠道）组织。
	ErrDshOrganizationMissing = errors.New("no DSH-enabled organization for this user")
	// ErrDshOrganizationDenied：指定的组织对调用方不可用（非成员、未开通或已停用）。
	ErrDshOrganizationDenied = errors.New("organization is not available for DSH")
	// ErrDshChannelUnavailable：组织绑定的渠道缺失、停用或不再是 DSH 任务插件渠道。
	ErrDshChannelUnavailable = errors.New("the DSH channel is not available")
)

// ResolveDshOrganizationForUser 为会话请求（/pg/agent、/api/dsh）解析组织。
//
// preferredOrgId 来自会话级请求头或账号上下文：用户明确要求过的组织只要成员
// 关系成立就优先；如果该组织尚未开通 DSH，不报错而是回退到自动选择——页面上
// 不该因为用户切到了别的组织上下文而变成死页面。
func ResolveDshOrganizationForUser(userId int, preferredOrgId int) (*model.Organization, int, error) {
	if userId <= 0 {
		return nil, 0, ErrDshOrganizationDenied
	}
	enabledOrganizationIds := setting.DshEnabledOrganizationIds()
	if len(enabledOrganizationIds) == 0 {
		return nil, 0, ErrDshOrganizationMissing
	}
	if preferredOrgId > 0 {
		// 明确指定了组织就必须先证明成员关系，避免把"非成员头"静默降级成别的组织。
		if _, _, err := GetOrganizationForMember(userId, preferredOrgId, false); err != nil {
			return nil, 0, ErrDshOrganizationDenied
		}
	}
	candidates, err := dshCandidateOrganizations(userId, enabledOrganizationIds)
	if err != nil {
		return nil, 0, err
	}
	if len(candidates) == 0 {
		return nil, 0, ErrDshOrganizationMissing
	}
	chosen := candidates[0]
	if preferredOrgId > 0 {
		for i := range candidates {
			if candidates[i].Id == preferredOrgId {
				chosen = candidates[i]
				break
			}
		}
	}
	channelId, err := resolveDshChannel(setting.GetDshOrganizationChannels()[chosen.Id])
	if err != nil {
		return nil, 0, err
	}
	return &chosen, channelId, nil
}

// ResolveDshOrganizationForToken 为 /v1/tasks 的令牌请求解析组织。
//
// 与按用户解析不同，这里不接受自动选择：令牌本身就是作用域声明，个人令牌
// （organizationId <= 0）必须被严格拒绝，管理员/root 也不例外。
func ResolveDshOrganizationForToken(userId, organizationId int) (*model.Organization, int, error) {
	if organizationId <= 0 {
		return nil, 0, ErrDshOrganizationRequired
	}
	organization, _, err := GetOrganizationForMember(userId, organizationId, false)
	if err != nil {
		return nil, 0, ErrDshOrganizationDenied
	}
	if !slices.Contains(setting.DshEnabledOrganizationIds(), organization.Id) {
		return nil, 0, ErrDshOrganizationDenied
	}
	channelId, err := resolveDshChannel(setting.GetDshOrganizationChannels()[organization.Id])
	if err != nil {
		return nil, 0, err
	}
	return organization, channelId, nil
}

// DshOrganizationGroupsForUser 返回用户可用 DSH 组织的规范化分组，供
// /api/user/models?endpoint=agent&plugin=dsh 把组织分组并入模型查询。
// 渠道不可用的组织不计入：模型列表不该展示一个必然提交失败的组织。
func DshOrganizationGroupsForUser(userId int) []string {
	if userId <= 0 {
		return nil
	}
	enabledOrganizationIds := setting.DshEnabledOrganizationIds()
	if len(enabledOrganizationIds) == 0 {
		return nil
	}
	candidates, err := dshCandidateOrganizations(userId, enabledOrganizationIds)
	if err != nil {
		common.SysLog(fmt.Sprintf("dsh organization: list groups for user %d failed: %s", userId, err.Error()))
		return nil
	}
	bindings := setting.GetDshOrganizationChannels()
	seen := make(map[string]struct{}, len(candidates))
	groups := make([]string, 0, len(candidates))
	for i := range candidates {
		if _, err := resolveDshChannel(bindings[candidates[i].Id]); err != nil {
			continue
		}
		group := NormalizeOrganizationGroup(candidates[i].Group)
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		groups = append(groups, group)
	}
	return groups
}

// dshCandidateOrganizations 返回用户的活跃成员关系 ∩ active 组织 ∩ 已开通组织，
// 按组织 ID 升序。不按余额过滤：余额耗尽交给预扣费报"额度不足"。
func dshCandidateOrganizations(userId int, enabledOrganizationIds []int) ([]model.Organization, error) {
	if userId <= 0 || len(enabledOrganizationIds) == 0 {
		return nil, nil
	}
	memberOrgIds := activeOrganizationMemberWithEnabledUserQuery(model.DB).
		Select("organization_members.organization_id").
		Where("organization_members.user_id = ? AND organization_members.status = ?", userId, model.OrganizationMemberStatusActive)

	var candidates []model.Organization
	err := model.DB.
		Where("id IN (?) AND status = ? AND id IN ?", memberOrgIds, model.OrganizationStatusActive, enabledOrganizationIds).
		Order("id asc").
		Find(&candidates).Error
	if err != nil {
		return nil, err
	}
	return candidates, nil
}

// resolveDshChannel 校验组织绑定的渠道仍然是可用的 DSH 任务插件渠道。
func resolveDshChannel(channelId int) (int, error) {
	if channelId <= 0 {
		return 0, ErrDshChannelUnavailable
	}
	channel, err := model.CacheGetChannel(channelId)
	if err != nil || channel == nil {
		return 0, ErrDshChannelUnavailable
	}
	if channel.Status != common.ChannelStatusEnabled {
		return 0, ErrDshChannelUnavailable
	}
	if channel.Type != constant.ChannelTypeTaskPlugin {
		return 0, ErrDshChannelUnavailable
	}
	if strings.TrimSpace(channel.GetSetting().TaskPluginKey) != setting.DshTaskPluginKey {
		return 0, ErrDshChannelUnavailable
	}
	return channel.Id, nil
}
