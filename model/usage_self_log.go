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
	"errors"

	"github.com/QuantumNous/new-api/common"
)

// Self-usage exports can be grouped by local day or by local hour.
const (
	SelfUsageDaySeconds  int64 = 86400
	SelfUsageHourSeconds int64 = 3600
)

// SelfUsageLogRow is one aggregated text/fee usage row. Task and MJ billing
// logs are deliberately excluded; their spend and status come from the tasks
// and midjourneys tables.
type SelfUsageLogRow struct {
	BucketKey          int64
	BillingAccountType string
	BillingAccountID   int
	OrganizationID     int
	OrganizationName   string
	TokenID            int
	TokenName          string
	ModelName          string
	RequestCount       int
	PromptTokens       int
	CompletionTokens   int
	CacheTokens        int
	Quota              int
}

// GetSelfUsageLogRows aggregates the caller's text/fee logs by local day,
// billing account, key, and model. Fee rows add spend but never requests;
// refunds net against consume.
func GetSelfUsageLogRows(actorUserId int, startTs, endTs int64) ([]SelfUsageLogRow, error) {
	return GetSelfUsageLogRowsByBucket(actorUserId, startTs, endTs, SelfUsageDaySeconds)
}

// GetSelfUsageLogRowsByBucket is GetSelfUsageLogRows with a caller-chosen
// bucket size, used by exports that group by hour instead of by day.
func GetSelfUsageLogRowsByBucket(actorUserId int, startTs, endTs, bucketSeconds int64) ([]SelfUsageLogRow, error) {
	if actorUserId <= 0 {
		return nil, errors.New("用户 ID 无效")
	}
	if bucketSeconds <= 0 {
		bucketSeconds = SelfUsageDaySeconds
	}
	offset := localUTCOffsetSeconds()

	var rows []SelfUsageLogRow
	query := LOG_DB.Table("logs").
		Select("FLOOR((created_at + ?) / ?) AS bucket_key, "+
			"billing_account_type, billing_account_id, organization_id, organization_name, "+
			"token_id, token_name, model_name, "+
			"SUM(CASE WHEN type = ? AND usage_source <> ? THEN 1 ELSE 0 END) AS request_count, "+
			"COALESCE(SUM(CASE WHEN type = ? THEN prompt_tokens ELSE 0 END), 0) AS prompt_tokens, "+
			"COALESCE(SUM(CASE WHEN type = ? THEN completion_tokens ELSE 0 END), 0) AS completion_tokens, "+
			"COALESCE(SUM(CASE WHEN type = ? THEN cache_tokens ELSE 0 END), 0) AS cache_tokens, "+
			"COALESCE(SUM(CASE WHEN type = ? THEN quota WHEN type = ? THEN -quota ELSE 0 END), 0) AS quota",
			offset, bucketSeconds,
			LogTypeConsume, LogUsageSourceFee,
			LogTypeConsume,
			LogTypeConsume,
			LogTypeConsume,
			LogTypeConsume, LogTypeRefund).
		Where("actor_user_id = ?", actorUserId).
		Where("created_at >= ? AND created_at <= ?", startTs, endTs).
		Where("usage_source IN ?", []string{LogUsageSourceText, LogUsageSourceFee}).
		Where("type IN ?", []int{LogTypeConsume, LogTypeRefund}).
		// 滚动升级安全网：旧节点写出的日志带 DB 默认 source=text。在回填追上之前，
		// 用模型前缀和 task 标记把任务/MJ 行排除，避免文本行与任务表重复计数。
		// LIKE 里的 `_` 是通配符，必须转义，否则 mjpeg-* 这类文本模型会被误伤；
		// ClickHouse 不支持 ESCAPE 子句，改用 startsWith。
		Where("model_name <> ?", "swap_face").
		Where("COALESCE(other, '') NOT LIKE ?", `%"is_task":true%`).
		Where("COALESCE(other, '') NOT LIKE ?", `%"task_id"%`)
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		query = query.Where("NOT startsWith(model_name, 'mj_')")
	} else {
		query = query.Where("model_name NOT LIKE 'mj!_%' ESCAPE '!'")
	}
	err := query.
		Group("bucket_key, billing_account_type, billing_account_id, organization_id, organization_name, token_id, token_name, model_name").
		Order("bucket_key, model_name").
		Scan(&rows).Error
	if err != nil {
		common.SysError("failed to query self usage logs: " + err.Error())
		return nil, errors.New("查询用量失败")
	}
	if rows == nil {
		rows = []SelfUsageLogRow{}
	}
	return rows, nil
}
