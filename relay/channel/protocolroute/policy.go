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
	"bytes"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// Policy 描述一个"上游可能同时说多种协议"的渠道如何把客户端协议映射到上游:
// 每条入站协议要么按原协议直达上游,要么转成 OpenAI chat 发出、再把响应转回。
// 渠道适配器把它作为包级变量持有,并把 Adaptor 的方法逐个转发到这里。
type Policy struct {
	// ChannelLabel 是错误文案前缀,如 "astraflow adaptor"。
	ChannelLabel string
	// LogLabel 是降级日志前缀,如 "astraflow"。
	LogLabel string
	// NativeWhenUndeclared 是"模型没有声明 model_protocols"时的基线: 列在这里的
	// 协议默认直达上游。声明过的模型一律以声明为准。
	NativeWhenUndeclared []string
	// DowngradeUndeclared 决定基线本身是否也受降级记忆约束。为真时,未声明的模型
	// 撞上游拒绝后同样会被记住并降级——基线是"上游应该支持"的推断,拒绝就是推断被
	// 证伪的证据。为假时基线是硬承诺,不受记忆影响(AstraFlow 的历史语义)。
	DowngradeUndeclared bool
	// SkipEmptyBearer 为真时,渠道 key 为空就不发 Authorization 头
	// (上游无鉴权,如本地 SGLang/vLLM)。
	SkipEmptyBearer bool
}

// useNativeProtocol 报告本次请求能否直连上游的该协议。
// 必须区分"未命中声明"与"命中但不含该协议": 两者的现状基线可能不同。未配置时
// 即按 NativeWhenUndeclared 走,保证零回归;命中时以声明为准。基线是否也吃降级
// 记忆由 DowngradeUndeclared 决定,见该字段。
func (p Policy) useNativeProtocol(info *relaycommon.RelayInfo, protocol string) bool {
	if info == nil || info.ChannelMeta == nil {
		return slices.Contains(p.NativeWhenUndeclared, protocol)
	}
	protocols, declared := info.ChannelOtherSettings.ResolveModelProtocols(upstreamModelID(info))
	if !declared {
		if !slices.Contains(p.NativeWhenUndeclared, protocol) {
			return false
		}
		// 基线是"上游应该支持"的推断: 不再吃记忆的策略(如 AstraFlow)到此为止。
		return !p.DowngradeUndeclared || !nativeProtocolDowngraded(info, protocol)
	}
	return slices.Contains(protocols, protocol) && !nativeProtocolDowngraded(info, protocol)
}

// nativeMessagesRequest 报告本次 /v1/messages 会话是否直连上游的 Anthropic 端点。
// 排除 RelayModeResponses*: 全局 ChatCompletionsToResponsesPolicy 会把 Claude 格式
// 请求改道 Responses 协议,那类会话收发都是 Responses 形状,不走 Anthropic 端点。
// 三处必须同判——GetRequestURL 选路径、SetupRequestHeader 补 Claude 协议头、
// DoResponse 选响应处理器,判法不一致会让请求与响应解析错配。
func (p Policy) nativeMessagesRequest(info *relaycommon.RelayInfo) bool {
	return info != nil &&
		info.RelayFormat == types.RelayFormatClaude &&
		info.RelayMode != constant.RelayModeResponses &&
		info.RelayMode != constant.RelayModeResponsesCompact &&
		p.useNativeProtocol(info, dto.ModelProtocolMessages)
}

// nativeGeminiRequest 报告本次 gemini 文本会话能否直连上游 gemini 端点: 模型声明了
// 原生 gemini,或请求体被透传(host 不调用 ConvertGeminiRequest,上游收到的就是客户端
// 原始 gemini 报文,只能按 gemini 端点收发)。否则按基线降级为 chat。
func (p Policy) nativeGeminiRequest(info *relaycommon.RelayInfo) bool {
	if path, action := geminiRequestPath(info); path == "" || !isGeminiTextAction(action) {
		return false
	}
	return p.useNativeProtocol(info, dto.ModelProtocolGemini) || bodyPassthroughEnabled(info)
}

