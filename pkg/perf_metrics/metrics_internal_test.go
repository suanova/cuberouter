package perfmetrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearHotBuckets(t *testing.T) {
	t.Helper()
	hotBuckets.Range(func(k, v any) bool {
		hotBuckets.Delete(k)
		return true
	})
}

func TestRecordBucketsByChannel(t *testing.T) {
	// Record 依赖 perf_metrics_setting.Enabled（默认 true，测试环境不加载配置）。
	clearHotBuckets(t)
	defer clearHotBuckets(t)

	Record(Sample{Model: "m", Group: "g", ChannelId: 1, LatencyMs: 1500, Success: true, OutputTokens: 10, GenerationMs: 500})
	Record(Sample{Model: "m", Group: "g", ChannelId: 2, LatencyMs: 3000, Success: true, OutputTokens: 10, GenerationMs: 500})
	Record(Sample{Model: "m", Group: "g", ChannelId: 1, LatencyMs: 500, Success: true, OutputTokens: 10, GenerationMs: 500})

	// 不同 channel 的样本必须落进各自的桶键。
	var keys []bucketKey
	hotBuckets.Range(func(k, v any) bool {
		keys = append(keys, k.(bucketKey))
		return true
	})
	require.Len(t, keys, 2) // 两个不同 channel
	channels := map[int]bool{}
	for _, k := range keys {
		channels[k.channelId] = true
	}
	assert.True(t, channels[1])
	assert.True(t, channels[2])
}

func TestBucketPointPercentilesFromHist(t *testing.T) {
	var hist [histCellCount]int64
	hist[4] = 80 // [1000, 2000)
	hist[8] = 20 // [16000, 32000)
	p := buildBucketPoint(1000, counters{
		requestCount: 100, successCount: 100, totalLatencyMs: 400_000,
		outputTokens: 200, generationMs: 1000,
		latHist: hist,
	})
	assert.Equal(t, int64(1000), p.Ts)
	assert.Equal(t, int64(4000), p.AvgLatencyMs)
	// success_rate 为 0..100 百分数：与既有 series/GroupResult 口径一致
	// （spec §5.1 旧字段语义不动；前端 formatUptimePct 直接渲染百分数）。
	assert.Equal(t, float64(100), p.SuccessRate)
	assert.Equal(t, float64(200), p.AvgTps) // 200 tok / 1000ms = 200 tps
	assert.Equal(t, int64(100), p.RequestCount)
	// P50：rank=50 落在单元 4（[1000,2000)）内部 → 单元下界 1000ms
	assert.Equal(t, int64(1000), p.P50LatencyMs)
	// P95：rank=95，单元 4 累计到 80，单元 8（[16000,32000)）累计到 100；
	// 跨越边界估计 → 单元 8 下界，区间断言允许实现级微调
	p95 := p.P95LatencyMs
	assert.GreaterOrEqual(t, p95, int64(16000))
	assert.Less(t, p95, int64(32000))
	// P99 同单元 8
	assert.GreaterOrEqual(t, p.P99LatencyMs, int64(16000))
	assert.Less(t, p.P99LatencyMs, int64(32000))
	// ttft 无数据 → -1
	assert.Equal(t, int64(-1), p.P95TtftMs)
}

func TestBuildBucketPointEmptyAndTail(t *testing.T) {
	p := buildBucketPoint(2000, counters{}) // requestCount=0
	assert.Equal(t, int64(-1), p.P50LatencyMs)
	assert.Equal(t, int64(-1), p.P95LatencyMs)
	assert.Equal(t, int64(-1), p.P99LatencyMs)
	assert.Equal(t, int64(-1), p.P95TtftMs)

	var tail [histCellCount]int64
	tail[histCellCount-1] = 10 // 全在尾桶
	pt := buildBucketPoint(3000, counters{requestCount: 10, successCount: 10, ttftSumMs: 0, ttftCount: 0, latHist: tail})
	assert.Equal(t, int64(240000), pt.P95LatencyMs) // 尾桶近似上限
}

// drain()/addCounters() 直方图对称性：flush 失败回填路径必须完整恢复直方图两数组。
func TestDrainAddCountersHistSymmetry(t *testing.T) {
	var b atomicBucket
	for i := 0; i < 3; i++ {
		b.add(Sample{LatencyMs: 1500, TtftMs: 300, HasTtft: true, Success: true, OutputTokens: 10, GenerationMs: 1000})
	}
	want := b.snapshot()

	drained := b.drain()
	require.Equal(t, want, drained)
	require.Zero(t, b.snapshot().requestCount)

	b.addCounters(drained)
	require.Equal(t, want, b.snapshot())
}

// Query 的 DB 行路径由 model 层测试与集成验证覆盖；这里直接喂 merged 桶，
// 验证 buildQueryResult 的跨渠道汇总（groups[].series）与 per-channel 明细
// （groups[].channels）装配：channels 不含 channel_id=0，series 含全部渠道。
func TestBuildQueryResultChannels(t *testing.T) {
	merged := map[bucketKey]counters{
		{model: "m", group: "g", channelId: 0, bucketTs: 1000}: {requestCount: 2, successCount: 2, totalLatencyMs: 2000},
		{model: "m", group: "g", channelId: 1, bucketTs: 1000}: {requestCount: 3, successCount: 3, totalLatencyMs: 9000},
		{model: "m", group: "g", channelId: 2, bucketTs: 1000}: {requestCount: 5, successCount: 5, totalLatencyMs: 25000},
		{model: "m", group: "g", channelId: 1, bucketTs: 2000}: {requestCount: 7, successCount: 7, totalLatencyMs: 28000},
	}
	result := buildQueryResult("m", merged, map[int]string{1: "up-1"})
	require.Len(t, result.Groups, 1)
	group := result.Groups[0]
	require.Equal(t, "g", group.Group)

	// series 跨渠道汇总：1000 桶 = ch0+ch1+ch2（2+3+5=10），2000 桶 = ch1（7）
	require.Len(t, group.Series, 2)
	assert.Equal(t, int64(1000), group.Series[0].Ts)
	assert.Equal(t, int64(10), group.Series[0].RequestCount)
	assert.Equal(t, int64(3600), group.Series[0].AvgLatencyMs) // 36000ms/10
	assert.Equal(t, int64(2000), group.Series[1].Ts)
	assert.Equal(t, int64(7), group.Series[1].RequestCount)
	assert.Equal(t, float64(100), group.Series[0].SuccessRate)
	assert.Equal(t, float64(100), group.SuccessRate) // 17/17

	// channels 明细：升序、含 DB 来的 channel_name、channel_id=0 不展示
	require.Len(t, group.Channels, 2)
	first, second := group.Channels[0], group.Channels[1]
	require.Equal(t, 1, first.ChannelId)
	assert.Equal(t, "up-1", first.ChannelName)
	require.Len(t, first.Series, 2)
	assert.Equal(t, int64(3), first.Series[0].RequestCount)
	assert.Equal(t, int64(7), first.Series[1].RequestCount)
	require.Equal(t, 2, second.ChannelId)
	assert.Equal(t, "", second.ChannelName) // 无 DB 行（纯热点桶）→ 空串，前端回退 #<id>
	require.Len(t, second.Series, 1)
	assert.Equal(t, int64(5), second.Series[0].RequestCount)
}
