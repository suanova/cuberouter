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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// organizationMemberQuotaTestResponse 用前端消费的字段名断言响应契约：
// limit / used / organization_quota / allocated / over_allocated。
type organizationMemberQuotaTestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Limit             int  `json:"limit"`
		Used              int  `json:"used"`
		OrganizationQuota int  `json:"organization_quota"`
		Allocated         int  `json:"allocated"`
		OverAllocated     bool `json:"over_allocated"`
	} `json:"data"`
}

// createOrganizationMemberQuotaTestOrg 建一个 active 组织，owner 与 admin 各一条成员行。
func createOrganizationMemberQuotaTestOrg(t *testing.T, organizationQuota int) (model.User, model.User, model.Organization) {
	t.Helper()
	owner := model.User{Username: "member-quota-controller-owner", AffCode: "member-quota-controller-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	admin := model.User{Username: "member-quota-controller-admin", AffCode: "member-quota-controller-admin", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&owner).Error)
	require.NoError(t, model.DB.Create(&admin).Error)
	organization := model.Organization{
		Name:        "Member Quota Controller Org",
		Slug:        "member-quota-controller-org",
		Status:      model.OrganizationStatusActive,
		OwnerUserId: owner.Id,
		CreatedBy:   owner.Id,
		Quota:       organizationQuota,
	}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: owner.Id, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleAdmin, Status: model.OrganizationMemberStatusActive}).Error)
	return owner, admin, organization
}

// createOrganizationMemberQuotaTestMember 建一个带配额与已用值的普通成员。
func createOrganizationMemberQuotaTestMember(t *testing.T, organizationId int, username string, quotaLimit, usedQuota int) model.User {
	t.Helper()
	user := model.User{Username: username, AffCode: username, Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: organizationId,
		UserId:         user.Id,
		Role:           model.OrganizationRoleMember,
		Status:         model.OrganizationMemberStatusActive,
		QuotaLimit:     quotaLimit,
		UsedQuota:      usedQuota,
	}).Error)
	return user
}

// performOrganizationMemberQuotaRequest 直调 handler：补齐路由层提供的 :id/:userId
// 参数与鉴权中间件写入的操作者身份、访问模式。
func performOrganizationMemberQuotaRequest(t *testing.T, method string, operatorUserId, organizationId, targetUserId int, accessMode, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{
		{Key: "id", Value: strconv.Itoa(organizationId)},
		{Key: "userId", Value: strconv.Itoa(targetUserId)},
	}
	c.Set("id", operatorUserId)
	common.SetContextKey(c, constant.ContextKeyOrganizationAccessMode, accessMode)
	c.Request = httptest.NewRequest(method, "/api/organizations/"+strconv.Itoa(organizationId)+"/members/"+strconv.Itoa(targetUserId)+"/quota", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	switch method {
	case http.MethodGet:
		GetOrganizationMemberQuota(c)
	case http.MethodPut:
		UpdateOrganizationMemberQuota(c)
	}
	return recorder
}

func decodeOrganizationMemberQuotaResponse(t *testing.T, recorder *httptest.ResponseRecorder) organizationMemberQuotaTestResponse {
	t.Helper()
	var response organizationMemberQuotaTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

// TestOrganizationMemberQuotaGetReturnsBudgetPayload 读接口按前端契约返回五个字段：
// 成员上限、已用、组织总额度、当前已分配、是否超额分配。
func TestOrganizationMemberQuotaGetReturnsBudgetPayload(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	owner, _, organization := createOrganizationMemberQuotaTestOrg(t, 300)
	target := createOrganizationMemberQuotaTestMember(t, organization.Id, "member-quota-get-target", 80, 30)
	createOrganizationMemberQuotaTestMember(t, organization.Id, "member-quota-get-other", 100, 0)

	recorder := performOrganizationMemberQuotaRequest(t, http.MethodGet, owner.Id, organization.Id, target.Id, service.OrganizationAccessModeManagement, "")

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	response := decodeOrganizationMemberQuotaResponse(t, recorder)
	assert.True(t, response.Success)
	assert.Equal(t, 80, response.Data.Limit)
	assert.Equal(t, 30, response.Data.Used)
	assert.Equal(t, 300, response.Data.OrganizationQuota)
	assert.Equal(t, 180, response.Data.Allocated)
	assert.False(t, response.Data.OverAllocated)
}

// TestOrganizationMemberQuotaGetRequiresFullMemberVisibility 普通成员与外部用户都读不到
// 成员配额：前者缺 view_members_full，后者根本没有组织身份。
func TestOrganizationMemberQuotaGetRequiresFullMemberVisibility(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	_, _, organization := createOrganizationMemberQuotaTestOrg(t, 300)
	member := createOrganizationMemberQuotaTestMember(t, organization.Id, "member-quota-get-restricted", 0, 0)
	outsider := model.User{Username: "member-quota-get-outsider", AffCode: "member-quota-get-outsider", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&outsider).Error)

	cases := []struct {
		name           string
		operatorUserId int
	}{
		{name: "organization member", operatorUserId: member.Id},
		{name: "non member", operatorUserId: outsider.Id},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := performOrganizationMemberQuotaRequest(t, http.MethodGet, tc.operatorUserId, organization.Id, member.Id, service.OrganizationAccessModeWorkspace, "")

			require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
			response := decodeOrganizationMemberQuotaResponse(t, recorder)
			assert.False(t, response.Success)
			assert.Contains(t, response.Message, "denied")
		})
	}
}

