package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAstraFlowChannelRegistration 锁定 AstraFlow 渠道（59）在三层注册表中
// 的映射契约：渠道类型 → API 类型、端点类型。AstraFlow 的 Seedance 视频走
// Ark 风格任务端点（/v1/videos/generations/tasks），端点类型必须是
// ark-video，而不是 OpenAI video 任务端点（openai-video 归 Sora / doubao）。
func TestAstraFlowChannelRegistration(t *testing.T) {
	apiType, ok := ChannelType2APIType(constant.ChannelTypeAstraFlow)
	require.True(t, ok)
	assert.Equal(t, constant.APITypeAstraFlow, apiType)

	endpointTypes := GetEndpointTypesByChannelType(constant.ChannelTypeAstraFlow, "doubao-seedance-2-0-260128")
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeArkVideo}, endpointTypes)
}

// TestDoubaoVideoChannelRegistration 锁定 doubao-video 渠道（54）的端点类型
// 映射：OpenAI video 任务端点。此前遗漏该渠道导致定价页对 doubao 视频模型
// 展示聊天示例而非视频示例。
func TestDoubaoVideoChannelRegistration(t *testing.T) {
	endpointTypes := GetEndpointTypesByChannelType(constant.ChannelTypeDoubaoVideo, "doubao-seedance-2-0-260128")
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIVideo}, endpointTypes)
}

// TestAstraFlowMultiModelEndpointTypes 锁定 AstraFlow 渠道（59）的多模态端点
// 类型映射契约：视频模型（Seedance）走 Ark 风格任务端点（ark-video）；普通
// 文本/响应模型暴露 OpenAI 与 OpenAI 响应端点；response-only 模型只暴露响应
// 端点；生图模型额外前置 image-generation 端点。
func TestAstraFlowMultiModelEndpointTypes(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		wantTypes []constant.EndpointType
	}{
		{
			name:      "seedance video model uses ark-video",
			model:     "doubao-seedance-2-0-260128",
			wantTypes: []constant.EndpointType{constant.EndpointTypeArkVideo},
		},
		{
			name:      "chat model exposes openai and openai-response",
			model:     "deepseek-v3",
			wantTypes: []constant.EndpointType{constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIResponse},
		},
		{
			name:      "response-only model exposes openai-response only",
			model:     "o3-pro",
			wantTypes: []constant.EndpointType{constant.EndpointTypeOpenAIResponse},
		},
		{
			name:      "image generation model prepends image-generation",
			model:     "gpt-image-1",
			wantTypes: []constant.EndpointType{constant.EndpointTypeImageGeneration, constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIResponse},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpointTypes := GetEndpointTypesByChannelType(constant.ChannelTypeAstraFlow, tt.model)
			assert.Equal(t, tt.wantTypes, endpointTypes)
		})
	}
}

// TestVideoOnlyChannelsKeepOpenAIVideo 锁定拆分后的回归契约：Sora/DoubaoVideo
// 渠道不随模型名变化，始终只暴露 OpenAI video 任务端点。
func TestVideoOnlyChannelsKeepOpenAIVideo(t *testing.T) {
	for _, channelType := range []int{constant.ChannelTypeSora, constant.ChannelTypeDoubaoVideo} {
		endpointTypes := GetEndpointTypesByChannelType(channelType, "some-chat-model")
		assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIVideo}, endpointTypes)
	}
}

// TestAstraFlowEndpointTypeDiffersFromOpenAIVideo 锁定 AstraFlow（ark-video）
// 与 Sora（openai-video）的端点类型区分，防止回归到共用一个视频端点类型。
func TestAstraFlowEndpointTypeDiffersFromOpenAIVideo(t *testing.T) {
	astraFlow := GetEndpointTypesByChannelType(constant.ChannelTypeAstraFlow, "doubao-seedance-2-0-260128")
	sora := GetEndpointTypesByChannelType(constant.ChannelTypeSora, "sora-2")
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeArkVideo}, astraFlow)
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIVideo}, sora)
}
