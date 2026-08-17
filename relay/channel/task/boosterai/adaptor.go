package boosterai

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// ============================
// Request / Response structures
// ============================

// ContentItem represents an element in the content array.
// Based on Volcengine Ark content format: https://www.volcengine.com/docs/85621/1792707
type ContentItem struct {
	Type     string    `json:"type"`                // "text" | "image_url" | "video_url" | "audio_url"
	Text     string    `json:"text,omitempty"`      // for text type
	ImageURL *MediaURL `json:"image_url,omitempty"` // for image_url type
	VideoURL *MediaURL `json:"video_url,omitempty"` // for video_url type
	AudioURL *MediaURL `json:"audio_url,omitempty"` // for audio_url type
}

type MediaURL struct {
	URL string `json:"url"`
}

// requestPayload is the submit task request body.
type requestPayload struct {
	Model    string        `json:"model"`
	Content  []ContentItem `json:"content"`
	Duration int           `json:"duration"`
	Ratio    string        `json:"ratio"`
}

// submitResponseBody is the response from submitting a task.
type submitResponseBody struct {
	ID string `json:"id"`
}

// queryTaskResponseBody is the response from querying a task.
type queryTaskResponseBody struct {
	ID        string `json:"id"`
	Model     string `json:"model"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	Content   *struct {
		VideoURL string `json:"video_url"`
	} `json:"content,omitempty"`
	Duration   int    `json:"duration"`
	Resolution string `json:"resolution"`
	Ratio      string `json:"ratio"`
	Error      *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
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
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	return relaycommon.ValidateBasicTaskRequest(c, info, "")
}

// BuildRequestURL constructs the upstream URL for submitting a task.
func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/api/v3/contents/generations/tasks", a.baseURL), nil
}

// BuildRequestHeader sets required headers.
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

// BuildRequestBody converts TaskSubmitReq into BoosterAI/Seedance format.
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return nil, fmt.Errorf("request not found in context")
	}
	req, ok := v.(relaycommon.TaskSubmitReq)
	if !ok {
		return nil, fmt.Errorf("invalid request type in context")
	}

	body, err := a.convertToRequestPayload(&req)
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

// DoResponse handles upstream response, returns taskID etc.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	// Parse BoosterAI response
	var sResp submitResponseBody
	if err := common.Unmarshal(responseBody, &sResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	if sResp.ID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = sResp.ID
	ov.TaskID = sResp.ID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName
	c.JSON(http.StatusOK, ov)
	return sResp.ID, responseBody, nil
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

// FetchTask fetches task status from the BoosterAI API.
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s/api/v3/contents/generations/tasks/%s", baseUrl, taskID)

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

	// Map BoosterAI status to internal status
	// Statuses: queued, running, succeeded, failed, expired, cancelled
	switch resTask.Status {
	case "queued":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "10%"
	case "running":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case "succeeded":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		if resTask.Content != nil {
			taskResult.Url = resTask.Content.VideoURL
		}
	case "failed", "expired", "cancelled":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		if resTask.Error != nil {
			taskResult.Reason = resTask.Error.Message
		} else {
			taskResult.Reason = fmt.Sprintf("task %s", resTask.Status)
		}
	default:
		return nil, fmt.Errorf("unknown task status: %s", resTask.Status)
	}

	return &taskResult, nil
}

// ============================
// OpenAI video response conversion
// ============================

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var boosterResp queryTaskResponseBody
	if err := common.Unmarshal(originTask.Data, &boosterResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal boosterai task data failed")
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.CreatedAt = boosterResp.CreatedAt
	openAIVideo.CompletedAt = boosterResp.UpdatedAt

	if boosterResp.Content != nil && boosterResp.Content.VideoURL != "" {
		openAIVideo.SetMetadata("url", boosterResp.Content.VideoURL)
	}

	if boosterResp.Error != nil && boosterResp.Error.Message != "" {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: boosterResp.Error.Message,
			Code:    "task_failed",
		}
	}

	jsonData, _ := common.Marshal(openAIVideo)
	return jsonData, nil
}

// ============================
// helpers
// ============================

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq) (*requestPayload, error) {
	r := requestPayload{
		Model:    defaultString(req.Model, "seedance-2"),
		Duration: defaultInt(req.Duration, 15),
		Ratio:    defaultString(req.Size, "16:9"),
	}

	// Build content array
	// Must contain exactly one type=text item as the prompt
	// Optionally contains type=image_url items for reference images
	r.Content = append(r.Content, ContentItem{
		Type: "text",
		Text: req.Prompt,
	})

	// Add reference images if present
	for _, img := range req.Images {
		if strings.TrimSpace(img) == "" {
			continue
		}
		r.Content = append(r.Content, ContentItem{
			Type:     "image_url",
			ImageURL: &MediaURL{URL: img},
		})
	}

	// Allow metadata to override content fields (e.g., video_url, audio_url)
	if err := req.UnmarshalMetadata(&r); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}

	return &r, nil
}

func defaultString(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func defaultInt(v int, def int) int {
	if v == 0 {
		return def
	}
	return v
}
