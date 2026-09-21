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

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 创建组织是平台管理动作：只有 admin/root 能建。运营(5)虽然在 admin(10) 之下、却在
// UserAuth(1) 之上，是这条门最容易加错的一档，所以单列一行锁住它。
//
// 库是子测试共享的，所以每个断言都按 created_by 收窄——否则计数会被别的子测试串味。
func TestCreateOrganizationRouteRequiresPlatformAdmin(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)

	tests := []struct {
		name       string
		role       int
		wantStatus int
	}{
		{name: "common user rejected", role: common.RoleCommonUser, wantStatus: http.StatusForbidden},
		{name: "ops user rejected", role: common.RoleOpsUser, wantStatus: http.StatusForbidden},
		{name: "admin allowed", role: common.RoleAdminUser, wantStatus: http.StatusOK},
		{name: "root allowed", role: common.RoleRootUser, wantStatus: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actor := createOrganizationPolicyRouterUser(t, "create-route-"+strings.ReplaceAll(test.name, " ", "-"), test.role)

			req := httptest.NewRequest(http.MethodPost, "/api/organizations", strings.NewReader(fmt.Sprintf(`{"name":"Route Org %d"}`, actor.Id)))
			req.Header.Set("Authorization", actor.GetAccessToken())
			req.Header.Set("New-Api-User", strconv.Itoa(actor.Id))
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			var created int64
			require.NoError(t, model.DB.Model(&model.Organization{}).Where("created_by = ?", actor.Id).Count(&created).Error)

			if test.wantStatus != http.StatusOK {
				require.Equal(t, test.wantStatus, recorder.Code)
				require.Contains(t, recorder.Body.String(), "AUTH_INSUFFICIENT_PRIVILEGE")
				// 被拒就是没有副作用，不只是返回了 403。
				assert.Zero(t, created)
				return
			}

			require.Equal(t, http.StatusOK, recorder.Code)
			assert.Contains(t, recorder.Body.String(), `"success":true`)
			require.EqualValues(t, 1, created)

			var organization model.Organization
			require.NoError(t, model.DB.Where("created_by = ?", actor.Id).First(&organization).Error)

			var member model.OrganizationMember
			require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, actor.Id).First(&member).Error)
			assert.Equal(t, model.OrganizationRoleOwner, member.Role)
			assert.Equal(t, model.OrganizationMemberStatusActive, member.Status)
		})
	}
}
