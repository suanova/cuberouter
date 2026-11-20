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
		FilterOptions: dto.SiteUsageFilterOptions{},
	}, nil
}
