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
package plugins_test

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const dshBaseURL = "http://127.0.0.1:3080"

func loadDSHPlugin(t *testing.T) *jsplugin.LoadedPlugin {
	t.Helper()
	source, err := builtinplugins.Source("dsh")
	require.NoError(t, err)
	registry := jsplugin.NewRegistry()
	plugin, err := registry.RegisterFactory(source, jsplugin.Options{Key: "dsh"})
	require.NoError(t, err)
	return plugin
}

func dshSubmitContext(requestBody map[string]any) map[string]any {
	return map[string]any{
		"baseUrl":       dshBaseURL,
		"apiKey":        "dsh-api-demo-key",
		"model":         "dsh",
		"upstreamModel": "dsh",
		"action":        "",
		"requestBody":   requestBody,
	}
}

func TestDSHTaskPluginMetadata(t *testing.T) {
	plugin := loadDSHPlugin(t)
	assert.Equal(t, 1, plugin.Meta.APIVersion)
	assert.Equal(t, "dsh", plugin.Meta.Key)
	assert.Equal(t, []string{"dsh"}, plugin.Meta.Models)
	assert.Equal(t, "per_task", plugin.Meta.FetchMode)
	assert.Empty(t, plugin.Meta.ChannelTypes)
	assert.Empty(t, plugin.Meta.Protocols)
	assert.Empty(t, plugin.Meta.Routes)
}

func TestDSHBuildSubmitRequest(t *testing.T) {
	plugin := loadDSHPlugin(t)

	t.Run("builds a first-turn submit without session fields", func(t *testing.T) {
		value, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", dshSubmitContext(map[string]any{
			"model":  "dsh",
			"prompt": "列一下当前目录",
			"preset": "standard",
		}))
		require.NoError(t, err)
		descriptor := decodePluginValue(t, value)

		assert.Equal(t, dshBaseURL+"/api/v1/tasks", descriptor["url"])
		assert.Equal(t, "POST", descriptor["method"])
		assert.Equal(t, "agent", descriptor["action"])
		assert.Equal(t, map[string]any{
			"X-API-Key":    "dsh-api-demo-key",
			"Content-Type": "application/json",
			"Accept":       "application/json",
		}, descriptor["headers"])
		// 网关模型名不是 harness 的模型 id，不能透传成 dsh 的 model 覆盖项。
		assert.Equal(t, map[string]any{
			"prompt": "列一下当前目录",
			"preset": "standard",
		}, descriptor["body"])
	})

	t.Run("continues a session and passes supported options through", func(t *testing.T) {
		value, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", dshSubmitContext(map[string]any{
			"model":     "dsh",
			"prompt":    "继续",
			"sessionId": "api-1750000000-abc",
			"preset":    "code",
			"cwd":       "/workspace",
			"maxTokens": float64(4096),
			"timeoutMs": float64(120000),
			"system":    "be brief",
			"unrelated": "dropped",
		}))
		require.NoError(t, err)
		descriptor := decodePluginValue(t, value)
		assert.Equal(t, map[string]any{
			"prompt":    "继续",
			"sessionId": "api-1750000000-abc",
			"preset":    "code",
			"cwd":       "/workspace",
			"maxTokens": float64(4096),
			"timeoutMs": float64(120000),
			"system":    "be brief",
		}, descriptor["body"])
	})

	t.Run("appends uploaded files to the prompt as a readable manifest", func(t *testing.T) {
		value, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", dshSubmitContext(map[string]any{
			"model":  "dsh",
			"prompt": "总结这两份材料",
			"attachments": []any{
				map[string]any{"name": "报告.pdf", "path": "/data/uploads/7/ab12-报告.pdf"},
				map[string]any{"name": "photo.png", "path": "/data/uploads/7/cd34-photo.png", "size": float64(1024)},
			},
		}))
		require.NoError(t, err)
		descriptor := decodePluginValue(t, value)

		// dsh-api 不认识附件字段，清单只能进提示词；原始 attachments 不能透传。
		assert.Equal(t, map[string]any{
			"prompt": "总结这两份材料\n\n[Uploaded files]\n" +
				"The user attached these files; they are readable on disk:\n" +
				"- /data/uploads/7/ab12-报告.pdf (报告.pdf)\n" +
				"- /data/uploads/7/cd34-photo.png (photo.png)",
		}, descriptor["body"])
	})

	t.Run("drops malformed attachment entries", func(t *testing.T) {
		value, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", dshSubmitContext(map[string]any{
			"model":  "dsh",
			"prompt": "hi",
			"attachments": []any{
				"not-an-object",
				map[string]any{"path": "relative/path.pdf", "name": "relative.pdf"},
				map[string]any{"path": "/data/uploads/7/line\nbreak.pdf", "name": "break.pdf"},
				map[string]any{"name": "no-path.pdf"},
			},
		}))
		require.NoError(t, err)
		descriptor := decodePluginValue(t, value)
		assert.Equal(t, map[string]any{"prompt": "hi"}, descriptor["body"])
	})

	t.Run("normalizes a trailing slash in the channel base URL", func(t *testing.T) {
		ctx := dshSubmitContext(map[string]any{"model": "dsh", "prompt": "hi"})
		ctx["baseUrl"] = dshBaseURL + "///"
		value, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", ctx)
		require.NoError(t, err)
		descriptor := decodePluginValue(t, value)
		assert.Equal(t, dshBaseURL+"/api/v1/tasks", descriptor["url"])
	})

	t.Run("omits the auth header when the channel has no key", func(t *testing.T) {
		ctx := dshSubmitContext(map[string]any{"model": "dsh", "prompt": "hi"})
		ctx["apiKey"] = ""
		value, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", ctx)
		require.NoError(t, err)
		descriptor := decodePluginValue(t, value)
		headers := descriptor["headers"].(map[string]any)
		assert.NotContains(t, headers, "X-API-Key")
	})

	t.Run("rejects a missing prompt", func(t *testing.T) {
		_, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", dshSubmitContext(map[string]any{
			"model":  "dsh",
			"prompt": "   ",
		}))
		require.ErrorContains(t, err, "prompt is required")
	})

	t.Run("rejects a non-string session id", func(t *testing.T) {
		_, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", dshSubmitContext(map[string]any{
			"model":     "dsh",
			"prompt":    "hi",
			"sessionId": float64(1),
		}))
		require.ErrorContains(t, err, "sessionId must be a string")
	})
}

