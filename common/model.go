package common

import "strings"

var (
	// OpenAIResponseOnlyModels is a list of models that are only available for OpenAI responses.
	OpenAIResponseOnlyModels = []string{
		"o3-pro",
		"o3-deep-research",
		"o4-mini-deep-research",
	}
	// 这里是按模型名推断的老机制，只保留上游命名约定明确的系列。
	// 曾包含 "qwen-image"，但该子串同时命中只支持编辑的 qwen-image-edit-*，
	// 会把编辑模型当成文生图模型；qwen 系列改由模型元数据标签声明，见下方
	// ModelTagTextToImage。
	ImageGenerationModels = []string{
		"dall-e-3",
		"dall-e-2",
		"gpt-image-1",
		"prefix:imagen-",
		"flux-",
		"flux.1-",
	}
	OpenAITextModels = []string{
		"gpt-",
		"o1",
		"o3",
		"o4",
		"chatgpt",
	}
	VideoGenerationModels = []string{
		"seedance",
		"sora",
	}
)

// ModelTagTextToImage / ModelTagImageToImage 是声明模型图片能力的模型元数据标签，
// 由运维在「模型元数据」页填写，取代按模型名猜测：名字分不清生成与编辑
// （qwen-image-edit-* 同样含 "qwen-image"）。标签写入契约见模型元数据抽屉的
// TagInput——多个标签以逗号连接。
const (
	// ModelTagTextToImage 表示模型接受提示词出图，即 /v1/images/generations。
	ModelTagTextToImage = "text-to-image"
	// ModelTagImageToImage 表示模型接受提示词加参考图出图，即 /v1/images/edits。
	ModelTagImageToImage = "image-to-image"
)

// HasModelTag 判断逗号分隔的模型标签串中是否含有指定标签，忽略大小写与首尾空白。
// 只做整项相等比较，绝不做子串匹配：标签是运维的显式声明，不是名称猜测。
func HasModelTag(tags string, tag string) bool {
	if tags == "" || tag == "" {
		return false
	}
	for _, item := range strings.Split(tags, ",") {
		if strings.EqualFold(strings.TrimSpace(item), tag) {
			return true
		}
	}
	return false
}

func IsOpenAIResponseOnlyModel(modelName string) bool {
	for _, m := range OpenAIResponseOnlyModels {
		if strings.Contains(modelName, m) {
			return true
		}
	}
	return false
}

func IsImageGenerationModel(modelName string) bool {
	modelName = strings.ToLower(modelName)
	for _, m := range ImageGenerationModels {
		if strings.Contains(modelName, m) {
			return true
		}
		if strings.HasPrefix(m, "prefix:") && strings.HasPrefix(modelName, strings.TrimPrefix(m, "prefix:")) {
			return true
		}
	}
	return false
}

func IsOpenAITextModel(modelName string) bool {
	modelName = strings.ToLower(modelName)
	for _, m := range OpenAITextModels {
		if strings.Contains(modelName, m) {
			return true
		}
	}
	return false
}

// IsOpenAIVideoModel 判断模型是否为 OpenAI video 任务模型（Seedance/Sora 等）。
// 用于 AstraFlow 等多模态渠道在端点类型映射时区分视频模型与普通模型。
func IsOpenAIVideoModel(modelName string) bool {
	modelName = strings.ToLower(modelName)
	for _, m := range VideoGenerationModels {
		if strings.Contains(modelName, m) {
			return true
		}
	}
	return false
}