// TestOrganizationMemberQuotaUpdateSetsLimit PUT 的 limit 经 handler 落到服务层：
// 成员行更新、已用值保留、审计落库。
func TestOrganizationMemberQuotaUpdateSetsLimit(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	owner, _, organization := createOrganizationMemberQuotaTestOrg(t, 300)
	target := createOrganizationMemberQuotaTestMember(t, organization.Id, "member-quota-put-target", 0, 25)

	recorder := performOrganizationMemberQuotaRequest(t, http.MethodPut, owner.Id, organization.Id, target.Id, service.OrganizationAccessModeManagement, `{"limit":100}`)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	response := decodeOrganizationMemberQuotaResponse(t, recorder)
	assert.True(t, response.Success)

	var stored model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, target.Id).First(&stored).Error)
	assert.Equal(t, 100, stored.QuotaLimit)
	assert.Equal(t, 25, stored.UsedQuota)

	var audit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND target_id = ? AND action_type = ?", organization.Id, stored.Id, "organization.member.quota.update").First(&audit).Error)
	assert.Equal(t, owner.Id, audit.OperatorUserId)
}

// TestOrganizationMemberQuotaUpdateRejectsInvalidLimit 预算超额与负数上限都必须 400
// 且不落库；这两条拒绝由服务层给出，handler 只负责映射为 success:false。
func TestOrganizationMemberQuotaUpdateRejectsInvalidLimit(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	owner, _, organization := createOrganizationMemberQuotaTestOrg(t, 50)
	target := createOrganizationMemberQuotaTestMember(t, organization.Id, "member-quota-put-invalid", 0, 0)

	cases := []struct {
		name    string
		body    string
		message string
	}{
		{name: "budget rejection", body: `{"limit":60}`, message: "budget exceeded"},
		{name: "negative limit", body: `{"limit":-1}`, message: "must not be negative"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := performOrganizationMemberQuotaRequest(t, http.MethodPut, owner.Id, organization.Id, target.Id, service.OrganizationAccessModeManagement, tc.body)

			require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			response := decodeOrganizationMemberQuotaResponse(t, recorder)
			assert.False(t, response.Success)
			assert.Contains(t, response.Message, tc.message)

			var stored model.OrganizationMember
			require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, target.Id).First(&stored).Error)
			assert.Zero(t, stored.QuotaLimit)
		})
	}
}
