package perfmetrics

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var lineRe = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*(\{[^}]+\})? -?[0-9.eE+]+$`)

func TestPrometheusTextFormatAndValues(t *testing.T) {
	// 注册表先复位
	procMu.Lock()
	procSeries = map[string]*procCounter{}
	procMu.Unlock()

	Record(Sample{Model: "m-a", Group: "g1", ChannelId: 0, LatencyMs: 1500, HasTtft: true, TtftMs: 300, Success: true, OutputTokens: 10, GenerationMs: 500})
	Record(Sample{Model: "m-a", Group: "g1", ChannelId: 0, LatencyMs: 2500, HasTtft: true, TtftMs: 400, Success: false, OutputTokens: 5, GenerationMs: 700})

	text := PrometheusText()
	require.NotEmpty(t, text)
	for _, l := range strings.Split(strings.TrimSpace(text), "\n") {
		if strings.HasPrefix(l, "#") {
			continue
		}
		require.Regexp(t, lineRe, l, "bad line: %s", l)
	}
	// relay_requests_total 计数=2（含失败）；latency _count 同样=2
	assert.Contains(t, text, `cuberouter_relay_requests_total{model="m-a",group="g1"} 2`)
	assert.Contains(t, text, `cuberouter_relay_latency_seconds_count{model="m-a",group="g1"} 2`)
	// 1.5s → 落在 1s..2s 单元：要求 _bucket{le="1"} 累计 >= 0 且 le="2" 累计 >= 1（精确断言 1）
	assert.Contains(t, text, `cuberouter_relay_latency_seconds_bucket{model="m-a",group="g1",le="1"} 0`)
	assert.Contains(t, text, `cuberouter_relay_latency_seconds_bucket{model="m-a",group="g1",le="2"} 1`)
	// sum 单调、gauge 行存在
	assert.Contains(t, text, "cuberouter_inflight_requests ")
	assert.Contains(t, text, "cuberouter_overload_rejects_total ")
}
