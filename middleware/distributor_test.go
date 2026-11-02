package middleware

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelMatchesExpectedTaskPluginUsesGenericChannelSetting(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeTaskPlugin}
	channel.SetSetting(dto.ChannelSettings{TaskPluginKey: "generic-alpha"})

	assert.True(t, channelMatchesExpectedTaskPlugin(nil, channel, "generic-alpha"))
	assert.False(t, channelMatchesExpectedTaskPlugin(nil, channel, "generic-beta"))
	assert.False(t, channelMatchesExpectedTaskPlugin(nil, channel, ""))
}

func TestChannelMatchesExpectedTaskPluginUsesPinnedLegacyIndex(t *testing.T) {
	registry := jsplugin.NewRegistry()
	alpha, err := registry.Register(distributorTaskPluginSource("legacy-alpha", constant.ChannelTypeKling), jsplugin.Options{})
	require.NoError(t, err)
	pinnedGeneration := registry.Generation()

	require.NoError(t, registry.Unregister("legacy-alpha"))
	_, err = registry.Register(distributorTaskPluginSource("legacy-beta", constant.ChannelTypeKling), jsplugin.Options{})
	require.NoError(t, err)

	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{
		Generation: pinnedGeneration,
		Plugin:     alpha,
	})
	channel := &model.Channel{Type: constant.ChannelTypeKling}

	assert.True(t, channelMatchesExpectedTaskPlugin(c, channel, "legacy-alpha"))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, channel, "legacy-beta"))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeJimeng}, "legacy-alpha"))
}

func TestChannelMatchesExpectedTaskPluginRejectsUnindexedLegacyChannel(t *testing.T) {
	registry := jsplugin.NewRegistry()
	plugin, err := registry.Register(distributorTaskPluginSource("legacy-alpha", constant.ChannelTypeKling), jsplugin.Options{})
	require.NoError(t, err)

	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{
		Generation: registry.Generation(),
		Plugin:     plugin,
	})

	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeJimeng}, "legacy-alpha"))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: 0}, "legacy-alpha"))
	assert.True(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeJimeng}, ""))
	assert.False(t, channelMatchesExpectedTaskPlugin(nil, &model.Channel{Type: constant.ChannelTypeKling}, "legacy-alpha"))

	c.Set("expected_task_plugin_key", "legacy-alpha")
	setupErr := SetupContextForSelectedChannel(c, &model.Channel{Type: constant.ChannelTypeJimeng}, "task-model")
	require.NotNil(t, setupErr)
	assert.Contains(t, setupErr.Error(), "does not match")
}

func TestSharedEndpointRebindsToSelectedLegacyProvider(t *testing.T) {
	registry := jsplugin.NewRegistry()
	_, err := registry.Register(distributorEndpointPluginSource("gemini-shared", constant.ChannelTypeGemini), jsplugin.Options{})
	require.NoError(t, err)
	_, err = registry.Register(distributorEndpointPluginSource("vertex-shared", constant.ChannelTypeVertexAi), jsplugin.Options{})
	require.NoError(t, err)
	candidates := registry.Generation().LookupEndpointCandidates("POST", "/v1/responses", "task-model")
	require.Len(t, candidates, 2)

	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Generation: registry.Generation(), Plugin: candidates[0].Plugin})
	c.Set(jsplugin.ContextKeyPinnedEndpoint, jsplugin.PinnedEndpoint{
		Generation: registry.Generation(),
		Plugin:     candidates[0].Plugin,
		Protocol:   candidates[0].Protocol,
		Operation:  candidates[0].Operation,
		Model:      "task-model",
		Candidates: candidates,
	})
	c.Set("expected_task_plugin_key", candidates[0].Plugin.Meta.Key)

	geminiChannel := &model.Channel{Id: 1, Type: constant.ChannelTypeGemini}
	vertexChannel := &model.Channel{Id: 2, Type: constant.ChannelTypeVertexAi}
	assert.True(t, channelMatchesExpectedTaskPlugin(c, geminiChannel, candidates[0].Plugin.Meta.Key))
	assert.True(t, channelMatchesExpectedTaskPlugin(c, vertexChannel, candidates[0].Plugin.Meta.Key))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeKling}, candidates[0].Plugin.Meta.Key))

	require.Nil(t, SetupContextForSelectedChannel(c, vertexChannel, "task-model"))
	pinnedValue, exists := c.Get(jsplugin.ContextKeyPinnedEndpoint)
	require.True(t, exists)
	pinned, ok := pinnedValue.(jsplugin.PinnedEndpoint)
	require.True(t, ok)
	assert.Equal(t, "vertex-shared", pinned.Plugin.Meta.Key)
	assert.Equal(t, "vertex-shared", c.GetString("expected_task_plugin_key"))
	assert.Equal(t, "vertex-shared", c.GetString("task_plugin_key"))
	assert.True(t, channelMatchesExpectedTaskPlugin(c, geminiChannel, "vertex-shared"), "a retry may select another declared provider")
}

func distributorTaskPluginSource(key string, channelType int) string {
	return fmt.Sprintf(`
export const meta = {
  apiVersion: 1,
  key: %q,
  name: %q,
  version: "1.0.0",
  author: {name: "Test"},
  channelTypes: [%d],
  models: ["task-model"],
  fetchMode: "per_task",
};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {taskId: "task"}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {status: "SUCCESS"}; }
`, key, key, channelType)
}

