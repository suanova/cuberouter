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
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// 并发用例要的是真正的行级/唯一约束竞争，SQLite 的单写者模型给不出来，
// 所以必须跑在真实 MySQL/PostgreSQL 上。DSN 沿用仓库既有约定
// （TEST_MYSQL_DSN / TEST_POSTGRES_DSN），未配置就跳过。
//
// 这些用例会建表、插数据、最后删除自己造的行。为了不误伤别人指向的库，
// 再加一道显式开关：只有 ORGANIZATION_TEST_ALLOW_DESTRUCTIVE=1 时才真正执行。
func setupOrganizationExternalConcurrencyDB(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	databaseType := strings.TrimSpace(os.Getenv("ORGANIZATION_TEST_DB_TYPE"))
	dsn := strings.TrimSpace(os.Getenv("ORGANIZATION_TEST_DSN"))
	if databaseType == "" || dsn == "" {
		t.Skip("set ORGANIZATION_TEST_DB_TYPE (mysql/postgres) and ORGANIZATION_TEST_DSN to run the organization concurrency tests")
	}
	if os.Getenv("ORGANIZATION_TEST_ALLOW_DESTRUCTIVE") != "1" {
		t.Skip("set ORGANIZATION_TEST_ALLOW_DESTRUCTIVE=1 to run the destructive organization concurrency tests")
	}

	databaseType = strings.ToLower(databaseType)
	var dialector gorm.Dialector
	switch databaseType {
	case "mysql":
		dialector = mysql.Open(dsn)
	case "postgres", "postgresql":
		dialector = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
	default:
		t.Fatalf("unsupported ORGANIZATION_TEST_DB_TYPE %q", databaseType)
	}
	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(4)

	serviceTestDBMu.Lock()
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldMainType, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
	oldRedisEnabled := common.RedisEnabled
	model.DB, model.LOG_DB = db, db
	common.RedisEnabled = false
	if databaseType == "mysql" {
		common.SetDatabaseTypes(common.DatabaseTypeMySQL, common.DatabaseTypeMySQL)
	} else {
		common.SetDatabaseTypes(common.DatabaseTypePostgreSQL, common.DatabaseTypePostgreSQL)
	}
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.SetDatabaseTypes(oldMainType, oldLogType)
		common.RedisEnabled = oldRedisEnabled
		serviceTestDBMu.Unlock()
		require.NoError(t, sqlDB.Close())
	})
	return db, databaseType
}
