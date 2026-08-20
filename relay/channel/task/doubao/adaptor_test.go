package doubao

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertToRequestPayloadMetadataContentPreservesText 是回归测试：
// metadata.content 提供的 text 项是客户端提示词，即使顶层 content 与
// prompt 均为空，也必须原样保留，不得被旧式分支改写为空 text 项。
func TestConvertToRequestPayloadMetadataContentPreservesText(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model: "doubao-seedance-2-0-260128",
		Metadata: map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{"type": "text", "text": "metadata prompt"},
				map[string]interface{}{
					"type":      "image_url",
					"image_url": map[string]interface{}{"url": "https://example.com/first.png"},
					"role":      "first_frame",
				},
			},
		},
	}

	r, err := adaptor.convertToRequestPayload(&req)
	require.NoError(t, err)

	require.Len(t, r.Content, 2)
	assert.Equal(t, "text", r.Content[0].Type)
	assert.Equal(t, "metadata prompt", *r.Content[0].Text)
	assert.Equal(t, "image_url", r.Content[1].Type)
	assert.Equal(t, "first_frame", *r.Content[1].Role)
}

// TestConvertToRequestPayloadContentPassthrough 锁定直传契约：客户端给定的
// content 数组原样透传（含 role 标注），不追加额外 text 项；顶层
// duration/resolution 直接落到载荷顶层字段。
func TestConvertToRequestPayloadContentPassthrough(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:      "doubao-seedance-2-0-260128",
		Resolution: "720p",
		Duration:   10,
		Content: []relaycommon.TaskContentItem{
			{Type: "text", Text: lo.ToPtr("跟随参考风格的舞蹈表演")},
			{Type: "video_url", VideoURL: &relaycommon.TaskMediaURL{URL: lo.ToPtr("https://example.com/motion_ref.mp4")}, Role: lo.ToPtr("reference_video")},
		},
	}

	r, err := adaptor.convertToRequestPayload(&req)
	require.NoError(t, err)

	assert.Equal(t, req.Content, r.Content)
	require.NotNil(t, r.Duration)
	assert.Equal(t, 10, int(*r.Duration))
	assert.Equal(t, "720p", r.Resolution)
}

// TestConvertToRequestPayloadLegacyPromptOwnsText 锁定旧式契约：无客户端
// content 时 prompt 独占唯一的 text 项；seconds 作为 duration 兜底。
func TestConvertToRequestPayloadLegacyPromptOwnsText(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:   "doubao-seedance-1-5-pro",
		Prompt:  "animate the first frame",
		Images:  []string{"https://example.com/first.png"},
		Seconds: "8",
	}

	r, err := adaptor.convertToRequestPayload(&req)
	require.NoError(t, err)

	require.Len(t, r.Content, 2)
	assert.Equal(t, "image_url", r.Content[0].Type)
	assert.Equal(t, "text", r.Content[1].Type)
	assert.Equal(t, "animate the first frame", *r.Content[1].Text)
	require.NotNil(t, r.Duration)
	assert.Equal(t, 8, int(*r.Duration))
}

// TestConvertToRequestPayloadRatioDefault 锁定 ratio 转发契约：客户端显式
// 传 ratio 时原样下发；缺失时按 Ark 规范默认 "adaptive"，避免上游报
// "ratio is required"。
func TestConvertToRequestPayloadRatioDefault(t *testing.T) {
	adaptor := &TaskAdaptor{}

	explicit := relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-0-260128",
		Prompt: "p",
		Ratio:  lo.ToPtr("16:9"),
	}
	r, err := adaptor.convertToRequestPayload(&explicit)
	require.NoError(t, err)
	assert.Equal(t, "16:9", r.Ratio)

	absent := relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-0-260128",
		Prompt: "p",
	}
	r, err = adaptor.convertToRequestPayload(&absent)
	require.NoError(t, err)
	assert.Equal(t, "adaptive", r.Ratio)

	// 显式空串不是合法 ratio，与缺省一样按 Ark 规范落到 adaptive。
	explicitEmpty := relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-0-260128",
		Prompt: "p",
		Ratio:  lo.ToPtr(""),
	}
	r, err = adaptor.convertToRequestPayload(&explicitEmpty)
	require.NoError(t, err)
	assert.Equal(t, "adaptive", r.Ratio)
}

