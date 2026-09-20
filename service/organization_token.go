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

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OrganizationTokenRequest = model.Token

type OrganizationTokenListRequest struct {
	Offset            int
	Limit             int
	Keyword           string
	Status            int
	ResponsibleUserId int
	Visibility        string
	Group             string
}

type OrganizationTokenBatchCreateRequest struct {
	TokenCount     int
	IdempotencyKey string
	Token          OrganizationTokenRequest
}

type OrganizationTokenBatchCreateResult struct {
	Tokens          []*model.Token `json:"tokens"`
	TokenCount      int            `json:"token_count"`
	SecretAvailable bool           `json:"secret_available"`
}

type OrganizationTokenBatchDeleteRequest struct {
	Ids            []int
	IdempotencyKey string
}

type OrganizationTokenTransferRequest struct {
	ResponsibleUserId int
	Reason            string
}

type UpdateOrganizationTokenResponsibilityRequest struct {
	ResponsibleUserId int
	Reason            string
}

const (
	organizationTokenBatchCreateOperation = "organization_token_batch_create"
	organizationTokenBatchDeleteOperation = "organization_token_batch_delete"
)

var (
	errOrganizationTokenResponsibleUserInactive   = errors.New("responsible user must be active member")
	ErrOrganizationTokenResponsibleMemberDisabled = errors.New("organization token responsible member disabled")
	ErrOrganizationTokenResponsibleUserDisabled   = errors.New("organization token responsible user disabled")
	ErrOrganizationTokenEnableForbidden           = errors.New("organization token enable forbidden")
)

type organizationTokenBatchCreateIdempotencyResult struct {
	TokenIds   []int `json:"token_ids"`
	TokenCount int   `json:"token_count"`
}

type organizationTokenBatchDeleteIdempotencyResult struct {
	DeletedCount int `json:"deleted_count"`
}