func TestDSHParseSubmitResponse(t *testing.T) {
	plugin := loadDSHPlugin(t)
	submitContext := dshSubmitContext(map[string]any{"model": "dsh", "prompt": "hi"})

	t.Run("returns the upstream task id and session state", func(t *testing.T) {
		body := map[string]any{
			"ok": true,
			"data": map[string]any{
				"taskId":    "task-1",
				"sessionId": "api-1750000000-abc",
				"status":    "running",
			},
		}
		value, err := plugin.Engine.Call(t.Context(), "parseSubmitResponse", submitContext, map[string]any{
			"statusCode": 202,
			"headers":    map[string]any{},
			"body":       body,
		})
		require.NoError(t, err)
		parsed := decodePluginValue(t, value)

		assert.Equal(t, "task-1", parsed["taskId"])
		assert.Equal(t, body, parsed["taskData"])
		// 完整回执保留在 taskData：首次轮询覆盖之前，页面据此拿到 sessionId。
		assert.Equal(t, map[string]any{"sessionId": "api-1750000000-abc"}, parsed["state"])
	})

	t.Run("surfaces the dsh error code and message", func(t *testing.T) {
		_, err := plugin.Engine.Call(t.Context(), "parseSubmitResponse", submitContext, map[string]any{
			"statusCode": 409,
			"headers":    map[string]any{},
			"body": map[string]any{
				"ok":    false,
				"error": map[string]any{"code": "preset_conflict", "message": "preset cannot change mid-session"},
			},
		})
		require.ErrorContains(t, err, "dsh preset_conflict: preset cannot change mid-session")
	})

	t.Run("rejects a successful envelope without a task id", func(t *testing.T) {
		_, err := plugin.Engine.Call(t.Context(), "parseSubmitResponse", submitContext, map[string]any{
			"statusCode": 202,
			"headers":    map[string]any{},
			"body":       map[string]any{"ok": true, "data": map[string]any{"status": "running"}},
		})
		require.ErrorContains(t, err, "dsh did not return a task id")
	})
}

func TestDSHBuildQueryRequest(t *testing.T) {
	plugin := loadDSHPlugin(t)

	value, err := plugin.Engine.Call(t.Context(), "buildQueryRequest", map[string]any{
		"taskId":  "task-1",
		"baseUrl": dshBaseURL,
		"apiKey":  "dsh-api-demo-key",
	})
	require.NoError(t, err)
	descriptor := decodePluginValue(t, value)
	assert.Equal(t, dshBaseURL+"/api/v1/tasks/task-1", descriptor["url"])
	assert.Equal(t, "GET", descriptor["method"])
	assert.Equal(t, "dsh-api-demo-key", descriptor["headers"].(map[string]any)["X-API-Key"])

	value, err = plugin.Engine.Call(t.Context(), "buildQueryRequest", map[string]any{
		"taskId":  "task/with space",
		"baseUrl": dshBaseURL,
		"apiKey":  "dsh-api-demo-key",
	})
	require.NoError(t, err)
	descriptor = decodePluginValue(t, value)
	assert.Equal(t, dshBaseURL+"/api/v1/tasks/task%2Fwith%20space", descriptor["url"])

	_, err = plugin.Engine.Call(t.Context(), "buildQueryRequest", map[string]any{
		"taskId":  "",
		"baseUrl": dshBaseURL,
	})
	require.ErrorContains(t, err, "dsh task id is missing")
}

