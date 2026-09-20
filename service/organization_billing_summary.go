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
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

type OrganizationMemberBillingItem struct {
	ResponsibleUserId int `json:"responsible_user_id"`
	Quota             int `json:"quota"`
	RequestCount      int `json:"request_count"`
	TokenCount        int `json:"token_count"`
}

type OrganizationBillingSummary struct {
	Quota             int                             `json:"quota"`
	UsedQuota         int                             `json:"used_quota"`
	AvailableQuota    int                             `json:"available_quota"`
	CurrentMonthQuota int                             `json:"current_month_quota"`
	OrganizationKeys  int64                           `json:"organization_keys"`
	Members           []OrganizationMemberBillingItem `json:"members"`
}

type OrganizationMemberBillingSummary struct {
	ResponsibleUserId   int `json:"responsible_user_id"`
	ResponsibleKeyCount int `json:"responsible_key_count"`
	CurrentMonthQuota   int `json:"current_month_quota"`
	RequestCount        int `json:"request_count"`
}

type OrganizationBillingUserSummaryRequest struct {
	Month      string
	StartMonth string
	EndMonth   string
}

type OrganizationBillingUserSummaryItem struct {
	ResponsibleUserId      int    `json:"responsible_user_id"`
	ResponsibleUsername    string `json:"responsible_username"`
	ResponsibleDisplayName string `json:"responsible_display_name"`
	Role                   string `json:"role"`
	Status                 string `json:"status"`
	Quota                  int    `json:"quota"`
	RequestCount           int    `json:"request_count"`
	PromptTokens           int    `json:"prompt_tokens"`
	CompletionTokens       int    `json:"completion_tokens"`
	TokenCount             int    `json:"token_count"`
}

type OrganizationBillingUserSummaryResponse struct {
	Month      string                               `json:"month"`
	MonthStart int64                                `json:"month_start"`
	MonthEnd   int64                                `json:"month_end"`
	Items      []OrganizationBillingUserSummaryItem `json:"items"`
}

type OrganizationBillingMonthlySummaryRequest struct {
	Months int
}

type OrganizationBillingMonthlySummaryItem struct {
	Month                string `json:"month"`
	MonthStart           int64  `json:"month_start"`
	MonthEnd             int64  `json:"month_end"`
	Quota                int    `json:"quota"`
	RequestCount         int    `json:"request_count"`
	PromptTokens         int    `json:"prompt_tokens"`
	CompletionTokens     int    `json:"completion_tokens"`
	TokenCount           int    `json:"token_count"`
	ResponsibleUserCount int    `json:"responsible_user_count"`
}

type OrganizationBillingMonthlySummaryResponse struct {
	Items []OrganizationBillingMonthlySummaryItem `json:"items"`
}

type OrganizationBillingDetailListRequest struct {
	Offset          int
	Limit           int
	Month           string
	TokenName       string
	ResponsibleName string
	ModelName       string
	Group           string
	RequestId       string
	StartTimestamp  int64
	EndTimestamp    int64
}

func GetOrganizationBillingSummary(operatorUserId, organizationId int, accessMode string) (*OrganizationBillingSummary, error) {
	actor, err := GetOrganizationActorContextForAccessMode(operatorUserId, organizationId, accessMode, true)
	if err != nil {
		return nil, err
	}
	if !actor.Capabilities.CanViewOrganizationBillingSummary {
		return nil, errors.New("permission denied")
	}
	organization := actor.Organization
	monthStart := currentMonthStartTimestamp()
	var monthQuota int
	if err := organizationBillingRecordUsageQuery(model.DB.Model(&model.OrganizationBillingRecord{}), organizationId).
		Where("created_at >= ?", monthStart).
		Select("COALESCE(SUM(usage_quota), 0)").
		Scan(&monthQuota).Error; err != nil {
		return nil, err
	}
	var keyCount int64
	if err := model.DB.Model(&model.Token{}).Where("scope_type = ? AND organization_id = ?", model.TokenScopeOrganization, organizationId).Count(&keyCount).Error; err != nil {
		return nil, err
	}
	members, err := organizationMemberBillingItems(organizationId, monthStart)
	if err != nil {
		return nil, err
	}
	return &OrganizationBillingSummary{Quota: organization.Quota, UsedQuota: organization.UsedQuota, AvailableQuota: organization.Quota - organization.UsedQuota, CurrentMonthQuota: monthQuota, OrganizationKeys: keyCount, Members: members}, nil
}