func distributorEndpointPluginSource(key string, channelType int) string {
	return fmt.Sprintf(`
export const meta = {
  apiVersion: 1,
  key: %q,
  name: %q,
  version: "1.0.0",
  author: {name: "Test"},
  channelTypes: [%d],
  models: ["task-model"],
  fetchMode: "per_task",
  protocols: [{name: "openai_responses", supports: ["stream", "sync", "background"]}],
};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {taskId: "task"}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {status: "SUCCESS"}; }
export const protocols = {openai_responses: {
  decodeRequest: function(ctx) { return {kind: "submit", model: "task-model", requestBody: ctx.body.value}; },
  renderEvents: function() { return {events: [], state: null, done: false}; },
  renderFinal: function() { return {output: []}; },
}};
`, key, key, channelType)
}

// TestGetModelRequestPgVideoGenerations 保护多媒体 studio 视频 tab 会话入口的
// 渠道路由契约：POST /pg/video/generations 从统一任务体取模型参与渠道选择；
// GET /pg/video/generations/:task_id 按 task_id 查询，不做模型渠道选择（回归时
// 轮询会拿到 "Model name is required" 400，视频结果永远查不出来）。
func TestGetModelRequestPgVideoGenerations(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newContext := func(method, target, body string) *gin.Context {
		request := httptest.NewRequest(method, target, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = request
		return c
	}

	t.Run("submit reads the model from the unified task body", func(t *testing.T) {
		c := newContext(http.MethodPost, "/pg/video/generations", `{"model":"viduq3-pro","prompt":"a cat"}`)

		modelRequest, shouldSelectChannel, err := getModelRequest(c)
		require.NoError(t, err)
		require.True(t, shouldSelectChannel)
		assert.Equal(t, "viduq3-pro", modelRequest.Model)
		assert.Equal(t, relayconstant.RelayModeVideoSubmit, c.GetInt("relay_mode"))
	})

	t.Run("fetch by task id skips channel selection", func(t *testing.T) {
		c := newContext(http.MethodGet, "/pg/video/generations/task-123", "")

		_, shouldSelectChannel, err := getModelRequest(c)
		require.NoError(t, err)
		assert.False(t, shouldSelectChannel)
		assert.Equal(t, relayconstant.RelayModeVideoFetchByID, c.GetInt("relay_mode"))
	})

	t.Run("the v1 video submit path keeps the same behavior", func(t *testing.T) {
		c := newContext(http.MethodPost, "/v1/video/generations", `{"model":"kling-v2","prompt":"a cat"}`)

		modelRequest, shouldSelectChannel, err := getModelRequest(c)
		require.NoError(t, err)
		require.True(t, shouldSelectChannel)
		assert.Equal(t, "kling-v2", modelRequest.Model)
		assert.Equal(t, relayconstant.RelayModeVideoSubmit, c.GetInt("relay_mode"))
	})
}

// TestGetModelRequestPgImagesEditsMultipart 保护图生图 multipart 上报的渠道路由契约：
// 前端把参考图/蒙版作为 file、标量走 multipart 表单字段直发 /pg/images/edits（不走
// type-63），发行器必须能从 multipart 表单里读出 model 参与渠道选择；否则 model 为空，
// Distribute 会以 "model name cannot be empty"(distributor.model_name_required) 400 拒绝。
func TestGetModelRequestPgImagesEditsMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)

	makeMultipart := func(model string) (string, string) {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		_ = w.WriteField("model", model)
		_ = w.WriteField("prompt", "a blue coat")
		part, _ := w.CreateFormFile("image", "reference.png")
		_, _ = part.Write([]byte("PNG"))
		_ = w.Close()
		return buf.String(), w.FormDataContentType()
	}

	t.Run("multipart /pg/images/edits yields the model", func(t *testing.T) {
		body, ct := makeMultipart("Qwen-Image-2.1")
		req := httptest.NewRequest(http.MethodPost, "/pg/images/edits", strings.NewReader(body))
		req.Header.Set("Content-Type", ct)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = req

		modelRequest, shouldSelectChannel, err := getModelRequest(c)
		require.NoError(t, err)
		require.True(t, shouldSelectChannel)
		assert.Equal(t, "Qwen-Image-2.1", modelRequest.Model)
	})

	t.Run("multipart doesn't consume the body for later replay", func(t *testing.T) {
		body, ct := makeMultipart("Qwen-Image-2.1")
		req := httptest.NewRequest(http.MethodPost, "/pg/images/edits", strings.NewReader(body))
		req.Header.Set("Content-Type", ct)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = req

		_, _, err := getModelRequest(c)
		require.NoError(t, err)

		// 读完后请求体仍是可重放的完整 multipart（参考图文件 part 保留）。
		storage, sErr := common.GetBodyStorage(c)
		require.NoError(t, sErr)
		rest, sErr := storage.Bytes()
		require.NoError(t, sErr)
		assert.Contains(t, string(rest), "Qwen-Image-2.1")
		assert.Contains(t, string(rest), "PNG")
	})

	t.Run("JSON body edit keeps reading model via json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/pg/images/edits",
			strings.NewReader(`{"model":"Qwen-Image-2.1","prompt":"a blue coat"}`))
		req.Header.Set("Content-Type", "application/json")
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = req

		modelRequest, shouldSelectChannel, err := getModelRequest(c)
		require.NoError(t, err)
		require.True(t, shouldSelectChannel)
		assert.Equal(t, "Qwen-Image-2.1", modelRequest.Model)
	})
}
