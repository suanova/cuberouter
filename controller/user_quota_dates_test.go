package controller

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupQuotaDatesTestDB adds the quota_data table (used by the dashboard
// queries) on top of the shared controller test DB.
// setupManageUserTestDB (not setupOpsUserTestDB): the ops helper calls
// i18n.Init(), a process-wide sync.Once — new test files must not trigger
// that initialization (see the Task 4 note; user_manage_test.go asserts raw
// i18n keys).
func setupQuotaDatesTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupManageUserTestDB(t)
	require.NoError(t, db.Exec(`CREATE TABLE quota_data (
		id integer primary key autoincrement,
		user_id integer, username text, model_name text, created_at integer,
		use_group text, token_id integer, channel_id integer, node_name text,
		token_used integer, count integer, quota integer,
		scope_type text, scope_id integer, billing_account_type text,
		billing_account_id integer, organization_id integer, responsible_user_id integer
	)`).Error)
	return db
}

func seedQuotaData(t *testing.T, db *gorm.DB, userId int, username, modelName string, createdAt, quota, tokenUsed int) {
	t.Helper()
	require.NoError(t, db.Table("quota_data").Create(&model.QuotaData{
		UserID: userId, Username: username, ModelName: modelName, CreatedAt: int64(createdAt),
		Quota: quota, TokenUsed: tokenUsed, Count: 1,
	}).Error)
}

// Pure role-hierarchy gate: root always; otherwise myRole > targetRole.
func TestCanViewUserDashboard(t *testing.T) {
	cases := []struct {
		name       string
		myRole     int
		targetRole int
		want       bool
	}{
		{name: "admin views common", myRole: common.RoleAdminUser, targetRole: common.RoleCommonUser, want: true},
		{name: "admin views ops", myRole: common.RoleAdminUser, targetRole: common.RoleOpsUser, want: true},
		{name: "admin cannot view admin", myRole: common.RoleAdminUser, targetRole: common.RoleAdminUser, want: false},
		{name: "admin cannot view root", myRole: common.RoleAdminUser, targetRole: common.RoleRootUser, want: false},
		{name: "root views root", myRole: common.RoleRootUser, targetRole: common.RoleRootUser, want: true},
		{name: "root views anyone", myRole: common.RoleRootUser, targetRole: common.RoleCommonUser, want: true},
		{name: "ops views common", myRole: common.RoleOpsUser, targetRole: common.RoleCommonUser, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, canViewUserDashboard(tc.myRole, tc.targetRole))
		})
	}
}

// Role matrix: admin (10) sees common (1) and ops (5); rejected for admin
// (10) and root (100); root (100) sees anyone.
func TestGetUserQuotaDatesByAdminRoleMatrix(t *testing.T) {
	db := setupQuotaDatesTestDB(t)
	commonUser := model.User{Username: "qd-common", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "qd-common-aff"}
	opsUser := model.User{Username: "qd-ops", Password: "password", Role: common.RoleOpsUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "qd-ops-aff"}
	adminUser := model.User{Username: "qd-admin", Password: "password", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "qd-admin-aff"}
	rootUser := model.User{Username: "qd-root", Password: "password", Role: common.RoleRootUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "qd-root-aff"}
	for _, u := range []*model.User{&commonUser, &opsUser, &adminUser, &rootUser} {
		require.NoError(t, db.Create(u).Error)
	}

	gin.SetMode(gin.TestMode)
	// Real router + role-setting middleware: gin.CreateTestContext does not
	// populate :id path params, and the handler reads role from the context.
	runAs := func(myRole int, targetId int) (bool, string) {
		router := gin.New()
		router.GET("/api/user/:id/quota-dates", func(c *gin.Context) {
			c.Set("role", myRole)
			GetUserQuotaDatesByAdmin(c)
		})
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/user/%d/quota-dates?start_timestamp=0&end_timestamp=100", targetId), nil))
		var body struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
		return body.Success, body.Message
	}

	// admin (10): sees common and ops
	ok, _ := runAs(common.RoleAdminUser, commonUser.Id)
	assert.True(t, ok, "admin must see a common user")
	ok, _ = runAs(common.RoleAdminUser, opsUser.Id)
	assert.True(t, ok, "admin must see an ops user")
	// admin cannot see another admin or root
	ok, msg := runAs(common.RoleAdminUser, adminUser.Id)
	assert.False(t, ok)
	assert.Equal(t, common.TranslateMessage(expectCtx(), i18n.MsgAdminCannotViewUserDashboard), msg)
	ok, _ = runAs(common.RoleAdminUser, rootUser.Id)
	assert.False(t, ok, "admin must not see root")
	// root sees anyone
	ok, _ = runAs(common.RoleRootUser, rootUser.Id)
	assert.True(t, ok, "root sees root")
	ok, _ = runAs(common.RoleRootUser, commonUser.Id)
	assert.True(t, ok, "root sees common")
}

// Range guard: > 1 month rejected; exactly 1 month allowed.
func TestGetUserQuotaDatesByAdminRangeGuard(t *testing.T) {
	db := setupQuotaDatesTestDB(t)
	target := model.User{Username: "qd-range", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "qd-range-aff"}
	require.NoError(t, db.Create(&target).Error)

	gin.SetMode(gin.TestMode)
	call := func(start, end int64) (bool, string) {
		router := gin.New()
		router.GET("/api/user/:id/quota-dates", func(c *gin.Context) {
			c.Set("role", common.RoleAdminUser)
			GetUserQuotaDatesByAdmin(c)
		})
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/user/%d/quota-dates?start_timestamp=%d&end_timestamp=%d", target.Id, start, end), nil))
		var body struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
		return body.Success, body.Message
	}

	ok, _ := call(0, adminQuotaDatesMaxRange)
	assert.True(t, ok, "boundary == 1 month allowed")
	ok, msg := call(0, adminQuotaDatesMaxRange+1)
	assert.False(t, ok)
	assert.Equal(t, common.TranslateMessage(expectCtx(), i18n.MsgAdminQuotaDatesRangeExceeded), msg)
}

