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
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestOrganizationAuditSnapshotInviteStoresTargetEmailRoleAndLinkedUsername(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "audit-snapshot-invite-admin", common.RoleCommonUser)
	target := createServiceTestUser(t, "audit-snapshot-invite-target", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&target).Update("email", "target@example.com").Error)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Audit Snapshot Invite Org"})
	require.NoError(t, err)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error { return nil })

	invite, err := CreateEmailInvite(admin.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "target@example.com", Role: model.OrganizationRoleMember})
	require.NoError(t, err)

	var log model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND target_type = ?", organization.Id, "invite").Order("id desc").First(&log).Error)
	require.NotEmpty(t, invite.Token)
	require.Equal(t, "target@example.com", log.TargetName)
	require.Contains(t, log.TargetMetadata, "\"target_email\":\"target@example.com\"")
	require.Contains(t, log.TargetMetadata, "\"role\":\"member\"")
	require.Contains(t, log.TargetMetadata, "\"target_username\":\"audit-snapshot-invite-target\"")
	require.NotContains(t, log.TargetMetadata, invite.Token)
	require.NotContains(t, log.AfterData, invite.Token)
	require.NotContains(t, log.AfterData, "\"token\"")
}

func TestOrganizationAuditSnapshotMemberUpdateStoresUsernameEmailRoleAndStatus(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "audit-snapshot-member-admin", common.RoleCommonUser)
	member := createServiceTestUser(t, "audit-snapshot-member-user", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&member).Update("email", "member@example.com").Error)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Audit Snapshot Member Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	err = UpdateOrganizationMember(admin.Id, organization.Id, OrganizationAccessModeManagement, member.Id, UpdateMemberRequest{Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusDisabled, IdempotencyKey: testOrganizationIdempotencyKey(t, "disable-member")})
	require.NoError(t, err)

	var log model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND target_type = ? AND action_type = ?", organization.Id, "member", organizationAuditActionMemberUpdate).Order("id desc").First(&log).Error)
	require.Equal(t, "audit-snapshot-member-user", log.TargetName)
	require.Contains(t, log.TargetMetadata, "\"user_id\":")
	require.Contains(t, log.TargetMetadata, "\"username\":\"audit-snapshot-member-user\"")
	require.Contains(t, log.TargetMetadata, "\"email\":\"member@example.com\"")
	require.Contains(t, log.TargetMetadata, "\"role\":\"admin\"")
	require.Contains(t, log.TargetMetadata, "\"status\":\"disabled\"")
}

func TestOrganizationAuditSnapshotMemberRemoveStoresUsernameEmailRoleAndRemovedStatus(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "audit-snapshot-remove-admin", common.RoleCommonUser)
	member := createServiceTestUser(t, "audit-snapshot-remove-user", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&member).Update("email", "removed-member@example.com").Error)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Audit Snapshot Remove Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	err = RemoveOrganizationMember(admin.Id, organization.Id, OrganizationAccessModeManagement, member.Id, RemoveMemberRequest{Reason: "remove test", IdempotencyKey: testOrganizationIdempotencyKey(t, "remove-member")})
	require.NoError(t, err)

	var log model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND target_type = ? AND action_type = ?", organization.Id, "member", organizationAuditActionMemberRemove).Order("id desc").First(&log).Error)
	require.Equal(t, "audit-snapshot-remove-user", log.TargetName)
	require.Contains(t, log.TargetMetadata, "\"user_id\":")
	require.Contains(t, log.TargetMetadata, "\"username\":\"audit-snapshot-remove-user\"")
	require.Contains(t, log.TargetMetadata, "\"email\":\"removed-member@example.com\"")
	require.Contains(t, log.TargetMetadata, "\"role\":\"member\"")
	require.Contains(t, log.TargetMetadata, "\"status\":\"removed\"")
}

