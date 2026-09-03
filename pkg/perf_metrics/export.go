package perfmetrics

import (
	"math"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// 进程级 Prometheus 导出（spec §6）：进程注册表与热桶同一边界，在 Record() 内
// 同步累加（Record 运行于 gopool goroutine、每请求一次，普通互斥锁粒度可接受）；
// 重启归零符合 counter 语义，多实例由 Prometheus 按 pod 分别抓取。series 键
// 为 model "\x00" group，不含 channel 维度（高基数，见 spec §6 范围外）。
//
// procCounter 字段与 Sample/热桶 counters 一一对应镜像（succ/outTokens/genMs
// 暂不对应导出指标，仅随样本记账保持注册表为完整进程镜像，供后续扩展）。
type procCounter struct {
	req, succ, latSumMs, ttftSumMs, ttftN, outTokens, genMs int64
	latCells                                                [histCellCount]int64
	ttftCells                                               [histCellCount]int64
}

var (
	procMu     sync.RWMutex
	procSeries = map[string]*procCounter{}

	// procAttempts/procRejects 是容量计数路径写入的进程级 counter（capacity.go
	// 的 RelayRequestEnd/RecordOverloadReject 内累加），由 PrometheusText 导出。
	procAttempts atomic.Int64
	procRejects  atomic.Int64
)

// recordProc 把样本累加进进程级注册表。记账条件与热桶 atomicBucket.add 完全
// 一致（含直方图双数组），因此直方图桶累计 ≡ _count/总量 恒等，_sum 与 _count
// 覆盖同一批样本。
func recordProc(sample Sample) {
	key := sample.Model + "\x00" + sample.Group
	procMu.Lock()
	pc := procSeries[key]
	if pc == nil {
		pc = &procCounter{}
		procSeries[key] = pc
	}
	pc.req++
	if sample.Success {
		pc.succ++
	}
	if sample.LatencyMs > 0 {
		pc.latSumMs += sample.LatencyMs
	}
	if sample.HasTtft && sample.TtftMs >= 0 {
		pc.ttftSumMs += sample.TtftMs
		pc.ttftN++
	}
	if sample.OutputTokens > 0 && sample.GenerationMs > 0 {
		pc.outTokens += sample.OutputTokens
		pc.genMs += sample.GenerationMs
	}
	pc.latCells[histIndex(sample.LatencyMs)]++
	if sample.HasTtft && sample.TtftMs >= 0 {
		pc.ttftCells[histIndex(sample.TtftMs)]++
	}
	procMu.Unlock()
}

// PrometheusText 渲染进程级指标的 Prometheus 文本 exposition（spec §6）。
// 布局：每行以 \n 结尾；# HELP/# TYPE 注释先行；系列标签顺序固定 model,group；
// 直方图 le 边界升序（latencyBoundsMs/1000 的秒字符串），le="+Inf" 行 = 样本
// 总量，_sum 为秒浮点、_count = 总量。多系列时同一 metric family 连续输出
// （族内逐系列），否则 Prometheus 文本解析器会以 family 不连续拒绝整份输出。
// 数字格式：counter/gauge 用 FormatInt，直方图 bucket/_sum 用 FormatFloat。
func PrometheusText() string {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	// 锁内拷贝快照、锁外渲染：读锁持有期间仅做值拷贝，不阻塞 Record 太久，
	// 渲染期（含 FormatFloat/Builder 增长）不占锁。
	procMu.RLock()
	type seriesEntry struct {
		key string
		c   procCounter
	}
	series := make([]seriesEntry, 0, len(procSeries))
	for key, pc := range procSeries {
		series = append(series, seriesEntry{key: key, c: *pc})
	}
	procMu.RUnlock()
	sort.Slice(series, func(i, j int) bool { return series[i].key < series[j].key })
	labels := make([]string, len(series))
	for i, s := range series {
		parts := strings.SplitN(s.key, "\x00", 2)
		model, group := parts[0], ""
		if len(parts) == 2 {
			group = parts[1]
		}
		labels[i] = "model=" + quoteLabelValue(model) + ",group=" + quoteLabelValue(group)
	}

	var b strings.Builder

	b.WriteString("# HELP cuberouter_relay_requests_total Total relay requests (incl. failures) since process start.\n")
	b.WriteString("# TYPE cuberouter_relay_requests_total counter\n")
	for i, s := range series {
		writeSample(&b, "cuberouter_relay_requests_total", labels[i], strconv.FormatInt(s.c.req, 10))
	}

	b.WriteString("# HELP cuberouter_relay_latency_seconds Relay latency in seconds.\n")
	b.WriteString("# TYPE cuberouter_relay_latency_seconds histogram\n")
	for i, s := range series {
		writeHistogramSeries(&b, "cuberouter_relay_latency_seconds", labels[i], &s.c.latCells, s.c.req, secondsFloat(s.c.latSumMs))
	}

	b.WriteString("# HELP cuberouter_relay_ttft_seconds Relay time-to-first-token in seconds.\n")
	b.WriteString("# TYPE cuberouter_relay_ttft_seconds histogram\n")
	for i, s := range series {
		writeHistogramSeries(&b, "cuberouter_relay_ttft_seconds", labels[i], &s.c.ttftCells, s.c.ttftN, secondsFloat(s.c.ttftSumMs))
	}

	b.WriteString("# HELP cuberouter_inflight_requests Relay requests currently in flight (process-local gauge).\n")
	b.WriteString("# TYPE cuberouter_inflight_requests gauge\n")
	writeSample(&b, "cuberouter_inflight_requests", "", strconv.FormatInt(relayInflight.Load(), 10))

	b.WriteString("# HELP cuberouter_overload_rejects_total Total relay requests rejected with HTTP 503 by overload protection since process start.\n")
	b.WriteString("# TYPE cuberouter_overload_rejects_total counter\n")
	writeSample(&b, "cuberouter_overload_rejects_total", "", strconv.FormatInt(procRejects.Load(), 10))

	b.WriteString("# HELP cuberouter_relay_attempts_total Total relay attempts (incl. auth/rate-limit/overload-rejected) since process start.\n")
	b.WriteString("# TYPE cuberouter_relay_attempts_total counter\n")
	writeSample(&b, "cuberouter_relay_attempts_total", "", strconv.FormatInt(procAttempts.Load(), 10))

	b.WriteString("# HELP go_goroutines Number of goroutines that currently exist.\n")
	b.WriteString("# TYPE go_goroutines gauge\n")
	writeSample(&b, "go_goroutines", "", strconv.FormatInt(int64(runtime.NumGoroutine()), 10))

	b.WriteString("# HELP go_memstats_alloc_bytes Number of bytes allocated and still in use.\n")
	b.WriteString("# TYPE go_memstats_alloc_bytes gauge\n")
	writeSample(&b, "go_memstats_alloc_bytes", "", strconv.FormatUint(mem.Alloc, 10))

	b.WriteString("# HELP go_memstats_heap_objects Number of allocated objects.\n")
	b.WriteString("# TYPE go_memstats_heap_objects gauge\n")
	writeSample(&b, "go_memstats_heap_objects", "", strconv.FormatUint(mem.HeapObjects, 10))

	return b.String()
}

// writeHistogramSeries 输出单个系列的一个直方图族：le 边界行（latencyBoundsMs
// 升序、毫秒换算秒的浮点字符串）＋ le="+Inf" 行（= 样本总量）＋ _sum ＋
// _count。total 与桶累计恒等（recordProc 与热桶同条件记账），sumSeconds 由
// 毫秒和经 secondsFloat 换算。
func writeHistogramSeries(b *strings.Builder, base, labels string, cells *[histCellCount]int64, total int64, sumSeconds float64) {
	cumulative := int64(0)
	for i, boundMs := range latencyBoundsMs {
		cumulative += cells[i]
		writeSample(b, base+"_bucket", labels+",le="+quoteLabelValue(strconv.FormatFloat(secondsFloat(boundMs), 'g', -1, 64)), strconv.FormatFloat(float64(cumulative), 'g', -1, 64))
	}
	writeSample(b, base+"_bucket", labels+",le="+quoteLabelValue("+Inf"), strconv.FormatFloat(float64(total), 'g', -1, 64))
	writeSample(b, base+"_sum", labels, strconv.FormatFloat(sumSeconds, 'g', -1, 64))
	writeSample(b, base+"_count", labels, strconv.FormatInt(total, 10))
}

// writeSample 追加一行样本：labels 非空时输出 name{labels} value，为空时输出
// 无标签的 name value（进程级单值指标不带花括号）。
func writeSample(b *strings.Builder, name, labels, value string) {
	b.WriteString(name)
	if labels != "" {
		b.WriteByte('{')
		b.WriteString(labels)
		b.WriteByte('}')
	}
	b.WriteByte(' ')
	b.WriteString(value)
	b.WriteByte('\n')
}

// quoteLabelValue 按 Prometheus 文本格式转义标签值：反斜杠、双引号、换行三类
// 必须转义，其余字符（含 Unicode）原样保留。
func quoteLabelValue(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	v = strings.ReplaceAll(v, "\n", `\n`)
	return `"` + v + `"`
}

// secondsFloat 把毫秒整数换算为秒浮点。换算本身不会产生非有限值（int64 毫秒
// 转 float64 秒远未溢出），但 exposition 输出纪律（与 billing 饱和一致的防御
// 要求）规定：任何除法/换算结果在写出前都必须做非有限检查，非法值输出 0。
func secondsFloat(ms int64) float64 {
	v := float64(ms) / 1000
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}
