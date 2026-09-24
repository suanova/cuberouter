package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newBillingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedisEnabled })
	if commonGroupCol == "" {
		commonGroupCol = "`group`"
		commonKeyCol = "`key`"
		commonTrueVal = "1"
		commonFalseVal = "0"
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err, "open test db")
	require.NoError(t, db.AutoMigrate(&Log{}), "migrate log")
	return db
}

func TestGetUserBillingAgg(t *testing.T) {
	db := newBillingTestDB(t)
	origLOG_DB := LOG_DB
	LOG_DB = db
	defer func() { LOG_DB = origLOG_DB }()

	loc := time.Local
	// day1 = 2026-07-01 local, day2 = 2026-07-02 local
	day1 := time.Date(2026, 7, 1, 10, 0, 0, 0, loc).Unix()
	day2 := time.Date(2026, 7, 2, 15, 30, 0, 0, loc).Unix()
	startTs := time.Date(2026, 7, 1, 0, 0, 0, 0, loc).Unix()
	endTs := time.Date(2026, 7, 2, 23, 59, 59, 0, loc).Unix()

	// 插入消费记录(type=2): gpt-4o 两条(day1, day2),claude 一条(day1)
	logs := []Log{
		{UserId: 10, Username: "alice", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: day1, PromptTokens: 100, CompletionTokens: 50, Quota: 1000},
		{UserId: 10, Username: "alice", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: day2, PromptTokens: 200, CompletionTokens: 100, Quota: 2000},
		{UserId: 10, Username: "alice", ModelName: "claude", Type: LogTypeConsume, CreatedAt: day1, PromptTokens: 300, CompletionTokens: 0, Quota: 3000},
		// 非消费记录应被过滤
		{UserId: 10, Username: "alice", ModelName: "gpt-4o", Type: LogTypeTopup, CreatedAt: day1, Quota: 9999},
		// 其它用户应被过滤(同名不同 ID 也应过滤)
		{UserId: 11, Username: "bob", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: day1, Quota: 9999},
	}
	for i := range logs {
		require.NoError(t, db.Create(&logs[i]).Error, "create log")
	}

	// 汇总(按模型)
	summary, err := GetUserBillingAgg(10, startTs, endTs, false)
	assert.NoError(t, err)
	byModel := map[string]BillingAggRow{}
	for _, r := range summary {
		byModel[r.ModelName] = r
	}
	assert.Equal(t, 300, byModel["gpt-4o"].PromptTokens) // 100(day1)+200(day2)
	assert.Equal(t, 150, byModel["gpt-4o"].CompletionTokens)
	assert.Equal(t, 3000, byModel["gpt-4o"].Quota)
	assert.Equal(t, 2, byModel["gpt-4o"].RequestCount)
	assert.Equal(t, 300, byModel["claude"].PromptTokens)
	assert.Equal(t, 3000, byModel["claude"].Quota)
	assert.Equal(t, int64(0), byModel["gpt-4o"].DayKey) // 汇总无 day_key

	// 按日(按模型)
	daily, err := GetUserBillingAgg(10, startTs, endTs, true)
	assert.NoError(t, err)
	// 期望 3 行:gpt-4o@day1, claude@day1, gpt-4o@day2
	assert.Len(t, daily, 3)
	dateOf := func(dayKey int64) string { return BillingDayKeyToDate(dayKey) }
	seen := map[string]int{}
	for _, r := range daily {
		key := r.ModelName + "@" + dateOf(r.DayKey)
		seen[key]++
	}
	assert.Equal(t, 1, seen["gpt-4o@2026-07-01"])
	assert.Equal(t, 1, seen["gpt-4o@2026-07-02"])
	assert.Equal(t, 1, seen["claude@2026-07-01"])

	// 空结果返回非 nil 空切片
	empty, err := GetUserBillingAgg(999, startTs, endTs, false)
	assert.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Len(t, empty, 0)

	// 非正数 ID 应报错
	_, err = GetUserBillingAgg(0, startTs, endTs, false)
	assert.Error(t, err)
}

func TestGetReconciliationReport(t *testing.T) {
	db := newBillingTestDB(t)
	origLOG_DB := LOG_DB
	LOG_DB = db
	defer func() { LOG_DB = origLOG_DB }()

	loc := time.Local
	day1 := time.Date(2026, 7, 1, 10, 0, 0, 0, loc).Unix()
	startTs := time.Date(2026, 7, 1, 0, 0, 0, 0, loc).Unix()
	endTs := time.Date(2026, 7, 2, 23, 59, 59, 0, loc).Unix()

	// alice: 2 个模型;bob: 1 个模型;非消费记录应过滤
	logs := []Log{
		{UserId: 10, Username: "alice", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: day1, PromptTokens: 100, CompletionTokens: 50, Quota: 1000},
		{UserId: 10, Username: "alice", ModelName: "claude", Type: LogTypeConsume, CreatedAt: day1, PromptTokens: 200, Quota: 2000},
		{UserId: 11, Username: "bob", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: day1, Quota: 5000},
		{UserId: 10, Username: "alice", ModelName: "gpt-4o", Type: LogTypeTopup, CreatedAt: day1, Quota: 9999},
	}
	for i := range logs {
		require.NoError(t, db.Create(&logs[i]).Error, "create log")
	}

	rows, err := GetReconciliationReport(startTs, endTs)
	require.NoError(t, err, "reconciliation query")
	require.Len(t, rows, 2, "want exactly two user rows before indexed access")
	// 按 quota 降序:bob(5000) 在前,alice(3000) 在后
	assert.Equal(t, 11, rows[0].UserId)
	assert.Equal(t, 5000, rows[0].Quota)
	assert.Equal(t, 1, rows[0].ModelCount)
	assert.Equal(t, 10, rows[1].UserId)
	assert.Equal(t, 3000, rows[1].Quota)
	assert.Equal(t, 2, rows[1].ModelCount) // gpt-4o + claude
	assert.Equal(t, 300, rows[1].PromptTokens)
	assert.Equal(t, 50, rows[1].CompletionTokens)

	// 空区间返回非 nil 空切片
	empty, err := GetReconciliationReport(0, 0)
	assert.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Len(t, empty, 0)
}
