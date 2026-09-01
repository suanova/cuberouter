package astraflow

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertToRequestPayloadContentPassthrough 锁定直传契约：客户端给定的
// content 数组（含 role 标注与参考视频/音频）必须原样落到 input.content，
// 顶层 duration/resolution 落到 parameters，且不追加额外 text 项。
func TestConvertToRequestPayloadContentPassthrough(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:      "doubao-seedance-2-0-260128",
		Resolution: "720p",
		Duration:   10,
		Content: []relaycommon.TaskContentItem{
			{Type: "text", Text: lo.ToPtr("跟随参考风格的舞蹈表演")},
			{Type: "image_url", ImageURL: &relaycommon.TaskMediaURL{URL: lo.ToPtr("https://example.com/style_ref.jpg")}, Role: lo.ToPtr("reference_image")},
			{Type: "video_url", VideoURL: &relaycommon.TaskMediaURL{URL: lo.ToPtr("https://example.com/motion_ref.mp4")}, Role: lo.ToPtr("reference_video")},
			{Type: "audio_url", AudioURL: &relaycommon.TaskMediaURL{URL: lo.ToPtr("https://example.com/music_ref.mp3")}, Role: lo.ToPtr("reference_audio")},
		},
	}

	body, err := adaptor.convertToRequestPayload(&req)
	require.NoError(t, err)

	assert.Equal(t, "doubao-seedance-2-0-260128", body.Model)
	assert.Equal(t, req.Content, body.Input.Content)
	require.NotNil(t, body.Parameters.Duration)
	assert.Equal(t, 10, int(*body.Parameters.Duration))
	assert.Equal(t, "720p", body.Parameters.Resolution)
}

// TestConvertToRequestPayloadContentWithoutText 锁定补全契约：content 中没有
// text 项时（提示词仍在顶层 prompt 字段），提示词必须作为 text 项追加，
// 因为上游要求至少一个 text 项。
func TestConvertToRequestPayloadContentWithoutText(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-0-260128",
		Prompt: "一只猫在晒太阳",
		Content: []relaycommon.TaskContentItem{
			{Type: "image_url", ImageURL: &relaycommon.TaskMediaURL{URL: lo.ToPtr("https://example.com/a.jpg")}, Role: lo.ToPtr("first_frame")},
		},
	}

	body, err := adaptor.convertToRequestPayload(&req)
	require.NoError(t, err)

	require.Len(t, body.Input.Content, 2)
	assert.Equal(t, "image_url", body.Input.Content[0].Type)
	assert.Equal(t, "first_frame", *body.Input.Content[0].Role)
	assert.Equal(t, "text", body.Input.Content[1].Type)
	assert.Equal(t, "一只猫在晒太阳", *body.Input.Content[1].Text)
}

// TestConvertToRequestPayloadLegacyFields 锁定旧式字段契约：prompt/images/
// seconds 仍按统一字段语义转换，prompt 独占唯一的 text 项。
func TestConvertToRequestPayloadLegacyFields(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:   "doubao-seedance-1-5-pro",
		Prompt:  "animate the first frame",
		Images:  []string{"https://example.com/first.png"},
		Seconds: "8",
	}

	body, err := adaptor.convertToRequestPayload(&req)
	require.NoError(t, err)

	require.Len(t, body.Input.Content, 2)
	assert.Equal(t, "image_url", body.Input.Content[0].Type)
	require.NotNil(t, body.Input.Content[0].ImageURL)
	assert.Equal(t, "https://example.com/first.png", *body.Input.Content[0].ImageURL.URL)
	assert.Equal(t, "text", body.Input.Content[1].Type)
	assert.Equal(t, "animate the first frame", *body.Input.Content[1].Text)
	require.NotNil(t, body.Parameters.Duration)
	assert.Equal(t, 8, int(*body.Parameters.Duration))
}

