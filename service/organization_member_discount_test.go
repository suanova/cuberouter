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
	"github.com/stretchr/testify/require"
)

// createMemberModelDiscountTestOrg 建一个 owner（组织创建者）+ admin + member 三档成员的组织。
// owner 的 owner 角色由 organization.owner_user_id 决定，不靠成员行里的 role 值。
func createMemberModelDiscountTestOrg(t *testing.T) (model.User, model.User, model.User, *model.Organization) {
	t.Helper()
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "member-discount-owner", common.RoleCommonUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Member Discount Org"})
	require.NoError(t, err)
	admin := createServiceTestUser(t, "member-discount-admin", common.RoleCommonUser)
	member := createServiceTestUser(t, "member-discount-member", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	return owner, admin, member, organization
}

func storedMemberModelDiscounts(t *testing.T, userId int) string {
	t.Helper()
	var stored model.User
	require.NoError(t, model.DB.Select("id", "model_discounts").First(&stored, userId).Error)
	return stored.ModelDiscounts
}

func TestOrganizationMemberModelDiscountsCapabilityMapping(t *testing.T) {
	owner, admin, member, organization := createMemberModelDiscountTestOrg(t)

	ownerActor, err := GetOrganizationActorContextForAccessMode(owner.Id, organization.Id, OrganizationAccessModeManagement, true)
	require.NoError(t, err)
	require.True(t, ownerActor.Capabilities.CanManageMemberDiscounts)

	adminActor, err := GetOrganizationActorContextForAccessMode(admin.Id, organization.Id, OrganizationAccessModeManagement, true)
	require.NoError(t, err)
	require.True(t, adminActor.Capabilities.CanManageMemberDiscounts)

	memberActor, err := GetOrganizationActorContextForAccessMode(member.Id, organization.Id, OrganizationAccessModeWorkspace, true)
	require.NoError(t, err)
	require.False(t, memberActor.Capabilities.CanManageMemberDiscounts)
}

func TestOrganizationMemberModelDiscountsPermissionMatrix(t *testing.T) {
	owner, admin, member, organization := createMemberModelDiscountTestOrg(t)
	outsider := createServiceTestUser(t, "member-discount-outsider", common.RoleCommonUser)
	discounts := map[string]float64{"gpt-discount-a": 0.5}
	expectedRaw := model.MarshalUserModelDiscounts(discounts)

	testCases := []struct {
		name       string
		operator   model.User
		accessMode string
		wantErr    string
	}{
		{name: "owner", operator: owner, accessMode: OrganizationAccessModeManagement},
		{name: "admin", operator: admin, accessMode: OrganizationAccessModeManagement},
		{name: "member", operator: member, accessMode: OrganizationAccessModeManagement, wantErr: "permission denied"},
		{name: "outsider", operator: outsider, accessMode: OrganizationAccessModeManagement, wantErr: "permission denied"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := UpdateOrganizationMemberModelDiscounts(tc.operator.Id, organization.Id, tc.accessMode, member.Id, discounts)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				require.Empty(t, storedMemberModelDiscounts(t, member.Id))
				return
			}
			require.NoError(t, err)
			require.Equal(t, expectedRaw, storedMemberModelDiscounts(t, member.Id))
			// 清空，隔离下一个用例。
			require.NoError(t, UpdateOrganizationMemberModelDiscounts(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, map[string]float64{}))
		})
	}
}

func TestOrganizationMemberModelDiscountsRequireActiveTargetMember(t *testing.T) {
	owner, _, _, organization := createMemberModelDiscountTestOrg(t)
	outsider := createServiceTestUser(t, "member-discount-target-outsider", common.RoleCommonUser)
	crossOrgOwner := createServiceTestUser(t, "member-discount-cross-owner", common.RoleCommonUser)
	crossOrganization, err := CreateOrganization(crossOrgOwner.Id, CreateOrganizationRequest{Name: "Member Discount Cross Org"})
	require.NoError(t, err)
	crossOrgMember := createServiceTestUser(t, "member-discount-cross-member", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: crossOrganization.Id, UserId: crossOrgMember.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	disabledMember := createServiceTestUser(t, "member-discount-disabled", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: disabledMember.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusDisabled}).Error)

	testCases := []struct {
		name         string
		targetUserId int
	}{
		{name: "non member", targetUserId: outsider.Id},
		{name: "cross organization member", targetUserId: crossOrgMember.Id},
		{name: "inactive member", targetUserId: disabledMember.Id},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := UpdateOrganizationMemberModelDiscounts(owner.Id, organization.Id, OrganizationAccessModeManagement, tc.targetUserId, map[string]float64{"gpt-discount-a": 0.5})
			require.ErrorContains(t, err, "active organization member required")
			require.Empty(t, storedMemberModelDiscounts(t, tc.targetUserId))
		})
	}

	discounts, err := GetOrganizationMemberModelDiscounts(owner.Id, organization.Id, OrganizationAccessModeManagement, outsider.Id)
	require.ErrorContains(t, err, "active organization member required")
	require.Nil(t, discounts)

	var auditCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationAuditLog{}).Where("organization_id = ? AND action_type = ?", organization.Id, organizationAuditActionMemberModelDiscountsUpdate).Count(&auditCount).Error)
	require.Zero(t, auditCount)
}

