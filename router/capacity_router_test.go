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

// TestCapacityMetricsRouteRegistered 断言 /api/capacity-metrics 随 registerApiRoutes
// 挂到三个共享前缀（/api、/api/v1、/api/v2），防前缀漂移与路由拼写错误。
func TestCapacityMetricsRouteRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, prefix := range []string{"/api", "/api/v1", "/api/v2"} {
		assert.Containsf(t, routes, http.MethodGet+" "+prefix+"/capacity-metrics",
			"capacity-metrics 路由缺失: GET %s/capacity-metrics", prefix)
	}
}

// TestCapacityMetricsRequiresAdmin 走真实 HTTP 链断言 AdminAuth 门禁：
// 无凭证 401、普通用户 PAT 403、管理员 PAT 放行到 handler（200 + 空 series）。
// PAT 为 User.access_token 上的不透明凭据（model.ValidateAccessToken 路径）。
func TestCapacityMetricsRequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	previousDB := model.DB
	previousDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousSQLitePath := common.SQLitePath
	previousMasterNode := common.IsMasterNode
	previousRedisEnabled := common.RedisEnabled
	common.SQLitePath = t.TempDir() + "/router-capacity.db"
	common.IsMasterNode = false
	common.RedisEnabled = false
	t.Setenv("SQL_DSN", "")
	require.NoError(t, model.InitDB())
	database := model.DB
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.CapacityMetric{}))
	t.Cleanup(func() {
		sqlDB, closeErr := database.DB()
		require.NoError(t, closeErr)
		require.NoError(t, sqlDB.Close())
		model.DB = previousDB
		common.SetDatabaseTypes(previousDatabaseType, previousLogDatabaseType)
		common.SQLitePath = previousSQLitePath
		common.IsMasterNode = previousMasterNode
		common.RedisEnabled = previousRedisEnabled
	})

	adminPAT := "cap-admin-pat-01"
	userPAT := "cap-user-pat-0001"
	require.NoError(t, database.Create(&model.User{
		Username:    "cap-admin",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "capadmin01",
		AccessToken: &adminPAT,
	}).Error)
	require.NoError(t, database.Create(&model.User{
		Username:    "cap-user",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "capuser0001",
		AccessToken: &userPAT,
	}).Error)

	engine := gin.New()
	SetApiRouter(engine)

	testCases := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{name: "missing credential rejected", wantStatus: http.StatusUnauthorized},
		{name: "common user forbidden", authHeader: "Bearer " + userPAT, wantStatus: http.StatusForbidden},
		{name: "admin allowed", authHeader: "Bearer " + adminPAT, wantStatus: http.StatusOK},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/capacity-metrics", nil)
			if tc.authHeader != "" {
				request.Header.Set("Authorization", tc.authHeader)
			}
			engine.ServeHTTP(recorder, request)
			require.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			if tc.wantStatus == http.StatusOK {
				var payload map[string]any
				require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
				assert.Equal(t, true, payload["success"])
				data, ok := payload["data"].(map[string]any)
				require.True(t, ok, "data object missing")
				series, ok := data["series"].([]any)
				require.True(t, ok, "series array missing")
				assert.Empty(t, series)
			}
		})
	}
}