func ListOrganizationTokens(operatorUserId, organizationId int, accessMode string, req OrganizationTokenListRequest) ([]*model.Token, int64, error) {
	if operatorUserId <= 0 || organizationId <= 0 {
		return nil, 0, errors.New("invalid organization token request")
	}
	actor, err := GetOrganizationActorContextForAccessMode(operatorUserId, organizationId, accessMode, true)
	if err != nil {
		return nil, 0, err
	}
	if actor.Organization.Status == model.OrganizationStatusDissolved && !actor.IsPlatformAdmin {
		return nil, 0, errors.New("organization dissolved")
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	query := model.DB.Model(&model.Token{}).Where("scope_type = ? AND organization_id = ?", model.TokenScopeOrganization, organizationId)
	if !actor.Capabilities.CanViewOrganizationWideData {
		query = query.Where("(responsible_user_id = ? AND visibility <> ?) OR visibility = ?", operatorUserId, model.TokenVisibilityPublic, model.TokenVisibilityPublic)
	} else if req.ResponsibleUserId > 0 {
		query = query.Where("responsible_user_id = ?", req.ResponsibleUserId)
	}
	query, err = applyOrganizationTokenListFilters(query, req, actor.Organization.Group)
	if err != nil {
		return nil, 0, err
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
	if err := hydrateOrganizationTokenResponsibleUsers(tokens); err != nil {
		return nil, 0, err
	}
	if err := hydrateOrganizationTokenAvailability(tokens); err != nil {
		return nil, 0, err
	}
	return tokens, total, nil
}

func applyOrganizationTokenListFilters(query *gorm.DB, req OrganizationTokenListRequest, organizationGroup string) (*gorm.DB, error) {
	keyword := strings.TrimSpace(req.Keyword)
	if keyword != "" {
		pattern, err := model.SanitizeLikePattern(keyword)
		if err != nil {
			return nil, err
		}
		query = query.Where("name LIKE ? ESCAPE '!'", pattern)
	}
	if req.Status > 0 {
		query = query.Where("status = ?", req.Status)
	}
	visibility := strings.TrimSpace(req.Visibility)
	if visibility != "" {
		if visibility != model.TokenVisibilityPrivate && visibility != model.TokenVisibilityPublic {
			return nil, errors.New("invalid token visibility")
		}
		query = query.Where("visibility = ?", visibility)
	}
	group := strings.TrimSpace(req.Group)
	if group != "" {
		groupClause := clause.Eq{Column: clause.Column{Name: "group"}, Value: group}
		if group == normalizeOrganizationGroup(organizationGroup) {
			query = query.Where(model.DB.Where(groupClause).Or(clause.Eq{Column: clause.Column{Name: "group"}, Value: ""}))
		} else {
			query = query.Where(groupClause)
		}
	}
	return query, nil
}

func hydrateOrganizationTokenResponsibleUsers(tokens []*model.Token) error {
	if len(tokens) == 0 {
		return nil
	}
	responsibleUserIds := make([]int, 0, len(tokens))
	seen := map[int]bool{}
	for _, token := range tokens {
		if token == nil || token.ResponsibleUserId <= 0 || seen[token.ResponsibleUserId] {
			continue
		}
		seen[token.ResponsibleUserId] = true
		responsibleUserIds = append(responsibleUserIds, token.ResponsibleUserId)
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
	for _, token := range tokens {
		user, ok := userById[token.ResponsibleUserId]
		if !ok {
			continue
		}
		token.ResponsibleUsername = user.Username
		token.ResponsibleDisplayName = user.DisplayName
	}
	return nil
}

func hydrateOrganizationTokenAvailability(tokens []*model.Token) error {
	if len(tokens) == 0 {
		return nil
	}
	tokenIds := make([]int, 0, len(tokens))
	for _, token := range tokens {
		if token != nil && token.Id > 0 {
			tokenIds = append(tokenIds, token.Id)
		}
	}
	if len(tokenIds) == 0 {
		return nil
	}
	var blockers []model.OrganizationTokenSystemBlocker
	if err := model.DB.Where("token_id IN ? AND status = ?", tokenIds, model.OrganizationTokenBlockerStatusActive).Order("id asc").Find(&blockers).Error; err != nil {
		return err
	}
	blockersByTokenId := make(map[int][]model.OrganizationTokenSystemBlocker, len(blockers))
	for _, blocker := range blockers {
		blockersByTokenId[blocker.TokenId] = append(blockersByTokenId[blocker.TokenId], blocker)
	}
	for _, token := range tokens {
		if token == nil {
			continue
		}
		applyOrganizationTokenAvailability(token, blockersByTokenId[token.Id])
		// 组织 Key 完整 secret 在列表和详情每次都返回，masked 预览仅用于展示。
		fillOrganizationTokenKeyPreview(token)
	}
	return nil
}

func applyOrganizationTokenAvailability(token *model.Token, blockers []model.OrganizationTokenSystemBlocker) {
	if token == nil {
		return
	}
	token.UnavailableReasons = nil
	token.DisabledBySystems = false
	token.SystemDisabledReason = ""
	token.SystemDisabledRefId = 0
	token.SystemDisabledAt = 0
	if len(blockers) == 0 {
		return
	}
	for _, blocker := range blockers {
		token.UnavailableReasons = append(token.UnavailableReasons, blocker.Reason)
	}
	blocker := selectedOrganizationTokenSystemBlocker(blockers)
	if organizationTokenBlockerReasonIsSystem(blocker.Reason) {
		token.DisabledBySystems = true
		token.SystemDisabledReason = blocker.Reason
		token.SystemDisabledRefId = blocker.RefId
		token.SystemDisabledAt = blocker.DisabledAt
	}
}

func organizationTokenBatchCreateRequestHash(operatorUserId, organizationId int, req OrganizationTokenBatchCreateRequest) (string, error) {
	data, err := common.Marshal(map[string]any{
		"operation_type":       organizationTokenBatchCreateOperation,
		"operator_user_id":     operatorUserId,
		"organization_id":      organizationId,
		"token_count":          req.TokenCount,
		"name":                 strings.TrimSpace(req.Token.Name),
		"expired_time":         req.Token.ExpiredTime,
		"remain_quota":         req.Token.RemainQuota,
		"unlimited_quota":      req.Token.UnlimitedQuota,
		"model_limits_enabled": req.Token.ModelLimitsEnabled,
		"model_limits":         req.Token.ModelLimits,
		"allow_ips":            organizationTokenAllowIpsValue(req.Token.AllowIps),
		"group":                req.Token.Group,
		"cross_group_retry":    req.Token.CrossGroupRetry,
		"visibility":           normalizeOrganizationTokenVisibility(req.Token.Visibility),
		"responsible_user_id":  req.Token.ResponsibleUserId,
	})
	if err != nil {
		return "", err
	}
	return common.GenerateHMAC(string(data)), nil
}

func organizationTokenBatchDeleteRequestHash(operatorUserId, organizationId int, ids []int) (string, error) {
	data, err := common.Marshal(map[string]any{
		"operation_type":   organizationTokenBatchDeleteOperation,
		"operator_user_id": operatorUserId,
		"organization_id":  organizationId,
		"ids":              ids,
	})
	if err != nil {
		return "", err
	}
	return common.GenerateHMAC(string(data)), nil
}

func organizationTokenAllowIpsValue(allowIps *string) string {
	if allowIps == nil {
		return ""
	}
	return *allowIps
}

func prepareOrganizationTokenBatchCreateIdempotencyWithTx(tx *gorm.DB, operatorUserId, organizationId int, idempotencyKey string, requestHash string) (*OrganizationTokenBatchCreateResult, *model.OrganizationIdempotencyRecord, error) {
	state, err := prepareOrganizationIdempotencyWithTx(tx, operatorUserId, organizationId, organizationTokenBatchCreateOperation, idempotencyKey, requestHash)
	if err != nil {
		return nil, nil, err
	}
	if state.Replayed {
		var result organizationTokenBatchCreateIdempotencyResult
		if err := common.Unmarshal([]byte(state.Record.ResultJson), &result); err != nil || result.TokenCount <= 0 {
			return nil, nil, errors.New("organization idempotency conflict")
		}
		tokens, err := loadOrganizationTokenIdempotencySummaryWithTx(tx, organizationId, result.TokenIds)
		if err != nil {
			return nil, nil, err
		}
		return &OrganizationTokenBatchCreateResult{Tokens: tokens, TokenCount: result.TokenCount, SecretAvailable: true}, nil, nil
	}
	return nil, state.Record, nil
}

func completeOrganizationTokenBatchCreateIdempotencyWithTx(tx *gorm.DB, record *model.OrganizationIdempotencyRecord, tokenIds []int, tokenCount int) error {
	if record == nil {
		return nil
	}
	return completeOrganizationIdempotencyWithTx(tx, &organizationIdempotencyState{Record: record}, organizationTokenBatchCreateIdempotencyResult{TokenIds: tokenIds, TokenCount: tokenCount})
}

func loadOrganizationTokenIdempotencySummaryWithTx(tx *gorm.DB, organizationId int, tokenIds []int) ([]*model.Token, error) {
	tokens := make([]*model.Token, 0, len(tokenIds))
	if len(tokenIds) == 0 {
		return tokens, nil
	}
	var stored []model.Token
	if err := tx.Unscoped().Where("scope_type = ? AND organization_id = ? AND id IN ?", model.TokenScopeOrganization, organizationId, tokenIds).Find(&stored).Error; err != nil {
		return nil, err
	}
	byId := make(map[int]*model.Token, len(stored))
	for i := range stored {
		model.NormalizeTokenScope(&stored[i])
		// 组织 Key 完整 secret 在列表和详情每次都返回，仅额外补全 masked 预览用于展示。
		fillOrganizationTokenKeyPreview(&stored[i])
		byId[stored[i].Id] = &stored[i]
	}
	for _, tokenId := range tokenIds {
		token, ok := byId[tokenId]
		if !ok {
			token = &model.Token{Id: tokenId, ScopeType: model.TokenScopeOrganization, ScopeId: organizationId, OrganizationId: organizationId, Visibility: model.TokenVisibilityPrivate}
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}

func ensureOrganizationTokenReplayViewAllowed(actor *OrganizationActorContext, tokens []*model.Token) error {
	for _, token := range tokens {
		if !organizationPolicyCanViewTokenResource(actor, token) {
			return errors.New("permission denied")
		}
	}
	return nil
}

func prepareOrganizationTokenBatchDeleteIdempotencyWithTx(tx *gorm.DB, operatorUserId, organizationId int, idempotencyKey string, requestHash string) (int, *model.OrganizationIdempotencyRecord, error) {
	state, err := prepareOrganizationIdempotencyWithTx(tx, operatorUserId, organizationId, organizationTokenBatchDeleteOperation, idempotencyKey, requestHash)
	if err != nil {
		return 0, nil, err
	}
	if state.Replayed {
		var result organizationTokenBatchDeleteIdempotencyResult
		if err := common.Unmarshal([]byte(state.Record.ResultJson), &result); err != nil || result.DeletedCount < 0 {
			return 0, nil, errors.New("organization idempotency conflict")
		}
		return result.DeletedCount, nil, nil
	}
	return 0, state.Record, nil
}

func completeOrganizationTokenBatchDeleteIdempotencyWithTx(tx *gorm.DB, record *model.OrganizationIdempotencyRecord, deletedCount int) error {
	if record == nil {
		return nil
	}
	return completeOrganizationIdempotencyWithTx(tx, &organizationIdempotencyState{Record: record}, organizationTokenBatchDeleteIdempotencyResult{DeletedCount: deletedCount})
}

func normalizeOrganizationTokenBatchDeleteIds(rawIds []int) ([]int, error) {
	seen := map[int]bool{}
	ids := make([]int, 0, len(rawIds))
	for _, id := range rawIds {
		if id <= 0 {
			return nil, errors.New("invalid token id")
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	return ids, nil
}

func GetOrganizationToken(operatorUserId, organizationId int, accessMode string, tokenId int) (*model.Token, error) {
	if operatorUserId <= 0 || organizationId <= 0 || tokenId <= 0 {
		return nil, errors.New("invalid organization token request")
	}
	actor, err := GetOrganizationActorContextForAccessMode(operatorUserId, organizationId, accessMode, true)
	if err != nil {
		return nil, err
	}
	if actor.Organization.Status == model.OrganizationStatusDissolved && !actor.IsPlatformAdmin {
		return nil, errors.New("organization dissolved")
	}
	token, err := getOrganizationTokenWithTx(model.DB, organizationId, tokenId)
	if err != nil {
		return nil, err
	}
	if err := hydrateOrganizationTokenAvailability([]*model.Token{token}); err != nil {
		return nil, err
	}
	if !organizationPolicyCanViewTokenResource(actor, token) {
		return nil, gorm.ErrRecordNotFound
	}
	return token, nil
}

func CreateOrganizationToken(operatorUserId, organizationId int, accessMode string, req OrganizationTokenRequest, auditMetadata ...OrganizationAuditRequestMetadata) (*model.Token, error) {
	if operatorUserId <= 0 || organizationId <= 0 {
		return nil, errors.New("invalid organization token request")
	}
	if err := validateOrganizationTokenRequest(req, false); err != nil {
		return nil, err
	}
	key, err := common.GenerateKey()
	if err != nil {
		return nil, errors.New("failed to generate token")
	}
	now := common.GetTimestamp()
	var created model.Token
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockOrganizationTokenWriteWithTx(tx, organizationId); err != nil {
			return err
		}
		actor, err := getOrganizationActorContextWithTxForAccessMode(tx, operatorUserId, organizationId, accessMode, true)
		if err != nil {
			return err
		}
		organization := actor.Organization
		if actor.ReadOnly {
			return organizationActorWriteDeniedError(actor)
		}
		if organizationActorHasPlatformAuthority(actor) {
			return errors.New("permission denied")
		}
		if !actor.Capabilities.CanManageAllTokens && req.ResponsibleUserId != 0 && req.ResponsibleUserId != operatorUserId {
			return errors.New("permission denied")
		}
		if err := ensureOrganizationTokenVisibilityAllowed(actor.Capabilities.CanManageAllTokens, req.Visibility); err != nil {
			return err
		}
		if organization.Status == model.OrganizationStatusDissolved {
			return errors.New("organization dissolved")
		}
		if err := validateOrganizationTokenGroup(organization.Group, req.Group); err != nil {
			return err
		}
		responsibleUserId := req.ResponsibleUserId
		if responsibleUserId == 0 {
			if organizationActorHasPlatformAuthority(actor) {
				return errors.New("responsible user is required")
			}
			responsibleUserId = operatorUserId
		}
		if err := ensureOrganizationTokenResponsibleUserWithTx(tx, organization, normalizeOrganizationTokenVisibility(req.Visibility), responsibleUserId); err != nil {
			return err
		}
		count, err := countOrganizationTokensWithTx(tx, organizationId)
		if err != nil {
			return err
		}
		maxTokens := operation_setting.GetMaxUserTokens()
		if int(count) >= maxTokens {
			return errors.New("organization token limit reached")
		}
		created = model.Token{
			UserId:             responsibleUserId,
			Name:               strings.TrimSpace(req.Name),
			Key:                key,
			Status:             common.TokenStatusEnabled,
			CreatedTime:        now,
			AccessedTime:       now,
			ExpiredTime:        req.ExpiredTime,
			RemainQuota:        req.RemainQuota,
			UnlimitedQuota:     req.UnlimitedQuota,
			ModelLimitsEnabled: req.ModelLimitsEnabled,
			ModelLimits:        req.ModelLimits,
			AllowIps:           req.AllowIps,
			Group:              req.Group,
			CrossGroupRetry:    req.CrossGroupRetry,
			ScopeType:          model.TokenScopeOrganization,
			ScopeId:            organizationId,
			Visibility:         normalizeOrganizationTokenVisibility(req.Visibility),
			OrganizationId:     organizationId,
			CreatorUserId:      operatorUserId,
			ResponsibleUserId:  responsibleUserId,
			UpdatedAt:          now,
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		return recordOrganizationAudit(tx, organization, operatorUserId, actor.OperatorRoleForAudit, organizationAuditActionTokenCreate, "token", created.Id, nil, created, "", auditMetadata...)
	})
	if err != nil {
		return nil, err
	}
	model.NormalizeTokenScope(&created)
	return &created, nil
}

func BatchCreateOrganizationTokens(operatorUserId, organizationId int, accessMode string, req OrganizationTokenBatchCreateRequest, auditMetadata ...OrganizationAuditRequestMetadata) (*OrganizationTokenBatchCreateResult, error) {
	if operatorUserId <= 0 || organizationId <= 0 {
		return nil, errors.New("invalid organization token request")
	}
	var err error
	req.IdempotencyKey, err = requireOrganizationIdempotencyKey(req.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	if req.TokenCount <= 0 {
		req.TokenCount = 1
	}
	if req.TokenCount > 100 {
		return nil, errors.New("token count exceeds maximum")
	}
	if err := validateOrganizationTokenRequest(req.Token, false); err != nil {
		return nil, err
	}
	requestHash, err := organizationTokenBatchCreateRequestHash(operatorUserId, organizationId, req)
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	created := make([]*model.Token, 0, req.TokenCount)
	var replayResult *OrganizationTokenBatchCreateResult
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockOrganizationTokenWriteWithTx(tx, organizationId); err != nil {
			return err
		}
		actor, err := getOrganizationActorContextWithTxForAccessMode(tx, operatorUserId, organizationId, accessMode, true)
		if err != nil {
			return err
		}
		organization := actor.Organization
		if actor.ReadOnly {
			return organizationActorWriteDeniedError(actor)
		}
		if organizationActorHasPlatformAuthority(actor) {
			return errors.New("permission denied")
		}
		if !actor.Capabilities.CanManageAllTokens && req.Token.ResponsibleUserId != 0 && req.Token.ResponsibleUserId != operatorUserId {
			return errors.New("permission denied")
		}
		if err := ensureOrganizationTokenVisibilityAllowed(actor.Capabilities.CanManageAllTokens, req.Token.Visibility); err != nil {
			return err
		}
		if organization.Status == model.OrganizationStatusDissolved {
			return errors.New("organization dissolved")
		}
		if err := validateOrganizationTokenGroup(organization.Group, req.Token.Group); err != nil {
			return err
		}
		var idempotencyRecord *model.OrganizationIdempotencyRecord
		if req.IdempotencyKey != "" {
			result, record, err := prepareOrganizationTokenBatchCreateIdempotencyWithTx(tx, operatorUserId, organizationId, req.IdempotencyKey, requestHash)
			if err != nil {
				return err
			}
			if result != nil {
				if err := ensureOrganizationTokenReplayViewAllowed(actor, result.Tokens); err != nil {
					return err
				}
				replayResult = result
				return nil
			}
			idempotencyRecord = record
		}
		responsibleUserId := req.Token.ResponsibleUserId
		if responsibleUserId == 0 {
			if organizationActorHasPlatformAuthority(actor) {
				return errors.New("responsible user is required")
			}
			responsibleUserId = operatorUserId
		}
		if err := ensureOrganizationTokenResponsibleUserWithTx(tx, organization, normalizeOrganizationTokenVisibility(req.Token.Visibility), responsibleUserId); err != nil {
			return err
		}
		count, err := countOrganizationTokensWithTx(tx, organizationId)
		if err != nil {
			return err
		}
		maxTokens := operation_setting.GetMaxUserTokens()
		if int(count)+req.TokenCount > maxTokens {
			return errors.New("organization token limit reached")
		}
		baseName := strings.TrimSpace(req.Token.Name)
		for i := 0; i < req.TokenCount; i++ {
			key, err := common.GenerateKey()
			if err != nil {
				return errors.New("failed to generate token")
			}
			name := baseName
			if req.TokenCount > 1 {
				name = baseName + "-" + common.GetRandomString(6)
			}
			token := model.Token{
				UserId:             responsibleUserId,
				Name:               name,
				Key:                key,
				Status:             common.TokenStatusEnabled,
				CreatedTime:        now,
				AccessedTime:       now,
				ExpiredTime:        req.Token.ExpiredTime,
				RemainQuota:        req.Token.RemainQuota,
				UnlimitedQuota:     req.Token.UnlimitedQuota,
				ModelLimitsEnabled: req.Token.ModelLimitsEnabled,
				ModelLimits:        req.Token.ModelLimits,
				AllowIps:           req.Token.AllowIps,
				Group:              req.Token.Group,
				CrossGroupRetry:    req.Token.CrossGroupRetry,
				ScopeType:          model.TokenScopeOrganization,
				ScopeId:            organizationId,
				Visibility:         normalizeOrganizationTokenVisibility(req.Token.Visibility),
				OrganizationId:     organizationId,
				CreatorUserId:      operatorUserId,
				ResponsibleUserId:  responsibleUserId,
				UpdatedAt:          now,
			}
			if err := tx.Create(&token).Error; err != nil {
				return err
			}
			if err := recordOrganizationAudit(tx, organization, operatorUserId, actor.OperatorRoleForAudit, organizationAuditActionTokenCreate, "token", token.Id, nil, token, "", auditMetadata...); err != nil {
				return err
			}
			model.NormalizeTokenScope(&token)
			created = append(created, &token)
		}
		tokenIds := make([]int, 0, len(created))
		for _, token := range created {
			tokenIds = append(tokenIds, token.Id)
		}
		return completeOrganizationTokenBatchCreateIdempotencyWithTx(tx, idempotencyRecord, tokenIds, req.TokenCount)
	})
	if err != nil {
		return nil, err
	}
	if replayResult != nil {
		return replayResult, nil
	}
	return &OrganizationTokenBatchCreateResult{Tokens: created, TokenCount: len(created), SecretAvailable: true}, nil
}

func UpdateOrganizationToken(operatorUserId, organizationId int, accessMode string, tokenId int, req OrganizationTokenRequest, auditMetadata ...OrganizationAuditRequestMetadata) (*model.Token, error) {
	if operatorUserId <= 0 || organizationId <= 0 || tokenId <= 0 {
		return nil, errors.New("invalid organization token request")
	}
	if err := validateOrganizationTokenRequest(req, true); err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	var updated model.Token
	var affectedKey string
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockOrganizationTokenWriteWithTx(tx, organizationId); err != nil {
			return err
		}
		actor, err := getOrganizationActorContextWithTxForAccessMode(tx, operatorUserId, organizationId, accessMode, true)
		if err != nil {
			return err
		}
		organization := actor.Organization
		if actor.ReadOnly {
			return organizationActorWriteDeniedError(actor)
		}
		if organization.Status == model.OrganizationStatusDissolved {
			return errors.New("organization dissolved")
		}
		if err := validateOrganizationTokenGroup(organization.Group, req.Group); err != nil {
			return err
		}
		if !actor.Capabilities.CanManageAllTokens {
			if err := ensureActiveOrganizationMemberLockedWithTx(tx, organizationId, operatorUserId); err != nil {
				return err
			}
		}
		token, err := getOrganizationTokenWithTx(tx, organizationId, tokenId)
		if err != nil {
			return err
		}
		affectedKey = token.Key
		if !actor.Capabilities.CanManageAllTokens {
			if token.ResponsibleUserId != operatorUserId || token.Visibility == model.TokenVisibilityPublic {
				return errors.New("permission denied")
			}
			if req.ResponsibleUserId != 0 && req.ResponsibleUserId != token.ResponsibleUserId {
				return errors.New("permission denied")
			}
		}
		if err := ensureOrganizationTokenVisibilityAllowed(actor.Capabilities.CanManageAllTokens, req.Visibility); err != nil {
			return err
		}
		before := *token
		if req.Status == common.TokenStatusEnabled {
			if err := clearOrganizationTokenManualBlockersForActorWithTx(tx, token.Id, organizationTokenBlockerClearanceForActor(actor), now); err != nil {
				return err
			}
			token.Status = common.TokenStatusEnabled
		}
		responsibleUserId := req.ResponsibleUserId
		if responsibleUserId == 0 {
			responsibleUserId = token.ResponsibleUserId
		}
		if responsibleUserId == 0 {
			responsibleUserId = token.UserId
		}
		visibility := strings.TrimSpace(req.Visibility)
		if visibility == "" {
			visibility = token.Visibility
		}
		visibility = normalizeOrganizationTokenVisibility(visibility)
		if actor.Capabilities.CanManageAllTokens {
			if err := ensureOrganizationTokenResponsibleUserWithTx(tx, organization, visibility, responsibleUserId); err != nil {
				return err
			}
		} else if responsibleUserId != token.ResponsibleUserId {
			return errors.New("permission denied")
		}
		if req.Status == common.TokenStatusDisabled {
			if err := ensureOrganizationTokenSystemBlockerWithTx(tx, token, organizationTokenBlockerReasonManualDisabled, organizationTokenBlockerRefTypeOperator, operatorUserId, operatorUserId, organizationTokenBlockerClearanceForActor(actor), now); err != nil {
				return err
			}
		}
		token.Name = strings.TrimSpace(req.Name)
		token.ExpiredTime = req.ExpiredTime
		token.RemainQuota = req.RemainQuota
		token.UnlimitedQuota = req.UnlimitedQuota
		token.ModelLimitsEnabled = req.ModelLimitsEnabled
		token.ModelLimits = req.ModelLimits
		token.AllowIps = req.AllowIps
		token.Group = req.Group
		token.CrossGroupRetry = req.CrossGroupRetry
		token.Visibility = visibility
		token.UserId = responsibleUserId
		token.ResponsibleUserId = responsibleUserId
		token.UpdatedAt = now
		updateFields := []string{"name", "expired_time", "remain_quota", "unlimited_quota", "model_limits_enabled", "model_limits", "allow_ips", "group", "cross_group_retry", "visibility", "user_id", "responsible_user_id", "updated_at"}
		if req.Status == common.TokenStatusEnabled {
			updateFields = append(updateFields, "status")
		}
		if err := tx.Model(token).Select(updateFields).Updates(token).Error; err != nil {
			return err
		}
		if err := reconcileOrganizationTokenSystemBlockerWithTx(tx, token, now); err != nil {
			return err
		}
		if err := tx.First(&updated, token.Id).Error; err != nil {
			return err
		}
		return recordOrganizationAudit(tx, organization, operatorUserId, actor.OperatorRoleForAudit, organizationAuditActionTokenUpdate, "token", token.Id, before, updated, "", auditMetadata...)
	})
	if err != nil {
		return nil, err
	}
	invalidateOrganizationTokenCaches(affectedKey)
	model.NormalizeTokenScope(&updated)
	if err := hydrateOrganizationTokenAvailability([]*model.Token{&updated}); err != nil {
		return nil, err
	}
	// 组织 Key 完整 secret 在列表和详情每次都返回，仅额外补全 masked 预览用于展示。
	fillOrganizationTokenKeyPreview(&updated)
	return &updated, nil
}

func DeleteOrganizationToken(operatorUserId, organizationId int, accessMode string, tokenId int, auditMetadata ...OrganizationAuditRequestMetadata) error {
	if operatorUserId <= 0 || organizationId <= 0 || tokenId <= 0 {
		return errors.New("invalid organization token request")
	}
	var affectedKey string
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockOrganizationTokenWriteWithTx(tx, organizationId); err != nil {
			return err
		}
		actor, err := getOrganizationActorContextWithTxForAccessMode(tx, operatorUserId, organizationId, accessMode, true)
		if err != nil {
			return err
		}
		organization := actor.Organization
		if actor.ReadOnly {
			return organizationActorWriteDeniedError(actor)
		}
		if organization.Status == model.OrganizationStatusDissolved {
			return errors.New("organization dissolved")
		}
		if !actor.Capabilities.CanManageAllTokens {
			if err := ensureActiveOrganizationMemberLockedWithTx(tx, organizationId, operatorUserId); err != nil {
				return err
			}
		}
		token, err := getOrganizationTokenWithTx(tx, organizationId, tokenId)
		if err != nil {
			return err
		}
		if !organizationPolicyCanManageTokenResource(actor, token) {
			return errors.New("permission denied")
		}
		affectedKey = token.Key
		token.UpdatedAt = common.GetTimestamp()
		if err := tx.Model(token).Select("updated_at").Updates(token).Error; err != nil {
			return err
		}
		before := *token
		if err := tx.Delete(token).Error; err != nil {
			return err
		}
		return recordOrganizationAudit(tx, organization, operatorUserId, actor.OperatorRoleForAudit, organizationAuditActionTokenDelete, "token", token.Id, before, nil, "", auditMetadata...)
	})
	if err != nil {
		return err
	}
	invalidateOrganizationTokenCaches(affectedKey)
	return nil
}

func BatchDeleteOrganizationTokens(operatorUserId, organizationId int, accessMode string, req OrganizationTokenBatchDeleteRequest, auditMetadata ...OrganizationAuditRequestMetadata) (int, error) {
	if operatorUserId <= 0 || organizationId <= 0 || len(req.Ids) == 0 {
		return 0, errors.New("invalid organization token batch delete request")
	}
	var err error
	req.IdempotencyKey, err = requireOrganizationIdempotencyKey(req.IdempotencyKey)
	if err != nil {
		return 0, err
	}
	ids, err := normalizeOrganizationTokenBatchDeleteIds(req.Ids)
	if err != nil {
		return 0, err
	}
	requestHash, err := organizationTokenBatchDeleteRequestHash(operatorUserId, organizationId, ids)
	if err != nil {
		return 0, err
	}
	deletedCount := len(ids)
	var affectedKeys []string
	replayDelete := false
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockOrganizationTokenWriteWithTx(tx, organizationId); err != nil {
			return err
		}
		actor, err := getOrganizationActorContextWithTxForAccessMode(tx, operatorUserId, organizationId, accessMode, true)
		if err != nil {
			return err
		}
		organization := actor.Organization
		if actor.ReadOnly {
			return organizationActorWriteDeniedError(actor)
		}
		if organization.Status == model.OrganizationStatusDissolved {
			return errors.New("organization dissolved")
		}
		var idempotencyRecord *model.OrganizationIdempotencyRecord
		if req.IdempotencyKey != "" {
			replayCount, record, err := prepareOrganizationTokenBatchDeleteIdempotencyWithTx(tx, operatorUserId, organizationId, req.IdempotencyKey, requestHash)
			if err != nil {
				return err
			}
			if record == nil {
				deletedCount = replayCount
				replayDelete = true
			} else {
				idempotencyRecord = record
			}
		}
		if !actor.Capabilities.CanManageAllTokens {
			if err := ensureActiveOrganizationMemberLockedWithTx(tx, organizationId, operatorUserId); err != nil {
				return err
			}
		}
		query := tx
		if replayDelete {
			query = query.Unscoped()
		}
		query = query.Where("scope_type = ? AND organization_id = ? AND id IN ?", model.TokenScopeOrganization, organizationId, ids)
		if !actor.Capabilities.CanManageAllTokens {
			query = query.Where("responsible_user_id = ? AND visibility = ?", operatorUserId, model.TokenVisibilityPrivate)
		}
		var tokens []model.Token
		if err := query.Find(&tokens).Error; err != nil {
			return err
		}
		if len(tokens) != len(ids) {
			return errors.New("permission denied")
		}
		if replayDelete {
			return nil
		}
		now := common.GetTimestamp()
		affectedKeys = make([]string, 0, len(tokens))
		for i := range tokens {
			tokens[i].UpdatedAt = now
			if err := tx.Model(&tokens[i]).Select("updated_at").Updates(&tokens[i]).Error; err != nil {
				return err
			}
			before := tokens[i]
			affectedKeys = append(affectedKeys, tokens[i].Key)
			if err := tx.Delete(&tokens[i]).Error; err != nil {
				return err
			}
			if err := recordOrganizationAudit(tx, organization, operatorUserId, actor.OperatorRoleForAudit, organizationAuditActionTokenDelete, "token", tokens[i].Id, before, nil, "", auditMetadata...); err != nil {
				return err
			}
		}
		return completeOrganizationTokenBatchDeleteIdempotencyWithTx(tx, idempotencyRecord, len(ids))
	})
	if err != nil {
		return 0, err
	}
	invalidateOrganizationTokenCaches(affectedKeys...)
	return deletedCount, nil
}

