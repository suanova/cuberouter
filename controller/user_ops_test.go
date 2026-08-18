package controller

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOpsUserTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// The i18n bundle is only initialized from main.go, so tests that hit
	// handlers rendering messages must initialize it themselves (sync.Once).
	i18n.Init()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.User{}))

	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	// Drain in-flight campaign dispatch goroutines before restoring the global
	// handles: cleanup is LIFO, so this drain runs before the restore above.
	t.Cleanup(service.DrainCampaignDispatches)
	return db
}

func TestGetOpsInviteesMasksPhone(t *testing.T) {
	db := setupOpsUserTestDB(t)
	inviter := model.User{Username: "ops-list", Password: "password", Role: common.RoleOpsUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "ops-list-aff"}
	require.NoError(t, db.Create(&inviter).Error)
	inviteeToken := "invitee-list-token"
	require.NoError(t, db.Create(&model.User{Username: "invitee-1", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", InviterId: inviter.Id, Phone: "13812345678", AffCode: "invitee-1-aff", AccessToken: &inviteeToken}).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/ops/user/?p=1&page_size=10", nil)
	c.Set("id", inviter.Id)
	GetOpsInvitees(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Items []struct {
				Phone string `json:"phone"`
			} `json:"items"`
			Total int `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.Equal(t, 1, body.Data.Total)
	require.Len(t, body.Data.Items, 1)
	assert.Equal(t, "138****5678", body.Data.Items[0].Phone)
	assert.NotContains(t, recorder.Body.String(), "invitee-list-token", "invitee access token must not be exposed")
}

func TestSearchOpsInviteesMasksPhoneAndUsesKeyword(t *testing.T) {
	db := setupOpsUserTestDB(t)
	inviter := model.User{Username: "ops-search", Password: "password", Role: common.RoleOpsUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "ops-search-aff"}
	require.NoError(t, db.Create(&inviter).Error)
	kwHitToken := "invitee-search-token"
	require.NoError(t, db.Create(&model.User{Username: "kw-hit", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", InviterId: inviter.Id, Phone: "13911112222", AffCode: "kw-hit-aff", AccessToken: &kwHitToken}).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/ops/user/search?keyword=kw-hit&p=1&page_size=10", nil)
	c.Set("id", inviter.Id)
	SearchOpsInvitees(c)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Items []struct {
				Username string `json:"username"`
				Phone    string `json:"phone"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.True(t, body.Success)
	require.Len(t, body.Data.Items, 1)
	assert.Equal(t, "kw-hit", body.Data.Items[0].Username)
	assert.Equal(t, "139****2222", body.Data.Items[0].Phone)
	assert.NotContains(t, recorder.Body.String(), "invitee-search-token", "invitee access token must not be exposed")
}

func TestExportOpsInviteesWritesCsv(t *testing.T) {
	db := setupOpsUserTestDB(t)
	inviter := model.User{Username: "ops-export", Password: "password", Role: common.RoleOpsUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "ops-export-aff"}
	require.NoError(t, db.Create(&inviter).Error)
	require.NoError(t, db.Create(&model.User{Username: "export-row", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", InviterId: inviter.Id, Quota: 1000, AffCode: "export-row-aff", AffCount: 2, AffQuota: 500}).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/ops/user/export", strings.NewReader(`{"keyword":"export-row","format":"csv"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", inviter.Id)
	c.Set("username", "ops-operator")
	ExportOpsInvitees(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/csv; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.Contains(t, recorder.Header().Get("Content-Disposition"), "attachment")
	raw := recorder.Body.Bytes()
	assert.True(t, len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF, "must start with UTF-8 BOM")
	rows, err := csv.NewReader(strings.NewReader(string(raw[3:]))).ReadAll()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rows), 2)
	assert.Equal(t, opsUserExportHeaders(c), rows[0])
	assert.Equal(t, "export-row", rows[1][1])
}

// TestExportOpsInviteesFailsWithoutWritingCsvOnDbError verifies that a
// failed export batch surfaces as an error response instead of a successful
// (partial) CSV: all batches are fetched before the header is written.
func TestExportOpsInviteesFailsWithoutWritingCsvOnDbError(t *testing.T) {
	db := setupOpsUserTestDB(t)
	inviter := model.User{Username: "ops-export-fail", Password: "password", Role: common.RoleOpsUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "ops-export-fail-aff"}
	require.NoError(t, db.Create(&inviter).Error)
	own := model.User{Username: "export-fail-row", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", InviterId: inviter.Id, AffCode: "export-fail-row-aff"}
	require.NoError(t, db.Create(&own).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/ops/user/export", strings.NewReader(fmt.Sprintf(`{"ids":[%d],"format":"csv"}`, own.Id)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", inviter.Id)
	c.Set("username", "ops-operator")
	ExportOpsInvitees(c)

	var body struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.False(t, body.Success)
	raw := recorder.Body.Bytes()
	assert.False(t, len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF, "must not write a CSV on export failure")
}

func TestCsvSafeCell(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"plain", "plain"},
		{"=1+1", "'=1+1"},
		{"+SUM(A1:A2)", "'+SUM(A1:A2)"},
		{"-1+2", "'-1+2"},
		{"@cmd", "'@cmd"},
		{"\tindent", "'\tindent"},
		{"\rreturn", "'\rreturn"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, csvSafeCell(tt.in), "input %q", tt.in)
	}
}

func TestExportOpsInviteesCsvNeutralizesFormulas(t *testing.T) {
	db := setupOpsUserTestDB(t)
	inviter := model.User{Username: "ops-export-inject", Password: "password", Role: common.RoleOpsUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "ops-export-inject-aff"}
	require.NoError(t, db.Create(&inviter).Error)
	require.NoError(t, db.Create(&model.User{
		Username: "=2+5", DisplayName: "+SUM(A1:A2)", Password: "password",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		Group: "@default", InviterId: inviter.Id, AffCode: "-1+1",
	}).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/ops/user/export", strings.NewReader(`{"keyword":"2+5","format":"csv"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", inviter.Id)
	c.Set("username", "ops-operator")
	ExportOpsInvitees(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	raw := recorder.Body.Bytes()
	require.GreaterOrEqual(t, len(raw), 3)
	rows, err := csv.NewReader(strings.NewReader(string(raw[3:]))).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 2) // header + the formula-flagged invitee
	// Username, DisplayName, Group and AffCode must be neutralized against
	// spreadsheet formula evaluation while keeping the original value.
	// (下标随 CSV 布局：request_count 后新增 total_prompt_tokens /
	// total_completion_tokens 两列，AffCode 由 10 顺延至 12。)
	assert.Equal(t, "'=2+5", rows[1][1])
	assert.Equal(t, "'+SUM(A1:A2)", rows[1][2])
	assert.Equal(t, "'@default", rows[1][5])
	assert.Equal(t, "'-1+1", rows[1][12])
}

func TestExportOpsInviteesHonorsIdsOverKeyword(t *testing.T) {
	db := setupOpsUserTestDB(t)
	inviter := model.User{Username: "ops-export-ids", Password: "password", Role: common.RoleOpsUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "ops-export-ids-aff"}
	require.NoError(t, db.Create(&inviter).Error)
	own := model.User{Username: "ids-own", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", InviterId: inviter.Id, AffCode: "ids-own-aff"}
	require.NoError(t, db.Create(&own).Error)
	foreign := model.User{Username: "ids-foreign", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", InviterId: 999999, AffCode: "ids-foreign-aff"}
	require.NoError(t, db.Create(&foreign).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/ops/user/export", strings.NewReader(
		fmt.Sprintf(`{"ids":[%d,%d],"keyword":"ids-own"}`, own.Id, foreign.Id)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", inviter.Id)
	c.Set("username", "ops-operator")
	ExportOpsInvitees(c)

	rows, err := csv.NewReader(strings.NewReader(string(recorder.Body.Bytes()[3:]))).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 2) // header + only the inviter's user
	assert.Equal(t, "ids-own", rows[1][1])
}

func TestExportOpsInviteesRejectsUnsupportedFormat(t *testing.T) {
	setupOpsUserTestDB(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/ops/user/export", strings.NewReader(`{"format":"xlsx"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", 1)
	c.Set("username", "ops-operator")
	ExportOpsInvitees(c)

	var body struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.False(t, body.Success)
	// The bundle is initialized in setupOpsUserTestDB, so the message is
	// rendered rather than returned as its raw key.
	assert.Equal(t, i18n.T(c, i18n.MsgOpsExportUnsupportedFormat, map[string]any{"Format": "xlsx"}), body.Message)
}

func TestGetOpsUserColumnsReturnsMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/ops/user/columns", nil)
	GetOpsUserColumns(c)

	var body struct {
		Success bool                `json:"success"`
		Data    []OpsUserColumnMeta `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.True(t, body.Success)
	require.NotEmpty(t, body.Data)
	required := map[string]bool{}
	for _, col := range body.Data {
		if col.Required {
			required[col.Key] = true
		}
	}
	assert.True(t, required["id"])
	assert.True(t, required["username"])
}
