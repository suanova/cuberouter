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
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetSelfUsageRejectsRangeOver31Days(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/usage/self?start=2026-01-01&end=2026-03-01", nil)
	c.Set("id", 7)

	GetSelfUsage(c)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "时间跨度不能超过 31 天")
}

func newSelfUsageControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	origDB, origLog := model.DB, model.LOG_DB
	origOptions := common.OptionMap
	origRedis := common.RedisEnabled
	model.DB, model.LOG_DB = db, db
	common.OptionMap = map[string]string{model.LogUsageBackfillDoneOption: "true"}
	common.RedisEnabled = false
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Log{},
		&model.Task{},
		&model.Midjourney{},
		&model.Token{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.Model{},
	))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = origDB, origLog
		common.OptionMap = origOptions
		common.RedisEnabled = origRedis
	})
	return db
}

func TestGetSelfUsageControllerOrgMembership(t *testing.T) {
	db := newSelfUsageControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{Id: 7, Username: "u7", Quota: 0}).Error)
	require.NoError(t, db.Create(&model.Organization{Id: 9, Name: "Acme", Slug: "acme", OwnerUserId: 99, Quota: 100}).Error)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/usage/self?start=2026-11-03&end=2026-11-03&overview_organization_id=9", nil)
	c.Set("id", 7)

	GetSelfUsage(c)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "organization access denied")
}
