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

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
)

// CanAccessDsh reports whether userId may use the DSH Agent.
//
// Everyone — platform administrators and root included — needs at least one
// active membership in an active organization that is both allowlisted and
// bound to a DSH channel. No organization is enabled until an administrator
// binds it, so the feature stays closed by default.
func CanAccessDsh(userId int) bool {
	if userId <= 0 {
		return false
	}
	enabledOrganizationIds := setting.DshEnabledOrganizationIds()
	if len(enabledOrganizationIds) == 0 {
		return false
	}
	return userHasActiveMembershipInAnyOrganization(userId, enabledOrganizationIds)
}

func userHasActiveMembershipInAnyOrganization(userId int, organizationIds []int) bool {
	var count int64
	err := model.DB.Table("organization_members").
		Joins("JOIN organizations ON organizations.id = organization_members.organization_id").
		Where("organization_members.user_id = ? AND organization_members.status = ?", userId, model.OrganizationMemberStatusActive).
		Where("organizations.status = ? AND organizations.id IN ?", model.OrganizationStatusActive, organizationIds).
		Count(&count).Error
	if err != nil {
		common.SysError(fmt.Sprintf("dsh access: count organizations for user %d failed: %s", userId, err.Error()))
		return false
	}
	return count > 0
}
