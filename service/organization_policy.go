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

const (
	OrganizationAccessModeWorkspace  = "workspace"
	OrganizationAccessModeManagement = "management"
	OrganizationAccessModeReadOnly   = "read_only"
	OrganizationAccessModeAdmin      = "admin"

	OrganizationPolicyRolePlatformAdmin = "platform_admin"
	OrganizationPolicyRolePlatformRoot  = "platform_root"

	OrganizationCapabilityViewOrganization              = "view_organization"
	OrganizationCapabilityUpdateOrganization            = "update_organization"
	OrganizationCapabilityDisableOrganization           = "disable_organization"
	OrganizationCapabilityEnableOrganization            = "enable_organization"
	OrganizationCapabilityDissolveOrganization          = "dissolve_organization"
	OrganizationCapabilityViewMembersFull               = "view_members_full"
	OrganizationCapabilityManageMembers                 = "manage_members"
	OrganizationCapabilityTransferOwner                 = "transfer_owner"
	OrganizationCapabilityViewInvitations               = "view_invitations"
	OrganizationCapabilityManageInvitations             = "manage_invitations"
	OrganizationCapabilityViewOrganizationTokens        = "view_organization_tokens"
	OrganizationCapabilityManageOrganizationTokens      = "manage_organization_tokens"
	OrganizationCapabilityCreatePublicOrganizationToken = "create_public_organization_token"
	OrganizationCapabilityViewOrganizationLogs          = "view_organization_logs"
	OrganizationCapabilityViewOrganizationUsage         = "view_organization_usage"
	OrganizationCapabilityViewOrganizationAuditLogs     = "view_organization_audit_logs"
	OrganizationCapabilityAdjustOrganizationQuota       = "adjust_organization_quota"
	OrganizationCapabilityViewReadOnlyOrganization      = "view_read_only_organization"
	OrganizationCapabilityExitOrganization              = "exit_organization"
	OrganizationCapabilityViewResource                  = "view_resource"
	OrganizationCapabilityManageResource                = "manage_resource"

	OrganizationPolicyResourceToken = "token"
)

type OrganizationPolicyUser struct {
	Id           int
	PlatformRole int
}

type OrganizationPolicyOrganization struct {
	Id          int
	Status      string
	OwnerUserId int
}

type OrganizationPolicyDisableState struct {
	ActiveSources   []string `json:"active_sources"`
	EffectiveSource string   `json:"effective_source"`
	CanSelfEnable   bool     `json:"can_self_enable"`
}

type OrganizationPolicyMember struct {
	UserId int
	Role   string
	Status string
}

type OrganizationPolicyResource struct {
	Type              string
	OwnerUserId       int
	ResponsibleUserId int
	Visibility        string
	Status            int
}

type OrganizationPolicyInput struct {
	CurrentUser  OrganizationPolicyUser
	Organization OrganizationPolicyOrganization
	DisableState OrganizationPolicyDisableState
	Member       *OrganizationPolicyMember
	Resource     OrganizationPolicyResource
	AccessMode   string
}

type OrganizationPolicyDecision struct {
	Allowed      bool
	AccessMode   string
	Role         string
	Capabilities []string
	ReadOnly     bool
	Code         types.ErrorCode
	Message      string
}

func (decision OrganizationPolicyDecision) HasCapability(capability string) bool {
	for _, item := range decision.Capabilities {
		if item == capability {
			return true
		}
	}
	return false
}

func EvaluateOrganizationPolicy(input OrganizationPolicyInput) OrganizationPolicyDecision {
	normalizeOrganizationPolicyInput(&input)
	decision := OrganizationPolicyDecision{Allowed: false, AccessMode: input.AccessMode, Role: organizationPolicyRole(input)}
	if input.CurrentUser.Id <= 0 || input.Organization.Id <= 0 {
		return denyOrganizationPolicy(decision, types.ErrorCodeOrganizationAccessDenied, "organization access denied")
	}

	switch input.AccessMode {
	case OrganizationAccessModeWorkspace:
		decision = evaluateWorkspaceOrganizationPolicy(input, decision)
	case OrganizationAccessModeManagement:
		decision = evaluateManagementOrganizationPolicy(input, decision)
	case OrganizationAccessModeReadOnly:
		decision = evaluateReadOnlyOrganizationPolicy(input, decision)
	case OrganizationAccessModeAdmin:
		decision = evaluateAdminOrganizationPolicy(input, decision)
	default:
		decision = denyOrganizationPolicy(decision, types.ErrorCodeOrganizationAccessDenied, "organization access denied")
	}
	return applyOrganizationResourcePolicy(input, decision)
}

