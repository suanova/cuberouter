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
	"strings"
	"time"

	"gorm.io/gorm/clause"
)

// API policy is read from the database on every execution, including queued
// requests. Browser Studio moderation is deliberately independent of this table.
type ImageAPIPolicy struct {
	GroupName  string `json:"group" gorm:"primaryKey;size:64"`
	Moderation string `json:"moderation" gorm:"size:16"`
	UpdatedBy  int    `json:"updated_by"`
	UpdatedAt  int64  `json:"updated_at"`
}

func ImageAPIModeration(group string) (string, error) {
	var rows []ImageAPIPolicy
	if err := DB.Where("group_name = ?", group).Limit(1).Find(&rows).Error; err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "external", nil
	}
	p := rows[0]
	if p.Moderation != "required" && p.Moderation != "external" {
		return "", errors.New("invalid_image_api_policy")
	}
	return p.Moderation, nil
}

func ListImageAPIPolicies() ([]ImageAPIPolicy, error) {
	var rows []ImageAPIPolicy
	err := DB.Order("group_name").Find(&rows).Error
	return rows, err
}

func SaveImageAPIPolicy(p ImageAPIPolicy) error {
	if strings.TrimSpace(p.GroupName) != p.GroupName || len(p.GroupName) == 0 || len(p.GroupName) > 64 ||
		(p.Moderation != "external" && p.Moderation != "required") {
		return errors.New("invalid_image_api_policy")
	}
	p.UpdatedAt = time.Now().Unix()
	return DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "group_name"}},
		DoUpdates: clause.AssignmentColumns([]string{"moderation", "updated_by", "updated_at"})}).Create(&p).Error
}
