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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationMemberTokenUsable(t *testing.T) {
	tests := []struct {
		name        string
		orgQuota    int
		orgUsed     int
		memberLimit int
		memberUsed  int
		wantErr     error
	}{
		{name: "organization quota is available", orgQuota: 10000},
		{name: "organization quota is zero", orgQuota: 0, wantErr: ErrOrganizationQuotaUnavailable},
		{name: "organization quota is exhausted", orgQuota: 10000, orgUsed: 10000, wantErr: ErrOrganizationQuotaUnavailable},
		{name: "organization quota is over-used", orgQuota: 10000, orgUsed: 12000, wantErr: ErrOrganizationQuotaUnavailable},
		{name: "configured member has remaining", orgQuota: 10000, memberLimit: 500, memberUsed: 200},
		{name: "configured member at limit", orgQuota: 10000, memberLimit: 500, memberUsed: 500, wantErr: ErrOrganizationMemberQuotaExhausted},
		{name: "configured member over limit", orgQuota: 10000, memberLimit: 500, memberUsed: 600, wantErr: ErrOrganizationMemberQuotaExhausted},
		{name: "organization is checked before member", orgQuota: 10000, orgUsed: 10000, memberLimit: 500, memberUsed: 500, wantErr: ErrOrganizationQuotaUnavailable},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			organization := &model.Organization{Quota: tc.orgQuota, UsedQuota: tc.orgUsed}
			member := &model.OrganizationMember{QuotaLimit: tc.memberLimit, UsedQuota: tc.memberUsed}
			err := ensureOrganizationMemberTokenUsable(organization, member)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func ptrString(value string) *string {
	return &value
}

func assertMemberDefaultToken(t *testing.T, token *model.Token, wantUnlimited bool, wantQuota int) {
	t.Helper()
	assert.Equal(t, wantUnlimited, token.UnlimitedQuota)
	assert.Equal(t, wantQuota, token.RemainQuota)
	assert.EqualValues(t, -1, token.ExpiredTime)
	assert.False(t, token.ModelLimitsEnabled)
	assert.Empty(t, token.ModelLimits)
	require.NotNil(t, token.AllowIps)
	assert.Empty(t, *token.AllowIps)
	assert.Empty(t, token.Group)
	assert.False(t, token.CrossGroupRetry)
	assert.Equal(t, model.TokenVisibilityPrivate, token.Visibility)
}

func TestOrganizationTokenMemberCreateAppliesDefaults(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t) // org quota 2000
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{
		Name: "member-default", ExpiredTime: 123, RemainQuota: 999, UnlimitedQuota: true,
		ModelLimitsEnabled: true, ModelLimits: "m1", AllowIps: ptrString("1.2.3.4"),
		Group: "vip", CrossGroupRetry: true,
	})
	require.NoError(t, err)
	assertMemberDefaultToken(t, token, true, 0)
	require.Equal(t, member.Id, token.ResponsibleUserId)
	require.Equal(t, member.Id, token.UserId)
}

func TestOrganizationTokenMemberCreateRejectsUnavailableOrganizationQuota(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("used_quota", 2000).Error)
	_, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "no-org-quota"})
	require.ErrorIs(t, err, ErrOrganizationQuotaUnavailable)
}

func TestOrganizationTokenMemberCreateRejectsExhaustedMemberQuota(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).
		Updates(map[string]any{"quota_limit": 500, "used_quota": 500}).Error)
	_, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "no-member-quota"})
	require.ErrorIs(t, err, ErrOrganizationMemberQuotaExhausted)
}

func TestOrganizationTokenMemberCreateUsesOrganizationDefaultQuota(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("default_member_token_quota", 15).Error)
	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{
		Name: "member-default-quota", RemainQuota: 999, UnlimitedQuota: true,
	})
	require.NoError(t, err)
	assertMemberDefaultToken(t, token, false, 15)
}

func TestOrganizationTokenMemberCreateChecksOrganizationBeforeMember(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("used_quota", 2000).Error)

	// 成员还有剩余，但组织额度已耗尽：仍然拒绝。
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).
		Updates(map[string]any{"quota_limit": 500, "used_quota": 0}).Error)
	_, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "org-exhausted"})
	require.ErrorIs(t, err, ErrOrganizationQuotaUnavailable)

	// 两边都耗尽：先报组织错误。
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).
		Updates(map[string]any{"quota_limit": 500, "used_quota": 500}).Error)
	_, err = CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "both-exhausted"})
	require.ErrorIs(t, err, ErrOrganizationQuotaUnavailable)
}

