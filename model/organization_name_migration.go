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
	"gorm.io/gorm/clause"
)

const organizationNameNormalizedIndex = "idx_organizations_name_normalized"
const organizationNameMySQLCollation = "utf8mb4_bin"

type organizationNameNormalizedBackfillColumn struct {
	NameNormalized string `gorm:"column:name_normalized;type:varchar(128)"`
}

func (organizationNameNormalizedBackfillColumn) TableName() string {
	return "organizations"
}

func NormalizeOrganizationName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func prepareOrganizationNameUniquenessMigration(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&Organization{}) {
		return nil
	}
	complete, err := organizationNameUniquenessMigrationComplete(db)
	if err != nil {
		return err
	}
	if complete {
		return nil
	}
	if !db.Migrator().HasColumn(&Organization{}, "NameNormalized") {
		if err := db.Migrator().AddColumn(&organizationNameNormalizedBackfillColumn{}, "NameNormalized"); err != nil {
			return err
		}
	}
	var organizations []Organization
	if err := db.Select("id", "name", "name_normalized").Order("id asc").Find(&organizations).Error; err != nil {
		return err
	}
	for _, organization := range organizations {
		normalized := NormalizeOrganizationName(organization.Name)
		if err := db.Table("organizations").Where("id = ?", organization.Id).
			UpdateColumn("name_normalized", normalized).Error; err != nil {
			return err
		}
	}
	if err := ensureOrganizationNameNormalizedComparison(db); err != nil {
		return err
	}
	var conflict struct {
		NameNormalized string
		Count          int64
	}
	err = db.Table("organizations").
		Select("name_normalized, COUNT(*) AS count").
		Group("name_normalized").
		Having("COUNT(*) > 1").
		Order("name_normalized asc").
		Take(&conflict).Error
	if err == nil {
		return fmt.Errorf("duplicate normalized organization name %q", conflict.NameNormalized)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return nil
}

func organizationNameUniquenessMigrationComplete(db *gorm.DB) (bool, error) {
	if !db.Migrator().HasIndex(&Organization{}, organizationNameNormalizedIndex) {
		return false, nil
	}
	if db.Dialector.Name() != "mysql" {
		return true, nil
	}
	var collation string
	if err := db.Raw(
		"SELECT COLLATION_NAME FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?",
		"organizations", "name_normalized",
	).Scan(&collation).Error; err != nil {
		return false, err
	}
	return strings.EqualFold(collation, organizationNameMySQLCollation), nil
}

func ensureOrganizationNameNormalizedComparison(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "mysql" {
		return nil
	}
	return db.Exec(
		fmt.Sprintf("ALTER TABLE ? MODIFY COLUMN ? varchar(128) CHARACTER SET utf8mb4 COLLATE %s NOT NULL", organizationNameMySQLCollation),
		clause.Table{Name: "organizations"}, clause.Column{Name: "name_normalized"},
	).Error
}
