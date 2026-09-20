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
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"

	"gorm.io/gorm"
)

type AccountContext struct {
	Type         string                         `json:"type"`
	Id           int                            `json:"id"`
	Name         string                         `json:"name"`
	Slug         string                         `json:"slug,omitempty"`
	Role         string                         `json:"role,omitempty"`
	Status       string                         `json:"status,omitempty"`
	AccessMode   string                         `json:"access_mode,omitempty"`
	Capabilities *OrganizationActorCapabilities `json:"capabilities,omitempty"`
}

type AccountContextsResponse struct {
	Current  *AccountContext  `json:"current"`
	Contexts []AccountContext `json:"contexts"`
}

type AccountContextError struct {
	Status  int
	Code    types.ErrorCode
	Message string
}

func (err *AccountContextError) Error() string {
	if err == nil {
		return ""
	}
	return err.Message
}

func ListAccountContexts(userId int) (*AccountContextsResponse, error) {
	if userId <= 0 {
		return nil, newAccountContextError(http.StatusForbidden, types.ErrorCodeOrganizationAccessDenied, "invalid user")
	}
	current, err := ResolveCurrentAccountContext(userId)
	if err != nil {
		return nil, err
	}
	contexts, err := availableAccountContexts(userId)
	if err != nil {
		return nil, err
	}
	return &AccountContextsResponse{Current: current, Contexts: contexts}, nil
}

func SetCurrentAccountContext(userId int, typ string, id int) (*AccountContext, error) {
	if userId <= 0 {
		return nil, newAccountContextError(http.StatusForbidden, types.ErrorCodeOrganizationAccessDenied, "invalid user")
	}
	context, err := resolveRequestedAccountContext(userId, typ, id)
	if err != nil {
		return nil, err
	}
	if err := saveUserAccountContext(userId, context.Type, context.Id); err != nil {
		return nil, err
	}
	return context, nil
}

func ResolveCurrentAccountContext(userId int) (*AccountContext, error) {
	var setting model.UserAccountContext
	err := model.DB.Where("user_id = ?", userId).First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return personalAccountContext(userId)
	}
	if err != nil {
		return nil, err
	}
	typ := setting.ContextType
	id := setting.ContextId
	if typ == "" || typ == model.AccountContextTypePersonal {
		return personalAccountContext(userId)
	}
	context, err := resolveRequestedAccountContext(userId, typ, id)
	if err != nil {
		personal, personalErr := personalAccountContext(userId)
		if personalErr != nil {
			return nil, personalErr
		}
		_ = saveUserAccountContext(userId, personal.Type, personal.Id)
		return personal, nil
	}
	return context, nil
}

func availableAccountContexts(userId int) ([]AccountContext, error) {
	personal, err := personalAccountContext(userId)
	if err != nil {
		return nil, err
	}
	contexts := []AccountContext{*personal}

	var rows []struct {
		Id          int
		Name        string
		Slug        string
		Role        string
		Status      string
		OwnerUserId int
	}
	err = model.DB.Table("organizations").
		Select("organizations.id, organizations.name, organizations.slug, organizations.status, organizations.owner_user_id, organization_members.role").
		Joins("JOIN organization_members ON organization_members.organization_id = organizations.id").
		Where("organization_members.user_id = ? AND organization_members.status = ? AND organizations.status = ?", userId, model.OrganizationMemberStatusActive, model.OrganizationStatusActive).
		Order("organizations.id desc").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		organization := model.Organization{Id: row.Id, Name: row.Name, Slug: row.Slug, Status: row.Status, OwnerUserId: row.OwnerUserId}
		member := model.OrganizationMember{OrganizationId: row.Id, UserId: userId, Role: row.Role, Status: model.OrganizationMemberStatusActive}
		context, err := organizationAccountContext(userId, &organization, &member, OrganizationAccessModeWorkspace)
		if err != nil {
			return nil, err
		}
		contexts = append(contexts, *context)
	}
	return contexts, nil
}

