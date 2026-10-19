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
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// restrictMemberModelAccess 给组织成员行写入受限模型白名单；成员行保持 active，使
// model.GetUserModelAllowList 得到非 nil 的有效集合。
func restrictMemberModelAccess(t *testing.T, organizationId, userId int, modelAccess string) {
	t.Helper()
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", organizationId, userId).
		Update("model_access", modelAccess).Error)
}

// countOrganizationTokenRows 统计组织下的 Key 数量，用于断言被拒请求没有副作用。
func countOrganizationTokenRows(t *testing.T, organizationId int) int64 {
	t.Helper()
	var count int64
	require.NoError(t, model.DB.Model(&model.Token{}).
		Where("scope_type = ? AND organization_id = ?", model.TokenScopeOrganization, organizationId).
		Count(&count).Error)
	return count
}

// countOrganizationAuditRows 统计某审计动作的行数，用于断言被拒请求没有写审计。
func countOrganizationAuditRows(t *testing.T, organizationId int, actionType string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, model.DB.Model(&model.OrganizationAuditLog{}).
		Where("organization_id = ? AND action_type = ?", organizationId, actionType).
		Count(&count).Error)
	return count
}

// TestOrganizationTokenCreateEnforcesResponsibleMemberModelAccess 覆盖建组织 Key 时按责任成员
// 白名单校验 model limits 的三态：集内放行、任一条集外拒绝且不落库/不写审计、白名单 nil
// （成员不受限）全部放行；不启用或为空按「未设置」放行。
func TestOrganizationTokenCreateEnforcesResponsibleMemberModelAccess(t *testing.T) {
	tests := []struct {
		name               string
		restrictMember     bool // false = 成员行无白名单（nil）
		modelLimitsEnabled bool
		modelLimits        string
		wantDeniedModel    string // 空串 = 期望成功
	}{
		{name: "limits within the member allow list", restrictMember: true, modelLimitsEnabled: true, modelLimits: "allowed-model"},
		{name: "limits outside the member allow list", restrictMember: true, modelLimitsEnabled: true, modelLimits: "denied-model", wantDeniedModel: "denied-model"},
		{name: "one outside entry rejects the whole list", restrictMember: true, modelLimitsEnabled: true, modelLimits: "allowed-model,denied-model", wantDeniedModel: "denied-model"},
		{name: "disabled limits pass regardless", restrictMember: true, modelLimitsEnabled: false, modelLimits: "denied-model"},
		{name: "empty limits count as unset", restrictMember: true, modelLimitsEnabled: true, modelLimits: ""},
		{name: "unrestricted responsible member passes any limits", restrictMember: false, modelLimitsEnabled: true, modelLimits: "denied-model"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			owner, member, organization := createOrganizationTokenTestOrg(t)
			if tc.restrictMember {
				restrictMemberModelAccess(t, organization.Id, member.Id, `["allowed-model"]`)
			}

			token, err := CreateOrganizationToken(owner.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{
				Name:               "member-access-create",
				ResponsibleUserId:  member.Id,
				Visibility:         model.TokenVisibilityPrivate,
				ExpiredTime:        -1,
				ModelLimitsEnabled: tc.modelLimitsEnabled,
				ModelLimits:        tc.modelLimits,
			})

			if tc.wantDeniedModel != "" {
				require.ErrorContains(t, err, tc.wantDeniedModel)
				assert.Zero(t, countOrganizationTokenRows(t, organization.Id), "a rejected create must not store a key")
				assert.Zero(t, countOrganizationAuditRows(t, organization.Id, organizationAuditActionTokenCreate), "a rejected create must not write an audit row")
				return
			}
			require.NoError(t, err)
			assert.EqualValues(t, 1, countOrganizationTokenRows(t, organization.Id))
			assert.Equal(t, member.Id, token.ResponsibleUserId)
			assert.Equal(t, tc.modelLimitsEnabled, token.ModelLimitsEnabled)
			assert.Equal(t, tc.modelLimits, token.ModelLimits)
		})
	}
}

