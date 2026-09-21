package astraflow

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/security_setting"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// relayChainCase 定义一次完整非任务中继链路断言：假上游记录收到的请求，
// 断言方法与路径、Bearer 鉴权、请求体逐字节一致，随后用适配器 DoResponse
// 解析假上游返回体并确认 usage 与回写内容。
type relayChainCase struct {
	name                 string
	path                 string
	relayMode            int
	relayFormat          types.RelayFormat
	model                string
	protocols            map[string][]string // 渠道 model_protocols 声明
	requestBody          string
	upstreamBody         string
	isStream             bool
	wantUpstreamPath     string // 上游应命中的路径;空表示等于 path
	skipRequestEquality  bool   // 转换过的请求体不做逐字节断言
	wantPromptTokens     int    // -1 表示不断言
	wantCompletionTokens int    // -1 表示不断言
	wantBodyContains     string
}

// TestRelayChainNonTaskModes 锁定多模态非任务链路契约：chat/responses/
// embeddings/rerank/image 五种模式共用同一 OpenAI 直传链路——上游路径沿用客户端
// 路径、携带 Bearer 鉴权、请求体原样转发；响应按各自协议解析：chat/embeddings/
// image 走 chat 处理器，rerank 走 Rerank 处理器并取其原生 usage，responses
// 走 Responses 处理器并取其原生 usage。
func TestRelayChainNonTaskModes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []relayChainCase{
		{
			name:             "chat completions",
			path:             "/v1/chat/completions",
			relayMode:        relayconstant.RelayModeChatCompletions,
			relayFormat:      types.RelayFormatOpenAI,
			model:            "deepseek-v3",
			requestBody:      `{"model":"deepseek-v3","messages":[{"role":"user","content":"hi"}]}`,
			upstreamBody:     `{"id":"chatcmpl-1","object":"chat.completion","created":1700000000,"model":"deepseek-v3","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":8,"total_tokens":20}}`,
			wantPromptTokens: 12, wantCompletionTokens: 8,
			wantBodyContains: `"chatcmpl-1"`,
		},
		{
			name:             "responses",
			path:             "/v1/responses",
			relayMode:        relayconstant.RelayModeResponses,
			relayFormat:      types.RelayFormatOpenAIResponses,
			model:            "deepseek-v3",
			requestBody:      `{"model":"deepseek-v3","input":"hi"}`,
			upstreamBody:     `{"id":"resp_1","object":"response","created_at":1700000000,"model":"deepseek-v3","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"pong","annotations":[]}]}],"usage":{"input_tokens":5,"output_tokens":3,"total_tokens":8}}`,
			wantPromptTokens: 5, wantCompletionTokens: 3,
			wantBodyContains: `"resp_1"`,
		},
		{
			name:             "responses native passthrough",
			path:             "/v1/responses",
			wantUpstreamPath: "/v1/responses",
			relayMode:        relayconstant.RelayModeResponses,
			relayFormat:      types.RelayFormatOpenAIResponses,
			model:            "gpt-5.5",
			protocols:        map[string][]string{"gpt-*": {dto.ModelProtocolChat, dto.ModelProtocolResponses}},
			requestBody:      `{"model":"gpt-5.5","input":"hi"}`,
			upstreamBody:     `{"id":"resp_1","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"pong","annotations":[]}]}],"usage":{"input_tokens":5,"output_tokens":3,"total_tokens":8}}`,
			wantPromptTokens: 5, wantCompletionTokens: 3,
			wantBodyContains: `"resp_1"`,
		},
		{
			name:                "responses downgraded when the model does not declare them",
			path:                "/v1/responses",
			wantUpstreamPath:    "/v1/chat/completions",
			relayMode:           relayconstant.RelayModeResponses,
			relayFormat:         types.RelayFormatOpenAIResponses,
			model:               "claude-sonnet-5",
			protocols:           map[string][]string{"claude-*": {dto.ModelProtocolChat, dto.ModelProtocolMessages}},
			requestBody:         `{"model":"claude-sonnet-5","input":"hi"}`,
			upstreamBody:        `{"id":"chatcmpl-1","object":"chat.completion","created":1700000000,"model":"claude-sonnet-5","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`,
			skipRequestEquality: true,
			wantPromptTokens:    5, wantCompletionTokens: 3,
			wantBodyContains: `"object":"response"`,
		},
		{
			name:             "embeddings",
			path:             "/v1/embeddings",
			relayMode:        relayconstant.RelayModeEmbeddings,
			relayFormat:      types.RelayFormatEmbedding,
			model:            "text-embedding-3-large",
			requestBody:      `{"model":"text-embedding-3-large","input":"hi"}`,
			upstreamBody:     `{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"model":"text-embedding-3-large","usage":{"prompt_tokens":8,"total_tokens":8}}`,
			wantPromptTokens: 8, wantCompletionTokens: 0,
			wantBodyContains: `"embedding"`,
		},
		{
			name:             "rerank",
			path:             "/v1/rerank",
			relayMode:        relayconstant.RelayModeRerank,
			relayFormat:      types.RelayFormatRerank,
			model:            "qwen3-reranker-8b",
			requestBody:      `{"model":"qwen3-reranker-8b","query":"hi","documents":["a","b"]}`,
			upstreamBody:     `{"results":[{"index":0,"document":{"text":"a"},"relevance_score":0.97},{"index":1,"document":{"text":"b"},"relevance_score":0.19}],"usage":{"total_tokens":42}}`,
			wantPromptTokens: 42, wantCompletionTokens: -1,
			wantBodyContains: `"relevance_score"`,
		},
		{
			name:             "image generation",
			path:             "/v1/images/generations",
			relayMode:        relayconstant.RelayModeImagesGenerations,
			relayFormat:      types.RelayFormatOpenAIImage,
			model:            "gpt-image-1",
			requestBody:      `{"model":"gpt-image-1","prompt":"a cat","n":1,"size":"1024x1024"}`,
			upstreamBody:     `{"created":1700000000,"data":[{"url":"https://cdn.example.com/cat.png"}]}`,
			wantPromptTokens: -1, wantCompletionTokens: -1,
			wantBodyContains: `https://cdn.example.com/cat.png`,
		},
		{
			name:             "anthropic messages native passthrough",
			path:             "/v1/messages",
			wantUpstreamPath: "/v1/messages",
			relayMode:        relayconstant.RelayModeChatCompletions,
			relayFormat:      types.RelayFormatClaude,
			model:            "glm-5.3",
			protocols:        map[string][]string{"glm-*": {dto.ModelProtocolChat, dto.ModelProtocolMessages}},
			requestBody:      `{"model":"glm-5.3","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`,
			upstreamBody:     `{"id":"msg_1","type":"message","role":"assistant","model":"glm-5.3","content":[{"type":"text","text":"pong"}],"stop_reason":"end_turn","usage":{"input_tokens":3,"output_tokens":2}}`,
			wantPromptTokens: 3, wantCompletionTokens: 2,
			wantBodyContains: `"pong"`,
		},
		{
			name:                "anthropic messages downgraded when model does not declare them",
			path:                "/v1/messages",
			wantUpstreamPath:    "/v1/chat/completions",
			relayMode:           relayconstant.RelayModeChatCompletions,
			relayFormat:         types.RelayFormatClaude,
			model:               "deepseek-v3",
			protocols:           map[string][]string{"claude-*": {dto.ModelProtocolChat, dto.ModelProtocolMessages}},
			requestBody:         `{"model":"deepseek-v3","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`,
			upstreamBody:        `{"id":"chatcmpl-1","object":"chat.completion","created":1700000000,"model":"deepseek-v3","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`,
			skipRequestEquality: true,
			wantPromptTokens:    3, wantCompletionTokens: 2,
			wantBodyContains: `"type":"message"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runRelayChain(t, tt)
		})
	}
}

// TestGetRequestURLRejectsCleartextUpstream 锁定发送前防线：安全开关开启时，
// 非回环的明文上游在 GetRequestURL 阶段即被拒绝，Bearer 凭证不会发出（CWE-319）。
// 既有全链路用例使用 httptest 回环地址，不受影响。
func TestGetRequestURLRejectsCleartextUpstream(t *testing.T) {
	old := security_setting.GetSecuritySetting().RequireHTTPSChannelBaseURL
	security_setting.GetSecuritySetting().RequireHTTPSChannelBaseURL = true
	defer func() { security_setting.GetSecuritySetting().RequireHTTPSChannelBaseURL = old }()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "http://upstream.example.com",
			ChannelType:       constant.ChannelTypeAstraFlow,
			ApiKey:            "sk-test",
			UpstreamModelName: "deepseek-v3",
		},
		RequestURLPath: "/v1/chat/completions",
	}

	_, err := adaptor.GetRequestURL(info)
	require.ErrorContains(t, err, "must use HTTPS")
}

// TestGetRequestURLKeepsResponsesPathWhenPassThroughEnabled 锁定透传与会话降级的边界:
// 请求体透传时(全局 PassThroughRequestEnabled 或渠道 PassThroughBodyEnabled)host
// 不会调用 ConvertOpenAIResponsesRequest(relay/responses_handler.go),发往上游的是
// 客户端原始的 Responses 报文,因此即使模型未声明原生 responses 也必须打
// {base}/v1/responses;关闭透传时同一模型才降级到 chat 端点。
func TestGetRequestURLKeepsResponsesPathWhenPassThroughEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldGlobalPassThrough := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	defer func() { model_setting.GetGlobalSettings().PassThroughRequestEnabled = oldGlobalPassThrough }()

	newInfo := func() *relaycommon.RelayInfo {
		return &relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelBaseUrl:    "https://api.modelverse.cn",
				ChannelType:       constant.ChannelTypeAstraFlow,
				UpstreamModelName: "claude-sonnet-5",
				ChannelOtherSettings: dto.ChannelOtherSettings{
					ModelProtocols: map[string][]string{"claude-*": {dto.ModelProtocolChat, dto.ModelProtocolMessages}},
				},
			},
			RequestURLPath:  "/v1/responses",
			RelayFormat:     types.RelayFormatOpenAIResponses,
			RelayMode:       relayconstant.RelayModeResponses,
			OriginModelName: "claude-sonnet-5",
		}
	}

	adaptor := &Adaptor{}

	// 反例: 关闭透传时该模型确实降级到 chat 端点。
	url, err := adaptor.GetRequestURL(newInfo())
	require.NoError(t, err)
	assert.Equal(t, "https://api.modelverse.cn/v1/chat/completions", url)

	channelInfo := newInfo()
	channelInfo.ChannelSetting.PassThroughBodyEnabled = true
	url, err = adaptor.GetRequestURL(channelInfo)
	require.NoError(t, err)
	assert.Equal(t, "https://api.modelverse.cn/v1/responses", url, "a pass-through body was never converted")

	model_setting.GetGlobalSettings().PassThroughRequestEnabled = true
	url, err = adaptor.GetRequestURL(newInfo())
	require.NoError(t, err)
	assert.Equal(t, "https://api.modelverse.cn/v1/responses", url, "a pass-through body was never converted")
}

// TestConvertClaudeRequestKeepsNativeBodyWhenDeclared 锁定: 声明原生 messages 的
// 模型,请求体指针原样返回(不做 Anthropic→chat 转换);未声明时返回转换后的
// OpenAI chat 请求。
func TestConvertClaudeRequestKeepsNativeBodyWhenDeclared(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	newInfo := func(protocols map[string][]string) *relaycommon.RelayInfo {
		return &relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelType:          constant.ChannelTypeAstraFlow,
				UpstreamModelName:    "glm-5.3",
				ChannelOtherSettings: dto.ChannelOtherSettings{ModelProtocols: protocols},
			},
			RelayFormat:     types.RelayFormatClaude,
			RelayMode:       relayconstant.RelayModeChatCompletions,
			OriginModelName: "glm-5.3",
		}
	}

	adaptor := &Adaptor{}
	request := &dto.ClaudeRequest{
		Model:     "glm-5.3",
		MaxTokens: lo.ToPtr(uint(16)),
		Messages:  []dto.ClaudeMessage{{Role: "user", Content: "hi"}},
	}

	converted, err := adaptor.ConvertClaudeRequest(context, newInfo(map[string][]string{"glm-*": {dto.ModelProtocolChat, dto.ModelProtocolMessages}}), request)
	require.NoError(t, err)
	assert.Same(t, request, converted, "declared-native messages must be forwarded untouched")

	converted, err = adaptor.ConvertClaudeRequest(context, newInfo(map[string][]string{"claude-*": {dto.ModelProtocolChat, dto.ModelProtocolMessages}}), request)
	require.NoError(t, err)
	_, isClaudeRequest := converted.(*dto.ClaudeRequest)
	assert.False(t, isClaudeRequest, "unmatched model must still be converted to a chat request")
}

// TestConvertOpenAIResponsesRequestDowngradesToChat 锁定降级契约：客户端确实以
// Responses 协议发起、且模型未声明原生 responses 时，请求体必须转成 OpenAI chat
// 请求（链路用例的 DoRequest 直接收原始请求体，转换与否只能在这里锁住）；声明了
// responses 或未配置时原样透传。改道会话（RelayFormat 非 Responses，见
// ChatCompletionsToResponsesPolicy）的上游报文本来就是 Responses，响应由 host 的
// OaiResponsesToChat* 固定解析，同样不得改动请求形状。
func TestConvertOpenAIResponsesRequestDowngradesToChat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	newInfo := func(protocols map[string][]string, relayFormat types.RelayFormat) *relaycommon.RelayInfo {
		return &relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelType:          constant.ChannelTypeAstraFlow,
				UpstreamModelName:    "claude-sonnet-5",
				ChannelOtherSettings: dto.ChannelOtherSettings{ModelProtocols: protocols},
			},
			RelayFormat:     relayFormat,
			RelayMode:       relayconstant.RelayModeResponses,
			OriginModelName: "claude-sonnet-5",
		}
	}

	tests := []struct {
		name        string
		protocols   map[string][]string
		relayFormat types.RelayFormat
		wantChat    bool
	}{
		{
			name:        "declared responses stays native",
			protocols:   map[string][]string{"claude-*": {dto.ModelProtocolChat, dto.ModelProtocolResponses}},
			relayFormat: types.RelayFormatOpenAIResponses,
		},
		{
			name:        "declared without responses downgrades to chat",
			protocols:   map[string][]string{"claude-*": {dto.ModelProtocolChat, dto.ModelProtocolMessages}},
			relayFormat: types.RelayFormatOpenAIResponses,
			wantChat:    true,
		},
		{
			name:        "no declaration keeps the responses baseline",
			relayFormat: types.RelayFormatOpenAIResponses,
		},
		{
			name:        "policy reroute stays native",
			protocols:   map[string][]string{"claude-*": {dto.ModelProtocolChat, dto.ModelProtocolMessages}},
			relayFormat: types.RelayFormatClaude,
		},
	}

	adaptor := &Adaptor{}
	request := dto.OpenAIResponsesRequest{Model: "claude-sonnet-5", Input: []byte(`"hi"`)}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converted, err := adaptor.ConvertOpenAIResponsesRequest(context, newInfo(tt.protocols, tt.relayFormat), request)
			require.NoError(t, err)
			if !tt.wantChat {
				assert.Equal(t, request, converted, "responses request must be forwarded untouched")
				return
			}
			chatRequest, ok := converted.(*dto.GeneralOpenAIRequest)
			require.True(t, ok, "expected a chat request, got %T", converted)
			assert.Equal(t, "claude-sonnet-5", chatRequest.Model)
			require.Len(t, chatRequest.Messages, 1)
			assert.Equal(t, "hi", chatRequest.Messages[0].Content)
		})
	}
}

// runRelayChain 执行一次完整链路：DoRequest 打向假上游并断言请求契约，
// DoResponse 解析响应并断言 usage 与回写内容。
func runRelayChain(t *testing.T, tt relayChainCase) {
	t.Helper()

	var gotMethod, gotPath, gotAuth string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(tt.upstreamBody))
	}))
	defer server.Close()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBufferString(tt.requestBody))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Header.Set("Accept", "application/json")

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:       server.URL,
			ChannelType:          constant.ChannelTypeAstraFlow,
			ApiKey:               "sk-test",
			UpstreamModelName:    tt.model,
			ChannelOtherSettings: dto.ChannelOtherSettings{ModelProtocols: tt.protocols},
		},
		RequestURLPath:  tt.path,
		RelayFormat:     tt.relayFormat,
		RelayMode:       tt.relayMode,
		IsStream:        tt.isStream,
		OriginModelName: tt.model,
	}

	respAny, err := adaptor.DoRequest(context, info, bytes.NewBufferString(tt.requestBody))
	require.NoError(t, err)
	resp, ok := respAny.(*http.Response)
	require.True(t, ok, "expected *http.Response, got %T", respAny)
	defer func() { _ = resp.Body.Close() }()

	// 上游请求契约：方法、路径、Bearer 鉴权、请求体逐字节一致。
	wantPath := tt.wantUpstreamPath
	if wantPath == "" {
		wantPath = tt.path
	}
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, wantPath, gotPath)
	assert.Equal(t, "Bearer sk-test", gotAuth)
	if !tt.skipRequestEquality {
		assert.Equal(t, tt.requestBody, string(gotBody))
	}

	usageAny, apiErr := adaptor.DoResponse(context, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usageAny)
	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok, "expected *dto.Usage, got %T", usageAny)

	if tt.wantPromptTokens >= 0 {
		assert.Equal(t, tt.wantPromptTokens, usage.PromptTokens)
	}
	if tt.wantCompletionTokens >= 0 {
		assert.Equal(t, tt.wantCompletionTokens, usage.CompletionTokens)
	}
	if tt.wantBodyContains != "" {
		assert.Contains(t, recorder.Body.String(), tt.wantBodyContains)
	}
}

// TestRelayChainStreamingChatCompletion 锁定流式链路契约：SSE 流经
// OaiStreamHandler 转发，内容拼接到客户端，usage 由响应文本估算得出。
func TestRelayChainStreamingChatCompletion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// StreamScannerHandler 需要正数 StreamingTimeout，测试环境由 main 的
	// InitEnv 才会设置，这里按既有流式测试的模式保存/恢复。
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 300
	defer func() { constant.StreamingTimeout = oldStreamingTimeout }()

	const requestBody = `{"model":"deepseek-v3","messages":[{"role":"user","content":"hi"}],"stream":true}`
	const upstreamBody = "" +
		`data: {"id":"chatcmpl-s1","object":"chat.completion.chunk","created":1700000000,"model":"deepseek-v3","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}` + "\n\n" +
		`data: {"id":"chatcmpl-s1","object":"chat.completion.chunk","created":1700000000,"model":"deepseek-v3","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n" +
		`data: [DONE]` + "\n"

	var gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(upstreamBody))
	}))
	defer server.Close()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(requestBody))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Header.Set("Accept", "text/event-stream")

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    server.URL,
			ChannelType:       constant.ChannelTypeAstraFlow,
			ApiKey:            "sk-test",
			UpstreamModelName: "deepseek-v3",
		},
		RequestURLPath:  "/v1/chat/completions",
		RelayFormat:     types.RelayFormatOpenAI,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		IsStream:        true,
		OriginModelName: "deepseek-v3",
	}

	respAny, err := adaptor.DoRequest(context, info, bytes.NewBufferString(requestBody))
	require.NoError(t, err)
	resp, ok := respAny.(*http.Response)
	require.True(t, ok, "expected *http.Response, got %T", respAny)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, "/v1/chat/completions", gotPath)
	assert.Equal(t, "Bearer sk-test", gotAuth)

	usageAny, apiErr := adaptor.DoResponse(context, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usageAny)
	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok, "expected *dto.Usage, got %T", usageAny)
	assert.Greater(t, usage.CompletionTokens, 0, "stream usage must be derived from relayed text")
	assert.Contains(t, recorder.Body.String(), "Hello")
}

// TestRelayChainStreamingResponsesDowngradedToChat 锁定流式降级契约: 模型只声明
// chat(未声明 responses)时, 客户端 stream:true 的 /v1/responses 必须打成
// {base}/v1/chat/completions, 上游的 chat SSE 在网关内转成 Responses 事件流——
// 客户端收到 response.output_text.delta, 不是 chat 分块, 也不得被判为不完整流。
func TestRelayChainStreamingResponsesDowngradedToChat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 300
	defer func() { constant.StreamingTimeout = oldStreamingTimeout }()

	const requestBody = `{"model":"claude-sonnet-5","input":"hi","stream":true}`
	const upstreamBody = "" +
		`data: {"id":"chatcmpl-s1","object":"chat.completion.chunk","created":1700000000,"model":"claude-sonnet-5","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}` + "\n\n" +
		`data: {"id":"chatcmpl-s1","object":"chat.completion.chunk","created":1700000000,"model":"claude-sonnet-5","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}` + "\n\n" +
		`data: [DONE]` + "\n"

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(upstreamBody))
	}))
	defer server.Close()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(requestBody))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Header.Set("Accept", "text/event-stream")

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    server.URL,
			ChannelType:       constant.ChannelTypeAstraFlow,
			ApiKey:            "sk-test",
			UpstreamModelName: "claude-sonnet-5",
			ChannelOtherSettings: dto.ChannelOtherSettings{
				ModelProtocols: map[string][]string{"claude-*": {dto.ModelProtocolChat, dto.ModelProtocolMessages}},
			},
		},
		RequestURLPath:  "/v1/responses",
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RelayMode:       relayconstant.RelayModeResponses,
		IsStream:        true,
		OriginModelName: "claude-sonnet-5",
	}

	respAny, err := adaptor.DoRequest(context, info, bytes.NewBufferString(requestBody))
	require.NoError(t, err)
	resp, ok := respAny.(*http.Response)
	require.True(t, ok, "expected *http.Response, got %T", respAny)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, "/v1/chat/completions", gotPath, "a model without a native responses declaration must be served over chat")

	usageAny, apiErr := adaptor.DoResponse(context, resp, info)
	require.Nil(t, apiErr, "a converted chat stream must not be reported as an incomplete responses stream")
	require.NotNil(t, usageAny)
	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok, "expected *dto.Usage, got %T", usageAny)
	assert.Equal(t, 4, usage.PromptTokens)
	assert.Equal(t, 2, usage.CompletionTokens)

	clientBody := recorder.Body.String()
	assert.Contains(t, clientBody, `"type":"response.output_text.delta"`, "client must receive the responses event stream")
	assert.Contains(t, clientBody, "Hello")
	assert.NotContains(t, clientBody, "content_block_delta")
	assert.NotContains(t, clientBody, "chat.completion.chunk", "chat chunks would mean the upstream shape leaked to the client")
}

