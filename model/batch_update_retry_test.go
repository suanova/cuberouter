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
package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func batchStoreDelta(t *testing.T, type_ int, id int) int {
	t.Helper()
	batchUpdateLocks[type_].Lock()
	defer batchUpdateLocks[type_].Unlock()
	return batchUpdateStores[type_][id]
}

func batchStoreLen(t *testing.T, type_ int) int {
	t.Helper()
	batchUpdateLocks[type_].Lock()
	defer batchUpdateLocks[type_].Unlock()
	return len(batchUpdateStores[type_])
}

// 批量落库失败必须把该条增量回填缓冲、下轮重试——丢一轮就是永久幻影：
// DB 从此偏高，之后每一次水合（含 TTL 到期那次）都把丢掉的扣减复活成可用余额
// （preconsume-risk-analysis.md §3.5-③、§11）。
func TestBatchUpdateRetriesFailedEntries(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)

	user := createReserveTestUser(t, 1000)
	token := createReserveTestToken(t, 1000)

	addNewRecord(BatchUpdateTypeUserQuota, user.Id, -100)
	addNewRecord(BatchUpdateTypeTokenQuota, token.Id, -100)

	// 注入落库失败：表改名后 UPDATE 必然报错（三库通用，不依赖方言错误码）。
	require.NoError(t, DB.Exec("ALTER TABLE users RENAME TO users_batch_hidden").Error)
	require.NoError(t, DB.Exec("ALTER TABLE tokens RENAME TO tokens_batch_hidden").Error)

	batchUpdate()

	assert.Equal(t, -100, batchStoreDelta(t, BatchUpdateTypeUserQuota, user.Id), "失败条目必须回填缓冲")
	assert.Equal(t, -100, batchStoreDelta(t, BatchUpdateTypeTokenQuota, token.Id), "失败条目必须回填缓冲")

	// 恢复后重试：恰好写一次。
	require.NoError(t, DB.Exec("ALTER TABLE users_batch_hidden RENAME TO users").Error)
	require.NoError(t, DB.Exec("ALTER TABLE tokens_batch_hidden RENAME TO tokens").Error)

	batchUpdate()
	batchUpdate() // 再跑一轮：不得重复写

	assert.Equal(t, 900, getUserQuotaFromDB(t, user.Id))
	assert.Equal(t, 900, getTokenFromDB(t, token.Id).RemainQuota)
	assert.Zero(t, batchStoreLen(t, BatchUpdateTypeUserQuota), "写成功后缓冲应清空")
	assert.Zero(t, batchStoreLen(t, BatchUpdateTypeTokenQuota), "写成功后缓冲应清空")
}