func TestDSHParseTaskResult(t *testing.T) {
	plugin := loadDSHPlugin(t)
	queryContext := map[string]any{
		"taskId": "task-1",
		"model":  "dsh",
	}

	cases := []struct {
		name   string
		data   map[string]any
		status string
		reason string
	}{
		{name: "running", data: map[string]any{"status": "running"}, status: "IN_PROGRESS"},
		{name: "completed", data: map[string]any{"status": "completed", "reply": "done"}, status: "SUCCESS"},
		{name: "max-tokens keeps the partial reply", data: map[string]any{"status": "max-tokens", "reply": "partial"}, status: "SUCCESS"},
		{name: "canceled", data: map[string]any{"status": "canceled"}, status: "SUCCESS"},
		{
			name:   "failed",
			data:   map[string]any{"status": "failed", "error": map[string]any{"code": "BLOCKED", "message": "turn blocked by policy"}},
			status: "FAILURE",
			reason: "BLOCKED: turn blocked by policy",
		},
		{
			name:   "failed falls back to the turn end reason",
			data:   map[string]any{"status": "failed", "turnEnd": "error"},
			status: "FAILURE",
			reason: "agent turn ended: error",
		},
		{name: "timeout", data: map[string]any{"status": "timeout"}, status: "FAILURE", reason: "agent task timed out"},
		{name: "unrecognized status", data: map[string]any{"status": "paused"}, status: "UNKNOWN"},
		{name: "missing status", data: map[string]any{}, status: "UNKNOWN"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			value, err := plugin.Engine.Call(t.Context(), "parseTaskResult", queryContext, map[string]any{
				"ok":   true,
				"data": testCase.data,
			}, map[string]any{"statusCode": 200, "headers": map[string]any{}})
			require.NoError(t, err)
			parsed := decodePluginValue(t, value)

			assert.Equal(t, testCase.status, parsed["status"])
			assert.Equal(t, "task-1", parsed["taskId"])
			if testCase.reason == "" {
				assert.NotContains(t, parsed, "reason")
			} else {
				assert.Equal(t, testCase.reason, parsed["reason"])
			}
		})
	}

	t.Run("persists the session id reported by polling", func(t *testing.T) {
		value, err := plugin.Engine.Call(t.Context(), "parseTaskResult", queryContext, map[string]any{
			"ok": true,
			"data": map[string]any{
				"taskId":    "task-1",
				"status":    "completed",
				"sessionId": "api-1750000000-abc",
				"reply":     "done",
			},
		}, map[string]any{"statusCode": 200, "headers": map[string]any{}})
		require.NoError(t, err)
		parsed := decodePluginValue(t, value)
		assert.Equal(t, map[string]any{"sessionId": "api-1750000000-abc"}, parsed["state"])
	})

	t.Run("treats an error envelope as a poll failure", func(t *testing.T) {
		_, err := plugin.Engine.Call(t.Context(), "parseTaskResult", queryContext, map[string]any{
			"ok":    false,
			"error": map[string]any{"code": "unknown_task", "message": "task not found"},
		}, map[string]any{"statusCode": 500, "headers": map[string]any{}})
		require.ErrorContains(t, err, "dsh unknown_task: task not found")
	})
}

// /pg 与 legacy 任务入口按 TaskSubmitReq 解码请求体后再交给插件；续接字段必须
// 在这个结构体上声明，否则 sessionId/preset 会在解码时被静默丢弃。
func TestDSHContinuationFieldsSurviveTheTaskRequestStruct(t *testing.T) {
	var request relaycommon.TaskSubmitReq
	require.NoError(t, common.Unmarshal([]byte(`{"model":"dsh","prompt":"继续","sessionId":"api-1750000000-abc","preset":"code"}`), &request))
	require.Equal(t, "api-1750000000-abc", request.SessionID)
	require.Equal(t, "code", request.Preset)

	value, err := loadDSHPlugin(t).Engine.Call(t.Context(), "buildSubmitRequest", dshSubmitContext(decodePluginValue(t, request)))
	require.NoError(t, err)
	descriptor := decodePluginValue(t, value)
	assert.Equal(t, map[string]any{
		"prompt":    "继续",
		"sessionId": "api-1750000000-abc",
		"preset":    "code",
	}, descriptor["body"])
}