func GetOrganizationPolicyDecisionForUser(userId int, organizationId int, accessMode string) (OrganizationPolicyDecision, error) {
	input, err := loadOrganizationPolicyInput(model.DB, userId, organizationId, accessMode)
	if err != nil {
		return OrganizationPolicyDecision{}, err
	}
	return EvaluateOrganizationPolicy(input), nil
}

func requireOrganizationPolicyCapabilityWithTx(tx *gorm.DB, userId int, organizationId int, accessMode string, capability string) error {
	input, err := loadOrganizationPolicyInput(tx, userId, organizationId, accessMode)
	if err != nil {
		return err
	}
	decision := EvaluateOrganizationPolicy(input)
	if !decision.Allowed {
		if decision.Code == types.ErrorCodeOrganizationAccessDenied {
			return errors.New("permission denied")
		}
		if decision.Message != "" {
			return errors.New(decision.Message)
		}
		return errors.New("permission denied")
	}
	if capability != "" && !decision.HasCapability(capability) {
		return errors.New("permission denied")
	}
	return nil
}

func loadOrganizationPolicyInput(tx *gorm.DB, userId int, organizationId int, accessMode string) (OrganizationPolicyInput, error) {
	if tx == nil || userId <= 0 || organizationId <= 0 {
		return OrganizationPolicyInput{}, errors.New("invalid organization request")
	}
	role, err := getUserRoleWithTx(tx, userId)
	if err != nil {
		return OrganizationPolicyInput{}, err
	}
	var organization model.Organization
	if err := tx.Where("id = ?", organizationId).First(&organization).Error; err != nil {
		return OrganizationPolicyInput{}, err
	}
	input := OrganizationPolicyInput{
		CurrentUser:  OrganizationPolicyUser{Id: userId, PlatformRole: role},
		Organization: organizationPolicyOrganizationFromModel(organization),
		DisableState: loadOrganizationPolicyDisableState(tx, organizationId),
		AccessMode:   accessMode,
	}
	var member model.OrganizationMember
	err = tx.Where("organization_id = ? AND user_id = ? AND status = ?", organizationId, userId, model.OrganizationMemberStatusActive).First(&member).Error
	if err == nil {
		input.Member = &OrganizationPolicyMember{UserId: member.UserId, Role: member.Role, Status: member.Status}
		return input, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return input, nil
	}
	return OrganizationPolicyInput{}, err
}

func organizationPolicyOrganizationFromModel(organization model.Organization) OrganizationPolicyOrganization {
	return OrganizationPolicyOrganization{Id: organization.Id, Status: organization.Status, OwnerUserId: organization.OwnerUserId}
}

func loadOrganizationPolicyDisableState(tx *gorm.DB, organizationId int) OrganizationPolicyDisableState {
	state := OrganizationPolicyDisableState{CanSelfEnable: true}
	var records []model.OrganizationDisableRecord
	err := tx.Where("organization_id = ? AND status = ?", organizationId, model.OrganizationRecordStatusActive).Find(&records).Error
	if err != nil || len(records) == 0 {
		return state
	}
	state.ActiveSources = make([]string, 0, len(records))
	for _, record := range records {
		state.ActiveSources = append(state.ActiveSources, record.Source)
		if state.EffectiveSource == "" {
			state.EffectiveSource = record.Source
		}
		if record.Source == model.OrganizationDisableSourcePlatform {
			state.CanSelfEnable = false
			state.EffectiveSource = record.Source
		}
	}
	return state
}

func normalizeOrganizationPolicyInput(input *OrganizationPolicyInput) {
	if input.Organization.Status == "" {
		input.Organization.Status = model.OrganizationStatusActive
	}
	if input.DisableState.ActiveSources == nil {
		input.DisableState.ActiveSources = []string{}
	}
}

func evaluateWorkspaceOrganizationPolicy(input OrganizationPolicyInput, decision OrganizationPolicyDecision) OrganizationPolicyDecision {
	if input.Organization.Status == model.OrganizationStatusDisabled {
		return denyOrganizationPolicy(decision, types.ErrorCodeOrganizationDisabled, "organization disabled")
	}
	if input.Organization.Status == model.OrganizationStatusDissolved {
		return denyOrganizationPolicy(decision, types.ErrorCodeOrganizationDissolved, "organization dissolved")
	}
	if !organizationPolicyActiveMember(input) {
		return denyOrganizationPolicy(decision, types.ErrorCodeOrganizationAccessDenied, "organization access denied")
	}
	decision.Allowed = true
	decision.Capabilities = capabilitiesForOrganizationRole(decision.Role, input, false)
	return decision
}

