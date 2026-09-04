package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCapacityMetricsHoursClamp 覆盖 parseHoursParam 的参数边界：
// 空/非法/<=0 回落默认值；超过 720（30 天，与 perf-metrics 窗口上限一致）截断。
// 路由注册与 AdminAuth 权限断言在 router 包（capacity_router_test.go）。
func TestCapacityMetricsHoursClamp(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		def  int
		want int
	}{
		{name: "empty falls back to default", raw: "", def: 24, want: 24},
		{name: "negative falls back to default", raw: "-1", def: 24, want: 24},
		{name: "zero falls back to default", raw: "0", def: 24, want: 24},
		{name: "non-numeric falls back to default", raw: "abc", def: 24, want: 24},
		{name: "huge value clamped to 720", raw: "9999", def: 24, want: 720},
		{name: "cap boundary kept", raw: "720", def: 24, want: 720},
		{name: "valid hours kept", raw: "48", def: 24, want: 48},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseHoursParam(tc.raw, tc.def))
		})
	}
}
