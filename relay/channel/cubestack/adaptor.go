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
	"errors"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/protocolroute"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

// policy 是本渠道的多协议路由决策。基线是三条文本协议全部原生: 上游（vLLM/SGLang）
// 原生提供 /v1/chat/completions、/v1/responses, Anthropic Messages 挂在
// /anthropic/v1/messages 前缀下, 因此**不需要任何配置**就能三条都按原协议直达;
// 只有上游明确回"不支持该协议"时, 该 (渠道, 模型, 协议) 才会在进程内被记住并降级为
// 转 chat。gemini 不在基线里, 需要时按 model_protocols 声明。
// 路由、Header、转换与响应分派都在 protocolroute 里, 与 AstraFlow 共用。
var policy = protocolroute.Policy{
	ChannelLabel: "cubestack adaptor",
	LogLabel:     "cubestack",
	NativeWhenUndeclared: []string{
		dto.ModelProtocolChat,
		dto.ModelProtocolResponses,
		dto.ModelProtocolMessages,
	},
	// 上游把 Anthropic API 挂在 /anthropic 下, 而 OpenAI 协议仍在根路径:
	// 原生 messages 需要加这个前缀才能命中路由, chat/responses 不受影响。
	MessagesPathPrefix: "/anthropic",
	// 本地 SGLang/vLLM 上游通常不校验凭证(与视频侧 task/cubestack 一致):
	// 渠道 key 为空时不要把一个空的 Bearer 发给上游。
	SkipEmptyBearer: true,
	// 基线同样吃降级记忆: 上面三条全是"上游应该原生支持"的推断(零配置即用),
	// 一旦上游明确回"不支持该协议",推断即被证伪,该组合转 chat 发出。
	DowngradeUndeclared: true,
	// 本渠道的上游就是本地/内网的 SGLang·vLLM, base URL 常为明文 http://<IP>:port;
	// 回环豁免覆盖不到内网地址, 而"防止渠道 key 明文出网"对本就无鉴权的本地上游
	// 并不成立, 故不做传输安全校验(security_setting 的全局开关对该渠道不生效)。
	AllowCleartextUpstream: true,
}

type Adaptor struct {
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return policy.GetRequestURL(info)
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	return policy.SetupRequestHeader(c, req, info)
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return policy.ConvertOpenAIResponsesRequest(c, info, request)
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	// 传 a: DoApiRequest 会回调 a.GetRequestURL / a.SetupRequestHeader。
	return policy.DoRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	return policy.DoResponse(c, resp, info)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return policy.ConvertClaudeRequest(c, info, request)
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return policy.ConvertGeminiRequest(c, info, request)
}

// Ensure compile-time interface check.
var _ channel.Adaptor = (*Adaptor)(nil)