func GetMyOrganizationMemberBilling(operatorUserId, organizationId int, accessMode string) (*OrganizationMemberBillingSummary, error) {
	actor, err := GetOrganizationActorContextForAccessMode(operatorUserId, organizationId, accessMode, true)
	if err != nil {
		return nil, err
	}
	if !actor.Capabilities.CanViewOrganization || !actor.Capabilities.CanViewOrganizationUsage {
		return nil, errors.New("permission denied")
	}
	monthStart := currentMonthStartTimestamp()
	var keyCount int64
	if err := model.DB.Model(&model.Token{}).Where("scope_type = ? AND organization_id = ? AND responsible_user_id = ?", model.TokenScopeOrganization, organizationId, operatorUserId).Count(&keyCount).Error; err != nil {
		return nil, err
	}
	var row struct {
		Quota        int
		RequestCount int
	}
	if err := organizationBillingRecordUsageQuery(model.DB.Model(&model.OrganizationBillingRecord{}), organizationId).
		Where("responsible_user_id = ? AND created_at >= ?", operatorUserId, monthStart).
		Select("COALESCE(SUM(usage_quota), 0) AS quota, COALESCE(SUM(CASE WHEN record_type = ? AND usage_quota > 0 THEN 1 ELSE 0 END), 0) AS request_count", model.OrganizationBillingRecordTypeSettle).
		Scan(&row).Error; err != nil {
		return nil, err
	}
	return &OrganizationMemberBillingSummary{ResponsibleUserId: operatorUserId, ResponsibleKeyCount: int(keyCount), CurrentMonthQuota: row.Quota, RequestCount: row.RequestCount}, nil
}

func currentMonthStartTimestamp() int64 {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()
}

func organizationMemberBillingItems(organizationId int, monthStart int64) ([]OrganizationMemberBillingItem, error) {
	var rows []OrganizationMemberBillingItem
	if err := organizationBillingRecordUsageQuery(model.DB.Model(&model.OrganizationBillingRecord{}), organizationId).
		Select("responsible_user_id, COALESCE(SUM(usage_quota), 0) AS quota, COALESCE(SUM(CASE WHEN record_type = ? AND usage_quota > 0 THEN 1 ELSE 0 END), 0) AS request_count", model.OrganizationBillingRecordTypeSettle).
		Where("created_at >= ?", monthStart).
		Group("responsible_user_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return rows, nil
	}
	type responsibleTokenCount struct {
		ResponsibleUserId int
		TokenCount        int64
	}
	var tokenCounts []responsibleTokenCount
	if err := model.DB.Model(&model.Token{}).
		Select("responsible_user_id, COUNT(*) AS token_count").
		Where("scope_type = ? AND organization_id = ?", model.TokenScopeOrganization, organizationId).
		Group("responsible_user_id").
		Scan(&tokenCounts).Error; err != nil {
		return nil, err
	}
	tokenCountByResponsible := make(map[int]int64, len(tokenCounts))
	for _, count := range tokenCounts {
		tokenCountByResponsible[count.ResponsibleUserId] = count.TokenCount
	}
	for i := range rows {
		rows[i].TokenCount = int(tokenCountByResponsible[rows[i].ResponsibleUserId])
	}
	return rows, nil
}

