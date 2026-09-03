package middleware

import (
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/gin-gonic/gin"
)

// RelayCapacity 统计进入 relay 子组的请求：进入时在途并发 +1，请求结束（含下游
// 任一中止，defer 保证执行）时 -1 并给当前容量桶记一次尝试（attempts）。
// 挂在 SystemPerformanceCheck 之前：即使后续被过载保护以 503 拒绝（perf check
// abort 展开下游链）、或被鉴权/限流拒绝，本次请求也已被计入 attempts，且其处理
// 期间在途 gauge 覆盖（含被拒前的等待/检查时段）。rejected_503 由
// SystemPerformanceCheck 的 503 分支调用 RecordOverloadReject 精确计数（见
// performance.go），即被过载保护拒绝的请求同时计入 attempts 与 rejected_503。
func RelayCapacity() gin.HandlerFunc {
	return func(c *gin.Context) {
		perfmetrics.RelayRequestStart()
		defer perfmetrics.RelayRequestEnd()
		c.Next()
	}
}
