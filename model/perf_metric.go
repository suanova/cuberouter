package model

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PerfMetric stores aggregated relay performance metrics for the model square.
type PerfMetric struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	ModelName      string `json:"model_name" gorm:"size:128;uniqueIndex:idx_perf_model_group_channel_bucket,priority:1"`
	Group          string `json:"group" gorm:"column:group;size:64;uniqueIndex:idx_perf_model_group_channel_bucket,priority:2"`
	ChannelId      int64  `json:"channel_id" gorm:"index;uniqueIndex:idx_perf_model_group_channel_bucket,priority:3"`
	ChannelName    string `json:"channel_name" gorm:"size:128"`
	BucketTs       int64  `json:"bucket_ts" gorm:"uniqueIndex:idx_perf_model_group_channel_bucket,priority:4;index:idx_perf_bucket_ts"`
	RequestCount   int64  `json:"-" gorm:"default:0"`
	SuccessCount   int64  `json:"-" gorm:"default:0"`
	TotalLatencyMs int64  `json:"-" gorm:"default:0"`
	TtftSumMs      int64  `json:"-" gorm:"default:0"`
	TtftCount      int64  `json:"-" gorm:"default:0"`
	OutputTokens   int64  `json:"-" gorm:"default:0"`
	GenerationMs   int64  `json:"-" gorm:"default:0"`
	// LatB0..LatB12 / TtftB0..TtftB12 为延迟与首字节延迟的直方图单元计数，
	// 列名 lat_b{i} / ttft_b{i}。单元数 13（0..12）与 pkg/perf_metrics/hist.go
	// 的 histCellCount 同步；model 层不得 import perfmetrics（perfmetrics 依赖
	// model），故此处以字面量 13 维护并交叉引用，改动需两边同步。
	LatB0   int64 `json:"-" gorm:"default:0"`
	LatB1   int64 `json:"-" gorm:"default:0"`
	LatB2   int64 `json:"-" gorm:"default:0"`
	LatB3   int64 `json:"-" gorm:"default:0"`
	LatB4   int64 `json:"-" gorm:"default:0"`
	LatB5   int64 `json:"-" gorm:"default:0"`
	LatB6   int64 `json:"-" gorm:"default:0"`
	LatB7   int64 `json:"-" gorm:"default:0"`
	LatB8   int64 `json:"-" gorm:"default:0"`
	LatB9   int64 `json:"-" gorm:"default:0"`
	LatB10  int64 `json:"-" gorm:"default:0"`
	LatB11  int64 `json:"-" gorm:"default:0"`
	LatB12  int64 `json:"-" gorm:"default:0"`
	TtftB0  int64 `json:"-" gorm:"default:0"`
	TtftB1  int64 `json:"-" gorm:"default:0"`
	TtftB2  int64 `json:"-" gorm:"default:0"`
	TtftB3  int64 `json:"-" gorm:"default:0"`
	TtftB4  int64 `json:"-" gorm:"default:0"`
	TtftB5  int64 `json:"-" gorm:"default:0"`
	TtftB6  int64 `json:"-" gorm:"default:0"`
	TtftB7  int64 `json:"-" gorm:"default:0"`
	TtftB8  int64 `json:"-" gorm:"default:0"`
	TtftB9  int64 `json:"-" gorm:"default:0"`
	TtftB10 int64 `json:"-" gorm:"default:0"`
	TtftB11 int64 `json:"-" gorm:"default:0"`
	TtftB12 int64 `json:"-" gorm:"default:0"`
}

func (PerfMetric) TableName() string {
	return "perf_metrics"
}

// LatB 返回第 i 个延迟直方图单元计数（0..12，13 与 pkg/perf_metrics/hist.go 的
// histCellCount 同步）。越界返回 0；调用方约定只传 0..12。
func (p *PerfMetric) LatB(i int) int64 {
	switch i {
	case 0:
		return p.LatB0
	case 1:
		return p.LatB1
	case 2:
		return p.LatB2
	case 3:
		return p.LatB3
	case 4:
		return p.LatB4
	case 5:
		return p.LatB5
	case 6:
		return p.LatB6
	case 7:
		return p.LatB7
	case 8:
		return p.LatB8
	case 9:
		return p.LatB9
	case 10:
		return p.LatB10
	case 11:
		return p.LatB11
	case 12:
		return p.LatB12
	default:
		return 0
	}
}

// TtftB 返回第 i 个首字节延迟直方图单元计数（0..12，13 与 histCellCount 同步）。
func (p *PerfMetric) TtftB(i int) int64 {
	switch i {
	case 0:
		return p.TtftB0
	case 1:
		return p.TtftB1
	case 2:
		return p.TtftB2
	case 3:
		return p.TtftB3
	case 4:
		return p.TtftB4
	case 5:
		return p.TtftB5
	case 6:
		return p.TtftB6
	case 7:
		return p.TtftB7
	case 8:
		return p.TtftB8
	case 9:
		return p.TtftB9
	case 10:
		return p.TtftB10
	case 11:
		return p.TtftB11
	case 12:
		return p.TtftB12
	default:
		return 0
	}
}

