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
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupAdminTokenControllerTest(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	origDB, origLog := model.DB, model.LOG_DB
	origRedis := common.RedisEnabled
	origTranslate := common.TranslateMessage
	model.DB, model.LOG_DB = db, db
	common.RedisEnabled = false
	common.TranslateMessage = func(_ *gin.Context, key string, _ ...map[string]any) string {
		return key
	}
	t.Cleanup(func() {
		model.DB, model.LOG_DB = origDB, origLog
		common.RedisEnabled = origRedis
		common.TranslateMessage = origTranslate
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}))

	require.NoError(t, db.Create(&[]model.User{
		{Id: 201, Username: "root", DisplayName: "Root", Role: common.RoleRootUser, Status: common.UserStatusEnabled, AffCode: "root"},
		{Id: 202, Username: "admin", DisplayName: "Admin", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, AffCode: "admin"},
		{Id: 203, Username: "ops", DisplayName: "Ops", Role: common.RoleOpsUser, Status: common.UserStatusEnabled, AffCode: "ops"},
		{Id: 204, Username: "alice", DisplayName: "Alice Chen", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "alice"},
		{Id: 205, Username: "bob", DisplayName: "Bob", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "bob"},
		{Id: 206, Username: "admin2", DisplayName: "Admin Two", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, AffCode: "admin2"},
	}).Error)
	require.NoError(t, db.Create(&[]model.Token{
		{Id: 301, UserId: 201, Name: "root-key", Key: "root-key", ScopeType: model.TokenScopePersonal, Status: common.TokenStatusEnabled},
		{Id: 302, UserId: 202, Name: "admin-key", Key: "admin-key", ScopeType: model.TokenScopePersonal, Status: common.TokenStatusEnabled},
		{Id: 303, UserId: 203, Name: "ops-key", Key: "ops-key", ScopeType: model.TokenScopePersonal, Status: common.TokenStatusEnabled},
		{Id: 304, UserId: 204, Name: "alice-key", Key: "alice-secret-key", ScopeType: model.TokenScopePersonal, Status: common.TokenStatusEnabled},
		{Id: 305, UserId: 205, Name: "bob-key", Key: "bob-key", ScopeType: model.TokenScopePersonal, Status: common.TokenStatusEnabled},
		{Id: 306, UserId: 204, Name: "alice-expired", Key: "alice-expired", ScopeType: model.TokenScopePersonal, Status: common.TokenStatusExpired, ExpiredTime: common.GetTimestamp() - 60},
		{Id: 307, UserId: 204, Name: "org-key", Key: "org-key", ScopeType: model.TokenScopeOrganization, OrganizationId: 9, Status: common.TokenStatusEnabled},
		{Id: 308, UserId: 206, Name: "admin2-key", Key: "admin2-key", ScopeType: model.TokenScopePersonal, Status: common.TokenStatusEnabled},
	}).Error)
}

func performAdminTokenRequest(
	handler gin.HandlerFunc,
	userId int,
	role int,
	method string,
	target string,
	body []byte,
	tokenId string,
) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, target, bytes.NewReader(body))
	if body != nil {
		c.Request.Header.Set("Content-Type", "application/json")
	}
	c.Set("id", userId)
	c.Set("role", role)
	c.Set("username", "admin")
	if tokenId != "" {
		c.Params = gin.Params{{Key: "id", Value: tokenId}}
	}
	handler(c)
	return recorder
}

type adminTokenItem struct {
	Id            int    `json:"id"`
	Key           string `json:"key"`
	OwnerUsername string `json:"owner_username"`
	OwnerDisplay  string `json:"owner_display_name"`
	Status        int    `json:"status"`
	ScopeType     string `json:"scope_type"`
}

type adminTokenResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Total int              `json:"total"`
		Items []adminTokenItem `json:"items"`
		Key   string           `json:"key"`
	} `json:"data"`
}

func decodeAdminTokenResponse(t *testing.T, recorder *httptest.ResponseRecorder) adminTokenResponse {
	t.Helper()
	var body adminTokenResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}

func latestAdminTokenAudit(t *testing.T) map[string]any {
	t.Helper()
	var log model.Log
	require.NoError(t, model.LOG_DB.Where("type = ?", model.LogTypeManage).Order("id desc").First(&log).Error)
	var other map[string]any
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	op, ok := other["op"].(map[string]any)
	require.True(t, ok)
	return op
}

func TestAdminListTokensReturnsMaskedOwnerRows(t *testing.T) {
	setupAdminTokenControllerTest(t)

	recorder := performAdminTokenRequest(AdminListTokens, 202, common.RoleAdminUser, http.MethodGet,
		"/api/admin/token/?p=1&size=20&username=alice", nil, "")
	body := decodeAdminTokenResponse(t, recorder)

	require.True(t, body.Success)
	assert.Equal(t, 2, body.Data.Total)
	require.Len(t, body.Data.Items, 2)

	var aliceKey *adminTokenItem
	for i := range body.Data.Items {
		if body.Data.Items[i].Id == 304 {
			aliceKey = &body.Data.Items[i]
		}
		if body.Data.Items[i].ScopeType == model.TokenScopeOrganization {
			t.Fatalf("organization key leaked into all-users list: %+v", body.Data.Items[i])
		}
	}
	require.NotNil(t, aliceKey)
	assert.Equal(t, "alice", aliceKey.OwnerUsername)
	assert.Equal(t, "Alice Chen", aliceKey.OwnerDisplay)
	assert.NotEqual(t, "alice-secret-key", aliceKey.Key)
}

