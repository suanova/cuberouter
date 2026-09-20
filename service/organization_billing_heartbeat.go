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
	"time"

	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/bytedance/gopkg/util/gopool"
)

// StartOrganizationBillingSessionHeartbeat 为一条在途的组织账本会话起续租协程。
//
// 长流式请求可能跑几十分钟，而会话 TTL 只有流式超时的两倍。没有续租的话，
// 修复协程会在请求还在进行时就把这笔预扣当成崩溃残留退掉，
// 用户等于白嫖了一次调用，且结算时账已经不平。
//
// 返回的 stop 函数是幂等的，调用方 defer 即可，重复调用不会 panic。
func StartOrganizationBillingSessionHeartbeat(relayInfo *relaycommon.RelayInfo) func() {
	if relayInfo == nil || relayInfo.OrganizationBillingSessionId == 0 {
		return func() {}
	}
	ttlSeconds := organizationBillingSessionTTLSeconds()
	interval := time.Duration(ttlSeconds/2) * time.Second
	if interval < time.Second {
		interval = time.Second
	}

	done := make(chan struct{})
	var closeOnce sync.Once
	gopool.Go(func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := TouchOrganizationBillingSession(relayInfo, ttlSeconds); err != nil {
					logger.LogWarn(context.Background(), fmt.Sprintf("organization billing heartbeat failed: %v", err))
				}
			case <-done:
				return
			}
		}
	})

	return func() {
		closeOnce.Do(func() {
			close(done)
		})
	}
}