// TestConvertToRequestPayloadMetadataOverlay 锁定 metadata 覆盖语义：
// metadata.content 整体替换 input.content，其中的 text 项必须原样保留——
// 即使顶层 prompt 为空也绝不被改写或清空；其余 Ark 顶层参数落入 parameters。
func TestConvertToRequestPayloadMetadataOverlay(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:    "doubao-seedance-2-0-260128",
		Prompt:   "",
		Duration: 5,
		Metadata: map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{"type": "text", "text": "metadata prompt"},
				map[string]interface{}{
					"type":      "image_url",
					"image_url": map[string]interface{}{"url": "https://example.com/first.png"},
					"role":      "first_frame",
				},
			},
			"service_tier": "flex",
			"duration":     12,
		},
	}

	body, err := adaptor.convertToRequestPayload(&req)
	require.NoError(t, err)

	require.Len(t, body.Input.Content, 2)
	assert.Equal(t, "text", body.Input.Content[0].Type)
	assert.Equal(t, "metadata prompt", *body.Input.Content[0].Text)
	assert.Equal(t, "image_url", body.Input.Content[1].Type)
	assert.Equal(t, "first_frame", *body.Input.Content[1].Role)
	assert.Equal(t, "flex", body.Parameters.ServiceTier)
	require.NotNil(t, body.Parameters.Duration)
	// metadata 覆盖优先于顶层 duration。
	assert.Equal(t, 12, int(*body.Parameters.Duration))
}

// TestConvertToRequestPayloadDurationPriority 锁定 duration 优先级：
// metadata 覆盖 > 顶层 duration > seconds。
func TestConvertToRequestPayloadDurationPriority(t *testing.T) {
	adaptor := &TaskAdaptor{}

	topLevelBeatsSeconds := relaycommon.TaskSubmitReq{
		Model:    "doubao-seedance-2-0-260128",
		Prompt:   "p",
		Duration: 8,
		Seconds:  "4",
	}
	body, err := adaptor.convertToRequestPayload(&topLevelBeatsSeconds)
	require.NoError(t, err)
	require.NotNil(t, body.Parameters.Duration)
	assert.Equal(t, 8, int(*body.Parameters.Duration))

	secondsAsFallback := relaycommon.TaskSubmitReq{
		Model:   "doubao-seedance-2-0-260128",
		Prompt:  "p",
		Seconds: "4",
	}
	body, err = adaptor.convertToRequestPayload(&secondsAsFallback)
	require.NoError(t, err)
	require.NotNil(t, body.Parameters.Duration)
	assert.Equal(t, 4, int(*body.Parameters.Duration))
}

// TestConvertToRequestPayloadRatioDefault 锁定 ratio 转发契约：客户端显式
// 传 ratio 时原样落到 parameters；缺失时按 Ark 规范默认 "adaptive"，避免
// 上游报 "ratio is required"。
func TestConvertToRequestPayloadRatioDefault(t *testing.T) {
	adaptor := &TaskAdaptor{}

	explicit := relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-0-260128",
		Prompt: "p",
		Ratio:  lo.ToPtr("16:9"),
	}
	body, err := adaptor.convertToRequestPayload(&explicit)
	require.NoError(t, err)
	assert.Equal(t, "16:9", body.Parameters.Ratio)

	absent := relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-0-260128",
		Prompt: "p",
	}
	body, err = adaptor.convertToRequestPayload(&absent)
	require.NoError(t, err)
	assert.Equal(t, "adaptive", body.Parameters.Ratio)

	// 显式空串不是合法 ratio，与缺省一样按 Ark 规范落到 adaptive。
	explicitEmpty := relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-0-260128",
		Prompt: "p",
		Ratio:  lo.ToPtr(""),
	}
	body, err = adaptor.convertToRequestPayload(&explicitEmpty)
	require.NoError(t, err)
	assert.Equal(t, "adaptive", body.Parameters.Ratio)
}

