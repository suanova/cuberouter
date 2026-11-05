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

// SelfTokenListRequest is the personal API Keys page's list request: the
// caller's personal keys plus the organization keys they may use.
type SelfTokenListRequest struct {
	Offset         int
	Limit          int
	Keyword        string
	Token          string
	Status         int
	Visibility     string
	OrganizationID int
}

// ListSelfTokens is the personal API Keys page: personal keys plus the
// organization keys the caller may use, in one paginated list. The two sets
// share the tokens table, so the union happens in SQL rather than by merging
// two paginated responses in the browser.
func ListSelfTokens(userId int, req SelfTokenListRequest) ([]*model.Token, int64, error) {
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

	query := model.SelfTokensQuery(userId)
	if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
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
	if err := hydrateSelfOrganizationTokenNames(tokens); err != nil {
		return nil, 0, err
	}
	if err := hydrateOrganizationTokenUserNames(tokens); err != nil {
		return nil, 0, err
	}
	if err := hydrateOrganizationTokenAvailability(tokens); err != nil {
		return nil, 0, err
	}
	// Organization rows keep the full secret and its key_preview: the page masks
	// the display while the copy button uses the secret directly.
	for _, token := range tokens {
		if token.ScopeType == model.TokenScopePersonal {
			// The personal list keeps its existing masked-key behavior; the
			// full secret is fetched per row when the user copies it.
			token.Key = token.GetMaskedKey()
			token.KeyPreview = ""
		}
	}
	return tokens, total, nil
}

func hydrateSelfOrganizationTokenNames(tokens []*model.Token) error {
	if len(tokens) == 0 {
		return nil
	}
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
