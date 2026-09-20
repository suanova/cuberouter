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
	"github.com/QuantumNous/new-api/model"
)

type OrganizationQuotaDataRequest struct {
	StartTimestamp int64
	EndTimestamp   int64
}

func GetOrganizationQuotaData(operatorUserId int, organizationId int, accessMode string, req OrganizationQuotaDataRequest) ([]*model.QuotaData, error) {
	actor, err := GetOrganizationActorContextForAccessMode(operatorUserId, organizationId, accessMode, true)
	if err != nil {
		return nil, err
	}

	tx := model.DB.Table("quota_data").
		Where("billing_account_type = ? AND billing_account_id = ? AND organization_id = ?", model.AccountContextTypeOrganization, organizationId, organizationId)
	if !actor.Capabilities.CanViewOrganizationWideData {
		tx = tx.Where("responsible_user_id = ?", operatorUserId)
	}
	if req.StartTimestamp > 0 {
		tx = tx.Where("created_at >= ?", req.StartTimestamp)
	}
	if req.EndTimestamp > 0 {
		tx = tx.Where("created_at <= ?", req.EndTimestamp)
	}

	var quotaData []*model.QuotaData
	if err := tx.Order("created_at asc").Find(&quotaData).Error; err != nil {
		return nil, err
	}
	return quotaData, nil
}
