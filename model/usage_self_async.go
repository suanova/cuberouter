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

	"gorm.io/gorm"
)

// SelfUsageAsyncRow is one aggregated task/Midjourney usage row. Spend and
// status come from the source tables, never from the task/MJ billing logs.
type SelfUsageAsyncRow struct {
	BucketKey          int64
	BillingAccountType string
	BillingAccountID   int
	OrganizationID     int
	OrganizationName   string
	ResponsibleUserID  int
	ResponsibleName    string
	TokenID            int
	TokenName          string
	ModelName          string
	RequestCount       int
	SuccessCount       int
	FailureCount       int
	Quota              int
}

type selfUsageAsyncKey struct {
	BucketKey          int64
	BillingAccountType string
	BillingAccountID   int
	OrganizationID     int
	OrganizationName   string
	ResponsibleUserID  int
	ResponsibleName    string
	TokenID            int
	TokenName          string
	ModelName          string
}

// usageAsyncScope selects whose rows the task and Midjourney loaders read.
// Exactly one field is set: a person's own usage, or one organization's.
type usageAsyncScope struct {
	ActorUserId    int
	OrganizationId int
}

func (scope usageAsyncScope) apply(query *gorm.DB) *gorm.DB {
	if scope.OrganizationId > 0 {
		return query.Where("billing_account_type = ? AND billing_account_id = ? AND organization_id = ?",
			AccountContextTypeOrganization, scope.OrganizationId, scope.OrganizationId)
	}
	return query.Where("actor_user_id = ?", scope.ActorUserId)
}

// localBucketKey mirrors the SQL bucket_key expression used for log aggregation.
func localBucketKey(ts, bucketSeconds int64) int64 {
	if bucketSeconds <= 0 {
		bucketSeconds = SelfUsageDaySeconds
	}
	return (ts + int64(localUTCOffsetSeconds())) / bucketSeconds
}

// usageAsyncBatchSize bounds how many task/MJ rows are held while aggregating.
// The aggregation state is a map keyed by the same dimensions as the result,
// so streaming batches keeps memory proportional to the result, not the input.
const usageAsyncBatchSize = 1000

func selfUsageBillingType(value string) string {
	if value == "" {
		return AccountContextTypePersonal
	}
	return value
}

// GetSelfUsageTaskRows aggregates the caller's tasks by local submit day,
// billing account, key, and model.
func GetSelfUsageTaskRows(actorUserId int, startTs, endTs int64) ([]SelfUsageAsyncRow, error) {
	return GetSelfUsageTaskRowsByBucket(actorUserId, startTs, endTs, SelfUsageDaySeconds)
}

// GetSelfUsageTaskRowsByBucket is GetSelfUsageTaskRows with a caller-chosen
// bucket size, used by exports that group by hour instead of by day.
func GetSelfUsageTaskRowsByBucket(actorUserId int, startTs, endTs, bucketSeconds int64) ([]SelfUsageAsyncRow, error) {
	return getUsageTaskRowsByBucket(usageAsyncScope{ActorUserId: actorUserId}, startTs, endTs, bucketSeconds, 0, false)
}

// GetOrganizationUsageTaskRowsByBucket aggregates the organization's tasks by
// local submit day or hour, responsible member, key, and model.
func GetOrganizationUsageTaskRowsByBucket(organizationId int, startTs, endTs, bucketSeconds int64, responsibleUserId int) ([]SelfUsageAsyncRow, error) {
	if organizationId <= 0 {
		return nil, errors.New("组织 ID 无效")
	}
	return getUsageTaskRowsByBucket(usageAsyncScope{OrganizationId: organizationId}, startTs, endTs, bucketSeconds, responsibleUserId, true)
}

