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
	"fmt"
	"net/url"
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/gin-gonic/gin"
)

// upstreamModelID 返回发往上游的模型名。
func upstreamModelID(info *relaycommon.RelayInfo) string {
	if info != nil && info.UpstreamModelName != "" {
		return info.UpstreamModelName
	}
	if info == nil {
		return ""
	}
	return info.OriginModelName
}

const (
	geminiActionGenerateContent       = "generateContent"
	geminiActionStreamGenerateContent = "streamGenerateContent"
)

// geminiRequestPath 返回去掉查询串的客户端 gemini 路径与其中的 action(最后一个 ':'
// 之后的部分);非 gemini 格式返回空路径。客户端查询串只承载鉴权(?key=<网关令牌>)等
// 传输信息,不转发给上游;分隔符可能被客户端转义成 %3A,而路由与请求校验用的是解码后
// 的路径,这里同样解码后再取 action。
func geminiRequestPath(info *relaycommon.RelayInfo) (path string, action string) {
	if info == nil || info.RelayFormat != types.RelayFormatGemini || info.RequestURLPath == "" {
		return "", ""
	}
	path = info.RequestURLPath
	if idx := strings.IndexByte(path, '?'); idx >= 0 {
		path = path[:idx]
	}
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	return path, path[strings.LastIndex(path, ":")+1:]
}

// isGeminiTextAction 报告 action 是否是 gemini 文本生成。只有这两种 action 参与
// "直传 or 转 chat"的分流,其余 action 在两条路上都不通,保持快速失败。
func isGeminiTextAction(action string) bool {
	return action == geminiActionGenerateContent || action == geminiActionStreamGenerateContent
}

// bodyPassthroughEnabled 报告请求体是否被原样透传(全局或渠道级开关): 此时 host
// 不会调用 ConvertXxxRequest,上游收到的是客户端原始报文。容忍 ChannelMeta 为 nil。
func bodyPassthroughEnabled(info *relaycommon.RelayInfo) bool {
	if model_setting.GetGlobalSettings().PassThroughRequestEnabled {
		return true
	}
	return info != nil && info.ChannelMeta != nil && info.ChannelSetting.PassThroughBodyEnabled
}

// downgradeToChatRequest 把非 chat 协议的请求体转成 OpenAI chat 请求(各降级路径共用,
// 由 GetRequestURL 指向 chat 端点)。流式请求要带上 stream_options.include_usage:
// 否则上游不回 usage, 结算只能按已中继文本估算。
func downgradeToChatRequest(c *gin.Context, info *relaycommon.RelayInfo, request any) (*dto.GeneralOpenAIRequest, error) {
	result, err := service.ConvertRequest(c, info, types.RelayFormatOpenAI, request)
	if err != nil {
		return nil, err
	}
	aiRequest, ok := result.Value.(*dto.GeneralOpenAIRequest)
	if !ok {
		return nil, fmt.Errorf("expected OpenAI chat completions request, got %T", result.Value)
	}
	if info.SupportStreamOptions && info.IsStream {
		aiRequest.StreamOptions = &dto.StreamOptions{IncludeUsage: true}
	}
	return aiRequest, nil
}
