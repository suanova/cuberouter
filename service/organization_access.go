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
	"github.com/QuantumNous/new-api/types"

	"gorm.io/gorm"
)

const organizationAuditOperatorRolePlatformRoot = "platform_root"

type OrganizationActorCapabilities struct {
	CanViewOrganization               bool `json:"can_view_organization"`
	CanViewOrganizationWideData       bool `json:"can_view_organization_wide_data"`
	CanViewOrganizationUsage          bool `json:"can_view_organization_usage"`
	CanViewOrganizationTokens         bool `json:"can_view_organization_tokens"`
	CanViewOrganizationLogs           bool `json:"can_view_organization_logs"`
	CanUpdateOrganization             bool `json:"can_update_organization"`
	CanDisableOrganization            bool `json:"can_disable_organization"`
	CanEnableOrganization             bool `json:"can_enable_organization"`
	CanManageMembers                  bool `json:"can_manage_members"`
	CanTransferOwner                  bool `json:"can_transfer_owner"`
	CanAddMembersDirectly             bool `json:"can_add_members_directly"`
	CanExitOrganization               bool `json:"can_exit_organization"`
	CanViewInvites                    bool `json:"can_view_invites"`
	CanCreateInvites                  bool `json:"can_create_invites"`
	CanRevokeInvites                  bool `json:"can_revoke_invites"`
	CanManageAllTokens                bool `json:"can_manage_all_tokens"`
	CanModifyOrganizationGroup        bool `json:"can_modify_organization_group"`
	CanDissolveOrganization           bool `json:"can_dissolve_organization"`
	CanViewAudit                      bool `json:"can_view_audit"`
	CanViewOrganizationBillingSummary bool `json:"can_view_organization_billing_summary"`
	ShowReturnOrganizationCenter      bool `json:"show_return_organization_center"`
	ShowReturnPersonalCenter          bool `json:"show_return_personal_center"`
}

type OrganizationActorContext struct {
	UserId               int                           `json:"user_id"`
	OrganizationId       int                           `json:"organization_id"`
	Organization         *model.Organization           `json:"-"`
	Member               *model.OrganizationMember     `json:"-"`
	AccessMode           string                        `json:"access_mode"`
	Role                 string                        `json:"role"`
	OrganizationRole     string                        `json:"organization_role"`
	PlatformRole         string                        `json:"platform_role"`
	IsOrganizationMember bool                          `json:"is_organization_member"`
	IsOrganizationAdmin  bool                          `json:"is_organization_admin"`
	IsPlatformAdmin      bool                          `json:"is_platform_admin"`
	IsPlatformRoot       bool                          `json:"is_platform_root"`
	OperatorRoleForAudit string                        `json:"operator_role_for_audit"`
	ReadOnly             bool                          `json:"read_only"`
	Capabilities         OrganizationActorCapabilities `json:"capabilities"`
	policyDecision       OrganizationPolicyDecision
}

func GetOrganizationActorContextForAccessMode(userId int, organizationId int, accessMode string, allowDissolvedRead bool) (*OrganizationActorContext, error) {
	return getOrganizationActorContextWithTxForAccessMode(model.DB, userId, organizationId, accessMode, allowDissolvedRead)
}

func getOrganizationActorContextWithTxForAccessMode(tx *gorm.DB, userId int, organizationId int, accessMode string, allowDissolvedRead bool) (*OrganizationActorContext, error) {
	if tx == nil || userId <= 0 || organizationId <= 0 {
		return nil, errors.New("invalid organization request")
	}
	switch accessMode {
	case OrganizationAccessModeWorkspace, OrganizationAccessModeManagement, OrganizationAccessModeReadOnly, OrganizationAccessModeAdmin:
	default:
		return nil, errors.New("organization access denied")
	}
	var organization model.Organization
	if err := tx.Where("id = ?", organizationId).First(&organization).Error; err != nil {
		return nil, err
	}
	if !allowDissolvedRead && organization.Status == model.OrganizationStatusDissolved {
		return nil, errors.New("organization dissolved")
	}

	role, err := getUserRoleWithTx(tx, userId)
	if err != nil {
		return nil, err
	}
	actor := &OrganizationActorContext{
		UserId:         userId,
		OrganizationId: organizationId,
		Organization:   &organization,
		ReadOnly:       organization.Status == model.OrganizationStatusDissolved,
	}
	var member model.OrganizationMember
	err = tx.Where("organization_id = ? AND user_id = ? AND status = ?", organizationId, userId, model.OrganizationMemberStatusActive).First(&member).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	var activeMember *model.OrganizationMember
	if err == nil {
		activeMember = &member
	}

	policyInput := OrganizationPolicyInput{
		CurrentUser:  OrganizationPolicyUser{Id: userId, PlatformRole: role},
		Organization: organizationPolicyOrganizationFromModel(organization),
		DisableState: loadOrganizationPolicyDisableState(tx, organizationId),
		AccessMode:   accessMode,
	}
	if activeMember != nil {
		policyInput.Member = &OrganizationPolicyMember{UserId: activeMember.UserId, Role: activeMember.Role, Status: activeMember.Status}
	}
	isPlatformAdmin := role >= common.RoleAdminUser
	decision := EvaluateOrganizationPolicy(policyInput)
	if !decision.Allowed {
		if decision.Message != "" {
			return nil, errors.New(decision.Message)
		}
		return nil, errors.New("permission denied")
	}

	organizationRole := organizationPolicyMemberRole(policyInput)
	actor.Member = activeMember
	actor.AccessMode = decision.AccessMode
	actor.IsOrganizationMember = activeMember != nil
	actor.IsOrganizationAdmin = organizationRole == model.OrganizationRoleOwner || organizationRole == model.OrganizationRoleAdmin
	actor.OrganizationRole = organizationRole
	actor.Role = decision.Role
	actor.IsPlatformAdmin = isPlatformAdmin
	actor.IsPlatformRoot = role >= common.RoleRootUser
	if actor.IsPlatformRoot {
		actor.PlatformRole = OrganizationPolicyRolePlatformRoot
	} else if actor.IsPlatformAdmin {
		actor.PlatformRole = OrganizationPolicyRolePlatformAdmin
	}
	actor.OperatorRoleForAudit = decision.Role
	actor.ReadOnly = decision.ReadOnly
	actor.Capabilities = organizationActorCapabilitiesFromPolicy(decision)
	actor.policyDecision = decision
	return actor, nil
}