func ListOrganizationBillingUserSummaries(operatorUserId, organizationId int, accessMode string, req OrganizationBillingUserSummaryRequest) (*OrganizationBillingUserSummaryResponse, error) {
	actor, err := GetOrganizationActorContextForAccessMode(operatorUserId, organizationId, accessMode, true)
	if err != nil {
		return nil, err
	}
	month, monthStart, monthEnd, err := resolveOrganizationBillingUserSummaryRange(req)
	if err != nil {
		return nil, err
	}
	query := model.DB.Table("organization_members").
		Select("organization_members.user_id AS responsible_user_id, users.username AS responsible_username, users.display_name AS responsible_display_name, organization_members.role, organization_members.status").
		Joins("LEFT JOIN users ON users.id = organization_members.user_id").
		Where("organization_members.organization_id = ? AND organization_members.status NOT IN ?", organizationId, []string{model.OrganizationMemberStatusRemoved, model.OrganizationMemberStatusExited})
	if !actor.Capabilities.CanViewOrganizationWideData {
		query = query.Where("organization_members.user_id = ?", operatorUserId)
	}
	var items []OrganizationBillingUserSummaryItem
	if err := query.Order("organization_members.id asc").Scan(&items).Error; err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return &OrganizationBillingUserSummaryResponse{Month: month, MonthStart: monthStart, MonthEnd: monthEnd - 1, Items: items}, nil
	}

	// 批量查询所有成员的聚合统计（单次 SQL，替代 N+1 循环）
	type userAggregate struct {
		ResponsibleUserId int
		Quota             int
		RequestCount      int
		PromptTokens      int
		CompletionTokens  int
	}
	var aggregates []userAggregate
	aggQuery := organizationBillingRecordUsageQuery(organizationBillingRecordBaseQuery(model.DB.Model(&model.OrganizationBillingRecord{}), actor, organizationId, operatorUserId), organizationId).
		Where("created_at >= ? AND created_at < ?", monthStart, monthEnd).
		Select("responsible_user_id, COALESCE(SUM(usage_quota), 0) AS quota, COALESCE(SUM(CASE WHEN record_type = ? AND usage_quota > 0 THEN 1 ELSE 0 END), 0) AS request_count, COALESCE(SUM(CASE WHEN record_type = ? THEN prompt_tokens ELSE 0 END), 0) AS prompt_tokens, COALESCE(SUM(CASE WHEN record_type = ? THEN completion_tokens ELSE 0 END), 0) AS completion_tokens", model.OrganizationBillingRecordTypeSettle, model.OrganizationBillingRecordTypeSettle, model.OrganizationBillingRecordTypeSettle).
		Group("responsible_user_id")
	if err := aggQuery.Scan(&aggregates).Error; err != nil {
		return nil, err
	}
	aggMap := make(map[int]userAggregate, len(aggregates))
	for _, a := range aggregates {
		aggMap[a.ResponsibleUserId] = a
	}

	// 批量查询每个成员使用的不同 token 数（单次 SQL）
	type userTokenCount struct {
		ResponsibleUserId int
		TokenCount        int64
	}
	var tokenCounts []userTokenCount
	tokenQuery := organizationBillingRecordBaseQuery(model.DB.Model(&model.OrganizationBillingRecord{}), actor, organizationId, operatorUserId).
		Where("created_at >= ? AND created_at < ?", monthStart, monthEnd).
		Select("responsible_user_id, COUNT(DISTINCT CASE WHEN record_type = ? AND token_id > 0 THEN token_id END) AS token_count", model.OrganizationBillingRecordTypeSettle).
		Group("responsible_user_id")
	if err := tokenQuery.Scan(&tokenCounts).Error; err != nil {
		return nil, err
	}
	tokenCountMap := make(map[int]int64, len(tokenCounts))
	for _, tc := range tokenCounts {
		tokenCountMap[tc.ResponsibleUserId] = tc.TokenCount
	}

	// 内存 JOIN 回成员列表
	for i := range items {
		if a, ok := aggMap[items[i].ResponsibleUserId]; ok {
			items[i].Quota = a.Quota
			items[i].RequestCount = a.RequestCount
			items[i].PromptTokens = a.PromptTokens
			items[i].CompletionTokens = a.CompletionTokens
		}
		items[i].TokenCount = int(tokenCountMap[items[i].ResponsibleUserId])
	}
	return &OrganizationBillingUserSummaryResponse{Month: month, MonthStart: monthStart, MonthEnd: monthEnd - 1, Items: items}, nil
}

