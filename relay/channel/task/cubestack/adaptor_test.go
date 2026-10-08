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
package cubestack

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testUpstreamModel = "MiniMax-H3"

func newAdaptor(baseURL, key string) *TaskAdaptor {
	a := &TaskAdaptor{}
	a.Init(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       64,
			ChannelBaseUrl:    baseURL,
			ApiKey:            key,
			UpstreamModelName: testUpstreamModel,
		},
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
		OriginModelName: testUpstreamModel,
	})
	return a
}

func newTestContext(t *testing.T, req relaycommon.TaskSubmitReq) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("task_request", req)
	return c
}

func newRelayInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: testUpstreamModel,
		},
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
		OriginModelName: testUpstreamModel,
	}
}

func buildBody(t *testing.T, a *TaskAdaptor, req relaycommon.TaskSubmitReq) map[string]any {
	t.Helper()
	reader, err := a.BuildRequestBody(newTestContext(t, req), newRelayInfo())
	require.NoError(t, err)
	payload, err := io.ReadAll(reader)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, common.Unmarshal(payload, &body))
	return body
}

func TestBuildRequestBody_TextToVideoDefaults(t *testing.T) {
	a := newAdaptor("http://sglang.local:30000", "")
	body := buildBody(t, a, relaycommon.TaskSubmitReq{
		Prompt:   "a serene koi pond",
		Duration: 5,
		Size:     "720p",
	})

	assert.Equal(t, testUpstreamModel, body["model"])
	assert.Equal(t, "a serene koi pond", body["prompt"])
	assert.Equal(t, float64(5), body["seconds"])
	assert.Equal(t, "t2va", body["task"])
	assert.Equal(t, float64(1), body["num_outputs_per_prompt"])
	assert.Equal(t, float64(defaultNumInferenceSteps), body["num_inference_steps"])
	assert.Equal(t, defaultFlowShift, body["flow_shift"])
	assert.Equal(t, defaultAudioFlowShift, body["audio_flow_shift"])
	_, hasSeed := body["seed"]
	assert.False(t, hasSeed, "seed must be omitted when not provided")

	conditions, ok := body["conditions"].([]any)
	require.True(t, ok)
	assert.Empty(t, conditions)

	target, ok := body["target"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(defaultShortEdge), target["short_edge"])
	assert.Equal(t, defaultAspectRatio, target["aspect_ratio"])
	assert.Equal(t, float64(5), target["duration_seconds"])
}

func TestBuildRequestBody_FirstAndLastFrame(t *testing.T) {
	first := "data:image/jpeg;base64,AAAA"
	last := "data:image/jpeg;base64,BBBB"
	a := newAdaptor("http://sglang.local:30000", "")
	body := buildBody(t, a, relaycommon.TaskSubmitReq{
		Prompt:   "continue the scene",
		Duration: 5,
		Images:   []string{first, last},
	})

	assert.Equal(t, "fl2va", body["task"])

	conditions, ok := body["conditions"].([]any)
	require.True(t, ok)
	require.Len(t, conditions, 2)

	firstCond, ok := conditions[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "image", firstCond["type"])
	assert.Equal(t, first, firstCond["uri"])
	assert.Equal(t, "keyframe", firstCond["role"])
	assert.Equal(t, float64(0), firstCond["frame_index"])

	lastCond, ok := conditions[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, last, lastCond["uri"])
	assert.Equal(t, float64(-1), lastCond["frame_index"])

	// 关键帧任务跟随输入画幅。
	target, ok := body["target"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "auto", target["aspect_ratio"])
}

func TestBuildRequestBody_MetadataOverrides(t *testing.T) {
	a := newAdaptor("http://sglang.local:30000", "")
	body := buildBody(t, a, relaycommon.TaskSubmitReq{
		Prompt:   "reference shot",
		Duration: 8,
		Size:     "720p",
		Metadata: map[string]interface{}{
			"task":                   "ref2va",
			"aspect_ratio":           "9:16",
			"short_edge":             512,
			"num_inference_steps":    30,
			"flow_shift":             8.5,
			"audio_flow_shift":       2.5,
			"seed":                   42,
			"num_outputs_per_prompt": 2,
		},
	})

	assert.Equal(t, "ref2va", body["task"])
	assert.Equal(t, float64(30), body["num_inference_steps"])
	assert.Equal(t, 8.5, body["flow_shift"])
	assert.Equal(t, 2.5, body["audio_flow_shift"])
	assert.Equal(t, float64(42), body["seed"])
	assert.Equal(t, float64(2), body["num_outputs_per_prompt"])

	target, ok := body["target"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(512), target["short_edge"])
	assert.Equal(t, "9:16", target["aspect_ratio"])
}