func TransferOrganizationTokenResponsibility(operatorUserId, organizationId int, accessMode string, tokenId, targetUserId int, reason string, auditMetadata ...OrganizationAuditRequestMetadata) (*model.Token, error) {
	return UpdateOrganizationTokenResponsibility(operatorUserId, organizationId, accessMode, tokenId, UpdateOrganizationTokenResponsibilityRequest{ResponsibleUserId: targetUserId, Reason: reason}, auditMetadata...)
}

func UpdateOrganizationTokenResponsibility(operatorUserId, organizationId int, accessMode string, tokenId int, req UpdateOrganizationTokenResponsibilityRequest, auditMetadata ...OrganizationAuditRequestMetadata) (*model.Token, error) {
	if operatorUserId <= 0 || organizationId <= 0 || tokenId <= 0 || req.ResponsibleUserId <= 0 {
		return nil, errors.New("invalid organization token responsibility request")
	}
	reason := strings.TrimSpace(req.Reason)
	now := common.GetTimestamp()
	var updated model.Token
	var affectedKey string
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockOrganizationTokenWriteWithTx(tx, organizationId); err != nil {
			return err
		}
		actor, err := getOrganizationActorContextWithTxForAccessMode(tx, operatorUserId, organizationId, accessMode, true)
		if err != nil {
			return err
		}
		organization := actor.Organization
		if actor.ReadOnly {
			return organizationActorWriteDeniedError(actor)
		}
		if !actor.Capabilities.CanManageAllTokens {
			return errors.New("permission denied")
		}
		if organization.Status == model.OrganizationStatusDissolved {
			return errors.New("organization dissolved")
		}
		token, err := getOrganizationTokenWithTx(tx, organizationId, tokenId)
		if err != nil {
			return err
		}
		if err := ensureOrganizationTokenResponsibleUserWithTx(tx, organization, token.Visibility, req.ResponsibleUserId); err != nil {
			return err
		}
		affectedKey = token.Key
		before := *token
		token.UserId = req.ResponsibleUserId
		token.ResponsibleUserId = req.ResponsibleUserId
		token.TransferReason = reason
		token.UpdatedAt = now
		if err := tx.Model(token).Select("user_id", "responsible_user_id", "transfer_reason", "updated_at").Updates(token).Error; err != nil {
			return err
		}
		if err := reconcileOrganizationTokenSystemBlockerWithTx(tx, token, now); err != nil {
			return err
		}
		if err := tx.First(token, token.Id).Error; err != nil {
			return err
		}
		updated = *token
		return recordOrganizationAudit(tx, organization, operatorUserId, actor.OperatorRoleForAudit, organizationAuditActionTokenResponsibilityUpdate, "token", token.Id, before, updated, reason, auditMetadata...)
	})
	if err != nil {
		return nil, err
	}
	invalidateOrganizationTokenCaches(affectedKey)
	model.NormalizeTokenScope(&updated)
	// 组织 Key 完整 secret 在列表和详情每次都返回，仅额外补全 masked 预览用于展示。
	fillOrganizationTokenKeyPreview(&updated)
	return &updated, nil
}

