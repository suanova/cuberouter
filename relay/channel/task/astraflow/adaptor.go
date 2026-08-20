package astraflow

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

// ============================
// Request / Response structures
// ============================

// ContentItem、MediaURL 与网关统一的 Ark 内容项类型一致，
// 客户端直传的 content 数组可零拷贝透传。
type ContentItem = relaycommon.TaskContentItem

type MediaURL = relaycommon.TaskMediaURL

// taskInput 对应提交体中的 input 部分，与 Ark 的 content 数组一致。
type taskInput struct {
	Content []ContentItem `json:"content,omitempty"`
}

// taskParameters 与 Ark 顶层除 model/content 以外的参数一一对应，
// 嵌套在提交体的 parameters 之下。
type taskParameters struct {
	CallbackURL           string         `json:"callback_url,omitempty"`
	ReturnLastFrame       *dto.BoolValue `json:"return_last_frame,omitempty"`
	ServiceTier           string         `json:"service_tier,omitempty"`
	ExecutionExpiresAfter *dto.IntValue  `json:"execution_expires_after,omitempty"`
	GenerateAudio         *dto.BoolValue `json:"generate_audio,omitempty"`
	Draft                 *dto.BoolValue `json:"draft,omitempty"`
	Tools                 []struct {
		Type string `json:"type,omitempty"`
	} `json:"tools,omitempty"`
	SafetyIdentifier string         `json:"safety_identifier,omitempty"`
	Priority         *dto.IntValue  `json:"priority,omitempty"`
	Resolution       string         `json:"resolution,omitempty"`
	Ratio            string         `json:"ratio,omitempty"`
	Duration         *dto.IntValue  `json:"duration,omitempty"`
	Frames           *dto.IntValue  `json:"frames,omitempty"`
	Seed             *dto.IntValue  `json:"seed,omitempty"`
	CameraFixed      *dto.BoolValue `json:"camera_fixed,omitempty"`
	Watermark        *dto.BoolValue `json:"watermark,omitempty"`
}

// requestPayload 是提交任务请求体：
// { "model": "...", "input": {"content": [...]}, "parameters": {...} }。
type requestPayload struct {
	Model      string         `json:"model"`
	Input      taskInput      `json:"input"`
	Parameters taskParameters `json:"parameters,omitempty"`
}

type submitResponseBody struct {
	Output struct {
		TaskID string `json:"task_id"`
	} `json:"output"`
}

