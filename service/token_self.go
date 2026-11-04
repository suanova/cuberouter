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
	"strings"

	"github.com/QuantumNous/new-api/model"
)

// SelfOrganizationTokenListRequest is the member-side organization key list:
// the keys they can use, not every key in the organization.
type SelfOrganizationTokenListRequest struct {
	Offset         int
	Limit          int
	Keyword        string
	Token          string
	Status         int
	Visibility     string
	OrganizationID int
}

// ListSelfOrganizationTokens lists the organization keys the user can use
// across their active memberships. Keys are returned with their full secret,
// as the organization key page does, because a key the caller may use is a key
// they may copy.
func ListSelfOrganizationTokens(userId int, req SelfOrganizationTokenListRequest) ([]*model.Token, int64, error) {
	if userId <= 0 {
		return nil, 0, errors.New("invalid user token request")
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	visibility := strings.TrimSpace(req.Visibility)
	if visibility != "" && visibility != model.TokenVisibilityPrivate && visibility != model.TokenVisibilityPublic {
		return nil, 0, errors.New("invalid token visibility")
	}

	query := model.SelfOrganizationTokensQuery(userId)
	if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
		// The page's search box is a contains search; the shared LIKE helper
		// treats a bare term as exact, so add the wildcards here.
		if !strings.Contains(keyword, "%") {
			keyword = "%" + keyword + "%"
		}
		pattern, err := model.SanitizeLikePattern(keyword)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where("name LIKE ? ESCAPE '!'", pattern)
	}
	if rawToken := strings.TrimSpace(req.Token); rawToken != "" {
		rawToken = strings.TrimPrefix(rawToken, "sk-")
		if !strings.Contains(rawToken, "%") {
			rawToken = "%" + rawToken + "%"
		}
		pattern, err := model.SanitizeLikePattern(rawToken)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where(model.TokenKeyColumn()+" LIKE ? ESCAPE '!'", pattern)
	}
	if req.Status > 0 {
		query = applyOrganizationTokenStatusFilter(query, req.Status)
	}
	if visibility != "" {
		query = query.Where("visibility = ?", visibility)
	}
	if req.OrganizationID > 0 {
		query = query.Where("organization_id = ?", req.OrganizationID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var tokens []*model.Token
	if err := query.Order("id desc").Limit(req.Limit).Offset(req.Offset).Find(&tokens).Error; err != nil {
		return nil, 0, err
	}
	for _, token := range tokens {
		model.NormalizeTokenScope(token)
	}
	if err := hydrateOrganizationTokenUserNames(tokens); err != nil {
		return nil, 0, err
	}
	if err := hydrateSelfOrganizationTokenNames(tokens); err != nil {
		return nil, 0, err
	}
	for _, token := range tokens {
		fillOrganizationTokenKeyPreview(token)
	}
	if err := hydrateOrganizationTokenAvailability(tokens); err != nil {
		return nil, 0, err
	}
	return tokens, total, nil
}

func hydrateSelfOrganizationTokenNames(tokens []*model.Token) error {
	organizationIds := make([]int, 0)
	seen := make(map[int]bool)
	for _, token := range tokens {
		if token == nil || token.OrganizationId <= 0 || seen[token.OrganizationId] {
			continue
		}
		seen[token.OrganizationId] = true
		organizationIds = append(organizationIds, token.OrganizationId)
	}
	if len(organizationIds) == 0 {
		return nil
	}
	var organizations []model.Organization
	if err := model.DB.Select("id", "name").Where("id IN ?", organizationIds).Find(&organizations).Error; err != nil {
		return err
	}
	names := make(map[int]string, len(organizations))
	for _, organization := range organizations {
		names[organization.Id] = organization.Name
	}
	for _, token := range tokens {
		if token != nil {
			token.OrganizationName = names[token.OrganizationId]
		}
	}
	return nil
}