func validateOrganizationTokenRequest(req OrganizationTokenRequest, requireStatus bool) error {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return errors.New("token name is required")
	}
	if len(name) > 50 {
		return errors.New("token name too long")
	}
	if requireStatus && req.Status != common.TokenStatusEnabled && req.Status != common.TokenStatusDisabled {
		return errors.New("invalid token status")
	}
	if !req.UnlimitedQuota {
		if req.RemainQuota < 0 {
			return errors.New("token quota cannot be negative")
		}
		maxQuotaValue := int(1000000000 * common.QuotaPerUnit)
		if req.RemainQuota > maxQuotaValue {
			return errors.New("token quota exceeds maximum")
		}
	}
	visibility := strings.TrimSpace(req.Visibility)
	if visibility != "" && visibility != model.TokenVisibilityPrivate && visibility != model.TokenVisibilityPublic {
		return errors.New("invalid token visibility")
	}
	return nil
}

func normalizeOrganizationTokenVisibility(visibility string) string {
	visibility = strings.TrimSpace(visibility)
	if visibility == "" {
		return model.TokenVisibilityPrivate
	}
	return visibility
}

func ensureOrganizationTokenVisibilityAllowed(canManageAllTokens bool, visibility string) error {
	if normalizeOrganizationTokenVisibility(visibility) == model.TokenVisibilityPublic && !canManageAllTokens {
		return errors.New("permission denied")
	}
	return nil
}