func evaluateManagementOrganizationPolicy(input OrganizationPolicyInput, decision OrganizationPolicyDecision) OrganizationPolicyDecision {
	if input.Organization.Status == model.OrganizationStatusDissolved {
		return denyOrganizationPolicy(decision, types.ErrorCodeOrganizationDissolved, "organization dissolved")
	}
	if !organizationPolicyActiveMember(input) || (decision.Role != model.OrganizationRoleOwner && decision.Role != model.OrganizationRoleAdmin) {
		return denyOrganizationPolicy(decision, types.ErrorCodeOrganizationAccessDenied, "organization access denied")
	}
	decision.Allowed = true
	decision.Capabilities = capabilitiesForOrganizationRole(decision.Role, input, false)
	if input.Organization.Status == model.OrganizationStatusDisabled {
		decision.Capabilities = disabledManagementCapabilities(decision.Capabilities, input.DisableState.CanSelfEnable)
	}
	return decision
}

func evaluateReadOnlyOrganizationPolicy(input OrganizationPolicyInput, decision OrganizationPolicyDecision) OrganizationPolicyDecision {
	if input.Organization.Status == model.OrganizationStatusDissolved {
		return denyOrganizationPolicy(decision, types.ErrorCodeOrganizationDissolved, "organization dissolved")
	}
	if input.Organization.Status != model.OrganizationStatusDisabled {
		return denyOrganizationPolicy(decision, types.ErrorCodeOrganizationAccessDenied, "organization access denied")
	}
	if !organizationPolicyActiveMember(input) || (decision.Role != model.OrganizationRoleOwner && decision.Role != model.OrganizationRoleAdmin) {
		return denyOrganizationPolicy(decision, types.ErrorCodeOrganizationDisabled, "organization disabled")
	}
	decision.Allowed = true
	decision.ReadOnly = true
	decision.Capabilities = readOnlyOrganizationCapabilities(decision.Role)
	return decision
}

func evaluateAdminOrganizationPolicy(input OrganizationPolicyInput, decision OrganizationPolicyDecision) OrganizationPolicyDecision {
	if !organizationPolicyPlatformAdmin(input) {
		return denyOrganizationPolicy(decision, types.ErrorCodeOrganizationAccessDenied, "organization access denied")
	}
	if input.Organization.Status == model.OrganizationStatusDissolved {
		decision.Allowed = true
		decision.ReadOnly = true
		decision.Code = types.ErrorCodeOrganizationDissolved
		decision.Message = "organization dissolved"
		decision.Capabilities = readOnlyOrganizationCapabilities(decision.Role)
		return decision
	}
	decision.Allowed = true
	decision.Capabilities = capabilitiesForPlatformRole(decision.Role, input, false)
	return decision
}

func organizationPolicyRole(input OrganizationPolicyInput) string {
	if input.AccessMode != OrganizationAccessModeAdmin {
		return organizationPolicyMemberRole(input)
	}
	if input.CurrentUser.PlatformRole >= common.RoleRootUser {
		return OrganizationPolicyRolePlatformRoot
	}
	if input.CurrentUser.PlatformRole >= common.RoleAdminUser {
		return OrganizationPolicyRolePlatformAdmin
	}
	return ""
}

func organizationPolicyMemberRole(input OrganizationPolicyInput) string {
	if input.Member == nil || input.Member.Status != model.OrganizationMemberStatusActive {
		return ""
	}
	if input.Organization.OwnerUserId > 0 && input.Organization.OwnerUserId == input.CurrentUser.Id {
		return model.OrganizationRoleOwner
	}
	if input.Member.Role == model.OrganizationRoleOwner {
		return model.OrganizationRoleMember
	}
	return input.Member.Role
}

func organizationPolicyPlatformAdmin(input OrganizationPolicyInput) bool {
	return input.CurrentUser.PlatformRole >= common.RoleAdminUser
}

func organizationPolicyActiveMember(input OrganizationPolicyInput) bool {
	return input.Member != nil && input.Member.Status == model.OrganizationMemberStatusActive
}