func TestOrganizationTokenDefaultChangeDoesNotAffectExistingKeys(t *testing.T) {
	admin, member, organization := createOrganizationTokenTestOrg(t)
	created, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "before-change"})
	require.NoError(t, err)
	assert.True(t, created.UnlimitedQuota)

	value := 100
	_, err = UpdateOrganization(admin.Id, organization.Id, OrganizationAccessModeManagement, UpdateOrganizationRequest{
		Name: organization.Name, DefaultMemberTokenQuota: &value,
	})
	require.NoError(t, err)

	var stored model.Token
	require.NoError(t, model.DB.First(&stored, created.Id).Error)
	assert.True(t, stored.UnlimitedQuota)
	assert.Zero(t, stored.RemainQuota)

	after, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "after-change"})
	require.NoError(t, err)
	assert.False(t, after.UnlimitedQuota)
	assert.Equal(t, 100, after.RemainQuota)
}

func TestOrganizationTokenAdminCreateKeepsExplicitFields(t *testing.T) {
	admin, _, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(admin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{
		Name: "admin-fields", ExpiredTime: 123, UnlimitedQuota: true,
		ModelLimitsEnabled: true, ModelLimits: "m1", AllowIps: ptrString("1.2.3.4"),
		CrossGroupRetry: true,
	})
	require.NoError(t, err)
	require.True(t, token.UnlimitedQuota)
	require.EqualValues(t, 123, token.ExpiredTime)
	require.True(t, token.ModelLimitsEnabled)
	require.Equal(t, "m1", token.ModelLimits)
	require.Equal(t, "1.2.3.4", *token.AllowIps)
	require.True(t, token.CrossGroupRetry)
}

func TestOrganizationTokenPlatformAdminWorkspaceKeepsExplicitFields(t *testing.T) {
	_, platformMember, organization := createOrganizationTokenPlatformMember(t, "org-token-platform-defaults")
	token, err := CreateOrganizationToken(platformMember.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{
		Name: "platform-fields", UnlimitedQuota: true, ExpiredTime: 123, ModelLimits: "m1",
	})
	require.NoError(t, err)
	require.True(t, token.UnlimitedQuota)
	require.EqualValues(t, 123, token.ExpiredTime)
}

func TestOrganizationTokenOrganizationAdminCreateKeepsExplicitFields(t *testing.T) {
	_, _, organization := createOrganizationTokenTestOrg(t)
	orgAdmin := createServiceTestUser(t, "org-token-org-admin", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: organization.Id, UserId: orgAdmin.Id,
		Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive,
	}).Error)
	token, err := CreateOrganizationToken(orgAdmin.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{
		Name: "org-admin-fields", UnlimitedQuota: true, ExpiredTime: 123,
	})
	require.NoError(t, err)
	require.True(t, token.UnlimitedQuota)
	require.EqualValues(t, 123, token.ExpiredTime)
}

// insertOrganizationTokenForTest 直接落一把组织 Key，绕过成员建 Key 的默认值策略。
// 计费/会话/日志测试关心的是 Key 上已有的字段，不是创建策略。
func insertOrganizationTokenForTest(t *testing.T, organization *model.Organization, member model.User, req OrganizationTokenRequest) (*model.Token, error) {
	t.Helper()
	return insertOrganizationTokenAsForTest(t, organization, member, member, req)
}

func insertOrganizationTokenAsForTest(t *testing.T, organization *model.Organization, creator, responsible model.User, req OrganizationTokenRequest) (*model.Token, error) {
	t.Helper()
	key, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	token := req
	token.UserId = responsible.Id
	token.Key = key
	if token.Status == 0 {
		token.Status = common.TokenStatusEnabled
	}
	token.CreatedTime = now
	token.AccessedTime = now
	token.UpdatedAt = now
	token.ScopeType = model.TokenScopeOrganization
	token.ScopeId = organization.Id
	token.OrganizationId = organization.Id
	token.CreatorUserId = creator.Id
	token.ResponsibleUserId = responsible.Id
	if token.Visibility == "" {
		token.Visibility = model.TokenVisibilityPrivate
	}
	if err := model.DB.Create(&token).Error; err != nil {
		return nil, err
	}
	model.NormalizeTokenScope(&token)
	return &token, nil
}

func TestOrganizationTokenMemberBatchCreateRejectsMultipleKeys(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	_, err := BatchCreateOrganizationTokens(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenBatchCreateRequest{
		TokenCount:     2,
		IdempotencyKey: testOrganizationIdempotencyKey(t, "member-batch-two"),
		Token:          OrganizationTokenRequest{Name: "member-batch"},
	})
	require.ErrorContains(t, err, "member cannot create multiple keys at once")
}

