package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupManageUserTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.UserSession{}, &model.Log{}, &model.CasbinRule{}, &model.AuthzRole{},
	))

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

func performManageUserRequest(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	return performManageUserRequestWithRole(t, body, common.RoleRootUser)
}

func performManageUserRequestWithRole(t *testing.T, body string, role int) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/manage", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", 9999)
	c.Set("role", role)
	c.Set("username", "operator")
	ManageUser(c)
	return recorder
}

func TestManageUserDisableAdvancesAuthVersionOnceAndRevokesSession(t *testing.T) {
	db := setupManageUserTestDB(t)
	now := time.Now().Unix()
	user := model.User{
		Username: "managed-disable-user", Password: "password", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1,
	}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&model.UserSession{
		SID: "managed-disable-session", UserID: user.Id, Version: 1, UserAuthVersion: 1,
		Status: model.UserSessionStatusActive, RefreshHash: "refresh-hash", LoginMethod: "password",
		LastActiveAt: now, ExpiresAt: now + 3600,
	}).Error)

	recorder := performManageUserRequest(t, fmt.Sprintf(`{"id":%d,"action":"disable"}`, user.Id))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var updated model.User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, common.UserStatusDisabled, updated.Status)
	assert.EqualValues(t, 2, updated.AuthVersion)
	var session model.UserSession
	require.NoError(t, db.First(&session, "sid = ?", "managed-disable-session").Error)
	assert.Equal(t, model.UserSessionStatusRevoked, session.Status)
}

func TestManageUserDemoteAdvancesAuthVersionAndRevokesSessionsOnce(t *testing.T) {
	db := setupManageUserTestDB(t)
	previousMaster := common.IsMasterNode
	common.IsMasterNode = false
	t.Cleanup(func() { common.IsMasterNode = previousMaster })
	require.NoError(t, authz.Init(db))

	now := time.Now().Unix()
	user := model.User{
		Username: "managed-demote-user", Password: "password", Role: common.RoleAdminUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1,
	}
	require.NoError(t, db.Create(&user).Error)
	for _, sid := range []string{"managed-demote-session-one", "managed-demote-session-two"} {
		require.NoError(t, db.Create(&model.UserSession{
			SID: sid, UserID: user.Id, Version: 1, UserAuthVersion: 1,
			Status: model.UserSessionStatusActive, RefreshHash: "refresh-" + sid, LoginMethod: "password",
			LastActiveAt: now, ExpiresAt: now + 3600,
		}).Error)
	}

	sessionUpdateCount := 0
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:count_demote_session_updates", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "user_sessions" {
			sessionUpdateCount++
		}
	}))

	recorder := performManageUserRequest(t, fmt.Sprintf(`{"id":%d,"action":"demote"}`, user.Id))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var updated model.User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, common.RoleCommonUser, updated.Role)
	assert.EqualValues(t, 2, updated.AuthVersion)
	var sessions []model.UserSession
	require.NoError(t, db.Where("user_id = ?", user.Id).Order("sid asc").Find(&sessions).Error)
	require.Len(t, sessions, 2)
	for _, session := range sessions {
		assert.Equal(t, model.UserSessionStatusRevoked, session.Status)
		assert.Equal(t, "admin_demote", session.RevokedReason)
	}
	assert.Equal(t, 1, sessionUpdateCount)
}

func TestManageUserDeleteReturnsImmediatelyAndUnknownActionFails(t *testing.T) {
	db := setupManageUserTestDB(t)
	deleted := model.User{
		Username: "managed-delete-user", Password: "password", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "delete-aff",
	}
	require.NoError(t, db.Create(&deleted).Error)

	recorder := performManageUserRequest(t, fmt.Sprintf(`{"id":%d,"action":"delete"}`, deleted.Id))
	assert.Contains(t, recorder.Body.String(), `"success":true`)
	var deletedCount int64
	require.NoError(t, db.Unscoped().Model(&model.User{}).Where("id = ? AND deleted_at IS NOT NULL", deleted.Id).Count(&deletedCount).Error)
	assert.EqualValues(t, 1, deletedCount)

	unchanged := model.User{
		Username: "managed-unknown-user", Password: "password", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "unknown-aff",
	}
	require.NoError(t, db.Create(&unchanged).Error)
	recorder = performManageUserRequest(t, fmt.Sprintf(`{"id":%d,"action":"unknown"}`, unchanged.Id))
	assert.Contains(t, recorder.Body.String(), `"success":false`)
	require.NoError(t, db.First(&unchanged, unchanged.Id).Error)
	assert.EqualValues(t, 1, unchanged.AuthVersion)
	assert.Equal(t, common.UserStatusEnabled, unchanged.Status)
}

