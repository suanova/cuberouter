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

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRecordConsumeLogWritesStructuredUsageFields(t *testing.T) {
	db := newBillingTestDB(t)
	orig := LOG_DB
	LOG_DB = db
	t.Cleanup(func() { LOG_DB = orig })

	RecordConsumeLog(&gin.Context{}, 7, RecordConsumeLogParams{
		ModelName:   "gpt-4o",
		Quota:       10,
		CacheTokens: 42,
	})
	var got Log
	require.NoError(t, db.Where("user_id = ?", 7).First(&got).Error)
	assert.Equal(t, 42, got.CacheTokens)
	assert.Equal(t, LogUsageSourceText, got.UsageSource)

	RecordConsumeLog(&gin.Context{}, 7, RecordConsumeLogParams{
		ModelName:   "gpt-4o",
		Quota:       1,
		UsageSource: LogUsageSourceFee,
	})
	got = Log{}
	require.NoError(t, db.Where("user_id = ? AND quota = 1", 7).First(&got).Error)
	assert.Equal(t, LogUsageSourceFee, got.UsageSource)
}

func TestRecordTaskBillingLogWritesStructuredUsageFields(t *testing.T) {
	db := newBillingTestDB(t)
	orig := LOG_DB
	LOG_DB = db
	t.Cleanup(func() { LOG_DB = orig })

	RecordTaskBillingLog(RecordTaskBillingLogParams{UserId: 7, LogType: LogTypeConsume, ModelName: "task-model", Quota: 5})
	var got Log
	require.NoError(t, db.Where("user_id = ?", 7).First(&got).Error)
	assert.Equal(t, LogUsageSourceTask, got.UsageSource)

	RecordTaskBillingLog(RecordTaskBillingLogParams{UserId: 7, LogType: LogTypeRefund, ModelName: "mj_imagine", Quota: 5, UsageSource: LogUsageSourceMJ})
	got = Log{}
	require.NoError(t, db.Where("user_id = ? AND type = ?", 7, LogTypeRefund).First(&got).Error)
	assert.Equal(t, LogUsageSourceMJ, got.UsageSource)
}

func TestEnsureSelfUsageLogIndex(t *testing.T) {
	db := newBillingTestDB(t)
	require.NoError(t, db.Migrator().DropIndex(&Log{}, "idx_logs_actor_created_at"))
	require.NoError(t, ensureSelfUsageLogIndexes(db))
	assert.True(t, db.Migrator().HasIndex(&Log{}, "idx_logs_actor_created_at"))
}

// TestEnsureSelfUsageLogIndexConfiguredDatabases exercises the explicit index
// creation on real MySQL and PostgreSQL instances, where AutoMigrate alone is
// not proof that the upgrade path repairs a missing index.
func TestEnsureSelfUsageLogIndexConfiguredDatabases(t *testing.T) {
	cases := []struct {
		name string
		env  string
		open func(string) (*gorm.DB, error)
	}{
		{
			name: "mysql",
			env:  "TEST_MYSQL_DSN",
			open: func(dsn string) (*gorm.DB, error) {
				return gorm.Open(mysql.Open(dsn), &gorm.Config{})
			},
		},
		{
			name: "postgres",
			env:  "TEST_POSTGRES_DSN",
			open: func(dsn string) (*gorm.DB, error) {
				return gorm.Open(postgres.New(postgres.Config{
					DSN:                  dsn,
					PreferSimpleProtocol: true,
				}), &gorm.Config{})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(tc.env))
			if dsn == "" {
				t.Skip(tc.env + " is not configured")
			}
			db, err := tc.open(dsn)
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			require.NoError(t, db.AutoMigrate(&Log{}))
			if db.Migrator().HasIndex(&Log{}, "idx_logs_actor_created_at") {
				require.NoError(t, db.Migrator().DropIndex(&Log{}, "idx_logs_actor_created_at"))
			}

			require.NoError(t, ensureSelfUsageLogIndexes(db))
			assert.True(t, db.Migrator().HasIndex(&Log{}, "idx_logs_actor_created_at"))
		})
	}
}
