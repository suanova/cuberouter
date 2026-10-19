package astraflow

import (
	"errors"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	"github.com/QuantumNous/new-api/relay/channel/protocolroute"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

// policy 是本渠道的多协议路由决策。基线只有 responses: 未声明 model_protocols
// 的模型保持"messages 转 chat、responses 原样直连"的既有行为; 声明过的模型以
// 声明为准。路由、Header、转换与响应分派都在 protocolroute 里,与 CubeStack 共用。
var policy = protocolroute.Policy{
	ChannelLabel:         "astraflow adaptor",
	LogLabel:             "astraflow",
	NativeWhenUndeclared: []string{dto.ModelProtocolResponses},
	// DowngradeUndeclared 保持默认的 false: 这里的基线是历史承诺——未声明的模型
	// 一律按 responses 直连,上游拒绝也不改变路由。CubeStack 的基线则吃降级记忆。
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
	// JSON 请求(图像生成、以及以 base64 内联参考图的编辑)原样透传 DTO;multipart
	// /v1/images/edits 必须交给 openai 适配器重建请求体: host 只把表单值解析进 DTO,
	// 文件部分(image/mask)只有重建才会带上, Content-Type 也要换成新 boundary——
	// 否则上游收到的是 JSON 报文配客户端的旧 boundary, 文件全丢且请求无法解析。
	return (&openai.Adaptor{}).ConvertImageRequest(c, info, request)
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return policy.ConvertOpenAIResponsesRequest(c, info, request)
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if info.RelayMode == constant.RelayModeImagesEdits && !openai.IsJSONRequest(c) {
		// multipart 图片编辑: 请求体已由 ConvertImageRequest 重建, c.Request.Header
		// 上是新 boundary, 走表单发送路径把该 Content-Type 带给上游。谓词与转换路径
		// 同一个(openai.IsJSONRequest), 两条路不会选中不同形状。
		resp, err := channel.DoFormRequest(a, c, info, requestBody)
		if err != nil {
			return resp, err
		}
		// 单协议会话不会命中, 但表单路径同样要经过降级探测, 保持与 DoApiRequest 一致。
		policy.DetectRejectedNativeProtocol(info, resp)
		return resp, nil
	}
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
