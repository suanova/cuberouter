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
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOrganizationCanonicalRestfulRoutesAreRegistered(t *testing.T) {
	// gin.SetMode 写的是 gin 的包级 modeName，非原子写。放在 t.Parallel() 之前，
	// 它就在顺序阶段执行；放到之后会与同样并行运行的其它用例并发写同一个全局。
	gin.SetMode(gin.TestMode)
	t.Parallel()

	r := gin.New()
	require.NotPanics(t, func() {
		SetApiRouter(r)
	})

	routes := registeredRouteSet(r)

	require.Contains(t, routes, "GET /api/account-contexts")
	require.Contains(t, routes, "PUT /api/account-contexts/current")
	require.Contains(t, routes, "GET /api/organizations")
	require.Contains(t, routes, "POST /api/organizations")
	require.Contains(t, routes, "GET /api/organizations/self")
	require.Contains(t, routes, "GET /api/organizations/:id")
	require.Contains(t, routes, "PATCH /api/organizations/:id")
	require.Contains(t, routes, "DELETE /api/organizations/:id")
	require.Contains(t, routes, "PUT /api/organizations/:id/owner")
	require.Contains(t, routes, "GET /api/organizations/:id/members")
	require.Contains(t, routes, "PATCH /api/organizations/:id/members/:userId")
	require.Contains(t, routes, "GET /api/organizations/:id/members/:userId/model-discounts")
	require.Contains(t, routes, "PUT /api/organizations/:id/members/:userId/model-discounts")
	require.Contains(t, routes, "GET /api/organizations/:id/members/:userId/model-access")
	require.Contains(t, routes, "PUT /api/organizations/:id/members/:userId/model-access")
	require.Contains(t, routes, "GET /api/organizations/:id/members/:userId/quota")
	require.Contains(t, routes, "PUT /api/organizations/:id/members/:userId/quota")
	require.Contains(t, routes, "DELETE /api/organizations/:id/members/me")
	require.Contains(t, routes, "DELETE /api/organizations/:id/members/:userId")
	require.Contains(t, routes, "GET /api/organizations/:id/invitations")
	require.Contains(t, routes, "POST /api/organizations/:id/invitations")
	require.Contains(t, routes, "DELETE /api/organizations/:id/invitations/:invitationId")
	require.Contains(t, routes, "GET /api/organization-invitations/:token")
	require.Contains(t, routes, "PATCH /api/organization-invitations/:token")
	require.Contains(t, routes, "GET /api/organizations/:id/tokens")
	require.Contains(t, routes, "POST /api/organizations/:id/tokens")
	require.Contains(t, routes, "POST /api/organizations/:id/token-batches")
	require.Contains(t, routes, "POST /api/organizations/:id/token-deletions")
	require.Contains(t, routes, "GET /api/organizations/:id/tokens/:tokenId")
	require.Contains(t, routes, "PATCH /api/organizations/:id/tokens/:tokenId")
	require.Contains(t, routes, "DELETE /api/organizations/:id/tokens/:tokenId")
	require.Contains(t, routes, "PATCH /api/organizations/:id/tokens/:tokenId/responsible-user")
	require.Contains(t, routes, "GET /api/organizations/:id/quota-data")
	require.Contains(t, routes, "GET /api/organizations/:id/logs")
	require.Contains(t, routes, "GET /api/organizations/:id/logs/stats")
	require.Contains(t, routes, "GET /api/organizations/:id/midjourney-tasks")
	require.Contains(t, routes, "GET /api/organizations/:id/billing/summary")
	require.Contains(t, routes, "GET /api/organizations/:id/billing/members/me")
	require.Contains(t, routes, "GET /api/organizations/:id/billing/user-summaries")
	require.Contains(t, routes, "GET /api/organizations/:id/billing/monthly-summaries")
	require.Contains(t, routes, "GET /api/organizations/:id/billing/records")
	require.Contains(t, routes, "GET /api/organizations/:id/audit-logs")
	require.Contains(t, routes, "GET /api/admin/organizations")
	require.Contains(t, routes, "GET /api/admin/organizations/:id")
	require.Contains(t, routes, "PATCH /api/admin/organizations/:id")
	require.Contains(t, routes, "PATCH /api/admin/organizations/:id/status")
	require.Contains(t, routes, "DELETE /api/admin/organizations/:id")
	require.Contains(t, routes, "PUT /api/admin/organizations/:id/owner")
	require.Contains(t, routes, "POST /api/admin/organizations/:id/quota-adjustments")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/members")
	require.Contains(t, routes, "POST /api/admin/organizations/:id/members")
	require.Contains(t, routes, "PATCH /api/admin/organizations/:id/members/:userId")
	require.Contains(t, routes, "DELETE /api/admin/organizations/:id/members/:userId")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/tokens")
	require.NotContains(t, routes, "POST /api/admin/organizations/:id/tokens")
	require.NotContains(t, routes, "POST /api/admin/organizations/:id/token-batches")
	require.Contains(t, routes, "POST /api/admin/organizations/:id/token-deletions")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/tokens/:tokenId")
	require.Contains(t, routes, "PATCH /api/admin/organizations/:id/tokens/:tokenId")
	require.Contains(t, routes, "DELETE /api/admin/organizations/:id/tokens/:tokenId")
	require.Contains(t, routes, "PATCH /api/admin/organizations/:id/tokens/:tokenId/responsible-user")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/logs")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/logs/stats")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/tasks")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/midjourney-tasks")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/quota-data")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/billing/summary")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/billing/user-summaries")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/billing/monthly-summaries")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/billing/records")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/audit-logs")
}

