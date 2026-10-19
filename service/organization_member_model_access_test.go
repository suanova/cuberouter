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

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createMemberModelAccessTestOrg 建一个 owner（组织创建者）+ admin + member 三档成员的组织。
// owner 的 owner 角色由 organization.owner_user_id 决定，不靠成员行里的 role 值。
func createMemberModelAccessTestOrg(t *testing.T) (model.User, model.User, model.User, *model.Organization) {
	t.Helper()
	setupServiceTestDB(t)
	owner := createServiceTestUser(t, "member-model-access-owner", common.RoleCommonUser)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "Member Model Access Org"})
	require.NoError(t, err)
	// universe 从 abilities 表读出来，而 setupServiceTestDB 不建这张表；缺表只会让
	// GORM 打一行错误日志、结果恒为空，所以这里先把表建出来。
	require.NoError(t, model.DB.AutoMigrate(&model.Ability{}))
	admin := createServiceTestUser(t, "member-model-access-admin", common.RoleCommonUser)
	member := createServiceTestUser(t, "member-model-access-member", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	return owner, admin, member, organization
}

// memberRowId 取成员行 id；审计的 target_id 是成员行而不是用户。
func memberRowId(t *testing.T, organizationId, userId int) int {
	t.Helper()
	var row model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organizationId, userId).First(&row).Error)
	return row.Id
}

// storedMemberModelAccess 读成员行 model_access 列的原值。
func storedMemberModelAccess(t *testing.T, organizationId, userId int) string {
	t.Helper()
	var stored model.OrganizationMember
	require.NoError(t, model.DB.Select("id", "model_access").Where("organization_id = ? AND user_id = ?", organizationId, userId).First(&stored).Error)
	return stored.ModelAccess
}

// requireOrganizationMemberModelAccessEmpty 断言组织里没有任何成员行带着白名单；
// 目标可能是别的组织的成员（本组织没有成员行），所以按行扫而不是按目标 id 查。
func requireOrganizationMemberModelAccessEmpty(t *testing.T, organizationId int) {
	t.Helper()
	var rows []model.OrganizationMember
	require.NoError(t, model.DB.Select("id", "model_access").Where("organization_id = ?", organizationId).Find(&rows).Error)
	for _, row := range rows {
		require.Empty(t, row.ModelAccess, "member row %d must stay unrestricted", row.Id)
	}
}

// seedMemberModelAccessGroupAbilities 让给定模型在 default 分组可见：组织组的 universe
// 正是从这个 abilities 表读出来的。
func seedMemberModelAccessGroupAbilities(t *testing.T, models ...string) {
	t.Helper()
	abilities := make([]model.Ability, 0, len(models))
	for _, modelName := range models {
		abilities = append(abilities, model.Ability{Group: DefaultOrganizationGroup, Model: modelName, ChannelId: 1, Enabled: true})
	}
	require.NoError(t, model.DB.Create(&abilities).Error)
}

// useMemberModelAccessMiniRedis 打开 Redis 并接到独立的 miniredis 实例上，让白名单缓存
// 真正走命中/失效路径；Redis 关闭时 Refresh 是 no-op，证明不了服务确实调用了它。
// 返回实例本身：组织级失效要断言「全部成员的键一起消失」，得直接看键集合。
func useMemberModelAccessMiniRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	previousRedisEnabled := common.RedisEnabled
	previousRDB := common.RDB
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	common.RedisEnabled = true
	common.RDB = client
	t.Cleanup(func() {
		_ = client.Close()
		common.RedisEnabled = previousRedisEnabled
		common.RDB = previousRDB
	})
	return server
}

func TestOrganizationMemberModelAccessCapabilityMapping(t *testing.T) {
	owner, admin, member, organization := createMemberModelAccessTestOrg(t)

	ownerActor, err := GetOrganizationActorContextForAccessMode(owner.Id, organization.Id, OrganizationAccessModeManagement, true)
	require.NoError(t, err)
	require.True(t, ownerActor.Capabilities.CanManageMemberModels)

	adminActor, err := GetOrganizationActorContextForAccessMode(admin.Id, organization.Id, OrganizationAccessModeManagement, true)
	require.NoError(t, err)
	require.True(t, adminActor.Capabilities.CanManageMemberModels)

	memberActor, err := GetOrganizationActorContextForAccessMode(member.Id, organization.Id, OrganizationAccessModeWorkspace, true)
	require.NoError(t, err)
	require.False(t, memberActor.Capabilities.CanManageMemberModels)
}

