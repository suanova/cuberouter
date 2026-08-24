package controller

import (
	"bytes"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupPluginTestDB swaps in an in-memory SQLite database with the tables the
// plugin handlers and channel selection touch, and forces Redis off so user
// caches fall through to the database.
func setupPluginTestDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Plugin{}, &model.Channel{}, &model.Ability{}, &model.User{}))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
	})
}

func putPluginContext(body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/plugin/", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

// TestUpdatePluginPreservesServerOwnedFields: a full-object save from an edit
// payload must not zero created_time, and must keep the cached skill content
// when the skill source is unchanged (regression: edits used to wipe both).
// A changed skill source must still wipe the cached skill for re-fetch.
func TestUpdatePluginPreservesServerOwnedFields(t *testing.T) {
	setupPluginTestDB(t)
	original := &model.Plugin{
		Name:           "Echo",
		Slug:           "echo",
		Description:    "d",
		Enabled:        true,
		McpUrl:         "http://127.0.0.1:9101/mcp",
		SkillSource:    "http://127.0.0.1:9102/skill.md",
		SkillContent:   "# Echo Skill",
		SkillFetchedAt: 111,
		CreatedTime:    999,
		UpdatedTime:    1000,
	}
	require.NoError(t, model.DB.Create(original).Error)

	// Edit payload carries no created_time / skill_content, mimicking the
	// admin drawer's updatePlugin({ ...values, id }).
	c, w := putPluginContext(fmt.Sprintf(
		`{"id":%d,"name":"Echo v2","description":"d2","enabled":true,"mcp_url":"http://127.0.0.1:9101/mcp","skill_source":"http://127.0.0.1:9102/skill.md"}`,
		original.Id,
	))
	UpdatePlugin(c)

	require.Equal(t, 200, w.Code)
	var resp struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)

	stored, err := model.GetPluginByID(original.Id)
	require.NoError(t, err)
	assert.Equal(t, "Echo v2", stored.Name)
	assert.Equal(t, int64(999), stored.CreatedTime)
	assert.Equal(t, "# Echo Skill", stored.SkillContent)
	assert.Equal(t, int64(111), stored.SkillFetchedAt)

	// A changed skill source must wipe the cached skill so it is re-fetched.
	c2, w2 := putPluginContext(fmt.Sprintf(
		`{"id":%d,"name":"Echo v2","description":"d2","enabled":true,"mcp_url":"http://127.0.0.1:9101/mcp","skill_source":"http://127.0.0.1:9102/other.md"}`,
		original.Id,
	))
	UpdatePlugin(c2)

	require.Equal(t, 200, w2.Code)
	stored, err = model.GetPluginByID(original.Id)
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:9102/other.md", stored.SkillSource)
	assert.Equal(t, int64(999), stored.CreatedTime)
	assert.Empty(t, stored.SkillContent)
	assert.Equal(t, int64(0), stored.SkillFetchedAt)
}

// TestSelectPluginRoundChannel: the first relay attempt inside a plugin round
// must run against a Distribute-equivalent pre-selected channel. Before the
// fix, round 0 ran with no channel context and failed with an empty upstream
// URL (`Post "/v1/chat/completions": unsupported protocol scheme ""`).
func TestSelectPluginRoundChannel(t *testing.T) {
	setupPluginTestDB(t)

	baseURL := "http://127.0.0.1:9102"
	channel := &model.Channel{
		Type:    constant.ChannelTypeOpenAI,
		Key:     "sk-mock",
		Name:    "mock-openai",
		BaseURL: &baseURL,
		Models:  "mock-model",
		Group:   "default",
		Status:  common.ChannelStatusEnabled,
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group:     "default",
		Model:     "mock-model",
		ChannelId: channel.Id,
		Enabled:   true,
	}).Error)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	subCtx, _ := gin.CreateTestContext(w)
	common.SetContextKey(subCtx, constant.ContextKeyUserGroup, "default")

	req := &dto.GeneralOpenAIRequest{
		Model:    "mock-model",
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
	}
	apiErr := selectPluginRoundChannel(subCtx, req, "default")
	require.Nil(t, apiErr)

	// The invariant under test: the sub-context now carries the same channel
	// keys middleware.Distribute would seed on the real /pg route.
	assert.Equal(t, channel.Id, common.GetContextKeyInt(subCtx, constant.ContextKeyChannelId))
	assert.Equal(t, baseURL, common.GetContextKeyString(subCtx, constant.ContextKeyChannelBaseUrl))
	assert.Equal(t, "sk-mock", common.GetContextKeyString(subCtx, constant.ContextKeyChannelKey))
	assert.Equal(t, constant.ChannelTypeOpenAI, common.GetContextKeyInt(subCtx, constant.ContextKeyChannelType))

	// Unknown model must surface a selection error instead of falling through
	// to an empty-channel first attempt.
	subCtx2, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(subCtx2, constant.ContextKeyUserGroup, "default")
	req2 := &dto.GeneralOpenAIRequest{Model: "missing-model"}
	apiErr2 := selectPluginRoundChannel(subCtx2, req2, "default")
	require.NotNil(t, apiErr2)
	assert.Equal(t, 0, common.GetContextKeyInt(subCtx2, constant.ContextKeyChannelId))
}
