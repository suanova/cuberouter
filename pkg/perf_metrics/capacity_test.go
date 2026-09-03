package perfmetrics

import (
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capacityUpsertCall struct {
	ts       int64
	attempts int64
	rejected int64
	peak     int64
}

func recordCapacityUpserts(t *testing.T, failFirst bool) (*[]capacityUpsertCall, func()) {
	t.Helper()
	calls := &[]capacityUpsertCall{}
	original := capacityUpsert
	failNext := failFirst
	capacityUpsert = func(m *model.CapacityMetric) error {
		if failNext {
			failNext = false
			return errors.New("db down")
		}
		*calls = append(*calls, capacityUpsertCall{
			ts: m.BucketTs, attempts: m.Attempts, rejected: m.Rejected503, peak: m.InflightPeak,
		})
		return nil
	}
	return calls, func() { capacityUpsert = original }
}

// C1 修复验证：flushCompletedCapacityBuckets 必须 drain pendingCaps 里的跨界桶
// （含零计数跳过）与恰好越界的 currentCap，并替换出新的当前桶。
func TestFlushCompletedCapacityBucketsDrainsPendingAndStaleCurrent(t *testing.T) {
	calls, restore := recordCapacityUpserts(t, false)
	defer restore()

	nowStart := bucketStart(time.Now().Unix())
	plantedOld := &capacityBucket{ts: nowStart - 2*3600}
	plantedOld.attempts.Store(10)
	plantedOld.rejected.Store(3)
	plantedOld.peak.Store(7)
	zeroCount := &capacityBucket{ts: nowStart - 3600} // 全零：不应产生 upsert
	staleCurrent := &capacityBucket{ts: nowStart - 1800}
	staleCurrent.attempts.Store(4)
	staleCurrent.rejected.Store(1)
	staleCurrent.peak.Store(2)

	capacityMu.Lock()
	pendingCaps = []*capacityBucket{plantedOld, zeroCount}
	currentCap = staleCurrent
	capacityMu.Unlock()

	flushCompletedCapacityBuckets()

	require.Len(t, *calls, 2) // 非空 pending 桶 + 越界 currentCap，零计数桶被跳过
	byTs := map[int64]capacityUpsertCall{}
	for _, c := range *calls {
		byTs[c.ts] = c
	}
	gotPending, ok := byTs[plantedOld.ts]
	require.True(t, ok, "pending 跨界桶应被 drain")
	assert.Equal(t, int64(10), gotPending.attempts)
	assert.Equal(t, int64(3), gotPending.rejected)
	assert.Equal(t, int64(7), gotPending.peak)
	gotCurrent, ok := byTs[staleCurrent.ts]
	require.True(t, ok, "越界 currentCap 应被 drain（不得丢弃）")
	assert.Equal(t, int64(4), gotCurrent.attempts)
	assert.Equal(t, int64(1), gotCurrent.rejected)
	assert.Equal(t, int64(2), gotCurrent.peak)
	_, ok = byTs[zeroCount.ts]
	assert.False(t, ok, "零计数桶不应产生 upsert")

	capacityMu.Lock()
	defer capacityMu.Unlock()
	require.NotNil(t, currentCap, "currentCap 应被替换为新桶")
	assert.NotSame(t, staleCurrent, currentCap)
	assert.Greater(t, currentCap.ts, staleCurrent.ts)
	assert.Empty(t, pendingCaps, "已完结桶 drain 后 pendingCaps 应清空")
}

// M3/M2：drain 失败（DB 错误）的桶重新入 pendingCaps，下轮 flush 重试成功且
// 只 upsert 一次（值已在 Swap(0) 取出，重试不重复计数）。
func TestFlushCompletedCapacityBucketsRetriesFailedDrain(t *testing.T) {
	calls, restore := recordCapacityUpserts(t, true) // 首次调用返回错误
	defer restore()

	nowStart := bucketStart(time.Now().Unix())
	b := &capacityBucket{ts: nowStart - 3600}
	b.attempts.Store(5)
	b.rejected.Store(2)
	b.peak.Store(4)

	capacityMu.Lock()
	pendingCaps = []*capacityBucket{b}
	currentCap = nil
	capacityMu.Unlock()

	flushCompletedCapacityBuckets()
	require.Empty(t, *calls, "drain 失败不应记为成功 upsert")
	capacityMu.Lock()
	require.Equal(t, []*capacityBucket{b}, pendingCaps, "失败桶应重新入队等待下轮")
	capacityMu.Unlock()

	flushCompletedCapacityBuckets()
	require.Len(t, *calls, 1)
	assert.Equal(t, capacityUpsertCall{ts: b.ts, attempts: 5, rejected: 2, peak: 4}, (*calls)[0])
	capacityMu.Lock()
	defer capacityMu.Unlock()
	assert.Empty(t, pendingCaps)
}