func TestOrganizationMemberModelAccessPermissionMatrix(t *testing.T) {
	owner, admin, member, organization := createMemberModelAccessTestOrg(t)
	outsider := createServiceTestUser(t, "member-model-access-outsider", common.RoleCommonUser)
	crossOrgOwner := createServiceTestUser(t, "member-model-access-cross-owner", common.RoleCommonUser)
	_, err := CreateOrganization(crossOrgOwner.Id, CreateOrganizationRequest{Name: "Member Model Access Cross Org"})
	require.NoError(t, err)
	models := []string{"gpt-access-a"}
	expectedRaw := model.MarshalMemberModelAccess(models)

	testCases := []struct {
		name     string
		operator model.User
		wantErr  string
	}{
		{name: "owner", operator: owner},
		{name: "admin", operator: admin},
		{name: "member", operator: member, wantErr: "permission denied"},
		{name: "outsider", operator: outsider, wantErr: "permission denied"},
		{name: "cross organization owner", operator: crossOrgOwner, wantErr: "permission denied"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			auditsBefore := countOrganizationAuditRows(t, organization.Id, organizationAuditActionMemberModelAccessUpdate)
			err := UpdateOrganizationMemberModelAccess(tc.operator.Id, organization.Id, OrganizationAccessModeManagement, member.Id, models)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				requireOrganizationMemberModelAccessEmpty(t, organization.Id)
				assert.Equal(t, auditsBefore, countOrganizationAuditRows(t, organization.Id, organizationAuditActionMemberModelAccessUpdate), "a rejected update must not write an audit row")
				return
			}
			require.NoError(t, err)
			require.Equal(t, expectedRaw, storedMemberModelAccess(t, organization.Id, member.Id))
			// 清空，隔离下一个用例。
			require.NoError(t, UpdateOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, nil))
		})
	}
}

func TestGetOrganizationMemberModelAccessPermission(t *testing.T) {
	owner, admin, member, organization := createMemberModelAccessTestOrg(t)
	outsider := createServiceTestUser(t, "member-model-access-read-outsider", common.RoleCommonUser)
	expected := []string{"gpt-access-a"}
	require.NoError(t, UpdateOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, expected))

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
			access, _, err := GetOrganizationMemberModelAccess(tc.operator.Id, organization.Id, tc.accessMode, member.Id)
			require.NoError(t, err)
			require.Equal(t, expected, access)
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
			access, universe, err := GetOrganizationMemberModelAccess(tc.operator.Id, organization.Id, tc.accessMode, member.Id)
			require.ErrorContains(t, err, tc.wantErr)
			require.Nil(t, access)
			require.Nil(t, universe)
		})
	}
}

func TestOrganizationMemberModelAccessRequireActiveTargetMember(t *testing.T) {
	owner, _, _, organization := createMemberModelAccessTestOrg(t)
	outsider := createServiceTestUser(t, "member-model-access-target-outsider", common.RoleCommonUser)
	crossOrgOwner := createServiceTestUser(t, "member-model-access-target-cross-owner", common.RoleCommonUser)
	crossOrganization, err := CreateOrganization(crossOrgOwner.Id, CreateOrganizationRequest{Name: "Member Model Access Target Cross Org"})
	require.NoError(t, err)
	crossOrgMember := createServiceTestUser(t, "member-model-access-target-cross-member", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: crossOrganization.Id, UserId: crossOrgMember.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	disabledMember := createServiceTestUser(t, "member-model-access-disabled", common.RoleCommonUser)
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
			err := UpdateOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, tc.targetUserId, []string{"gpt-access-a"})
			require.ErrorContains(t, err, "active organization member required")
			requireOrganizationMemberModelAccessEmpty(t, organization.Id)
		})
	}

	access, universe, err := GetOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, outsider.Id)
	require.ErrorContains(t, err, "active organization member required")
	require.Nil(t, access)
	require.Nil(t, universe)

	require.Zero(t, countOrganizationAuditRows(t, organization.Id, organizationAuditActionMemberModelAccessUpdate))
}

