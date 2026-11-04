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
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

// OrganizationUsageQuery is the organization's counterpart to SelfUsageQuery.
// The operator and organization always come from the session and the route;
// the rest are the table's filters.
type OrganizationUsageQuery struct {
	OperatorUserId    int
	OrganizationId    int
	AccessMode        string
	StartTs           int64
	EndTs             int64
	ModelName         string
	TokenId           int
	TokenName         string
	ResponsibleUserId int
	Page              int
	PageSize          int
}

// GetOrganizationUsage assembles the organization's usage from text/fee logs,
// tasks, and Midjourney rows, grouped by local day and responsible member.
func GetOrganizationUsage(query OrganizationUsageQuery) (dto.OrganizationUsageData, error) {
	actor, err := organizationUsageActor(query)
	if err != nil {
		return dto.OrganizationUsageData{}, err
	}
	merged, err := collectOrganizationUsageRows(query, actor, model.SelfUsageDaySeconds)
	if err != nil {
		return dto.OrganizationUsageData{}, err
	}
	filterOptions, err := collectOrganizationUsageFilterOptions(actor, merged)
	if err != nil {
		return dto.OrganizationUsageData{}, err
	}

	filtered := make([]usageRow, 0, len(merged))
	for _, row := range merged {
		if organizationUsageMatchesFilters(row, query) {
			filtered = append(filtered, row)
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
		items = append(items, toSelfUsageDTO(row))
	}

	return dto.OrganizationUsageData{
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
		FilterOptions: filterOptions,
	}, nil
}

// ExportOrganizationUsage returns every filtered row at the requested bucket
// size, using the same row assembly and filters as the table.
func ExportOrganizationUsage(query OrganizationUsageQuery, granularity string) (dto.SelfUsageExportData, error) {
	actor, err := organizationUsageActor(query)
	if err != nil {
		return dto.SelfUsageExportData{}, err
	}
	normalized, bucketSeconds := normalizeSelfUsageGranularity(granularity)
	merged, err := collectOrganizationUsageRows(query, actor, bucketSeconds)
	if err != nil {
		return dto.SelfUsageExportData{}, err
	}
	filtered := make([]usageRow, 0, len(merged))
	for _, row := range merged {
		if organizationUsageMatchesFilters(row, query) {
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

func organizationUsageActor(query OrganizationUsageQuery) (*OrganizationActorContext, error) {
	if query.OperatorUserId <= 0 || query.OrganizationId <= 0 {
		return nil, errors.New("无效的组织身份信息")
	}
	if query.StartTs > query.EndTs {
		return nil, errors.New("时间范围无效")
	}
	actor, err := GetOrganizationActorContextForAccessMode(query.OperatorUserId, query.OrganizationId, query.AccessMode, true)
	if err != nil {
		return nil, err
	}
	if !actor.Capabilities.CanViewOrganizationUsage {
		return nil, errors.New("permission denied")
	}
	return actor, nil
}

// collectOrganizationUsageRows loads and merges the organization's rows. A
// caller without organization-wide visibility is pinned to their own rows, the
// same rule the log endpoints use.
func collectOrganizationUsageRows(query OrganizationUsageQuery, actor *OrganizationActorContext, bucketSeconds int64) ([]usageRow, error) {
	responsibleUserId := query.ResponsibleUserId
	if !actor.Capabilities.CanViewOrganizationWideData {
		responsibleUserId = query.OperatorUserId
	}
	logRows, err := model.GetOrganizationUsageLogRowsByBucket(query.OrganizationId, query.StartTs, query.EndTs, bucketSeconds, responsibleUserId)
	if err != nil {
		return nil, err
	}
	taskRows, err := model.GetOrganizationUsageTaskRowsByBucket(query.OrganizationId, query.StartTs, query.EndTs, bucketSeconds, responsibleUserId)
	if err != nil {
		return nil, err
	}
	mjRows, err := model.GetOrganizationUsageMidjourneyRowsByBucket(query.OrganizationId, query.StartTs, query.EndTs, bucketSeconds, responsibleUserId)
	if err != nil {
		return nil, err
	}
	resolver, err := LoadModelTypeResolver()
	if err != nil {
		return nil, err
	}
	rows := mergeSelfUsageRows(logRows, taskRows, mjRows, resolver)
	hydrateOrganizationUsageResponsibleNames(rows)
	return rows, nil
}

func organizationUsageMatchesFilters(row usageRow, query OrganizationUsageQuery) bool {
	// The table's model filter is a dropdown of exact names, so a prefix must
	// not sweep in every sibling model (gpt-4o also matching gpt-4o-mini).
	if query.ModelName != "" && row.ModelName != query.ModelName {
		return false
	}
	if query.TokenId > 0 {
		if row.TokenID != query.TokenId {
			return false
		}
	} else if query.TokenName != "" && !strings.Contains(strings.ToLower(row.TokenName), strings.ToLower(query.TokenName)) {
		return false
	}
	if query.ResponsibleUserId > 0 && row.ResponsibleUserID != query.ResponsibleUserId {
		return false
	}
	return true
}

// hydrateOrganizationUsageResponsibleNames fills names missing from the rows'
// own snapshots, so members renamed since the call still read as a person
// rather than an id.
func hydrateOrganizationUsageResponsibleNames(rows []usageRow) {
	userIds := make([]int, 0)
	seen := make(map[int]bool)
	for _, row := range rows {
		if row.ResponsibleUserID <= 0 || row.ResponsibleName != "" || seen[row.ResponsibleUserID] {
			continue
		}
		seen[row.ResponsibleUserID] = true
		userIds = append(userIds, row.ResponsibleUserID)
	}
	if len(userIds) == 0 {
		return
	}
	var users []model.User
	if err := model.DB.Select("id", "username", "display_name").Where("id IN ?", userIds).Find(&users).Error; err != nil {
		return
	}
	names := make(map[int]string, len(users))
	for _, user := range users {
		name := user.DisplayName
		if name == "" {
			name = user.Username
		}
		names[user.Id] = name
	}
	for index := range rows {
		if rows[index].ResponsibleName == "" {
			rows[index].ResponsibleName = names[rows[index].ResponsibleUserID]
		}
	}
}

// collectOrganizationUsageFilterOptions feeds the table's dropdowns. It offers
// what the organization can use today plus what its rows actually mention, so
// a key or model that was deleted after the call was still billable stays
// filterable from the table it appears in.
func collectOrganizationUsageFilterOptions(actor *OrganizationActorContext, rows []usageRow) (dto.OrganizationUsageFilterOptions, error) {
	modelSet := make(map[string]struct{})
	for _, name := range GetGroupsEnabledModels([]string{NormalizeOrganizationGroup(actor.Organization.Group)}) {
		modelSet[name] = struct{}{}
	}
	for _, row := range rows {
		if row.ModelName != "" {
			modelSet[row.ModelName] = struct{}{}
		}
	}
	models := make([]string, 0, len(modelSet))
	for name := range modelSet {
		models = append(models, name)
	}
	sort.Strings(models)

	options := dto.OrganizationUsageFilterOptions{
		Models: models,
		Tokens: make([]dto.SelfUsageTokenOption, 0),
		Users:  make([]dto.OrganizationUsageUserOption, 0),
	}

	tokensById := make(map[int]dto.SelfUsageTokenOption)
	if actor.Capabilities.CanViewOrganizationTokens {
		var tokens []model.Token
		if err := model.DB.Model(&model.Token{}).
			Select("id, name").
			Where("scope_type = ? AND organization_id = ?", model.TokenScopeOrganization, actor.OrganizationId).
			Order("name, id").
			Find(&tokens).Error; err != nil {
			return dto.OrganizationUsageFilterOptions{}, err
		}
		for _, token := range tokens {
			if token.Name == "" {
				continue
			}
			tokensById[token.Id] = dto.SelfUsageTokenOption{
				Id:               token.Id,
				Name:             token.Name,
				OrganizationId:   actor.OrganizationId,
				OrganizationName: actor.Organization.Name,
			}
		}
	}
	for _, row := range rows {
		if row.TokenID <= 0 || row.TokenName == "" {
			continue
		}
		if _, ok := tokensById[row.TokenID]; !ok {
			tokensById[row.TokenID] = dto.SelfUsageTokenOption{
				Id:               row.TokenID,
				Name:             row.TokenName,
				OrganizationId:   actor.OrganizationId,
				OrganizationName: actor.Organization.Name,
			}
		}
	}
	for _, token := range tokensById {
		options.Tokens = append(options.Tokens, token)
	}
	sort.Slice(options.Tokens, func(i, j int) bool {
		if options.Tokens[i].Name != options.Tokens[j].Name {
			return options.Tokens[i].Name < options.Tokens[j].Name
		}
		return options.Tokens[i].Id < options.Tokens[j].Id
	})

	usersById := make(map[int]string)
	if actor.Capabilities.CanViewOrganizationWideData {
		var members []struct {
			Id          int
			Username    string
			DisplayName string
			Email       string
		}
		if err := model.DB.Table("organization_members").
			Select("users.id AS id, users.username AS username, users.display_name AS display_name, users.email AS email").
			Joins("JOIN users ON users.id = organization_members.user_id").
			Where("organization_members.organization_id = ? AND organization_members.status NOT IN ?",
				actor.OrganizationId, []string{model.OrganizationMemberStatusRemoved, model.OrganizationMemberStatusExited}).
			Group("users.id, users.username, users.display_name, users.email").
			Scan(&members).Error; err != nil {
			return dto.OrganizationUsageFilterOptions{}, err
		}
		for _, member := range members {
			name := member.DisplayName
			if name == "" {
				name = member.Username
			}
			if name == "" {
				name = member.Email
			}
			if name == "" {
				name = "#" + strconv.Itoa(member.Id)
			}
			usersById[member.Id] = name
		}
	}
	for _, row := range rows {
		if row.ResponsibleUserID <= 0 || row.ResponsibleName == "" {
			continue
		}
		if _, ok := usersById[row.ResponsibleUserID]; !ok {
			usersById[row.ResponsibleUserID] = row.ResponsibleName
		}
	}
	for id, name := range usersById {
		options.Users = append(options.Users, dto.OrganizationUsageUserOption{Id: id, Name: name})
	}
	sort.Slice(options.Users, func(i, j int) bool {
		left, right := strings.ToLower(options.Users[i].Name), strings.ToLower(options.Users[j].Name)
		if left != right {
			return left < right
		}
		return options.Users[i].Id < options.Users[j].Id
	})
	return options, nil
}
