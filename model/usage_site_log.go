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

	"gorm.io/gorm"
)

// SiteUsageFilter narrows a site-wide usage read. Zero values mean no filter.
type SiteUsageFilter struct {
	ActorUserId        int
	OrganizationId     int
	TokenId            int
	BillingAccountType string
}

func (filter SiteUsageFilter) apply(query *gorm.DB) *gorm.DB {
	if filter.ActorUserId > 0 {
		query = query.Where("actor_user_id = ?", filter.ActorUserId)
	}
	if filter.OrganizationId > 0 {
		query = query.Where("organization_id = ?", filter.OrganizationId)
	}
	if filter.TokenId > 0 {
		query = query.Where("token_id = ?", filter.TokenId)
	}
	switch filter.BillingAccountType {
	case "organization":
		query = query.Where("billing_account_type = ?", "organization")
	case "personal":
		// Legacy rows may carry an empty billing type; they bill personal.
		query = query.Where("(billing_account_type IS NULL OR billing_account_type = '' OR billing_account_type = 'personal')")
	}
	return query
}

// GetSiteUsageLogRowsByBucket aggregates every actor's text/fee logs by local
// day or hour, actor, billing account, key, and model. The actor is exposed in
// the responsible-user fields so the shared usage row merge keys on it.
func GetSiteUsageLogRowsByBucket(filter SiteUsageFilter, startTs, endTs, bucketSeconds int64) ([]SelfUsageLogRow, error) {
	if bucketSeconds <= 0 {
		bucketSeconds = SelfUsageDaySeconds
	}
	offset := localUTCOffsetSeconds()

	var rows []SelfUsageLogRow
	query := LOG_DB.Table("logs").
		Select("FLOOR((created_at + ?) / ?) AS bucket_key, "+
			"actor_user_id AS responsible_user_id, "+
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
			LogTypeConsume, LogTypeRefund)
	query = filter.apply(query)
	query = applySelfUsageLogRowConditions(query, startTs, endTs)
	err := query.
		Group("bucket_key, actor_user_id, billing_account_type, billing_account_id, organization_id, organization_name, token_id, token_name, model_name").
		Order("bucket_key, actor_user_id, model_name").
		Scan(&rows).Error
	if err != nil {
		common.SysError("failed to query site usage logs: " + err.Error())
		return nil, errors.New("查询用量失败")
	}
	if rows == nil {
		rows = []SelfUsageLogRow{}
	}
	return rows, nil
}
