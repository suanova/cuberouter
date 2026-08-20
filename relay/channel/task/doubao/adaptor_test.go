package doubao

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

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
