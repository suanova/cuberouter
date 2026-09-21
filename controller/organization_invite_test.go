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
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupOrganizationInviteControllerTestDB 只建邀请链路读写的表。Gin 测试上下文
// 里的 c.Set("id", ...) 就是 UserAuth 平时写入的位置，这里手工补上。
func setupOrganizationInviteControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db

	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.RedisEnabled = previousRedisEnabled
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.OrganizationInvite{},
		&model.OrganizationAuditLog{},
	))
	return db
}

// TestCreateOrganizationInviteRequestBindsForceRotate 锁定邀请请求体的绑定契约：
// target_email 优先、force_rotate 透传。前端在「重发邀请」时会带上 force_rotate，
// 绑定漏了就会退化成「邀请已存在」的 409。
func TestCreateOrganizationInviteRequestBindsForceRotate(t *testing.T) {
	t.Parallel()

	var req createOrganizationInviteRequest
	err := common.Unmarshal([]byte(`{"target_email":"Target@Example.com","email":"legacy@example.com","role":"admin","force_rotate":true}`), &req)

	require.NoError(t, err)
	require.Equal(t, "Target@Example.com", req.TargetEmail)
	require.Equal(t, "Target@Example.com", req.inviteEmail())
	require.Equal(t, "admin", req.Role)
	require.True(t, req.ForceRotate)
}

// TestCreateOrganizationInviteRequestFallsBackToLegacyEmail 守住老字段 email：
// 老客户端只发 email，不认 target_email，丢掉这一支老版前端就再也发不出邀请。
func TestCreateOrganizationInviteRequestFallsBackToLegacyEmail(t *testing.T) {
	t.Parallel()

	req := createOrganizationInviteRequest{Email: "Legacy@Example.com"}

	require.Equal(t, "Legacy@Example.com", req.inviteEmail())
}

func TestAcceptOrganizationInviteRequiresAcceptedStatus(t *testing.T) {
	// gin.SetMode 写的是 gin 的包级 modeName，非原子写；gin.CreateTestContext 会经由
	// gin.New() 读同一个变量。放在 t.Parallel() 之前，它就在顺序阶段执行，
	// 放到之后就会与同样并行运行的用例并发读写同一个全局。
	gin.SetMode(gin.TestMode)
	t.Parallel()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/organization-invitations/token", bytes.NewBufferString(`{"status":"revoked"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	AcceptOrganizationInvite(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "invalid invitation status")
}

// TestAcceptOrganizationInviteAllowsEmptyBody 覆盖不带请求体的接受：接受是默认动作，
// 前端直接 PATCH 一个空 body 也必须成功，否则用户点「接受」会拿到 400。
func TestAcceptOrganizationInviteAllowsEmptyBody(t *testing.T) {
	setupOrganizationInviteControllerTestDB(t)
	admin := model.User{Username: "invite-controller-admin", Password: "password", DisplayName: "Invite Controller Admin", Status: common.UserStatusEnabled, AffCode: "invite-controller-admin", Email: "admin@example.com"}
	user := model.User{Username: "invite-controller-user", Password: "password", DisplayName: "Invite Controller User", Status: common.UserStatusEnabled, AffCode: "invite-controller-user", Email: "target@example.com"}
	require.NoError(t, model.DB.Create(&admin).Error)
	require.NoError(t, model.DB.Create(&user).Error)
	organization := model.Organization{Name: "Invite Controller Org", Status: model.OrganizationStatusActive, OwnerUserId: admin.Id, CreatedBy: admin.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: user.Email, Role: model.OrganizationRoleMember, Token: "empty-body-token", Status: model.OrganizationInviteStatusPending, InviterUserId: admin.Id}
	require.NoError(t, model.DB.Create(&invite).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "token", Value: invite.Token}}
	c.Set("id", user.Id)
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/organization-invitations/"+invite.Token, nil)

	AcceptOrganizationInvite(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var storedInvite model.OrganizationInvite
	require.NoError(t, model.DB.First(&storedInvite, invite.Id).Error)
	require.Equal(t, model.OrganizationInviteStatusAccepted, storedInvite.Status)
}

// TestGetOrganizationInviteReturnsUnavailableCodeForRevokedInvite 已撤销的邀请要给
// 稳定错误码而不是 404：邀请页据此区分「链接失效」与「链接不存在」，文案不同。
func TestGetOrganizationInviteReturnsUnavailableCodeForRevokedInvite(t *testing.T) {
	setupOrganizationInviteControllerTestDB(t)
	admin := model.User{Username: "invite-controller-code-admin", Password: "password", DisplayName: "Invite Controller Code Admin", Status: common.UserStatusEnabled, AffCode: "invite-controller-code-admin", Email: "admin@example.com"}
	require.NoError(t, model.DB.Create(&admin).Error)
	organization := model.Organization{Name: "Invite Controller Code Org", Status: model.OrganizationStatusActive, OwnerUserId: admin.Id, CreatedBy: admin.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "target@example.com", Role: model.OrganizationRoleMember, Token: "revoked-code-token", Status: model.OrganizationInviteStatusRevoked, InviterUserId: admin.Id}
	require.NoError(t, model.DB.Create(&invite).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "token", Value: invite.Token}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/organization-invitations/"+invite.Token, nil)

	GetOrganizationInvite(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), string(types.ErrorCodeOrganizationInviteUnavailable))
}