func TestOrganizationTokenMemberBatchCreateAppliesDefaultsForSingleKey(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	result, err := BatchCreateOrganizationTokens(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenBatchCreateRequest{
		TokenCount:     1,
		IdempotencyKey: testOrganizationIdempotencyKey(t, "member-batch-one"),
		Token:          OrganizationTokenRequest{Name: "member-batch", UnlimitedQuota: true, ExpiredTime: 123},
	})
	require.NoError(t, err)
	require.Len(t, result.Tokens, 1)
	assertMemberDefaultToken(t, result.Tokens[0], true, 0)
}

func TestOrganizationTokenMemberUpdatePreservesRestrictedFields(t *testing.T) {
	_, member, organization := createOrganizationTokenTestOrg(t)
	created, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "member-edit"})
	require.NoError(t, err)

	// 模拟管理员代建/存量 Key 上的自定义受控字段。
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", created.Id).Updates(map[string]any{
		"unlimited_quota": true, "expired_time": 123, "remain_quota": 456,
		"model_limits_enabled": true, "model_limits": "m1",
		"allow_ips": "1.2.3.4", "cross_group_retry": true,
	}).Error)

	updated, err := UpdateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, created.Id, OrganizationTokenRequest{
		Name: "renamed", Status: common.TokenStatusEnabled,
		UnlimitedQuota: false, RemainQuota: 999, ExpiredTime: 999,
		ModelLimitsEnabled: false, ModelLimits: "", AllowIps: ptrString(""), CrossGroupRetry: false,
	})
	require.NoError(t, err)
	require.Equal(t, "renamed", updated.Name)
	require.True(t, updated.UnlimitedQuota)
	require.EqualValues(t, 123, updated.ExpiredTime)
	require.Equal(t, 456, updated.RemainQuota)
	require.True(t, updated.ModelLimitsEnabled)
	require.Equal(t, "m1", updated.ModelLimits)
	require.Equal(t, "1.2.3.4", *updated.AllowIps)
	require.True(t, updated.CrossGroupRetry)
}

func TestOrganizationTokenMemberDefaultsExternalDatabase(t *testing.T) {
	db, _ := setupOrganizationExternalConcurrencyDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Organization{}, &model.OrganizationMember{},
		&model.OrganizationDisableRecord{}, &model.OrganizationAuditLog{},
		&model.Token{}, &model.OrganizationTokenSystemBlocker{},
	))

	suffix := strings.ReplaceAll(common.GetUUID(), "-", "")[:8]
	owner := model.User{Username: "external-token-defaults-owner-" + suffix, Password: "password", DisplayName: "external-token-defaults-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "etd-o-" + suffix}
	member := model.User{Username: "external-token-defaults-member-" + suffix, Password: "password", DisplayName: "external-token-defaults-member", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "etd-m-" + suffix}
	require.NoError(t, db.Create(&owner).Error)
	require.NoError(t, db.Create(&member).Error)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "External Token Defaults " + suffix})
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("quota", 10000).Error)
	require.NoError(t, db.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	t.Cleanup(func() {
		require.NoError(t, db.Where("organization_id = ?", organization.Id).Delete(&model.OrganizationAuditLog{}).Error)
		require.NoError(t, db.Unscoped().Where("organization_id = ? AND scope_type = ?", organization.Id, model.TokenScopeOrganization).Delete(&model.Token{}).Error)
		require.NoError(t, db.Where("organization_id = ?", organization.Id).Delete(&model.OrganizationMember{}).Error)
		require.NoError(t, db.Where("id = ?", organization.Id).Delete(&model.Organization{}).Error)
		require.NoError(t, db.Unscoped().Delete(&model.User{}, []int{owner.Id, member.Id}).Error)
	})

	token, err := CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "external-member-default"})
	require.NoError(t, err)
	assert.True(t, token.UnlimitedQuota)
	assert.Zero(t, token.RemainQuota)

	require.NoError(t, db.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("default_member_token_quota", 100).Error)
	token, err = CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "external-member-configured"})
	require.NoError(t, err)
	assert.False(t, token.UnlimitedQuota)
	assert.Equal(t, 100, token.RemainQuota)

	require.NoError(t, db.Model(&model.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).
		Updates(map[string]any{"quota_limit": 500, "used_quota": 200}).Error)
	token, err = CreateOrganizationToken(member.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{Name: "external-member-configured-member-quota"})
	require.NoError(t, err)
	assert.False(t, token.UnlimitedQuota)
	assert.Equal(t, 100, token.RemainQuota)
}
