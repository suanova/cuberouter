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
package protocolroute

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/common_handler"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/security_setting"

	"github.com/gin-gonic/gin"
)

// GetRequestURL 按本次会话实际要发的协议选上游地址: 原生协议透传客户端路径,
// 降级会话则指向 chat 端点(请求体已由 ConvertXxxRequest 转成 chat)。
func (p Policy) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", fmt.Errorf("%s: relay info is nil", p.ChannelLabel)
	}
	if info.ChannelBaseUrl == "" {
		return "", fmt.Errorf("%s: channel base url is empty", p.ChannelLabel)
	}
	// 非任务中继会把渠道 API key 作为 Bearer 凭证发送到上游，拒绝明文上游
	// （https 或回环地址除外），避免凭证被明文传输（CWE-319）。AllowCleartextUpstream
	// 只豁免不携带凭证的上游——本地 SGLang/vLLM 惯例是无鉴权；只要渠道会发出
	// Authorization(API key 或头覆盖)，仍按安全设置校验，见该字段。
	if !p.AllowCleartextUpstream || info.ApiKey != "" || hasAuthorizationOverride(info) {
		if err := common.ValidateHTTPSChannelBaseURL(info.ChannelBaseUrl, security_setting.GetSecuritySetting().RequireHTTPSChannelBaseURL); err != nil {
			return "", fmt.Errorf("%s: %w", p.ChannelLabel, err)
		}
	}
	requestPath := info.RequestURLPath
	if requestPath == "" {
		return info.ChannelBaseUrl, nil
	}
	// gemini 原生格式: 文本生成 action 要么按声明直连上游 gemini 端点(与渠道类型 24
	// 同形),要么在未声明时降级到 chat 端点(请求体已被 ConvertGeminiRequest 转成 chat)。
	// 两类 gemini 路径都重建 URL: 客户端查询串里的 ?key=<网关令牌> 属于网关鉴权,
	// 不得转给上游。
	if path, action := geminiRequestPath(info); path != "" {
		if !isGeminiTextAction(action) {
			// 非文本 action(embedding/countTokens 等): 沿用客户端路径。
			return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, path, info.ChannelType), nil
		}
		if !p.nativeGeminiRequest(info) {
			return fmt.Sprintf("%s/v1/chat/completions", info.ChannelBaseUrl), nil
		}
		geminiAction := geminiActionGenerateContent
		if action == geminiActionStreamGenerateContent || info.IsStream {
			// 客户端用 alt=sse 表达流式时按流式 action 发上游(与渠道类型 24 一致);
			// 原生 gemini 流式还要关掉网关 ping——gemini 客户端会逐条 JSON 解析 SSE 事件。
			geminiAction = geminiActionStreamGenerateContent + "?alt=sse"
			info.DisablePing = true
		}
		version := model_setting.GetGeminiVersionSetting(info.UpstreamModelName)
		return fmt.Sprintf("%s/%s/models/%s:%s", info.ChannelBaseUrl, version, upstreamModelID(info), geminiAction), nil
	}
	// /v1/messages (RelayFormatClaude): 声明原生支持 messages 就透传客户端路径，
	// 否则请求体已被 ConvertClaudeRequest 转成 OpenAI 格式,必须打到 chat 端点。
	if info.RelayFormat == types.RelayFormatClaude &&
		info.RelayMode != constant.RelayModeResponses &&
		info.RelayMode != constant.RelayModeResponsesCompact {
		if p.nativeMessagesRequest(info) {
			return relaycommon.GetFullRequestURL(info.ChannelBaseUrl+p.MessagesPathPrefix, requestPath, info.ChannelType), nil
		}
		return fmt.Sprintf("%s/v1/chat/completions", info.ChannelBaseUrl), nil
	}
	// responses: 声明原生支持就透传,否则请求体已转成 chat,必须打到 chat 端点。
	if p.responsesDowngradedToChat(info) {
		return fmt.Sprintf("%s/v1/chat/completions", info.ChannelBaseUrl), nil
	}
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, requestPath, info.ChannelType), nil
}

// hasAuthorizationOverride 报告渠道是否配置了 Authorization 头覆盖。覆盖值本身就是
// 凭证, 因此明文上游校验必须把它和 API key 同等对待。
func hasAuthorizationOverride(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	for k := range info.HeadersOverride {
		if strings.EqualFold(k, "Authorization") {
			return true
		}
	}
	return false
}

func (p Policy) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	// 非任务模式（chat/responses/embeddings/image）需要携带渠道鉴权头；
	// 若渠道已配置 Authorization Header Override 则跳过默认 Bearer，避免覆盖自定义鉴权。
	hasAuthOverride := hasAuthorizationOverride(info)
	// SkipEmptyBearer 的上游本来就不校验凭证(本地 SGLang/vLLM),空 key 时不要把
	// 一个空的 Bearer 送到上游。
	if !hasAuthOverride && !(p.SkipEmptyBearer && info.ApiKey == "") {
		req.Set("Authorization", "Bearer "+info.ApiKey)
	}
	// 直连上游 Anthropic 端点时必须带上 Claude 协议头(与 claude 适配器一致):
	// 沿用客户端的 anthropic-version,缺省 2023-06-01;anthropic-beta 与渠道级
	// Claude 头一并转发。转换路径上游只听 OpenAI 协议、responses 路径是 Responses
	// 报文,都不加这些头。
	if p.nativeMessagesRequest(info) {
		anthropicVersion := c.Request.Header.Get("anthropic-version")
		if anthropicVersion == "" {
			anthropicVersion = "2023-06-01"
		}
		req.Set("anthropic-version", anthropicVersion)
		claude.CommonClaudeHeadersOperation(c, req, info)
	}
	return nil
}