func organizationActorWriteDeniedError(actor *OrganizationActorContext) error {
	if actor != nil && actor.policyDecision.Code == types.ErrorCodeOrganizationDissolved && actor.policyDecision.Message != "" {
		return errors.New(actor.policyDecision.Message)
	}
	return errors.New("permission denied")
}

func organizationActorHasPlatformAuthority(actor *OrganizationActorContext) bool {
	return actor != nil && actor.AccessMode == OrganizationAccessModeAdmin && (actor.Role == OrganizationPolicyRolePlatformAdmin || actor.Role == OrganizationPolicyRolePlatformRoot)
}

func getUserRoleWithTx(tx *gorm.DB, userId int) (int, error) {
	var user model.User
	if err := tx.Select("id", "role", "status").Where("id = ?", userId).First(&user).Error; err != nil {
		return 0, err
	}
	if user.Status != common.UserStatusEnabled {
		return 0, errors.New("permission denied")
	}
	return user.Role, nil
}

func organizationActorCapabilitiesFromPolicy(decision OrganizationPolicyDecision) OrganizationActorCapabilities {
	isPlatform := decision.Role == OrganizationPolicyRolePlatformAdmin || decision.Role == OrganizationPolicyRolePlatformRoot
	isOrganizationActor := !isPlatform && decision.Role != ""
	return OrganizationActorCapabilities{
		CanViewOrganization:               decision.HasCapability(OrganizationCapabilityViewOrganization),
		CanViewOrganizationWideData:       decision.HasCapability(OrganizationCapabilityViewMembersFull),
		CanViewOrganizationUsage:          decision.HasCapability(OrganizationCapabilityViewOrganizationUsage),
		CanViewOrganizationTokens:         decision.HasCapability(OrganizationCapabilityViewOrganizationTokens) || decision.HasCapability(OrganizationCapabilityManageOrganizationTokens),
		CanViewOrganizationLogs:           decision.HasCapability(OrganizationCapabilityViewOrganizationLogs),
		CanUpdateOrganization:             decision.HasCapability(OrganizationCapabilityUpdateOrganization),
		CanDisableOrganization:            decision.HasCapability(OrganizationCapabilityDisableOrganization),
		CanEnableOrganization:             decision.HasCapability(OrganizationCapabilityEnableOrganization),
		CanManageMembers:                  decision.HasCapability(OrganizationCapabilityManageMembers),
		CanTransferOwner:                  decision.HasCapability(OrganizationCapabilityTransferOwner),
		CanAddMembersDirectly:             isPlatform && decision.HasCapability(OrganizationCapabilityManageMembers),
		CanExitOrganization:               isOrganizationActor && decision.HasCapability(OrganizationCapabilityExitOrganization),
		CanViewInvites:                    decision.HasCapability(OrganizationCapabilityViewInvitations) || decision.HasCapability(OrganizationCapabilityManageInvitations),
		CanCreateInvites:                  !isPlatform && decision.HasCapability(OrganizationCapabilityManageInvitations),
		CanRevokeInvites:                  decision.HasCapability(OrganizationCapabilityManageInvitations),
		CanManageAllTokens:                decision.HasCapability(OrganizationCapabilityManageOrganizationTokens),
		CanModifyOrganizationGroup:        isPlatform && decision.HasCapability(OrganizationCapabilityUpdateOrganization),
		CanDissolveOrganization:           decision.HasCapability(OrganizationCapabilityDissolveOrganization),
		CanViewAudit:                      decision.HasCapability(OrganizationCapabilityViewOrganizationAuditLogs),
		CanViewOrganizationBillingSummary: decision.HasCapability(OrganizationCapabilityViewMembersFull) && decision.HasCapability(OrganizationCapabilityViewOrganizationUsage),
		ShowReturnOrganizationCenter:      isOrganizationActor,
		ShowReturnPersonalCenter:          isOrganizationActor,
	}
}