// TestParseTaskResult 锁定上游任务状态映射：成功携带视频地址与 token 用量，
// 失败/过期映射为失败终态，未知状态按进行中处理避免任务卡死轮询。
func TestParseTaskResult(t *testing.T) {
	adaptor := &TaskAdaptor{}

	tests := []struct {
		name           string
		body           string
		wantStatus     string
		wantProgress   string
		wantURL        string
		wantReason     string
		wantCompletion int
		wantTotal      int
	}{
		{
			name:         "pending maps to queued",
			body:         `{"output":{"task_id":"t1","task_status":"Pending"}}`,
			wantStatus:   model.TaskStatusQueued,
			wantProgress: "10%",
		},
		{
			name:         "running maps to in_progress",
			body:         `{"output":{"task_id":"t1","task_status":"Running"}}`,
			wantStatus:   model.TaskStatusInProgress,
			wantProgress: "50%",
		},
		{
			name:           "success carries url and usage",
			body:           `{"output":{"task_id":"t1","task_status":"Success","urls":["https://cdn.example.com/v.mp4"]},"usage":{"duration":5,"completion_tokens":109431}}`,
			wantStatus:     model.TaskStatusSuccess,
			wantProgress:   "100%",
			wantURL:        "https://cdn.example.com/v.mp4",
			wantCompletion: 109431,
			wantTotal:      109431,
		},
		{
			name:         "success without urls stays url-less",
			body:         `{"output":{"task_id":"t1","task_status":"Success"}}`,
			wantStatus:   model.TaskStatusSuccess,
			wantProgress: "100%",
		},
		{
			name:         "failure carries error message",
			body:         `{"output":{"task_id":"t1","task_status":"Failure","error_message":"内容审核失败"}}`,
			wantStatus:   model.TaskStatusFailure,
			wantProgress: "100%",
			wantReason:   "内容审核失败",
		},
		{
			name:         "expired without message falls back to status text",
			body:         `{"output":{"task_id":"t1","task_status":"Expired"}}`,
			wantStatus:   model.TaskStatusFailure,
			wantProgress: "100%",
			wantReason:   "task expired",
		},
		{
			name:         "unknown status treated as in_progress",
			body:         `{"output":{"task_id":"t1","task_status":"SomethingNew"}}`,
			wantStatus:   model.TaskStatusInProgress,
			wantProgress: "30%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskInfo, err := adaptor.ParseTaskResult([]byte(tt.body))
			require.NoError(t, err)

			assert.Equal(t, tt.wantStatus, taskInfo.Status)
			assert.Equal(t, tt.wantProgress, taskInfo.Progress)
			assert.Equal(t, tt.wantURL, taskInfo.Url)
			assert.Equal(t, tt.wantReason, taskInfo.Reason)
			assert.Equal(t, tt.wantCompletion, taskInfo.CompletionTokens)
			assert.Equal(t, tt.wantTotal, taskInfo.TotalTokens)
		})
	}
}

// TestParseTaskResultRejectsGarbage 锁定错误契约：无法解析的查询响应必须
// 报错，而不是被当成进行中无限轮询。
func TestParseTaskResultRejectsGarbage(t *testing.T) {
	adaptor := &TaskAdaptor{}

	taskInfo, err := adaptor.ParseTaskResult([]byte(`not json`))

	require.Error(t, err)
	assert.Nil(t, taskInfo)
}

