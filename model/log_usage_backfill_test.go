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
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestClassifyLogUsageSource(t *testing.T) {
	assert.Equal(t, LogUsageSourceMJ, ClassifyLogUsageSource("mj_imagine", nil))
	assert.Equal(t, LogUsageSourceMJ, ClassifyLogUsageSource("swap_face", nil))
	assert.Equal(t, LogUsageSourceTask, ClassifyLogUsageSource("wan3.0-video", map[string]any{"is_task": true}))
	assert.Equal(t, LogUsageSourceTask, ClassifyLogUsageSource("wan3.0-video", map[string]any{"pre_consumed_quota": 1.0}))
	assert.Equal(t, LogUsageSourceTask, ClassifyLogUsageSource("wan3.0-video", map[string]any{"task_id": "t1"}))
	assert.Equal(t, LogUsageSourceFee, ClassifyLogUsageSource("gpt-4o", map[string]any{"violation_fee": true}))
	assert.Equal(t, LogUsageSourceText, ClassifyLogUsageSource("gpt-4o", map[string]any{"cache_tokens": 12.0}))
}

func TestBackfillLogUsageFieldsIsIdempotent(t *testing.T) {
	db := newBillingTestDB(t)
	origLog, origDB := LOG_DB, DB
	origOptions := common.OptionMap
	LOG_DB, DB = db, db
	common.OptionMap = map[string]string{}
	require.NoError(t, db.AutoMigrate(&Option{}))
	t.Cleanup(func() { LOG_DB, DB, common.OptionMap = origLog, origDB, origOptions })

	rows := []Log{
		{UserId: 1, Type: LogTypeConsume, ModelName: "gpt-4o", Other: `{"cache_tokens":15}`},
		{UserId: 1, Type: LogTypeConsume, ModelName: "wan3.0-video", Other: `{"is_task":true}`},
		{UserId: 1, Type: LogTypeRefund, ModelName: "wan3.0-video", Other: `{"task_id":"t1","reason":"x"}`},
		{UserId: 1, Type: LogTypeConsume, ModelName: "mj_imagine", Other: `{}`},
	}
	require.NoError(t, db.Create(&rows).Error)

	require.NoError(t, BackfillLogUsageFields())
	var got []Log
	require.NoError(t, db.Order("id").Find(&got).Error)
	assert.Equal(t, 15, got[0].CacheTokens)
	assert.Equal(t, LogUsageSourceText, got[0].UsageSource)
	assert.Equal(t, LogUsageSourceTask, got[1].UsageSource)
	assert.Equal(t, LogUsageSourceTask, got[2].UsageSource)
	assert.Equal(t, LogUsageSourceMJ, got[3].UsageSource)
	assert.True(t, LogUsageBackfillComplete())

	// A second run must not change or fail anything.
	require.NoError(t, BackfillLogUsageFields())
}

func TestStartLogUsageBackfillHoldsLockUntilBackfillFinishes(t *testing.T) {
	db := newBillingTestDB(t)
	origLog, origDB := LOG_DB, DB
	origOptions := common.OptionMap
	origRedis, origRDB := common.RedisEnabled, common.RDB
	origMaster := common.IsMasterNode
	LOG_DB, DB = db, db
	common.OptionMap = map[string]string{}
	common.IsMasterNode = true
	require.NoError(t, db.AutoMigrate(&Option{}))

	mr := miniredis.RunT(t)
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		LOG_DB, DB, common.OptionMap = origLog, origDB, origOptions
		_ = common.RDB.Close()
		common.RedisEnabled, common.RDB = origRedis, origRDB
		common.IsMasterNode = origMaster
	})

	// Block the backfill's first query so the goroutine stays inside the
	// critical section while the test inspects the lock.
	entered := make(chan struct{})
	release := make(chan struct{})
	var enteredOnce, releaseOnce sync.Once
	closeRelease := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(closeRelease)
	db.Callback().Query().Before("gorm:query").Register("test:block_log_query", func(tx *gorm.DB) {
		if tx.Statement.Table == "logs" {
			enteredOnce.Do(func() { close(entered) })
			<-release
		}
	})

	StartLogUsageBackfill()

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		require.Fail(t, "backfill query did not start")
	}
	assert.True(t, mr.Exists("log:usage-backfill:lock"), "lock must be held while the backfill runs")

	closeRelease()
	require.Eventually(t, func() bool {
		return !mr.Exists("log:usage-backfill:lock")
	}, 5*time.Second, 10*time.Millisecond, "lock must be released after the backfill finishes")
}

func TestBackfillClickHouseLogUsageFieldsWaitsForMutation(t *testing.T) {
	db := newBillingTestDB(t)
	origLog := LOG_DB
	origType := common.LogDatabaseType()
	LOG_DB = db
	common.SetLogDatabaseType(common.DatabaseTypeClickHouse)
	t.Cleanup(func() {
		LOG_DB = origLog
		common.SetLogDatabaseType(origType)
	})

	// 捕获实际发给 ClickHouse 的 SQL 并终止执行：没有 mutations_sync 时
	// ALTER TABLE UPDATE 是异步的，usage_source 的完成标记会先于回填落下。
	var captured string
	db.Callback().Raw().Before("gorm:raw").Register("test:capture_backfill_sql", func(tx *gorm.DB) {
		captured = tx.Statement.SQL.String()
		_ = tx.AddError(errors.New("captured"))
	})

	require.Error(t, backfillClickHouseLogUsageFields())
	assert.Contains(t, captured, "SETTINGS mutations_sync = 2")
}

func TestRunLogUsageBackfillWithRetryRetriesTransientFailures(t *testing.T) {
	origOptions := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() { common.OptionMap = origOptions })

	calls := 0
	err := runLogUsageBackfillWithRetry(func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	}, []time.Duration{time.Millisecond, time.Millisecond})
	require.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestRunLogUsageBackfillWithRetryStopsAfterDelays(t *testing.T) {
	origOptions := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() { common.OptionMap = origOptions })

	calls := 0
	err := runLogUsageBackfillWithRetry(func() error {
		calls++
		return errors.New("still broken")
	}, []time.Duration{time.Millisecond})
	require.Error(t, err)
	assert.Equal(t, 2, calls)
}

func TestReleaseLogUsageBackfillLockOnlyDeletesOwnLock(t *testing.T) {
	mr := miniredis.RunT(t)
	origRedis, origRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RedisEnabled, common.RDB = origRedis, origRDB
	})

	require.NoError(t, mr.Set("log:usage-backfill:lock", "other-node"))
	releaseLogUsageBackfillLock("this-node")
	assert.True(t, mr.Exists("log:usage-backfill:lock"), "must not delete another node's lock")

	require.NoError(t, mr.Set("log:usage-backfill:lock", "this-node"))
	releaseLogUsageBackfillLock("this-node")
	assert.False(t, mr.Exists("log:usage-backfill:lock"))
}
