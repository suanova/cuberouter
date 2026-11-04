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
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

// ErrSelfUsageOrganizationForbidden is returned when the requested overview
// organization is not one the caller may see.
var ErrSelfUsageOrganizationForbidden = errors.New("organization access denied")

const (
	SelfUsageGranularityDay  = "day"
	SelfUsageGranularityHour = "hour"
	// MaxSelfUsageExportRows bounds one spreadsheet; older exports used the
	// same order of magnitude and warn when the filter matched more rows.
	MaxSelfUsageExportRows = 10000
)

type SelfUsageQuery struct {
	ActorUserId            int
	StartTs                int64
	EndTs                  int64
	ModelName              string
	TokenId                int
	TokenName              string
	BillingAccountType     string
	OrganizationID         int
	OverviewOrganizationID int
	Page                   int
	PageSize               int
}

type usageRow struct {
	BucketKey          int64
	Type               ModelUsageType
	BillingAccountType string
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
	PromptTokens       int
	CompletionTokens   int
	CacheTokens        int
	Quota              int
}

type usageRowKey struct {
	BucketKey          int64
	Type               ModelUsageType
	BillingAccountType string
	OrganizationID     int
	OrganizationName   string
	ResponsibleUserID  int
	ResponsibleName    string
	TokenID            int
	TokenName          string
	ModelName          string
}

func normalizeSelfUsageBillingType(value string) string {
	if value == "organization" {
		return "organization"
	}
	return "personal"
}

