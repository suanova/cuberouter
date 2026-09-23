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

const (
	OrganizationJoinRuleMatchTypeDomain = "domain"
	OrganizationJoinRuleMatchTypeEmail  = "email"
)

// OrganizationJoinRule 一条组织自动加入规则：注册邮箱命中即把用户加入该组织。
//
// PatternNormalized 上是全局唯一索引，跨组织互斥由数据库兜底：同一个域名或邮箱
// 只能属于一个组织，运行时才会只有一个候选。
type OrganizationJoinRule struct {
	Id                int    `json:"id"`
	OrganizationId    int    `json:"organization_id" gorm:"not null;index"`
	MatchType         string `json:"match_type" gorm:"type:varchar(16);not null"`
	Pattern           string `json:"pattern" gorm:"type:varchar(255);not null"`
	PatternNormalized string `json:"pattern_normalized" gorm:"type:varchar(191);not null;uniqueIndex"`
	CreatedBy         int    `json:"created_by" gorm:"not null"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt         int64  `json:"updated_at" gorm:"bigint;index"`
}
