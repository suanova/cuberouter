package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newPerfTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&PerfMetric{}, &CapacityMetric{}))
	return db
}

func TestPerfMetricUpsertAddsHistogramCells(t *testing.T) {
	db := newPerfTestDB(t)
	oldDB := DB
	DB = db
	defer func() { DB = oldDB }()

	var hist [13]int64
	hist[4] = 3 // 3 个 ~1.5s 样本
	m := &PerfMetric{ModelName: "gpt-x", Group: "default", ChannelId: 7, ChannelName: "up-7", BucketTs: 1000, RequestCount: 3, TotalLatencyMs: 4500}
	m.SetLatHist(hist)
	require.NoError(t, UpsertPerfMetric(m))

	m2 := &PerfMetric{ModelName: "gpt-x", Group: "default", ChannelId: 7, BucketTs: 1000, RequestCount: 2, TotalLatencyMs: 3000}
	var hist2 [13]int64
	hist2[4] = 2
	m2.SetLatHist(hist2)
	require.NoError(t, UpsertPerfMetric(m2))

	rows, err := GetPerfMetrics("gpt-x", "", 0, 5000)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	got := rows[0].LatHist()
	assert.Equal(t, int64(5), got[4])
	assert.Equal(t, int64(5), rows[0].RequestCount)
	assert.Equal(t, int64(7), rows[0].ChannelId)
}

func TestCapacityMetricUpsertPeakIsMax(t *testing.T) {
	db := newPerfTestDB(t)
	oldDB := DB
	DB = db
	defer func() { DB = oldDB }()

	require.NoError(t, UpsertCapacityMetric(&CapacityMetric{BucketTs: 2000, Attempts: 10, Rejected503: 1, InflightPeak: 8}))
	require.NoError(t, UpsertCapacityMetric(&CapacityMetric{BucketTs: 2000, Attempts: 5, Rejected503: 2, InflightPeak: 3}))
	require.NoError(t, UpsertCapacityMetric(&CapacityMetric{BucketTs: 2000, Attempts: 7, Rejected503: 0, InflightPeak: 12}))

	rows, err := GetCapacityMetrics(0, 5000)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(22), rows[0].Attempts)
	assert.Equal(t, int64(3), rows[0].Rejected503)
	assert.Equal(t, int64(12), rows[0].InflightPeak) // 峰值取 max，非累加

	require.NoError(t, DeleteCapacityBefore(1500))
	rows, err = GetCapacityMetrics(0, 5000)
	require.NoError(t, err)
	require.Len(t, rows, 1) // 2000 > 1500，未删
	require.NoError(t, DeleteCapacityBefore(2500))
	rows, err = GetCapacityMetrics(0, 5000)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestDropLegacyPerfUniqueIndexIdempotent(t *testing.T) {
	db := newPerfTestDB(t)
	oldDB := DB
	DB = db
	defer func() { DB = oldDB }()

	// 全新库从未创建过旧索引（新 schema 只带 idx_perf_model_group_channel_bucket），
	// 删除须无错返回——MySQL 的 DROP INDEX 无 IF EXISTS，靠存在性检查兜底。
	require.NoError(t, dropLegacyPerfUniqueIndex())

	// 模拟旧版本残留：注入旧唯一索引后再删，须成功且索引确实消失。
	require.NoError(t, db.Exec("CREATE INDEX idx_perf_model_group_bucket ON perf_metrics(model_name, `group`, bucket_ts)").Error)
	require.NoError(t, dropLegacyPerfUniqueIndex())

	var count int64
	require.NoError(t, db.Raw("SELECT count(*) FROM sqlite_master WHERE type='index' AND name='idx_perf_model_group_bucket'").Scan(&count).Error)
	assert.Zero(t, count)
}

func TestCapacityUpsertAdditiveWhenNoConflictAddsRow(t *testing.T) {
	// 无冲突行 → 新建：跨两个不同 bucket 各成一行，互不累加。
	db := newPerfTestDB(t)
	oldDB := DB
	DB = db
	defer func() { DB = oldDB }()

	require.NoError(t, UpsertCapacityMetric(&CapacityMetric{BucketTs: 1000, Attempts: 3, Rejected503: 1, InflightPeak: 5}))
	require.NoError(t, UpsertCapacityMetric(&CapacityMetric{BucketTs: 2000, Attempts: 4, Rejected503: 0, InflightPeak: 9}))

	rows, err := GetCapacityMetrics(0, 5000)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, int64(1000), rows[0].BucketTs)
	assert.Equal(t, int64(3), rows[0].Attempts)
	assert.Equal(t, int64(1), rows[0].Rejected503)
	assert.Equal(t, int64(5), rows[0].InflightPeak)
	assert.Equal(t, int64(2000), rows[1].BucketTs)
	assert.Equal(t, int64(4), rows[1].Attempts)
	assert.Equal(t, int64(0), rows[1].Rejected503)
	assert.Equal(t, int64(9), rows[1].InflightPeak)
}