func TestOrganizationMemberModelAccessProtectedTargets(t *testing.T) {
	owner, admin, _, organization := createMemberModelAccessTestOrg(t)

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
			err := UpdateOrganizationMemberModelAccess(tc.operatorId, organization.Id, OrganizationAccessModeManagement, tc.targetUserId, []string{"gpt-access-a"})
			require.ErrorContains(t, err, "organization member operation forbidden")
			require.Empty(t, storedMemberModelAccess(t, organization.Id, tc.targetUserId))
		})
	}
}

func TestOrganizationMemberModelAccessRejectsInvalidModels(t *testing.T) {
	owner, _, member, organization := createMemberModelAccessTestOrg(t)

	testCases := []struct {
		name            string
		models          []string
		wantErrExact    string
		wantErrContains string
	}{
		{name: "blank entry", models: []string{"gpt-access-a", "   "}, wantErrExact: "member model access entry 1: model name must not be empty"},
		{name: "overlong name", models: []string{strings.Repeat("m", model.MaxMemberModelNameLen+1)}, wantErrContains: "model name too long"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := UpdateOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, tc.models)
			// 校验错误原样返回（不加前缀、不包裹），控制器按通用组织错误通道落成 400。
			if tc.wantErrExact != "" {
				require.EqualError(t, err, tc.wantErrExact)
			} else {
				require.ErrorContains(t, err, tc.wantErrContains)
			}
			require.Empty(t, storedMemberModelAccess(t, organization.Id, member.Id))
			require.Zero(t, countOrganizationAuditRows(t, organization.Id, organizationAuditActionMemberModelAccessUpdate))
		})
	}
}

func TestUpdateOrganizationMemberModelAccessWritesAndAudits(t *testing.T) {
	owner, admin, member, organization := createMemberModelAccessTestOrg(t)
	memberRow := memberRowId(t, organization.Id, member.Id)

	// 写前排序、去重、trim；这条列此前为空，审计里的 before 就是无限制。
	require.NoError(t, UpdateOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, []string{" gpt-access-b ", "gpt-access-a", "gpt-access-a"}))

	require.Equal(t, `["gpt-access-a","gpt-access-b"]`, storedMemberModelAccess(t, organization.Id, member.Id))

	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND target_id = ? AND action_type = ?", organization.Id, memberRow, organizationAuditActionMemberModelAccessUpdate).First(&audit).Error)
	require.Equal(t, "member", audit.TargetType)
	require.Equal(t, member.Username, audit.TargetName)
	require.Equal(t, owner.Id, audit.OperatorUserId)
	require.Equal(t, model.OrganizationRoleOwner, audit.OperatorRole)
	require.Contains(t, audit.Reason, "lifts the restriction")
	// 之前没有白名单：ParseMemberModelAccess(NULL/空串) 返回 nil 切片，落进审计就是 null
	//（与管理端折扣审计同一口径）。
	require.Equal(t, "null", audit.BeforeData)
	require.Equal(t, `["gpt-access-a","gpt-access-b"]`, audit.AfterData)

	// 管理员可以改同一列；空数组即解除限制，审计里留下新旧两份白名单。
	require.NoError(t, UpdateOrganizationMemberModelAccess(admin.Id, organization.Id, OrganizationAccessModeManagement, member.Id, []string{}))
	require.Empty(t, storedMemberModelAccess(t, organization.Id, member.Id))

	var cleared model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND target_id = ? AND action_type = ?", organization.Id, memberRow, organizationAuditActionMemberModelAccessUpdate).Order("id desc").First(&cleared).Error)
	require.Equal(t, model.OrganizationRoleAdmin, cleared.OperatorRole)
	require.Equal(t, `["gpt-access-a","gpt-access-b"]`, cleared.BeforeData)
	require.Equal(t, "null", cleared.AfterData)

	access, _, err := GetOrganizationMemberModelAccess(admin.Id, organization.Id, OrganizationAccessModeManagement, member.Id)
	require.NoError(t, err)
	require.Empty(t, access)
}

