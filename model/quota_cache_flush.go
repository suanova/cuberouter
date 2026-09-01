package model

import (
	"sync"

	"github.com/bytedance/gopkg/util/gopool"
)

// quotaCacheWG tracks in-flight one-shot quota/cache workers spawned through
// goQuotaCacheUpdate. Those workers read the process-global Redis state
// (common.RedisEnabled, common.RDB, ...), which tests toggle while the workers
// may still be reading it, so tests flush the group before mutating the
// globals. Perpetual background loops (e.g. InitBatchUpdater) must NOT go
// through this helper — they never complete and would block the flush forever.
var quotaCacheWG sync.WaitGroup

// goQuotaCacheUpdate spawns a one-shot cache update on the gopool and tracks it
// so WaitForQuotaCacheWorkers can wait for completion. Add is called in the
// caller goroutine before the worker is scheduled, so a concurrent Wait cannot
// observe a missing Add.
func goQuotaCacheUpdate(fn func()) {
	quotaCacheWG.Add(1)
	gopool.Go(func() {
		defer quotaCacheWG.Done()
		fn()
	})
}

// WaitForQuotaCacheWorkers blocks until all in-flight quota/cache workers
// spawned via goQuotaCacheUpdate have completed. Tests use this before toggling
// the process-global Redis state that those workers read, so the toggle write
// cannot race with an in-flight worker read.
func WaitForQuotaCacheWorkers() {
	quotaCacheWG.Wait()
}
