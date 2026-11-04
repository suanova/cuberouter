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
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// SelfUsageAsyncRow is one aggregated task/Midjourney usage row. Spend and
// status come from the source tables, never from the task/MJ billing logs.
type SelfUsageAsyncRow struct {
	DayKey             int64
	BillingAccountType string
	BillingAccountID   int
	OrganizationID     int
	OrganizationName   string
	TokenID            int
	TokenName          string
	ModelName          string
	RequestCount       int
	SuccessCount       int
	FailureCount       int
	Quota              int
}

type selfUsageAsyncKey struct {
	DayKey             int64
	BillingAccountType string
	BillingAccountID   int
	OrganizationID     int
	OrganizationName   string
	TokenID            int
	TokenName          string
	ModelName          string
}

// localDayKey mirrors the SQL day_key expression used for log aggregation.
func localDayKey(ts int64) int64 {
	return (ts + int64(localUTCOffsetSeconds())) / int64(daySeconds)
}

func selfUsageBillingType(value string) string {
	if value == "" {
		return AccountContextTypePersonal
	}
	return value
}

// GetSelfUsageTaskRows aggregates the caller's tasks by local submit day,
// billing account, key, and model.
func GetSelfUsageTaskRows(actorUserId int, startTs, endTs int64) ([]SelfUsageAsyncRow, error) {
	if actorUserId <= 0 {
		return nil, errors.New("用户 ID 无效")
	}
	var tasks []Task
	err := DB.
		Where("actor_user_id = ? AND COALESCE(NULLIF(submit_time, 0), created_at) BETWEEN ? AND ?", actorUserId, startTs, endTs).
		Select("id, task_id, token_id, token_name, billing_account_type, billing_account_id, organization_id, organization_name, status, quota, submit_time, created_at, properties").
		Find(&tasks).Error
	if err != nil {
		common.SysError("failed to query self usage tasks: " + err.Error())
		return nil, errors.New("查询用量失败")
	}

	byKey := make(map[selfUsageAsyncKey]*SelfUsageAsyncRow)
	for i := range tasks {
		task := &tasks[i]
		ts := task.SubmitTime
		if ts == 0 {
			ts = task.CreatedAt
		}
		key := selfUsageAsyncKey{
			DayKey:             localDayKey(ts),
			BillingAccountType: selfUsageBillingType(task.BillingAccountType),
			BillingAccountID:   task.BillingAccountId,
			OrganizationID:     task.OrganizationId,
			OrganizationName:   task.OrganizationName,
			TokenID:            task.TokenId,
			TokenName:          task.TokenName,
			ModelName:          task.Properties.OriginModelName,
		}
		row := byKey[key]
		if row == nil {
			row = &SelfUsageAsyncRow{
				DayKey:             key.DayKey,
				BillingAccountType: key.BillingAccountType,
				BillingAccountID:   key.BillingAccountID,
				OrganizationID:     key.OrganizationID,
				OrganizationName:   key.OrganizationName,
				TokenID:            key.TokenID,
				TokenName:          key.TokenName,
				ModelName:          key.ModelName,
			}
			byKey[key] = row
		}
		row.RequestCount++
		switch task.Status {
		case TaskStatusSuccess:
			row.SuccessCount++
		case TaskStatusFailure:
			row.FailureCount++
		}
		row.Quota += task.Quota
	}

	return hydrateSelfUsageAsyncNames(flattenSelfUsageAsyncRows(byKey)), nil
}

