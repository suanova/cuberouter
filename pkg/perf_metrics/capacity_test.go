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
	ts          int64
	attempts    int64
	rejected    int64
	rejected429 int64
	peak        int64
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
			ts: m.BucketTs, attempts: m.Attempts, rejected: m.Rejected503, rejected429: m.Rejected429, peak: m.InflightPeak,
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
	plantedOld.rejected429.Store(6)
	plantedOld.peak.Store(7)
	zeroCount := &capacityBucket{ts: nowStart - 3600} // 全零：不应产生 upsert
	only429 := &capacityBucket{ts: nowStart - 2700}   // 仅 rejected429 非零：不得被零值跳过
	only429.rejected429.Store(2)
	staleCurrent := &capacityBucket{ts: nowStart - 1800}
	staleCurrent.attempts.Store(4)
	staleCurrent.rejected.Store(1)
	staleCurrent.rejected429.Store(5)
	staleCurrent.peak.Store(2)

	capacityMu.Lock()
	pendingCaps = []*capacityBucket{plantedOld, zeroCount, only429}
	currentCap = staleCurrent
	capacityMu.Unlock()

	flushCompletedCapacityBuckets()

	require.Len(t, *calls, 3) // 非空 pending 桶 ×2 + 越界 currentCap，零计数桶被跳过
	byTs := map[int64]capacityUpsertCall{}
	for _, c := range *calls {
		byTs[c.ts] = c
	}
	gotPending, ok := byTs[plantedOld.ts]
	require.True(t, ok, "pending 跨界桶应被 drain")
	assert.Equal(t, int64(10), gotPending.attempts)
	assert.Equal(t, int64(3), gotPending.rejected)
	assert.Equal(t, int64(6), gotPending.rejected429, "pending 桶的 rejected_429 值应原样 upsert")
	assert.Equal(t, int64(7), gotPending.peak)
	gotOnly429, ok := byTs[only429.ts]
	require.True(t, ok, "仅 rejected_429 非零的桶也应被 drain")
	assert.Equal(t, int64(0), gotOnly429.attempts)
	assert.Equal(t, int64(0), gotOnly429.rejected)
	assert.Equal(t, int64(2), gotOnly429.rejected429)
	assert.Equal(t, int64(0), gotOnly429.peak)
	gotCurrent, ok := byTs[staleCurrent.ts]
	require.True(t, ok, "越界 currentCap 应被 drain（不得丢弃）")
	assert.Equal(t, int64(4), gotCurrent.attempts)
	assert.Equal(t, int64(1), gotCurrent.rejected)
	assert.Equal(t, int64(5), gotCurrent.rejected429, "越界 currentCap 的 rejected_429 值应原样 upsert")
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
	b.rejected429.Store(6)
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
	assert.Equal(t, capacityUpsertCall{ts: b.ts, attempts: 5, rejected: 2, rejected429: 6, peak: 4}, (*calls)[0], "重试 upsert 应携带完整的 rejected_429 值（原值补上，不重复计数）")
	capacityMu.Lock()
	defer capacityMu.Unlock()
	assert.Empty(t, pendingCaps)
}

// CR 竞态修复验证：写路径（relayAttemptAdd/rejectAdd/rateLimitRejectAdd）的
// 跨界搬迁与加数整体在 capacityMu 内完成——加数只落在持锁瞬间仍为当前的桶
// 上，桶一旦被 flush 摘除即不可达，drain 的清零不可能丢写。确定性（无
// goroutine）：plant 一个已跨界的 currentCap（ts 早于当前桶起点、带既有计数），
// 依次走三个内部加数路径，断言计数全部落在新当前桶、旧桶以原值进 pendingCaps
// 且未被触碰。
func TestCapacityWriteAddsRollOverStaleCurrentBucket(t *testing.T) {
	nowStart := bucketStart(time.Now().Unix())
	oldBucket := &capacityBucket{ts: nowStart - 3600}
	oldBucket.attempts.Store(5)
	oldBucket.rejected.Store(2)
	oldBucket.rejected429.Store(3)

	capacityMu.Lock()
	currentCap = oldBucket
	pendingCaps = nil
	capacityMu.Unlock()

	relayAttemptAdd()
	rejectAdd()
	rateLimitRejectAdd()

	capacityMu.Lock()
	defer capacityMu.Unlock()
	require.NotNil(t, currentCap, "跨界后应换出新当前桶")
	assert.NotSame(t, oldBucket, currentCap, "旧桶不得继续充当 currentCap")
	assert.Greater(t, currentCap.ts, oldBucket.ts, "新桶起点应晚于旧桶")
	assert.Equal(t, int64(1), currentCap.attempts.Load(), "attempt 应落在新当前桶")
	assert.Equal(t, int64(1), currentCap.rejected.Load(), "503 拒绝应落在新当前桶")
	assert.Equal(t, int64(1), currentCap.rejected429.Load(), "429 拒绝应落在新当前桶")
	require.Len(t, pendingCaps, 1, "被替换的旧桶应入 pendingCaps 等待 drain")
	assert.Same(t, oldBucket, pendingCaps[0])
	assert.Equal(t, int64(5), pendingCaps[0].attempts.Load(), "旧桶既有值原样保留，未收到脱离桶的加数")
	assert.Equal(t, int64(2), pendingCaps[0].rejected.Load())
	assert.Equal(t, int64(3), pendingCaps[0].rejected429.Load())
}