func ensureOrganizationTokenResponsibleUserWithTx(tx *gorm.DB, organization *model.Organization, visibility string, targetUserId int) error {
	if organization == nil || targetUserId <= 0 {
		return errOrganizationTokenResponsibleUserInactive
	}
	var member model.OrganizationMember
	if err := model.LockForUpdate(activeOrganizationMemberWithEnabledUserQuery(tx)).
		Where("organization_members.organization_id = ? AND organization_members.user_id = ? AND organization_members.status = ?", organization.Id, targetUserId, model.OrganizationMemberStatusActive).
		First(&member).Error; err != nil {
		return errOrganizationTokenResponsibleUserInactive
	}
	if normalizeOrganizationTokenVisibility(visibility) == model.TokenVisibilityPublic && organization.OwnerUserId != targetUserId && member.Role != model.OrganizationRoleAdmin {
		return errors.New("public token responsible user must be organization owner or admin")
	}
	return nil
}

func lockOrganizationTokenWriteWithTx(tx *gorm.DB, organizationId int) error {
	var organization model.Organization
	return model.LockForUpdate(tx).Select("id").Where("id = ?", organizationId).First(&organization).Error
}

func organizationTokenBlockerClearanceForActor(actor *OrganizationActorContext) int {
	if actor == nil {
		return 0
	}
	if organizationActorHasPlatformAuthority(actor) {
		return organizationTokenBlockerClearancePlatform
	}
	if actor.Capabilities.CanManageAllTokens {
		return organizationTokenBlockerClearanceOrganizationAdmin
	}
	return organizationTokenBlockerClearanceMember
}

