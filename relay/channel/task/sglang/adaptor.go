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
// Package sglang 为 SGLang Diffusion（sglang serve）的 OpenAI 兼容视频
// API 提供任务适配器：POST /v1/videos 提交，GET /v1/videos/{id} 轮询，
// GET /v1/videos/{id}/content 取片。
//
// MiniMax-H3 等模型要求顶层的模型专属字段（task/conditions/target），
// 统一任务体里不存在这些字段，因此由本适配器在服务端合成：
//   - 无图 → task=t2va；有图 → task=fl2va，首帧/末帧写入 conditions
//   - size 档位 → target.short_edge，时长 → seconds + target.duration_seconds
//   - num_inference_steps / flow_shift / audio_flow_shift / seed 取
//     SGLang H3 cookbook 默认值，可用请求 metadata 同名字段覆盖
//
// H3 的时长上限是 15 秒，远小于网关通用的 MaxTaskDurationSeconds；
// 预扣费按请求时长计费，因此 4..15 的边界在请求校验阶段直接拒绝
// （400），而不是提交后被上游拒绝再退款。
package sglang

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// H3 采样参数默认值，与 SGLang MiniMax-H3 cookbook 的验证 profile 一致。
const (
	defaultDurationSeconds   = 5
	defaultShortEdge         = 768
	defaultAspectRatio       = "16:9"
	defaultNumInferenceSteps = 50
	defaultFlowShift         = 12.0
	defaultAudioFlowShift    = 3.0
)

// H3 请求参数边界（cookbook 验证档位）。
const (
	minDurationSeconds   = 4
	maxDurationSeconds   = 15
	minShortEdge         = 256
	maxShortEdge         = 2160
	maxNumInferenceSteps = 150
	maxNumOutputs        = 4
)

// supportedTasks 是 H3 接受的任务类型。
var supportedTasks = map[string]struct{}{
	"t2va":   {},
	"fl2va":  {},
	"ref2va": {},
}

// h3Condition 是 conditions 数组的单条条件（首帧/末帧关键帧）。
type h3Condition struct {
	Type       string `json:"type"`
	Uri        string `json:"uri"`
	Role       string `json:"role"`
	FrameIndex *int64 `json:"frame_index,omitempty"`
}