// TestOrganizationTokenUpdateEnforcesResponsibleMemberModelAccess 覆盖改组织 Key 时同样的
// 校验，并断言越界被拒的更新不落任何痕迹（存量 model limits 保持原样、不写审计）。
func TestOrganizationTokenUpdateEnforcesResponsibleMemberModelAccess(t *testing.T) {
	tests := []struct {
		name               string
		modelLimitsEnabled bool
		modelLimits        string
		wantDeniedModel    string
	}{
		{name: "limits within the member allow list", modelLimitsEnabled: true, modelLimits: "allowed-model"},
		{name: "limits outside the member allow list", modelLimitsEnabled: true, modelLimits: "denied-model", wantDeniedModel: "denied-model"},
		{name: "disabled limits pass regardless", modelLimitsEnabled: false, modelLimits: "denied-model"},
		{name: "empty limits count as unset", modelLimitsEnabled: true, modelLimits: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			owner, member, organization := createOrganizationTokenTestOrg(t)
			restrictMemberModelAccess(t, organization.Id, member.Id, `["allowed-model"]`)
			token, err := CreateOrganizationToken(owner.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{
				Name:               "member-access-update",
				ResponsibleUserId:  member.Id,
				Visibility:         model.TokenVisibilityPrivate,
				ExpiredTime:        -1,
				ModelLimitsEnabled: true,
				ModelLimits:        "allowed-model",
			})
			require.NoError(t, err)

			updated, err := UpdateOrganizationToken(owner.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{
				Name:               token.Name,
				Status:             common.TokenStatusEnabled,
				ResponsibleUserId:  member.Id,
				Visibility:         model.TokenVisibilityPrivate,
				ExpiredTime:        -1,
				ModelLimitsEnabled: tc.modelLimitsEnabled,
				ModelLimits:        tc.modelLimits,
			})

			var stored model.Token
			require.NoError(t, model.DB.First(&stored, token.Id).Error)
			if tc.wantDeniedModel != "" {
				require.ErrorContains(t, err, tc.wantDeniedModel)
				assert.True(t, stored.ModelLimitsEnabled, "a rejected update must leave the stored limits enabled")
				assert.Equal(t, "allowed-model", stored.ModelLimits, "a rejected update must leave the stored limits untouched")
				assert.Zero(t, countOrganizationAuditRows(t, organization.Id, organizationAuditActionTokenUpdate), "a rejected update must not write an audit row")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, token.Id, updated.Id)
			assert.Equal(t, tc.modelLimitsEnabled, stored.ModelLimitsEnabled)
			assert.Equal(t, tc.modelLimits, stored.ModelLimits)
		})
	}
}

// TestOrganizationTokenUpdateSkipsModelAccessCheckWhenLimitsUnchanged 覆盖白名单收窄后的存量
// Key：请求没有改动 model limits 时（改名、状态切换、改额度），不应再按收窄后的白名单拒绝，
// 否则这把 Key 会永久无法禁用或修复；运行时交集仍然会拦住越界的模型。
func TestOrganizationTokenUpdateSkipsModelAccessCheckWhenLimitsUnchanged(t *testing.T) {
	owner, member, organization := createOrganizationTokenTestOrg(t)
	token, err := CreateOrganizationToken(owner.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{
		Name:               "stale-limits-key",
		ResponsibleUserId:  member.Id,
		Visibility:         model.TokenVisibilityPrivate,
		ExpiredTime:        -1,
		ModelLimitsEnabled: true,
		ModelLimits:        "stale-model",
	})
	require.NoError(t, err)

	// 白名单在建 Key 之后收窄：存量 limits 现在落在白名单之外。
	restrictMemberModelAccess(t, organization.Id, member.Id, `["allowed-model"]`)

	updated, err := UpdateOrganizationToken(owner.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{
		Name:               "renamed-key",
		Status:             common.TokenStatusEnabled,
		ResponsibleUserId:  member.Id,
		Visibility:         model.TokenVisibilityPrivate,
		ExpiredTime:        -1,
		ModelLimitsEnabled: true,
		ModelLimits:        "stale-model",
	})
	require.NoError(t, err)
	assert.Equal(t, "renamed-key", updated.Name)

	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	assert.True(t, stored.ModelLimitsEnabled)
	assert.Equal(t, "stale-model", stored.ModelLimits)
}

