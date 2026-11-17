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
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cubyAppTestEngine serves the Cuby app handlers as one signed-in user, the
// way the auth middleware hands a request over: with the user id in "id".
func cubyAppTestEngine(t *testing.T, userId int) *gin.Engine {
	t.Helper()
	engine := gin.New()
	engine.Use(func(c *gin.Context) { c.Set("id", userId) })
	engine.GET("/api/account/apps", GetAccountCubyApps)
	engine.GET("/api/admin/cuby-apps", ListCubyApps)
	engine.POST("/api/admin/cuby-apps", CreateCubyApp)
	engine.POST("/api/admin/cuby-apps/:id/grants", CreateCubyAppGrant)
	engine.PUT("/api/admin/cuby-app-setting", UpdateCubyAppSetting)
	return engine
}

func serveCubyAppTestRequest(engine *gin.Engine, method string, path string, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)
	return recorder
}

func setupCubyAppControllerTest(t *testing.T) {
	t.Helper()
	db := setupControllerOrganizationTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.CubyApp{}, &model.CubyAppGrant{}, &model.Option{}))
	originalOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		require.NoError(t, service.SetCubyAppManagementEnabled(false))
		common.OptionMap = originalOptionMap
	})
}

// The path, the 404 and the JSON below are what the Cuby desktop client was
// built against (cuby: App/memmy-agent/src/apps/README.md). Changing any of
// them changes what every installed Cuby does.
func TestGetAccountCubyAppsAnswersTheCubyContract(t *testing.T) {
	setupCubyAppControllerTest(t)
	admin := model.User{Username: "cuby-admin", AffCode: "cuby-admin", Role: common.RoleAdminUser, Status: common.UserStatusEnabled}
	member := model.User{Username: "cuby-member", AffCode: "cuby-member", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	outsider := model.User{Username: "cuby-outsider", AffCode: "cuby-outsider", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	for _, user := range []*model.User{&admin, &member, &outsider} {
		require.NoError(t, model.DB.Create(user).Error)
	}
	organization := model.Organization{Name: "Harbour Ltd", Slug: "harbour", CreatedBy: admin.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive,
	}).Error)

	adminEngine := cubyAppTestEngine(t, admin.Id)
	created := serveCubyAppTestRequest(adminEngine, http.MethodPost, "/api/admin/cuby-apps", `{"app_id":"ai-company","name":"AI company"}`)
	require.Equal(t, http.StatusOK, created.Code, created.Body.String())
	var app model.CubyApp
	require.NoError(t, model.DB.Where("app_id = ?", "ai-company").First(&app).Error)
	grantsPath := fmt.Sprintf("/api/admin/cuby-apps/%d/grants", app.Id)
	for _, body := range []string{
		fmt.Sprintf(`{"subject_type":"user","subject_id":%d}`, member.Id),
		fmt.Sprintf(`{"subject_type":"organization","subject_id":%d}`, organization.Id),
	} {
		granted := serveCubyAppTestRequest(adminEngine, http.MethodPost, grantsPath, body)
		require.Equal(t, http.StatusOK, granted.Code, granted.Body.String())
	}

	memberEngine := cubyAppTestEngine(t, member.Id)
	notManaged := serveCubyAppTestRequest(memberEngine, http.MethodGet, "/api/account/apps", "")
	assert.Equal(t, http.StatusNotFound, notManaged.Code,
		"while the switch is off Cuby must get the same 404 an older instance gives")

	switched := serveCubyAppTestRequest(adminEngine, http.MethodPut, "/api/admin/cuby-app-setting", `{"enabled":true}`)
	require.Equal(t, http.StatusOK, switched.Code, switched.Body.String())

	answer := serveCubyAppTestRequest(memberEngine, http.MethodGet, "/api/account/apps", "")
	require.Equal(t, http.StatusOK, answer.Code)
	assert.JSONEq(t, fmt.Sprintf(`{
		"success": true,
		"message": "",
		"data": {"apps": [
			{"id": "ai-company", "sources": [
				{"type": "user"},
				{"type": "organization", "id": %d, "name": "Harbour Ltd"}
			]}
		]}
	}`, organization.Id), answer.Body.String())

	empty := serveCubyAppTestRequest(cubyAppTestEngine(t, outsider.Id), http.MethodGet, "/api/account/apps", "")
	require.Equal(t, http.StatusOK, empty.Code)
	assert.JSONEq(t, `{"success": true, "message": "", "data": {"apps": []}}`, empty.Body.String(),
		"an account without apps gets an empty list: for Cuby that is an answer, unlike the 404")
}

func TestCubyAppAdminRoutesTellConflictsFromMissingThings(t *testing.T) {
	setupCubyAppControllerTest(t)
	admin := model.User{Username: "cuby-admin", AffCode: "cuby-admin", Role: common.RoleAdminUser, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&admin).Error)
	engine := cubyAppTestEngine(t, admin.Id)

	require.Equal(t, http.StatusOK, serveCubyAppTestRequest(engine, http.MethodPost, "/api/admin/cuby-apps", `{"app_id":"ai-company"}`).Code)
	assert.Equal(t, http.StatusConflict, serveCubyAppTestRequest(engine, http.MethodPost, "/api/admin/cuby-apps", `{"app_id":"ai-company"}`).Code)
	assert.Equal(t, http.StatusBadRequest, serveCubyAppTestRequest(engine, http.MethodPost, "/api/admin/cuby-apps", `{"app_id":"AI Company"}`).Code)

	var app model.CubyApp
	require.NoError(t, model.DB.Where("app_id = ?", "ai-company").First(&app).Error)
	grantsPath := fmt.Sprintf("/api/admin/cuby-apps/%d/grants", app.Id)
	holder := fmt.Sprintf(`{"subject_type":"user","subject_id":%d}`, admin.Id)
	assert.Equal(t, http.StatusNotFound, serveCubyAppTestRequest(engine, http.MethodPost, "/api/admin/cuby-apps/9999/grants", holder).Code)
	assert.Equal(t, http.StatusNotFound, serveCubyAppTestRequest(engine, http.MethodPost, grantsPath, `{"subject_type":"user","subject_id":9999}`).Code)
	require.Equal(t, http.StatusOK, serveCubyAppTestRequest(engine, http.MethodPost, grantsPath, holder).Code)
	assert.Equal(t, http.StatusConflict, serveCubyAppTestRequest(engine, http.MethodPost, grantsPath, holder).Code)

	assert.Equal(t, http.StatusBadRequest, serveCubyAppTestRequest(engine, http.MethodPut, "/api/admin/cuby-app-setting", `{}`).Code,
		"a request that does not say on or off must not switch anything")

	listed := serveCubyAppTestRequest(engine, http.MethodGet, "/api/admin/cuby-apps", "")
	require.Equal(t, http.StatusOK, listed.Code)
	var payload struct {
		Data struct {
			Enabled bool                  `json:"enabled"`
			Apps    []service.CubyAppView `json:"apps"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(listed.Body.Bytes(), &payload))
	assert.False(t, payload.Data.Enabled)
	require.Len(t, payload.Data.Apps, 1)
	assert.EqualValues(t, 1, payload.Data.Apps[0].UserGrantCount)
}