func mergeSelfUsageRows(
	logRows []model.SelfUsageLogRow,
	taskRows []model.SelfUsageAsyncRow,
	mjRows []model.SelfUsageAsyncRow,
	resolver *ModelTypeResolver,
) []usageRow {
	byKey := make(map[usageRowKey]*usageRow)

	getOrCreate := func(key usageRowKey) *usageRow {
		row := byKey[key]
		if row == nil {
			row = &usageRow{
				BucketKey:          key.BucketKey,
				Type:               key.Type,
				BillingAccountType: key.BillingAccountType,
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
		return row
	}

	for _, item := range logRows {
		key := usageRowKey{
			BucketKey:          item.BucketKey,
			Type:               resolver.Resolve(item.ModelName),
			BillingAccountType: normalizeSelfUsageBillingType(item.BillingAccountType),
			OrganizationID:     item.OrganizationID,
			OrganizationName:   item.OrganizationName,
			ResponsibleUserID:  item.ResponsibleUserID,
			ResponsibleName:    item.ResponsibleName,
			TokenID:            item.TokenID,
			TokenName:          item.TokenName,
			ModelName:          item.ModelName,
		}
		row := getOrCreate(key)
		row.RequestCount += item.RequestCount
		row.SuccessCount += item.RequestCount
		row.PromptTokens += item.PromptTokens
		row.CompletionTokens += item.CompletionTokens
		row.CacheTokens += item.CacheTokens
		row.Quota += item.Quota
	}

	for _, item := range taskRows {
		key := usageRowKey{
			BucketKey:          item.BucketKey,
			Type:               resolver.Resolve(item.ModelName),
			BillingAccountType: normalizeSelfUsageBillingType(item.BillingAccountType),
			OrganizationID:     item.OrganizationID,
			OrganizationName:   item.OrganizationName,
			ResponsibleUserID:  item.ResponsibleUserID,
			ResponsibleName:    item.ResponsibleName,
			TokenID:            item.TokenID,
			TokenName:          item.TokenName,
			ModelName:          item.ModelName,
		}
		row := getOrCreate(key)
		row.RequestCount += item.RequestCount
		row.SuccessCount += item.SuccessCount
		row.FailureCount += item.FailureCount
		row.Quota += item.Quota
	}

	for _, item := range mjRows {
		key := usageRowKey{
			BucketKey:          item.BucketKey,
			Type:               resolver.Resolve(item.ModelName),
			BillingAccountType: normalizeSelfUsageBillingType(item.BillingAccountType),
			OrganizationID:     item.OrganizationID,
			OrganizationName:   item.OrganizationName,
			ResponsibleUserID:  item.ResponsibleUserID,
			ResponsibleName:    item.ResponsibleName,
			TokenID:            item.TokenID,
			TokenName:          item.TokenName,
			ModelName:          item.ModelName,
		}
		row := getOrCreate(key)
		row.RequestCount += item.RequestCount
		row.SuccessCount += item.SuccessCount
		row.FailureCount += item.FailureCount
		row.Quota += item.Quota
	}

	rows := make([]usageRow, 0, len(byKey))
	for _, row := range byKey {
		rows = append(rows, *row)
	}
	return rows
}

func buildSelfUsageOverview(rows []usageRow, personalQuota, organizationID int) dto.SelfUsageOverview {
	var overview dto.SelfUsageOverview
	overview.Personal.RemainingQuota = personalQuota
	if organizationID > 0 {
		overview.Organization = &dto.SelfUsageOrganizationStat{}
	}
	for _, row := range rows {
		if row.BillingAccountType == "organization" {
			if organizationID > 0 && row.OrganizationID == organizationID {
				overview.Organization.RequestCount += row.RequestCount
				overview.Organization.PromptTokens += row.PromptTokens
				overview.Organization.CompletionTokens += row.CompletionTokens
				overview.Organization.CacheTokens += row.CacheTokens
				overview.Organization.Quota += row.Quota
			}
			continue
		}
		overview.Personal.RequestCount += row.RequestCount
		overview.Personal.PromptTokens += row.PromptTokens
		overview.Personal.CompletionTokens += row.CompletionTokens
		overview.Personal.CacheTokens += row.CacheTokens
		overview.Personal.Quota += row.Quota
	}
	return overview
}

// organizationRemainingQuota implements spec §5.3: owner draws from the
// organization pool; a member with a configured limit gets the smaller of the
// member remaining and the organization remaining.
func organizationRemainingQuota(org *model.Organization, member *model.OrganizationMember, isOwner bool) int {
	if org == nil {
		return 0
	}
	organizationRemaining := org.Quota - org.UsedQuota
	if organizationRemaining < 0 {
		organizationRemaining = 0
	}
	if isOwner || member == nil || member.QuotaLimit <= 0 {
		return organizationRemaining
	}
	memberRemaining := member.QuotaLimit - member.UsedQuota
	if memberRemaining < 0 {
		memberRemaining = 0
	}
	if memberRemaining < organizationRemaining {
		return memberRemaining
	}
	return organizationRemaining
}

func selfUsageMatchesFilters(row usageRow, query SelfUsageQuery) bool {
	if query.ModelName != "" && !strings.Contains(strings.ToLower(row.ModelName), strings.ToLower(query.ModelName)) {
		return false
	}
	if query.TokenId > 0 {
		// 同名密钥可能分属个人与组织；有 id 时按身份过滤，避免互相命中。
		if row.TokenID != query.TokenId {
			return false
		}
	} else if query.TokenName != "" && !strings.Contains(strings.ToLower(row.TokenName), strings.ToLower(query.TokenName)) {
		return false
	}
	switch query.BillingAccountType {
	case "personal":
		if row.BillingAccountType == "organization" {
			return false
		}
	case "organization":
		if row.BillingAccountType != "organization" {
			return false
		}
	}
	if query.OrganizationID > 0 && row.OrganizationID != query.OrganizationID {
		return false
	}
	return true
}

func sumSelfUsageTotals(rows []usageRow) dto.SelfUsageTotals {
	var totals dto.SelfUsageTotals
	for _, row := range rows {
		totals.RequestCount += row.RequestCount
		totals.SuccessCount += row.SuccessCount
		totals.FailureCount += row.FailureCount
		totals.PromptTokens += row.PromptTokens
		totals.CompletionTokens += row.CompletionTokens
		totals.CacheTokens += row.CacheTokens
		totals.Quota += row.Quota
	}
	return totals
}

// buildSelfUsageChartPoints folds the filtered rows into the day/model buckets
// the usage charts draw. It runs on the same rows as the table totals, so the
// charts inherit the date range, filters, and actor/account scope instead of
// reading a second data source that could disagree.
func buildSelfUsageChartPoints(rows []usageRow) []dto.SelfUsageChartPoint {
	type chartKey struct {
		BucketKey int64
		ModelName string
	}
	type chartAgg struct {
		RequestCount int
		Quota        int
	}
	buckets := make(map[chartKey]chartAgg)
	for _, row := range rows {
		key := chartKey{BucketKey: row.BucketKey, ModelName: row.ModelName}
		agg := buckets[key]
		agg.RequestCount += row.RequestCount
		agg.Quota += row.Quota
		buckets[key] = agg
	}
	points := make([]dto.SelfUsageChartPoint, 0, len(buckets))
	for key, agg := range buckets {
		points = append(points, dto.SelfUsageChartPoint{
			Date:         model.BillingDayKeyToDate(key.BucketKey),
			ModelName:    key.ModelName,
			RequestCount: agg.RequestCount,
			Quota:        agg.Quota,
		})
	}
	sort.Slice(points, func(i, j int) bool {
		if points[i].Date != points[j].Date {
			return points[i].Date < points[j].Date
		}
		return points[i].ModelName < points[j].ModelName
	})
	return points
}

// collectSelfUsageFilterOptions feeds the model and API key dropdowns with
// what the caller can actually see and use: models enabled for their groups
// (narrowed by member model access), personal keys, and organization keys they
// are allowed to view. It is independent of the current table filters so the
// lists do not collapse to the current selection.
func collectSelfUsageFilterOptions(actorUserId int, user *model.User) (dto.SelfUsageFilterOptions, error) {
	organizations, err := ListUserOrganizations(actorUserId, false)
	if err != nil {
		return dto.SelfUsageFilterOptions{}, err
	}

	// 组织 key 使用组织自己的分组，可能有用户个人分组里没有的模型；只按
	// user.Group 取会让「通过组织 key 能用」的模型从下拉里消失。
	usableGroups := GetUserUsableGroups(user.Group)
	groupSet := make(map[string]struct{}, len(usableGroups)+len(organizations))
	for name := range usableGroups {
		groupSet[name] = struct{}{}
	}
	for _, organization := range organizations {
		if organization.Capabilities.CanViewOrganizationTokens && organization.Group != "" {
			groupSet[organization.Group] = struct{}{}
		}
	}
	groups := make([]string, 0, len(groupSet))
	for name := range groupSet {
		groups = append(groups, name)
	}
	models := GetGroupsEnabledModels(groups)
	models = model.FilterModelsByMemberAccess(model.GetUserModelAllowList(actorUserId), models)
	if models == nil {
		models = []string{}
	}
	sort.Strings(models)

	options := dto.SelfUsageFilterOptions{
		Models: models,
		Tokens: make([]dto.SelfUsageTokenOption, 0),
	}

	var personalTokens []model.Token
	if err := model.DB.
		Select("id, name").
		Where("(scope_type = ? OR scope_type = '') AND user_id = ?", model.TokenScopePersonal, actorUserId).
		Order("name, id").
		Find(&personalTokens).Error; err != nil {
		return dto.SelfUsageFilterOptions{}, err
	}
	for _, token := range personalTokens {
		if token.Name != "" {
			options.Tokens = append(options.Tokens, dto.SelfUsageTokenOption{Id: token.Id, Name: token.Name})
		}
	}

	for _, organization := range organizations {
		if !organization.Capabilities.CanViewOrganizationTokens {
			continue
		}
		query := model.DB.Model(&model.Token{}).
			Where("scope_type = ? AND organization_id = ?", model.TokenScopeOrganization, organization.Id)
		if !organization.Capabilities.CanViewOrganizationWideData {
			query = query.Where("(visibility <> ? AND (responsible_user_id = ? OR creator_user_id = ?)) OR visibility = ?",
				model.TokenVisibilityPublic, actorUserId, actorUserId, model.TokenVisibilityPublic)
		}
		var orgTokens []model.Token
		if err := query.Select("id, name").Order("name, id").Find(&orgTokens).Error; err != nil {
			return dto.SelfUsageFilterOptions{}, err
		}
		for _, token := range orgTokens {
			if token.Name != "" {
				options.Tokens = append(options.Tokens, dto.SelfUsageTokenOption{
					Id:               token.Id,
					Name:             token.Name,
					OrganizationId:   organization.Id,
					OrganizationName: organization.Name,
				})
			}
		}
	}

	sort.Slice(options.Tokens, func(i, j int) bool {
		if options.Tokens[i].Name != options.Tokens[j].Name {
			return options.Tokens[i].Name < options.Tokens[j].Name
		}
		if options.Tokens[i].OrganizationId != options.Tokens[j].OrganizationId {
			return options.Tokens[i].OrganizationId < options.Tokens[j].OrganizationId
		}
		return options.Tokens[i].Id < options.Tokens[j].Id
	})
	return options, nil
}

func sortSelfUsageRows(rows []usageRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := rows[i], rows[j]
		if left.BucketKey != right.BucketKey {
			return left.BucketKey > right.BucketKey
		}
		if left.BillingAccountType != right.BillingAccountType {
			return left.BillingAccountType < right.BillingAccountType
		}
		if left.Type != right.Type {
			return left.Type < right.Type
		}
		if left.OrganizationID != right.OrganizationID {
			return left.OrganizationID < right.OrganizationID
		}
		if left.OrganizationName != right.OrganizationName {
			return left.OrganizationName < right.OrganizationName
		}
		if left.ResponsibleName != right.ResponsibleName {
			return left.ResponsibleName < right.ResponsibleName
		}
		if left.ResponsibleUserID != right.ResponsibleUserID {
			return left.ResponsibleUserID < right.ResponsibleUserID
		}
		if left.ModelName != right.ModelName {
			return left.ModelName < right.ModelName
		}
		if left.TokenName != right.TokenName {
			return left.TokenName < right.TokenName
		}
		return left.TokenID < right.TokenID
	})
}

func normalizeSelfUsagePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func toSelfUsageDTO(row usageRow) dto.SelfUsageRow {
	return dto.SelfUsageRow{
		Date:               model.BillingDayKeyToDate(row.BucketKey),
		Type:               string(row.Type),
		BillingAccountType: row.BillingAccountType,
		OrganizationId:     row.OrganizationID,
		OrganizationName:   row.OrganizationName,
		ResponsibleUserId:  row.ResponsibleUserID,
		ResponsibleName:    row.ResponsibleName,
		TokenId:            row.TokenID,
		TokenName:          row.TokenName,
		ModelName:          row.ModelName,
		RequestCount:       row.RequestCount,
		SuccessCount:       row.SuccessCount,
		FailureCount:       row.FailureCount,
		PromptTokens:       row.PromptTokens,
		CompletionTokens:   row.CompletionTokens,
		CacheTokens:        row.CacheTokens,
		Quota:              row.Quota,
	}
}

func fillSelfUsageOrganizationOverview(overview *dto.SelfUsageOverview, actorUserId, organizationID int) error {
	var organization model.Organization
	if err := model.DB.Where("id = ?", organizationID).First(&organization).Error; err != nil {
		return ErrSelfUsageOrganizationForbidden
	}
	isOwner := organization.OwnerUserId == actorUserId
	var member model.OrganizationMember
	memberErr := model.DB.
		Where("organization_id = ? AND user_id = ? AND status = ?", organizationID, actorUserId, model.OrganizationMemberStatusActive).
		First(&member).Error
	if memberErr != nil && !isOwner {
		return ErrSelfUsageOrganizationForbidden
	}
	if overview.Organization == nil {
		overview.Organization = &dto.SelfUsageOrganizationStat{}
	}
	stat := overview.Organization
	stat.Id = organization.Id
	stat.Name = organization.Name
	stat.MemberQuotaLimit = member.QuotaLimit
	stat.MemberUsedQuota = member.UsedQuota
	stat.OrganizationQuota = organization.Quota
	stat.OrganizationUsedQuota = organization.UsedQuota
	stat.RemainingQuota = organizationRemainingQuota(&organization, &member, isOwner)
	return nil
}