func TestOrganizationMemberModelAccessAcceptsNullStoredColumn(t *testing.T) {
	owner, _, member, organization := createMemberModelAccessTestOrg(t)
	// 早于该列的行是 NULL，不是空串：读（before 快照）与写两条路径都得接得住。
	require.NoError(t, model.DB.Exec("UPDATE organization_members SET model_access = NULL WHERE organization_id = ? AND user_id = ?", organization.Id, member.Id).Error)

	access, _, err := GetOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id)
	require.NoError(t, err)
	require.Empty(t, access)

	require.NoError(t, UpdateOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, []string{"gpt-access-a"}))
	require.Equal(t, `["gpt-access-a"]`, storedMemberModelAccess(t, organization.Id, member.Id))

	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND target_id = ? AND action_type = ?", organization.Id, memberRowId(t, organization.Id, member.Id), organizationAuditActionMemberModelAccessUpdate).First(&audit).Error)
	require.Equal(t, "null", audit.BeforeData)
}

func TestGetOrganizationMemberModelAccessReturnsAccessAndUniverse(t *testing.T) {
	owner, _, member, organization := createMemberModelAccessTestOrg(t)
	seedMemberModelAccessGroupAbilities(t, "gpt-access-a", "gpt-access-b", "gpt-access-c")

	// 未配置时 access 为空（= 不限制），universe 是组织组当前可见的全部模型。
	access, universe, err := GetOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id)
	require.NoError(t, err)
	require.Empty(t, access)
	require.ElementsMatch(t, []string{"gpt-access-a", "gpt-access-b", "gpt-access-c"}, universe)

	require.NoError(t, UpdateOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, []string{"gpt-access-b"}))
	access, universe, err = GetOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id)
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-access-b"}, access)
	require.ElementsMatch(t, []string{"gpt-access-a", "gpt-access-b", "gpt-access-c"}, universe)
	// 白名单条目来自 universe：前端据此渲染勾选项，二者不能各自为政。
	require.Subset(t, universe, access)
}

func TestUpdateOrganizationMemberModelAccessRefreshesCachedAllowList(t *testing.T) {
	owner, _, member, organization := createMemberModelAccessTestOrg(t)
	useMemberModelAccessMiniRedis(t)

	// 先让缓存成型：不限制 → {"u":true}。
	require.Nil(t, model.GetUserModelAllowList(member.Id))

	require.NoError(t, UpdateOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, []string{"gpt-access-a"}))
	// 缓存若没被失效，这里读到的还是旧的「不限制」。
	require.Equal(t, []string{"gpt-access-a"}, model.GetUserModelAllowList(member.Id))

	require.NoError(t, UpdateOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, nil))
	require.Nil(t, model.GetUserModelAllowList(member.Id))
}

// TestExitOrganizationInvalidatesMemberModelAccessCache 受限成员退出组织：成员行不再是
// active，限制必须立即解除——缓存若滞留旧的受限集，用户就被一个已经退出的组织锁着。
func TestExitOrganizationInvalidatesMemberModelAccessCache(t *testing.T) {
	owner, _, member, organization := createMemberModelAccessTestOrg(t)
	useMemberModelAccessMiniRedis(t)

	require.NoError(t, UpdateOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, []string{"gpt-access-a"}))
	require.Equal(t, []string{"gpt-access-a"}, model.GetUserModelAllowList(member.Id))

	require.NoError(t, ExitOrganization(member.Id, organization.Id, OrganizationAccessModeWorkspace, ExitMemberRequest{Reason: "leave", IdempotencyKey: testOrganizationIdempotencyKey(t, "exit-member")}))

	require.Nil(t, model.GetUserModelAllowList(member.Id))
}

