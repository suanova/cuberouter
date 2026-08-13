package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GetUserInvitees returns InviteeBrief items (8 fields, no password/quota/
// token) with the raw phone — admins outrank the target, no masking.
func TestGetUserInviteesReturnsBriefItems(t *testing.T) {
	// setupManageUserTestDB instead of setupOpsUserTestDB: the ops helper calls
	// i18n.Init(), a process-wide sync.Once that permanently swaps
	// common.TranslateMessage to i18n.T. Tests in this package that run later
	// assert raw i18n keys in responses (user_manage_test.go) or depend on
	// being the first to initialize the bundle (user_ops_test.go), so this
	// file must not trigger that initialization.
	db := setupManageUserTestDB(t)
	target := model.User{Username: "invitees-target", Password: "password", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "invitees-target-aff"}
	require.NoError(t, db.Create(&target).Error)
	inviteeToken := "invitees-brief-token"
	require.NoError(t, db.Create(&model.User{Username: "invitee-brief", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", InviterId: target.Id, Phone: "13812345678", AffCode: "invitee-brief-aff", AccessToken: &inviteeToken}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/user/:id/invitees", GetUserInvitees)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/user/%d/invitees?p=1&page_size=10", target.Id), nil))

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Items []map[string]any `json:"items"`
			Total int              `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.Equal(t, 1, body.Data.Total)
	require.Len(t, body.Data.Items, 1)
	item := body.Data.Items[0]
	assert.Equal(t, "invitee-brief", item["username"])
	assert.Equal(t, "138****5678", item["phone"], "phone must be masked even for admins")
	assert.NotContains(t, item, "password")
	assert.NotContains(t, item, "quota")
	assert.NotContains(t, item, "used_quota")
	assert.NotContains(t, recorder.Body.String(), "invitee-brief-token", "invitee access token must not be exposed")
}

// GetUserInvitees respects the p/page_size query params.
func TestGetUserInviteesPagination(t *testing.T) {
	db := setupManageUserTestDB(t)
	target := model.User{Username: "invitees-paged-target", Password: "password", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "invitees-paged-target-aff"}
	require.NoError(t, db.Create(&target).Error)
	for i := 0; i < 3; i++ {
		require.NoError(t, db.Create(&model.User{Username: fmt.Sprintf("invitee-paged-%d", i), Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", InviterId: target.Id, AffCode: fmt.Sprintf("invitee-paged-%d-aff", i)}).Error)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/user/:id/invitees", GetUserInvitees)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/user/%d/invitees?p=1&page_size=2", target.Id), nil))

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Items []map[string]any `json:"items"`
			Total int              `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, 3, body.Data.Total)
	assert.Len(t, body.Data.Items, 2)
}

// GetUserInvitees rejects invalid :id with the invalid-id message.
func TestGetUserInviteesInvalidId(t *testing.T) {
	setupManageUserTestDB(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/user/:id/invitees", GetUserInvitees)
	for _, id := range []string{"0", "abc"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/user/"+id+"/invitees", nil))

		var body struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
		assert.False(t, body.Success)
		// Compare against what ApiErrorI18n renders for the key via
		// common.TranslateMessage, matching the handler regardless of whether
		// the i18n bundle has been initialized in this test process.
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/user/"+id+"/invitees", nil)
		assert.Equal(t, common.TranslateMessage(c, i18n.MsgInvalidId), body.Message)
	}
}