func TestOrganizationMemberModelDiscountsProtectedTargets(t *testing.T) {
	owner, admin, _, organization := createMemberModelDiscountTestOrg(t)

	testCases := []struct {
		name         string
		operatorId   int
		targetUserId int
	}{
		{name: "owner target", operatorId: owner.Id, targetUserId: owner.Id},
		{name: "peer admin target", operatorId: admin.Id, targetUserId: admin.Id},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := UpdateOrganizationMemberModelDiscounts(tc.operatorId, organization.Id, OrganizationAccessModeManagement, tc.targetUserId, map[string]float64{"gpt-discount-a": 0.5})
			require.ErrorContains(t, err, "organization member operation forbidden")
			require.Empty(t, storedMemberModelDiscounts(t, tc.targetUserId))
		})
	}
}

func TestUpdateOrganizationMemberModelDiscountsWritesAndAudits(t *testing.T) {
	owner, admin, member, organization := createMemberModelDiscountTestOrg(t)
	var memberRow model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).First(&memberRow).Error)

	// 等于全价的条目与"没有条目"等价，规范化时被丢弃。
	require.NoError(t, UpdateOrganizationMemberModelDiscounts(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, map[string]float64{"gpt-discount-a": 0.5, "gpt-discount-b": 1}))

	expected := map[string]float64{"gpt-discount-a": 0.5}
	require.Equal(t, model.MarshalUserModelDiscounts(expected), storedMemberModelDiscounts(t, member.Id))

	discounts, err := GetOrganizationMemberModelDiscounts(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id)
	require.NoError(t, err)
	require.Equal(t, expected, discounts)

	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND target_id = ? AND action_type = ?", organization.Id, memberRow.Id, organizationAuditActionMemberModelDiscountsUpdate).First(&audit).Error)
	require.Equal(t, "member", audit.TargetType)
	require.Equal(t, member.Username, audit.TargetName)
	require.Equal(t, owner.Id, audit.OperatorUserId)
	require.Equal(t, model.OrganizationRoleOwner, audit.OperatorRole)
	require.Contains(t, audit.Reason, "personal wallet")
	// 之前没有折扣表：ParseUserModelDiscounts("") 返回 nil map，落进审计就是 null
	//（与管理端用户折扣审计同一口径）。
	require.Equal(t, "null", audit.BeforeData)
	require.Equal(t, `{"gpt-discount-a":0.5}`, audit.AfterData)

	// 管理员可以改同一张表；空表即清空，审计里留下新旧两张表。
	require.NoError(t, UpdateOrganizationMemberModelDiscounts(admin.Id, organization.Id, OrganizationAccessModeManagement, member.Id, map[string]float64{}))
	require.Empty(t, storedMemberModelDiscounts(t, member.Id))

	var cleared model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND target_id = ? AND action_type = ?", organization.Id, memberRow.Id, organizationAuditActionMemberModelDiscountsUpdate).Order("id desc").First(&cleared).Error)
	require.Equal(t, model.OrganizationRoleAdmin, cleared.OperatorRole)
	require.Equal(t, `{"gpt-discount-a":0.5}`, cleared.BeforeData)
	require.Equal(t, "{}", cleared.AfterData)

	discounts, err = GetOrganizationMemberModelDiscounts(admin.Id, organization.Id, OrganizationAccessModeManagement, member.Id)
	require.NoError(t, err)
	require.Empty(t, discounts)
}

func TestUpdateOrganizationMemberModelDiscountsAcceptsNullStoredTable(t *testing.T) {
	owner, _, member, organization := createMemberModelDiscountTestOrg(t)
	// 早于该列的行是 NULL，不是空串：读写两条路径都得接得住。
	require.NoError(t, model.DB.Exec("UPDATE users SET model_discounts = NULL WHERE id = ?", member.Id).Error)

	discounts, err := GetOrganizationMemberModelDiscounts(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id)
	require.NoError(t, err)
	require.Empty(t, discounts)

	expected := map[string]float64{"gpt-discount-a": 0.5}
	require.NoError(t, UpdateOrganizationMemberModelDiscounts(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, expected))
	require.Equal(t, model.MarshalUserModelDiscounts(expected), storedMemberModelDiscounts(t, member.Id))
}

func TestGetOrganizationMemberModelDiscountsPermission(t *testing.T) {
	owner, admin, member, organization := createMemberModelDiscountTestOrg(t)
	outsider := createServiceTestUser(t, "member-discount-read-outsider", common.RoleCommonUser)
	expected := map[string]float64{"gpt-discount-a": 0.5}
	require.NoError(t, UpdateOrganizationMemberModelDiscounts(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, expected))

	allowed := []struct {
		name       string
		operator   model.User
		accessMode string
	}{
		{name: "owner", operator: owner, accessMode: OrganizationAccessModeWorkspace},
		{name: "admin", operator: admin, accessMode: OrganizationAccessModeManagement},
	}
	for _, tc := range allowed {
		t.Run(tc.name, func(t *testing.T) {
			discounts, err := GetOrganizationMemberModelDiscounts(tc.operator.Id, organization.Id, tc.accessMode, member.Id)
			require.NoError(t, err)
			require.Equal(t, expected, discounts)
		})
	}

	// 读走 workspace：普通成员能进组织但没有 view_members_full（permission denied），
	// 非成员连组织都进不去（策略层 organization access denied）。
	denied := []struct {
		name       string
		operator   model.User
		accessMode string
		wantErr    string
	}{
		{name: "member", operator: member, accessMode: OrganizationAccessModeWorkspace, wantErr: "permission denied"},
		{name: "outsider", operator: outsider, accessMode: OrganizationAccessModeWorkspace, wantErr: "organization access denied"},
	}
	for _, tc := range denied {
		t.Run(tc.name, func(t *testing.T) {
			discounts, err := GetOrganizationMemberModelDiscounts(tc.operator.Id, organization.Id, tc.accessMode, member.Id)
			require.ErrorContains(t, err, tc.wantErr)
			require.Nil(t, discounts)
		})
	}
}
