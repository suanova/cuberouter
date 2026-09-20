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

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

type TokenScopeContext struct {
	ScopeType          string
	ScopeId            int
	BillingAccountType string
	BillingAccountId   int
	OrganizationId     int
	CreatorUserId      int
	ResponsibleUserId  int
	ActorUserId        int
	OrganizationQuota  int
	AccountGroup       string
}

func activeOrganizationMemberWithEnabledUserQuery(tx *gorm.DB) *gorm.DB {
	return tx.Table("organization_members").
		Select("organization_members.*").
		Joins("JOIN users ON users.id = organization_members.user_id AND users.deleted_at IS NULL AND users.status = ?", common.UserStatusEnabled)
}

func ValidateTokenScopeForRelay(token *model.Token) (*TokenScopeContext, error) {
	if token == nil {
		return nil, errors.New("token is nil")
	}
	model.NormalizeTokenScope(token)
	if token.ScopeType == model.TokenScopePersonal {
		accountGroup := ""
		if user, err := model.GetUserCache(token.UserId); err == nil {
			accountGroup = user.Group
		}
		return &TokenScopeContext{
			ScopeType:          model.AccountContextTypePersonal,
			ScopeId:            token.ScopeId,
			BillingAccountType: model.AccountContextTypePersonal,
			BillingAccountId:   token.UserId,
			CreatorUserId:      token.CreatorUserId,
			ResponsibleUserId:  token.ResponsibleUserId,
			ActorUserId:        token.UserId,
			AccountGroup:       accountGroup,
		}, nil
	}
	if token.ScopeType != model.TokenScopeOrganization {
		return nil, errors.New("invalid token scope")
	}
	if token.OrganizationId <= 0 {
		return nil, errors.New("organization token missing organization")
	}
	if token.Visibility != model.TokenVisibilityPrivate && token.Visibility != model.TokenVisibilityPublic {
		return nil, errors.New("invalid token visibility")
	}
	var organization model.Organization
	if err := model.DB.Where("id = ?", token.OrganizationId).First(&organization).Error; err != nil {
		return nil, err
	}
	if organization.Status == model.OrganizationStatusDissolved {
		return nil, errors.New("organization dissolved")
	}
	if organization.Status != model.OrganizationStatusActive {
		return nil, errors.New("organization disabled")
	}
	var activeBlockerCount int64
	if err := model.DB.Model(&model.OrganizationTokenSystemBlocker{}).
		Where("token_id = ? AND organization_id = ? AND status = ?", token.Id, token.OrganizationId, model.OrganizationTokenBlockerStatusActive).
		Count(&activeBlockerCount).Error; err != nil {
		return nil, err
	}
	if activeBlockerCount > 0 {
		return nil, errors.New("organization token is blocked")
	}
	var responsibleMember model.OrganizationMember
	if err := activeOrganizationMemberWithEnabledUserQuery(model.DB).
		Where("organization_members.organization_id = ? AND organization_members.user_id = ? AND organization_members.status = ?", token.OrganizationId, token.ResponsibleUserId, model.OrganizationMemberStatusActive).
		First(&responsibleMember).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		var activeMemberCount int64
		if err := model.DB.Model(&model.OrganizationMember{}).
			Where("organization_id = ? AND user_id = ? AND status = ?", token.OrganizationId, token.ResponsibleUserId, model.OrganizationMemberStatusActive).
			Count(&activeMemberCount).Error; err != nil {
			return nil, err
		}
		if activeMemberCount == 0 {
			return nil, errors.New("organization member disabled")
		}
		return nil, errors.New("organization responsible user disabled")
	}
	if token.Visibility == model.TokenVisibilityPublic && organization.OwnerUserId != token.ResponsibleUserId && responsibleMember.Role != model.OrganizationRoleAdmin {
		return nil, errors.New("public token responsible user must be organization owner or admin")
	}
	return &TokenScopeContext{
		ScopeType:          model.AccountContextTypeOrganization,
		ScopeId:            token.OrganizationId,
		BillingAccountType: model.AccountContextTypeOrganization,
		BillingAccountId:   token.OrganizationId,
		OrganizationId:     token.OrganizationId,
		CreatorUserId:      token.CreatorUserId,
		ResponsibleUserId:  token.ResponsibleUserId,
		ActorUserId:        token.ResponsibleUserId,
		OrganizationQuota:  organization.Quota - organization.UsedQuota,
		AccountGroup:       normalizeOrganizationGroup(organization.Group),
	}, nil
}