// SetLatHist 将 13 个延迟直方图单元计数写入 LatB0..LatB12
// （单元数 13 与 pkg/perf_metrics/hist.go 的 histCellCount 同步）。
func (p *PerfMetric) SetLatHist(cells [13]int64) {
	p.LatB0 = cells[0]
	p.LatB1 = cells[1]
	p.LatB2 = cells[2]
	p.LatB3 = cells[3]
	p.LatB4 = cells[4]
	p.LatB5 = cells[5]
	p.LatB6 = cells[6]
	p.LatB7 = cells[7]
	p.LatB8 = cells[8]
	p.LatB9 = cells[9]
	p.LatB10 = cells[10]
	p.LatB11 = cells[11]
	p.LatB12 = cells[12]
}

// LatHist 以 [13]int64 返回全部延迟直方图单元计数（13 与 histCellCount 同步）。
func (p *PerfMetric) LatHist() [13]int64 {
	return [13]int64{p.LatB0, p.LatB1, p.LatB2, p.LatB3, p.LatB4, p.LatB5, p.LatB6, p.LatB7, p.LatB8, p.LatB9, p.LatB10, p.LatB11, p.LatB12}
}

// SetTtftHist 将 13 个首字节延迟直方图单元计数写入 TtftB0..TtftB12
// （单元数 13 与 pkg/perf_metrics/hist.go 的 histCellCount 同步）。
func (p *PerfMetric) SetTtftHist(cells [13]int64) {
	p.TtftB0 = cells[0]
	p.TtftB1 = cells[1]
	p.TtftB2 = cells[2]
	p.TtftB3 = cells[3]
	p.TtftB4 = cells[4]
	p.TtftB5 = cells[5]
	p.TtftB6 = cells[6]
	p.TtftB7 = cells[7]
	p.TtftB8 = cells[8]
	p.TtftB9 = cells[9]
	p.TtftB10 = cells[10]
	p.TtftB11 = cells[11]
	p.TtftB12 = cells[12]
}

// TtftHist 以 [13]int64 返回全部首字节延迟直方图单元计数（13 与 histCellCount 同步）。
func (p *PerfMetric) TtftHist() [13]int64 {
	return [13]int64{p.TtftB0, p.TtftB1, p.TtftB2, p.TtftB3, p.TtftB4, p.TtftB5, p.TtftB6, p.TtftB7, p.TtftB8, p.TtftB9, p.TtftB10, p.TtftB11, p.TtftB12}
}

func UpsertPerfMetric(metric *PerfMetric) error {
	if metric == nil || metric.RequestCount == 0 {
		return nil
	}
	updates := clause.Assignments(map[string]interface{}{
		"request_count":    gorm.Expr("perf_metrics.request_count + ?", metric.RequestCount),
		"success_count":    gorm.Expr("perf_metrics.success_count + ?", metric.SuccessCount),
		"total_latency_ms": gorm.Expr("perf_metrics.total_latency_ms + ?", metric.TotalLatencyMs),
		"ttft_sum_ms":      gorm.Expr("perf_metrics.ttft_sum_ms + ?", metric.TtftSumMs),
		"ttft_count":       gorm.Expr("perf_metrics.ttft_count + ?", metric.TtftCount),
		"output_tokens":    gorm.Expr("perf_metrics.output_tokens + ?", metric.OutputTokens),
		"generation_ms":    gorm.Expr("perf_metrics.generation_ms + ?", metric.GenerationMs),
	})
	updates = append(updates, histogramAssignments(metric)...)
	// 冲突目标须与唯一索引 idx_perf_model_group_channel_bucket 完全一致
	// （SQLite 要求逐列匹配，缺 channel_id 会在 prepare 阶段报错）。
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "model_name"},
			{Name: "group"},
			{Name: "channel_id"},
			{Name: "bucket_ts"},
		},
		DoUpdates: updates,
	}).Create(metric).Error
}

// histogramAssignments 生成直方图列的增量表达式（列名 lat_b{i}/ttft_b{i}）。
func histogramAssignments(m *PerfMetric) []clause.Assignment {
	out := make([]clause.Assignment, 0, 26)
	add := func(prefix string, get func(i int) int64) {
		for i := 0; i < 13; i++ { // 13 与 pkg/perf_metrics/hist.go 的 histCellCount 同步
			out = append(out, clause.Assignment{
				Column: clause.Column{Name: fmt.Sprintf("%s_b%d", prefix, i)},
				Value:  gorm.Expr(fmt.Sprintf("perf_metrics.%s_b%d + ?", prefix, i), get(i)),
			})
		}
	}
	add("lat", func(i int) int64 { return m.LatB(i) })
	add("ttft", func(i int) int64 { return m.TtftB(i) })
	return out
}

func GetPerfMetrics(modelName string, group string, startTs int64, endTs int64) ([]PerfMetric, error) {
	var metrics []PerfMetric
	query := DB.Model(&PerfMetric{}).
		Where("model_name = ? AND bucket_ts >= ? AND bucket_ts <= ?", modelName, startTs, endTs)
	if group != "" {
		query = query.Where(commonGroupCol+" = ?", group)
	}
	err := query.Order("bucket_ts ASC").Find(&metrics).Error
	return metrics, err
}

