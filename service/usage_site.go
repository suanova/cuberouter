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
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

type SiteUsageQuery struct {
	StartTs            int64
	EndTs              int64
	ModelName          string
	TokenId            int
	TokenName          string
	BillingAccountType string
	OrganizationID     int
	UserId             int
	// ChartGranularity is "hour" for the single-day chart; the table stays daily.
	ChartGranularity string
	Page             int
	PageSize         int
}

// collectSiteUsageRows loads and merges every actor's usage at the requested
// bucket size. Task/MJ billing logs are excluded by the model layer; their
// spend comes from the source tables instead.
func collectSiteUsageRows(query SiteUsageQuery, bucketSeconds int64) ([]usageRow, error) {
	filter := model.SiteUsageFilter{
		ActorUserId:        query.UserId,
		OrganizationId:     query.OrganizationID,
		TokenId:            query.TokenId,
		BillingAccountType: query.BillingAccountType,
	}
	logRows, err := model.GetSiteUsageLogRowsByBucket(filter, query.StartTs, query.EndTs, bucketSeconds)
	if err != nil {
		return nil, err
	}
	taskRows, err := model.GetSiteUsageTaskRowsByBucket(filter, query.StartTs, query.EndTs, bucketSeconds)
	if err != nil {
		return nil, err
	}
	mjRows, err := model.GetSiteUsageMidjourneyRowsByBucket(filter, query.StartTs, query.EndTs, bucketSeconds)
	if err != nil {
		return nil, err
	}
	resolver, err := LoadModelTypeResolver()
	if err != nil {
		return nil, err
	}
	rows := mergeSelfUsageRows(logRows, taskRows, mjRows, resolver)
	hydrateSiteUsageUserNames(rows)
	return rows, nil
}

// hydrateSiteUsageUserNames fills the display name of the actor dimension from
// the users table. A failed lookup only logs; the row keeps its empty name.
func hydrateSiteUsageUserNames(rows []usageRow) {
	userIDs := make([]int, 0)
	seen := make(map[int]struct{})
	for i := range rows {
		if rows[i].ResponsibleUserID <= 0 || rows[i].ResponsibleName != "" {
			continue
		}
		if _, ok := seen[rows[i].ResponsibleUserID]; ok {
			continue
		}
		seen[rows[i].ResponsibleUserID] = struct{}{}
		userIDs = append(userIDs, rows[i].ResponsibleUserID)
	}
	if len(userIDs) == 0 {
		return
	}
	var users []struct {
		Id          int
		Username    string
		DisplayName string
		Email       string
	}
	if err := model.DB.Table("users").
		Select("id, username, display_name, email").
		Where("id IN ?", userIDs).
		Scan(&users).Error; err != nil {
		common.SysError("failed to load site usage user names: " + err.Error())
		return
	}
	names := make(map[int]string, len(users))
	for _, user := range users {
		name := user.DisplayName
		if name == "" {
			name = user.Username
		}
		if name == "" {
			name = user.Email
		}
		names[user.Id] = name
	}
	for i := range rows {
		if rows[i].ResponsibleName == "" {
			rows[i].ResponsibleName = names[rows[i].ResponsibleUserID]
		}
	}
}

func siteUsageMatchesFilters(row usageRow, query SiteUsageQuery) bool {
	if !selfUsageMatchesFilters(row, SelfUsageQuery{
		ModelName:          query.ModelName,
		TokenId:            query.TokenId,
		TokenName:          query.TokenName,
		BillingAccountType: query.BillingAccountType,
		OrganizationID:     query.OrganizationID,
	}) {
		return false
	}
	if query.UserId > 0 && row.ResponsibleUserID != query.UserId {
		return false
	}
	return true
}

// toSiteUsageDTO exposes the actor dimension through the site user fields and
// leaves the organization-member fields empty: the site view is not an
// organization account posture.
func toSiteUsageDTO(row usageRow) dto.SelfUsageRow {
	item := toSelfUsageDTO(row)
	item.UserId = row.ResponsibleUserID
	item.UserName = row.ResponsibleName
	item.ResponsibleUserId = 0
	item.ResponsibleName = ""
	return item
}

// collectSiteUsageFilterOptions feeds the site table's dropdowns from the
// range-wide rows (not the active selection) plus the enabled site catalog, so
// a current selection cannot collapse its own option list and retired models
// stay filterable.
func collectSiteUsageFilterOptions(rows []usageRow) dto.SiteUsageFilterOptions {
	tokens := make(map[int]dto.SelfUsageTokenOption)
	users := make(map[int]dto.SiteUsageUserOption)
	organizations := make(map[int]dto.SiteUsageOrganizationOption)
	for _, row := range rows {
		if row.TokenID > 0 && row.TokenName != "" {
			tokens[row.TokenID] = dto.SelfUsageTokenOption{
				Id:               row.TokenID,
				Name:             row.TokenName,
				OrganizationId:   row.OrganizationID,
				OrganizationName: row.OrganizationName,
			}
		}
		if row.ResponsibleUserID > 0 {
			users[row.ResponsibleUserID] = dto.SiteUsageUserOption{
				Id:   row.ResponsibleUserID,
				Name: row.ResponsibleName,
			}
		}
		if row.OrganizationID > 0 {
			organizations[row.OrganizationID] = dto.SiteUsageOrganizationOption{
				Id:   row.OrganizationID,
				Name: row.OrganizationName,
			}
		}
	}

	options := dto.SiteUsageFilterOptions{
		Models:        usageModelFilterOptions(model.GetEnabledModels(), rows),
		Tokens:        make([]dto.SelfUsageTokenOption, 0, len(tokens)),
		Users:         make([]dto.SiteUsageUserOption, 0, len(users)),
		Organizations: make([]dto.SiteUsageOrganizationOption, 0, len(organizations)),
	}
	for _, token := range tokens {
		options.Tokens = append(options.Tokens, token)
	}
	sort.Slice(options.Tokens, func(i, j int) bool {
		if options.Tokens[i].Name != options.Tokens[j].Name {
			return options.Tokens[i].Name < options.Tokens[j].Name
		}
		return options.Tokens[i].Id < options.Tokens[j].Id
	})
	for _, user := range users {
		options.Users = append(options.Users, user)
	}
	sort.Slice(options.Users, func(i, j int) bool {
		if options.Users[i].Name != options.Users[j].Name {
			return options.Users[i].Name < options.Users[j].Name
		}
		return options.Users[i].Id < options.Users[j].Id
	})
	for _, organization := range organizations {
		options.Organizations = append(options.Organizations, organization)
	}
	sort.Slice(options.Organizations, func(i, j int) bool {
		if options.Organizations[i].Name != options.Organizations[j].Name {
			return options.Organizations[i].Name < options.Organizations[j].Name
		}
		return options.Organizations[i].Id < options.Organizations[j].Id
	})
	return options
}