func clearOrganizationTokenManualBlockersForActorWithTx(tx *gorm.DB, tokenId int, actorClearance int, now int64) error {
	var blockers []model.OrganizationTokenSystemBlocker
	if err := tx.Where("token_id = ? AND status = ?", tokenId, model.OrganizationTokenBlockerStatusActive).Order("id asc").Find(&blockers).Error; err != nil {
		return err
	}
	if len(blockers) == 0 {
		return nil
	}
	selected := selectedOrganizationTokenSystemBlocker(blockers)
	switch selected.Reason {
	case organizationTokenBlockerReasonMemberDisabled:
		return ErrOrganizationTokenResponsibleMemberDisabled
	case organizationTokenBlockerReasonUserPlatformDisabled:
		return ErrOrganizationTokenResponsibleUserDisabled
	}
	for _, blocker := range blockers {
		if blocker.Reason != organizationTokenBlockerReasonManualDisabled {
			return errors.New("permission denied")
		}
		clearance := blocker.ClearanceLevel
		if clearance == 0 {
			clearance = organizationTokenBlockerClearanceOrganizationAdmin
		}
		if clearance > actorClearance || clearance >= organizationTokenBlockerClearanceTerminal {
			return ErrOrganizationTokenEnableForbidden
		}
	}
	return tx.Model(&model.OrganizationTokenSystemBlocker{}).
		Where("token_id = ? AND reason = ? AND status = ?", tokenId, organizationTokenBlockerReasonManualDisabled, model.OrganizationTokenBlockerStatusActive).
		Updates(map[string]any{"status": model.OrganizationTokenBlockerStatusCleared, "cleared_at": now, "updated_at": now}).Error
}

