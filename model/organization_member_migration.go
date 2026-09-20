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
	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	organizationMemberUpdateAuditAction = "organization.member.update"
	organizationAuditTargetMember       = "member"
	organizationAuditRolePlatformAdmin  = "platform_admin"
	organizationAuditRolePlatformRoot   = "platform_root"
)

type organizationMemberAuditStatusSnapshot struct {
	Status string `json:"status"`
}

func prepareOrganizationMemberDisableSourceMigration(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&OrganizationMember{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&OrganizationMember{}, "DisabledSource") {
		if err := db.Migrator().AddColumn(&OrganizationMember{}, "DisabledSource"); err != nil {
			return err
		}
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var members []OrganizationMember
		if err := tx.Where("status = ? AND (disabled_source IS NULL OR disabled_source = '')", OrganizationMemberStatusDisabled).
			Order("id asc").
			Find(&members).Error; err != nil {
			return err
		}
		for i := range members {
			source, err := organizationMemberDisableSourceFromAudit(tx, members[i].Id)
			if err != nil {
				return err
			}
			if err := tx.Table("organization_members").
				Where("id = ? AND (disabled_source IS NULL OR disabled_source = '')", members[i].Id).
				UpdateColumn("disabled_source", source).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func organizationMemberDisableSourceFromAudit(tx *gorm.DB, memberId int) (string, error) {
	if !tx.Migrator().HasTable(&OrganizationAuditLog{}) {
		return OrganizationMemberDisableSourceOrganization, nil
	}
	var logs []OrganizationAuditLog
	if err := tx.Where("action_type = ? AND target_type = ? AND target_id = ?", organizationMemberUpdateAuditAction, organizationAuditTargetMember, memberId).
		Order("id asc").
		Find(&logs).Error; err != nil {
		return "", err
	}
	source := ""
	for _, log := range logs {
		var after organizationMemberAuditStatusSnapshot
		if err := common.UnmarshalJsonStr(log.AfterData, &after); err != nil {
			continue
		}
		switch after.Status {
		case OrganizationMemberStatusActive:
			source = ""
		case OrganizationMemberStatusDisabled:
			var before organizationMemberAuditStatusSnapshot
			_ = common.UnmarshalJsonStr(log.BeforeData, &before)
			logSource := OrganizationMemberDisableSourceOrganization
			if log.OperatorRole == organizationAuditRolePlatformAdmin || log.OperatorRole == organizationAuditRolePlatformRoot {
				logSource = OrganizationMemberDisableSourcePlatform
			}
			if before.Status != OrganizationMemberStatusDisabled || logSource == OrganizationMemberDisableSourcePlatform || source == "" {
				source = logSource
			}
		}
	}
	if source == "" {
		source = OrganizationMemberDisableSourceOrganization
	}
	return source, nil
}