func capabilitiesForOrganizationRole(role string, input OrganizationPolicyInput, readOnly bool) []string {
	if readOnly {
		return readOnlyOrganizationCapabilities(role)
	}
	switch role {
	case model.OrganizationRoleOwner:
		return dedupeCapabilities(
			commonOrganizationViewCapabilities()...,
		).with(
			OrganizationCapabilityUpdateOrganization,
			OrganizationCapabilityDisableOrganization,
			OrganizationCapabilityDissolveOrganization,
			OrganizationCapabilityManageMembers,
			OrganizationCapabilityTransferOwner,
			OrganizationCapabilityViewInvitations,
			OrganizationCapabilityManageInvitations,
			OrganizationCapabilityManageOrganizationTokens,
			OrganizationCapabilityCreatePublicOrganizationToken,
			OrganizationCapabilityViewOrganizationAuditLogs,
		).values()
	case model.OrganizationRoleAdmin:
		return dedupeCapabilities(
			commonOrganizationViewCapabilities()...,
		).with(
			OrganizationCapabilityUpdateOrganization,
			OrganizationCapabilityDisableOrganization,
			OrganizationCapabilityManageMembers,
			OrganizationCapabilityViewInvitations,
			OrganizationCapabilityManageInvitations,
			OrganizationCapabilityManageOrganizationTokens,
			OrganizationCapabilityCreatePublicOrganizationToken,
			OrganizationCapabilityViewOrganizationAuditLogs,
			OrganizationCapabilityExitOrganization,
		).values()
	case model.OrganizationRoleMember:
		return []string{
			OrganizationCapabilityViewOrganization,
			OrganizationCapabilityViewOrganizationTokens,
			OrganizationCapabilityViewOrganizationLogs,
			OrganizationCapabilityViewOrganizationUsage,
			OrganizationCapabilityExitOrganization,
		}
	default:
		return nil
	}
}

func capabilitiesForPlatformRole(role string, input OrganizationPolicyInput, readOnly bool) []string {
	if readOnly {
		return readOnlyOrganizationCapabilities(role)
	}
	capabilities := dedupeCapabilities(commonOrganizationViewCapabilities()...).with(
		OrganizationCapabilityUpdateOrganization,
		OrganizationCapabilityDisableOrganization,
		OrganizationCapabilityEnableOrganization,
		OrganizationCapabilityManageMembers,
		OrganizationCapabilityViewInvitations,
		OrganizationCapabilityManageInvitations,
		OrganizationCapabilityManageOrganizationTokens,
		OrganizationCapabilityViewOrganizationAuditLogs,
		OrganizationCapabilityAdjustOrganizationQuota,
	)
	if role == OrganizationPolicyRolePlatformRoot {
		capabilities = capabilities.with(OrganizationCapabilityTransferOwner, OrganizationCapabilityDissolveOrganization)
	}
	return capabilities.values()
}

func commonOrganizationViewCapabilities() []string {
	return []string{
		OrganizationCapabilityViewOrganization,
		OrganizationCapabilityViewMembersFull,
		OrganizationCapabilityViewOrganizationTokens,
		OrganizationCapabilityViewOrganizationLogs,
		OrganizationCapabilityViewOrganizationUsage,
	}
}

func disabledManagementCapabilities(capabilities []string, canSelfEnable bool) []string {
	filtered := dedupeCapabilities(
		OrganizationCapabilityViewOrganization,
		OrganizationCapabilityViewMembersFull,
	)
	if canSelfEnable {
		filtered = filtered.with(OrganizationCapabilityEnableOrganization)
	}
	for _, capability := range capabilities {
		if capability == OrganizationCapabilityViewOrganizationAuditLogs || capability == OrganizationCapabilityViewOrganizationLogs || capability == OrganizationCapabilityViewOrganizationUsage {
			filtered = filtered.with(capability)
		}
	}
	return filtered.values()
}

func readOnlyOrganizationCapabilities(role string) []string {
	capabilities := dedupeCapabilities(
		OrganizationCapabilityViewOrganization,
		OrganizationCapabilityViewReadOnlyOrganization,
		OrganizationCapabilityViewOrganizationLogs,
		OrganizationCapabilityViewOrganizationUsage,
	)
	if role == model.OrganizationRoleOwner || role == model.OrganizationRoleAdmin || role == OrganizationPolicyRolePlatformAdmin || role == OrganizationPolicyRolePlatformRoot {
		capabilities = capabilities.with(OrganizationCapabilityViewMembersFull, OrganizationCapabilityViewInvitations, OrganizationCapabilityViewOrganizationAuditLogs, OrganizationCapabilityViewOrganizationTokens)
	}
	return capabilities.values()
}

func applyOrganizationResourcePolicy(input OrganizationPolicyInput, decision OrganizationPolicyDecision) OrganizationPolicyDecision {
	if !decision.Allowed || input.Resource.Type == "" {
		return decision
	}
	switch input.Resource.Type {
	case OrganizationPolicyResourceToken:
		return applyTokenResourcePolicy(input, decision)
	default:
		return decision
	}
}

