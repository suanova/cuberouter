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
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUsageSiteRoutesRequireAdmin walks the real HTTP chain to pin the
// AdminAuth gate on the site-wide usage endpoint and its export.
func TestUsageSiteRoutesRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousSQLitePath := common.SQLitePath
	previousMasterNode := common.IsMasterNode
	previousRedisEnabled := common.RedisEnabled
	common.SQLitePath = t.TempDir() + "/router-usage-site.db"
	common.IsMasterNode = false
	common.RedisEnabled = false
	t.Setenv("SQL_DSN", "")
	require.NoError(t, model.InitDB())
	database := model.DB
	model.LOG_DB = database
	require.NoError(t, database.AutoMigrate(
		&model.User{},
		&model.Log{},
		&model.Task{},
		&model.Midjourney{},
		&model.Ability{},
		&model.Model{},
		&model.Token{},
	))
	t.Cleanup(func() {
		sqlDB, closeErr := database.DB()
		require.NoError(t, closeErr)
		require.NoError(t, sqlDB.Close())
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousDatabaseType, previousLogDatabaseType)
		common.SQLitePath = previousSQLitePath
		common.IsMasterNode = previousMasterNode
		common.RedisEnabled = previousRedisEnabled
	})

	adminPAT := "usage-site-admin-pat"
	userPAT := "usage-site-user-pat"
	require.NoError(t, database.Create(&model.User{
		Username:    "usage-site-admin",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "usagesiteadmin",
		AccessToken: &adminPAT,
	}).Error)
	require.NoError(t, database.Create(&model.User{
		Username:    "usage-site-user",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "usagesiteuser1",
		AccessToken: &userPAT,
	}).Error)

	engine := gin.New()
	SetApiRouter(engine)

	perform := func(path, authHeader string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if authHeader != "" {
			request.Header.Set("Authorization", authHeader)
		}
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		return recorder
	}

	sitePath := "/api/usage/site?start=2026-11-03&end=2026-11-03"
	assert.Equal(t, http.StatusUnauthorized, perform(sitePath, "").Code)
	assert.Equal(t, http.StatusForbidden, perform(sitePath, "Bearer "+userPAT).Code)
	adminSite := perform(sitePath, "Bearer "+adminPAT)
	require.Equal(t, http.StatusOK, adminSite.Code)
	assert.Contains(t, adminSite.Body.String(), `"success":true`)

	exportPath := "/api/usage/site/export?start=2026-11-03&end=2026-11-03&granularity=day"
	assert.Equal(t, http.StatusForbidden, perform(exportPath, "Bearer "+userPAT).Code)
	adminExport := perform(exportPath, "Bearer "+adminPAT)
	require.Equal(t, http.StatusOK, adminExport.Code)
	assert.Contains(t, adminExport.Body.String(), `"success":true`)
}
