package dto

// APIResponse 是 /api 下对外接口的统一响应形状(success/message/data),
// 与 controller 中 gin.H 的拼装方式一致,供 swagger 注解引用。
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