func TestBuildRequestBody_MetadataDurationAndResolution(t *testing.T) {
	a := newAdaptor("http://sglang.local:30000", "")
	body := buildBody(t, a, relaycommon.TaskSubmitReq{
		Prompt:   "p",
		Duration: 5,
		Size:     "720p",
		Metadata: map[string]interface{}{
			"duration":   10,
			"resolution": "360p",
		},
	})
	// metadata 覆盖优先于顶层字段，与计费侧同一优先级。
	assert.Equal(t, float64(10), body["seconds"])
	target, ok := body["target"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(10), target["duration_seconds"])
	assert.Equal(t, float64(512), target["short_edge"])
}

func TestBuildRequestBody_SizeShortEdgeMapping(t *testing.T) {
	cases := []struct {
		size      string
		shortEdge int
	}{
		{"360p", 512},
		{"540p", 512},
		{"720p", 768},
		{"1080p", 768},
		{"", 768},
	}
	a := newAdaptor("http://sglang.local:30000", "")
	for _, tc := range cases {
		body := buildBody(t, a, relaycommon.TaskSubmitReq{Prompt: "p", Duration: 5, Size: tc.size})
		target, ok := body["target"].(map[string]any)
		require.True(t, ok, "size %q", tc.size)
		assert.Equal(t, float64(tc.shortEdge), target["short_edge"], "size %q", tc.size)
	}
}

func TestBuildRequestBody_DurationFromSecondsString(t *testing.T) {
	a := newAdaptor("http://sglang.local:30000", "")
	body := buildBody(t, a, relaycommon.TaskSubmitReq{Prompt: "p", Seconds: "7"})
	assert.Equal(t, float64(7), body["seconds"])
	target, ok := body["target"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(7), target["duration_seconds"])
}

func TestValidateRequestAndSetAction_DurationBounds(t *testing.T) {
	a := newAdaptor("http://sglang.local:30000", "")
	cases := []struct {
		name      string
		req       relaycommon.TaskSubmitReq
		wantError bool
	}{
		{"valid 5s", relaycommon.TaskSubmitReq{Prompt: "p", Duration: 5}, false},
		{"valid 15s", relaycommon.TaskSubmitReq{Prompt: "p", Duration: 15}, false},
		{"too short 3s", relaycommon.TaskSubmitReq{Prompt: "p", Duration: 3}, true},
		{"too long 30s", relaycommon.TaskSubmitReq{Prompt: "p", Duration: 30}, true},
		{"metadata duration too long", relaycommon.TaskSubmitReq{
			Prompt:   "p",
			Duration: 5,
			Metadata: map[string]interface{}{"duration": 600},
		}, true},
		// 非正值同样越界。effectiveDuration 会原样采用 metadata.duration（含 0 与
		// 负数），区间判断先比 > 0 因而放过它；BuildRequestBody 再把它换成默认 5
		// 秒，而计费侧把非正值当作"未提供"、改用顶层 duration —— 用户按 15 秒付费
		// 而上游只出 5 秒。显式给值就必须落在档位内。
		{"metadata duration zero", relaycommon.TaskSubmitReq{
			Prompt:   "p",
			Duration: 15,
			Metadata: map[string]interface{}{"duration": 0},
		}, true},
		{"metadata duration negative", relaycommon.TaskSubmitReq{
			Prompt:   "p",
			Duration: 15,
			Metadata: map[string]interface{}{"duration": -5},
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestContext(t, tc.req)
			// 校验函数自身会从上下文读取已解析请求；这里直接调用 H3 边界检查，
			// 不依赖 UnmarshalBodyReusable 的 body 解析路径。
			taskErr := a.validateH3Bounds(c)
			if tc.wantError {
				require.NotNil(t, taskErr)
			} else {
				assert.Nil(t, taskErr)
			}
		})
	}
}

func TestValidateRequestAndSetAction_MetadataBounds(t *testing.T) {
	a := newAdaptor("http://sglang.local:30000", "")
	cases := []struct {
		name      string
		metadata  map[string]interface{}
		wantError bool
	}{
		{"steps too high", map[string]interface{}{"num_inference_steps": 999}, true},
		{"outputs too high", map[string]interface{}{"num_outputs_per_prompt": 8}, true},
		{"short_edge too high", map[string]interface{}{"short_edge": 9999}, true},
		{"bad task", map[string]interface{}{"task": "dance"}, true},
		{"good overrides", map[string]interface{}{
			"task": "t2va", "num_inference_steps": 100, "num_outputs_per_prompt": 4, "short_edge": 1024,
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestContext(t, relaycommon.TaskSubmitReq{Prompt: "p", Duration: 5, Metadata: tc.metadata})
			taskErr := a.validateH3Bounds(c)
			if tc.wantError {
				require.NotNil(t, taskErr)
			} else {
				assert.Nil(t, taskErr)
			}
		})
	}
}

