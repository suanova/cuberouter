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
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func testDefaultMemberTokenQuotaColumnMigration(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&Organization{}))

	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000_000)
	organization := Organization{
		Name:                    "Legacy Token Default " + suffix,
		Slug:                    "legacy-token-default-" + suffix,
		CreatedBy:               1,
		DefaultMemberTokenQuota: 999,
	}
	require.NoError(t, db.Create(&organization).Error)
	require.NoError(t, db.Migrator().DropColumn(&Organization{}, "default_member_token_quota"))
	require.False(t, db.Migrator().HasColumn(&Organization{}, "default_member_token_quota"))

	// 模拟旧库升级：列被 AutoMigrate 加回，存量行读 0；再次启动不重复 DDL。
	require.NoError(t, db.AutoMigrate(&Organization{}))
	require.NoError(t, db.AutoMigrate(&Organization{}))
	require.True(t, db.Migrator().HasColumn(&Organization{}, "default_member_token_quota"))

	var preserved Organization
	require.NoError(t, db.Where("id = ?", organization.Id).First(&preserved).Error)
	require.Equal(t, organization.Name, preserved.Name)
	require.Zero(t, preserved.DefaultMemberTokenQuota)
}

func TestMigrateOrganizationDefaultMemberTokenQuotaSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/org-token-default-migration.db"), &gorm.Config{})
	require.NoError(t, err)
	testDefaultMemberTokenQuotaColumnMigration(t, db)
}

func TestMigrateOrganizationDefaultMemberTokenQuotaMySQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not configured")
	}
	testDefaultMemberTokenQuotaColumnMigration(t, mysqlScratchDatabase(t, dsn))
}

func TestMigrateOrganizationDefaultMemberTokenQuotaPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not configured")
	}
	testDefaultMemberTokenQuotaColumnMigration(t, postgresScratchSchema(t, dsn))
}