// h3Target 描述输出画布：短边像素、宽高比、时长。
type h3Target struct {
	ShortEdge       int     `json:"short_edge,omitempty"`
	AspectRatio     string  `json:"aspect_ratio,omitempty"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
}

// h3Request 是 POST /v1/videos 的 MiniMax-H3 请求体。
type h3Request struct {
	Model               string        `json:"model"`
	Prompt              string        `json:"prompt"`
	Seconds             int           `json:"seconds,omitempty"`
	Task                string        `json:"task"`
	Conditions          []h3Condition `json:"conditions"`
	Target              *h3Target     `json:"target"`
	NumOutputsPerPrompt int           `json:"num_outputs_per_prompt"`
	NumInferenceSteps   int           `json:"num_inference_steps"`
	FlowShift           float64       `json:"flow_shift"`
	AudioFlowShift      float64       `json:"audio_flow_shift"`
	Seed                *int64        `json:"seed,omitempty"`
}

// h3Metadata 是允许经统一请求 metadata 覆盖的采样参数（model 字段由
// taskcommon.UnmarshalMetadata 强制剥离，防止绕过计费）。duration /
// resolution 与计费侧（VideoPriceRatiosFromTaskContext）保持同一优先级，
// 保证预扣费档位与实际提交档位一致。
type h3Metadata struct {
	Task                *string  `json:"task"`
	AspectRatio         *string  `json:"aspect_ratio"`
	ShortEdge           *int     `json:"short_edge"`
	Duration            *int     `json:"duration"`
	Resolution          *string  `json:"resolution"`
	NumInferenceSteps   *int     `json:"num_inference_steps"`
	FlowShift           *float64 `json:"flow_shift"`
	AudioFlowShift      *float64 `json:"audio_flow_shift"`
	Seed                *int64   `json:"seed"`
	NumOutputsPerPrompt *int     `json:"num_outputs_per_prompt"`
}

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

// GetChannelName returns the channel name.
func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

// GetModelList returns the list of supported models.
func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

// EstimateBilling 模型命中视频按秒表时按请求推导计费系数；未配表返回 nil，
// 保持倍率模型的差额结算路径不变。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	return helper.VideoPriceRatiosFromTaskContext(c, info.OriginModelName)
}

// resolveMetadata 解析统一请求里的 metadata 覆盖项。
func resolveMetadata(req relaycommon.TaskSubmitReq) (h3Metadata, error) {
	var meta h3Metadata
	if len(req.Metadata) > 0 {
		if err := taskcommon.UnmarshalMetadata(req.Metadata, &meta); err != nil {
			return meta, err
		}
	}
	return meta, nil
}

// effectiveDuration 按 metadata.duration > duration > seconds 的优先级取
// 生效时长（秒），与计费侧一致；均未提供时返回 0（由调用方填默认值）。
func effectiveDuration(meta h3Metadata, req relaycommon.TaskSubmitReq) int {
	if meta.Duration != nil {
		return *meta.Duration
	}
	if req.Duration > 0 {
		return req.Duration
	}
	if seconds := strings.TrimSpace(req.Seconds); seconds != "" {
		if v, err := strconv.Atoi(seconds); err == nil {
			return v
		}
	}
	return 0
}

// effectiveResolution 按 metadata.resolution > resolution > size 的优先级
// 取生效档位（如 "720p"），与计费侧一致。
func effectiveResolution(meta h3Metadata, req relaycommon.TaskSubmitReq) string {
	if meta.Resolution != nil && strings.TrimSpace(*meta.Resolution) != "" {
		return strings.TrimSpace(*meta.Resolution)
	}
	if strings.TrimSpace(req.Resolution) != "" {
		return strings.TrimSpace(req.Resolution)
	}
	return strings.TrimSpace(req.Size)
}

// ValidateRequestAndSetAction 解析并校验统一任务体，拒绝超出 H3 档位
// 的请求（时长 4..15s、短边/步数/输出数上界、task 白名单）。
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError {
	if taskErr := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate); taskErr != nil {
		return taskErr
	}
	return a.validateH3Bounds(c)
}

// validateH3Bounds 校验 metadata 覆盖项与 H3 档位的边界；请求缺失或
// metadata 解析失败时返回 400。
func (a *TaskAdaptor) validateH3Bounds(c *gin.Context) *taskdto.TaskError {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapper(err, "invalid_request", http.StatusBadRequest)
	}
	meta, err := resolveMetadata(req)
	if err != nil {
		return service.TaskErrorWrapper(err, "invalid_request", http.StatusBadRequest)
	}

	if seconds := effectiveDuration(meta, req); seconds > 0 && (seconds < minDurationSeconds || seconds > maxDurationSeconds) {
		return service.TaskErrorWrapper(
			fmt.Errorf("duration must be between %d and %d seconds for MiniMax H3", minDurationSeconds, maxDurationSeconds),
			"invalid_duration", http.StatusBadRequest)
	}
	if meta.ShortEdge != nil && (*meta.ShortEdge < minShortEdge || *meta.ShortEdge > maxShortEdge) {
		return service.TaskErrorWrapper(
			fmt.Errorf("short_edge must be between %d and %d", minShortEdge, maxShortEdge),
			"invalid_short_edge", http.StatusBadRequest)
	}
	if meta.NumInferenceSteps != nil && (*meta.NumInferenceSteps < 1 || *meta.NumInferenceSteps > maxNumInferenceSteps) {
		return service.TaskErrorWrapper(
			fmt.Errorf("num_inference_steps must be between 1 and %d", maxNumInferenceSteps),
			"invalid_num_inference_steps", http.StatusBadRequest)
	}
	if meta.NumOutputsPerPrompt != nil && (*meta.NumOutputsPerPrompt < 1 || *meta.NumOutputsPerPrompt > maxNumOutputs) {
		return service.TaskErrorWrapper(
			fmt.Errorf("num_outputs_per_prompt must be between 1 and %d", maxNumOutputs),
			"invalid_num_outputs_per_prompt", http.StatusBadRequest)
	}
	if meta.Task != nil {
		task := strings.TrimSpace(*meta.Task)
		if _, ok := supportedTasks[task]; !ok {
			return service.TaskErrorWrapper(
				fmt.Errorf("task %q is not supported; supported tasks: t2va, fl2va, ref2va", task),
				"invalid_task", http.StatusBadRequest)
		}
	}
	return nil
}

// BuildRequestURL 构造 SGLang 视频提交端点。
func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/v1/videos", strings.TrimRight(a.baseURL, "/")), nil
}

// BuildRequestHeader 设置请求头；SGLang 本地服务通常无鉴权，渠道 key
// 非空时以 Bearer 传递。
func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	if a.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+a.apiKey)
	}
	return nil
}

// shortEdgeFromSize 把统一 size 档位映射为 H3 target.short_edge。
// H3 base 的验证档位为 512/768 短边；720p 及以上统一取 768。
func shortEdgeFromSize(size string) int {
	switch strings.ToLower(strings.TrimSpace(size)) {
	case "360p", "540p":
		return 512
	default:
		return defaultShortEdge
	}
}

// collectImageURIs 收集图生视频输入：images 数组（校验阶段已把单图
// image 归一进来）及其余兜底。
func collectImageURIs(req relaycommon.TaskSubmitReq) []string {
	uris := make([]string, 0, len(req.Images)+1)
	for _, uri := range req.Images {
		if strings.TrimSpace(uri) != "" {
			uris = append(uris, uri)
		}
	}
	if len(uris) == 0 && strings.TrimSpace(req.Image) != "" {
		uris = append(uris, req.Image)
	}
	return uris
}

// BuildRequestBody 把统一任务体合成为 SGLang H3 请求体。
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	meta, err := resolveMetadata(req)
	if err != nil {
		return nil, err
	}

	images := collectImageURIs(req)

	task := "t2va"
	if len(images) > 0 {
		task = "fl2va"
	}
	if meta.Task != nil && strings.TrimSpace(*meta.Task) != "" {
		task = strings.TrimSpace(*meta.Task)
	}

	seconds := effectiveDuration(meta, req)
	if seconds <= 0 {
		seconds = defaultDurationSeconds
	}

	// H3 关键帧只接受 frame_index 0（首帧）与 -1（末帧）。
	zero := int64(0)
	minusOne := int64(-1)
	conditions := make([]h3Condition, 0, len(images))
	for i, uri := range images {
		var frameIndex *int64
		switch i {
		case 0:
			frameIndex = &zero
		case 1:
			frameIndex = &minusOne
		default:
			continue
		}
		conditions = append(conditions, h3Condition{Type: "image", Uri: uri, Role: "keyframe", FrameIndex: frameIndex})
	}

	shortEdge := shortEdgeFromSize(effectiveResolution(meta, req))
	if meta.ShortEdge != nil && *meta.ShortEdge > 0 {
		shortEdge = *meta.ShortEdge
	}

	aspectRatio := defaultAspectRatio
	if len(images) > 0 {
		// 关键帧任务跟随输入画幅。
		aspectRatio = "auto"
	}
	if meta.AspectRatio != nil && strings.TrimSpace(*meta.AspectRatio) != "" {
		aspectRatio = strings.TrimSpace(*meta.AspectRatio)
	}

	numInferenceSteps := defaultNumInferenceSteps
	if meta.NumInferenceSteps != nil && *meta.NumInferenceSteps > 0 {
		numInferenceSteps = *meta.NumInferenceSteps
	}
	flowShift := defaultFlowShift
	if meta.FlowShift != nil {
		flowShift = *meta.FlowShift
	}
	audioFlowShift := defaultAudioFlowShift
	if meta.AudioFlowShift != nil {
		audioFlowShift = *meta.AudioFlowShift
	}
	numOutputs := 1
	if meta.NumOutputsPerPrompt != nil && *meta.NumOutputsPerPrompt > 0 {
		numOutputs = *meta.NumOutputsPerPrompt
	}

	body := h3Request{
		Model:               info.UpstreamModelName,
		Prompt:              req.Prompt,
		Seconds:             seconds,
		Task:                task,
		Conditions:          conditions,
		Target:              &h3Target{ShortEdge: shortEdge, AspectRatio: aspectRatio, DurationSeconds: float64(seconds)},
		NumOutputsPerPrompt: numOutputs,
		NumInferenceSteps:   numInferenceSteps,
		FlowShift:           flowShift,
		AudioFlowShift:      audioFlowShift,
		Seed:                meta.Seed,
	}
	if body.Model == "" {
		body.Model = req.Model
	}

	payload, err := common.Marshal(body)
	if err != nil {
		return nil, errors.Wrap(err, "marshal sglang request body failed")
	}
	return strings.NewReader(string(payload)), nil
}

type submitResponse struct {
	ID     string `json:"id"`
	TaskID string `json:"task_id"`
}

// DoRequest 发送提交请求。
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// ParseResponse 解析提交响应，取上游任务 id。
func (a *TaskAdaptor) ParseResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*channel.TaskSubmitResponse, *taskdto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	var sResp submitResponse
	if err := common.Unmarshal(responseBody, &sResp); err != nil {
		return nil, service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
	}

	upstreamID := sResp.ID
	if upstreamID == "" {
		upstreamID = sResp.TaskID
	}
	if upstreamID == "" {
		return nil, service.TaskErrorWrapper(errors.New("task_id is empty"), "invalid_response", http.StatusInternalServerError)
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	return &channel.TaskSubmitResponse{
		UpstreamTaskID: upstreamID,
		TaskData:       responseBody,
		ClientResponse: ov,
	}, nil
}

// FetchTask 查询上游任务状态。
func (a *TaskAdaptor) FetchTask(baseURL, key string, task *model.Task, proxy string) (*http.Response, error) {
	if task == nil || task.GetUpstreamTaskID() == "" {
		return nil, errors.New("missing upstream task id")
	}
	requestURL := fmt.Sprintf("%s/v1/videos/%s", strings.TrimRight(baseURL, "/"), url.PathEscape(task.GetUpstreamTaskID()))
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, errors.Wrap(err, "new proxy http client failed")
	}
	return client.Do(req)
}

type pollError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type pollResponse struct {
	ID       string     `json:"id"`
	Status   string     `json:"status"`
	Progress *float64   `json:"progress"`
	Error    *pollError `json:"error"`
}

// contentURL 由轮询请求地址推导取片端点；ResultURL 写入该值后由网关
// /v1/videos/{task_id}/content 代理回传 MP4（浏览器无需直连上游内网）。
func contentURL(resp *http.Response, task *model.Task) string {
	if resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return ""
	}
	return fmt.Sprintf("%s://%s/v1/videos/%s/content",
		resp.Request.URL.Scheme, resp.Request.URL.Host, url.PathEscape(task.GetUpstreamTaskID()))
}

// ParseTaskResult 解析轮询响应为统一任务状态。
func (a *TaskAdaptor) ParseTaskResult(task *model.Task, resp *http.Response, respBody []byte) (*relaycommon.TaskInfo, error) {
	var pResp pollResponse
	if err := common.Unmarshal(respBody, &pResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	status := strings.ToLower(strings.TrimSpace(pResp.Status))
	taskResult := &relaycommon.TaskInfo{Code: 0}

	applyProgress := func(base string) {
		if pResp.Progress != nil && *pResp.Progress > 0 {
			taskResult.Progress = fmt.Sprintf("%.0f%%", *pResp.Progress)
		} else {
			taskResult.Progress = base
		}
	}

	switch status {
	case "queued", "submitted", "pending":
		taskResult.Status = model.TaskStatusQueued
		applyProgress(taskcommon.ProgressQueued)
	case "in_progress", "processing", "running":
		taskResult.Status = model.TaskStatusInProgress
		applyProgress(taskcommon.ProgressInProgress)
	case "completed", "succeeded", "success":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = taskcommon.ProgressComplete
		taskResult.Url = contentURL(resp, task)
	case "failed", "error", "cancelled", "canceled", "expired":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = taskcommon.ProgressComplete
		reason := "upstream task failed"
		if pResp.Error != nil && pResp.Error.Message != "" {
			reason = pResp.Error.Message
		}
		taskResult.Reason = reason
	default:
		// 未知状态视为在途，交由轮询服务按未识别状态处理。
		taskResult.Status = model.TaskStatusInProgress
		applyProgress(taskcommon.ProgressInProgress)
	}
	return taskResult, nil
}