// TestParseResponseReturnsUpstreamIDAndCachesTaskData 锁定 ID 分离契约：
// 提交流程拿到上游真实 task_id（入库供轮询使用），ClientResponse 中只出现
// 网关预生成的公开 task ID。ParseResponse 不写回客户端，由控制器呈现。
func TestParseResponseReturnsUpstreamIDAndCachesTaskData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public_123"},
		OriginModelName: "doubao-seedance-2-0-260128",
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"output":{"task_id":"upstream_456"}}`)),
	}

	submitResp, taskErr := adaptor.ParseResponse(context, resp, info)

	require.Nil(t, taskErr)
	require.NotNil(t, submitResp)
	assert.Equal(t, "upstream_456", submitResp.UpstreamTaskID)
	assert.JSONEq(t, `{"output":{"task_id":"upstream_456"}}`, string(submitResp.TaskData))
	require.NotNil(t, submitResp.ClientResponse)
	ov, ok := submitResp.ClientResponse.(*dto.OpenAIVideo)
	require.True(t, ok)
	assert.Equal(t, "task_public_123", ov.ID)
	assert.Equal(t, "task_public_123", ov.TaskID)
	assert.Equal(t, "doubao-seedance-2-0-260128", ov.Model)
	// 客户端未收到任何写入；上游原始 ID 不泄露给客户端。
	assert.Empty(t, recorder.Body.String())
}

// TestBuildRequestURL 锁定提交端点：上游固定路径 /v1/tasks/submit。
func TestBuildRequestURL(t *testing.T) {
	adaptor := &TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://api.modelverse.cn"},
	})

	url, err := adaptor.BuildRequestURL(nil)
	require.NoError(t, err)
	assert.Equal(t, "https://api.modelverse.cn/v1/tasks/submit", url)
}

// TestFetchTaskHitsStatusEndpoint 锁定轮询契约：GET {base}/v1/tasks/status
// 查询参数 task_id，并携带 Bearer 鉴权头。
func TestFetchTaskHitsStatusEndpoint(t *testing.T) {
	var gotMethod, gotPath, gotTaskID, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotTaskID = r.URL.Query().Get("task_id")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"task_id":"t_1","task_status":"Pending"}}`))
	}))
	defer server.Close()

	adaptor := &TaskAdaptor{}
	resp, err := adaptor.FetchTask(server.URL, "sk-test", map[string]any{"task_id": "t_1"}, "")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.MethodGet, gotMethod)
	assert.Equal(t, "/v1/tasks/status", gotPath)
	assert.Equal(t, "t_1", gotTaskID)
	assert.Equal(t, "Bearer sk-test", gotAuth)
}

// TestBuildRequestBodyWireShape 锁定线上报文形状：model 为顶层字段，
// content 数组嵌套在 input 之下，其余 Ark 参数嵌套在 parameters 之下；
// 模型映射时以上游模型名替换。
func TestBuildRequestBodyWireShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set("task_request", relaycommon.TaskSubmitReq{
		Model:    "doubao-seedance-2-0-260128",
		Prompt:   "a castle in the sky",
		Duration: 5,
	})

	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
		ChannelMeta: &relaycommon.ChannelMeta{
			IsModelMapped:     true,
			UpstreamModelName: "ep-2026-seedance",
		},
	}

	reader, err := adaptor.BuildRequestBody(context, info)
	require.NoError(t, err)

	var envelope struct {
		Model      string                 `json:"model"`
		Input      map[string]interface{} `json:"input"`
		Parameters map[string]interface{} `json:"parameters"`
	}
	require.NoError(t, common.DecodeJson(reader, &envelope))

	assert.Equal(t, "ep-2026-seedance", envelope.Model)
	content, ok := envelope.Input["content"].([]interface{})
	require.True(t, ok, "input.content must be an array")
	require.Len(t, content, 1)
	item, ok := content[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "text", item["type"])
	assert.Equal(t, "a castle in the sky", item["text"])
	assert.Equal(t, float64(5), envelope.Parameters["duration"])
}

