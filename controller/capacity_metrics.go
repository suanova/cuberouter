package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// parseHoursParam 解析 hours 查询参数：空/非法/<=0 → def；>720（30 天窗口上限，
// 与 perf-metrics 一致）→ 720。controller/perf_metrics.go 的手写解析后续可收敛到
// 本函数，但本次不改动 perf_metrics.go（范围控制）。
func parseHoursParam(raw string, def int) int {
	if raw == "" {
		return def
	}
	h, err := strconv.Atoi(raw)
	if err != nil || h <= 0 {
		return def
	}
	if h > 720 {
		return 720
	}
	return h
}

// GetCapacityMetrics 返回网关容量趋势（GET /api/capacity-metrics?hours=，AdminAuth）。
//
// 数据来自 capacity_metrics 表（按容量桶聚合，键与 perf 桶同为 bucketStart 粒度）。
// 注意近似语义：行由 flushLoop 在桶完结后写入，进行中的热点桶不落库，因此最新
// 数据点可能滞后当前墙钟桶至多一个 flush 周期；inflight_peak 为 2s 采样近似峰值。
// attempts 是进入 relay 子组的请求数（含被过载保护 503 拒绝、被鉴权/限流拒绝者，
// 见 middleware.RelayCapacity）；rejected_503 是其中 SystemPerformanceCheck 过载
// 拒绝的 503 子集；RPS = attempts / 桶秒数由前端换算。
func GetCapacityMetrics(c *gin.Context) {
	hours := parseHoursParam(c.Query("hours"), 24)
	endTs := time.Now().Unix()
	rows, err := model.GetCapacityMetrics(endTs-int64(hours)*3600, endTs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	type point struct {
		Ts           int64 `json:"ts"`
		Attempts     int64 `json:"attempts"`
		Rejected503  int64 `json:"rejected_503"`
		InflightPeak int64 `json:"inflight_peak"`
	}
	series := make([]point, 0, len(rows))
	for _, r := range rows {
		series = append(series, point{Ts: r.BucketTs, Attempts: r.Attempts, Rejected503: r.Rejected503, InflightPeak: r.InflightPeak})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"series": series,
		},
	})
}
