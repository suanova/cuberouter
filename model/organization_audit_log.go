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

type OrganizationAuditLog struct {
	Id                  int    `json:"id"`
	OrganizationId      int    `json:"organization_id" gorm:"index;not null"`
	OrganizationName    string `json:"organization_name" gorm:"type:varchar(128);default:''"`
	OrganizationSlug    string `json:"organization_slug" gorm:"type:varchar(64);index;default:''"`
	OperatorUserId      int    `json:"operator_user_id" gorm:"index;not null"`
	OperatorUsername    string `json:"operator_username" gorm:"type:varchar(64);default:''"`
	OperatorDisplayName string `json:"operator_display_name" gorm:"type:varchar(128);default:''"`
	OperatorRole        string `json:"operator_role" gorm:"type:varchar(16);default:''"`
	ActionType          string `json:"action_type" gorm:"type:varchar(64);index;not null"`
	TargetType          string `json:"target_type" gorm:"type:varchar(32);index;not null"`
	TargetId            int    `json:"target_id" gorm:"index;default:0"`
	TargetName          string `json:"target_name" gorm:"type:varchar(255);default:''"`
	TargetMetadata      string `json:"target_metadata" gorm:"type:text"`
	BeforeData          string `json:"before_data" gorm:"type:text"`
	AfterData           string `json:"after_data" gorm:"type:text"`
	Reason              string `json:"reason" gorm:"type:varchar(255);default:''"`
	Ip                  string `json:"ip" gorm:"type:varchar(64);default:''"`
	UserAgent           string `json:"user_agent" gorm:"type:varchar(512);default:''"`
	CreatedAt           int64  `json:"created_at" gorm:"bigint;index"`
}
