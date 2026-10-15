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
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func imageAPITestDB(t *testing.T) *gorm.DB {
	t.Helper()
	t.Setenv("IMAGE_STUDIO_CHANNEL_MODEL", "qwen-image-2.1")
	t.Setenv("IMAGE_STUDIO_ALLOWED_GROUPS", "trial")
	t.Setenv("IMAGE_STUDIO_AUDIT_ENABLED", "true")
	t.Setenv("IMAGE_STUDIO_RELAY_TOKEN", strings.Repeat("r", 40))
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	old, redis := model.DB, common.RedisEnabled
	model.DB, common.RedisEnabled = db, false
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { model.DB, common.RedisEnabled = old, redis; sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.ImageStudioAudit{}, &model.ImageAPIPolicy{}, &model.ImageModerationPolicy{}, &model.ImageModerationLease{}))
	require.NoError(t, db.Create(&model.User{Id: 701, Username: "api-test", Group: "trial", Status: common.UserStatusEnabled, Quota: 100000}).Error)
	require.NoError(t, db.Create(&model.Token{Id: 711, UserId: 701, Key: strings.Repeat("k", 48), Name: "limited-image-key", Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 1000}).Error)
	return db
}

func apiTestIdentity() *studioAPIIdentity {
	return &studioAPIIdentity{TokenID: 711, Group: "trial", ClientIP: "127.0.0.1", RequestID: strings.Repeat("x", 32)}
}

func TestImageAPIOrganizationUsesAccountPolicyBillingAndRechecksMembership(t *testing.T) {
	db := imageAPITestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Organization{}, &model.OrganizationMember{}, &model.OrganizationTokenSystemBlocker{}))
	require.NoError(t, db.Create(&model.Organization{Id: 900, Name: "Image organization", Slug: "image-org", Group: "trial", Status: model.OrganizationStatusActive, OwnerUserId: 701, CreatedBy: 701, Quota: 10000}).Error)
	require.NoError(t, db.Create(&model.OrganizationMember{OrganizationId: 900, UserId: 701, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, db.Model(&model.Token{}).Where("id = ?", 711).Updates(map[string]any{
		"scope_type": model.TokenScopeOrganization, "scope_id": 900, "organization_id": 900,
		"visibility": model.TokenVisibilityPrivate, "creator_user_id": 701, "responsible_user_id": 701,
	}).Error)
	// The personal account has no image enrollment. Only its organization does.
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 701).Update("group", "default").Error)
	require.NoError(t, model.SaveImageAPIPolicy(model.ImageAPIPolicy{GroupName: "trial", Moderation: "required"}))
	user, err := model.GetUserById(701, false)
	require.NoError(t, err)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.NoError(t, prepareStudioAPIIdentity(c, user, apiTestIdentity()))
	require.Equal(t, "required", c.GetString("image_api_moderation"))
	require.Equal(t, "trial", c.GetString(string(constant.ContextKeyUserGroup)))
	require.Equal(t, model.AccountContextTypeOrganization, c.GetString(string(constant.ContextKeyBillingAccountType)))
	require.Equal(t, 900, c.GetInt(string(constant.ContextKeyBillingAccountId)))
	require.Equal(t, 711, c.GetInt("token_id"))
	// Queued execution revalidates current organization state before any GPU call.
	require.NoError(t, db.Model(&model.OrganizationMember{}).Where("organization_id = ?", 900).Update("status", model.OrganizationMemberStatusDisabled).Error)
	require.Error(t, prepareStudioAPIIdentity(c, user, apiTestIdentity()))
	require.NoError(t, db.Model(&model.OrganizationMember{}).Where("organization_id = ?", 900).Update("status", model.OrganizationMemberStatusActive).Error)
	require.NoError(t, db.Model(&model.Organization{}).Where("id = ?", 900).Update("group", "default").Error)
	require.Error(t, prepareStudioAPIIdentity(c, user, apiTestIdentity()))
}

