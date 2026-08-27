package astraflow

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertOpenAIRequestPassthrough 锁定 OpenAI 直传契约：消息请求必须原样
// 转发给上游，nil 请求必须报错而不是 panic。
func TestConvertOpenAIRequestPassthrough(t *testing.T) {
	adaptor := &Adaptor{}
	request := &dto.GeneralOpenAIRequest{Model: "deepseek-v3"}

	out, err := adaptor.ConvertOpenAIRequest(nil, nil, request)
	require.NoError(t, err)
	got, ok := out.(*dto.GeneralOpenAIRequest)
	require.True(t, ok, "expected *dto.GeneralOpenAIRequest, got %T", out)
	assert.Equal(t, request, got)

	_, err = adaptor.ConvertOpenAIRequest(nil, nil, nil)
	require.Error(t, err)
}

// TestConvertOpenAIResponsesRequestPassthrough 锁定 /v1/responses 直传契约：
// Responses 请求必须原样转发给上游，而不是返回 not implemented。
func TestConvertOpenAIResponsesRequestPassthrough(t *testing.T) {
	adaptor := &Adaptor{}
	request := dto.OpenAIResponsesRequest{Model: "deepseek-v3"}

	out, err := adaptor.ConvertOpenAIResponsesRequest(nil, nil, request)
	require.NoError(t, err)
	got, ok := out.(dto.OpenAIResponsesRequest)
	require.True(t, ok, "expected dto.OpenAIResponsesRequest, got %T", out)
	assert.Equal(t, request, got)
}

// TestConvertEmbeddingRequestPassthrough 锁定 /v1/embeddings 直传契约：
// Embedding 请求必须原样转发给上游，而不是返回 not implemented。
func TestConvertEmbeddingRequestPassthrough(t *testing.T) {
	adaptor := &Adaptor{}
	request := dto.EmbeddingRequest{Model: "text-embedding-3-large", Input: []string{"hello"}}

	out, err := adaptor.ConvertEmbeddingRequest(nil, nil, request)
	require.NoError(t, err)
	got, ok := out.(dto.EmbeddingRequest)
	require.True(t, ok, "expected dto.EmbeddingRequest, got %T", out)
	assert.Equal(t, request, got)
}

// TestConvertImageRequestPassthrough 锁定 /v1/images/generations 直传契约：
// Image 请求必须原样转发给上游，而不是返回 not implemented。
func TestConvertImageRequestPassthrough(t *testing.T) {
	adaptor := &Adaptor{}
	request := dto.ImageRequest{Model: "gpt-image-1", Prompt: "a cat on a boat"}

	out, err := adaptor.ConvertImageRequest(nil, nil, request)
	require.NoError(t, err)
	got, ok := out.(dto.ImageRequest)
	require.True(t, ok, "expected dto.ImageRequest, got %T", out)
	assert.Equal(t, request, got)
}

// TestGetRequestURLForwardsRequestPath 锁定 URL 直传契约：AstraFlow 各模式下
// 上游 URL 直接沿用客户端请求路径（/v1/chat/completions、/v1/responses、
// /v1/embeddings、/v1/images/generations），保证多模态请求到达正确端点。
func TestGetRequestURLForwardsRequestPath(t *testing.T) {
	adaptor := &Adaptor{}

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "chat completions", path: "/v1/chat/completions", want: "https://api.modelverse.cn/v1/chat/completions"},
		{name: "responses", path: "/v1/responses", want: "https://api.modelverse.cn/v1/responses"},
		{name: "embeddings", path: "/v1/embeddings", want: "https://api.modelverse.cn/v1/embeddings"},
		{name: "image generations", path: "/v1/images/generations", want: "https://api.modelverse.cn/v1/images/generations"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl: "https://api.modelverse.cn",
					ChannelType:    constant.ChannelTypeAstraFlow,
				},
				RequestURLPath: tt.path,
			}

			url, err := adaptor.GetRequestURL(info)
			require.NoError(t, err)
			assert.Equal(t, tt.want, url)
		})
	}
}
