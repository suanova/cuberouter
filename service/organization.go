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

	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

// GetOrganizationForMember 读取组织与其当前用户的有效成员记录。
// 禁用中的组织一律拒绝；已解散的组织只有 allowDissolvedRead 时才放行（只读场景）。
func GetOrganizationForMember(userId int, organizationId int, allowDissolvedRead bool) (*model.Organization, *model.OrganizationMember, error) {
	organization, member, err := getOrganizationMember(userId, organizationId)
	if err != nil {
		return nil, nil, err
	}
	if organization.Status == model.OrganizationStatusDisabled {
		return nil, nil, errors.New("organization disabled")
	}
	if !allowDissolvedRead && organization.Status == model.OrganizationStatusDissolved {
		return nil, nil, errors.New("organization dissolved")
	}
	return organization, member, nil
}

func getOrganizationMember(userId int, organizationId int) (*model.Organization, *model.OrganizationMember, error) {
	return getOrganizationMemberWithTx(model.DB, userId, organizationId)
}

func getOrganizationMemberWithTx(tx *gorm.DB, userId int, organizationId int) (*model.Organization, *model.OrganizationMember, error) {
	var organization model.Organization
	if err := tx.Where("id = ?", organizationId).First(&organization).Error; err != nil {
		return nil, nil, err
	}
	var member model.OrganizationMember
	if err := tx.Where("organization_id = ? AND user_id = ? AND status = ?", organizationId, userId, model.OrganizationMemberStatusActive).First(&member).Error; err != nil {
		return nil, nil, errors.New("permission denied")
	}
	return &organization, &member, nil
}