func TestOrganizationAuditQueryDoesNotHydrateMissingTargetSnapshot(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "audit-no-hydrate-admin", common.RoleCommonUser)
	member := createServiceTestUser(t, "audit-no-hydrate-member", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Audit No Hydrate Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, UpdateOrganizationMember(admin.Id, organization.Id, OrganizationAccessModeManagement, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusDisabled, IdempotencyKey: testOrganizationIdempotencyKey(t, "disable-member")}))
	require.NoError(t, model.DB.Model(&model.OrganizationAuditLog{}).Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionMemberUpdate).Updates(map[string]any{"target_name": "", "target_metadata": ""}).Error)

	logs, _, err := ListOrganizationAuditLogs(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationAuditQueryRequest{TargetType: "member", ActionType: organizationAuditActionMemberUpdate, Limit: 20})
	require.NoError(t, err)
	require.NotEmpty(t, logs)
	require.Empty(t, logs[0].TargetName)
	require.Empty(t, logs[0].TargetMetadata)
}

func TestOrganizationAuditSnapshotTokenStoresMaskedAPIKeyNameAndUsernames(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "audit-snapshot-token-admin", common.RoleCommonUser)
	responsible := createServiceTestUser(t, "audit-snapshot-token-owner", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Audit Snapshot Token Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: responsible.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	created, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "Audit Token", ResponsibleUserId: responsible.Id})
	require.NoError(t, err)

	var log model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND target_type = ? AND target_id = ?", organization.Id, "token", created.Id).Order("id desc").First(&log).Error)
	require.NotContains(t, log.TargetName, created.Key)
	require.Contains(t, log.TargetName, "***")
	require.NotContains(t, log.TargetMetadata, created.Key)
	require.Contains(t, log.TargetMetadata, "\"api_key\":\"sk-")
	require.Contains(t, log.TargetMetadata, "***")
	require.Contains(t, log.TargetMetadata, "\"name\":\"Audit Token\"")
	require.Contains(t, log.TargetMetadata, "\"responsible_username\":\"audit-snapshot-token-owner\"")
	require.Contains(t, log.TargetMetadata, "\"creator_username\":\"audit-snapshot-token-admin\"")
	require.False(t, strings.Contains(log.TargetMetadata, "\"key\":"))
}

func TestOrganizationAuditRedactsSensitivePayloadFields(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "audit-redaction-admin", common.RoleCommonUser)
	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Audit Redaction Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Where("organization_id = ?", organization.Id).Delete(&model.OrganizationAuditLog{}).Error)
	fullKey := "sk-redaction-full-key-1234567890"
	rawInviteToken := "raw-invitation-token-123456"
	tokenHash := model.HashOrganizationInviteToken(rawInviteToken)
	tokenCiphertext := "encrypted-token-ciphertext"
	fullPrompt := "full user prompt should not be audited"
	fullResponse := "full model response should not be audited"
	sensitiveReason := "api_key=" + fullKey + " invite_token=" + rawInviteToken + " tokenHash=" + tokenHash + " tokenCiphertext=" + tokenCiphertext + " prompt=" + fullPrompt + " response=" + fullResponse

	payload := map[string]any{
		"name":             "safe audit target",
		"reason":           sensitiveReason,
		"api_key":          fullKey,
		"apiKey":           fullKey,
		"authHeader":       "Bearer " + fullKey,
		"key":              fullKey,
		"token":            rawInviteToken,
		"token_hash":       tokenHash,
		"tokenHash":        tokenHash,
		"token_ciphertext": tokenCiphertext,
		"TokenCiphertext":  tokenCiphertext,
		"clientSecret":     "client-secret-value",
		"accessToken":      "access-token-value",
		"prompt":           fullPrompt,
		"response":         fullResponse,
		"requestBody":      fullPrompt,
		"responseBody":     fullResponse,
		"nested": map[string]any{
			"content":     fullPrompt,
			"api_key":     fullKey,
			"accessToken": "nested-access-token",
		},
		"items": []any{
			map[string]any{"response_body": fullResponse},
		},
	}

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		return recordOrganizationAudit(tx, organization, admin.Id, model.OrganizationRoleOwner, "organization.test.redaction", "quota_adjustment", 0, payload, payload, sensitiveReason)
	})
	require.NoError(t, err)

	var log model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", organization.Id, "organization.test.redaction").First(&log).Error)
	combined := log.TargetName + log.TargetMetadata + log.BeforeData + log.AfterData + log.Reason
	require.Contains(t, combined, "safe audit target")
	require.Contains(t, combined, "***")
	require.NotContains(t, combined, fullKey)
	require.NotContains(t, combined, rawInviteToken)
	require.NotContains(t, combined, tokenHash)
	require.NotContains(t, combined, tokenCiphertext)
	require.NotContains(t, combined, "client-secret-value")
	require.NotContains(t, combined, "access-token-value")
	require.NotContains(t, combined, "nested-access-token")
	require.NotContains(t, combined, fullPrompt)
	require.NotContains(t, combined, fullResponse)
	require.NotContains(t, combined, "token_hash")
	require.NotContains(t, combined, "tokenHash")
	require.NotContains(t, combined, "token_ciphertext")
	require.NotContains(t, combined, "TokenCiphertext")
	require.NotContains(t, combined, "clientSecret")
	require.NotContains(t, combined, "accessToken")
	require.NotContains(t, combined, "\"prompt\"")
	require.NotContains(t, combined, "\"response\"")
	require.NotContains(t, combined, "requestBody")
	require.NotContains(t, combined, "responseBody")
}