// ExportSiteUsage returns every filtered site row at the requested bucket
// size, up to MaxSelfUsageExportRows, so the client can build one sheet.
func ExportSiteUsage(query SiteUsageQuery, granularity string) (dto.SelfUsageExportData, error) {
	if query.StartTs > query.EndTs {
		return dto.SelfUsageExportData{}, errors.New("时间范围无效")
	}
	normalized, bucketSeconds := normalizeSelfUsageGranularity(granularity)

	merged, err := collectSiteUsageRows(query, bucketSeconds)
	if err != nil {
		return dto.SelfUsageExportData{}, err
	}
	filtered := make([]usageRow, 0, len(merged))
	for _, row := range merged {
		if siteUsageMatchesFilters(row, query) {
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
		item := toSiteUsageDTO(row)
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

// siteUsageHasNarrowingFilter reports whether the query hides rows in SQL, so
// the option list needs a second range-wide read to stay complete.
func siteUsageHasNarrowingFilter(query SiteUsageQuery) bool {
	return query.UserId > 0 ||
		query.OrganizationID > 0 ||
		query.TokenId > 0 ||
		query.BillingAccountType != ""
}

// GetSiteUsage assembles the site-wide usage table and chart from text/fee
// logs, tasks, and Midjourney rows. It mirrors GetSelfUsage apart from the
// scope and the absent account overview.
func GetSiteUsage(query SiteUsageQuery) (dto.SiteUsageData, error) {
	if query.StartTs > query.EndTs {
		return dto.SiteUsageData{}, errors.New("时间范围无效")
	}

	merged, err := collectSiteUsageRows(query, model.SelfUsageDaySeconds)
	if err != nil {
		return dto.SiteUsageData{}, err
	}
	optionRows := merged
	if siteUsageHasNarrowingFilter(query) {
		// The table rows are narrowed in SQL for speed; the dropdowns must still
		// describe the whole date range so an admin can switch directly from one
		// user/key/organization to another.
		optionQuery := query
		optionQuery.UserId = 0
		optionQuery.OrganizationID = 0
		optionQuery.TokenId = 0
		optionQuery.BillingAccountType = ""
		optionRows, err = collectSiteUsageRows(optionQuery, model.SelfUsageDaySeconds)
		if err != nil {
			return dto.SiteUsageData{}, err
		}
	}

	filtered := make([]usageRow, 0, len(merged))
	for _, row := range merged {
		if siteUsageMatchesFilters(row, query) {
			filtered = append(filtered, row)
		}
	}
	chartGranularity, chartBucketSeconds := normalizeSelfUsageGranularity(query.ChartGranularity)
	chartRows := filtered
	if chartGranularity == SelfUsageGranularityHour {
		if !selfUsageRangeIsSingleDay(query.StartTs, query.EndTs) {
			chartGranularity = SelfUsageGranularityDay
		} else {
			hourlyMerged, err := collectSiteUsageRows(query, chartBucketSeconds)
			if err != nil {
				return dto.SiteUsageData{}, err
			}
			chartRows = make([]usageRow, 0, len(hourlyMerged))
			for _, row := range hourlyMerged {
				if siteUsageMatchesFilters(row, query) {
					chartRows = append(chartRows, row)
				}
			}
		}
	}
	totals := sumSelfUsageTotals(filtered)
	sortSelfUsageRows(filtered)
	page, pageSize := normalizeSelfUsagePagination(query.Page, query.PageSize)
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
		items = append(items, toSiteUsageDTO(row))
	}

	return dto.SiteUsageData{
		BackfillComplete: model.LogUsageBackfillComplete(),
		Range: dto.SelfUsageRange{
			Start:          time.Unix(query.StartTs, 0).In(time.Local).Format("2006-01-02"),
			End:            time.Unix(query.EndTs, 0).In(time.Local).Format("2006-01-02"),
			StartTimestamp: query.StartTs,
			EndTimestamp:   query.EndTs,
		},
		Rows: dto.SelfUsageRows{
			Page:     page,
			PageSize: pageSize,
			Total:    len(filtered),
			Items:    items,
		},
		Totals:        totals,
		Charts:        buildSelfUsageChartPoints(chartRows, chartGranularity),
		FilterOptions: collectSiteUsageFilterOptions(optionRows),
	}, nil
}
