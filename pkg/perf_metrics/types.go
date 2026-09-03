package perfmetrics

import "sync/atomic"

type Store interface {
	Record(sample Sample)
	Query(params QueryParams) (QueryResult, error)
}

type Sample struct {
	Model        string
	Group        string
	ChannelId    int
	LatencyMs    int64
	TtftMs       int64
	HasTtft      bool
	Success      bool
	OutputTokens int64
	GenerationMs int64
}

type QueryParams struct {
	Model string
	Group string
	Hours int
	// ChannelId 0=不过滤（含 channel_id=0 的未分渠道行）；仅当 >0 时按渠道过滤。
	ChannelId int
}

// BucketPoint 增加字段（加法扩展，旧字段与语义不动）。success_rate 为 0..100
// 百分数（与 GroupResult/汇总口径一致）；分位数字段无数据时为 -1。
type BucketPoint struct {
	Ts           int64   `json:"ts"`
	AvgTtftMs    int64   `json:"avg_ttft_ms"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`
	AvgTps       float64 `json:"avg_tps"`
	RequestCount int64   `json:"request_count"`
	P50LatencyMs int64   `json:"p50_latency_ms"`
	P95LatencyMs int64   `json:"p95_latency_ms"`
	P99LatencyMs int64   `json:"p99_latency_ms"`
	P95TtftMs    int64   `json:"p95_ttft_ms"`
}

// ChannelResult 是某 group 下单个渠道的逐桶明细。channel_id=0（未分渠道）不产生
// 条目；channel_name 为空（纯热点桶尚无 DB 行）时前端回退 #<channel_id>。
type ChannelResult struct {
	ChannelId   int           `json:"channel_id"`
	ChannelName string        `json:"channel_name"`
	Series      []BucketPoint `json:"series"`
}

type GroupResult struct {
	Group        string          `json:"group"`
	AvgTtftMs    int64           `json:"avg_ttft_ms"`
	AvgLatencyMs int64           `json:"avg_latency_ms"`
	SuccessRate  float64         `json:"success_rate"`
	AvgTps       float64         `json:"avg_tps"`
	Series       []BucketPoint   `json:"series"`
	Channels     []ChannelResult `json:"channels,omitempty"`
}

type QueryResult struct {
	ModelName    string        `json:"model_name"`
	SeriesSchema string        `json:"series_schema"`
	Groups       []GroupResult `json:"groups"`
}

type ModelSummary struct {
	ModelName          string    `json:"model_name"`
	AvgLatencyMs       int64     `json:"avg_latency_ms"`
	SuccessRate        float64   `json:"success_rate"`
	AvgTps             float64   `json:"avg_tps"`
	RecentSuccessRates []float64 `json:"recent_success_rates,omitempty"`
	RequestCount       int64     `json:"-"`
}

type SummaryAllResult struct {
	Models []ModelSummary `json:"models"`
}

type bucketKey struct {
	model     string
	group     string
	channelId int
	bucketTs  int64
}

type counters struct {
	requestCount   int64
	successCount   int64
	totalLatencyMs int64
	ttftSumMs      int64
	ttftCount      int64
	outputTokens   int64
	generationMs   int64
	latHist        [histCellCount]int64
	ttftHist       [histCellCount]int64
}

// merge 把 o 逐单元加和进 c（含直方图两数组），并返回合并结果。
func (c *counters) merge(o counters) counters {
	c.requestCount += o.requestCount
	c.successCount += o.successCount
	c.totalLatencyMs += o.totalLatencyMs
	c.ttftSumMs += o.ttftSumMs
	c.ttftCount += o.ttftCount
	c.outputTokens += o.outputTokens
	c.generationMs += o.generationMs
	for i := 0; i < histCellCount; i++ {
		c.latHist[i] += o.latHist[i]
		c.ttftHist[i] += o.ttftHist[i]
	}
	return *c
}

type atomicBucket struct {
	requestCount   atomic.Int64
	successCount   atomic.Int64
	totalLatencyMs atomic.Int64
	ttftSumMs      atomic.Int64
	ttftCount      atomic.Int64
	outputTokens   atomic.Int64
	generationMs   atomic.Int64
	latHist        [histCellCount]atomic.Int64
	ttftHist       [histCellCount]atomic.Int64
}

func (b *atomicBucket) add(sample Sample) {
	b.requestCount.Add(1)
	if sample.Success {
		b.successCount.Add(1)
	}
	if sample.LatencyMs > 0 {
		b.totalLatencyMs.Add(sample.LatencyMs)
	}
	if sample.HasTtft && sample.TtftMs >= 0 {
		b.ttftSumMs.Add(sample.TtftMs)
		b.ttftCount.Add(1)
	}
	if sample.OutputTokens > 0 && sample.GenerationMs > 0 {
		b.outputTokens.Add(sample.OutputTokens)
		b.generationMs.Add(sample.GenerationMs)
	}
	b.latHist[histIndex(sample.LatencyMs)].Add(1)
	if sample.HasTtft && sample.TtftMs >= 0 {
		b.ttftHist[histIndex(sample.TtftMs)].Add(1)
	}
}

func (b *atomicBucket) snapshot() counters {
	var c counters
	c.requestCount = b.requestCount.Load()
	c.successCount = b.successCount.Load()
	c.totalLatencyMs = b.totalLatencyMs.Load()
	c.ttftSumMs = b.ttftSumMs.Load()
	c.ttftCount = b.ttftCount.Load()
	c.outputTokens = b.outputTokens.Load()
	c.generationMs = b.generationMs.Load()
	for i := 0; i < histCellCount; i++ {
		c.latHist[i] = b.latHist[i].Load()
		c.ttftHist[i] = b.ttftHist[i].Load()
	}
	return c
}

func (b *atomicBucket) drain() counters {
	var c counters
	c.requestCount = b.requestCount.Swap(0)
	c.successCount = b.successCount.Swap(0)
	c.totalLatencyMs = b.totalLatencyMs.Swap(0)
	c.ttftSumMs = b.ttftSumMs.Swap(0)
	c.ttftCount = b.ttftCount.Swap(0)
	c.outputTokens = b.outputTokens.Swap(0)
	c.generationMs = b.generationMs.Swap(0)
	for i := 0; i < histCellCount; i++ {
		c.latHist[i] = b.latHist[i].Swap(0)
		c.ttftHist[i] = b.ttftHist[i].Swap(0)
	}
	return c
}

func (b *atomicBucket) addCounters(c counters) {
	if c.requestCount != 0 {
		b.requestCount.Add(c.requestCount)
	}
	if c.successCount != 0 {
		b.successCount.Add(c.successCount)
	}
	if c.totalLatencyMs != 0 {
		b.totalLatencyMs.Add(c.totalLatencyMs)
	}
	if c.ttftSumMs != 0 {
		b.ttftSumMs.Add(c.ttftSumMs)
	}
	if c.ttftCount != 0 {
		b.ttftCount.Add(c.ttftCount)
	}
	if c.outputTokens != 0 {
		b.outputTokens.Add(c.outputTokens)
	}
	if c.generationMs != 0 {
		b.generationMs.Add(c.generationMs)
	}
	for i := 0; i < histCellCount; i++ {
		b.latHist[i].Add(c.latHist[i])
		b.ttftHist[i].Add(c.ttftHist[i])
	}
}