func TestManageUserPromoteOps(t *testing.T) {
	db := setupManageUserTestDB(t)

	t.Run("root promotes common to ops", func(t *testing.T) {
		user := model.User{
			Username: "promote-ops-root", Password: "password", Role: common.RoleCommonUser,
			Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "promote-ops-root-aff",
		}
		require.NoError(t, db.Create(&user).Error)
		recorder := performManageUserRequest(t, fmt.Sprintf(`{"id":%d,"action":"promote_ops"}`, user.Id))
		assert.Contains(t, recorder.Body.String(), `"success":true`)
		var updated model.User
		require.NoError(t, db.First(&updated, user.Id).Error)
		assert.Equal(t, common.RoleOpsUser, updated.Role)
	})

	t.Run("admin promotes common to ops", func(t *testing.T) {
		user := model.User{
			Username: "promote-ops-admin", Password: "password", Role: common.RoleCommonUser,
			Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "promote-ops-admin-aff",
		}
		require.NoError(t, db.Create(&user).Error)
		recorder := performManageUserRequestWithRole(t, fmt.Sprintf(`{"id":%d,"action":"promote_ops"}`, user.Id), common.RoleAdminUser)
		assert.Contains(t, recorder.Body.String(), `"success":true`)
		var updated model.User
		require.NoError(t, db.First(&updated, user.Id).Error)
		assert.Equal(t, common.RoleOpsUser, updated.Role)
	})

	t.Run("ops user cannot promote", func(t *testing.T) {
		user := model.User{
			Username: "promote-ops-forbidden", Password: "password", Role: common.RoleCommonUser,
			Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "promote-ops-forbidden-aff",
		}
		require.NoError(t, db.Create(&user).Error)
		recorder := performManageUserRequestWithRole(t, fmt.Sprintf(`{"id":%d,"action":"promote_ops"}`, user.Id), common.RoleOpsUser)
		assert.Contains(t, recorder.Body.String(), `"success":false`)
		assert.Contains(t, recorder.Body.String(), i18n.MsgUserAdminCannotPromote)
		var updated model.User
		require.NoError(t, db.First(&updated, user.Id).Error)
		assert.Equal(t, common.RoleCommonUser, updated.Role)
	})

	t.Run("already ops or higher rejected", func(t *testing.T) {
		ops := model.User{
			Username: "promote-ops-already", Password: "password", Role: common.RoleOpsUser,
			Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "promote-ops-already-aff",
		}
		require.NoError(t, db.Create(&ops).Error)
		recorder := performManageUserRequest(t, fmt.Sprintf(`{"id":%d,"action":"promote_ops"}`, ops.Id))
		assert.Contains(t, recorder.Body.String(), `"success":false`)
		assert.Contains(t, recorder.Body.String(), i18n.MsgOpsUserAlreadyOpsOrHigher)

		admin := model.User{
			Username: "promote-ops-admin-target", Password: "password", Role: common.RoleAdminUser,
			Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "promote-ops-admin-target-aff",
		}
		require.NoError(t, db.Create(&admin).Error)
		recorder = performManageUserRequest(t, fmt.Sprintf(`{"id":%d,"action":"promote_ops"}`, admin.Id))
		assert.Contains(t, recorder.Body.String(), `"success":false`)
	})
}

func TestManageUserDemoteOps(t *testing.T) {
	db := setupManageUserTestDB(t)

	t.Run("admin demotes ops to common", func(t *testing.T) {
		user := model.User{
			Username: "demote-ops-admin", Password: "password", Role: common.RoleOpsUser,
			Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "demote-ops-admin-aff",
		}
		require.NoError(t, db.Create(&user).Error)
		recorder := performManageUserRequestWithRole(t, fmt.Sprintf(`{"id":%d,"action":"demote_ops"}`, user.Id), common.RoleAdminUser)
		assert.Contains(t, recorder.Body.String(), `"success":true`)
		var updated model.User
		require.NoError(t, db.First(&updated, user.Id).Error)
		assert.Equal(t, common.RoleCommonUser, updated.Role)
	})

	t.Run("root demotes ops to common", func(t *testing.T) {
		user := model.User{
			Username: "demote-ops-root", Password: "password", Role: common.RoleOpsUser,
			Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "demote-ops-root-aff",
		}
		require.NoError(t, db.Create(&user).Error)
		recorder := performManageUserRequest(t, fmt.Sprintf(`{"id":%d,"action":"demote_ops"}`, user.Id))
		assert.Contains(t, recorder.Body.String(), `"success":true`)
		var updated model.User
		require.NoError(t, db.First(&updated, user.Id).Error)
		assert.Equal(t, common.RoleCommonUser, updated.Role)
	})

	t.Run("non-ops user rejected", func(t *testing.T) {
		user := model.User{
			Username: "demote-ops-not-ops", Password: "password", Role: common.RoleCommonUser,
			Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "demote-ops-not-ops-aff",
		}
		require.NoError(t, db.Create(&user).Error)
		recorder := performManageUserRequest(t, fmt.Sprintf(`{"id":%d,"action":"demote_ops"}`, user.Id))
		assert.Contains(t, recorder.Body.String(), `"success":false`)
		assert.Contains(t, recorder.Body.String(), i18n.MsgOpsUserNotOps)
	})

	t.Run("admin cannot demote a peer admin", func(t *testing.T) {
		admin := model.User{
			Username: "demote-ops-peer", Password: "password", Role: common.RoleAdminUser,
			Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "demote-ops-peer-aff",
		}
		require.NoError(t, db.Create(&admin).Error)
		recorder := performManageUserRequestWithRole(t, fmt.Sprintf(`{"id":%d,"action":"demote_ops"}`, admin.Id), common.RoleAdminUser)
		assert.Contains(t, recorder.Body.String(), `"success":false`)
		assert.Contains(t, recorder.Body.String(), i18n.MsgUserNoPermissionHigherLevel)
	})
}
