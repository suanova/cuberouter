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
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	LogUsageBackfillLastIDOption = "LogUsageBackfillLastID"
	LogUsageBackfillDoneOption   = "LogUsageBackfillComplete"
	logUsageBackfillBatchSize    = 500
)

// ClassifyLogUsageSource decides which billing ledger a historical log belongs
// to. MJ is checked first because its refund rows also carry task_id; the
// remaining task markers cover submission (is_task), settlement
// (pre_consumed_quota), and refund (task_id) rows.
func ClassifyLogUsageSource(modelName string, other map[string]any) string {
	if strings.HasPrefix(modelName, "mj_") || modelName == "swap_face" {
		return LogUsageSourceMJ
	}
	if other != nil {
		if isTask, _ := other["is_task"].(bool); isTask {
			return LogUsageSourceTask
		}
		if _, ok := other["pre_consumed_quota"]; ok {
			return LogUsageSourceTask
		}
		if _, ok := other["task_id"]; ok {
			return LogUsageSourceTask
		}
		if isFee, _ := other["violation_fee"].(bool); isFee {
			return LogUsageSourceFee
		}
	}
	return LogUsageSourceText
}

func LogUsageBackfillComplete() bool {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return common.OptionMap[LogUsageBackfillDoneOption] == "true"
}

// StartLogUsageBackfill runs the one-time backfill on the master node. A Redis
// lock keeps a misconfigured second master from racing it; a deployment without
// Redis (single-node SQLite) relies on the master check and the id watermark.
func StartLogUsageBackfill() {
	if !common.IsMasterNode || LogUsageBackfillComplete() {
		return
	}
	if common.RedisEnabled && common.RDB != nil {
		ok, err := common.RDB.SetNX(context.Background(), "log:usage-backfill:lock", common.NodeName, time.Hour).Result()
		if err != nil || !ok {
			common.SysLog("log usage backfill skipped: lock not acquired")
			return
		}
		defer func() { _ = common.RDB.Del(context.Background(), "log:usage-backfill:lock").Err() }()
	}
	go func() {
		if err := BackfillLogUsageFields(); err != nil {
			common.SysError("log usage backfill failed: " + err.Error())
			return
		}
		common.SysLog("log usage backfill completed")
	}()
}

// BackfillLogUsageFields is resumable: the last processed id lives in the
// options table, so a restart continues from the watermark instead of
// rescanning the whole log table.
func BackfillLogUsageFields() error {
	if LogUsageBackfillComplete() {
		return nil
	}
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		if err := backfillClickHouseLogUsageFields(); err != nil {
			return err
		}
		return UpdateOption(LogUsageBackfillDoneOption, "true")
	}
	lastID := logUsageBackfillLastID()
	for {
		var logs []Log
		if err := LOG_DB.Where("id > ?", lastID).Order("id asc").Limit(logUsageBackfillBatchSize).Find(&logs).Error; err != nil {
			return err
		}
		if len(logs) == 0 {
			break
		}
		for i := range logs {
			other, _ := common.StrToMap(logs[i].Other)
			cacheTokens := 0
			if v, ok := other["cache_tokens"].(float64); ok {
				cacheTokens = int(v)
			}
			if err := LOG_DB.Model(&Log{}).Where("id = ?", logs[i].Id).Updates(map[string]any{
				"cache_tokens": cacheTokens,
				"usage_source": ClassifyLogUsageSource(logs[i].ModelName, other),
			}).Error; err != nil {
				return err
			}
			lastID = int64(logs[i].Id)
		}
		if err := UpdateOption(LogUsageBackfillLastIDOption, strconv.FormatInt(lastID, 10)); err != nil {
			return err
		}
	}
	return UpdateOption(LogUsageBackfillDoneOption, "true")
}

// backfillClickHouseLogUsageFields is a single mutation: ClickHouse's display
// ids are assigned at read time, so the id watermark used on SQL engines does
// not apply here.
func backfillClickHouseLogUsageFields() error {
	return LOG_DB.Exec(`
ALTER TABLE logs UPDATE
  cache_tokens = JSONExtractInt(other, 'cache_tokens'),
  usage_source = multiIf(
    startsWith(model_name, 'mj_') OR model_name = 'swap_face', 'mj',
    JSONExtractBool(other, 'is_task') OR JSONHas(other, 'pre_consumed_quota') OR JSONHas(other, 'task_id'), 'task',
    JSONExtractBool(other, 'violation_fee'), 'fee',
    'text'
  )
WHERE 1`).Error
}

func logUsageBackfillLastID() int64 {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[LogUsageBackfillLastIDOption]
	common.OptionMapRWMutex.RUnlock()
	id, _ := strconv.ParseInt(raw, 10, 64)
	return id
}
