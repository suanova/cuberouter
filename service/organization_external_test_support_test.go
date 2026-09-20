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

// organizationExternalConcurrencyTarget 选出并发用例要连的库。
//
// DSN 沿用仓库既有约定（TEST_MYSQL_DSN / TEST_POSTGRES_DSN，同 model 包的迁移用例），
// 方言由变量名决定，所以正常情况下不需要再配一个类型开关。只有两个变量同时配置、
// 无法从名字判断用哪个时，才要求 ORGANIZATION_TEST_DB_TYPE 显式指定。
func organizationExternalConcurrencyTarget(t *testing.T) (string, string) {
	t.Helper()
	mysqlDSN := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	postgresDSN := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	switch {
	case mysqlDSN != "" && postgresDSN != "":
		switch strings.ToLower(strings.TrimSpace(os.Getenv("ORGANIZATION_TEST_DB_TYPE"))) {
		case "mysql":
			return "mysql", mysqlDSN
		case "postgres", "postgresql":
			return "postgres", postgresDSN
		default:
			t.Skip("TEST_MYSQL_DSN and TEST_POSTGRES_DSN are both set; " +
				"set ORGANIZATION_TEST_DB_TYPE=mysql or postgres to pick one")
		}
	case mysqlDSN != "":
		return "mysql", mysqlDSN
	case postgresDSN != "":
		return "postgres", postgresDSN
	}
	t.Skip("set TEST_MYSQL_DSN or TEST_POSTGRES_DSN to run the organization concurrency tests")
	return "", ""
}

// 并发用例要的是真正的行级/唯一约束竞争，SQLite 的单写者模型给不出来，
// 所以必须跑在真实 MySQL/PostgreSQL 上。
//
// 这些用例会建表、插数据、最后删除自己造的行。为了不误伤别人指向的库，
// 再加一道显式开关：只有 ORGANIZATION_TEST_ALLOW_DESTRUCTIVE=1 时才真正执行。
func setupOrganizationExternalConcurrencyDB(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	databaseType, dsn := organizationExternalConcurrencyTarget(t)
	if os.Getenv("ORGANIZATION_TEST_ALLOW_DESTRUCTIVE") != "1" {
		t.Skip("set ORGANIZATION_TEST_ALLOW_DESTRUCTIVE=1 to run the destructive organization concurrency tests")
	}

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
