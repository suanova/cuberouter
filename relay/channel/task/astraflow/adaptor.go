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
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
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

// ============================
// kling 家族请求形态
// ============================

// kling（kling-v3 / kling-v3-omni）不走 Ark content 数组：上游要求
// input.prompt + parameters 的家族方言（首尾帧、参考图、模式、声音都在
// parameters 下；运动控制的参考图/视频在 input 下）。与 seedance 的 Ark
// 形态是两套参数体系，见 modelverse 文档 video_api/Kling-v3 与 Kling-O3。
type klingInput struct {
	Prompt string `json:"prompt,omitempty"`
	// 运动控制（motion_control）的参考图 / 参考视频。
	ImgURL   string `json:"img_url,omitempty"`
	VideoURL string `json:"video_url,omitempty"`
}

// klingParameters 是 kling 家族的 parameters 集合：统一请求字段与客户端
// metadata 共同填充（metadata 显式值优先）。
type klingParameters struct {
	KlingV3Type          string                 `json:"kling_v3_type,omitempty"`
	Image                string                 `json:"image,omitempty"`
	ImageTail            string                 `json:"image_tail,omitempty"`
	Mode                 string                 `json:"mode,omitempty"`
	Sound                string                 `json:"sound,omitempty"`
	AspectRatio          string                 `json:"aspect_ratio,omitempty"`
	Duration             *dto.IntValue          `json:"duration,omitempty"`
	CharacterOrientation string                 `json:"character_orientation,omitempty"`
	KeepOriginalSound    string                 `json:"keep_original_sound,omitempty"`
	MultiShot            *dto.BoolValue         `json:"multi_shot,omitempty"`
	ShotType             string                 `json:"shot_type,omitempty"`
	MultiPrompt          []klingMultiPromptShot `json:"multi_prompt,omitempty"`
	ImageList            []klingImageRef        `json:"image_list,omitempty"`
	VideoList            []klingVideoRef        `json:"video_list,omitempty"`
}

type klingImageRef struct {
	ImageURL string `json:"image_url"`
	Type     string `json:"type,omitempty"`
}

type klingVideoRef struct {
	VideoURL          string `json:"video_url"`
	ReferType         string `json:"refer_type,omitempty"`
	KeepOriginalSound string `json:"keep_original_sound,omitempty"`
}

type klingMultiPromptShot struct {
	Index    int    `json:"index"`
	Prompt   string `json:"prompt"`
	Duration string `json:"duration"`
}

// klingRequestPayload 是 kling 家族的提交体：
// { "model": "...", "input": {"prompt": ...}, "parameters": {...} }。
type klingRequestPayload struct {
	Model      string          `json:"model"`
	Input      klingInput      `json:"input"`
	Parameters klingParameters `json:"parameters,omitempty"`
}

const (
	klingModeStd     = "std"
	klingModePro     = "pro"
	klingSoundOn     = "on"
	klingSoundOff    = "off"
	klingAspectRatio = "16:9"
	klingDefaultSec  = 5
	klingTypeT2V     = "t2v"
	klingTypeI2V     = "i2v"
	klingTypeMotion  = "motion_control"
	klingRefYes      = "ref"
	klingRefNo       = "noref"
)

var (
	klingModes        = map[string]bool{klingModeStd: true, klingModePro: true, "4k": true}
	klingSounds       = map[string]bool{klingSoundOn: true, klingSoundOff: true}
	klingAspectRatios = map[string]bool{"16:9": true, "9:16": true, "1:1": true}
	klingTypes        = map[string]bool{klingTypeT2V: true, klingTypeI2V: true, klingTypeMotion: true}
)

