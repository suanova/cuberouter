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
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupControllerOrganizationTestDB 起一套带组织表的独立 SQLite 库。解散、退出成员
// 这些控制器路径会一路走到幂等记录、审计与令牌阻塞表，表少一个就会在断言之前失败。
func setupControllerOrganizationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	gin.SetMode(gin.TestMode)

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
		&model.Token{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.OrganizationInvite{},
		&model.OrganizationAuditLog{},
		&model.OrganizationDisableRecord{},
		&model.OrganizationTokenSystemBlocker{},
		&model.OrganizationIdempotencyRecord{},
		&model.OrganizationBillingSession{},
		&model.Task{},
		&model.Midjourney{},
	))
	return db
}

// TestRemoveOrganizationMemberRejectsMalformedJSON 覆盖「删除成员」带请求体的删除：
// body 解析失败必须是 400 且带解析错误，不能因为 body 坏掉就去执行删除。
func TestRemoveOrganizationMemberRejectsMalformedJSON(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "userId", Value: "2"}}
	c.Set("id", 1)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/organizations/1/members/2", strings.NewReader(`{`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Idempotency-Key", "malformed-member-remove")

	RemoveOrganizationMember(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "unexpected EOF")
}

// TestExitOrganizationRejectsInvalidTransferTarget 覆盖退出组织时给出非法承接人：
// 必须 400 且成员关系原样保留。这里最怕的是先改状态再校验承接人——那会让用户
// 直接丢掉组织身份，而组织那边也没有拿到他名下的令牌。
func TestExitOrganizationRejectsInvalidTransferTarget(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	owner := model.User{Username: "exit-owner", AffCode: "exit-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	member := model.User{Username: "exit-member", AffCode: "exit-member", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&owner).Error)
	require.NoError(t, model.DB.Create(&member).Error)
	organization := model.Organization{Name: "Exit Org", Slug: "exit-org", Status: model.OrganizationStatusActive, OwnerUserId: owner.Id, CreatedBy: owner.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: owner.Id, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(organization.Id)}}
	c.Set("id", member.Id)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/organizations/"+strconv.Itoa(organization.Id)+"/members/me?transfer_to_user_id=invalid", nil)
	c.Request.Header.Set("Idempotency-Key", "invalid-exit-transfer")

	ExitOrganization(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "invalid transfer_to_user_id")
	var stored model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).First(&stored).Error)
	require.Equal(t, model.OrganizationMemberStatusActive, stored.Status)
}

// TestDissolveOrganizationUsesBodyConfirmationAndReasonInAudit 覆盖解散的两条契约：
// 二次确认来自请求体里的组织名，以及原因必须落到审计日志——解散不可逆，审计里没有
// 原因就没人说得清这次解散是谁、为什么发起的。
func TestDissolveOrganizationUsesBodyConfirmationAndReasonInAudit(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	owner := model.User{Username: "dissolve-query-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&owner).Error)
	organization := model.Organization{Name: "Dissolve Query Org", Slug: "dissolve-query-org", Status: model.OrganizationStatusActive, OwnerUserId: owner.Id, CreatedBy: owner.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	member := model.OrganizationMember{OrganizationId: organization.Id, UserId: owner.Id, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive}
	require.NoError(t, model.DB.Create(&member).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(organization.Id)}}
	c.Set("id", owner.Id)
	common.SetContextKey(c, constant.ContextKeyOrganizationAccessMode, service.OrganizationAccessModeManagement)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/organizations/"+strconv.Itoa(organization.Id), strings.NewReader(`{"confirm_name":"Dissolve Query Org","reason":"closing now"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Idempotency-Key", "controller-dissolve")

	DissolveOrganization(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", organization.Id, "organization.dissolve").First(&audit).Error)
	require.Equal(t, "closing now", audit.Reason)
}