// TestRemoveOrganizationMemberInvalidatesMemberModelAccessCache 受限成员被移除：同退出，
// 限制随成员关系一起消失。
func TestRemoveOrganizationMemberInvalidatesMemberModelAccessCache(t *testing.T) {
	owner, _, member, organization := createMemberModelAccessTestOrg(t)
	useMemberModelAccessMiniRedis(t)

	require.NoError(t, UpdateOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, []string{"gpt-access-a"}))
	require.Equal(t, []string{"gpt-access-a"}, model.GetUserModelAllowList(member.Id))

	require.NoError(t, RemoveOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, RemoveMemberRequest{Reason: "cleanup", IdempotencyKey: testOrganizationIdempotencyKey(t, "remove-member")}))

	require.Nil(t, model.GetUserModelAllowList(member.Id))
}

// TestUpdateOrganizationMemberStatusInvalidatesMemberModelAccessCache status 变更两个方向
// 都要跟上：禁用后限制解除，重新启用后限制回来（不能停在禁用期间缓存的「不限制」）。
func TestUpdateOrganizationMemberStatusInvalidatesMemberModelAccessCache(t *testing.T) {
	owner, _, member, organization := createMemberModelAccessTestOrg(t)
	useMemberModelAccessMiniRedis(t)

	require.NoError(t, UpdateOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, []string{"gpt-access-a"}))
	require.Equal(t, []string{"gpt-access-a"}, model.GetUserModelAllowList(member.Id))

	require.NoError(t, UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusDisabled, Reason: "inactive", IdempotencyKey: testOrganizationIdempotencyKey(t, "disable-member")}))
	require.Nil(t, model.GetUserModelAllowList(member.Id))

	require.NoError(t, UpdateOrganizationMember(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, UpdateMemberRequest{Status: model.OrganizationMemberStatusActive, Reason: "reinstated"}))
	require.Equal(t, []string{"gpt-access-a"}, model.GetUserModelAllowList(member.Id))
}

// TestAddOrganizationMemberInvalidatesMemberModelAccessCache 重新加入（Add 的既有行分支会
// 复活带白名单的旧成员行）：限制必须立刻生效，而不是停在退出期间缓存的「不限制」。
func TestAddOrganizationMemberInvalidatesMemberModelAccessCache(t *testing.T) {
	_, _, _, organization := createMemberModelAccessTestOrg(t)
	platformAdmin := createServiceTestUser(t, "member-model-access-platform-admin", common.RoleAdminUser)
	target := createServiceTestUser(t, "member-model-access-readded", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusExited, ModelAccess: model.MarshalMemberModelAccess([]string{"gpt-access-a"})}).Error)
	useMemberModelAccessMiniRedis(t)
	require.Nil(t, model.GetUserModelAllowList(target.Id))

	_, err := AddOrganizationMember(platformAdmin.Id, organization.Id, OrganizationAccessModeAdmin, AddMemberRequest{UserId: target.Id, Role: model.OrganizationRoleMember, Reason: "re-add"})
	require.NoError(t, err)

	require.Equal(t, []string{"gpt-access-a"}, model.GetUserModelAllowList(target.Id))
}

// TestAcceptInviteInvalidatesMemberModelAccessCache 邀请接受走 ReactivateOrganizationMember：
// 新加入（或复活）的成员行必须立刻进入有效集计算，旧交集不能留在缓存里。
func TestAcceptInviteInvalidatesMemberModelAccessCache(t *testing.T) {
	owner, _, member, organization := createMemberModelAccessTestOrg(t)
	// 该成员在另一个组织里受限：两个组织都在 active 才谈得上交集。
	otherOwner := createServiceTestUser(t, "member-model-access-invite-other-owner", common.RoleCommonUser)
	otherOrganization, err := CreateOrganization(otherOwner.Id, CreateOrganizationRequest{Name: "Member Model Access Invite Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: otherOrganization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive, ModelAccess: model.MarshalMemberModelAccess([]string{"gpt-access-b", "gpt-access-c"})}).Error)
	// 本组织的成员行停在 exited，带着上一轮的白名单：接受邀请会把它复活。
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).
		Updates(map[string]any{"status": model.OrganizationMemberStatusExited, "model_access": model.MarshalMemberModelAccess([]string{"gpt-access-a", "gpt-access-b"})}).Error)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", member.Id).Update("email", "member-model-access-invite@example.com").Error)
	useMemberModelAccessMiniRedis(t)
	withInviteEmailSender(t, func(subject string, receiver string, content string) error { return nil })

	require.Equal(t, []string{"gpt-access-b", "gpt-access-c"}, model.GetUserModelAllowList(member.Id))

	invite, err := CreateEmailInvite(owner.Id, organization.Id, OrganizationAccessModeWorkspace, CreateInviteRequest{Email: "member-model-access-invite@example.com", Role: model.OrganizationRoleMember})
	require.NoError(t, err)
	require.NoError(t, AcceptInvite(invite.Token, member.Id))

	// 复活的成员行（{a,b}）与另一组织的行（{b,c}）取交集。
	require.Equal(t, []string{"gpt-access-b"}, model.GetUserModelAllowList(member.Id))
}

