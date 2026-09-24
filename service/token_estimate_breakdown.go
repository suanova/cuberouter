package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// GetTokenEstimateBreakdown 返回当前请求的 token 估算构成分解（请求作用域：
// 由 CountRequestToken 写入 gin context，随 PriceData.PreConsumeDetail 落库）。
// CountToken 关闭或该请求未经过估算时返回 nil。每个请求只读自己的 context，
// 并发请求互不干扰，也不会复用其他请求的过期数据。
func GetTokenEstimateBreakdown(c *gin.Context) *types.TokenEstimateBreakdown {
	if c == nil {
		return nil
	}
	v, ok := common.GetContextKey(c, constant.ContextKeyTokenEstimateBreakdown)
	if !ok {
		return nil
	}
	bd, ok := v.(*types.TokenEstimateBreakdown)
	if !ok {
		return nil
	}
	cp := *bd
	return &cp
}