// 附件清单同样走 /pg 入口的结构体解码：不在 TaskSubmitReq 上声明就会被丢掉，
// 智能体只会收到一条不含任何文件信息的提示词。
func TestDSHAttachmentsSurviveTheTaskRequestStruct(t *testing.T) {
	var request relaycommon.TaskSubmitReq
	require.NoError(t, common.Unmarshal([]byte(`{"model":"dsh","prompt":"看看这张图","attachments":[{"name":"a.png","path":"/data/uploads/7/x-a.png","mimeType":"image/png","size":2048}]}`), &request))
	require.Len(t, request.Attachments, 1)

	value, err := loadDSHPlugin(t).Engine.Call(t.Context(), "buildSubmitRequest", dshSubmitContext(decodePluginValue(t, request)))
	require.NoError(t, err)
	descriptor := decodePluginValue(t, value)
	assert.Equal(t, map[string]any{
		"prompt": "看看这张图\n\n[Uploaded files]\n" +
			"The user attached these files; they are readable on disk:\n" +
			"- /data/uploads/7/x-a.png (a.png)",
	}, descriptor["body"])
}

func dshArtifactTaskContext(status string, data map[string]any) map[string]any {
	return map[string]any{
		"taskId": "task-1",
		"status": status,
		"action": "agent",
		"data":   data,
	}
}

func decodePluginArtifacts(t *testing.T, value any) []map[string]any {
	t.Helper()
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	var artifacts []map[string]any
	require.NoError(t, common.Unmarshal(encoded, &artifacts))
	return artifacts
}

func TestDSHListArtifacts(t *testing.T) {
	plugin := loadDSHPlugin(t)
	payload := map[string]any{
		"ok": true,
		"data": map[string]any{
			"status": "completed",
			"artifacts": []any{
				map[string]any{
					"key":      "medical-llms.pptx",
					"name":     "medical-llms.pptx",
					"size":     float64(65536),
					"mimeType": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
				},
				map[string]any{"key": "cover.png", "mimeType": "image/png"},
				map[string]any{"key": "intro.mp4", "mimeType": "video/mp4"},
				map[string]any{"key": "voice.mp3", "mimeType": "audio/mpeg"},
				map[string]any{"key": "notes", "mimeType": ""},
			},
		},
	}

	value, err := plugin.Engine.Call(t.Context(), "listArtifacts", dshArtifactTaskContext("SUCCESS", payload))
	require.NoError(t, err)
	artifacts := decodePluginArtifacts(t, value)

	// 文件类型按 MIME 归类：演示文稿归 file，其余归各自的媒体类。
	require.Len(t, artifacts, 5)
	assert.Equal(t, map[string]any{
		"key":      "medical-llms.pptx",
		"type":     "file",
		"mimeType": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"name":     "medical-llms.pptx",
	}, artifacts[0])
	assert.Equal(t, map[string]any{"key": "cover.png", "type": "image", "mimeType": "image/png"}, artifacts[1])
	assert.Equal(t, map[string]any{"key": "intro.mp4", "type": "video", "mimeType": "video/mp4"}, artifacts[2])
	assert.Equal(t, map[string]any{"key": "voice.mp3", "type": "audio", "mimeType": "audio/mpeg"}, artifacts[3])
	assert.Equal(t, map[string]any{"key": "notes", "type": "file"}, artifacts[4])
}

func TestDSHListArtifactsSkipsUnsettledAndMalformedEntries(t *testing.T) {
	plugin := loadDSHPlugin(t)
	list := func(status string, payload map[string]any) []map[string]any {
		value, err := plugin.Engine.Call(t.Context(), "listArtifacts", dshArtifactTaskContext(status, payload))
		require.NoError(t, err)
		return decodePluginArtifacts(t, value)
	}

	payload := map[string]any{
		"ok": true,
		"data": map[string]any{
			"artifacts": []any{
				map[string]any{"key": "report.md", "mimeType": "text/markdown; charset=utf-8"},
				map[string]any{"key": "report.md", "mimeType": "text/markdown; charset=utf-8"},
				map[string]any{"key": "  "},
				map[string]any{"name": "without-key.html"},
				"not-an-object",
			},
		},
	}

	// 未结算的任务没有可下载的产物。
	assert.Empty(t, list("IN_PROGRESS", payload))
	assert.Empty(t, list("FAILURE", payload))

	// 重复键与无效条目被丢弃，避免网关因重复 key 拒绝整份清单。
	artifacts := list("SUCCESS", payload)
	require.Len(t, artifacts, 1)
	assert.Equal(t, map[string]any{
		"key":      "report.md",
		"type":     "file",
		"mimeType": "text/markdown; charset=utf-8",
	}, artifacts[0])
}

