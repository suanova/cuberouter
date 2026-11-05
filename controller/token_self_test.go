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

func newSelfTokenControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	origDB, origLog := model.DB, model.LOG_DB
	origOptions := common.OptionMap
	origRedis := common.RedisEnabled
	model.DB, model.LOG_DB = db, db
	common.OptionMap = map[string]string{}
	common.RedisEnabled = false
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.OrganizationTokenSystemBlocker{},
	))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = origDB, origLog
		common.OptionMap = origOptions
		common.RedisEnabled = origRedis
	})
	return db
}

func seedSelfTokenController(t *testing.T) {
	t.Helper()
	db := model.DB
	require.NoError(t, db.Create(&[]model.User{
		{Id: 101, Username: "alice", DisplayName: "Alice", AffCode: "alice", Status: common.UserStatusEnabled, Role: common.RoleCommonUser},
	}).Error)
	require.NoError(t, db.Create(&model.Organization{
		Id: 9, Name: "Acme", Slug: "acme", Status: model.OrganizationStatusActive, OwnerUserId: 101,
	}).Error)
	require.NoError(t, db.Create(&model.OrganizationMember{
		OrganizationId: 9, UserId: 101, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive,
	}).Error)
	require.NoError(t, db.Create(&[]model.Token{
		{Id: 1, UserId: 101, Name: "personal", Key: "personal", ScopeType: model.TokenScopePersonal, Status: common.TokenStatusEnabled},
		{Id: 2, UserId: 101, Name: "acme-public", Key: "acme-public", ScopeType: model.TokenScopeOrganization, OrganizationId: 9, Visibility: model.TokenVisibilityPublic, Status: common.TokenStatusEnabled},
	}).Error)
}

func TestGetSelfTokensControllerReturnsPersonalAndUsableOrganizationKeys(t *testing.T) {
	newSelfTokenControllerTestDB(t)
	seedSelfTokenController(t)

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/token/self/all?p=1&size=20", nil)
	c.Set("id", 101)

	GetSelfTokens(c)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"total":2`)
	assert.Contains(t, body, `"name":"personal"`)
	assert.Contains(t, body, `"name":"acme-public"`)
	assert.Contains(t, body, `"organization_name":"Acme"`)
	assert.Contains(t, body, `"key":"acme-public"`)
	assert.Contains(t, body, `"key_preview":"`)
	// The personal row keeps the masked-key behavior of the personal list.
	assert.Contains(t, body, `"key":"pe****al"`)
}