func getOrganizationTokenWithTx(tx *gorm.DB, organizationId, tokenId int) (*model.Token, error) {
	var token model.Token
	if err := tx.Where("id = ? AND scope_type = ? AND organization_id = ?", tokenId, model.TokenScopeOrganization, organizationId).First(&token).Error; err != nil {
		return nil, err
	}
	model.NormalizeTokenScope(&token)
	return &token, nil
}

// fillOrganizationTokenKeyPreview 补全 masked key 预览，但不清空完整 key。
// 组织 Key 完整 secret 在列表、详情、更新、转交、批量幂等重试每次都按操作者权限返回，
// masked 预览仅用于前端展示，保留完整 key 供客户端按需取用。
func fillOrganizationTokenKeyPreview(token *model.Token) {
	if token == nil || token.KeyPreview != "" {
		return
	}
	token.KeyPreview = model.MaskTokenKeyPreview(token.Key)
}

func ensureActiveOrganizationMemberWithTx(tx *gorm.DB, organizationId, userId int) error {
	return ensureActiveOrganizationMemberWithLockingTx(tx, organizationId, userId, false)
}

func ensureActiveOrganizationMemberLockedWithTx(tx *gorm.DB, organizationId, userId int) error {
	return ensureActiveOrganizationMemberWithLockingTx(tx, organizationId, userId, true)
}

func ensureActiveOrganizationMemberWithLockingTx(tx *gorm.DB, organizationId, userId int, lock bool) error {
	var member model.OrganizationMember
	query := activeOrganizationMemberWithEnabledUserQuery(tx)
	if lock {
		query = model.LockForUpdate(query)
	}
	if err := query.Where("organization_members.organization_id = ? AND organization_members.user_id = ? AND organization_members.status = ?", organizationId, userId, model.OrganizationMemberStatusActive).
		First(&member).Error; err != nil {
		return errOrganizationTokenResponsibleUserInactive
	}
	return nil
}

func countOrganizationTokensWithTx(tx *gorm.DB, organizationId int) (int64, error) {
	var count int64
	err := tx.Model(&model.Token{}).Where("scope_type = ? AND organization_id = ?", model.TokenScopeOrganization, organizationId).Count(&count).Error
	return count, err
}

func invalidateOrganizationTokenCaches(keys ...string) {
	for _, key := range keys {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if err := model.InvalidateTokenCache(key); err != nil {
			common.SysError("failed to invalidate organization token cache: " + err.Error())
		}
	}
}
