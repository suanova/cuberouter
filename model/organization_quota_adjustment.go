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
package model

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type OrganizationQuotaAdjustment struct {
	Id             int    `json:"id"`
	OrganizationId int    `json:"organization_id" gorm:"index;not null"`
	OperatorUserId int    `json:"operator_user_id" gorm:"index;not null"`
	QuotaDelta     int    `json:"quota_delta" gorm:"not null"`
	QuotaBefore    int    `json:"quota_before" gorm:"not null"`
	QuotaAfter     int    `json:"quota_after" gorm:"not null"`
	UsedQuota      int    `json:"used_quota" gorm:"not null;default:0"`
	Reason         string `json:"reason" gorm:"type:varchar(255);default:''"`
	IdempotencyKey string `json:"idempotency_key" gorm:"type:varchar(191);uniqueIndex;not null"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint;index"`
}

type organizationQuotaAdjustmentIdempotencyBackfillColumn struct {
	IdempotencyKey string `gorm:"type:varchar(191)"`
}

func (organizationQuotaAdjustmentIdempotencyBackfillColumn) TableName() string {
	return "organization_quota_adjustments"
}

func (adjustment *OrganizationQuotaAdjustment) BeforeSave(tx *gorm.DB) error {
	if strings.TrimSpace(adjustment.IdempotencyKey) == "" {
		return errors.New("quota adjustment idempotency key is required")
	}
	return nil
}

func prepareOrganizationQuotaAdjustmentIdempotencyMigration(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&OrganizationQuotaAdjustment{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&OrganizationQuotaAdjustment{}, "idempotency_key") {
		if err := db.Migrator().AddColumn(&organizationQuotaAdjustmentIdempotencyBackfillColumn{}, "IdempotencyKey"); err != nil {
			return err
		}
	}
	var adjustments []OrganizationQuotaAdjustment
	if err := db.Select("id", "idempotency_key").
		Where("idempotency_key = ? OR idempotency_key IS NULL", "").
		Find(&adjustments).Error; err != nil {
		return err
	}
	for _, adjustment := range adjustments {
		key := fmt.Sprintf("quota-adjustment:legacy:%d", adjustment.Id)
		if err := db.Exec("UPDATE organization_quota_adjustments SET idempotency_key = ? WHERE id = ?", key, adjustment.Id).Error; err != nil {
			return err
		}
	}
	return nil
}