func getUsageTaskRowsByBucket(scope usageAsyncScope, startTs, endTs, bucketSeconds int64, responsibleUserId int, groupByResponsible bool) ([]SelfUsageAsyncRow, error) {
	if scope.ActorUserId <= 0 && scope.OrganizationId <= 0 {
		return nil, errors.New("用户 ID 无效")
	}
	if bucketSeconds <= 0 {
		bucketSeconds = SelfUsageDaySeconds
	}
	var tasks []Task
	byKey := make(map[selfUsageAsyncKey]*SelfUsageAsyncRow)
	query := DB.Model(&Task{})
	query = scope.apply(query).
		Where("COALESCE(NULLIF(submit_time, 0), created_at) BETWEEN ? AND ?", startTs, endTs)
	if responsibleUserId > 0 {
		query = query.Where("responsible_user_id = ?", responsibleUserId)
	}
	err := query.
		Select("id, task_id, token_id, token_name, billing_account_type, billing_account_id, organization_id, organization_name, responsible_user_id, responsible_name, status, quota, submit_time, created_at, properties").
		FindInBatches(&tasks, usageAsyncBatchSize, func(_ *gorm.DB, _ int) error {
			aggregateUsageTasksInto(byKey, tasks, bucketSeconds, groupByResponsible)
			return nil
		}).Error
	if err != nil {
		common.SysError("failed to query usage tasks: " + err.Error())
		return nil, errors.New("查询用量失败")
	}
	return hydrateSelfUsageAsyncNames(flattenSelfUsageAsyncRows(byKey)), nil
}

