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

// GetOrganizationUsageLogRowsByBucket aggregates the organization's text/fee
// logs by local day or hour, responsible member, key, and model. A non-zero
// responsibleUserId narrows the read to one member, which is what a member
// without organization-wide visibility gets.
func GetOrganizationUsageLogRowsByBucket(organizationId int, startTs, endTs, bucketSeconds int64, responsibleUserId int) ([]SelfUsageLogRow, error) {
	if organizationId <= 0 {
		return nil, errors.New("组织 ID 无效")
	}
	if bucketSeconds <= 0 {
		bucketSeconds = SelfUsageDaySeconds
	}
	offset := localUTCOffsetSeconds()

	var rows []SelfUsageLogRow
	query := LOG_DB.Table("logs").
		Select("FLOOR((created_at + ?) / ?) AS bucket_key, "+
			"billing_account_type, billing_account_id, organization_id, organization_name, "+
			"responsible_user_id, MAX(responsible_name) AS responsible_name, "+
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
		Where("billing_account_type = ? AND billing_account_id = ? AND organization_id = ?",
			AccountContextTypeOrganization, organizationId, organizationId)
	if responsibleUserId > 0 {
		query = query.Where("responsible_user_id = ?", responsibleUserId)
	}
	query = applySelfUsageLogRowConditions(query, startTs, endTs)
	err := query.
		Group("bucket_key, billing_account_type, billing_account_id, organization_id, organization_name, responsible_user_id, token_id, token_name, model_name").
		Order("bucket_key, responsible_user_id, model_name").
		Scan(&rows).Error
	if err != nil {
		common.SysError("failed to query organization usage logs: " + err.Error())
		return nil, errors.New("查询用量失败")
	}
	if rows == nil {
		rows = []SelfUsageLogRow{}
	}
	return rows, nil
}