func TestAdminGetTokenKeyWritesKeyViewAudit(t *testing.T) {
	setupAdminTokenControllerTest(t)

	recorder := performAdminTokenRequest(AdminGetTokenKey, 202, common.RoleAdminUser, http.MethodPost,
		"/api/admin/token/304/key", nil, "304")
	body := decodeAdminTokenResponse(t, recorder)

	require.True(t, body.Success)
	assert.Equal(t, "alice-secret-key", body.Data.Key)

	op := latestAdminTokenAudit(t)
	assert.Equal(t, "token.key_view", op["action"])
	params, ok := op["params"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alice", params["target_username"])
	assert.EqualValues(t, 304, params["token_id"])
}

func TestAdminGetTokenKeyRejectsHigherRole(t *testing.T) {
	setupAdminTokenControllerTest(t)

	recorder := performAdminTokenRequest(AdminGetTokenKey, 202, common.RoleAdminUser, http.MethodPost,
		"/api/admin/token/301/key", nil, "301")
	body := decodeAdminTokenResponse(t, recorder)

	assert.False(t, body.Success)
	assert.Equal(t, i18n.MsgAuthInsufficientPrivilege, body.Message)

	recorder = performAdminTokenRequest(AdminGetTokenKey, 202, common.RoleAdminUser, http.MethodPost,
		"/api/admin/token/308/key", nil, "308")
	body = decodeAdminTokenResponse(t, recorder)
	assert.False(t, body.Success)
	assert.Equal(t, i18n.MsgAuthInsufficientPrivilege, body.Message)

	var count int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("type = ?", model.LogTypeManage).Count(&count).Error)
	assert.Zero(t, count)
}

func TestAdminUpdateTokenStatusHonorsExhaustedGuard(t *testing.T) {
	setupAdminTokenControllerTest(t)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", 305).Updates(map[string]any{
		"status":          common.TokenStatusExhausted,
		"remain_quota":    0,
		"unlimited_quota": false,
	}).Error)

	recorder := performAdminTokenRequest(AdminUpdateTokenStatus, 202, common.RoleAdminUser, http.MethodPut,
		"/api/admin/token/305/status", []byte(`{"status":1}`), "305")
	body := decodeAdminTokenResponse(t, recorder)

	assert.False(t, body.Success)
	assert.Equal(t, i18n.MsgTokenExhaustedCannotEable, body.Message)
}

func TestAdminUpdateTokenStatusWritesAuditAndHonorsExpiredGuard(t *testing.T) {
	setupAdminTokenControllerTest(t)

	recorder := performAdminTokenRequest(AdminUpdateTokenStatus, 202, common.RoleAdminUser, http.MethodPut,
		"/api/admin/token/304/status", []byte(`{"status":2}`), "304")
	body := decodeAdminTokenResponse(t, recorder)
	require.True(t, body.Success)

	var stored model.Token
	require.NoError(t, model.DB.First(&stored, "id = ?", 304).Error)
	assert.Equal(t, common.TokenStatusDisabled, stored.Status)

	op := latestAdminTokenAudit(t)
	assert.Equal(t, "token.status_disable", op["action"])

	recorder = performAdminTokenRequest(AdminUpdateTokenStatus, 202, common.RoleAdminUser, http.MethodPut,
		"/api/admin/token/306/status", []byte(`{"status":1}`), "306")
	body = decodeAdminTokenResponse(t, recorder)
	assert.False(t, body.Success)
	assert.Equal(t, i18n.MsgTokenExpiredCannotEnable, body.Message)
}

type sqlCaptureLogger struct {
	logger.Interface
	mu   sync.Mutex
	sqls []string
}

func (l *sqlCaptureLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	l.mu.Lock()
	l.sqls = append(l.sqls, sql)
	l.mu.Unlock()
	l.Interface.Trace(ctx, begin, fc, err)
}

func (l *sqlCaptureLogger) lastUpdateOn(t *testing.T, table string) string {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := len(l.sqls) - 1; i >= 0; i-- {
		sql := strings.TrimSpace(l.sqls[i])
		if strings.HasPrefix(strings.ToUpper(sql), "UPDATE") && strings.Contains(sql, table) {
			return sql
		}
	}
	return ""
}

func TestAdminUpdateTokenStatusOnlyWritesStatus(t *testing.T) {
	setupAdminTokenControllerTest(t)
	capture := &sqlCaptureLogger{Interface: logger.Discard}
	model.DB = model.DB.Session(&gorm.Session{Logger: capture})

	recorder := performAdminTokenRequest(AdminUpdateTokenStatus, 202, common.RoleAdminUser, http.MethodPut,
		"/api/admin/token/304/status", []byte(`{"status":2}`), "304")
	body := decodeAdminTokenResponse(t, recorder)
	require.True(t, body.Success)

	updateSQL := capture.lastUpdateOn(t, "tokens")
	require.NotEmpty(t, updateSQL)
	assert.Contains(t, updateSQL, "status")
	assert.NotContains(t, updateSQL, "remain_quota")
	assert.NotContains(t, updateSQL, "allow_ips")
	assert.NotContains(t, updateSQL, "model_limits")
}

func TestAdminDeleteTokenWritesAudit(t *testing.T) {
	setupAdminTokenControllerTest(t)

	recorder := performAdminTokenRequest(AdminDeleteToken, 202, common.RoleAdminUser, http.MethodDelete,
		"/api/admin/token/304", nil, "304")
	body := decodeAdminTokenResponse(t, recorder)
	require.True(t, body.Success)

	var deleted model.Token
	err := model.DB.First(&deleted, "id = ?", 304).Error
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.NoError(t, model.DB.Unscoped().First(&deleted, "id = ?", 304).Error)

	op := latestAdminTokenAudit(t)
	assert.Equal(t, "token.delete", op["action"])
}
