package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListModelsSupportsOpenAIAndGeminiAuthentication(t *testing.T) {
	setupRelayRouterTestDB(t)

	user := model.User{
		Username: "models-user",
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    100,
	}
	require.NoError(t, model.DB.Create(&user).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		UserId:         user.Id,
		Key:            "modelstestkey",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	engine := gin.New()
	SetRelayRouter(engine)

	tests := []struct {
		name           string
		path           string
		headerName     string
		expectedObject string
		expectedField  string
	}{
		{
			name:           "OpenAI bearer token",
			path:           "/v1/models",
			headerName:     "Authorization",
			expectedObject: "list",
			expectedField:  "data",
		},
		{
			name:          "Gemini API key header",
			path:          "/v1/models",
			headerName:    "x-goog-api-key",
			expectedField: "models",
		},
		{
			name:          "Gemini API key query",
			path:          "/v1/models?key=modelstestkey",
			expectedField: "models",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			if test.headerName != "" {
				value := "modelstestkey"
				if test.headerName == "Authorization" {
					value = "Bearer " + value
				}
				request.Header.Set(test.headerName, value)
			}

			engine.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusOK, recorder.Code)
			var payload map[string]any
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
			assert.Contains(t, payload, test.expectedField)
			assert.NotContains(t, payload, "error")
			if test.expectedObject != "" {
				assert.Equal(t, test.expectedObject, payload["object"])
			}
		})
	}
}

func TestRelayRouterRegistersClaudeTokenCountingEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetRelayRouter(engine)

	for _, route := range engine.Routes() {
		if route.Method == http.MethodPost && route.Path == "/v1/messages/count_tokens" {
			return
		}
	}

	t.Fatal("POST /v1/messages/count_tokens route is not registered")
}

func setupRelayRouterTestDB(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	originalIsMasterNode := common.IsMasterNode
	originalRedisEnabled := common.RedisEnabled
	originalSQLitePath := common.SQLitePath
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")

	common.IsMasterNode = false
	common.RedisEnabled = false
	common.SQLitePath = fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, os.Setenv("SQL_DSN", "local"))
	require.NoError(t, model.InitDB())
	model.LOG_DB = model.DB
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.Token{}, &model.Ability{}, &model.Organization{}, &model.OrganizationMember{}, &model.UserSubscription{}, &model.UserSession{}))

	t.Cleanup(func() {
		if sqlDB, err := model.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		common.IsMasterNode = originalIsMasterNode
		common.RedisEnabled = originalRedisEnabled
		common.SQLitePath = originalSQLitePath
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		if hadSQLDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalSQLDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	})
}

// TestPgVideoRoutesRegisterWithoutConflict 保护多媒体 studio 视频 tab 的会话鉴权
// 入口注册：/pg/video 是独立路由组（AllowTaskPluginChannel 标记必须先于组级
// Distribute 生效，因此不能嵌套进 /pg 组），提交与查询两条路由缺一不可。
func TestPgVideoRoutesRegisterWithoutConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	require.NotPanics(t, func() {
		SetRelayRouter(engine)
	})

	var pgVideoPaths []string
	for _, r := range engine.Routes() {
		if strings.HasPrefix(r.Path, "/pg/video") {
			pgVideoPaths = append(pgVideoPaths, r.Method+" "+r.Path)
		}
	}
	require.ElementsMatch(t, []string{
		"POST /pg/video/generations",
		"GET /pg/video/generations/:task_id",
	}, pgVideoPaths)
}

// seedPlaygroundOrgFallbackFixture 造一个「钱包耗尽 + 组织可用」的会话用户，
// 返回其登录 access token 与组织分组名。
func seedPlaygroundOrgFallbackFixture(t *testing.T, userGroup, orgGroup string, userQuota, orgQuota int) string {
	t.Helper()
	user := model.User{
		Username:    "pg-fb-" + userGroup,
		DisplayName: "pg-fb-" + userGroup,
		Status:      common.UserStatusEnabled,
		Group:       userGroup,
		Quota:       userQuota,
		AuthVersion: 1,
		AffCode:     "pgfb-" + userGroup,
	}
	require.NoError(t, model.DB.Create(&user).Error)
	organization := &model.Organization{
		Name:        "org-" + orgGroup,
		Slug:        "org-" + orgGroup,
		Group:       orgGroup,
		Status:      model.OrganizationStatusActive,
		Quota:       orgQuota,
		OwnerUserId: user.Id,
		CreatedBy:   user.Id,
	}
	require.NoError(t, model.DB.Create(organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: organization.Id,
		UserId:         user.Id,
		Role:           model.OrganizationRoleOwner,
		Status:         model.OrganizationMemberStatusActive,
	}).Error)
	bundle, err := service.CreateLoginSession(user.Id, "test", "", "")
	require.NoError(t, err)
	return bundle.AccessToken
}

func postPlaygroundChatCompletion(t *testing.T, engine *gin.Engine, accessToken string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(`{"model":"pg-fb-model","messages":[{"role":"user","content":"hi"}]}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	engine.ServeHTTP(recorder, request)
	return recorder
}

// TestPlaygroundOrganizationFallbackSwitchesGroupBeforeDistribute 用真实中间件链
// 验证顺序：会话鉴权通过后、渠道选择之前，钱包耗尽用户的 usingGroup 已切到组织
// 分组（Distribute 的「无可用渠道」报错里带的就是这个分组名）。
// 顺序若错（兜底在 UserAuth 前拿不到用户、在 Distribute 后才生效），报错里只会
// 出现个人分组。
func TestPlaygroundOrganizationFallbackSwitchesGroupBeforeDistribute(t *testing.T) {
	// 断言对象是 Distribute 经 i18n.T 渲染的报错文本，而 i18n bundle 只在
	// main.go 初始化，测试须自行初始化（sync.Once，与 controller 测试同规）。
	require.NoError(t, i18n.Init())
	setupRelayRouterTestDB(t)
	accessToken := seedPlaygroundOrgFallbackFixture(t, "pg-fb-user", "pg-fb-org", 0, 100000)

	engine := gin.New()
	SetRelayRouter(engine)
	recorder := postPlaygroundChatCompletion(t, engine, accessToken)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "pg-fb-org")
	assert.NotContains(t, recorder.Body.String(), "pg-fb-user")
}

func TestPlaygroundOrganizationFallbackKeepsPersonalGroupAboveThreshold(t *testing.T) {
	// 同上：报错文本由 i18n.T 渲染，bundle 需先初始化。
	require.NoError(t, i18n.Init())
	setupRelayRouterTestDB(t)
	accessToken := seedPlaygroundOrgFallbackFixture(t, "pg-fb-user", "pg-fb-org", 1000000, 100000)

	engine := gin.New()
	SetRelayRouter(engine)
	recorder := postPlaygroundChatCompletion(t, engine, accessToken)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "pg-fb-user")
	assert.NotContains(t, recorder.Body.String(), "pg-fb-org")
}