func TestImageAPIRechecksRevocationModelQuotaAndIP(t *testing.T) {
	db := imageAPITestDB(t)
	user, err := model.GetUserById(701, false)
	require.NoError(t, err)
	check := func() error {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		return prepareStudioAPIIdentity(c, user, apiTestIdentity())
	}
	require.NoError(t, check())
	for _, tc := range []struct {
		field     string
		bad, good any
	}{
		{"status", common.TokenStatusDisabled, common.TokenStatusEnabled},
		{"expired_time", time.Now().Unix() - 1, int64(-1)},
		{"remain_quota", 0, 1000},
		{"allow_ips", "192.0.2.1", ""},
		{"model_limits_enabled", true, false},
	} {
		t.Run(tc.field, func(t *testing.T) {
			require.NoError(t, db.Model(&model.Token{}).Where("id = ?", 711).Update(tc.field, tc.bad).Error)
			require.Error(t, check())
			require.NoError(t, db.Model(&model.Token{}).Where("id = ?", 711).Update(tc.field, tc.good).Error)
			require.NoError(t, check())
		})
	}
	user.Group = "default"
	require.Error(t, check())
	user.Group = "trial"
	require.NoError(t, db.Delete(&model.Token{}, 711).Error)
	require.Error(t, check())
}

func TestImageAPIPrivateExecutionKeepsRealTokenBillingAndRejectsWrongRoute(t *testing.T) {
	imageAPITestDB(t)
	p := studioTestParameters()
	p.API = apiTestIdentity()
	body, _ := common.Marshal(p)
	r := gin.New()
	calls := 0
	handler := func(c *gin.Context) {
		calls++
		require.Equal(t, 711, c.GetInt("token_id"))
		require.False(t, c.GetBool("token_unlimited_quota"))
		require.Equal(t, 1000, c.GetInt("token_quota"))
		info := relaycommon.GenRelayInfoImage(c, &dto.ImageRequest{})
		require.False(t, info.IsPlayground, "queued API must debit the real token")
		c.Status(200)
	}
	r.POST("/internal/image-studio/execute-api", PrepareImageStudioRelay, handler)
	r.POST("/internal/image-studio/execute", PrepareImageStudioRelay, handler)
	job := strings.Repeat("e", 32)
	t.Cleanup(func() { studioExecutions.Lock(); delete(studioExecutions.seen, job); studioExecutions.Unlock() })
	send := func(path string) int {
		request := httptest.NewRequest("POST", path, bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+strings.Repeat("r", 40))
		request.Header.Set("X-Image-Owner", "cuberouter-701")
		request.Header.Set("X-Image-Job", job)
		request.Header.Set("X-Image-Execution-Deadline", strconv.FormatInt(time.Now().Add(25*time.Minute).Unix(), 10))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, request)
		return w.Code
	}
	require.Equal(t, 400, send("/internal/image-studio/execute"))
	require.Equal(t, 200, send("/internal/image-studio/execute-api"))
	require.Equal(t, 1, calls)
	rows, err := model.FindImageStudioAudits(job, "", 701)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "api", rows[0].Source)
	require.Equal(t, 711, rows[0].TokenID)
	require.Equal(t, "external", rows[0].ModerationPolicy)
}

func TestImageAPIParsingPreservesInputAndRejectsBypassFields(t *testing.T) {
	t.Setenv("IMAGE_STUDIO_CHANNEL_MODEL", "qwen-image-2.1")
	for _, tc := range []struct {
		body           string
		handled, valid bool
	}{
		{`{"model":"qwen-image-2.1","prompt":"一隻貓\nflower","size":"768x512","extra_fields":{"cfg":0,"steps":12,"seed":123}}`, true, true},
		{`{"model":"another-image-model","unrelated":true}`, false, true},
		{`{"model":"qwen-image-2.1","prompt":"cat","moderation":"external"}`, true, false},
		{`{"model":"qwen-image-2.1","prompt":"cat","_api":{"token_id":1}}`, true, false},
		{`{"model":"qwen-image-2.1","prompt":"cat","extra_fields":{"steps":1.5}}`, true, false},
		{`{"model":"qwen-image-2.1","prompt":"cat","size":"-1x512"}`, true, false},
		{`{"model":"qwen-image-2.1","prompt":"cat","n":2}`, true, false},
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/v1/images/generations", strings.NewReader(tc.body))
		c.Request.Header.Set("Content-Type", "application/json")
		p, handled, err := imageAPIParameters(c)
		require.Equal(t, tc.handled, handled)
		require.Equal(t, tc.valid, err == nil)
		if handled && err == nil {
			require.Equal(t, "一隻貓\nflower", p.Prompt)
			require.Zero(t, p.CFG)
			require.Equal(t, 12, p.Steps)
			require.EqualValues(t, 123, p.Seed)
		}
		common.CleanupBodyStorage(c)
	}
	p := studioTestParameters()
	p.Mode = "edit"
	p.Images = []string{base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\ninput"))}
	body, kind, path, err := studioRelayBody(p, "qwen-image-2.1")
	require.NoError(t, err)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", kind)
	actual, handled, err := imageAPIParameters(c)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, p.Images, actual.Images)
	require.Equal(t, p.Prompt, actual.Prompt)
	common.CleanupBodyStorage(c)
}