// Timestamp validation: parse failures, negative values, reversed intervals,
// and extreme int64 pairs are all rejected before the range is used, while
// valid extreme-but-ordered values still pass (no overflow rejection).
func TestGetUserQuotaDatesByAdminTimestampValidation(t *testing.T) {
	db := setupQuotaDatesTestDB(t)
	target := model.User{Username: "qd-ts", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "qd-ts-aff"}
	require.NoError(t, db.Create(&target).Error)

	gin.SetMode(gin.TestMode)
	call := func(query string) (bool, string) {
		router := gin.New()
		router.GET("/api/user/:id/quota-dates", func(c *gin.Context) {
			c.Set("role", common.RoleAdminUser)
			GetUserQuotaDatesByAdmin(c)
		})
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/user/%d/quota-dates?%s", target.Id, query), nil))
		var body struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
		return body.Success, body.Message
	}

	cases := []struct {
		name  string
		query string
	}{
		{name: "malformed start", query: "start_timestamp=abc&end_timestamp=100"},
		{name: "malformed end", query: "start_timestamp=0&end_timestamp=abc"},
		{name: "missing timestamps", query: ""},
		{name: "negative start", query: "start_timestamp=-100&end_timestamp=100"},
		{name: "negative end", query: "start_timestamp=0&end_timestamp=-100"},
		{name: "reversed interval", query: "start_timestamp=100&end_timestamp=0"},
		{name: "overflow-prone extremes", query: fmt.Sprintf("start_timestamp=0&end_timestamp=%d", int64(math.MaxInt64))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, msg := call(tc.query)
			assert.False(t, ok)
			assert.Equal(t, common.TranslateMessage(expectCtx(), i18n.MsgAdminQuotaDatesRangeExceeded), msg)
		})
	}

	ok, _ := call(fmt.Sprintf("start_timestamp=%d&end_timestamp=%d", int64(math.MaxInt64)-1, int64(math.MaxInt64)))
	assert.True(t, ok, "valid extreme-but-ordered values must not be rejected")
}

// Response shape: data.user is the dashboard brief; data.dates matches
// GetQuotaDataByUserId output.
func TestGetUserQuotaDatesByAdminResponseShape(t *testing.T) {
	db := setupQuotaDatesTestDB(t)
	target := model.User{Username: "qd-shape", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "qd-shape-aff", Quota: 5000, UsedQuota: 1200, RequestCount: 30}
	require.NoError(t, db.Create(&target).Error)
	seedQuotaData(t, db, target.Id, target.Username, "gpt-4o", 1000, 150, 40)
	seedQuotaData(t, db, target.Id, target.Username, "gpt-4o", 2000, 250, 60)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/user/:id/quota-dates", func(c *gin.Context) {
		c.Set("role", common.RoleAdminUser)
		GetUserQuotaDatesByAdmin(c)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/user/%d/quota-dates?start_timestamp=0&end_timestamp=100000", target.Id), nil))

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			User struct {
				Id           int    `json:"id"`
				Username     string `json:"username"`
				Quota        int    `json:"quota"`
				UsedQuota    int    `json:"used_quota"`
				RequestCount int    `json:"request_count"`
			} `json:"user"`
			Dates []map[string]any `json:"dates"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.Equal(t, target.Id, body.Data.User.Id)
	assert.Equal(t, "qd-shape", body.Data.User.Username)
	assert.Equal(t, 5000, body.Data.User.Quota)
	assert.Equal(t, 1200, body.Data.User.UsedQuota)
	assert.Equal(t, 30, body.Data.User.RequestCount)
	require.Len(t, body.Data.Dates, 2)
	assert.Equal(t, "gpt-4o", body.Data.Dates[0]["model_name"])
}

// Invalid :id and missing target user.
func TestGetUserQuotaDatesByAdminInvalidIdAndMissingUser(t *testing.T) {
	setupQuotaDatesTestDB(t)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/api/user/:id/quota-dates", func(c *gin.Context) {
		c.Set("role", common.RoleAdminUser)
		GetUserQuotaDatesByAdmin(c)
	})

	var body struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/user/abc/quota-dates", nil))
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.False(t, body.Success)
	assert.Equal(t, common.TranslateMessage(expectCtx(), i18n.MsgInvalidId), body.Message)

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/user/999999/quota-dates", nil))
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.False(t, body.Success)
	assert.Equal(t, common.TranslateMessage(expectCtx(), i18n.MsgUserNotExists), body.Message)
}

// expectCtx returns a throwaway gin context used only to render an expected
// message. A real context with a non-nil Request is required: the pre-init
// fallback of common.TranslateMessage calls c.Header and would panic on nil,
// and after i18n.Init() it renders via i18n.T -> GetLangFromContext ->
// c.GetHeader, which also needs c.Request (gin.CreateTestContext leaves it
// nil). Same pattern as user_invitees_test.go.
func expectCtx() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return c
}