// GetSelfUsageMidjourneyRows aggregates the caller's MJ tasks. MJ timestamps
// are milliseconds, unlike tasks' seconds.
func GetSelfUsageMidjourneyRows(actorUserId int, startTs, endTs int64) ([]SelfUsageAsyncRow, error) {
	if actorUserId <= 0 {
		return nil, errors.New("用户 ID 无效")
	}
	const mjTimeExpr = "COALESCE(NULLIF(submit_time, 0), NULLIF(start_time, 0), finish_time)"
	var journeys []Midjourney
	err := DB.
		Where("actor_user_id = ? AND "+mjTimeExpr+" >= ? AND "+mjTimeExpr+" < ?", actorUserId, startTs*1000, (endTs+1)*1000).
		Select("id, token_id, token_name, billing_account_type, billing_account_id, organization_id, organization_name, action, status, quota, submit_time, start_time, finish_time").
		Find(&journeys).Error
	if err != nil {
		common.SysError("failed to query self usage midjourneys: " + err.Error())
		return nil, errors.New("查询用量失败")
	}

	byKey := make(map[selfUsageAsyncKey]*SelfUsageAsyncRow)
	for i := range journeys {
		journey := &journeys[i]
		tsMs := journey.SubmitTime
		if tsMs == 0 {
			tsMs = journey.StartTime
		}
		if tsMs == 0 {
			tsMs = journey.FinishTime
		}
		if tsMs <= 0 {
			continue
		}
		key := selfUsageAsyncKey{
			DayKey:             localDayKey(tsMs / 1000),
			BillingAccountType: selfUsageBillingType(journey.BillingAccountType),
			BillingAccountID:   journey.BillingAccountId,
			OrganizationID:     journey.OrganizationId,
			OrganizationName:   journey.OrganizationName,
			TokenID:            journey.TokenId,
			TokenName:          journey.TokenName,
			ModelName:          midjourneyUsageModelName(journey.Action),
		}
		row := byKey[key]
		if row == nil {
			row = &SelfUsageAsyncRow{
				DayKey:             key.DayKey,
				BillingAccountType: key.BillingAccountType,
				BillingAccountID:   key.BillingAccountID,
				OrganizationID:     key.OrganizationID,
				OrganizationName:   key.OrganizationName,
				TokenID:            key.TokenID,
				TokenName:          key.TokenName,
				ModelName:          key.ModelName,
			}
			byKey[key] = row
		}
		row.RequestCount++
		success, failure := midjourneyUsageStatusBucket(journey.Status)
		if success {
			row.SuccessCount++
		}
		if failure {
			row.FailureCount++
		}
		row.Quota += journey.Quota
	}

	return hydrateSelfUsageAsyncNames(flattenSelfUsageAsyncRows(byKey)), nil
}

func midjourneyUsageModelName(action string) string {
	normalized := strings.ToUpper(strings.TrimSpace(action))
	if normalized == "SWAP_FACE" {
		return "swap_face"
	}
	return "mj_" + strings.ToLower(normalized)
}

func midjourneyUsageStatusBucket(status string) (success, failure bool) {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "SUCCESS":
		return true, false
	case "FAILURE", "FAILED", "FAIL", "CANCELED", "CANCELLED", "CANCEL", "TIMEOUT", "TIMED_OUT", "TIME_OUT", "UNKNOWN":
		return false, true
	default:
		return false, false
	}
}

func flattenSelfUsageAsyncRows(byKey map[selfUsageAsyncKey]*SelfUsageAsyncRow) []SelfUsageAsyncRow {
	rows := make([]SelfUsageAsyncRow, 0, len(byKey))
	for _, row := range byKey {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].DayKey != rows[j].DayKey {
			return rows[i].DayKey < rows[j].DayKey
		}
		if rows[i].BillingAccountType != rows[j].BillingAccountType {
			return rows[i].BillingAccountType < rows[j].BillingAccountType
		}
		if rows[i].ModelName != rows[j].ModelName {
			return rows[i].ModelName < rows[j].ModelName
		}
		return rows[i].TokenID < rows[j].TokenID
	})
	return rows
}

func hydrateSelfUsageAsyncNames(rows []SelfUsageAsyncRow) []SelfUsageAsyncRow {
	if len(rows) == 0 {
		return rows
	}
	tokenIDs := make([]int, 0)
	organizationIDs := make([]int, 0)
	for i := range rows {
		if rows[i].TokenName == "" && rows[i].TokenID > 0 {
			tokenIDs = append(tokenIDs, rows[i].TokenID)
		}
		if rows[i].OrganizationName == "" && rows[i].OrganizationID > 0 {
			organizationIDs = append(organizationIDs, rows[i].OrganizationID)
		}
	}

	tokenNames := make(map[int]string)
	if len(tokenIDs) > 0 {
		var tokens []struct {
			Id   int
			Name string
		}
		if err := DB.Table("tokens").Select("id, name").Where("id IN ?", tokenIDs).Scan(&tokens).Error; err == nil {
			for _, token := range tokens {
				tokenNames[token.Id] = token.Name
			}
		}
	}
	organizationNames := make(map[int]string)
	if len(organizationIDs) > 0 {
		var organizations []struct {
			Id   int
			Name string
		}
		if err := DB.Table("organizations").Select("id, name").Where("id IN ?", organizationIDs).Scan(&organizations).Error; err == nil {
			for _, organization := range organizations {
				organizationNames[organization.Id] = organization.Name
			}
		}
	}

	for i := range rows {
		if rows[i].TokenName == "" {
			rows[i].TokenName = tokenNames[rows[i].TokenID]
		}
		if rows[i].OrganizationName == "" {
			rows[i].OrganizationName = organizationNames[rows[i].OrganizationID]
		}
	}
	return rows
}
