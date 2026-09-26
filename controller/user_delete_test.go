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
package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createDeleteTestUser 建一个可用于硬删用例的账号。aff_code 有唯一索引，逐个给定
// 显式值，避免同一用例里的多个账号在空串上撞索引。
func createDeleteTestUser(t *testing.T, username string, role int) model.User {
	t.Helper()
	user := model.User{
		Username: username,
		AffCode:  username,
		Role:     role,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&user).Error)
	return user
}

// performDeleteUserRequest 以 operator 的身份直接调用 DELETE /api/user/:id/ 的处理函数，
// 与 manage user 的用例一致：绕过中间件，只摆好 handler 读取的上下文键。
func performDeleteUserRequest(t *testing.T, operator model.User, targetId int) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(targetId)}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/user/"+strconv.Itoa(targetId)+"/", nil)
	c.Set("id", operator.Id)
	c.Set("role", operator.Role)
	c.Set("username", operator.Username)
	DeleteUser(c)
	return recorder
}

func countOrganizationMembershipRows(t *testing.T, member model.OrganizationMember) int64 {
	t.Helper()
	var count int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("id = ?", member.Id).Count(&count).Error)
	return count
}

// TestDeleteUserPurgesOrganizationMembership 覆盖硬删的收口：删账号必须把它的组织成员
// 行一起带走。留下成员行意味着下一个复用该 user id 的账号会继承前一个账号的组织身份。
func TestDeleteUserPurgesOrganizationMembership(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	root := createDeleteTestUser(t, "del-root", common.RoleRootUser)
	owner := createDeleteTestUser(t, "del-owner", common.RoleCommonUser)
	target := createDeleteTestUser(t, "del-target", common.RoleCommonUser)
	organization := model.Organization{Name: "Delete Org", Slug: "delete-org", Status: model.OrganizationStatusActive, OwnerUserId: owner.Id, CreatedBy: owner.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	member := model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}
	require.NoError(t, model.DB.Create(&member).Error)

	recorder := performDeleteUserRequest(t, root, target.Id)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	assert.Zero(t, countOrganizationMembershipRows(t, member))
}

// TestDeleteUserRefusesOrganizationOwner 覆盖拒绝路径：账号还是组织 owner 时不能删。
// 拒绝必须是 409 + 稳定 code + 点名组织的文案——管理员得知道去哪儿处理，而不是
// 只看到一句"删除失败"。
func TestDeleteUserRefusesOrganizationOwner(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	root := createDeleteTestUser(t, "del-root-owner", common.RoleRootUser)
	target := createDeleteTestUser(t, "del-owner-target", common.RoleCommonUser)
	organization := model.Organization{Name: "Owner Org", Slug: "owner-org", Status: model.OrganizationStatusActive, OwnerUserId: target.Id, CreatedBy: target.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	member := model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive}
	require.NoError(t, model.DB.Create(&member).Error)

	recorder := performDeleteUserRequest(t, root, target.Id)
	require.Equal(t, http.StatusConflict, recorder.Code, recorder.Body.String())
	body := recorder.Body.String()
	assert.Contains(t, body, string(types.ErrorCodeOrganizationOperationBlocked))
	assert.Contains(t, body, organization.Name, "拒绝文案必须点名组织，否则管理员不知道去哪儿处理")

	assert.Equal(t, int64(1), countOrganizationMembershipRows(t, member))
	// 拒绝必须连账号一起留下：账号被删掉而成员行留下才是最坏的结果。
	var stored model.User
	require.NoError(t, model.DB.Unscoped().Where("id = ?", target.Id).First(&stored).Error)
	assert.False(t, stored.DeletedAt.Valid)
}

