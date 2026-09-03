package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetricsExportAuthBranches 覆盖 MetricsExport 的鉴权分支。设置键是包级配置
// 变量（测试不可写），经可注入的 metricsExportGate 提供固定返回值（每子用例
// 临时替换、结束恢复）。handler 以最小 gin engine 挂载执行（仿 middleware 测试
// 模式）：404/401 为纯状态响应无 body，recorder.Code 需经引擎路径落盘。
func TestMetricsExportAuthBranches(t *testing.T) {
	originalGate := metricsExportGate
	defer func() { metricsExportGate = originalGate }()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		setting     perf_metrics_setting.PerfMetricsSetting
		auth        string
		wantStatus  int
		wantMetrics bool
	}{
		{name: "export disabled returns 404", setting: perf_metrics_setting.PerfMetricsSetting{ExportEnabled: false, ExportToken: ""}, auth: "", wantStatus: http.StatusNotFound},
		{name: "enabled without token returns 200", setting: perf_metrics_setting.PerfMetricsSetting{ExportEnabled: true, ExportToken: ""}, auth: "", wantStatus: http.StatusOK, wantMetrics: true},
		{name: "missing bearer header returns 401", setting: perf_metrics_setting.PerfMetricsSetting{ExportEnabled: true, ExportToken: "secret"}, auth: "secret", wantStatus: http.StatusUnauthorized},
		{name: "wrong token returns 401", setting: perf_metrics_setting.PerfMetricsSetting{ExportEnabled: true, ExportToken: "secret"}, auth: "Bearer nope", wantStatus: http.StatusUnauthorized},
		{name: "correct token returns 200", setting: perf_metrics_setting.PerfMetricsSetting{ExportEnabled: true, ExportToken: "secret"}, auth: "Bearer secret", wantStatus: http.StatusOK, wantMetrics: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			metricsExportGate = func() perf_metrics_setting.PerfMetricsSetting { return tc.setting }

			engine := gin.New()
			engine.GET("/api/metrics", MetricsExport)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
			if tc.auth != "" {
				request.Header.Set("Authorization", tc.auth)
			}
			engine.ServeHTTP(recorder, request)

			assert.Equal(t, tc.wantStatus, recorder.Code)
			if tc.wantMetrics {
				require.NotEmpty(t, recorder.Body.String())
				assert.Contains(t, recorder.Body.String(), "cuberouter_")
			}
		})
	}
}
