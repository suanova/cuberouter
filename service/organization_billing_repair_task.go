/*
Copyright (C) 2023-2026 QuantumNous
Copyright (C) 2026 CubeRouter

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	organizationBillingRepairTickInterval = 1 * time.Minute
	organizationBillingRepairBatchSize    = 100
)

var (
	organizationBillingRepairOnce    sync.Once
	organizationBillingRepairRunning atomic.Bool
)

// StartOrganizationBillingRepairTask 起崩溃恢复协程，回收超时未结算的组织账本会话。
//
// 只有主节点跑：会话认领靠带条件的 UPDATE 抢，多实例同时扫会互相打乱租约，
// 而组织的额度是全平台共享的，重复修复的代价是真金白银。
func StartOrganizationBillingRepairTask() {
	organizationBillingRepairOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}

		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("organization billing repair task started: tick=%s", organizationBillingRepairTickInterval))

			ticker := time.NewTicker(organizationBillingRepairTickInterval)
			defer ticker.Stop()

			// 先跑一轮再进循环：进程刚起来时积压的孤儿会话不用等一个 tick。
			runOrganizationBillingRepairOnce()
			for range ticker.C {
				runOrganizationBillingRepairOnce()
			}
		})
	})
}

func runOrganizationBillingRepairOnce() {
	// 上一轮还没跑完就跳过这一拍，避免堆积时并发扫同一批会话。
	if !organizationBillingRepairRunning.CompareAndSwap(false, true) {
		return
	}
	defer organizationBillingRepairRunning.Store(false)

	ctx := context.Background()
	totalRepaired := 0
	for {
		repaired, err := RepairExpiredOrganizationBillingSessions(organizationBillingRepairBatchSize, common.GetTimestamp())
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("organization billing repair task failed: %v", err))
			return
		}
		if repaired == 0 {
			break
		}
		totalRepaired += repaired
		// 不满一批说明已经清空，再查一次是白跑。
		if repaired < organizationBillingRepairBatchSize {
			break
		}
	}
	if common.DebugEnabled && totalRepaired > 0 {
		logger.LogDebug(ctx, "organization billing repair task: repaired_count=%d", totalRepaired)
	}
}