func ListOrganizationBillingMonthlySummaries(operatorUserId, organizationId int, accessMode string, req OrganizationBillingMonthlySummaryRequest) (*OrganizationBillingMonthlySummaryResponse, error) {
	actor, err := GetOrganizationActorContextForAccessMode(operatorUserId, organizationId, accessMode, true)
	if err != nil {
		return nil, err
	}
	organization := actor.Organization
	months := normalizeOrganizationBillingMonths(req.Months)
	now := time.Now()
	currentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	organizationCreatedMonth := organizationBillingMonthStart(organization.CreatedAt, now.Location())

	// 先确定所有需要查询的月份区间
	type monthBucket struct {
		month     string
		startUnix int64
		endUnix   int64
	}
	var buckets []monthBucket
	for i := 0; i < months; i++ {
		monthStartTime := currentMonth.AddDate(0, -i, 0)
		if monthStartTime.Before(organizationCreatedMonth) {
			break
		}
		monthEndTime := monthStartTime.AddDate(0, 1, 0)
		buckets = append(buckets, monthBucket{
			month:     monthStartTime.Format("2006-01"),
			startUnix: monthStartTime.Unix(),
			endUnix:   monthEndTime.Unix(),
		})
	}
	if len(buckets) == 0 {
		return &OrganizationBillingMonthlySummaryResponse{Items: []OrganizationBillingMonthlySummaryItem{}}, nil
	}

	type monthAggregate struct {
		Quota                int
		RequestCount         int
		PromptTokens         int
		CompletionTokens     int
		TokenCount           int
		ResponsibleUserCount int
	}
	items := make([]OrganizationBillingMonthlySummaryItem, 0, len(buckets))
	for _, b := range buckets {
		var aggregate monthAggregate
		query := organizationBillingRecordUsageQuery(organizationBillingRecordBaseQuery(model.DB.Model(&model.OrganizationBillingRecord{}), actor, organizationId, operatorUserId), organizationId).
			Where("created_at >= ? AND created_at < ?", b.startUnix, b.endUnix).
			Select("COALESCE(SUM(usage_quota), 0) AS quota, COALESCE(SUM(CASE WHEN record_type = ? AND usage_quota > 0 THEN 1 ELSE 0 END), 0) AS request_count, COALESCE(SUM(CASE WHEN record_type = ? THEN prompt_tokens ELSE 0 END), 0) AS prompt_tokens, COALESCE(SUM(CASE WHEN record_type = ? THEN completion_tokens ELSE 0 END), 0) AS completion_tokens, COUNT(DISTINCT CASE WHEN record_type = ? AND token_id > 0 THEN token_id END) AS token_count, COUNT(DISTINCT CASE WHEN record_type = ? AND usage_quota > 0 AND responsible_user_id > 0 THEN responsible_user_id END) AS responsible_user_count", model.OrganizationBillingRecordTypeSettle, model.OrganizationBillingRecordTypeSettle, model.OrganizationBillingRecordTypeSettle, model.OrganizationBillingRecordTypeSettle, model.OrganizationBillingRecordTypeSettle)
		if err := query.Scan(&aggregate).Error; err != nil {
			return nil, err
		}
		items = append(items, OrganizationBillingMonthlySummaryItem{
			Month:                b.month,
			MonthStart:           b.startUnix,
			MonthEnd:             b.endUnix - 1,
			Quota:                aggregate.Quota,
			RequestCount:         aggregate.RequestCount,
			PromptTokens:         aggregate.PromptTokens,
			CompletionTokens:     aggregate.CompletionTokens,
			TokenCount:           aggregate.TokenCount,
			ResponsibleUserCount: aggregate.ResponsibleUserCount,
		})
	}
	return &OrganizationBillingMonthlySummaryResponse{Items: items}, nil
}

