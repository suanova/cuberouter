package middleware

import (
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/gin-gonic/gin"
)

// RelayCapacity 统计进入 relay 子组的请求：进入时在途并发 +1，请求结束（含下游
// 任一中止，defer 保证执行）时 -1 并给当前容量桶记一次尝试（attempts）。
// 在全部 8 个挂载点中它都是本组链上最先执行的计数中间件——紧接 RouteTag/
// pinRoute 之后、先于 TokenAuth/UserAuth 与 SystemPerformanceCheck——保证口径
// 统一（spec §3.2「含未过鉴权/被拒」）：进入子组的请求无论其后被鉴权拒绝、被
// 过载保护以 503 拒绝、还是被限流拒绝，都已计入 attempts，且处理期间（含被拒
// 前的检查/等待时段）在途 gauge 覆盖。被过载保护 503 拒绝的请求另由
// SystemPerformanceCheck 的 503 分支调用 RecordOverloadReject 计入
// rejected_503（见 performance.go），即 attempts ⊇ rejected_503。
func RelayCapacity() gin.HandlerFunc {
	return func(c *gin.Context) {
		perfmetrics.RelayRequestStart()
		defer perfmetrics.RelayRequestEnd()
		c.Next()
	}
}