// collectSelfUsageRows loads and merges the caller's usage at the requested
// bucket size (day for the table, day or hour for exports). Task/MJ billing
// logs are excluded by the model layer; their spend comes from the source
// tables instead.
func collectSelfUsageRows(query SelfUsageQuery, bucketSeconds int64) ([]usageRow, error) {
	logRows, err := model.GetSelfUsageLogRowsByBucket(query.ActorUserId, query.StartTs, query.EndTs, bucketSeconds)
	if err != nil {
		return nil, err
	}
	taskRows, err := model.GetSelfUsageTaskRowsByBucket(query.ActorUserId, query.StartTs, query.EndTs, bucketSeconds)
	if err != nil {
		return nil, err
	}
	mjRows, err := model.GetSelfUsageMidjourneyRowsByBucket(query.ActorUserId, query.StartTs, query.EndTs, bucketSeconds)
	if err != nil {
		return nil, err
	}
	resolver, err := LoadModelTypeResolver()
	if err != nil {
		return nil, err
	}
	return mergeSelfUsageRows(logRows, taskRows, mjRows, resolver), nil
}

// GetSelfUsage assembles the caller's usage from text/fee logs, tasks, and
// Midjourney rows. Task/MJ logs are excluded by the model layer.
func GetSelfUsage(query SelfUsageQuery) (dto.SelfUsageData, error) {
	if query.ActorUserId <= 0 {
		return dto.SelfUsageData{}, errors.New("用户 ID 无效")
	}
	if query.StartTs > query.EndTs {
		return dto.SelfUsageData{}, errors.New("时间范围无效")
	}

	merged, err := collectSelfUsageRows(query, model.SelfUsageDaySeconds)
	if err != nil {
		return dto.SelfUsageData{}, err
	}

	user, err := model.GetUserById(query.ActorUserId, false)
	if err != nil {
		return dto.SelfUsageData{}, err
	}
	filterOptions, err := collectSelfUsageFilterOptions(query.ActorUserId, user)
	if err != nil {
		return dto.SelfUsageData{}, err
	}
	overview := buildSelfUsageOverview(merged, user.Quota, query.OverviewOrganizationID)
	if query.OverviewOrganizationID > 0 {
		if err := fillSelfUsageOrganizationOverview(&overview, query.ActorUserId, query.OverviewOrganizationID); err != nil {
			return dto.SelfUsageData{}, err
		}
	}

	filtered := make([]usageRow, 0, len(merged))
	for _, row := range merged {
		if selfUsageMatchesFilters(row, query) {
			filtered = append(filtered, row)
		}
	}
	totals := sumSelfUsageTotals(filtered)
	sortSelfUsageRows(filtered)
	page, pageSize := normalizeSelfUsagePagination(query.Page, query.PageSize)
	// Guard the multiplication: a huge page number would overflow into a
	// negative start and panic the slice below.
	start := len(filtered)
	if page-1 <= len(filtered)/pageSize {
		start = (page - 1) * pageSize
	}
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	items := make([]dto.SelfUsageRow, 0, end-start)
	for _, row := range filtered[start:end] {
		items = append(items, toSelfUsageDTO(row))
	}

	return dto.SelfUsageData{
		BackfillComplete: model.LogUsageBackfillComplete(),
		Range: dto.SelfUsageRange{
			Start:          time.Unix(query.StartTs, 0).In(time.Local).Format("2006-01-02"),
			End:            time.Unix(query.EndTs, 0).In(time.Local).Format("2006-01-02"),
			StartTimestamp: query.StartTs,
			EndTimestamp:   query.EndTs,
		},
		Overview: overview,
		Rows: dto.SelfUsageRows{
			Page:     page,
			PageSize: pageSize,
			Total:    len(filtered),
			Items:    items,
		},
		Totals:        totals,
		Charts:        buildSelfUsageChartPoints(filtered),
		FilterOptions: filterOptions,
	}, nil
}

