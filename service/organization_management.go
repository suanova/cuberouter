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
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const organizationAuditOperatorRolePlatformAdmin = "platform_admin"

type OrganizationManagementListRequest struct {
	Keyword string
	Status  string
	Group   string
}

type OrganizationManagementView struct {
	model.Organization
	OwnerUsername       string `json:"owner_username"`
	OwnerDisplayName    string `json:"owner_display_name"`
	OwnerEmail          string `json:"owner_email"`
	ActiveMemberCount   int64  `json:"active_member_count"`
	DisabledMemberCount int64  `json:"disabled_member_count"`
	TotalMemberCount    int64  `json:"total_member_count"`
	EnabledTokenCount   int64  `json:"enabled_token_count"`
	DisabledTokenCount  int64  `json:"disabled_token_count"`
	TotalTokenCount     int64  `json:"total_token_count"`
}

func ListOrganizationsForManagement(operatorUserId int, req OrganizationManagementListRequest, pageInfo *common.PageInfo) ([]OrganizationManagementView, int64, error) {
	if operatorUserId <= 0 || !model.IsAdmin(operatorUserId) {
		return nil, 0, errors.New("permission denied")
	}
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	if pageInfo.Page <= 0 {
		pageInfo.Page = 1
	}
	if pageInfo.PageSize <= 0 {
		pageInfo.PageSize = common.ItemsPerPage
	}
	base := model.DB.Model(&model.Organization{})
	base = applyOrganizationManagementFilters(base, req)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var organizations []model.Organization
	if err := base.Order("organizations.id desc").Limit(pageInfo.PageSize).Offset(pageInfo.GetStartIdx()).Find(&organizations).Error; err != nil {
		return nil, 0, err
	}
	views := make([]OrganizationManagementView, 0, len(organizations))
	for _, organization := range organizations {
		views = append(views, OrganizationManagementView{Organization: organization})
	}
	if err := hydrateOrganizationManagementViews(views); err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

func applyOrganizationManagementFilters(query *gorm.DB, req OrganizationManagementListRequest) *gorm.DB {
	statuses := parseOrganizationManagementStatuses(req.Status)
	query = query.Where("organizations.status IN ?", statuses)
	if group := strings.TrimSpace(req.Group); group != "" {
		query = query.Where(clause.Eq{Column: clause.Column{Table: "organizations", Name: "group"}, Value: group})
	}
	keyword := strings.TrimSpace(req.Keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Joins("LEFT JOIN users owners ON owners.id = organizations.owner_user_id")
		where := "organizations.name LIKE ? OR organizations.slug LIKE ? OR owners.username LIKE ? OR owners.display_name LIKE ? OR owners.email LIKE ?"
		args := []any{like, like, like, like, like}
		if id, err := strconv.Atoi(keyword); err == nil && id > 0 {
			where += " OR organizations.id = ? OR owners.id = ?"
			args = append(args, id, id)
		}
		query = query.Where(where, args...)
	}
	return query
}

func parseOrganizationManagementStatuses(status string) []string {
	status = strings.TrimSpace(status)
	if status == "" {
		return allOrganizationManagementStatuses()
	}
	parts := strings.Split(status, ",")
	statuses := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != model.OrganizationStatusActive && part != model.OrganizationStatusDisabled && part != model.OrganizationStatusDissolved {
			continue
		}
		if !seen[part] {
			seen[part] = true
			statuses = append(statuses, part)
		}
	}
	if len(statuses) == 0 {
		return allOrganizationManagementStatuses()
	}
	return statuses
}

func allOrganizationManagementStatuses() []string {
	return []string{model.OrganizationStatusActive, model.OrganizationStatusDisabled, model.OrganizationStatusDissolved}
}

func hydrateOrganizationManagementViews(views []OrganizationManagementView) error {
	if len(views) == 0 {
		return nil
	}
	organizationIds := make([]int, 0, len(views))
	ownerIds := make([]int, 0, len(views))
	ownerSeen := map[int]bool{}
	indexByOrganizationId := map[int]int{}
	for i, view := range views {
		organizationIds = append(organizationIds, view.Id)
		indexByOrganizationId[view.Id] = i
		if view.OwnerUserId > 0 && !ownerSeen[view.OwnerUserId] {
			ownerSeen[view.OwnerUserId] = true
			ownerIds = append(ownerIds, view.OwnerUserId)
		}
	}
	if len(ownerIds) > 0 {
		var owners []model.User
		if err := model.DB.Select("id", "username", "display_name", "email").Where("id IN ?", ownerIds).Find(&owners).Error; err != nil {
			return err
		}
		ownerById := map[int]model.User{}
		for _, owner := range owners {
			ownerById[owner.Id] = owner
		}
		for i := range views {
			owner, ok := ownerById[views[i].OwnerUserId]
			if !ok {
				continue
			}
			views[i].OwnerUsername = owner.Username
			views[i].OwnerDisplayName = owner.DisplayName
			views[i].OwnerEmail = owner.Email
		}
	}
	type countRow struct {
		OrganizationId int
		Status         string
		Count          int64
	}
	var memberCounts []countRow
	if err := model.DB.Model(&model.OrganizationMember{}).Select("organization_id, status, COUNT(*) as count").Where("organization_id IN ?", organizationIds).Group("organization_id, status").Scan(&memberCounts).Error; err != nil {
		return err
	}
	for _, row := range memberCounts {
		idx, ok := indexByOrganizationId[row.OrganizationId]
		if !ok {
			continue
		}
		views[idx].TotalMemberCount += row.Count
		if row.Status == model.OrganizationMemberStatusActive {
			views[idx].ActiveMemberCount += row.Count
		}
		if row.Status == model.OrganizationMemberStatusDisabled {
			views[idx].DisabledMemberCount += row.Count
		}
	}
	var tokenCounts []countRow
	if err := model.DB.Model(&model.Token{}).Select("organization_id, status, COUNT(*) as count").Where("scope_type = ? AND organization_id IN ?", model.TokenScopeOrganization, organizationIds).Group("organization_id, status").Scan(&tokenCounts).Error; err != nil {
		return err
	}
	for _, row := range tokenCounts {
		idx, ok := indexByOrganizationId[row.OrganizationId]
		if !ok {
			continue
		}
		views[idx].TotalTokenCount += row.Count
		if row.Status == strconv.Itoa(common.TokenStatusEnabled) {
			views[idx].EnabledTokenCount += row.Count
		}
		if row.Status == strconv.Itoa(common.TokenStatusDisabled) {
			views[idx].DisabledTokenCount += row.Count
		}
	}
	return nil
}

func validateOrganizationUpdateRequest(req UpdateOrganizationRequest) (string, string, *string, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return "", "", nil, errors.New("invalid organization request")
	}
	if utf8.RuneCountInString(name) > 64 {
		return "", "", nil, errors.New("organization name too long")
	}
	description := strings.TrimSpace(req.Description)
	var group *string
	if req.Group != nil {
		trimmedGroup := strings.TrimSpace(*req.Group)
		if trimmedGroup == "" {
			trimmedGroup = "default"
		}
		group = &trimmedGroup
	}
	return name, description, group, nil
}