// TestRelayChainStreamingNativeMessages 锁定原生 Anthropic 流式契约: 声明 messages
// 的模型直接收发 Anthropic SSE, usage 取自报文的 input_tokens/output_tokens,
// 不按文本估算。
func TestRelayChainStreamingNativeMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 300
	defer func() { constant.StreamingTimeout = oldStreamingTimeout }()

	const requestBody = `{"model":"glm-5.3","max_tokens":16,"messages":[{"role":"user","content":"hi"}],"stream":true}`
	const upstreamBody = "" +
		"event: message_start\n" +
		`data: {"type":"message_start","message":{"id":"msg_s1","type":"message","role":"assistant","model":"glm-5.3","content":[],"usage":{"input_tokens":7,"output_tokens":1}}}` + "\n\n" +
		"event: content_block_start\n" +
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n" +
		"event: content_block_delta\n" +
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}` + "\n\n" +
		"event: content_block_stop\n" +
		`data: {"type":"content_block_stop","index":0}` + "\n\n" +
		"event: message_delta\n" +
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}` + "\n\n" +
		"event: message_stop\n" +
		`data: {"type":"message_stop"}` + "\n\n"

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(upstreamBody))
	}))
	defer server.Close()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(requestBody))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Header.Set("Accept", "text/event-stream")

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    server.URL,
			ChannelType:       constant.ChannelTypeAstraFlow,
			ApiKey:            "sk-test",
			UpstreamModelName: "glm-5.3",
			ChannelOtherSettings: dto.ChannelOtherSettings{
				ModelProtocols: map[string][]string{"glm-*": {dto.ModelProtocolChat, dto.ModelProtocolMessages}},
			},
		},
		RequestURLPath:  "/v1/messages",
		RelayFormat:     types.RelayFormatClaude,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		IsStream:        true,
		OriginModelName: "glm-5.3",
	}

	respAny, err := adaptor.DoRequest(context, info, bytes.NewBufferString(requestBody))
	require.NoError(t, err)
	resp, ok := respAny.(*http.Response)
	require.True(t, ok, "expected *http.Response, got %T", respAny)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, "/v1/messages", gotPath)

	usageAny, apiErr := adaptor.DoResponse(context, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usageAny)
	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok, "expected *dto.Usage, got %T", usageAny)
	assert.Equal(t, 7, usage.PromptTokens, "prompt tokens must come from message_start, not the estimator")
	assert.Equal(t, 5, usage.CompletionTokens, "completion tokens must come from message_delta, not the estimator")

	clientBody := recorder.Body.String()
	assert.Contains(t, clientBody, "content_block_delta", "native anthropic events must pass through untouched")
	assert.Contains(t, clientBody, "Hello")
}