// ExportSelfUsage returns every filtered row at the requested bucket size. It
// reuses the table's row assembly and filters, but keeps all matching rows
// (up to MaxSelfUsageExportRows) so the client can build one sheet.
func ExportSelfUsage(query SelfUsageQuery, granularity string) (dto.SelfUsageExportData, error) {
	if query.ActorUserId <= 0 {
		return dto.SelfUsageExportData{}, errors.New("用户 ID 无效")
	}
	if query.StartTs > query.EndTs {
		return dto.SelfUsageExportData{}, errors.New("时间范围无效")
	}
	normalized, bucketSeconds := normalizeSelfUsageGranularity(granularity)

	merged, err := collectSelfUsageRows(query, bucketSeconds)
	if err != nil {
		return dto.SelfUsageExportData{}, err
	}
	filtered := make([]usageRow, 0, len(merged))
	for _, row := range merged {
		if selfUsageMatchesFilters(row, query) {
			filtered = append(filtered, row)
		}
	}
	sortSelfUsageRows(filtered)

	total := len(filtered)
	truncated := total > MaxSelfUsageExportRows
	if truncated {
		filtered = filtered[:MaxSelfUsageExportRows]
	}
	rows := make([]dto.SelfUsageRow, 0, len(filtered))
	for _, row := range filtered {
		item := toSelfUsageDTO(row)
		item.Date = formatSelfUsageBucket(row.BucketKey, normalized)
		rows = append(rows, item)
	}
	return dto.SelfUsageExportData{
		Granularity: normalized,
		Rows:        rows,
		Total:       total,
		Truncated:   truncated,
	}, nil
}

// normalizeSelfUsageGranularity maps the public granularity onto a bucket
// size. Unknown values (and the absent default) mean day, as the UI promises.
func normalizeSelfUsageGranularity(value string) (string, int64) {
	if value == SelfUsageGranularityHour {
		return SelfUsageGranularityHour, model.SelfUsageHourSeconds
	}
	return SelfUsageGranularityDay, model.SelfUsageDaySeconds
}

// formatSelfUsageBucket restores the local wall-clock label from a bucket key.
// The key is local-shifted at write time, so UTC formatting reproduces it.
func formatSelfUsageBucket(bucketKey int64, granularity string) string {
	if granularity == SelfUsageGranularityHour {
		return time.Unix(bucketKey*model.SelfUsageHourSeconds, 0).UTC().Format("2006-01-02 15:00")
	}
	return model.BillingDayKeyToDate(bucketKey)
}
