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
	"strings"

	"gorm.io/gorm"
)

const (
	OrganizationStatusActive    = "active"
	OrganizationStatusDisabled  = "disabled"
	OrganizationStatusDissolved = "dissolved"

	OrganizationRoleOwner  = "owner"
	OrganizationRoleAdmin  = "admin"
	OrganizationRoleMember = "member"

	OrganizationMemberStatusActive   = "active"
	OrganizationMemberStatusDisabled = "disabled"
	OrganizationMemberStatusExited   = "exited"
	OrganizationMemberStatusRemoved  = "removed"

	OrganizationMemberDisableSourceOrganization = "organization"
	OrganizationMemberDisableSourcePlatform     = "platform"

	OrganizationInviteTypeEmail = "email"

	OrganizationInviteStatusPending  = "pending"
	OrganizationInviteStatusAccepted = "accepted"
	OrganizationInviteStatusExpired  = "expired"
	OrganizationInviteStatusRevoked  = "revoked"

	AccountContextTypePersonal     = "personal"
	AccountContextTypeOrganization = "organization"

	OrganizationDefaultQuota = 0
)

type Organization struct {
	Id             int    `json:"id"`
	Name           string `json:"name" gorm:"type:varchar(128);not null;index"`
	NameNormalized string `json:"-" gorm:"type:varchar(128);not null;uniqueIndex:idx_organizations_name_normalized"`
	Slug           string `json:"slug" gorm:"type:varchar(64);not null;uniqueIndex"`
	Description    string `json:"description" gorm:"type:varchar(512);default:''"`
	Group          string `json:"group" gorm:"type:varchar(64);default:'default'"`
	Status         string `json:"status" gorm:"type:varchar(16);not null;index;default:'active'"`
	Quota          int    `json:"quota" gorm:"not null;default:0"`
	UsedQuota      int    `json:"used_quota" gorm:"not null;default:0"`
	RequestCount   int    `json:"request_count" gorm:"not null;default:0"`
	OwnerUserId    int    `json:"owner_user_id" gorm:"index;not null;default:0"`
	CreatedBy      int    `json:"created_by" gorm:"index;not null"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt      int64  `json:"updated_at" gorm:"bigint;index"`
	DissolvedAt    int64  `json:"dissolved_at" gorm:"bigint;default:0"`
}

func (organization *Organization) BeforeSave(tx *gorm.DB) error {
	organization.Name = strings.TrimSpace(organization.Name)
	organization.NameNormalized = NormalizeOrganizationName(organization.Name)
	return nil
}

func (organization *Organization) BeforeCreate(tx *gorm.DB) error {
	if organization.Status == "" {
		organization.Status = OrganizationStatusActive
	}
	if organization.Group == "" {
		organization.Group = "default"
	}
	if organization.Quota == 0 {
		// 新组织默认不可直接消费，需平台配置或调整额度后才可用。
		organization.Quota = OrganizationDefaultQuota
	}
	if organization.OwnerUserId == 0 {
		organization.OwnerUserId = organization.CreatedBy
	}
	return nil
}
