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

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyLogUsageSource(t *testing.T) {
	assert.Equal(t, LogUsageSourceMJ, ClassifyLogUsageSource("mj_imagine", nil))
	assert.Equal(t, LogUsageSourceMJ, ClassifyLogUsageSource("swap_face", nil))
	assert.Equal(t, LogUsageSourceTask, ClassifyLogUsageSource("wan3.0-video", map[string]any{"is_task": true}))
	assert.Equal(t, LogUsageSourceTask, ClassifyLogUsageSource("wan3.0-video", map[string]any{"pre_consumed_quota": 1.0}))
	assert.Equal(t, LogUsageSourceTask, ClassifyLogUsageSource("wan3.0-video", map[string]any{"task_id": "t1"}))
	assert.Equal(t, LogUsageSourceFee, ClassifyLogUsageSource("gpt-4o", map[string]any{"violation_fee": true}))
	assert.Equal(t, LogUsageSourceText, ClassifyLogUsageSource("gpt-4o", map[string]any{"cache_tokens": 12.0}))
}

func TestBackfillLogUsageFieldsIsIdempotent(t *testing.T) {
	db := newBillingTestDB(t)
	origLog, origDB := LOG_DB, DB
	origOptions := common.OptionMap
	LOG_DB, DB = db, db
	common.OptionMap = map[string]string{}
	require.NoError(t, db.AutoMigrate(&Option{}))
	t.Cleanup(func() { LOG_DB, DB, common.OptionMap = origLog, origDB, origOptions })

	rows := []Log{
		{UserId: 1, Type: LogTypeConsume, ModelName: "gpt-4o", Other: `{"cache_tokens":15}`},
		{UserId: 1, Type: LogTypeConsume, ModelName: "wan3.0-video", Other: `{"is_task":true}`},
		{UserId: 1, Type: LogTypeRefund, ModelName: "wan3.0-video", Other: `{"task_id":"t1","reason":"x"}`},
		{UserId: 1, Type: LogTypeConsume, ModelName: "mj_imagine", Other: `{}`},
	}
	require.NoError(t, db.Create(&rows).Error)

	require.NoError(t, BackfillLogUsageFields())
	var got []Log
	require.NoError(t, db.Order("id").Find(&got).Error)
	assert.Equal(t, 15, got[0].CacheTokens)
	assert.Equal(t, LogUsageSourceText, got[0].UsageSource)
	assert.Equal(t, LogUsageSourceTask, got[1].UsageSource)
	assert.Equal(t, LogUsageSourceTask, got[2].UsageSource)
	assert.Equal(t, LogUsageSourceMJ, got[3].UsageSource)
	assert.True(t, LogUsageBackfillComplete())

	// A second run must not change or fail anything.
	require.NoError(t, BackfillLogUsageFields())
}
