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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type quotaDataSQLRecorder struct {
	sql string
}

func (r *quotaDataSQLRecorder) LogMode(logger.LogLevel) logger.Interface { return r }
func (r *quotaDataSQLRecorder) Info(context.Context, string, ...any)     {}
func (r *quotaDataSQLRecorder) Warn(context.Context, string, ...any)     {}
func (r *quotaDataSQLRecorder) Error(context.Context, string, ...any)    {}

func (r *quotaDataSQLRecorder) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	r.sql, _ = fc()
}

// TestUpsertQuotaDataPostgresQualifiesConflictUpdateColumns 用 DryRun 抓生成的 SQL，
// 不连真实数据库。
//
// PostgreSQL 要求 ON CONFLICT DO UPDATE 的 SET 右侧列引用带表名限定，写成裸
// `count + ?` 会直接报 ambiguous column；SQLite/MySQL 对此宽容，只有真实 PG 会炸，
// 所以这条断言必须靠方言特定的 SQL 生成路径来守。改动 upsertQuotaData 的累加表达式
// 时这个用例是第一道警报。
func TestUpsertQuotaDataPostgresQualifiesConflictUpdateColumns(t *testing.T) {
	recorder := &quotaDataSQLRecorder{}
	db, err := gorm.Open(postgres.Open("host=localhost user=test dbname=test sslmode=disable"), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		Logger:                 recorder,
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)

	previousDB := DB
	DB = db
	t.Cleanup(func() { DB = previousDB })

	require.NoError(t, upsertQuotaData(&QuotaData{
		UserID:    1,
		Username:  "member",
		ModelName: "gpt-test",
		CreatedAt: 3600,
		Count:     1,
		Quota:     2,
		TokenUsed: 3,
	}))
	require.Contains(t, recorder.sql, `"quota_data"."count"`)
	require.Contains(t, recorder.sql, `"quota_data"."quota"`)
	require.Contains(t, recorder.sql, `"quota_data"."token_used"`)
}