// TestSoftDeleteUserKeepsOrganizationMembership 软删不是账号销毁：users 行还在（id 不会被
// 复用、邮箱被永久占位），因此成员行必须原样留着。这条钉住"没有把收口顺手接到软删路径上"。
func TestSoftDeleteUserKeepsOrganizationMembership(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	owner := createDeleteTestUser(t, "softdel-owner", common.RoleCommonUser)
	target := createDeleteTestUser(t, "softdel-target", common.RoleCommonUser)
	organization := model.Organization{Name: "Soft Delete Org", Slug: "soft-delete-org", Status: model.OrganizationStatusActive, OwnerUserId: owner.Id, CreatedBy: owner.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	member := model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}
	require.NoError(t, model.DB.Create(&member).Error)

	body, err := common.Marshal(map[string]any{"id": target.Id, "action": "delete"})
	require.NoError(t, err)
	recorder := performManageUserRequest(t, string(body))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"success":true`)

	assert.Equal(t, int64(1), countOrganizationMembershipRows(t, member), "软删路径不得收口成员行")

	var softDeleted model.User
	require.NoError(t, model.DB.Unscoped().Where("id = ?", target.Id).First(&softDeleted).Error)
	assert.True(t, softDeleted.DeletedAt.Valid, "软删必须留下 users 行")
}

// TestAggregatedDeleteUserPurgesOrganizationMembership 覆盖第二个硬删入口：聚合 API 的
// 删除同样要收口组织成员行，否则两条入口的语义就分叉了。
func TestAggregatedDeleteUserPurgesOrganizationMembership(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	root := createDeleteTestUser(t, "aggdel-root", common.RoleRootUser)
	owner := createDeleteTestUser(t, "aggdel-owner", common.RoleCommonUser)
	target := createDeleteTestUser(t, "aggdel-target", common.RoleCommonUser)
	organization := model.Organization{Name: "Aggregated Delete Org", Slug: "aggregated-delete-org", Status: model.OrganizationStatusActive, OwnerUserId: owner.Id, CreatedBy: owner.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	member := model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}
	require.NoError(t, model.DB.Create(&member).Error)

	recorder := performAggregatedDeleteUserRequest(t, root, target.Id)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"status":"success"`)

	assert.Zero(t, countOrganizationMembershipRows(t, member))
}

// TestAggregatedDeleteUserRefusesOrganizationOwner 覆盖聚合入口的拒绝：该端点的响应体没有
// code 位，拒绝原因只能写在 message 里，因此 message 必须点名组织与处置动作。
func TestAggregatedDeleteUserRefusesOrganizationOwner(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	root := createDeleteTestUser(t, "aggdel-root-owner", common.RoleRootUser)
	target := createDeleteTestUser(t, "aggdel-owner-target", common.RoleCommonUser)
	organization := model.Organization{Name: "Aggregated Owner Org", Slug: "aggregated-owner-org", Status: model.OrganizationStatusActive, OwnerUserId: target.Id, CreatedBy: target.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	member := model.OrganizationMember{OrganizationId: organization.Id, UserId: target.Id, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive}
	require.NoError(t, model.DB.Create(&member).Error)

	recorder := performAggregatedDeleteUserRequest(t, root, target.Id)
	body := recorder.Body.String()
	require.Equal(t, http.StatusOK, recorder.Code, body)
	assert.Contains(t, body, `"status":"fail"`)
	assert.Contains(t, body, organization.Name, "拒绝原因必须点名组织")

	assert.Equal(t, int64(1), countOrganizationMembershipRows(t, member))
	var stored model.User
	require.NoError(t, model.DB.Unscoped().Where("id = ?", target.Id).First(&stored).Error)
	assert.False(t, stored.DeletedAt.Valid)
}

func performAggregatedDeleteUserRequest(t *testing.T, operator model.User, targetId int) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "user_id", Value: strconv.Itoa(targetId)}}
	c.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/users/%d/delete", targetId), nil)
	c.Set("id", operator.Id)
	c.Set("role", operator.Role)
	c.Set("username", operator.Username)
	AggregatedDeleteUser(c)
	return recorder
}