// allowListCacheKeys 只返回缓存载荷键，排除失效时留下的版本栅栏键：组织级断言关心的是
// 各成员的清单缓存是否一起消失，栅栏键本来就要保留下来拦住并发回填。
func allowListCacheKeys(server *miniredis.Miniredis) []string {
	keys := make([]string, 0, len(server.Keys()))
	for _, key := range server.Keys() {
		if strings.HasPrefix(key, "user:model_allowlist:") && !strings.HasPrefix(key, "user:model_allowlist:ver:") {
			keys = append(keys, key)
		}
	}
	return keys
}

// TestOrganizationStatusChangeInvalidatesMemberModelAccessCaches 组织级状态变更要遍历该组织
// 全部 active 成员逐个失效：禁用让所有人的限制解除，启用让限制回来。
func TestOrganizationStatusChangeInvalidatesMemberModelAccessCaches(t *testing.T) {
	owner, _, member, organization := createMemberModelAccessTestOrg(t)
	secondMember := createServiceTestUser(t, "member-model-access-second", common.RoleCommonUser)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: secondMember.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	server := useMemberModelAccessMiniRedis(t)

	require.NoError(t, UpdateOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, []string{"gpt-access-a"}))
	require.NoError(t, UpdateOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, secondMember.Id, []string{"gpt-access-b"}))
	// 两名成员各回填一个缓存键：组织级失效的断言看的就是它们一起消失。
	require.Equal(t, []string{"gpt-access-a"}, model.GetUserModelAllowList(member.Id))
	require.Equal(t, []string{"gpt-access-b"}, model.GetUserModelAllowList(secondMember.Id))
	require.Len(t, allowListCacheKeys(server), 2)

	require.NoError(t, DisableOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "maintenance"))
	require.Empty(t, allowListCacheKeys(server))
	require.Nil(t, model.GetUserModelAllowList(member.Id))
	require.Nil(t, model.GetUserModelAllowList(secondMember.Id))

	require.NoError(t, EnableOrganization(owner.Id, organization.Id, OrganizationAccessModeManagement, organization.Slug, "back"))
	require.Empty(t, allowListCacheKeys(server))
	require.Equal(t, []string{"gpt-access-a"}, model.GetUserModelAllowList(member.Id))
	require.Equal(t, []string{"gpt-access-b"}, model.GetUserModelAllowList(secondMember.Id))
}

// TestDissolveOrganizationInvalidatesMemberModelAccessCaches 组织解散后没有人能再解除限制
// （组织与管理员都不在了），缓存必须立刻失效，让重算结果（解除）生效。
func TestDissolveOrganizationInvalidatesMemberModelAccessCaches(t *testing.T) {
	owner, _, member, organization := createMemberModelAccessTestOrg(t)
	server := useMemberModelAccessMiniRedis(t)

	require.NoError(t, UpdateOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, []string{"gpt-access-a"}))
	require.Equal(t, []string{"gpt-access-a"}, model.GetUserModelAllowList(member.Id))
	require.Len(t, allowListCacheKeys(server), 1)

	require.NoError(t, dissolveOrganizationForTest(t, owner.Id, organization.Id, "closed"))

	require.Empty(t, allowListCacheKeys(server))
	require.Nil(t, model.GetUserModelAllowList(member.Id))
}

