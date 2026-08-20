package dto

// Ark 风格视频任务状态与错误码，对应 /v1/videos/generations/tasks 端点
// 对外的返回语义（与火山方舟 Seedance 一致）。
const (
	ArkVideoStatusQueued    = "queued"
	ArkVideoStatusRunning   = "running"
	ArkVideoStatusSucceeded = "succeeded"
	ArkVideoStatusFailed    = "failed"
	ArkVideoStatusExpired   = "expired"

	ArkVideoErrorFailed  = "video_task_failed"
	ArkVideoErrorExpired = "video_task_expired"
)

// ArkVideoTask 是 /v1/videos/generations/tasks 端点对外返回的任务形态。
// 任何状态都返回 id/status/model/created_at/updated_at；成功态补充
// content.video_url / output.duration / usage.completion_tokens；
// 终态失败返回 error{code,message}。
type ArkVideoTask struct {
	ID        string           `json:"id"`
	Model     string           `json:"model"`
	Status    string           `json:"status"`
	CreatedAt int64            `json:"created_at"`
	UpdatedAt int64            `json:"updated_at,omitempty"`
	Content   *ArkVideoContent `json:"content,omitempty"`
	Output    *ArkVideoOutput  `json:"output,omitempty"`
	Usage     *ArkVideoUsage   `json:"usage,omitempty"`
	Error     *ArkVideoError   `json:"error,omitempty"`
}

type ArkVideoContent struct {
	VideoURL string `json:"video_url"`
}

type ArkVideoOutput struct {
	Duration int `json:"duration"`
}

type ArkVideoUsage struct {
	CompletionTokens int `json:"completion_tokens"`
}

type ArkVideoError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewArkVideoSubmit 构建提交响应：id 为网关预生成的公开任务 ID（vt_...），
// 状态固定为 queued，created_at 由调用方传入当前时间戳。
func NewArkVideoSubmit(id, model string, createdAt int64) *ArkVideoTask {
	return &ArkVideoTask{
		ID:        id,
		Model:     model,
		Status:    ArkVideoStatusQueued,
		CreatedAt: createdAt,
	}
}