func (p Policy) DoRequest(a channel.Adaptor, c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	resp, err := channel.DoApiRequest(a, c, info, requestBody)
	if err != nil {
		return resp, err
	}
	p.DetectRejectedNativeProtocol(info, resp)
	return resp, nil
}

func (p Policy) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	// responses 的报文只有 Responses 处理器认得: chat 处理器会把响应体按 chat 解析,
	// 非流式拿不到 usage,流式会把 response.completed 收尾判成不完整流。
	// 声明原生 messages 的模型只在本次会话确实是 Anthropic Messages 时才交给
	// claude 处理器: 全局 ChatCompletionsToResponsesPolicy 会把 Claude 格式请求
	// 改道 Responses 协议(relay/claude_handler.go),那类响应不是 Anthropic 形状。
	switch {
	case p.nativeMessagesRequest(info):
		return (&claude.Adaptor{}).DoResponse(c, resp, info)
	case p.nativeGeminiRequest(info):
		// 上游收到的是 gemini 原生报文: 交给 gemini 适配器按 action 选处理器。
		// 降级会话(转成 chat 发出)不走这里——chat 报文由默认分支的 openai 处理器
		// 按 RelayFormatGemini 转回 gemini 形状。
		return (&gemini.Adaptor{}).DoResponse(c, resp, info)
	case info.RelayMode == constant.RelayModeResponses:
		if p.responsesDowngradedToChat(info) {
			// 上游收到的是 chat 响应,转成 Responses 形状给客户端。
			if info.IsStream {
				return openai.OaiChatToResponsesStreamHandler(c, info, resp)
			}
			return openai.OaiChatToResponsesHandler(c, info, resp)
		}
		if info.IsStream {
			return openai.OaiResponsesStreamHandler(c, info, resp)
		}
		return openai.OaiResponsesHandler(c, info, resp)
	case info.RelayMode == constant.RelayModeImagesGenerations || info.RelayMode == constant.RelayModeImagesEdits:
		// images 的响应是 OpenAI Images 形状(usage 用 input_tokens/output_tokens,
		// 没有 choices),chat 处理器读不到 usage 会退回估算;交给图片处理器解析,
		// 与 openai 适配器的图片分支保持一致。
		if info.IsStream {
			return openai.OpenaiImageStreamHandler(c, info, resp)
		}
		return openai.OpenaiImageHandler(c, info, resp)
	case info.RelayMode == constant.RelayModeRerank:
		// rerank 的响应是 Jina/Cohere 形状(results + relevance_score),不是 OpenAI
		// chat 报文,chat 处理器解析不出 usage;交给 Rerank 处理器解析并回写。
		return common_handler.RerankHandler(c, info, resp)
	default:
		if info.IsStream {
			return openai.OaiStreamHandler(c, info, resp)
		}
		return openai.OpenaiHandler(c, info, resp)
	}
}

func (p Policy) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	// 声明原生支持 messages 的模型: 请求体原样转发,响应由 claude 处理器解析。
	if p.useNativeProtocol(info, dto.ModelProtocolMessages) {
		return request, nil
	}
	// 上游只说 OpenAI 协议: 把 Anthropic 请求体转成 OpenAI chat 请求，
	// 由 GetRequestURL 指向 /v1/chat/completions；响应侧由 openai handler
	// 按 RelayFormatClaude 转回 Anthropic 格式（含流式逐块转换）。
	return downgradeToChatRequest(c, info, request)
}

func (p Policy) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	// 声明原生支持 gemini 的模型: 请求体交给 gemini 适配器做与渠道类型 24 相同的
	// 形状归一化后原样转发,响应由 gemini 处理器解析。
	if p.useNativeProtocol(info, dto.ModelProtocolGemini) {
		return (&gemini.Adaptor{}).ConvertGeminiRequest(c, info, request)
	}
	// 只有文本生成 action 参与降级: 其余 action(countTokens 等)两条路都不通, 保持
	// 快速失败——否则转换后的 chat 报体会被发到客户端的 gemini action 路径上。
	if _, action := geminiRequestPath(info); !isGeminiTextAction(action) {
		return nil, errors.New("not implemented")
	}
	// 上游只说 OpenAI 协议: 把 gemini 请求体转成 OpenAI chat 请求，由 GetRequestURL
	// 指向 /v1/chat/completions；响应侧由 openai 处理器按 RelayFormatGemini 转回
	// gemini 形状（含流式逐块转换）。
	return downgradeToChatRequest(c, info, request)
}

func (p Policy) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	// 声明原生支持 responses 的模型(含未配置时的基线)、以及改道来的会话
	// (见 responsesDowngradedToChat): 请求体原样转发。
	if !p.responsesDowngradedToChat(info) {
		return request, nil
	}
	// 其余模型: 降级为 chat 请求,响应侧由 OaiChatToResponses* 转回 Responses 形状。
	return downgradeToChatRequest(c, info, &request)
}