func TestOrganizationLegacyRoutesAreRemoved(t *testing.T) {
	// 同上：gin.SetMode 必须在 t.Parallel() 之前，否则与并行用例抢 gin 的包级 modeName。
	gin.SetMode(gin.TestMode)
	t.Parallel()

	r := gin.New()
	require.NotPanics(t, func() {
		SetApiRouter(r)
	})

	routes := registeredRouteSet(r)

	for route := range routes {
		method, path, _ := strings.Cut(route, " ")
		if strings.HasPrefix(path, "/api/organization-invites") {
			t.Fatalf("legacy organization invite route restored: %s", route)
		}
		if strings.HasPrefix(path, "/api/organizations/history") {
			t.Fatalf("legacy organization history route restored: %s", route)
		}
		if path == "/api/organization-invitations/:token" && method == "POST" {
			t.Fatalf("legacy invitation acceptance method restored: %s", route)
		}
		if !isOrganizationRoutePath(path) {
			continue
		}
		for _, fragment := range []string{"/disable", "/enable", "/owner-transfer", "/leave", "/resend", "/acceptances", "/audits"} {
			require.NotContains(t, path, fragment, "legacy organization route restored: %s", route)
		}
	}

	require.NotContains(t, routes, "GET /api/account-contexts/")
	require.NotContains(t, routes, "GET /api/organizations/")
	require.NotContains(t, routes, "POST /api/organizations/")
	require.NotContains(t, routes, "GET /api/admin/organizations/")
	require.NotContains(t, routes, "PUT /api/organizations/:id")
	require.NotContains(t, routes, "POST /api/organizations/:id/members")
	require.NotContains(t, routes, "POST /api/organizations/:id/tokens/batch")
	require.NotContains(t, routes, "POST /api/organizations/:id/tokens/batch-deletions")
	require.NotContains(t, routes, "PATCH /api/organizations/:id/tokens/:tokenId/responsibility")
	require.NotContains(t, routes, "PUT /api/organizations/:id/tokens/:tokenId")
	require.NotContains(t, routes, "GET /api/organizations/:id/data")
	require.NotContains(t, routes, "GET /api/organizations/:id/log-stats")
	require.NotContains(t, routes, "GET /api/organizations/:id/billing-summary")
	require.NotContains(t, routes, "GET /api/organizations/:id/member-billing/me")
	require.NotContains(t, routes, "GET /api/organizations/:id/billing/details")
	require.NotContains(t, routes, "GET /api/organizations/:id/audit")
	require.NotContains(t, routes, "GET /api/organizations/:id/invites")
	require.NotContains(t, routes, "POST /api/organizations/:id/invites")
	require.NotContains(t, routes, "DELETE /api/organizations/:id/invites/:inviteId")
}

// TestOrganizationMemberQuotaRoutesEnforceCapabilities 验证路由层的两道权限门：
// GET 与成员名单同权（view_members_full），PUT 是独立的 manage_member_quotas。
// 普通成员两个方向都被拒绝，owner 能读能写。
func TestOrganizationMemberQuotaRoutesEnforceCapabilities(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	owner := createOrganizationPolicyRouterUser(t, "router-member-quota-owner", common.RoleCommonUser)
	member := createOrganizationPolicyRouterUser(t, "router-member-quota-member", common.RoleCommonUser)
	organization, err := service.CreateOrganization(owner.Id, service.CreateOrganizationRequest{Name: "Router Member Quota Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("quota", 300).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)

	quotaPath := "/api/organizations/" + strconv.Itoa(organization.Id) + "/members/" + strconv.Itoa(member.Id) + "/quota"
	perform := func(method string, user model.User, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, quotaPath, strings.NewReader(body))
		req.Header.Set("Authorization", user.GetAccessToken())
		req.Header.Set("New-Api-User", strconv.Itoa(user.Id))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Account-Context-Type", "organization")
		req.Header.Set("X-Account-Context-Id", strconv.Itoa(organization.Id))
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder
	}

	for _, method := range []string{http.MethodGet, http.MethodPut} {
		recorder := perform(method, member, `{"limit":50}`)
		require.Equal(t, http.StatusForbidden, recorder.Code, method+" "+recorder.Body.String())
		require.Contains(t, recorder.Body.String(), `"code":"organization_access_denied"`)
	}

	readRecorder := perform(http.MethodGet, owner, "")
	require.Equal(t, http.StatusOK, readRecorder.Code, readRecorder.Body.String())
	require.Contains(t, readRecorder.Body.String(), `"success":true`)
	require.Contains(t, readRecorder.Body.String(), `"organization_quota":300`)
	require.Contains(t, readRecorder.Body.String(), `"over_allocated":false`)

	writeRecorder := perform(http.MethodPut, owner, `{"limit":50}`)
	require.Equal(t, http.StatusOK, writeRecorder.Code, writeRecorder.Body.String())
	require.Contains(t, writeRecorder.Body.String(), `"success":true`)

	var stored model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, member.Id).First(&stored).Error)
	require.Equal(t, 50, stored.QuotaLimit)

	updatedRecorder := perform(http.MethodGet, owner, "")
	require.Equal(t, http.StatusOK, updatedRecorder.Code, updatedRecorder.Body.String())
	require.Contains(t, updatedRecorder.Body.String(), `"limit":50`)
	require.Contains(t, updatedRecorder.Body.String(), `"allocated":50`)
}

func isOrganizationRoutePath(path string) bool {
	return strings.HasPrefix(path, "/api/organizations") ||
		strings.HasPrefix(path, "/api/organization-invitations") ||
		strings.HasPrefix(path, "/api/admin/organizations")
}

func registeredRouteSet(r *gin.Engine) map[string]struct{} {
	routes := make(map[string]struct{})
	for _, route := range r.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	return routes
}