func TestParseResponse(t *testing.T) {
	a := newAdaptor("http://sglang.local:30000", "")
	info := &relaycommon.RelayInfo{
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "pub_123"},
		OriginModelName: testUpstreamModel,
	}

	cases := []struct {
		name      string
		body      string
		wantID    string
		wantError bool
	}{
		{"id field", `{"id":"video_gen_abc","status":"queued"}`, "video_gen_abc", false},
		{"task_id fallback", `{"task_id":"video_gen_xyz"}`, "video_gen_xyz", false},
		{"empty", `{"status":"queued"}`, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{Body: io.NopCloser(strings.NewReader(tc.body))}
			result, taskErr := a.ParseResponse(newTestContext(t, relaycommon.TaskSubmitReq{}), resp, info)
			if tc.wantError {
				require.NotNil(t, taskErr)
				return
			}
			require.Nil(t, taskErr)
			require.NotNil(t, result)
			assert.Equal(t, tc.wantID, result.UpstreamTaskID)

			video, ok := result.ClientResponse.(*dto.OpenAIVideo)
			require.True(t, ok)
			assert.Equal(t, "pub_123", video.TaskID)
			assert.Equal(t, "queued", video.Status)
		})
	}
}

func TestParseTaskResult(t *testing.T) {
	task := &model.Task{
		TaskID:      "pub_123",
		PrivateData: model.TaskPrivateData{UpstreamTaskID: "video_gen_abc"},
	}
	resp := &http.Response{Request: &http.Request{URL: &url.URL{Scheme: "http", Host: "sglang.local:30000"}}}
	a := newAdaptor("http://sglang.local:30000", "")

	cases := []struct {
		name       string
		body       string
		wantStatus string
		wantURL    string
		wantReason string
	}{
		{
			name:       "queued",
			body:       `{"id":"video_gen_abc","status":"queued"}`,
			wantStatus: model.TaskStatusQueued,
		},
		{
			name:       "in_progress with progress",
			body:       `{"id":"video_gen_abc","status":"in_progress","progress":42.7}`,
			wantStatus: model.TaskStatusInProgress,
		},
		{
			name:       "completed",
			body:       `{"id":"video_gen_abc","status":"completed"}`,
			wantStatus: model.TaskStatusSuccess,
			wantURL:    "http://sglang.local:30000/v1/videos/video_gen_abc/content",
		},
		{
			name:       "failed with error message",
			body:       `{"id":"video_gen_abc","status":"failed","error":{"code":"oom","message":"out of memory"}}`,
			wantStatus: model.TaskStatusFailure,
			wantReason: "out of memory",
		},
		{
			name:       "unknown status stays in progress",
			body:       `{"id":"video_gen_abc","status":"weird"}`,
			wantStatus: model.TaskStatusInProgress,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := a.ParseTaskResult(task, resp, []byte(tc.body))
			require.NoError(t, err)
			assert.Equal(t, tc.wantStatus, result.Status)
			if tc.wantURL != "" {
				assert.Equal(t, tc.wantURL, result.Url)
			}
			if tc.wantReason != "" {
				assert.Equal(t, tc.wantReason, result.Reason)
			}
		})
	}
}

func TestParseTaskResult_InProgressProgressDefault(t *testing.T) {
	task := &model.Task{PrivateData: model.TaskPrivateData{UpstreamTaskID: "video_gen_abc"}}
	resp := &http.Response{Request: &http.Request{URL: &url.URL{Scheme: "http", Host: "sglang.local:30000"}}}
	a := newAdaptor("http://sglang.local:30000", "")

	result, err := a.ParseTaskResult(task, resp, []byte(`{"status":"in_progress"}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusInProgress, result.Status)
	assert.Equal(t, taskcommon.ProgressInProgress, result.Progress)
}

func TestFetchTask_SendsBearerAndPath(t *testing.T) {
	var gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"status":"queued"}`))
	}))
	defer server.Close()

	a := newAdaptor(server.URL, "sk-test")
	task := &model.Task{PrivateData: model.TaskPrivateData{UpstreamTaskID: "video_gen_abc"}}
	resp, err := a.FetchTask(server.URL, "sk-test", task, "")
	require.NoError(t, err)
	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, "/v1/videos/video_gen_abc", gotPath)
	assert.Equal(t, "Bearer sk-test", gotAuth)
}
