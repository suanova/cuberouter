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

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

var (
	ErrAdminTokenNotFound  = errors.New("admin token target not found")
	ErrAdminTokenForbidden = errors.New("admin token target forbidden")
)

// AdminTokenOperator identifies the administrator performing a token action.
type AdminTokenOperator struct {
	UserId int
	Role   int
}

// AdminPersonalTokenListRequest is the admin API Keys page's all-users list
// request. It only covers personal keys; organization keys have their own
// management surface.
type AdminPersonalTokenListRequest struct {
	Offset   int
	Limit    int
	Keyword  string
	Token    string
	Username string
	Status   int
}

// ListAdminPersonalTokens returns the personal keys visible to an operator:
// root sees every user's personal keys; any other administrator sees personal
// keys owned by a lower-role user or by themselves.
func ListAdminPersonalTokens(op AdminTokenOperator, req AdminPersonalTokenListRequest) ([]*model.Token, int64, error) {
	if op.UserId <= 0 {
		return nil, 0, errors.New("invalid admin token operator")
	}
	if op.Role < common.RoleAdminUser {
		return nil, 0, ErrAdminTokenForbidden
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	query := model.AdminPersonalTokensQuery(op.UserId, op.Role)

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
	if username := strings.TrimSpace(req.Username); username != "" {
		if !strings.Contains(username, "%") {
			username = "%" + username + "%"
		}
		pattern, err := model.SanitizeLikePattern(username)
		if err != nil {
			return nil, 0, err
		}
		ownerIds := model.DB.Model(&model.User{}).Select("id").
			Where("username LIKE ? ESCAPE '!' OR display_name LIKE ? ESCAPE '!'", pattern, pattern)
		query = query.Where("user_id IN (?)", ownerIds)
	}
	if req.Status > 0 {
		query = applyOrganizationTokenStatusFilter(query, req.Status)
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
	if err := hydrateAdminPersonalTokenOwners(tokens); err != nil {
		return nil, 0, err
	}
	for _, token := range tokens {
		token.Key = token.GetMaskedKey()
		token.KeyPreview = ""
	}
	return tokens, total, nil
}

// GetAdminManagedPersonalToken loads a personal token together with its owner
// and enforces the admin role hierarchy. Organization keys are treated as
// unavailable on this admin surface.
func GetAdminManagedPersonalToken(op AdminTokenOperator, tokenId int) (*model.Token, *model.User, error) {
	if op.UserId <= 0 {
		return nil, nil, errors.New("invalid admin token operator")
	}
	if op.Role < common.RoleAdminUser {
		return nil, nil, ErrAdminTokenForbidden
	}
	token, err := model.GetTokenById(tokenId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrAdminTokenNotFound
		}
		return nil, nil, err
	}
	model.NormalizeTokenScope(token)
	if token.ScopeType != model.TokenScopePersonal {
		return nil, nil, ErrAdminTokenNotFound
	}
	owner, err := model.GetUserById(token.UserId, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrAdminTokenNotFound
		}
		return nil, nil, err
	}
	if op.Role == common.RoleRootUser || op.Role > owner.Role || op.UserId == owner.Id {
		return token, owner, nil
	}
	return nil, nil, ErrAdminTokenForbidden
}

// hydrateAdminPersonalTokenOwners fills the display-only owner names for one
// page of tokens in a single users query.
func hydrateAdminPersonalTokenOwners(tokens []*model.Token) error {
	if len(tokens) == 0 {
		return nil
	}
	ownerIds := make([]int, 0, len(tokens))
	seen := make(map[int]bool, len(tokens))
	for _, token := range tokens {
		if token == nil || token.UserId <= 0 || seen[token.UserId] {
			continue
		}
		seen[token.UserId] = true
		ownerIds = append(ownerIds, token.UserId)
	}
	if len(ownerIds) == 0 {
		return nil
	}
	var users []model.User
	if err := model.DB.Select("id", "username", "display_name").Where("id IN ?", ownerIds).Find(&users).Error; err != nil {
		return err
	}
	owners := make(map[int]model.User, len(users))
	for _, user := range users {
		owners[user.Id] = user
	}
	for _, token := range tokens {
		if token == nil {
			continue
		}
		if owner, ok := owners[token.UserId]; ok {
			token.OwnerUsername = owner.Username
			token.OwnerDisplayName = owner.DisplayName
		}
	}
	return nil
}