// TestOrganizationTokenUpdateRechecksModelAccessWhenResponsibleChanges 覆盖换责任成员但
// limits 原样带回的更新：收窄的是新责任成员的白名单，若只在 limits 变化时校验，就会把
// Key 交给一个用不了这些模型的成员；校验必须把责任成员的变化也算作「需要重查」。
func TestOrganizationTokenUpdateRechecksModelAccessWhenResponsibleChanges(t *testing.T) {
	owner, memberA, organization := createOrganizationTokenTestOrg(t)
	restrictMemberModelAccess(t, organization.Id, memberA.Id, `["allowed-model"]`)
	memberB := createServiceTestUser(t, "org-token-member-b", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: organization.Id,
		UserId:         memberB.Id,
		Role:           model.OrganizationRoleMember,
		Status:         model.OrganizationMemberStatusActive,
	}).Error)
	restrictMemberModelAccess(t, organization.Id, memberB.Id, `["other-model"]`)

	token, err := CreateOrganizationToken(owner.Id, organization.Id, OrganizationAccessModeWorkspace, OrganizationTokenRequest{
		Name:               "handover-key",
		ResponsibleUserId:  memberA.Id,
		Visibility:         model.TokenVisibilityPrivate,
		ExpiredTime:        -1,
		ModelLimitsEnabled: true,
		ModelLimits:        "allowed-model",
	})
	require.NoError(t, err)

	_, err = UpdateOrganizationToken(owner.Id, organization.Id, OrganizationAccessModeWorkspace, token.Id, OrganizationTokenRequest{
		Name:               token.Name,
		Status:             common.TokenStatusEnabled,
		ResponsibleUserId:  memberB.Id,
		Visibility:         model.TokenVisibilityPrivate,
		ExpiredTime:        -1,
		ModelLimitsEnabled: true,
		// Unchanged: the narrowing is in the new holder's allow list, not the limits.
		ModelLimits: "allowed-model",
	})

	require.ErrorContains(t, err, "allowed-model")
	var stored model.Token
	require.NoError(t, model.DB.First(&stored, token.Id).Error)
	assert.Equal(t, memberA.Id, stored.ResponsibleUserId, "a rejected hand-over must leave the holder untouched")
	assert.Equal(t, "allowed-model", stored.ModelLimits, "a rejected hand-over must leave the stored limits untouched")
	assert.Zero(t, countOrganizationAuditRows(t, organization.Id, organizationAuditActionTokenUpdate), "a rejected hand-over must not write an audit row")
}

// TestOrganizationTokenBatchCreateEnforcesResponsibleMemberModelAccess 覆盖批量建 Key 的同一
// 校验：越界的整批拒绝且不落库，集内的照常建成。
func TestOrganizationTokenBatchCreateEnforcesResponsibleMemberModelAccess(t *testing.T) {
	owner, member, organization := createOrganizationTokenTestOrg(t)
	restrictMemberModelAccess(t, organization.Id, member.Id, `["allowed-model"]`)

	batchRequest := func(idempotencySuffix string, modelLimits string) OrganizationTokenBatchCreateRequest {
		return OrganizationTokenBatchCreateRequest{
			TokenCount:     2,
			IdempotencyKey: testOrganizationIdempotencyKey(t, idempotencySuffix),
			Token: OrganizationTokenRequest{
				Name:               "member-access-batch",
				ResponsibleUserId:  member.Id,
				Visibility:         model.TokenVisibilityPrivate,
				ExpiredTime:        -1,
				ModelLimitsEnabled: true,
				ModelLimits:        modelLimits,
			},
		}
	}

	_, err := BatchCreateOrganizationTokens(owner.Id, organization.Id, OrganizationAccessModeWorkspace, batchRequest("denied", "denied-model"))
	require.ErrorContains(t, err, "denied-model")
	assert.Zero(t, countOrganizationTokenRows(t, organization.Id), "a rejected batch must not store keys")

	result, err := BatchCreateOrganizationTokens(owner.Id, organization.Id, OrganizationAccessModeWorkspace, batchRequest("allowed", "allowed-model"))
	require.NoError(t, err)
	require.Len(t, result.Tokens, 2)
	assert.EqualValues(t, 2, countOrganizationTokenRows(t, organization.Id))
}
