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
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestBatchUpdateRetryOnExternalDatabases 是批量落库重试的三库验证
// （AGENTS.md：涉及 DB 写行为的改动须在真实 MySQL / PostgreSQL 上验证，SQLite 由
// TestBatchUpdateRetriesFailedEntries 覆盖）：失败一轮 → 增量仍在缓冲 → 引擎恢复后
// 恰好写一次。
//
// 用例通过临时重命名 users / tokens 表注入失败（不依赖任何方言的错误码），因此必须
// 指向一次性库；未设置 BATCH_UPDATE_TEST_ALLOW_DESTRUCTIVE=1 时跳过，避免误伤。
func TestBatchUpdateRetryOnExternalDatabases(t *testing.T) {
	cases := []struct {
		name  string
		env   string
		dbTyp common.DatabaseType
		open  func(dsn string) (*gorm.DB, error)
	}{
		{
			name:  "mysql",
			env:   "TEST_MYSQL_DSN",
			dbTyp: common.DatabaseTypeMySQL,
			open: func(dsn string) (*gorm.DB, error) {
				return gorm.Open(mysql.Open(dsn), &gorm.Config{})
			},
		},
		{
			name:  "postgres",
			env:   "TEST_POSTGRES_DSN",
			dbTyp: common.DatabaseTypePostgreSQL,
			open: func(dsn string) (*gorm.DB, error) {
				return gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), &gorm.Config{})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(tc.env))
			if dsn == "" {
				t.Skipf("%s is not configured", tc.env)
			}
			require.Equal(t, "1", os.Getenv("BATCH_UPDATE_TEST_ALLOW_DESTRUCTIVE"),
				"用例会重命名 users / tokens 表，确认指向一次性库后再打开")

			db, err := tc.open(dsn)
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })

			originalDB, originalLogDB := DB, LOG_DB
			originalMainType, originalLogType := common.MainDatabaseType(), common.LogDatabaseType()
			t.Cleanup(func() {
				DB, LOG_DB = originalDB, originalLogDB
				common.SetDatabaseTypes(originalMainType, originalLogType)
				initCol()
			})
			DB, LOG_DB = db, db
			common.SetDatabaseTypes(tc.dbTyp, tc.dbTyp)
			initCol()

			require.NoError(t, db.AutoMigrate(&User{}, &Token{}))
			resetBatchUpdateTestState(t)

			user := createReserveTestUser(t, 1000)
			token := createReserveTestToken(t, 1000)
			addNewRecord(BatchUpdateTypeUserQuota, user.Id, -100)
			addNewRecord(BatchUpdateTypeTokenQuota, token.Id, -100)

			require.NoError(t, DB.Exec("ALTER TABLE users RENAME TO users_batch_hidden").Error)
			require.NoError(t, DB.Exec("ALTER TABLE tokens RENAME TO tokens_batch_hidden").Error)
			t.Cleanup(func() {
				// 断言失败也要把表名恢复回去，避免污染同一库上的后续用例。
				DB.Exec("ALTER TABLE users_batch_hidden RENAME TO users")
				DB.Exec("ALTER TABLE tokens_batch_hidden RENAME TO tokens")
			})

			batchUpdate()

			assert.Equal(t, -100, batchStoreDelta(t, BatchUpdateTypeUserQuota, user.Id), "失败条目必须回填缓冲")
			assert.Equal(t, -100, batchStoreDelta(t, BatchUpdateTypeTokenQuota, token.Id), "失败条目必须回填缓冲")

			require.NoError(t, DB.Exec("ALTER TABLE users_batch_hidden RENAME TO users").Error)
			require.NoError(t, DB.Exec("ALTER TABLE tokens_batch_hidden RENAME TO tokens").Error)

			batchUpdate()
			batchUpdate() // 再跑一轮：不得重复写

			assert.Equal(t, 900, getUserQuotaFromDB(t, user.Id), "恢复后应恰好写一次")
			assert.Equal(t, 900, getTokenFromDB(t, token.Id).RemainQuota, "恢复后应恰好写一次")
			assert.Zero(t, batchStoreLen(t, BatchUpdateTypeUserQuota), "写成功后缓冲应清空")
			assert.Zero(t, batchStoreLen(t, BatchUpdateTypeTokenQuota), "写成功后缓冲应清空")
		})
	}
}
