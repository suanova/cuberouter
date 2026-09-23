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
package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 规则配置是凭证级控制（成员能读到组织全部 API Key），只有 root 能碰：
// 平台管理员(10)在这条门上必须被拒。
func TestOrganizationJoinRuleRoutesRequireRoot(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)

	owner := createOrganizationPolicyRouterUser(t, "join-rule-route-owner", common.RoleRootUser)
	organization := model.Organization{
		Name:        "Join Rule Route Org",
		Slug:        "join-rule-route-org",
		Status:      model.OrganizationStatusActive,
		OwnerUserId: owner.Id,
		CreatedBy:   owner.Id,
	}
	require.NoError(t, model.DB.Create(&organization).Error)

	tests := []struct {
		name       string
		role       int
		wantStatus int
	}{
		{name: "common user rejected", role: common.RoleCommonUser, wantStatus: http.StatusForbidden},
		{name: "admin rejected", role: common.RoleAdminUser, wantStatus: http.StatusForbidden},
		{name: "root allowed", role: common.RoleRootUser, wantStatus: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actor := createOrganizationPolicyRouterUser(t, "join-rule-route-"+strings.ReplaceAll(test.name, " ", "-"), test.role)
			req := httptest.NewRequest(http.MethodGet, "/api/admin/organizations/"+strconv.Itoa(organization.Id)+"/join-rules", nil)
			req.Header.Set("Authorization", actor.GetAccessToken())
			req.Header.Set("New-Api-User", strconv.Itoa(actor.Id))
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			assert.Equal(t, test.wantStatus, recorder.Code, recorder.Body.String())
		})
	}
}

// 行级校验失败是前端逐行出文案的契约：400 + organization_join_rule_invalid +
// line_errors。越界 pattern 会撞上 varchar(191) 的唯一索引，一旦漏掉校验就是
// 500（原始数据库错误），所以这里同时锁住"整批不写入"。
func TestCreateOrganizationJoinRulesReportsLineErrors(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	originalEmailVerification := common.EmailVerificationEnabled
	common.EmailVerificationEnabled = true
	t.Cleanup(func() { common.EmailVerificationEnabled = originalEmailVerification })

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)

	root := createOrganizationPolicyRouterUser(t, "join-rule-line-error-root", common.RoleRootUser)
	organization := model.Organization{
		Name:        "Join Rule Line Error Org",
		Slug:        "join-rule-line-error-org",
		Status:      model.OrganizationStatusActive,
		OwnerUserId: root.Id,
		CreatedBy:   root.Id,
	}
	require.NoError(t, model.DB.Create(&organization).Error)

	tooLong := strings.Repeat("a", 190) + ".com"
	body := fmt.Sprintf(`{"patterns":["not a domain","%s"],"reason":"route contract"}`, tooLong)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/organizations/"+strconv.Itoa(organization.Id)+"/join-rules", strings.NewReader(body))
	req.Header.Set("Authorization", root.GetAccessToken())
	req.Header.Set("New-Api-User", strconv.Itoa(root.Id))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	assert.Contains(t, recorder.Body.String(), `"code":"organization_join_rule_invalid"`)
	assert.Contains(t, recorder.Body.String(), `"line_errors"`)
	assert.Contains(t, recorder.Body.String(), `"invalid_pattern"`)

	var stored int64
	require.NoError(t, model.DB.Model(&model.OrganizationJoinRule{}).Where("organization_id = ?", organization.Id).Count(&stored).Error)
	assert.Zero(t, stored)
}

// 成功的写入与行级错误共用 400/200 两种信封，前端按 code 分支，所以成功路径也得锁住
// {"rules":...,"notices":...}。审计行里留下的是归一化后的 pattern：审计页看到的那一列
// 必须能指认是哪条规则（targetType 没有对应 case 时它就是空串）。
//
// 删除刻意不带请求体、原因只从 query 给，锁住"空 body 不报错 + query 兜底"。
func TestOrganizationJoinRuleRoutesRecordPatternInAuditTargetName(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	originalEmailVerification := common.EmailVerificationEnabled
	common.EmailVerificationEnabled = true
	t.Cleanup(func() { common.EmailVerificationEnabled = originalEmailVerification })

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)

	root := createOrganizationPolicyRouterUser(t, "join-rule-audit-root", common.RoleRootUser)
	organization := model.Organization{
		Name:        "Join Rule Audit Org",
		Slug:        "join-rule-audit-org",
		Status:      model.OrganizationStatusActive,
		OwnerUserId: root.Id,
		CreatedBy:   root.Id,
	}
	require.NoError(t, model.DB.Create(&organization).Error)

	basePath := "/api/admin/organizations/" + strconv.Itoa(organization.Id) + "/join-rules"
	createReq := httptest.NewRequest(http.MethodPost, basePath, strings.NewReader(`{"patterns":["*.Enterprise.COM"],"reason":"route contract"}`))
	createReq.Header.Set("Authorization", root.GetAccessToken())
	createReq.Header.Set("New-Api-User", strconv.Itoa(root.Id))
	createReq.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, createReq)

	require.Equal(t, http.StatusOK, createRecorder.Code, createRecorder.Body.String())
	assert.Contains(t, createRecorder.Body.String(), `"success":true`)
	assert.Contains(t, createRecorder.Body.String(), `"rules"`)
	assert.Contains(t, createRecorder.Body.String(), `"notices"`)

	var rule model.OrganizationJoinRule
	require.NoError(t, model.DB.Where("organization_id = ?", organization.Id).First(&rule).Error)
	assert.Equal(t, "*.enterprise.com", rule.PatternNormalized)

	var createAudit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", organization.Id, "organization.join_rule.create").First(&createAudit).Error)
	assert.Equal(t, "join_rule", createAudit.TargetType)
	assert.Equal(t, rule.Id, createAudit.TargetId)
	assert.Equal(t, "*.enterprise.com", createAudit.TargetName)

	deleteReq := httptest.NewRequest(http.MethodDelete, basePath+"/"+strconv.Itoa(rule.Id)+"?reason=route%20contract%20cleanup", nil)
	deleteReq.Header.Set("Authorization", root.GetAccessToken())
	deleteReq.Header.Set("New-Api-User", strconv.Itoa(root.Id))
	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, deleteReq)

	require.Equal(t, http.StatusOK, deleteRecorder.Code, deleteRecorder.Body.String())
	assert.Contains(t, deleteRecorder.Body.String(), `"success":true`)

	var deleteAudit model.OrganizationAuditLog
	require.NoError(t, model.DB.Where("organization_id = ? AND action_type = ?", organization.Id, "organization.join_rule.delete").First(&deleteAudit).Error)
	assert.Equal(t, rule.Id, deleteAudit.TargetId)
	assert.Equal(t, "*.enterprise.com", deleteAudit.TargetName)
	assert.Equal(t, "route contract cleanup", deleteAudit.Reason)

	var remaining int64
	require.NoError(t, model.DB.Model(&model.OrganizationJoinRule{}).Where("organization_id = ?", organization.Id).Count(&remaining).Error)
	assert.Zero(t, remaining)
}
