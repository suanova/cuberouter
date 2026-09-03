package perfmetrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistIndexBoundaries(t *testing.T) {
	// 半开区间 [bound[i-1], bound[i])；<=0 进 0；>=最大边界进尾桶
	cases := []struct {
		ms   int64
		want int
	}{
		{0, 0}, {99, 0}, {100, 1}, {249, 1}, {250, 2},
		{1000, 4}, {239999, 11}, {240000, 12}, {600000, 12},
	}
	for _, c := range cases {
		require.Equal(t, c.want, histIndex(c.ms), "histIndex(%d)", c.ms)
	}
}

func TestQuantileInterpolation(t *testing.T) {
	// 100 个样本：全部 1000ms → p50/p95/p99 均应=1000（跨越边界估计：rank 落在
	// 单元 4 = [1000,2000) 内部 → 取单元下界 1000，无插值偏移）
	var hist [histCellCount]int64
	hist[4] = 100
	for _, q := range []float64{0.5, 0.95, 0.99} {
		got := quantileMs(q, &hist, 100)
		assert.Equal(t, float64(1000), got, "q=%v", q)
	}
	// 混合分布：一半 500ms（单元 2）一半 2500ms（单元 5），p50 应=500
	var hist2 [histCellCount]int64
	hist2[2] = 50
	hist2[5] = 50
	assert.Equal(t, float64(500), quantileMs(0.5, &hist2, 100))
	// 空桶 → -1；全在尾桶 → 240000
	assert.Equal(t, float64(-1), quantileMs(0.5, &[histCellCount]int64{}, 0))
	var tail [histCellCount]int64
	tail[histCellCount-1] = 10
	assert.Equal(t, float64(240000), quantileMs(0.5, &tail, 10))
}

func TestCountersHistAddAndMerge(t *testing.T) {
	var b atomicBucket
	s := Sample{LatencyMs: 1500, TtftMs: 300, HasTtft: true, Success: true, OutputTokens: 10, GenerationMs: 1000}
	b.add(s)
	snap := b.snapshot()
	require.Equal(t, int64(1), snap.requestCount)
	// 1500ms 落在单元 4（[1000,2000)），300ms 落在单元 2（[250,500)）
	assert.Equal(t, int64(1), snap.latHist[4])
	assert.Equal(t, int64(1), snap.ttftHist[2])
	var merged counters
	merged.merge(snap)
	merged.merge(snap)
	require.Equal(t, int64(2), merged.requestCount)
	assert.Equal(t, int64(2), merged.latHist[4])
}