func applyTokenResourcePolicy(input OrganizationPolicyInput, decision OrganizationPolicyDecision) OrganizationPolicyDecision {
	canViewAll := decision.HasCapability(OrganizationCapabilityViewMembersFull) || decision.HasCapability(OrganizationCapabilityManageOrganizationTokens)
	canManageAll := !decision.ReadOnly && decision.HasCapability(OrganizationCapabilityManageOrganizationTokens)
	isResponsible := input.Resource.ResponsibleUserId > 0 && input.Resource.ResponsibleUserId == input.CurrentUser.Id
	isOwner := input.Resource.OwnerUserId > 0 && input.Resource.OwnerUserId == input.CurrentUser.Id
	isPublic := input.Resource.Visibility == model.TokenVisibilityPublic
	isPrivate := !isPublic
	canView := decision.HasCapability(OrganizationCapabilityViewOrganizationTokens) && (canViewAll || (isPrivate && (isResponsible || isOwner)) || isPublic)
	canManage := canManageAll || (!decision.ReadOnly && (isResponsible || isOwner) && input.Resource.Visibility != model.TokenVisibilityPublic)
	capabilities := dedupeCapabilities(decision.Capabilities...)
	if canView {
		capabilities = capabilities.with(OrganizationCapabilityViewResource)
	}
	if canManage {
		capabilities = capabilities.with(OrganizationCapabilityManageResource)
	}
	decision.Capabilities = capabilities.values()
	return decision
}

func EvaluateOrganizationResourcePolicyForActor(actor *OrganizationActorContext, resource OrganizationPolicyResource) OrganizationPolicyDecision {
	if actor == nil || actor.Organization == nil {
		return denyOrganizationPolicy(OrganizationPolicyDecision{}, types.ErrorCodeOrganizationAccessDenied, "organization access denied")
	}
	platformRole := common.RoleCommonUser
	if actor.IsPlatformRoot {
		platformRole = common.RoleRootUser
	} else if actor.IsPlatformAdmin {
		platformRole = common.RoleAdminUser
	}
	input := OrganizationPolicyInput{
		CurrentUser:  OrganizationPolicyUser{Id: actor.UserId, PlatformRole: platformRole},
		Organization: organizationPolicyOrganizationFromModel(*actor.Organization),
		DisableState: OrganizationPolicyDisableState{CanSelfEnable: true},
		Resource:     resource,
		AccessMode:   actor.AccessMode,
	}
	if actor.Member != nil {
		input.Member = &OrganizationPolicyMember{UserId: actor.Member.UserId, Role: actor.Member.Role, Status: actor.Member.Status}
	}
	return EvaluateOrganizationPolicy(input)
}

func organizationPolicyCanViewTokenResource(actor *OrganizationActorContext, token *model.Token) bool {
	decision := EvaluateOrganizationResourcePolicyForActor(actor, organizationPolicyTokenResource(token))
	return decision.Allowed && decision.HasCapability(OrganizationCapabilityViewResource)
}

func organizationPolicyCanManageTokenResource(actor *OrganizationActorContext, token *model.Token) bool {
	decision := EvaluateOrganizationResourcePolicyForActor(actor, organizationPolicyTokenResource(token))
	return decision.Allowed && decision.HasCapability(OrganizationCapabilityManageResource)
}

func organizationPolicyTokenResource(token *model.Token) OrganizationPolicyResource {
	if token == nil {
		return OrganizationPolicyResource{Type: OrganizationPolicyResourceToken}
	}
	return OrganizationPolicyResource{
		Type:              OrganizationPolicyResourceToken,
		OwnerUserId:       token.UserId,
		ResponsibleUserId: token.ResponsibleUserId,
		Visibility:        token.Visibility,
		Status:            token.Status,
	}
}

func denyOrganizationPolicy(decision OrganizationPolicyDecision, code types.ErrorCode, message string) OrganizationPolicyDecision {
	decision.Allowed = false
	decision.Code = code
	decision.Message = message
	return decision
}

type capabilitySet struct {
	valuesByName map[string]bool
	ordered      []string
}

func dedupeCapabilities(capabilities ...string) capabilitySet {
	set := capabilitySet{valuesByName: map[string]bool{}, ordered: []string{}}
	return set.with(capabilities...)
}

func (set capabilitySet) with(capabilities ...string) capabilitySet {
	for _, capability := range capabilities {
		if capability == "" || set.valuesByName[capability] {
			continue
		}
		set.valuesByName[capability] = true
		set.ordered = append(set.ordered, capability)
	}
	return set
}

func (set capabilitySet) values() []string {
	return set.ordered
}