type queryTaskResponseBody struct {
	Output struct {
		TaskID       string   `json:"task_id"`
		TaskStatus   string   `json:"task_status"`
		URLs         []string `json:"urls"`
		ErrorMessage string   `json:"error_message"`
	} `json:"output"`
	Usage struct {
		Duration         int `json:"duration"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

// ValidateRequestAndSetAction parses body, validates fields and sets default action.
// Ark 风格 content 数组请求由本渠道接受。
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	return relaycommon.ValidateBasicTaskRequestWithArkContent(c, info, constant.TaskActionGenerate)
}

// BuildRequestURL constructs the upstream URL for submitting a task.
func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/v1/tasks/submit", a.baseURL), nil
}

// BuildRequestHeader sets required headers.
func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

// BuildRequestBody converts TaskSubmitReq into the provider's submit format.
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	body, err := a.convertToRequestPayload(&req)
	if err != nil {
		return nil, errors.Wrap(err, "convert request payload failed")
	}
	if info.IsModelMapped {
		body.Model = info.UpstreamModelName
	} else {
		info.UpstreamModelName = body.Model
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// DoRequest delegates to common helper.
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse handles upstream submit response, returns taskID etc.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	var sResp submitResponseBody
	if err := common.Unmarshal(responseBody, &sResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	if sResp.Output.TaskID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	// Ark 风格端点（/v1/videos/generations/tasks）直接返回 Ark 提交形态，
	// 其余路径保持 OpenAI 视频格式。
	if relaycommon.IsArkVideoPath(c) {
		c.JSON(http.StatusOK, dto.NewArkVideoSubmit(info.PublicTaskID, info.OriginModelName, time.Now().Unix()))
		return sResp.Output.TaskID, responseBody, nil
	}

	ov := dto.NewOpenAIVideo()
	// 与其他 task 适配器保持一致：对外返回网关预生成的公开 task ID，
	// 上游原始 ID 由提交流程存入 PrivateData.UpstreamTaskID。
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName
	c.JSON(http.StatusOK, ov)
	return sResp.Output.TaskID, responseBody, nil
}

// GetModelList returns the list of supported models.
func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

// GetChannelName returns the channel name.
func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

// ============================
// Task querying
// ============================

// FetchTask fetches task status from the upstream API.
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s/v1/tasks/status?task_id=%s", baseUrl, taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

// ParseTaskResult parses the task query response.
func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	resTask := queryTaskResponseBody{}
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	// 上游任务状态：Pending / Running / Success / Failure / Expired
	switch resTask.Output.TaskStatus {
	case "Pending":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "10%"
	case "Running":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case "Success":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		if len(resTask.Output.URLs) > 0 {
			taskResult.Url = resTask.Output.URLs[0]
		}
		// 解析 usage 信息用于按倍率计费。上游仅返回 completion_tokens，
		// 视频生成没有 prompt 用量，因此把它同时作为 total_tokens 参与
		// token 结算（与 doubao 适配器一致：成功时按 token 差额结算，
		// 而非一直保留预扣额度）。
		taskResult.CompletionTokens = resTask.Usage.CompletionTokens
		taskResult.TotalTokens = resTask.Usage.CompletionTokens
	case "Failure", "Expired":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = resTask.Output.ErrorMessage
		if taskResult.Reason == "" {
			taskResult.Reason = fmt.Sprintf("task %s", strings.ToLower(resTask.Output.TaskStatus))
		}
	default:
		// Unknown status, treat as processing
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}

	return &taskResult, nil
}

// ============================
// OpenAI video response conversion
// ============================

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var afResp queryTaskResponseBody
	if err := common.Unmarshal(originTask.Data, &afResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal task data failed")
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.TaskID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	if len(afResp.Output.URLs) > 0 {
		openAIVideo.SetMetadata("url", afResp.Output.URLs[0])
	}
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt
	openAIVideo.Model = originTask.Properties.OriginModelName

	if originTask.Status == model.TaskStatusFailure {
		// 失败/过期等终态折叠为失败后必须携带 terminal error，否则客户端会把
		// 失败任务当成成功处理。优先用上游错误信息，为空时退回状态文案
		// （与 ParseTaskResult 保持一致）。
		message := afResp.Output.ErrorMessage
		if message == "" {
			message = fmt.Sprintf("task %s", strings.ToLower(afResp.Output.TaskStatus))
		}
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: message,
			Code:    "task_failed",
		}
	}

	return common.Marshal(openAIVideo)
}

// ConvertToArkVideo 将任务转换为 Ark 风格视频响应（/v1/videos/generations/tasks
// 查询端点的对外返回）。与 ConvertToOpenAIVideo 共用 originTask.Data 中缓存的
// 上游任务快照；终态失败时按原始上游状态区分 expired / failed。
func (a *TaskAdaptor) ConvertToArkVideo(originTask *model.Task) ([]byte, error) {
	var afResp queryTaskResponseBody
	if err := common.Unmarshal(originTask.Data, &afResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal task data failed")
	}

	task := dto.ArkVideoTask{
		ID:        originTask.TaskID,
		Model:     originTask.Properties.OriginModelName,
		CreatedAt: originTask.CreatedAt,
		UpdatedAt: originTask.UpdatedAt,
	}

	switch originTask.Status {
	case model.TaskStatusQueued, model.TaskStatusSubmitted:
		task.Status = dto.ArkVideoStatusQueued
	case model.TaskStatusInProgress:
		task.Status = dto.ArkVideoStatusRunning
	case model.TaskStatusSuccess:
		task.Status = dto.ArkVideoStatusSucceeded
		if len(afResp.Output.URLs) > 0 {
			task.Content = &dto.ArkVideoContent{VideoURL: afResp.Output.URLs[0]}
		}
		if afResp.Usage.Duration > 0 {
			task.Output = &dto.ArkVideoOutput{Duration: afResp.Usage.Duration}
		}
		if afResp.Usage.CompletionTokens > 0 {
			task.Usage = &dto.ArkVideoUsage{CompletionTokens: afResp.Usage.CompletionTokens}
		}
	case model.TaskStatusFailure:
		// Expired 终态在 ParseTaskResult 阶段与 Failure 一并折叠为 FAILURE，
		// 这里依据缓存的原始上游状态区分 expired 与 failed。
		task.Status = dto.ArkVideoStatusFailed
		errorCode := dto.ArkVideoErrorFailed
		if afResp.Output.TaskStatus == "Expired" {
			task.Status = dto.ArkVideoStatusExpired
			errorCode = dto.ArkVideoErrorExpired
		}
		message := afResp.Output.ErrorMessage
		if message == "" {
			message = fmt.Sprintf("task %s", strings.ToLower(afResp.Output.TaskStatus))
		}
		task.Error = &dto.ArkVideoError{Code: errorCode, Message: message}
	}

	return common.Marshal(&task)
}

// ============================
// helpers
// ============================

// convertToRequestPayload 将网关统一请求转换为本渠道的提交格式：
// model 与 input.content[] 沿用 Ark 内容协议，其余 Ark 顶层参数嵌套在
// parameters 之下。duration 优先级：metadata 覆盖 > 顶层 duration > seconds。
func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq) (*requestPayload, error) {
	r := requestPayload{
		Model: req.Model,
	}

	if len(req.Content) > 0 {
		r.Input.Content = append(r.Input.Content, req.Content...)
	} else if req.HasImage() {
		for _, imgURL := range req.Images {
			r.Input.Content = append(r.Input.Content, ContentItem{
				Type:     "image_url",
				ImageURL: &MediaURL{URL: lo.ToPtr(imgURL)},
			})
		}
	}

	// metadata 与 Ark 顶层参数名对齐：content 整体替换 input.content，
	// 其余参数（duration/resolution/seed/...）落入 parameters。
	var metaShim struct {
		Content []ContentItem `json:"content,omitempty"`
	}
	if err := req.UnmarshalMetadata(&metaShim); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata content failed")
	}
	// 顶层 content 与 metadata.content 都是客户端直接给定的内容数组，
	// 其中的 text 项（即客户端提示词）都必须原样保留。
	contentFromClient := len(req.Content) > 0
	if len(metaShim.Content) > 0 {
		r.Input.Content = metaShim.Content
		contentFromClient = true
	}
	if err := req.UnmarshalMetadata(&r.Parameters); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata parameters failed")
	}

	if r.Parameters.Duration == nil {
		if req.Duration > 0 {
			r.Parameters.Duration = lo.ToPtr(dto.IntValue(req.Duration))
		} else if sec, _ := strconv.Atoi(req.Seconds); sec > 0 {
			r.Parameters.Duration = lo.ToPtr(dto.IntValue(sec))
		}
	}

	if r.Parameters.Resolution == "" {
		r.Parameters.Resolution = req.Resolution
	}

	// ratio 与 Ark 规范一致默认 "adaptive"：上游在字段缺失时报
	// "ratio is required"，因此显式下发规范默认值。req.Ratio 为指针，
	// 以区分客户端缺省（nil）与显式空串；两者都落到 adaptive，空串
	// 不是合法 ratio。
	if r.Parameters.Ratio == "" && req.Ratio != nil {
		r.Parameters.Ratio = *req.Ratio
	}
	if r.Parameters.Ratio == "" {
		r.Parameters.Ratio = "adaptive"
	}

	if contentFromClient {
		// content 直传时其中的 text 项即提示词；仅在非空 text 缺失且顶层
		// prompt 非空时才补齐，避免改写客户端给定的提示词。空的 text 项
		// 不视为已携带提示词，直接用顶层 prompt 覆盖或追加。
		hasText := false
		emptyTextIdx := -1
		for i, c := range r.Input.Content {
			if c.Type == "text" {
				if c.Text != nil && strings.TrimSpace(*c.Text) != "" {
					hasText = true
					break
				}
				if emptyTextIdx == -1 {
					emptyTextIdx = i
				}
			}
		}
		if !hasText && strings.TrimSpace(req.Prompt) != "" {
			if emptyTextIdx >= 0 {
				r.Input.Content[emptyTextIdx].Text = lo.ToPtr(req.Prompt)
			} else {
				r.Input.Content = append(r.Input.Content, ContentItem{Type: "text", Text: lo.ToPtr(req.Prompt)})
			}
		}
	} else {
		r.Input.Content = lo.Reject(r.Input.Content, func(c ContentItem, _ int) bool { return c.Type == "text" })
		r.Input.Content = append(r.Input.Content, ContentItem{
			Type: "text",
			Text: lo.ToPtr(req.Prompt),
		})
	}

	return &r, nil
}