// responsesDowngradedToChat 报告本次会话的 Responses 请求是否需要降级为 chat 发出
// (请求体转 chat、打 chat 端点、响应由 OaiChatToResponses* 转回)。
// 只有客户端确实以 Responses 协议发起的会话才降级: 全局
// ChatCompletionsToResponsesPolicy 会把 chat/messages 客户端改道到 Responses 协议
// (relay/chat_completions_via_responses.go),那类会话的 RelayMode 同样是 Responses,
// 但上游报文本来就是 Responses,响应侧由 host 的 OaiResponsesToChat* 固定按
// Responses 解析且不经过本 Policy 的 DoResponse——改了请求形状就会与响应解析错配。
// 请求体透传时同样不降级: host 不会调用 ConvertOpenAIResponsesRequest
// (relay/responses_handler.go),上游收到的是客户端原始的 Responses 报文,
// 只能打 Responses 端点。
func (p Policy) responsesDowngradedToChat(info *relaycommon.RelayInfo) bool {
	return info != nil &&
		info.RelayFormat == types.RelayFormatOpenAIResponses &&
		info.RelayMode == constant.RelayModeResponses &&
		// 先于透传判断: ChannelMeta 为 nil 时 useNativeProtocol 容忍 nil,bodyPassthroughEnabled 同样容忍。
		!p.useNativeProtocol(info, dto.ModelProtocolResponses) &&
		!bodyPassthroughEnabled(info)
}

// sentProtocol 返回本次请求实际发往上游的协议。
func (p Policy) sentProtocol(info *relaycommon.RelayInfo) string {
	if info.RelayMode == constant.RelayModeResponses {
		return dto.ModelProtocolResponses
	}
	if info.RelayFormat == types.RelayFormatClaude {
		return dto.ModelProtocolMessages
	}
	if p.nativeGeminiRequest(info) {
		return dto.ModelProtocolGemini
	}
	return dto.ModelProtocolChat
}

// DetectRejectedNativeProtocol 在上游非 2xx 且错误体命中"协议不支持"签名时,把
// (渠道, 模型, 协议) 记入降级记忆。读取的响应体用 MultiReader 还原,不影响后续
// service.RelayErrorHandler 解析。
func (p Policy) DetectRejectedNativeProtocol(info *relaycommon.RelayInfo, resp *http.Response) {
	if info == nil || info.ChannelMeta == nil || resp == nil || resp.Body == nil || resp.StatusCode < 400 {
		return
	}
	// 渠道测试用的是合成请求(header 透传同样对测试请求短路,
	// relay/channel/api_request.go),它命中的拒绝不构成"上游不支持该协议"的证据,
	// 不得因此永久降级生产流量的 (渠道, 模型, 协议)。
	if info.IsChannelTest {
		return
	}
	// 请求体透传时上游收到的就是客户端原始报文,形状改不了,降级无从谈起:
	// 记了也不会生效,只会在日志里误报"已降级"。
	if bodyPassthroughEnabled(info) {
		return
	}
	protocol := p.sentProtocol(info)
	if protocol == dto.ModelProtocolChat || !p.useNativeProtocol(info, protocol) {
		return
	}
	peeked, err := io.ReadAll(io.LimitReader(resp.Body, nativeProtocolBodyPeekLimit))
	if err != nil {
		return
	}
	resp.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(peeked), resp.Body), resp.Body}
	if !upstreamRejectsProtocol(string(peeked)) {
		return
	}
	key := nativeProtocolMemoKey(info, protocol)
	if _, loaded := nativeProtocolDowngradeMemo.LoadOrStore(key, time.Now()); loaded {
		return
	}
	common.SysLog(fmt.Sprintf(
		"%s: upstream rejected native %s (channel_id=%d model=%s status=%d), downgrading subsequent requests to chat: %s",
		p.LogLabel, protocol, info.ChannelId, upstreamModelID(info), resp.StatusCode, common.LocalLogPreview(string(peeked)),
	))
}