// TestRelayChainClaudeFormatResponsesModeKeepsResponsesHandler 锁定改道契约:
// 全局 ChatCompletionsToResponsesPolicy 会把 Claude 格式请求改道 Responses
// 协议(relay/claude_handler.go),此时 RelayFormat 仍是 Claude, RelayMode 已是
// Responses,上游收发都是 Responses 形状;即使模型声明了原生 messages,响应也
// 必须由 Responses 处理器解析——claude 处理器会把它当 Anthropic 解析,客户端将
// 收到 Anthropic 事件而非 Responses 事件。
func TestRelayChainClaudeFormatResponsesModeKeepsResponsesHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 300
	defer func() { constant.StreamingTimeout = oldStreamingTimeout }()

	const requestBody = `{"model":"glm-5.3","input":"hi","stream":true}`
	const upstreamBody = "" +
		`data: {"type":"response.output_text.delta","delta":"Hello"}` + "\n\n" +
		`data: {"type":"response.completed","response":{"id":"resp_s1","object":"response","status":"completed","model":"glm-5.3","usage":{"input_tokens":5,"output_tokens":3,"total_tokens":8}}}` + "\n\n"

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(upstreamBody))
	}))
	defer server.Close()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(requestBody))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Header.Set("Accept", "text/event-stream")

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    server.URL,
			ChannelType:       constant.ChannelTypeAstraFlow,
			ApiKey:            "sk-test",
			UpstreamModelName: "glm-5.3",
			ChannelOtherSettings: dto.ChannelOtherSettings{
				ModelProtocols: map[string][]string{"glm-*": {dto.ModelProtocolChat, dto.ModelProtocolMessages}},
			},
		},
		RequestURLPath:  "/v1/responses",
		RelayFormat:     types.RelayFormatClaude,
		RelayMode:       relayconstant.RelayModeResponses,
		IsStream:        true,
		OriginModelName: "glm-5.3",
	}

	respAny, err := adaptor.DoRequest(context, info, bytes.NewBufferString(requestBody))
	require.NoError(t, err)
	resp, ok := respAny.(*http.Response)
	require.True(t, ok, "expected *http.Response, got %T", respAny)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, "/v1/responses", gotPath)

	usageAny, apiErr := adaptor.DoResponse(context, resp, info)
	require.Nil(t, apiErr, "a responses reply must not be reported as an incomplete anthropic stream")
	require.NotNil(t, usageAny)
	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok, "expected *dto.Usage, got %T", usageAny)
	assert.Equal(t, 5, usage.PromptTokens)
	assert.Equal(t, 3, usage.CompletionTokens)

	clientBody := recorder.Body.String()
	assert.Contains(t, clientBody, `"type":"response.output_text.delta"`, "client must receive the responses event stream")
	assert.NotContains(t, clientBody, "content_block_delta", "claude events would mean the anthropic handler parsed a responses payload")
}

