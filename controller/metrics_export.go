package controller

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
	"github.com/gin-gonic/gin"
)

// metricsExportGate 可注入：生产指向 perf_metrics_setting.GetSetting；测试替换
// 为固定返回值（设置是包级配置变量，测试不可写，经此函数隔离）。
var metricsExportGate = func() perf_metrics_setting.PerfMetricsSetting {
	return perf_metrics_setting.GetSetting()
}

// MetricsExport 暴露 Prometheus 文本指标。未开启时 404（避免暴露存在性）；
// 配置了 token 时要求 Authorization: Bearer <token> 恒定时间比较。
func MetricsExport(c *gin.Context) {
	s := metricsExportGate()
	if !s.ExportEnabled {
		c.Status(http.StatusNotFound)
		return
	}
	if s.ExportToken != "" {
		auth := c.GetHeader("Authorization")
		got := strings.TrimPrefix(auth, "Bearer ")
		if auth == got { // 无 Bearer 前缀
			c.Status(http.StatusUnauthorized)
			return
		}
		wantSum := sha256.Sum256([]byte(s.ExportToken))
		gotSum := sha256.Sum256([]byte(got))
		if subtle.ConstantTimeCompare(wantSum[:], gotSum[:]) != 1 {
			c.Status(http.StatusUnauthorized)
			return
		}
	}
	c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(perfmetrics.PrometheusText()))
}