func ListOrganizationBillingDetails(operatorUserId, organizationId int, accessMode string, req OrganizationBillingDetailListRequest) ([]*model.OrganizationBillingRecord, int64, error) {
	actor, err := GetOrganizationActorContextForAccessMode(operatorUserId, organizationId, accessMode, true)
	if err != nil {
		return nil, 0, err
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	tx, err := applyOrganizationBillingDetailFilters(organizationBillingRecordBaseQuery(model.DB.Model(&model.OrganizationBillingRecord{}), actor, organizationId, operatorUserId), actor, req)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []*model.OrganizationBillingRecord
	if err := tx.Order("id desc").Limit(req.Limit).Offset(req.Offset).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	if err := hydrateOrganizationBillingRecordResponsibleUsers(records); err != nil {
		return nil, 0, err
	}
	if err := hydrateOrganizationBillingRecordLedgerQuotaDeltas(records); err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func hydrateOrganizationBillingRecordLedgerQuotaDeltas(records []*model.OrganizationBillingRecord) error {
	if len(records) == 0 {
		return nil
	}
	settleSessionIds := make([]int, 0, len(records))
	for _, record := range records {
		if record.RecordType == model.OrganizationBillingRecordTypeSettle && record.SessionId > 0 {
			settleSessionIds = append(settleSessionIds, record.SessionId)
		}
	}
	preConsumedSessionIds := make(map[int]struct{}, len(settleSessionIds))
	if len(settleSessionIds) > 0 {
		var sessionIds []int
		if err := model.DB.Model(&model.OrganizationBillingRecord{}).
			Where("organization_id = ? AND record_type = ? AND session_id IN ?", records[0].OrganizationId, model.OrganizationBillingRecordTypePreConsume, settleSessionIds).
			Pluck("session_id", &sessionIds).Error; err != nil {
			return err
		}
		for _, sessionId := range sessionIds {
			preConsumedSessionIds[sessionId] = struct{}{}
		}
	}
	for _, record := range records {
		switch record.RecordType {
		case model.OrganizationBillingRecordTypeAdjustment:
			record.LedgerQuotaDelta = record.QuotaDelta
		case model.OrganizationBillingRecordTypeSettle:
			if _, ok := preConsumedSessionIds[record.SessionId]; ok {
				record.LedgerQuotaDelta = record.UsedQuotaDelta
			} else {
				record.LedgerQuotaDelta = record.UsageQuota
			}
		default:
			record.LedgerQuotaDelta = record.UsedQuotaDelta
		}
	}
	return nil
}

func resolveOrganizationBillingUserSummaryRange(req OrganizationBillingUserSummaryRequest) (string, int64, int64, error) {
	if req.StartMonth != "" || req.EndMonth != "" {
		startMonth := req.StartMonth
		endMonth := req.EndMonth
		if startMonth == "" {
			startMonth = endMonth
		}
		if endMonth == "" {
			endMonth = startMonth
		}
		start, _, err := parseOrganizationBillingMonth(startMonth)
		if err != nil {
			return "", 0, 0, err
		}
		_, end, err := parseOrganizationBillingMonth(endMonth)
		if err != nil {
			return "", 0, 0, err
		}
		if start >= end {
			return "", 0, 0, errors.New("invalid billing month range")
		}
		return startMonth + "~" + endMonth, start, end, nil
	}
	month := req.Month
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	start, end, err := parseOrganizationBillingMonth(month)
	if err != nil {
		return "", 0, 0, err
	}
	return month, start, end, nil
}

func normalizeOrganizationBillingMonths(months int) int {
	if months <= 0 {
		return 12
	}
	if months > 36 {
		return 36
	}
	return months
}

func organizationBillingMonthStart(timestamp int64, location *time.Location) time.Time {
	if timestamp <= 0 {
		return time.Time{}
	}
	createdAt := time.Unix(timestamp, 0).In(location)
	return time.Date(createdAt.Year(), createdAt.Month(), 1, 0, 0, 0, 0, location)
}

func organizationBillingRecordUsageQuery(tx *gorm.DB, organizationId int) *gorm.DB {
	settledSessionIds := model.DB.Model(&model.OrganizationBillingRecord{}).
		Select("session_id").
		Where("organization_id = ? AND record_type = ? AND session_id > 0", organizationId, model.OrganizationBillingRecordTypeSettle)
	return tx.Where("organization_id = ?", organizationId).
		Where("record_type = ? OR (record_type = ? AND session_id IN (?))", model.OrganizationBillingRecordTypeSettle, model.OrganizationBillingRecordTypeRefund, settledSessionIds)
}

func organizationBillingRecordBaseQuery(tx *gorm.DB, actor *OrganizationActorContext, organizationId int, operatorUserId int) *gorm.DB {
	tx = tx.Where("organization_id = ?", organizationId)
	if actor == nil || !actor.Capabilities.CanViewOrganizationWideData {
		tx = tx.Where("responsible_user_id = ?", operatorUserId)
	}
	return tx
}

func applyOrganizationBillingDetailFilters(tx *gorm.DB, actor *OrganizationActorContext, req OrganizationBillingDetailListRequest) (*gorm.DB, error) {
	if req.StartTimestamp > 0 || req.EndTimestamp > 0 {
		if req.StartTimestamp > 0 {
			tx = tx.Where("created_at >= ?", req.StartTimestamp)
		}
		if req.EndTimestamp > 0 {
			tx = tx.Where("created_at <= ?", req.EndTimestamp)
		}
	} else if req.Month != "" {
		start, end, err := parseOrganizationBillingMonth(req.Month)
		if err != nil {
			return nil, err
		}
		tx = tx.Where("created_at >= ? AND created_at < ?", start, end)
	}
	if req.TokenName != "" {
		tx = tx.Where("token_name = ?", req.TokenName)
	}
	if actor != nil && actor.Capabilities.CanViewOrganizationWideData && req.ResponsibleName != "" {
		var err error
		tx, err = applyOrganizationBillingRecordResponsibleNameFilter(tx, req.ResponsibleName)
		if err != nil {
			return nil, err
		}
	}
	if req.ModelName != "" {
		modelNamePattern, err := model.SanitizeLikePattern(req.ModelName)
		if err != nil {
			return nil, err
		}
		tx = tx.Where("model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if req.Group != "" {
		tx = tx.Where("group_name = ?", req.Group)
	}
	if req.RequestId != "" {
		tx = tx.Where("request_id = ?", req.RequestId)
	}
	return tx, nil
}

func applyOrganizationBillingRecordResponsibleNameFilter(tx *gorm.DB, responsibleName string) (*gorm.DB, error) {
	var responsibleUserIds []int
	if err := model.DB.Model(&model.User{}).Where("username = ? OR display_name = ?", responsibleName, responsibleName).Pluck("id", &responsibleUserIds).Error; err != nil {
		return nil, err
	}
	if len(responsibleUserIds) == 0 {
		return tx.Where("1 = 0"), nil
	}
	return tx.Where("responsible_user_id IN ?", responsibleUserIds), nil
}

func hydrateOrganizationBillingRecordResponsibleUsers(records []*model.OrganizationBillingRecord) error {
	if len(records) == 0 {
		return nil
	}
	responsibleUserIds := make([]int, 0, len(records))
	seen := map[int]bool{}
	for _, record := range records {
		if record == nil || record.ResponsibleUserId <= 0 || seen[record.ResponsibleUserId] {
			continue
		}
		seen[record.ResponsibleUserId] = true
		responsibleUserIds = append(responsibleUserIds, record.ResponsibleUserId)
	}
	if len(responsibleUserIds) == 0 {
		return nil
	}
	var users []model.User
	if err := model.DB.Select("id", "username", "display_name").Where("id IN ?", responsibleUserIds).Find(&users).Error; err != nil {
		return err
	}
	userById := make(map[int]model.User, len(users))
	for _, user := range users {
		userById[user.Id] = user
	}
	for _, record := range records {
		user, ok := userById[record.ResponsibleUserId]
		if !ok {
			continue
		}
		record.ResponsibleUsername = user.Username
		record.ResponsibleDisplayName = user.DisplayName
	}
	return nil
}

var organizationBillingMonthPattern = regexp.MustCompile(`^\d{4}-\d{2}$`)

func parseOrganizationBillingMonth(month string) (int64, int64, error) {
	if !organizationBillingMonthPattern.MatchString(month) {
		return 0, 0, errors.New("invalid billing month")
	}
	start, err := time.ParseInLocation("2006-01", month, time.Local)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid billing month: %w", err)
	}
	return start.Unix(), start.AddDate(0, 1, 0).Unix(), nil
}
