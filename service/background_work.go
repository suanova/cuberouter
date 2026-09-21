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
	"sync"

	"github.com/bytedance/gopkg/util/gopool"
)

// backgroundWorkWG 跟踪本包派发的「一次性」后台 goroutine（退款落库、额度提醒、
// 性能采样等）。它们和请求 goroutine 一样会去读进程级全局——数据库句柄、
// common.RedisEnabled、common.QuotaRemindThreshold——而测试用例结束时会关闭并还原
// 这些全局，于是就成了和后台 goroutine 抢同一个全局。测试要先冲刷这一组再动全局。
//
// 常驻循环（会话修复协程、清理任务等）一律不进这里：它们不会结束，进来了冲刷就永不返回。
var backgroundWorkWG sync.WaitGroup

// goBackgroundWork 派发一个一次性后台任务。任务由 gopool 调度，调用方不等待。
// Add 在派发前于调用方 goroutine 完成，并发的 Wait 不会漏掉这一次。
func goBackgroundWork(fn func()) {
	backgroundWorkWG.Add(1)
	gopool.Go(func() {
		defer backgroundWorkWG.Done()
		fn()
	})
}

// WaitForBackgroundWork 阻塞到 goBackgroundWork 派发的一次性后台任务全部完成。
// 与 model.WaitForQuotaCacheWorkers 同理：让测试在改全局数据库句柄之前先等干净。
func WaitForBackgroundWork() {
	backgroundWorkWG.Wait()
}
