package middleware

import (
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/gin-gonic/gin"
)

// RelayCapacity 统计进入 relay 子组的请求：进入时在途并发 +1，请求结束（含下游
// 任一中止，defer 保证执行）时 -1 并给当前容量桶记一次尝试（attempts）。
// 挂在 SystemPerformanceCheck 之后：未过载检查的请求仍由 RecordOverloadReject
// 计为 rejected_503（见 performance.go），此处只统计真正进入转发链的请求——
// 其后被鉴权/限流拒绝的请求同样计入 attempts。
func RelayCapacity() gin.HandlerFunc {
	return func(c *gin.Context) {
		perfmetrics.RelayRequestStart()
		defer perfmetrics.RelayRequestEnd()
		c.Next()
	}
}