func TestDSHBuildContentRequest(t *testing.T) {
	plugin := loadDSHPlugin(t)

	value, err := plugin.Engine.Call(t.Context(), "buildContentRequest", map[string]any{
		"baseUrl":        dshBaseURL + "/",
		"apiKey":         "dsh-api-demo-key",
		"upstreamTaskId": "task-1",
		"artifactKey":    "medical-llms.pptx",
		"clientRequest":  map[string]any{"method": "GET"},
	})
	require.NoError(t, err)
	descriptor := decodePluginValue(t, value)
	assert.Equal(t, dshBaseURL+"/api/v1/tasks/task-1/files/medical-llms.pptx", descriptor["url"])
	assert.Equal(t, "GET", descriptor["method"])
	assert.Equal(t, map[string]any{"X-API-Key": "dsh-api-demo-key"}, descriptor["headers"])

	// 预览探测用 HEAD，密钥仍然随请求带上。
	value, err = plugin.Engine.Call(t.Context(), "buildContentRequest", map[string]any{
		"baseUrl":        dshBaseURL,
		"apiKey":         "dsh-api-demo-key",
		"upstreamTaskId": "task/with space",
		"artifactKey":    "a b.png",
		"clientRequest":  map[string]any{"method": "HEAD"},
	})
	require.NoError(t, err)
	descriptor = decodePluginValue(t, value)
	assert.Equal(t, dshBaseURL+"/api/v1/tasks/task%2Fwith%20space/files/a%20b.png", descriptor["url"])
	assert.Equal(t, "HEAD", descriptor["method"])

	_, err = plugin.Engine.Call(t.Context(), "buildContentRequest", map[string]any{
		"baseUrl":        dshBaseURL,
		"upstreamTaskId": "task-1",
	})
	require.ErrorContains(t, err, "missing identifiers")
}

// 网关把任务提交者的 userId 放进所有 dsh-api 请求，dsh-api 据此把任务固定到
// workspace/<userId> 并拒绝跨用户续会话/查询；匿名调用不得凭空多出头。
func TestDSHForwardsTaskOwner(t *testing.T) {
	plugin := loadDSHPlugin(t)

	t.Run("submit carries the owner in the body and headers", func(t *testing.T) {
		ctx := dshSubmitContext(map[string]any{"model": "dsh", "prompt": "hi"})
		ctx["userId"] = 7
		value, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", ctx)
		require.NoError(t, err)
		descriptor := decodePluginValue(t, value)

		assert.Equal(t, "7", descriptor["body"].(map[string]any)["userId"])
		assert.Equal(t, "7", descriptor["headers"].(map[string]any)["X-DSH-User-Id"])
	})

	t.Run("query and content requests carry the owner header", func(t *testing.T) {
		value, err := plugin.Engine.Call(t.Context(), "buildQueryRequest", map[string]any{
			"taskId":  "task-1",
			"baseUrl": dshBaseURL,
			"apiKey":  "dsh-api-demo-key",
			"userId":  7,
		})
		require.NoError(t, err)
		query := decodePluginValue(t, value)
		assert.Equal(t, "7", query["headers"].(map[string]any)["X-DSH-User-Id"])

		value, err = plugin.Engine.Call(t.Context(), "buildContentRequest", map[string]any{
			"baseUrl":        dshBaseURL,
			"apiKey":         "dsh-api-demo-key",
			"upstreamTaskId": "task-1",
			"artifactKey":    "deck.pptx",
			"userId":         7,
			"clientRequest":  map[string]any{"method": "GET"},
		})
		require.NoError(t, err)
		content := decodePluginValue(t, value)
		assert.Equal(t, "7", content["headers"].(map[string]any)["X-DSH-User-Id"])
	})

	t.Run("anonymous contexts keep the legacy payload", func(t *testing.T) {
		ctx := dshSubmitContext(map[string]any{"model": "dsh", "prompt": "hi"})
		ctx["userId"] = 0
		value, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", ctx)
		require.NoError(t, err)
		descriptor := decodePluginValue(t, value)
		assert.NotContains(t, descriptor["body"].(map[string]any), "userId")
		assert.NotContains(t, descriptor["headers"].(map[string]any), "X-DSH-User-Id")
	})
}
