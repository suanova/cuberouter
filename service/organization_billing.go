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

	"gorm.io/gorm"
)

// 组织账本的最小片段：平台管理员调整组织配额时，把这次增减记进账本，
// 组织的用量/账单页面才能看到"额度是管理员加上去的"而不是凭空多出来。
//
// 完整的组织计费（预扣、结算、租约与崩溃修复）在 service/organization_funding.go
// 与 organization_billing_*.go 里，这里只放账本写入的公共原语。

func createOrganizationBillingRecordTx(tx *gorm.DB, record *model.OrganizationBillingRecord) error {
	if record == nil {
		return nil
	}
	return tx.Create(record).Error
}

func createOrganizationQuotaAdjustmentBillingRecordTx(tx *gorm.DB, adjustment *model.OrganizationQuotaAdjustment) error {
	if adjustment == nil {
		return nil
	}
	record := &model.OrganizationBillingRecord{
		OrganizationId:    adjustment.OrganizationId,
		RecordKey:         "adjust:" + adjustment.IdempotencyKey,
		RecordType:        model.OrganizationBillingRecordTypeAdjustment,
		QuotaDelta:        adjustment.QuotaDelta,
		UsedQuotaDelta:    0,
		UsageQuota:        0,
		QuotaBefore:       adjustment.QuotaBefore,
		QuotaAfter:        adjustment.QuotaAfter,
		UsedQuotaBefore:   adjustment.UsedQuota,
		UsedQuotaAfter:    adjustment.UsedQuota,
		ResponsibleUserId: adjustment.OperatorUserId,
		CreatedAt:         adjustment.CreatedAt,
	}
	return createOrganizationBillingRecordTx(tx, record)
}
