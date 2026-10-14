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
package cubestack

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptorMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &Adaptor{}
	assert.Equal(t, "CubeStack", adaptor.GetChannelName())
	assert.Empty(t, adaptor.GetModelList(),
		"text models come from the upstream's --served-model-name, the video model belongs to the task adaptor")
	adaptor.Init(nil)
}

// TestLocalConvertersPassRequestThrough 锁定本渠道不改造请求体: chat/图像/重排/向量
// 都原样交给上游，只有语音在两条路径上都不通、保持快速失败。
func TestLocalConvertersPassRequestThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	adaptor := &Adaptor{}

	chatRequest := &dto.GeneralOpenAIRequest{Model: "qwen38-27b"}
	converted, err := adaptor.ConvertOpenAIRequest(context, nil, chatRequest)
	require.NoError(t, err)
	assert.Same(t, chatRequest, converted)

	imageRequest := dto.ImageRequest{}
	converted, err = adaptor.ConvertImageRequest(context, nil, imageRequest)
	require.NoError(t, err)
	assert.Equal(t, imageRequest, converted)

	embeddingRequest := dto.EmbeddingRequest{Model: "bge-m3"}
	converted, err = adaptor.ConvertEmbeddingRequest(context, nil, embeddingRequest)
	require.NoError(t, err)
	assert.Equal(t, embeddingRequest, converted)

	rerankRequest := dto.RerankRequest{}
	converted, err = adaptor.ConvertRerankRequest(context, relayconstant.RelayModeRerank, rerankRequest)
	require.NoError(t, err)
	assert.Equal(t, rerankRequest, converted)

	_, err = adaptor.ConvertOpenAIRequest(context, nil, nil)
	require.ErrorContains(t, err, "request is nil")

	_, err = adaptor.ConvertAudioRequest(context, nil, dto.AudioRequest{})
	require.ErrorContains(t, err, "not implemented")
}

// TestGetRequestURLRequiresRelayInfoAndBaseURL 锁定错误文案带渠道前缀: 两个渠道类型
// 共用同一个 Policy，报错必须能区分是谁发出去的。
func TestGetRequestURLRequiresRelayInfoAndBaseURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &Adaptor{}

	_, err := adaptor.GetRequestURL(nil)
	require.ErrorContains(t, err, "cubestack adaptor: relay info is nil")

	_, err = adaptor.GetRequestURL(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}})
	require.ErrorContains(t, err, "cubestack adaptor: channel base url is empty")
}

// TestGeminiIsOptInNotBaseline 锁定 gemini 与三条文本协议的区别: 基线里没有它，
// 未声明时文本生成 action 会被转成 chat 打到 /v1/chat/completions；只有渠道显式
// 声明 gemini 才直连上游的 :generateContent 端点。非文本 action 不参与分流，
// 恒沿用客户端路径。
func TestGeminiIsOptInNotBaseline(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const baseURL = "http://127.0.0.1:5001"

	newGeminiInfo := func(protocols map[string][]string) *relaycommon.RelayInfo {
		return &relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelId:            301,
				ChannelBaseUrl:       baseURL,
				ChannelType:          constant.ChannelTypeCubeStack,
				ApiKey:               "",
				UpstreamModelName:    "qwen38-27b",
				ChannelOtherSettings: dto.ChannelOtherSettings{ModelProtocols: protocols},
			},
			RequestURLPath:  "/v1beta/models/qwen38-27b:generateContent",
			RelayFormat:     types.RelayFormatGemini,
			RelayMode:       relayconstant.RelayModeChatCompletions,
			OriginModelName: "qwen38-27b",
		}
	}

	adaptor := &Adaptor{}

	url, err := adaptor.GetRequestURL(newGeminiInfo(nil))
	require.NoError(t, err)
	assert.Equal(t, baseURL+"/v1/chat/completions", url,
		"gemini must not be part of the cubestack baseline")

	url, err = adaptor.GetRequestURL(newGeminiInfo(map[string][]string{"qwen38-27b": {dto.ModelProtocolGemini}}))
	require.NoError(t, err)
	assert.Contains(t, url, "/models/qwen38-27b:generateContent")
	assert.NotContains(t, url, "/v1/chat/completions")

	nonText := newGeminiInfo(nil)
	nonText.RequestURLPath = "/v1beta/models/qwen38-27b:countTokens"
	url, err = adaptor.GetRequestURL(nonText)
	require.NoError(t, err)
	assert.Contains(t, url, ":countTokens", "non-text actions keep the client path on both branches")
}

// TestConvertGeminiRequestRejectsNonTextAction 锁定 gemini 非文本 action 的快速失败:
// 未声明 gemini 时它既不能原生转发也不该被转成 chat(转完会被发到客户端那个 action
// 路径上)，必须直接报错。
func TestConvertGeminiRequestRejectsNonTextAction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	context, _ := gin.CreateTestContext(httptest.NewRecorder())

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         302,
			ChannelBaseUrl:    "http://127.0.0.1:5001",
			ChannelType:       constant.ChannelTypeCubeStack,
			UpstreamModelName: "qwen38-27b",
		},
		RequestURLPath:  "/v1beta/models/qwen38-27b:countTokens",
		RelayFormat:     types.RelayFormatGemini,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		OriginModelName: "qwen38-27b",
	}

	_, err := (&Adaptor{}).ConvertGeminiRequest(context, info, &dto.GeminiChatRequest{})
	require.ErrorContains(t, err, "not implemented")
}