func resolveRequestedAccountContext(userId int, typ string, id int) (*AccountContext, error) {
	typ = strings.ToLower(strings.TrimSpace(typ))
	if typ == "" || typ == model.AccountContextTypePersonal {
		if id != 0 && id != userId {
			return nil, newAccountContextError(http.StatusForbidden, types.ErrorCodeOrganizationAccessDenied, "personal account context mismatch")
		}
		return personalAccountContext(userId)
	}
	if typ != model.AccountContextTypeOrganization || id <= 0 {
		return nil, newAccountContextError(http.StatusBadRequest, types.ErrorCodeOrganizationContextMismatch, "invalid account context")
	}
	organization, member, err := GetOrganizationForMember(userId, id, false)
	if err != nil {
		return nil, accountContextErrorFromErr(err)
	}
	if organization.Status != model.OrganizationStatusActive || member.Status != model.OrganizationMemberStatusActive {
		return nil, newAccountContextError(http.StatusForbidden, types.ErrorCodeOrganizationAccessDenied, "invalid account context")
	}
	return organizationAccountContext(userId, organization, member, OrganizationAccessModeWorkspace)
}

func organizationAccountContext(userId int, organization *model.Organization, member *model.OrganizationMember, accessMode string) (*AccountContext, error) {
	if organization == nil || member == nil {
		return nil, newAccountContextError(http.StatusForbidden, types.ErrorCodeOrganizationAccessDenied, "invalid account context")
	}
	platformRole, err := getUserRoleWithTx(model.DB, userId)
	if err != nil {
		return nil, err
	}
	decision := EvaluateOrganizationPolicy(OrganizationPolicyInput{
		CurrentUser:  OrganizationPolicyUser{Id: userId, PlatformRole: platformRole},
		Organization: organizationPolicyOrganizationFromModel(*organization),
		DisableState: loadOrganizationPolicyDisableState(model.DB, organization.Id),
		Member:       &OrganizationPolicyMember{UserId: member.UserId, Role: member.Role, Status: member.Status},
		AccessMode:   accessMode,
	})
	if !decision.Allowed {
		return nil, accountContextErrorFromPolicyDecision(decision)
	}
	capabilities := organizationActorCapabilitiesFromPolicy(decision)
	return &AccountContext{
		Type:         model.AccountContextTypeOrganization,
		Id:           organization.Id,
		Name:         organization.Name,
		Slug:         organization.Slug,
		Role:         decision.Role,
		Status:       organization.Status,
		AccessMode:   decision.AccessMode,
		Capabilities: &capabilities,
	}, nil
}

func personalAccountContext(userId int) (*AccountContext, error) {
	user, err := model.GetUserById(userId, false)
	if err != nil {
		return nil, err
	}
	name := user.DisplayName
	if name == "" {
		name = user.Username
	}
	return &AccountContext{Type: model.AccountContextTypePersonal, Id: userId, Name: name}, nil
}

func saveUserAccountContext(userId int, typ string, id int) error {
	if typ == model.AccountContextTypePersonal {
		id = userId
	}
	return model.DB.Save(&model.UserAccountContext{UserId: userId, ContextType: typ, ContextId: id, UpdatedAt: common.GetTimestamp()}).Error
}

func newAccountContextError(status int, code types.ErrorCode, message string) *AccountContextError {
	return &AccountContextError{Status: status, Code: code, Message: message}
}

func accountContextErrorFromErr(err error) *AccountContextError {
	message := err.Error()
	switch {
	case strings.Contains(message, "organization disabled"):
		return newAccountContextError(http.StatusForbidden, types.ErrorCodeOrganizationDisabled, message)
	case strings.Contains(message, "organization dissolved"):
		return newAccountContextError(http.StatusGone, types.ErrorCodeOrganizationDissolved, message)
	default:
		return newAccountContextError(http.StatusForbidden, types.ErrorCodeOrganizationAccessDenied, message)
	}
}

func accountContextErrorFromPolicyDecision(decision OrganizationPolicyDecision) *AccountContextError {
	status := http.StatusForbidden
	if decision.Code == types.ErrorCodeOrganizationContextMismatch {
		status = http.StatusBadRequest
	} else if decision.Code == types.ErrorCodeOrganizationDissolved {
		status = http.StatusGone
	}
	message := decision.Message
	if message == "" {
		message = "organization access denied"
	}
	return newAccountContextError(status, decision.Code, message)
}
