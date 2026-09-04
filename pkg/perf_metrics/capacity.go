package perfmetrics

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// 网关级容量计数（middleware 与过载/限流拒绝回调接线见 relay_capacity.go 与
// performance.go、model-rate-limit.go）。
// relayInflight 是进程内并发在途 gauge（RelayRequestStart/End 增减）；
// 容量桶与 perf 桶同用 bucketStart 粒度，sampleInflightLoop 每 2s 把 gauge
// CAS 进当前桶 InflightPeak（近似峰值，语义见 spec §3.2）。

var relayInflight atomic.Int64

// capacityUpsert 可注入：生产指向 model.UpsertCapacityMetric；测试替换为
// 内存记录器，避免测试依赖 DB。
var capacityUpsert = model.UpsertCapacityMetric

type capacityBucket struct {
	ts          int64
	attempts    atomic.Int64
	rejected    atomic.Int64
	rejected429 atomic.Int64
	peak        atomic.Int64
}

var (
	capacityMu sync.Mutex
	currentCap *capacityBucket
	// pendingCaps 收纳已跨界的完结桶等待 flush drain。桶跨界即入队、不得丢弃：
	// 否则 sampleInflightLoop 会在边界后 ≤2s 内让上一桶不可达，而 flush 只检查
	// currentCap 时会整桶漏 drain（C1 修复）。
	pendingCaps []*capacityBucket
)

// currentCapacityBucket 返回当前桶；跨界时把旧桶移入 pendingCaps 等待 drain。
func currentCapacityBucket() *capacityBucket {
	now := bucketStart(time.Now().Unix())
	capacityMu.Lock()
	defer capacityMu.Unlock()
	if currentCap == nil || currentCap.ts != now {
		if currentCap != nil {
			pendingCaps = append(pendingCaps, currentCap)
		}
		currentCap = &capacityBucket{ts: now}
	}
	return currentCap
}

func RelayRequestStart() { relayInflight.Add(1) }

func RelayRequestEnd() {
	relayInflight.Add(-1)
	currentCapacityBucket().attempts.Add(1)
	procAttempts.Add(1) // 进程级导出 counter（export.go 声明），进程存活期累计
}

func RelayInflight() int64 { return relayInflight.Load() }

func RecordOverloadReject() {
	currentCapacityBucket().rejected.Add(1)
	procRejects.Add(1) // 进程级导出 counter（export.go 声明）
}

// RecordRateLimitReject 记录一次限流 429 拒绝（调用点：model-rate-limit.go 的
// 429 分支）：当前容量桶 rejected429 +1，drain 时随 rejected_503 一并 upsert
// 到 rejected_429 列。与 RecordOverloadReject 同口径——限流中间件在所有挂载
// 点都位于 RelayCapacity 之后（见 relay_capacity.go 注释），被拒请求已计入
// attempts，故 rejected429 ⊆ attempts。429 不做进程级 Prometheus counter
// （rejected_503 已覆盖过载拒绝出口，429 属限流侧面，spec §6 不导出）。
func RecordRateLimitReject() {
	currentCapacityBucket().rejected429.Add(1)
}

// sampleInflightLoop 每 2s 把在途并发 CAS 进当前桶峰值。
func sampleInflightLoop() {
	for {
		time.Sleep(2 * time.Second)
		b := currentCapacityBucket()
		v := relayInflight.Load()
		for {
			cur := b.peak.Load()
			if v <= cur || b.peak.CompareAndSwap(cur, v) {
				break
			}
		}
	}
}

// flushCompletedCapacityBuckets drain 所有早于当前桶起点的完结容量桶：
// pendingCaps 中的跨界桶 + 可能恰在采样 tick 与 flush 之间越界的 currentCap
// （替换成新桶）。由 flushLoop 在 flushCompletedBuckets 之后调用。
// drain 失败（DB 错误）的桶重新入 pendingCaps，下轮 flush 重试。
//
// 已知残余竞态（亚微秒窗口，近似语义下可接受，见 spec §3.2）：某请求在桶
// 被 drain 之前恰好取到旧桶指针、drain 清零之后才 Add(1)，该次计数落入已
// 完结桶而丢失——桶一旦 drain 成功不再二次读取。
func flushCompletedCapacityBuckets() {
	current := bucketStart(time.Now().Unix())
	capacityMu.Lock()
	var toDrain []*capacityBucket
	keepPending := pendingCaps[:0]
	for _, b := range pendingCaps {
		if b.ts < current {
			toDrain = append(toDrain, b)
		} else {
			keepPending = append(keepPending, b)
		}
	}
	pendingCaps = keepPending
	if currentCap != nil && currentCap.ts < current {
		toDrain = append(toDrain, currentCap)
		currentCap = &capacityBucket{ts: current}
	}
	capacityMu.Unlock()
	for _, b := range toDrain {
		if !drainCapacityBucket(b) {
			capacityMu.Lock()
			pendingCaps = append(pendingCaps, b) // 值未清零（见 drainCapacityBucket），下轮原值重试
			capacityMu.Unlock()
		}
	}
}

// drainCapacityBucket 读取桶值并 upsert：成功（含空桶跳过）返回 true 并清零；
// DB 失败返回 false 且不清零，桶保持原值重新入队——重试必须真正补上数据
// （对应 perf flush 失败经 addCounters 回填的语义，见 M1）。清零与 Swap(0)
// 一样会丢清空瞬间晚到的写，属注释中已接受的残余竞态窗口。
func drainCapacityBucket(b *capacityBucket) bool {
	attempts, rejected, rejected429, peak := b.attempts.Load(), b.rejected.Load(), b.rejected429.Load(), b.peak.Load()
	if attempts == 0 && rejected == 0 && rejected429 == 0 && peak == 0 {
		return true
	}
	if err := capacityUpsert(&model.CapacityMetric{
		BucketTs: b.ts, Attempts: attempts, Rejected503: rejected, Rejected429: rejected429, InflightPeak: peak,
	}); err != nil {
		common.SysError("failed to flush capacity bucket: " + err.Error())
		return false
	}
	b.attempts.Store(0)
	b.rejected.Store(0)
	b.rejected429.Store(0)
	b.peak.Store(0)
	return true
}
