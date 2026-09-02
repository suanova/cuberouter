package plugins_test

import (
	"testing"

	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registerViduPlugin 加载内嵌 vidu 插件引擎(独立于 videoResponsesProtocol helper)。
func registerViduPlugin(t *testing.T) *jsplugin.LoadedPlugin {
	t.Helper()
	source, err := builtinplugins.Source("vidu")
	require.NoError(t, err)
	registry := jsplugin.NewRegistry()
	plugin, err := registry.RegisterFactory(source, jsplugin.Options{Key: "vidu"})
	require.NoError(t, err)
	return plugin
}

func TestViduBuildSubmitRequestQ3DefaultResolution(t *testing.T) {
	// Q3 系列未传 size 时,默认分辨率按文档为 720p(而不是 1080p)
	plugin := registerViduPlugin(t)
	for _, model := range []string{"viduq3-turbo", "viduq3", "viduq3-mix"} {
		value, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", map[string]any{
			"baseUrl":       "https://api.vidu.cn",
			"apiKey":        "k",
			"upstreamModel": model,
			"requestBody": map[string]any{
				"model":  model,
				"prompt": "test",
			},
		})
		require.NoError(t, err, model)
		descriptor, ok := value.(map[string]any)
		require.True(t, ok, model)
		body, ok := descriptor["body"].(map[string]any)
		require.True(t, ok, model)
		assert.Equal(t, "720p", body["resolution"], model)
	}
}

func TestViduRejectsQ2ProTextToVideo(t *testing.T) {
	// viduq2-pro 是参考/视频编辑类模型,不支持文生视频;
	// 能力矩阵校验在 decodeRequest 路径执行
	plugin := registerViduPlugin(t)
	_, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_responses", "decodeRequest"}, map[string]any{
		"body": map[string]any{
			"kind":  "json",
			"value": map[string]any{"model": "viduq2-pro", "input": "text only"},
		},
		"model": "viduq2-pro",
	})
	require.ErrorContains(t, err, "viduq2-pro does not support text-to-video")
}

func TestViduAllowsQ2ProImageToVideo(t *testing.T) {
	// viduq2-pro 支持图生视频(1 张图)
	plugin := registerViduPlugin(t)
	value, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_responses", "decodeRequest"}, map[string]any{
		"body": map[string]any{
			"kind": "json",
			"value": map[string]any{
				"model": "viduq2-pro",
				"input": []any{map[string]any{"role": "user", "content": []any{
					map[string]any{"type": "input_text", "text": "image to video"},
					map[string]any{"type": "input_image", "image_url": "https://cdn.example/a.png"},
				}}},
			},
		},
		"model": "viduq2-pro",
	})
	require.NoError(t, err)
	descriptor, ok := value.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "image_to_video", descriptor["action"])
}
