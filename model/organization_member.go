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

type OrganizationMember struct {
	Id             int    `json:"id"`
	OrganizationId int    `json:"organization_id" gorm:"uniqueIndex:idx_org_user;index;index:idx_org_members_org_status,priority:1;not null"`
	UserId         int    `json:"user_id" gorm:"uniqueIndex:idx_org_user;index;not null"`
	Role           string `json:"role" gorm:"type:varchar(16);not null;index"`
	Status         string `json:"status" gorm:"type:varchar(16);not null;index;index:idx_org_members_org_status,priority:2;default:'active'"`
	DisabledSource string `json:"disabled_source" gorm:"type:varchar(16);not null;default:''"`
	InvitedBy      int    `json:"invited_by" gorm:"index;default:0"`
	JoinedAt       int64  `json:"joined_at" gorm:"bigint;index;default:0"`
	LastActiveAt   int64  `json:"last_active_at" gorm:"bigint;index;default:0"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt      int64  `json:"updated_at" gorm:"bigint;index"`
	DisabledAt     int64  `json:"disabled_at" gorm:"bigint;default:0"`
	ExitedAt       int64  `json:"exited_at" gorm:"bigint;default:0"`
	RemovedAt      int64  `json:"removed_at" gorm:"bigint;default:0"`
}
