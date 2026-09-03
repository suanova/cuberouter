package perf_metrics_setting

import "github.com/QuantumNous/new-api/setting/config"

// PerfMetricsSetting：Enabled=false 时暂停 DB flush 与保留清理（perf_metrics
// 与 capacity_metrics 两张表）并冻结导出族，进程级计数继续运行。
type PerfMetricsSetting struct {
	Enabled       bool   `json:"enabled"`
	FlushInterval int    `json:"flush_interval"`
	BucketTime    string `json:"bucket_time"`
	RetentionDays int    `json:"retention_days"`
	// ExportEnabled/ExportToken 控制 /api/metrics 的 Prometheus 薄导出
	// （spec §6；配置 JSON 键，非表模型，无 gorm 标签）。默认 false/"" =
	// 不暴露端点；token 非空时要求 Authorization: Bearer <token>。
	ExportEnabled bool   `json:"export_enabled"`
	ExportToken   string `json:"export_token"`
}

var perfMetricsSetting = PerfMetricsSetting{
	Enabled:       true,
	FlushInterval: 5,
	BucketTime:    "hour",
	RetentionDays: 0,
}

func init() {
	config.GlobalConfig.Register("perf_metrics_setting", &perfMetricsSetting)
}

func GetSetting() PerfMetricsSetting {
	return perfMetricsSetting
}

func GetBucketSeconds() int64 {
	switch perfMetricsSetting.BucketTime {
	case "minute":
		return 60
	case "5min":
		return 300
	case "hour":
		return 3600
	default:
		return 3600
	}
}

func GetFlushIntervalMinutes() int {
	if perfMetricsSetting.FlushInterval < 1 {
		return 1
	}
	return perfMetricsSetting.FlushInterval
}