type PerfMetricSummary struct {
	ModelName      string `json:"model_name"`
	RequestCount   int64  `json:"request_count"`
	SuccessCount   int64  `json:"success_count"`
	TotalLatencyMs int64  `json:"total_latency_ms"`
	OutputTokens   int64  `json:"output_tokens"`
	GenerationMs   int64  `json:"generation_ms"`
}

type PerfMetricSummaryBucket struct {
	ModelName      string `json:"model_name"`
	BucketTs       int64  `json:"bucket_ts"`
	RequestCount   int64  `json:"request_count"`
	SuccessCount   int64  `json:"success_count"`
	TotalLatencyMs int64  `json:"total_latency_ms"`
	OutputTokens   int64  `json:"output_tokens"`
	GenerationMs   int64  `json:"generation_ms"`
}

func GetPerfMetricsSummaryAll(startTs int64, endTs int64, groups []string) ([]PerfMetricSummary, error) {
	var summaries []PerfMetricSummary
	query := DB.Model(&PerfMetric{}).
		Select("model_name, SUM(request_count) as request_count, SUM(success_count) as success_count, SUM(total_latency_ms) as total_latency_ms, SUM(output_tokens) as output_tokens, SUM(generation_ms) as generation_ms").
		Where("bucket_ts >= ? AND bucket_ts <= ?", startTs, endTs)
	if groups != nil {
		if len(groups) == 0 {
			return summaries, nil
		}
		query = query.Where(commonGroupCol+" IN ?", groups)
	}
	err := query.
		Group("model_name").
		Having("SUM(request_count) > 0").
		Find(&summaries).Error
	return summaries, err
}

func GetPerfMetricsSummaryBucketsAll(startTs int64, endTs int64, groups []string) ([]PerfMetricSummaryBucket, error) {
	var summaries []PerfMetricSummaryBucket
	query := DB.Model(&PerfMetric{}).
		Select("model_name, bucket_ts, SUM(request_count) as request_count, SUM(success_count) as success_count, SUM(total_latency_ms) as total_latency_ms, SUM(output_tokens) as output_tokens, SUM(generation_ms) as generation_ms").
		Where("bucket_ts >= ? AND bucket_ts <= ?", startTs, endTs)
	if groups != nil {
		if len(groups) == 0 {
			return summaries, nil
		}
		query = query.Where(commonGroupCol+" IN ?", groups)
	}
	err := query.
		Group("model_name, bucket_ts").
		Having("SUM(request_count) > 0").
		Order("bucket_ts ASC").
		Find(&summaries).Error
	return summaries, err
}

func DeletePerfMetricsBefore(cutoffTs int64) error {
	if cutoffTs <= 0 {
		return nil
	}
	return DB.Where("bucket_ts < ?", cutoffTs).Delete(&PerfMetric{}).Error
}

func PerfMetricStartTime(hours int) int64 {
	if hours <= 0 {
		hours = 24
	}
	return time.Now().Add(-time.Duration(hours) * time.Hour).Unix()
}

// CapacityMetric records per-bucket gateway capacity pressure (503 rejections
// and peak concurrent inflight requests), keyed by the flush bucket timestamp.
type CapacityMetric struct {
	BucketTs     int64 `json:"bucket_ts" gorm:"primaryKey"`
	Attempts     int64 `json:"attempts"`
	Rejected503  int64 `json:"rejected_503" gorm:"column:rejected_503"` // GORM 默认命名会去掉 503 前的下划线（rejected503）
	InflightPeak int64 `json:"inflight_peak"`
}

func (CapacityMetric) TableName() string { return "capacity_metrics" }

// UpsertCapacityMetric 先以冲突累加写入 attempts/rejected_503，再以条件更新取
// inflight_peak 最大值。不用 GREATEST/max()（方言差异，见 AGENTS.md）。
func UpsertCapacityMetric(m *CapacityMetric) error {
	if m == nil {
		return nil
	}
	err := DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "bucket_ts"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"attempts":     gorm.Expr("capacity_metrics.attempts + ?", m.Attempts),
			"rejected_503": gorm.Expr("capacity_metrics.rejected_503 + ?", m.Rejected503),
		}),
	}).Create(m).Error
	if err != nil {
		return err
	}
	res := DB.Model(&CapacityMetric{}).
		Where("bucket_ts = ? AND inflight_peak < ?", m.BucketTs, m.InflightPeak).
		Update("inflight_peak", m.InflightPeak)
	return res.Error
}

func GetCapacityMetrics(startTs int64, endTs int64) ([]CapacityMetric, error) {
	var rows []CapacityMetric
	err := DB.Where("bucket_ts >= ? AND bucket_ts <= ?", startTs, endTs).
		Order("bucket_ts ASC").Find(&rows).Error
	return rows, err
}

func DeleteCapacityBefore(cutoffTs int64) error {
	if cutoffTs <= 0 {
		return nil
	}
	return DB.Where("bucket_ts < ?", cutoffTs).Delete(&CapacityMetric{}).Error
}
