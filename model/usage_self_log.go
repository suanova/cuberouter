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

// SelfUsageLogRow is one aggregated text/fee usage row. Task and MJ billing
// logs are deliberately excluded; their spend and status come from the tasks
// and midjourneys tables.
type SelfUsageLogRow struct {
	DayKey             int64
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
	if actorUserId <= 0 {
		return nil, errors.New("用户 ID 无效")
	}
	offset := localUTCOffsetSeconds()

	var rows []SelfUsageLogRow
	err := LOG_DB.Table("logs").
		Select("FLOOR((created_at + ?) / ?) AS day_key, "+
			"billing_account_type, billing_account_id, organization_id, organization_name, "+
			"token_id, token_name, model_name, "+
			"SUM(CASE WHEN type = ? AND usage_source <> ? THEN 1 ELSE 0 END) AS request_count, "+
			"COALESCE(SUM(CASE WHEN type = ? THEN prompt_tokens ELSE 0 END), 0) AS prompt_tokens, "+
			"COALESCE(SUM(CASE WHEN type = ? THEN completion_tokens ELSE 0 END), 0) AS completion_tokens, "+
			"COALESCE(SUM(CASE WHEN type = ? THEN cache_tokens ELSE 0 END), 0) AS cache_tokens, "+
			"COALESCE(SUM(CASE WHEN type = ? THEN quota WHEN type = ? THEN -quota ELSE 0 END), 0) AS quota",
			offset, daySeconds,
			LogTypeConsume, LogUsageSourceFee,
			LogTypeConsume,
			LogTypeConsume,
			LogTypeConsume,
			LogTypeConsume, LogTypeRefund).
		Where("actor_user_id = ?", actorUserId).
		Where("created_at >= ? AND created_at <= ?", startTs, endTs).
		Where("usage_source IN ?", []string{LogUsageSourceText, LogUsageSourceFee}).
		Where("type IN ?", []int{LogTypeConsume, LogTypeRefund}).
		Group("day_key, billing_account_type, billing_account_id, organization_id, organization_name, token_id, token_name, model_name").
		Order("day_key, model_name").
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