// TestUpdateOrganizationMemberModelAccessExternalDatabase 在真实 MySQL / PostgreSQL 上盖一遍
// 写路径：成员行 FOR UPDATE 取 before、写白名单列、审计落库、空列表解除、NULL 列的 before
// 读取。SQLite 观测不到行锁，方言差异（含 NULL 扫描）也只有真库才验得出来。
// 需要 TEST_MYSQL_DSN 或 TEST_POSTGRES_DSN 且 ORGANIZATION_TEST_ALLOW_DESTRUCTIVE=1。
func TestUpdateOrganizationMemberModelAccessExternalDatabase(t *testing.T) {
	db, _ := setupOrganizationExternalConcurrencyDB(t)
	// 组织策略加载要读 organization_disable_records：PostgreSQL 下缺表会让事务中止
	//（SQLSTATE 25P02），所以这张表必须自建，不能靠别的用例遗留。
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.OrganizationDisableRecord{},
		&model.OrganizationAuditLog{},
		&model.Ability{},
	))

	suffix := strings.ReplaceAll(common.GetUUID(), "-", "")[:8]
	owner := model.User{Username: "external-model-access-owner-" + suffix, Password: "password", DisplayName: "external-model-access-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "ema-o-" + suffix}
	require.NoError(t, db.Create(&owner).Error)
	member := model.User{Username: "external-model-access-member-" + suffix, Password: "password", DisplayName: "external-model-access-member", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "ema-m-" + suffix}
	require.NoError(t, db.Create(&member).Error)
	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "External Model Access " + suffix})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	t.Cleanup(func() {
		require.NoError(t, db.Where("organization_id = ?", organization.Id).Delete(&model.OrganizationAuditLog{}).Error)
		require.NoError(t, db.Where("organization_id = ?", organization.Id).Delete(&model.OrganizationMember{}).Error)
		require.NoError(t, db.Where("id = ?", organization.Id).Delete(&model.Organization{}).Error)
		require.NoError(t, db.Unscoped().Delete(&model.User{}, []int{owner.Id, member.Id}).Error)
	})

	require.NoError(t, UpdateOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, []string{"gpt-access-b", "gpt-access-a"}))

	var stored model.OrganizationMember
	require.NoError(t, db.Select("id", "model_access").Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).First(&stored).Error)
	require.Equal(t, `["gpt-access-a","gpt-access-b"]`, stored.ModelAccess)

	var audit model.OrganizationAuditLog
	require.NoError(t, db.Where("organization_id = ? AND target_id = ? AND action_type = ?", organization.Id, stored.Id, organizationAuditActionMemberModelAccessUpdate).First(&audit).Error)
	require.Equal(t, "member", audit.TargetType)
	require.Equal(t, "null", audit.BeforeData)
	require.Equal(t, `["gpt-access-a","gpt-access-b"]`, audit.AfterData)

	// 空列表解除限制：列变回空串，审计留下旧白名单。
	require.NoError(t, UpdateOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, []string{}))
	require.NoError(t, db.Select("id", "model_access").Where("id = ?", stored.Id).First(&stored).Error)
	require.Empty(t, stored.ModelAccess)

	// 早于该列的行是 NULL：真实库上走一遍 NullString 的 before 读取与整行替换。
	require.NoError(t, db.Exec("UPDATE organization_members SET model_access = NULL WHERE id = ?", stored.Id).Error)
	require.NoError(t, UpdateOrganizationMemberModelAccess(owner.Id, organization.Id, OrganizationAccessModeManagement, member.Id, []string{"gpt-access-c"}))
	require.NoError(t, db.Select("id", "model_access").Where("id = ?", stored.Id).First(&stored).Error)
	require.Equal(t, `["gpt-access-c"]`, stored.ModelAccess)

	var restored model.OrganizationAuditLog
	require.NoError(t, db.Where("organization_id = ? AND target_id = ? AND action_type = ?", organization.Id, stored.Id, organizationAuditActionMemberModelAccessUpdate).Order("id desc").First(&restored).Error)
	require.Equal(t, "null", restored.BeforeData)
	require.Equal(t, `["gpt-access-c"]`, restored.AfterData)
}