// TestParseTaskResultExpiredIsFailure 是回归测试：上游 expired/cancelled
// 是终态，必须映射为失败，否则 default 分支会让任务永远按 IN_PROGRESS 轮询。
func TestParseTaskResultExpiredIsFailure(t *testing.T) {
	adaptor := &TaskAdaptor{}

	taskInfo, err := adaptor.ParseTaskResult([]byte(`{"id":"t1","status":"expired"}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusFailure, taskInfo.Status)
	assert.NotEmpty(t, taskInfo.Reason)

	taskInfo, err = adaptor.ParseTaskResult([]byte(`{"id":"t1","status":"cancelled","error":{"code":"user_cancel","message":"cancelled by user"}}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusFailure, taskInfo.Status)
	assert.Equal(t, "cancelled by user", taskInfo.Reason)
}

// TestConvertToOpenAIVideoFailureStatuses 是回归测试：失败/过期/取消等终态
// 映射为 FAILURE 后，转换出的视频必须携带 terminal error（优先上游错误信息，
// 为空时退回状态文案）；成功任务不得携带 error。
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
			data:        `{"id":"t1","status":"failed","error":{"code":"content_filter","message":"内容审核失败"}}`,
			status:      model.TaskStatusFailure,
			wantMessage: "内容审核失败",
		},
		{
			name:        "expired without message falls back to status text",
			data:        `{"id":"t1","status":"expired"}`,
			status:      model.TaskStatusFailure,
			wantMessage: "task expired",
		},
		{
			name:        "cancelled carries upstream message",
			data:        `{"id":"t1","status":"cancelled","error":{"code":"user_cancel","message":"cancelled by user"}}`,
			status:      model.TaskStatusFailure,
			wantMessage: "cancelled by user",
		},
		{
			name:        "success has no error",
			data:        `{"id":"t1","status":"succeeded","content":{"video_url":"https://cdn.example.com/v.mp4"}}`,
			status:      model.TaskStatusSuccess,
			wantMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originTask := &model.Task{
				TaskID:     "task_123",
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
			name:       "queued maps to queued",
			task:       newTask(`{"id":"t1","status":"queued"}`, model.TaskStatusQueued),
			wantStatus: dto.ArkVideoStatusQueued,
		},
		{
			name:       "running maps to running",
			task:       newTask(`{"id":"t1","status":"processing"}`, model.TaskStatusInProgress),
			wantStatus: dto.ArkVideoStatusRunning,
		},
		{
			name:           "succeeded carries url/duration/usage",
			task:           newTask(`{"id":"t1","status":"succeeded","content":{"video_url":"https://cdn.example.com/v.mp4"},"duration":5,"usage":{"completion_tokens":109431}}`, model.TaskStatusSuccess),
			wantStatus:     dto.ArkVideoStatusSucceeded,
			wantVideoURL:   "https://cdn.example.com/v.mp4",
			wantDuration:   5,
			wantCompletion: 109431,
		},
		{
			name:          "failed carries video_task_failed",
			task:          newTask(`{"id":"t1","status":"failed","error":{"code":"content_filter","message":"内容审核失败"}}`, model.TaskStatusFailure),
			wantStatus:    dto.ArkVideoStatusFailed,
			wantErrorCode: dto.ArkVideoErrorFailed,
			wantErrorMsg:  "内容审核失败",
		},
		{
			name:          "expired maps to expired with dedicated code",
			task:          newTask(`{"id":"t1","status":"expired"}`, model.TaskStatusFailure),
			wantStatus:    dto.ArkVideoStatusExpired,
			wantErrorCode: dto.ArkVideoErrorExpired,
			wantErrorMsg:  "task expired",
		},
		{
			name:          "cancelled maps to failed",
			task:          newTask(`{"id":"t1","status":"cancelled","error":{"code":"user_cancel","message":"cancelled by user"}}`, model.TaskStatusFailure),
			wantStatus:    dto.ArkVideoStatusFailed,
			wantErrorCode: dto.ArkVideoErrorFailed,
			wantErrorMsg:  "cancelled by user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := adaptor.ConvertToArkVideo(tt.task)
			require.NoError(t, err)

			var task dto.ArkVideoTask
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

// TestDoResponseArkSubmitShape 锁定 Ark 风格端点提交响应形态：
// /v1/videos/generations/tasks 返回 {id, model, status:"queued", created_at}，
// 不携带 OpenAI 视频字段（object/progress/task_id）与上游原始 task ID。
func TestDoResponseArkSubmitShape(t *testing.T) {
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
		Body:       io.NopCloser(strings.NewReader(`{"id":"upstream_456"}`)),
	}

	taskID, _, taskErr := adaptor.DoResponse(context, resp, info)

	require.Nil(t, taskErr)
	assert.Equal(t, "upstream_456", taskID)
	var task dto.ArkVideoTask
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &task))
	assert.Equal(t, "vt_123", task.ID)
	assert.Equal(t, "doubao-seedance-2-0-260128", task.Model)
	assert.Equal(t, dto.ArkVideoStatusQueued, task.Status)
	assert.Greater(t, task.CreatedAt, int64(0))
	// 提交响应只含 id/model/status/created_at，无 OpenAI 视频字段。
	assert.Empty(t, task.UpdatedAt)
	assert.Nil(t, task.Content)
	assert.Nil(t, task.Error)
	assert.NotContains(t, recorder.Body.String(), `"object"`)
	assert.NotContains(t, recorder.Body.String(), `"progress"`)
	assert.NotContains(t, recorder.Body.String(), "upstream_456")
}