// TestRelayChainStreamingResponses 锁定流式 responses 契约: 原生 Responses SSE
// 以 response.completed 收尾(没有 chat 的 finish_reason,也没有 [DONE]),必须按
// Responses 协议解析,不得判为不完整流。
func TestRelayChainStreamingResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 300
	defer func() { constant.StreamingTimeout = oldStreamingTimeout }()

	const requestBody = `{"model":"deepseek-v3","input":"hi","stream":true}`
	const upstreamBody = "" +
		`data: {"type":"response.output_text.delta","delta":"Hello"}` + "\n\n" +
		`data: {"type":"response.completed","response":{"id":"resp_s1","object":"response","status":"completed","model":"deepseek-v3","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}` + "\n\n"

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(upstreamBody))
	}))
	defer server.Close()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(requestBody))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Header.Set("Accept", "text/event-stream")

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    server.URL,
			ChannelType:       constant.ChannelTypeAstraFlow,
			ApiKey:            "sk-test",
			UpstreamModelName: "deepseek-v3",
		},
		RequestURLPath:  "/v1/responses",
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RelayMode:       relayconstant.RelayModeResponses,
		IsStream:        true,
		OriginModelName: "deepseek-v3",
	}

	respAny, err := adaptor.DoRequest(context, info, bytes.NewBufferString(requestBody))
	require.NoError(t, err)
	resp, ok := respAny.(*http.Response)
	require.True(t, ok, "expected *http.Response, got %T", respAny)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, "/v1/responses", gotPath)

	usageAny, apiErr := adaptor.DoResponse(context, resp, info)
	require.Nil(t, apiErr, "native responses stream must not be reported as incomplete")
	require.NotNil(t, usageAny)
	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok, "expected *dto.Usage, got %T", usageAny)
	assert.Equal(t, 4, usage.PromptTokens)
	assert.Equal(t, 2, usage.CompletionTokens)
	assert.Contains(t, recorder.Body.String(), "Hello")
}