// aggregateUsageTasksInto folds one batch of tasks into the shared aggregation map.
func aggregateUsageTasksInto(byKey map[selfUsageAsyncKey]*SelfUsageAsyncRow, tasks []Task, bucketSeconds int64, groupByResponsible bool) {
	for i := range tasks {
		task := &tasks[i]
		ts := task.SubmitTime
		if ts == 0 {
			ts = task.CreatedAt
		}
		responsibleUserId, responsibleName := 0, ""
		if groupByResponsible {
			responsibleUserId, responsibleName = task.ResponsibleUserId, task.ResponsibleName
		}
		key := selfUsageAsyncKey{
			BucketKey:          localBucketKey(ts, bucketSeconds),
			BillingAccountType: selfUsageBillingType(task.BillingAccountType),
			BillingAccountID:   task.BillingAccountId,
			OrganizationID:     task.OrganizationId,
			OrganizationName:   task.OrganizationName,
			ResponsibleUserID:  responsibleUserId,
			ResponsibleName:    responsibleName,
			TokenID:            task.TokenId,
			TokenName:          task.TokenName,
			ModelName:          task.Properties.OriginModelName,
		}
		row := byKey[key]
		if row == nil {
			row = &SelfUsageAsyncRow{
				BucketKey:          key.BucketKey,
				BillingAccountType: key.BillingAccountType,
				BillingAccountID:   key.BillingAccountID,
				OrganizationID:     key.OrganizationID,
				OrganizationName:   key.OrganizationName,
				ResponsibleUserID:  key.ResponsibleUserID,
				ResponsibleName:    key.ResponsibleName,
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
}

// GetSelfUsageMidjourneyRows aggregates the caller's MJ tasks. MJ timestamps
// are milliseconds, unlike tasks' seconds.
func GetSelfUsageMidjourneyRows(actorUserId int, startTs, endTs int64) ([]SelfUsageAsyncRow, error) {
	return GetSelfUsageMidjourneyRowsByBucket(actorUserId, startTs, endTs, SelfUsageDaySeconds)
}

// GetSelfUsageMidjourneyRowsByBucket is GetSelfUsageMidjourneyRows with a
// caller-chosen bucket size, used by exports that group by hour.
func GetSelfUsageMidjourneyRowsByBucket(actorUserId int, startTs, endTs, bucketSeconds int64) ([]SelfUsageAsyncRow, error) {
	return getUsageMidjourneyRowsByBucket(usageAsyncScope{ActorUserId: actorUserId}, startTs, endTs, bucketSeconds, 0, false)
}

// GetOrganizationUsageMidjourneyRowsByBucket aggregates the organization's MJ
// tasks by local submit day or hour, responsible member, key, and model.
func GetOrganizationUsageMidjourneyRowsByBucket(organizationId int, startTs, endTs, bucketSeconds int64, responsibleUserId int) ([]SelfUsageAsyncRow, error) {
	if organizationId <= 0 {
		return nil, errors.New("组织 ID 无效")
	}
	return getUsageMidjourneyRowsByBucket(usageAsyncScope{OrganizationId: organizationId}, startTs, endTs, bucketSeconds, responsibleUserId, true)
}

func getUsageMidjourneyRowsByBucket(scope usageAsyncScope, startTs, endTs, bucketSeconds int64, responsibleUserId int, groupByResponsible bool) ([]SelfUsageAsyncRow, error) {
	if scope.ActorUserId <= 0 && scope.OrganizationId <= 0 {
		return nil, errors.New("用户 ID 无效")
	}
	if bucketSeconds <= 0 {
		bucketSeconds = SelfUsageDaySeconds
	}
	const mjTimeExpr = "COALESCE(NULLIF(submit_time, 0), NULLIF(start_time, 0), finish_time)"
	var journeys []Midjourney
	byKey := make(map[selfUsageAsyncKey]*SelfUsageAsyncRow)
	query := DB.Model(&Midjourney{})
	query = scope.apply(query).
		Where(mjTimeExpr+" >= ? AND "+mjTimeExpr+" < ?", startTs*1000, (endTs+1)*1000)
	if responsibleUserId > 0 {
		query = query.Where("responsible_user_id = ?", responsibleUserId)
	}
	err := query.
		Select("id, token_id, token_name, billing_account_type, billing_account_id, organization_id, organization_name, responsible_user_id, responsible_name, action, status, quota, submit_time, start_time, finish_time").
		FindInBatches(&journeys, usageAsyncBatchSize, func(_ *gorm.DB, _ int) error {
			aggregateUsageJourneysInto(byKey, journeys, bucketSeconds, groupByResponsible)
			return nil
		}).Error
	if err != nil {
		common.SysError("failed to query usage midjourneys: " + err.Error())
		return nil, errors.New("查询用量失败")
	}
	return hydrateSelfUsageAsyncNames(flattenSelfUsageAsyncRows(byKey)), nil
}

// aggregateUsageJourneysInto folds one batch of Midjourney rows into the shared aggregation map.
func aggregateUsageJourneysInto(byKey map[selfUsageAsyncKey]*SelfUsageAsyncRow, journeys []Midjourney, bucketSeconds int64, groupByResponsible bool) {
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
		responsibleUserId, responsibleName := 0, ""
		if groupByResponsible {
			responsibleUserId, responsibleName = journey.ResponsibleUserId, journey.ResponsibleName
		}
		key := selfUsageAsyncKey{
			BucketKey:          localBucketKey(tsMs/1000, bucketSeconds),
			BillingAccountType: selfUsageBillingType(journey.BillingAccountType),
			BillingAccountID:   journey.BillingAccountId,
			OrganizationID:     journey.OrganizationId,
			OrganizationName:   journey.OrganizationName,
			ResponsibleUserID:  responsibleUserId,
			ResponsibleName:    responsibleName,
			TokenID:            journey.TokenId,
			TokenName:          journey.TokenName,
			ModelName:          midjourneyUsageModelName(journey.Action),
		}
		row := byKey[key]
		if row == nil {
			row = &SelfUsageAsyncRow{
				BucketKey:          key.BucketKey,
				BillingAccountType: key.BillingAccountType,
				BillingAccountID:   key.BillingAccountID,
				OrganizationID:     key.OrganizationID,
				OrganizationName:   key.OrganizationName,
				ResponsibleUserID:  key.ResponsibleUserID,
				ResponsibleName:    key.ResponsibleName,
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
		if rows[i].BucketKey != rows[j].BucketKey {
			return rows[i].BucketKey < rows[j].BucketKey
		}
		if rows[i].BillingAccountType != rows[j].BillingAccountType {
			return rows[i].BillingAccountType < rows[j].BillingAccountType
		}
		if rows[i].ResponsibleUserID != rows[j].ResponsibleUserID {
			return rows[i].ResponsibleUserID < rows[j].ResponsibleUserID
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
