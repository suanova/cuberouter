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
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setDshAllowedOrganizationsForMiddlewareTest(t *testing.T, organizationIds []int) {
	t.Helper()
	encoded := "[]"
	if len(organizationIds) > 0 {
		raw, err := common.Marshal(organizationIds)
		require.NoError(t, err)
		encoded = string(raw)
	}

	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap[setting.DshAllowedOrganizationIdsKey] = encoded
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		delete(common.OptionMap, setting.DshAllowedOrganizationIdsKey)
	})
}

func createDshMiddlewareTestUser(t *testing.T, username string, role int) model.User {
	t.Helper()
	user := model.User{Username: username, Password: "password", DisplayName: username, Role: role, Status: common.UserStatusEnabled, AffCode: username}
	require.NoError(t, model.DB.Create(&user).Error)
	return user
}

// probeDshMiddleware runs handler behind a route that seeds the caller identity
// the way the real auth middlewares do: "id" for the session/token user, the
// actor context key for token-scoped calls, and the resolved task plugin key.
func probeDshMiddleware(t *testing.T, handler gin.HandlerFunc, userId, actorUserId int, taskPluginKey string) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", userId)
		if actorUserId > 0 {
			c.Set(string(constant.ContextKeyActorUserId), actorUserId)
		}
		if taskPluginKey != "" {
			c.Set("expected_task_plugin_key", taskPluginKey)
		}
	})
	router.Use(handler)
	router.GET("/probe", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/probe", nil))
	return recorder
}

func TestDshAccessMiddleware(t *testing.T) {
	setupOrganizationMiddlewareTestDB(t, &model.User{}, &model.Organization{}, &model.OrganizationMember{})

	admin := createDshMiddlewareTestUser(t, "dsh-middleware-admin", common.RoleAdminUser)
	member := createDshMiddlewareTestUser(t, "dsh-middleware-member", common.RoleCommonUser)
	outsider := createDshMiddlewareTestUser(t, "dsh-middleware-outsider", common.RoleCommonUser)

	organization := model.Organization{Name: "DSH Middleware Organization", Status: model.OrganizationStatusActive}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: organization.Id,
		UserId:         member.Id,
		Role:           model.OrganizationRoleMember,
		Status:         model.OrganizationMemberStatusActive,
	}).Error)

	t.Run("denies a user outside the allowlisted organizations", func(t *testing.T) {
		setDshAllowedOrganizationsForMiddlewareTest(t, []int{organization.Id})
		recorder := probeDshMiddleware(t, DshAccess(), outsider.Id, 0, "")
		require.Equal(t, http.StatusForbidden, recorder.Code)
		require.Contains(t, recorder.Body.String(), translated(i18n.MsgDshAccessDenied))
	})

	t.Run("allows a member of an allowlisted organization", func(t *testing.T) {
		setDshAllowedOrganizationsForMiddlewareTest(t, []int{organization.Id})
		recorder := probeDshMiddleware(t, DshAccess(), member.Id, 0, "")
		require.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("allows a platform admin even when the allowlist is empty", func(t *testing.T) {
		setDshAllowedOrganizationsForMiddlewareTest(t, nil)
		recorder := probeDshMiddleware(t, DshAccess(), admin.Id, 0, "")
		require.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("denies everyone else when the allowlist is empty", func(t *testing.T) {
		setDshAllowedOrganizationsForMiddlewareTest(t, nil)
		recorder := probeDshMiddleware(t, DshAccess(), member.Id, 0, "")
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("evaluates the acting user rather than the token owner", func(t *testing.T) {
		setDshAllowedOrganizationsForMiddlewareTest(t, []int{organization.Id})
		recorder := probeDshMiddleware(t, DshAccess(), outsider.Id, member.Id, "")
		require.Equal(t, http.StatusOK, recorder.Code)

		recorder = probeDshMiddleware(t, DshAccess(), member.Id, outsider.Id, "")
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})
}

func TestDshSubmitAccessMiddleware(t *testing.T) {
	setupOrganizationMiddlewareTestDB(t, &model.User{}, &model.Organization{}, &model.OrganizationMember{})

	member := createDshMiddlewareTestUser(t, "dsh-submit-member", common.RoleCommonUser)
	outsider := createDshMiddlewareTestUser(t, "dsh-submit-outsider", common.RoleCommonUser)

	organization := model.Organization{Name: "DSH Submit Organization", Status: model.OrganizationStatusActive}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: organization.Id,
		UserId:         member.Id,
		Role:           model.OrganizationRoleMember,
		Status:         model.OrganizationMemberStatusActive,
	}).Error)
	setDshAllowedOrganizationsForMiddlewareTest(t, []int{organization.Id})

	t.Run("passes through for another task plugin", func(t *testing.T) {
		recorder := probeDshMiddleware(t, DshSubmitAccess(), outsider.Id, 0, "other-plugin")
		require.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("denies a DSH submission outside the allowlisted organizations", func(t *testing.T) {
		recorder := probeDshMiddleware(t, DshSubmitAccess(), outsider.Id, 0, setting.DshTaskPluginKey)
		require.Equal(t, http.StatusForbidden, recorder.Code)
		require.Contains(t, recorder.Body.String(), translated(i18n.MsgDshAccessDenied))
	})

	t.Run("allows a DSH submission from an allowlisted member", func(t *testing.T) {
		recorder := probeDshMiddleware(t, DshSubmitAccess(), member.Id, 0, setting.DshTaskPluginKey)
		require.Equal(t, http.StatusOK, recorder.Code)
	})
}