// TestConvertToArkVideo 锁定 Ark 风格视频端点（/v1/videos/generations/tasks）
// 查询响应的返回契约：任何状态都携带 id/status/model/created_at/updated_at；
// 成功态补充 content.video_url/output.duration/usage.completion_tokens；
// 终态失败按原始上游状态区分 expired 与 failed，并携带对应的 error code。
func TestConvertToArkVideo(t *testing.T) {
	adaptor := &TaskAdaptor{}

	newTask := func(data string, status model.TaskStatus) *model.Task {
		return &model.Task{
			TaskID:     "vt_123",
			Status:     status,
			Progress:   "100%",
			CreatedAt:  1000,
			UpdatedAt:  2000,
			Properties: model.Properties{OriginModelName: "doubao-seedance-2-0-260128"},
			Data:       json.RawMessage(data),
		}
	}

	tests := []struct {
		name           string
		task           *model.Task
		wantStatus     string
		wantVideoURL   string
		wantDuration   int
		wantCompletion int
		wantErrorCode  string
		wantErrorMsg   string
	}{
		{
			name:       "pending maps to queued",
			task:       newTask(`{"output":{"task_id":"t1","task_status":"Pending"}}`, model.TaskStatusQueued),
			wantStatus: taskdto.ArkVideoStatusQueued,
		},
		{
			name:       "running maps to running",
			task:       newTask(`{"output":{"task_id":"t1","task_status":"Running"}}`, model.TaskStatusInProgress),
			wantStatus: taskdto.ArkVideoStatusRunning,
		},
		{
			name:           "success carries url/duration/usage",
			task:           newTask(`{"output":{"task_id":"t1","task_status":"Success","urls":["https://cdn.example.com/v.mp4"]},"usage":{"duration":5,"completion_tokens":109431}}`, model.TaskStatusSuccess),
			wantStatus:     taskdto.ArkVideoStatusSucceeded,
			wantVideoURL:   "https://cdn.example.com/v.mp4",
			wantDuration:   5,
			wantCompletion: 109431,
		},
		{
			name:          "failure carries video_task_failed",
			task:          newTask(`{"output":{"task_id":"t1","task_status":"Failure","error_message":"内容审核失败"}}`, model.TaskStatusFailure),
			wantStatus:    taskdto.ArkVideoStatusFailed,
			wantErrorCode: taskdto.ArkVideoErrorFailed,
			wantErrorMsg:  "内容审核失败",
		},
		{
			name:          "expired maps to expired with dedicated code",
			task:          newTask(`{"output":{"task_id":"t1","task_status":"Expired"}}`, model.TaskStatusFailure),
			wantStatus:    taskdto.ArkVideoStatusExpired,
			wantErrorCode: taskdto.ArkVideoErrorExpired,
			wantErrorMsg:  "task expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := adaptor.ConvertToArkVideo(tt.task)
			require.NoError(t, err)

			var task taskdto.ArkVideoTask
			require.NoError(t, common.Unmarshal(data, &task))

			// 必含字段：id/status/model/created_at/updated_at
			assert.Equal(t, "vt_123", task.ID)
			assert.Equal(t, "doubao-seedance-2-0-260128", task.Model)
			assert.Equal(t, int64(1000), task.CreatedAt)
			assert.Equal(t, int64(2000), task.UpdatedAt)
			assert.Equal(t, tt.wantStatus, task.Status)

			if tt.wantVideoURL != "" {
				require.NotNil(t, task.Content)
				assert.Equal(t, tt.wantVideoURL, task.Content.VideoURL)
			} else {
				assert.Nil(t, task.Content)
			}
			if tt.wantDuration != 0 {
				require.NotNil(t, task.Output)
				assert.Equal(t, tt.wantDuration, task.Output.Duration)
			} else {
				assert.Nil(t, task.Output)
			}
			if tt.wantCompletion != 0 {
				require.NotNil(t, task.Usage)
				assert.Equal(t, tt.wantCompletion, task.Usage.CompletionTokens)
			} else {
				assert.Nil(t, task.Usage)
			}
			if tt.wantErrorCode != "" {
				require.NotNil(t, task.Error)
				assert.Equal(t, tt.wantErrorCode, task.Error.Code)
				assert.Equal(t, tt.wantErrorMsg, task.Error.Message)
			} else {
				assert.Nil(t, task.Error)
			}
		})
	}
}

