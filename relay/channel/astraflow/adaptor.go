package astraflow

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/common_handler"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/security_setting"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", errors.New("astraflow adaptor: relay info is nil")
	}
	if info.ChannelBaseUrl == "" {
		return "", errors.New("astraflow adaptor: channel base url is empty")
	}
	// 非任务中继会把渠道 API key 作为 Bearer 凭证发送到上游，拒绝明文上游
	// （https 或回环地址除外），避免凭证被明文传输（CWE-319）。
	if err := common.ValidateHTTPSChannelBaseURL(info.ChannelBaseUrl, security_setting.GetSecuritySetting().RequireHTTPSChannelBaseURL); err != nil {
		return "", fmt.Errorf("astraflow adaptor: %w", err)
	}
	requestPath := info.RequestURLPath
	if requestPath == "" {
		return info.ChannelBaseUrl, nil
	}
	// /v1/messages (RelayFormatClaude): 声明原生支持 messages 就透传客户端路径，
	// 否则请求体已被 ConvertClaudeRequest 转成 OpenAI 格式,必须打到 chat 端点。
	if info.RelayFormat == types.RelayFormatClaude &&
		info.RelayMode != constant.RelayModeResponses &&
		info.RelayMode != constant.RelayModeResponsesCompact {
		if nativeMessagesRequest(info) {
			return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, requestPath, info.ChannelType), nil
		}
		return fmt.Sprintf("%s/v1/chat/completions", info.ChannelBaseUrl), nil
	}
	// responses: 声明原生支持就透传,否则请求体已转成 chat,必须打到 chat 端点。
	if responsesDowngradedToChat(info) {
		return fmt.Sprintf("%s/v1/chat/completions", info.ChannelBaseUrl), nil
	}
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, requestPath, info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	// 非任务模式（chat/responses/embeddings/image）需要携带渠道鉴权头；
	// 若渠道已配置 Authorization Header Override 则跳过默认 Bearer，避免覆盖自定义鉴权。
	hasAuthOverride := false
	if len(info.HeadersOverride) > 0 {
		for k := range info.HeadersOverride {
			if strings.EqualFold(k, "Authorization") {
				hasAuthOverride = true
				break
			}
		}
	}
	if !hasAuthOverride {
		req.Set("Authorization", "Bearer "+info.ApiKey)
	}
	// 直连上游 Anthropic 端点时必须带上 Claude 协议头(与 claude 适配器一致):
	// 沿用客户端的 anthropic-version,缺省 2023-06-01;anthropic-beta 与渠道级
	// Claude 头一并转发。转换路径上游只听 OpenAI 协议、responses 路径是 Responses
	// 报文,都不加这些头。
	if nativeMessagesRequest(info) {
		anthropicVersion := c.Request.Header.Get("anthropic-version")
		if anthropicVersion == "" {
			anthropicVersion = "2023-06-01"
		}
		req.Set("anthropic-version", anthropicVersion)
		claude.CommonClaudeHeadersOperation(c, req, info)
	}
	return nil
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
	// 声明原生支持 responses 的模型(含未配置时的现状基线)、以及改道来的会话
	// (见 responsesDowngradedToChat): 请求体原样转发。
	if !responsesDowngradedToChat(info) {
		return request, nil
	}
	// 其余模型: 降级为 chat 请求,响应侧由 OaiChatToResponses* 转回 Responses 形状。
	result, err := service.ConvertRequest(c, info, types.RelayFormatOpenAI, &request)
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

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	resp, err := channel.DoApiRequest(a, c, info, requestBody)
	if err != nil {
		return resp, err
	}
	a.detectRejectedNativeProtocol(info, resp)
	return resp, nil
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	// responses 的报文只有 Responses 处理器认得: chat 处理器会把响应体按 chat 解析,
	// 非流式拿不到 usage,流式会把 response.completed 收尾判成不完整流。
	// 声明原生 messages 的模型只在本次会话确实是 Anthropic Messages 时才交给
	// claude 处理器: 全局 ChatCompletionsToResponsesPolicy 会把 Claude 格式请求
	// 改道 Responses 协议(relay/claude_handler.go),那类响应不是 Anthropic 形状。
	switch {
	case nativeMessagesRequest(info):
		return (&claude.Adaptor{}).DoResponse(c, resp, info)
	case info.RelayMode == constant.RelayModeResponses:
		if responsesDowngradedToChat(info) {
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

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	// 声明原生支持 messages 的模型: 请求体原样转发,响应由 claude 处理器解析。
	if useNativeProtocol(info, dto.ModelProtocolMessages) {
		return request, nil
	}
	// 上游只说 OpenAI 协议: 把 Anthropic 请求体转成 OpenAI chat 请求，
	// 由 GetRequestURL 指向 /v1/chat/completions；响应侧由 openai handler
	// 按 RelayFormatClaude 转回 Anthropic 格式（含流式逐块转换）。
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
	return a.ConvertOpenAIRequest(c, info, aiRequest)
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}

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

// useNativeProtocol 报告本次请求能否直连上游的该协议。
// 必须区分"未命中声明"与"命中但不含该协议": 两者现状基线不同——messages 的现状
// 是被转成 chat(基线 false),responses 的现状是原样直连(基线 true)。未配置时
// 即按基线走,保证零回归;命中时以声明为准。
func useNativeProtocol(info *relaycommon.RelayInfo, protocol string) bool {
	if info == nil || info.ChannelMeta == nil {
		return protocol == dto.ModelProtocolResponses
	}
	protocols, declared := info.ChannelOtherSettings.ResolveModelProtocols(upstreamModelID(info))
	if !declared {
		return protocol == dto.ModelProtocolResponses
	}
	return slices.Contains(protocols, protocol) && !nativeProtocolDowngraded(info, protocol)
}

// nativeMessagesRequest 报告本次 /v1/messages 会话是否直连上游的 Anthropic 端点。
// 排除 RelayModeResponses*: 全局 ChatCompletionsToResponsesPolicy 会把 Claude 格式
// 请求改道 Responses 协议,那类会话收发都是 Responses 形状,不走 Anthropic 端点。
// 三处必须同判——GetRequestURL 选路径、SetupRequestHeader 补 Claude 协议头、
// DoResponse 选响应处理器,判法不一致会让请求与响应解析错配。
func nativeMessagesRequest(info *relaycommon.RelayInfo) bool {
	return info != nil &&
		info.RelayFormat == types.RelayFormatClaude &&
		info.RelayMode != constant.RelayModeResponses &&
		info.RelayMode != constant.RelayModeResponsesCompact &&
		useNativeProtocol(info, dto.ModelProtocolMessages)
}

// nativeProtocolDowngradeMemo 记住"已声明原生、但上游明确拒绝"的 (渠道, 模型, 协议):
// 进程内生效,命中后该组合自动降级为转换,避免同一个配置错误每次都撞上游。
// 重启即失效;不写库、不改渠道配置。值是记录时刻,见 nativeProtocolDowngraded。
var nativeProtocolDowngradeMemo sync.Map // key: channelID|model|protocol, value: time.Time

const nativeProtocolBodyPeekLimit = 4 * 1024

// protocolUnsupportedMarkers 是上游拒绝某协议时的错误文案特征(小写子串)。
// 只收能指明"协议/端点本身不支持"的措辞: 裸 "unsupported" 会把参数级 4xx
// (如 "unsupported parameter: temperature") 也算作协议拒绝, 让该组合在进程内
// 被永久降级。实测: claude 模型打 /v1/responses 返回 500 "not implemented";
// 图像模型返回 400 "The requested operation is unsupported."。
var protocolUnsupportedMarkers = []string{
	"not implemented",
	"operation is unsupported",
	"does not support",
	"unsupported protocol",
	"unsupported endpoint",
}

func nativeProtocolMemoKey(info *relaycommon.RelayInfo, protocol string) string {
	return fmt.Sprintf("%d|%s|%s", info.ChannelId, upstreamModelID(info), protocol)
}

// nativeProtocolDowngraded 报告该 (渠道, 模型, 协议) 是否已被上游拒绝过。
// 记录带水位线: 只对"记录之后才开始"的请求生效。否则已经在途的请求会被中途翻转
// ——它按原生协议发出,响应却会被降级后的处理器按 chat 解析,把成功的原生报文判成
// 错误。StartTime 在 RelayInfo 构建时写入, 早于任何适配器调用,故以它为界:
// 记录时刻早于 StartTime 才降级, 即记录之前发出的请求保持它发出时的决定。
// StartTime 为零(手工构造的 RelayInfo)时按"命中即降级"处理。
func nativeProtocolDowngraded(info *relaycommon.RelayInfo, protocol string) bool {
	recordedAt, downgraded := nativeProtocolDowngradeMemo.Load(nativeProtocolMemoKey(info, protocol))
	if !downgraded {
		return false
	}
	if info.StartTime.IsZero() {
		return true
	}
	recorded, ok := recordedAt.(time.Time)
	if !ok {
		return true
	}
	return recorded.Before(info.StartTime)
}

func upstreamRejectsProtocol(message string) bool {
	lower := strings.ToLower(message)
	for _, marker := range protocolUnsupportedMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// sentProtocol 返回本次请求实际发往上游的协议。
func sentProtocol(info *relaycommon.RelayInfo) string {
	if info.RelayMode == constant.RelayModeResponses {
		return dto.ModelProtocolResponses
	}
	if info.RelayFormat == types.RelayFormatClaude {
		return dto.ModelProtocolMessages
	}
	return dto.ModelProtocolChat
}

// detectRejectedNativeProtocol 在上游非 2xx 且错误体命中"协议不支持"签名时,把
// (渠道, 模型, 协议) 记入降级记忆。读取的响应体用 MultiReader 还原,不影响后续
// service.RelayErrorHandler 解析。
func (a *Adaptor) detectRejectedNativeProtocol(info *relaycommon.RelayInfo, resp *http.Response) {
	if info == nil || info.ChannelMeta == nil || resp == nil || resp.Body == nil || resp.StatusCode < 400 {
		return
	}
	// 渠道测试用的是合成请求(header 透传同样对测试请求短路,
	// relay/channel/api_request.go),它命中的拒绝不构成"上游不支持该协议"的证据,
	// 不得因此永久降级生产流量的 (渠道, 模型, 协议)。
	if info.IsChannelTest {
		return
	}
	protocol := sentProtocol(info)
	if protocol == dto.ModelProtocolChat || !useNativeProtocol(info, protocol) {
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
		"astraflow: upstream rejected native %s (channel_id=%d model=%s status=%d), downgrading subsequent requests to chat: %s",
		protocol, info.ChannelId, upstreamModelID(info), resp.StatusCode, common.LocalLogPreview(string(peeked)),
	))
}

// responsesDowngradedToChat 报告本次会话的 Responses 请求是否需要降级为 chat 发出
// (请求体转 chat、打 chat 端点、响应由 OaiChatToResponses* 转回)。
// 只有客户端确实以 Responses 协议发起的会话才降级: 全局
// ChatCompletionsToResponsesPolicy 会把 chat/messages 客户端改道到 Responses 协议
// (relay/chat_completions_via_responses.go),那类会话的 RelayMode 同样是 Responses,
// 但上游报文本来就是 Responses,响应侧由 host 的 OaiResponsesToChat* 固定按
// Responses 解析且不经过本适配器的 DoResponse——改了请求形状就会与响应解析错配。
// 请求体透传时同样不降级: host 不会调用 ConvertOpenAIResponsesRequest
// (relay/responses_handler.go),上游收到的是客户端原始的 Responses 报文,
// 只能打 Responses 端点。
func responsesDowngradedToChat(info *relaycommon.RelayInfo) bool {
	return info != nil &&
		info.RelayFormat == types.RelayFormatOpenAIResponses &&
		info.RelayMode == constant.RelayModeResponses &&
		// 先于下面两个透传判断: ChannelMeta 为 nil 时 info.ChannelSetting 会 panic,
		// 而 useNativeProtocol 容忍 nil(该情形下 responses 取现状基线 true,短路返回 false)。
		!useNativeProtocol(info, dto.ModelProtocolResponses) &&
		!info.ChannelSetting.PassThroughBodyEnabled &&
		!model_setting.GetGlobalSettings().PassThroughRequestEnabled
}

// Ensure compile-time interface check.
var _ channel.Adaptor = (*Adaptor)(nil)
