package service

import (
	"sync"

	"github.com/QuantumNous/new-api/types"
)

// recordTokenEstimateBreakdown 记录最近一次估算的构成（每个请求在其 goroutine 内覆盖写入）。
// 构成结构体定义在 types 包（types.TokenEstimateBreakdown），随 PriceData.PreConsumeDetail 落库。
func recordTokenEstimateBreakdown(d *types.TokenEstimateBreakdown) {
	lastTokenEstimateMu.Lock()
	lastTokenEstimateBreakdown = d
	lastTokenEstimateMu.Unlock()
}

// GetLastTokenEstimateBreakdown 返回最近一次估算构成的副本（无记录时返回 nil）。
func GetLastTokenEstimateBreakdown() *types.TokenEstimateBreakdown {
	lastTokenEstimateMu.Lock()
	defer lastTokenEstimateMu.Unlock()
	if lastTokenEstimateBreakdown == nil {
		return nil
	}
	cp := *lastTokenEstimateBreakdown
	return &cp
}

var (
	lastTokenEstimateMu        sync.Mutex
	lastTokenEstimateBreakdown *types.TokenEstimateBreakdown
)