func TestImageAPIRequiredFailsClosedExternalIsExplicit(t *testing.T) {
	imageAPITestDB(t)
	t.Setenv("IMAGE_STUDIO_MODERATION_ENABLED", "false")
	for i, policy := range []string{"external", "required"} {
		job := fmt.Sprintf("%032d", i+90)
		p := studioTestParameters()
		p.API = apiTestIdentity()
		require.NoError(t, studioAudit(job, 701, p, "running"))
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/", nil)
		c.Set("id", 701)
		c.Set(common.RequestIdKey, job)
		c.Set("image_api_moderation", policy)
		b := &studioRelayWriter{ResponseWriter: c.Writer, status: 200}
		png := []byte("\x89PNG\r\n\x1a\nsafe-fixture")
		response, _ := common.Marshal(gin.H{"data": []gin.H{{"b64_json": base64.StdEncoding.EncodeToString(png)}}})
		b.Write(response)
		deliverStudioOutput(c, b)
		rows, err := model.FindImageStudioAudits(job, "", 701)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Len(t, rows[0].OutputHash, 64)
		if policy == "external" {
			require.Equal(t, 200, w.Code)
			require.Equal(t, "external", rows[0].ModerationState)
		} else {
			require.Equal(t, 422, w.Code)
			require.NotContains(t, w.Body.String(), "b64_json")
			require.Equal(t, "error", rows[0].ModerationState)
		}
	}
}

func TestImageAPIIngressDoesNotSubmitToOldDispatcher(t *testing.T) {
	imageAPITestDB(t)
	submits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" {
			submits++
		}
		fmt.Fprint(w, `{"epoch":"old"}`)
	}))
	defer server.Close()
	t.Setenv("IMAGE_STUDIO_DISPATCHER_URL", server.URL)
	t.Setenv("IMAGE_STUDIO_DISPATCHER_TOKEN", strings.Repeat("d", 40))
	c, w := gin.CreateTestContext(httptest.NewRecorder())
	_ = w
	c.Request = httptest.NewRequest("POST", "/v1/images/generations", strings.NewReader(`{"model":"qwen-image-2.1","prompt":"cat"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", 701)
	c.Set("token_id", 711)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "trial")
	QueueImageAPI(c)
	require.Equal(t, 503, c.Writer.Status())
	require.Zero(t, submits)
}

func TestStudioBrowserCannotFetchAPIJobOutput(t *testing.T) {
	imageAPITestDB(t)
	job := strings.Repeat("a", 32)
	p := studioTestParameters()
	p.API = apiTestIdentity()
	require.NoError(t, studioAudit(job, 701, p, "completed"))
	t.Setenv("IMAGE_STUDIO_DISPATCHER_URL", "http://127.0.0.1:1")
	t.Setenv("IMAGE_STUDIO_DISPATCHER_TOKEN", strings.Repeat("d", 40))
	for _, suffix := range []string{"", "/result", "/cancel"} {
		method := "GET"
		if suffix == "/cancel" {
			method = "POST"
		}
		require.Equal(t, 404, studioRequest(method, "/jobs/"+job+suffix, "{}", 701).Code)
	}
}