func TestOrganizationAuditSnapshotQuotaAdjustmentStoresDeltaBeforeAndAfter(t *testing.T) {
	setupServiceTestDB(t)
	platformAdmin := createServiceTestUser(t, "audit-snapshot-quota-platform", common.RoleAdminUser)
	creator := createServiceTestUser(t, "audit-snapshot-quota-creator", common.RoleCommonUser)
	organization, err := CreateOrganization(creator.Id, CreateOrganizationRequest{Name: "Audit Snapshot Quota Org"})
	require.NoError(t, err)

	adjustment, err := AdjustOrganizationQuota(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, OrganizationQuotaAdjustmentRequest{QuotaDelta: 123, Reason: "quota audit", IdempotencyKey: testOrganizationIdempotencyKey(t, "quota-adjust")})
	require.NoError(t, err)

	var log model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND target_type = ? AND target_id = ?", organization.Id, "quota_adjustment", adjustment.Id).Order("id desc").First(&log).Error)
	require.Equal(t, "quota audit", log.TargetName)
	require.Contains(t, log.TargetMetadata, "\"quota_delta\":123")
	require.Contains(t, log.TargetMetadata, "\"quota_before\":0")
	require.Contains(t, log.TargetMetadata, "\"quota_after\":123")
}

func TestOrganizationAuditSnapshotOrganizationStoresNameAndID(t *testing.T) {
	setupServiceTestDB(t)
	admin := createServiceTestUser(t, "audit-snapshot-org-admin", common.RoleCommonUser)

	organization, err := CreateOrganization(admin.Id, CreateOrganizationRequest{Name: "Audit Snapshot Org"})
	require.NoError(t, err)

	var log model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND target_type = ?", organization.Id, "organization").Order("id desc").First(&log).Error)
	require.Equal(t, "Audit Snapshot Org", log.TargetName)
	require.Contains(t, log.TargetMetadata, "\"id\":")
	require.Contains(t, log.TargetMetadata, "\"name\":\"Audit Snapshot Org\"")
}

func TestOrganizationAuditAccessModeOperatorRole(t *testing.T) {
	setupServiceTestDB(t)
	root := createServiceTestUser(t, "audit-access-mode-root", common.RoleRootUser)
	target := createServiceTestUser(t, "audit-access-mode-target", common.RoleCommonUser)
	organization, err := CreateOrganization(root.Id, CreateOrganizationRequest{Name: "Audit Access Mode Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	require.NoError(t, UpdateOrganizationMember(root.Id, organization.Id, OrganizationAccessModeManagement, target.Id, UpdateMemberRequest{Role: model.OrganizationRoleAdmin, Reason: "ordinary role"}))
	var ordinaryAudit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionMemberUpdate).Order("id desc").First(&ordinaryAudit).Error)
	require.Equal(t, model.OrganizationRoleOwner, ordinaryAudit.OperatorRole)

	require.NoError(t, UpdateOrganizationMember(root.Id, organization.Id, OrganizationAccessModeAdmin, target.Id, UpdateMemberRequest{Role: model.OrganizationRoleMember, Reason: "platform role"}))
	var adminAudit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionMemberUpdate).Order("id desc").First(&adminAudit).Error)
	require.Equal(t, OrganizationPolicyRolePlatformRoot, adminAudit.OperatorRole)
}