// init 注册 kling 家族的计费事实 schema：表达式保存时按此校验 u() 键并生成
// 冒烟测试向量（setting/billing_setting.SmokeTestTaskExprWithFacts）。事实的
// 实际取值由 deriveKlingRequest 推导，两者的键集合必须一致。
func init() {
	billingexpr.RegisterTaskUsageSchema(isKlingModel, map[string]billingexpr.UsageFieldSchema{
		"seconds":   {Type: "number", Unit: "second"},
		"mode":      {Type: "string", Enum: []string{klingModeStd, klingModePro, "4k"}},
		"sound":     {Type: "string", Enum: []string{klingSoundOn, klingSoundOff}},
		"variant":   {Type: "string", Enum: []string{klingTypeT2V, klingTypeI2V, klingTypeMotion}},
		"reference": {Type: "string", Enum: []string{klingRefYes, klingRefNo}},
	})
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

// EstimateBilling 模型命中视频按秒表时按请求推导计费系数(seconds/size/time,
// 与 JS 插件任务适配器语义一致);未配表返回 nil,保持倍率模型的 token 差额
// 结算路径不变。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	return helper.VideoPriceRatiosFromTaskContext(c, info.OriginModelName)
}

// ValidateRequestAndSetAction parses body, validates fields and sets default action.
// Ark 风格 content 数组请求由本渠道接受。
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *taskdto.TaskError) {
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

	// 成形按映射后的上游模型名选择家族（kling 方言 / 其余按 Ark 形态）。
	model := req.Model
	if info.IsModelMapped {
		model = info.UpstreamModelName
	} else {
		info.UpstreamModelName = model
	}
	req.Model = model

	var body any
	if isKlingModel(model) {
		body, err = a.convertKlingRequestPayload(&req, model)
	} else {
		body, err = a.convertToRequestPayload(&req)
	}
	if err != nil {
		return nil, errors.Wrap(err, "convert request payload failed")
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

// ParseResponse handles the upstream submit response and returns the parsed
// task result. It never writes to the client response — the controller presents
// ClientResponse after the durable task barrier and billing settlement.
func (a *TaskAdaptor) ParseResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*channel.TaskSubmitResponse, *taskdto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	var sResp submitResponseBody
	if err := common.Unmarshal(responseBody, &sResp); err != nil {
		return nil, service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
	}

	if sResp.Output.TaskID == "" {
		return nil, service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
	}

	// Ark 风格端点（/v1/videos/generations/tasks）返回 Ark 提交形态，
	// 其余路径保持 OpenAI 视频格式。
	var clientResponse any
	if relaycommon.IsArkVideoPath(c) {
		clientResponse = taskdto.NewArkVideoSubmit(info.PublicTaskID, info.OriginModelName, time.Now().Unix())
	} else {
		ov := dto.NewOpenAIVideo()
		// 与其他 task 适配器保持一致：对外返回网关预生成的公开 task ID，
		// 上游原始 ID 由提交流程存入 PrivateData.UpstreamTaskID。
		ov.ID = info.PublicTaskID
		ov.TaskID = info.PublicTaskID
		ov.CreatedAt = time.Now().Unix()
		ov.Model = info.OriginModelName
		clientResponse = ov
	}

	return &channel.TaskSubmitResponse{
		UpstreamTaskID: sResp.Output.TaskID,
		TaskData:       responseBody,
		ClientResponse: clientResponse,
	}, nil
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
func (a *TaskAdaptor) FetchTask(baseUrl, key string, task *model.Task, proxy string) (*http.Response, error) {
	if task == nil || task.GetUpstreamTaskID() == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	taskID := task.GetUpstreamTaskID()

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
func (a *TaskAdaptor) ParseTaskResult(task *model.Task, resp *http.Response, respBody []byte) (*relaycommon.TaskInfo, error) {
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
		// 终态事实：结算时与提交事实合并后重跑表达式（例如上游修正后的时长）。
		if resTask.Usage.Duration > 0 {
			taskResult.UsageFacts = map[string]any{"seconds": float64(resTask.Usage.Duration)}
		}
	case "Failure", "Expired":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = resTask.Output.ErrorMessage
		if taskResult.Reason == "" {
			taskResult.Reason = fmt.Sprintf("task %s", strings.ToLower(resTask.Output.TaskStatus))
		}
	default:
		// 未知状态映射为 UNKNOWN：轮询层按失败预算处理（连续超限 → 失败并退款），
		// 不再当作处理中无限等待 24h 超时——否则上游的真实失败态会被永久静默。
		taskResult.Status = model.TaskStatusUnknown
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

	task := taskdto.ArkVideoTask{
		ID:        originTask.TaskID,
		Model:     originTask.Properties.OriginModelName,
		CreatedAt: originTask.CreatedAt,
		UpdatedAt: originTask.UpdatedAt,
	}

	switch originTask.Status {
	case model.TaskStatusQueued, model.TaskStatusSubmitted:
		task.Status = taskdto.ArkVideoStatusQueued
	case model.TaskStatusInProgress:
		task.Status = taskdto.ArkVideoStatusRunning
	case model.TaskStatusSuccess:
		task.Status = taskdto.ArkVideoStatusSucceeded
		if len(afResp.Output.URLs) > 0 {
			task.Content = &taskdto.ArkVideoContent{VideoURL: afResp.Output.URLs[0]}
		}
		if afResp.Usage.Duration > 0 {
			task.Output = &taskdto.ArkVideoOutput{Duration: afResp.Usage.Duration}
		}
		if afResp.Usage.CompletionTokens > 0 {
			task.Usage = &taskdto.ArkVideoUsage{CompletionTokens: afResp.Usage.CompletionTokens}
		}
	case model.TaskStatusFailure:
		// Expired 终态在 ParseTaskResult 阶段与 Failure 一并折叠为 FAILURE，
		// 这里依据缓存的原始上游状态区分 expired 与 failed。
		task.Status = taskdto.ArkVideoStatusFailed
		errorCode := taskdto.ArkVideoErrorFailed
		if afResp.Output.TaskStatus == "Expired" {
			task.Status = taskdto.ArkVideoStatusExpired
			errorCode = taskdto.ArkVideoErrorExpired
		}
		message := afResp.Output.ErrorMessage
		if message == "" {
			message = fmt.Sprintf("task %s", strings.ToLower(afResp.Output.TaskStatus))
		}
		task.Error = &taskdto.ArkVideoError{Code: errorCode, Message: message}
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
		if req.Resolution != "" {
			r.Parameters.Resolution = req.Resolution
		} else if tier := helper.VideoResolutionTier(req.Size); tier != "" {
			// OpenAI 风格请求只传 size(如 1920x1080)时归一到档位,让上游与
			// 计费(视频按秒表)按同一档位取值。
			r.Parameters.Resolution = tier
		}
	}

	// 首帧/首尾帧模式(内容含无角色或 first_frame/last_frame 标注的
	// image_url,含 OpenAI 风格 images 转换来的无角色图)的输出比例由首帧图
	// 决定,上游拒绝携带 ratio(InvalidParameter.TaskTypeConstraint),
	// 此时整段不发送——客户端显式值与默认 adaptive 都跳过。
	// reference_image 等参考角色不受此限,按文生视频路径处理。
	hasFrameImage := false
	for _, item := range r.Input.Content {
		if item.Type != "image_url" {
			continue
		}
		role := ""
		if item.Role != nil {
			role = *item.Role
		}
		if role == "" || role == "first_frame" || role == "last_frame" {
			hasFrameImage = true
			break
		}
	}
	if hasFrameImage {
		r.Parameters.Ratio = ""
	} else {
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

// ============================
// kling 家族：成形与计费事实
// ============================

// isKlingModel 判断模型是否属于 kling 家族（kling-v3 / kling-v3-omni）。
func isKlingModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(model), "kling")
}

// convertKlingRequestPayload 把统一任务请求成形为 kling 方言提交体。
func (a *TaskAdaptor) convertKlingRequestPayload(req *relaycommon.TaskSubmitReq, model string) (*klingRequestPayload, error) {
	payload, _, err := deriveKlingRequest(req, model)
	return payload, err
}

// klingUsageFacts 推导 kling 的计费事实（tiered_expr 的 u() 变量）：
// seconds/mode/sound 通用，kling-v3 另有 variant（t2v/i2v/motion_control），
// kling-v3-omni 另有 reference（ref/noref）。
func klingUsageFacts(req *relaycommon.TaskSubmitReq, model string) (map[string]any, error) {
	_, facts, err := deriveKlingRequest(req, model)
	return facts, err
}

// deriveKlingRequest 同时产出成形载荷与计费事实：两条链路共用同一份推导，
// 避免"发给上游的参数"与"计费读取的档位"漂移（例如时长、模式不一致）。
func deriveKlingRequest(req *relaycommon.TaskSubmitReq, model string) (*klingRequestPayload, map[string]any, error) {
	omni := strings.Contains(strings.ToLower(model), "omni")
	payload := &klingRequestPayload{Model: model}
	if err := req.UnmarshalMetadata(&payload.Parameters); err != nil {
		return nil, nil, errors.Wrap(err, "unmarshal kling metadata parameters failed")
	}
	// 运动控制的参考图/视频在 input 下，metadata 直传。
	var metaInput struct {
		ImgURL   string `json:"img_url"`
		VideoURL string `json:"video_url"`
	}
	if err := req.UnmarshalMetadata(&metaInput); err != nil {
		return nil, nil, errors.Wrap(err, "unmarshal kling metadata input failed")
	}
	payload.Input.Prompt = req.Prompt
	payload.Input.ImgURL = metaInput.ImgURL
	payload.Input.VideoURL = metaInput.VideoURL

	images := req.Images
	if len(images) == 0 && req.Image != "" {
		images = []string{req.Image}
	}

	// 时长优先级与 Ark 路径一致：metadata 覆盖 > 顶层 duration > seconds > 缺省 5。
	if payload.Parameters.Duration == nil {
		if req.Duration > 0 {
			payload.Parameters.Duration = lo.ToPtr(dto.IntValue(req.Duration))
		} else if sec, parseErr := strconv.Atoi(strings.TrimSpace(req.Seconds)); parseErr == nil && sec > 0 {
			payload.Parameters.Duration = lo.ToPtr(dto.IntValue(sec))
		}
	}
	if payload.Parameters.Duration == nil {
		payload.Parameters.Duration = lo.ToPtr(dto.IntValue(klingDefaultSec))
	}
	seconds := int(*payload.Parameters.Duration)
	if seconds <= 0 || seconds > relaycommon.MaxTaskDurationSeconds {
		return nil, nil, fmt.Errorf("kling: duration %d out of range", seconds)
	}

	// 模式：metadata > 顶层 mode > 家族默认（v3=std，omni=pro）。
	if payload.Parameters.Mode == "" {
		payload.Parameters.Mode = req.Mode
	}
	if payload.Parameters.Mode == "" {
		if omni {
			payload.Parameters.Mode = klingModePro
		} else {
			payload.Parameters.Mode = klingModeStd
		}
	}
	if !klingModes[payload.Parameters.Mode] {
		return nil, nil, fmt.Errorf("kling: unsupported mode %q", payload.Parameters.Mode)
	}

	// 声音：默认 off（计费按 sound 档位，非法值直接拒绝，不静默落到默认价）。
	if payload.Parameters.Sound == "" {
		payload.Parameters.Sound = klingSoundOff
	}
	if !klingSounds[payload.Parameters.Sound] {
		return nil, nil, fmt.Errorf("kling: unsupported sound %q", payload.Parameters.Sound)
	}

	// 宽高比：kling 只认 16:9/9:16/1:1；顶层 ratio 里非 kling 取值（adaptive 等）
	// 不转发；视频编辑（refer_type=base）时上游允许省略，此时不下发默认值。
	videoEditBase := false
	for _, v := range payload.Parameters.VideoList {
		if v.ReferType == "base" {
			videoEditBase = true
		}
	}
	if payload.Parameters.AspectRatio == "" && !videoEditBase {
		if req.Ratio != nil && klingAspectRatios[*req.Ratio] {
			payload.Parameters.AspectRatio = *req.Ratio
		} else {
			payload.Parameters.AspectRatio = klingAspectRatio
		}
	}

	if omni {
		// omni：统一 images 映射为 image_list——单图首帧、双图首尾帧、
		// 三张及以上不带类型（参考图）；metadata.image_list 显式值优先。
		if len(payload.Parameters.ImageList) == 0 && len(images) > 0 {
			list := make([]klingImageRef, 0, len(images))
			for i, u := range images {
				ref := klingImageRef{ImageURL: u}
				if len(images) == 1 || (len(images) == 2 && i == 0) {
					ref.Type = "first_frame"
				} else if len(images) == 2 && i == 1 {
					ref.Type = "end_frame"
				}
				list = append(list, ref)
			}
			payload.Parameters.ImageList = list
		}
	} else {
		// v3：显式 kling_v3_type 优先；否则按入参推导（有参考视频/图 → motion_control，
		// 有首帧图 → i2v，否则 t2v）。
		if payload.Parameters.KlingV3Type == "" {
			switch {
			case payload.Input.VideoURL != "" || payload.Input.ImgURL != "":
				payload.Parameters.KlingV3Type = klingTypeMotion
			case len(images) > 0 || payload.Parameters.Image != "":
				payload.Parameters.KlingV3Type = klingTypeI2V
			default:
				payload.Parameters.KlingV3Type = klingTypeT2V
			}
		}
		if !klingTypes[payload.Parameters.KlingV3Type] {
			return nil, nil, fmt.Errorf("kling: unsupported kling_v3_type %q", payload.Parameters.KlingV3Type)
		}
		if payload.Parameters.KlingV3Type == klingTypeI2V {
			if payload.Parameters.Image == "" && len(images) > 0 {
				payload.Parameters.Image = images[0]
			}
			if payload.Parameters.ImageTail == "" && len(images) > 1 {
				payload.Parameters.ImageTail = images[1]
			}
		}
		if payload.Parameters.KlingV3Type == klingTypeMotion && payload.Input.ImgURL == "" && len(images) > 0 {
			payload.Input.ImgURL = images[0]
		}
	}

	facts := map[string]any{
		"seconds": float64(seconds),
		"mode":    payload.Parameters.Mode,
		"sound":   payload.Parameters.Sound,
	}
	if omni {
		reference := klingRefNo
		if len(payload.Parameters.VideoList) > 0 {
			reference = klingRefYes
		} else {
			for _, ref := range payload.Parameters.ImageList {
				if ref.Type == "" {
					reference = klingRefYes
					break
				}
			}
		}
		// 显式覆盖：metadata.reference = ref/noref（歧义场景由调用方定档）。
		var metaFacts struct {
			Reference string `json:"reference"`
		}
		if err := req.UnmarshalMetadata(&metaFacts); err == nil && (metaFacts.Reference == klingRefYes || metaFacts.Reference == klingRefNo) {
			reference = metaFacts.Reference
		}
		facts["reference"] = reference
	} else {
		facts["variant"] = payload.Parameters.KlingV3Type
	}
	return payload, facts, nil
}

// ExtractUsageFacts 是计费事实接口的兼容包装（tiered 闸门按
// TaskUsageFactsProvider 判定，故两个接口都要实现）。
func (a *TaskAdaptor) ExtractUsageFacts(c *gin.Context, info *relaycommon.RelayInfo) map[string]any {
	facts, err := a.ExtractUsageFactsValidated(c, info)
	if err != nil {
		return map[string]any{}
	}
	return facts
}

// ExtractUsageFactsValidated 为 tiered_expr 任务计费提供事实：kling 家族给出
// seconds/mode/sound/variant|reference；其余模型返回空事实（非本适配器职责）。
func (a *TaskAdaptor) ExtractUsageFactsValidated(c *gin.Context, info *relaycommon.RelayInfo) (map[string]any, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	model := req.Model
	if info != nil && info.UpstreamModelName != "" {
		model = info.UpstreamModelName
	}
	if !isKlingModel(model) {
		return map[string]any{}, nil
	}
	return klingUsageFacts(&req, model)
}
