package perfmetrics

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// 网关级容量计数（middleware 与过载拒绝回调接线见 Task 4）。
// relayInflight 是进程内并发在途 gauge（RelayRequestStart/End 增减）；
// 容量桶与 perf 桶同用 bucketStart 粒度，sampleInflightLoop 每 2s 把 gauge
// CAS 进当前桶 InflightPeak（近似峰值，语义见 spec §3.2）。

var relayInflight atomic.Int64

type capacityBucket struct {
	ts       int64
	attempts atomic.Int64
	rejected atomic.Int64
	peak     atomic.Int64
}

var (
	capacityMu sync.Mutex // 保护 currentCap 指针切换
	currentCap *capacityBucket
)

func currentCapacityBucket() *capacityBucket {
	now := bucketStart(time.Now().Unix())
	capacityMu.Lock()
	defer capacityMu.Unlock()
	if currentCap == nil || currentCap.ts != now {
		currentCap = &capacityBucket{ts: now}
	}
	return currentCap
}

func RelayRequestStart() { relayInflight.Add(1) }

func RelayRequestEnd() {
	relayInflight.Add(-1)
	currentCapacityBucket().attempts.Add(1)
}

func RelayInflight() int64 { return relayInflight.Load() }

func RecordOverloadReject() { currentCapacityBucket().rejected.Add(1) }

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

// flushCompletedCapacityBuckets drain 并 upsert 所有早于当前桶起点的完结容量桶。
// 由 flushLoop 在 flushCompletedBuckets 之后调用。
func flushCompletedCapacityBuckets() {
	current := bucketStart(time.Now().Unix())
	for {
		capacityMu.Lock()
		b := currentCap
		if b == nil || b.ts >= current {
			capacityMu.Unlock()
			return
		}
		currentCap = &capacityBucket{ts: current}
		capacityMu.Unlock()
		drainCapacityBucket(b)
	}
}

// drainCapacityBucket 交换清零计数并增量 upsert；换出后的桶不再有写入方
// （采样与计数都只写 currentCapacityBucket 返回的当前桶）。
func drainCapacityBucket(b *capacityBucket) {
	if b == nil {
		return
	}
	attempts, rejected, peak := b.attempts.Swap(0), b.rejected.Swap(0), b.peak.Swap(0)
	if attempts == 0 && rejected == 0 && peak == 0 {
		return
	}
	if err := model.UpsertCapacityMetric(&model.CapacityMetric{
		BucketTs: b.ts, Attempts: attempts, Rejected503: rejected, InflightPeak: peak,
	}); err != nil {
		common.SysError("failed to flush capacity bucket: " + err.Error())
	}
}