// TestConvertToOpenAIVideoFailureStatuses 是回归测试：失败/过期等终态
// 映射为 FAILURE 后，转换出的视频必须携带 terminal error（优先上游错误信息，
// 为空时退回状态文案，与 ParseTaskResult 一致）；成功任务不得携带 error。
func TestConvertToOpenAIVideoFailureStatuses(t *testing.T) {
	adaptor := &TaskAdaptor{}

	tests := []struct {
		name        string
		data        string
		status      model.TaskStatus
		wantMessage string
	}{
		{
			name:        "failed carries upstream error",
			data:        `{"output":{"task_id":"t1","task_status":"Failure","error_message":"内容审核失败"}}`,
			status:      model.TaskStatusFailure,
			wantMessage: "内容审核失败",
		},
		{
			name:        "expired without message falls back to status text",
			data:        `{"output":{"task_id":"t1","task_status":"Expired"}}`,
			status:      model.TaskStatusFailure,
			wantMessage: "task expired",
		},
		{
			name:        "success has no error",
			data:        `{"output":{"task_id":"t1","task_status":"Success","urls":["https://cdn.example.com/v.mp4"]}}`,
			status:      model.TaskStatusSuccess,
			wantMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originTask := &model.Task{
				TaskID:     "vt_123",
				Status:     tt.status,
				Progress:   "100%",
				CreatedAt:  1000,
				UpdatedAt:  2000,
				Properties: model.Properties{OriginModelName: "doubao-seedance-2-0-260128"},
				Data:       json.RawMessage(tt.data),
			}

			data, err := adaptor.ConvertToOpenAIVideo(originTask)
			require.NoError(t, err)

			var ov dto.OpenAIVideo
			require.NoError(t, common.Unmarshal(data, &ov))
			if tt.wantMessage == "" {
				assert.Nil(t, ov.Error)
			} else {
				require.NotNil(t, ov.Error)
				assert.Equal(t, tt.wantMessage, ov.Error.Message)
			}
		})
	}
}

// TestParseResponseArkSubmitShape 锁定 Ark 风格端点提交响应形态：
// /v1/videos/generations/tasks 返回 {id, model, status:"queued", created_at}，
// 不携带 OpenAI 视频字段（object/progress/task_id）与上游原始 task ID。
// ParseResponse 不写回客户端，ClientResponse 由控制器在持久化屏障后呈现。
func TestParseResponseArkSubmitShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations/tasks", nil)

	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "vt_123"},
		OriginModelName: "doubao-seedance-2-0-260128",
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"output":{"task_id":"upstream_456"}}`)),
	}

	submitResp, taskErr := adaptor.ParseResponse(context, resp, info)

	require.Nil(t, taskErr)
	require.NotNil(t, submitResp)
	assert.Equal(t, "upstream_456", submitResp.UpstreamTaskID)
	require.NotNil(t, submitResp.ClientResponse)
	task, ok := submitResp.ClientResponse.(*taskdto.ArkVideoTask)
	require.True(t, ok)
	assert.Equal(t, "vt_123", task.ID)
	assert.Equal(t, "doubao-seedance-2-0-260128", task.Model)
	assert.Equal(t, taskdto.ArkVideoStatusQueued, task.Status)
	assert.Greater(t, task.CreatedAt, int64(0))
	// 提交响应只含 id/model/status/created_at，无 OpenAI 视频字段。
	assert.Empty(t, task.UpdatedAt)
	assert.Nil(t, task.Content)
	assert.Nil(t, task.Error)
	// 原始上游响应原样缓存为 TaskData，供轮询阶段解析；客户端未收到任何写入。
	assert.Contains(t, string(submitResp.TaskData), "upstream_456")
	assert.Empty(t, recorder.Body.String())
}
