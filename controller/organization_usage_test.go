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
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newOrganizationUsageControllerTestDB(t *testing.T) *gorm.DB {
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
		&model.Ability{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.OrganizationDisableRecord{},
		&model.Model{},
	))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = origDB, origLog
		common.OptionMap = origOptions
		common.RedisEnabled = origRedis
	})
	return db
}

func seedOrganizationUsageController(t *testing.T) int64 {
	t.Helper()
	db := model.DB
	ts := time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&model.User{
		Id: 101, Username: "alice", DisplayName: "Alice Chen", AffCode: "alice",
		Status: common.UserStatusEnabled, Role: common.RoleCommonUser,
	}).Error)
	require.NoError(t, db.Create(&model.Organization{
		Id: 9, Name: "Acme", Slug: "acme", Status: model.OrganizationStatusActive, OwnerUserId: 101,
	}).Error)
	require.NoError(t, db.Create(&model.OrganizationMember{
		OrganizationId: 9, UserId: 101, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive,
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id: 1, UserId: 101, Name: "team-key", Key: "sk-team",
		ScopeType: model.TokenScopeOrganization, OrganizationId: 9,
	}).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId: 101, Username: "alice", ResponsibleUserId: 101, ResponsibleName: "Alice Chen",
		Type: model.LogTypeConsume, UsageSource: model.LogUsageSourceText, CreatedAt: ts,
		BillingAccountType: model.AccountContextTypeOrganization, BillingAccountId: 9, OrganizationId: 9,
		OrganizationName: "Acme", TokenId: 1, TokenName: "team-key", ModelName: "gpt-4o",
		RequestId: "usage-1", PromptTokens: 10, CompletionTokens: 5, Quota: 25,
	}).Error)
	return ts
}

func organizationUsageRequest(t *testing.T, rawURL string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, rawURL, nil)
	c.Params = gin.Params{{Key: "id", Value: "9"}}
	c.Set("id", 101)
	common.SetContextKey(c, constant.ContextKeyOrganizationAccessMode, service.OrganizationAccessModeManagement)
	return c, rec
}

func TestGetOrganizationUsageControllerReturnsAggregatedRows(t *testing.T) {
	newOrganizationUsageControllerTestDB(t)
	seedOrganizationUsageController(t)

	c, rec := organizationUsageRequest(t, "/api/organizations/9/usage?start=2026-11-03&end=2026-11-03")
	GetOrganizationUsage(c)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"total":1`)
	assert.Contains(t, body, `"responsible_name":"Alice Chen"`)
	assert.Contains(t, body, `"models":["gpt-4o"]`)
	assert.Contains(t, body, `"tokens":[`)
}

func TestExportOrganizationUsageControllerGroupsByHour(t *testing.T) {
	newOrganizationUsageControllerTestDB(t)
	seedOrganizationUsageController(t)

	c, rec := organizationUsageRequest(t, "/api/organizations/9/usage/export?start=2026-11-03&end=2026-11-03&granularity=hour")
	ExportOrganizationUsage(c)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"granularity":"hour"`)
	assert.Contains(t, body, `"date":"2026-11-03 10:00"`)
	assert.Contains(t, body, `"responsible_name":"Alice Chen"`)
}
