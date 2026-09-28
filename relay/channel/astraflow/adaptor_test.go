package astraflow

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetNativeProtocolDowngradeMemoForTest() {
	nativeProtocolDowngradeMemo.Range(func(key, _ any) bool {
		nativeProtocolDowngradeMemo.Delete(key)
		return true
	})
}

// newNativeMessagesInfo 构造一次声明原生支持 messages 的 glm-5.3 会话。
func newNativeMessagesInfo(baseURL string, channelID int) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         channelID,
			ChannelBaseUrl:    baseURL,
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
		OriginModelName: "glm-5.3",
	}
}

// newMessagesContext 构造一次 POST /v1/messages 的 gin 上下文。
func newMessagesContext() *gin.Context {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(`{"model":"glm-5.3"}`))
	return context
}

// newRejectingUpstream 起一个固定返回 status/body 的假上游。
func newRejectingUpstream(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

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

// TestConvertRerankRequestPassthrough 锁定 /v1/rerank 直传契约：
// Rerank 请求必须原样转发给上游，而不是返回 not implemented。
func TestConvertRerankRequestPassthrough(t *testing.T) {
	adaptor := &Adaptor{}
	request := dto.RerankRequest{Model: "qwen3-reranker-8b", Query: "hi", Documents: []any{"a", "b"}}

	out, err := adaptor.ConvertRerankRequest(nil, relayconstant.RelayModeRerank, request)
	require.NoError(t, err)
	got, ok := out.(dto.RerankRequest)
	require.True(t, ok, "expected dto.RerankRequest, got %T", out)
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

// TestSetupRequestHeaderSendsClaudeHeadersOnNativeMessages 锁定原生 Anthropic
// 直连的请求头契约: 声明原生 messages 且未被改道 Responses 时, 发往上游的请求
// 带 anthropic-version(客户端值优先, 缺省 2023-06-01)与客户端 anthropic-beta;
// 请求体被转成 chat 的模型(未声明)与改道 responses 的会话都不带这些头——上游
// 在那两条路径上根本不按 Anthropic 协议解析。
func TestSetupRequestHeaderSendsClaudeHeadersOnNativeMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		protocols     map[string][]string
		model         string
		relayMode     int
		clientVersion string
		clientBeta    string
		wantVersion   string
		wantBeta      string
	}{
		{
			name:          "native messages forwards the client headers",
			protocols:     map[string][]string{"glm-*": {dto.ModelProtocolChat, dto.ModelProtocolMessages}},
			model:         "glm-5.3",
			relayMode:     relayconstant.RelayModeChatCompletions,
			clientVersion: "2024-06-01",
			clientBeta:    "prompt-caching-2024-07-31",
			wantVersion:   "2024-06-01",
			wantBeta:      "prompt-caching-2024-07-31",
		},
		{
			name:        "native messages defaults the version",
			protocols:   map[string][]string{"glm-*": {dto.ModelProtocolChat, dto.ModelProtocolMessages}},
			model:       "glm-5.3",
			relayMode:   relayconstant.RelayModeChatCompletions,
			wantVersion: "2023-06-01",
		},
		{
			name:          "converted model gets no claude headers",
			protocols:     map[string][]string{"claude-*": {dto.ModelProtocolChat, dto.ModelProtocolMessages}},
			model:         "deepseek-v3",
			relayMode:     relayconstant.RelayModeChatCompletions,
			clientVersion: "2024-06-01",
			clientBeta:    "prompt-caching-2024-07-31",
		},
		{
			name:          "responses reroute gets no claude headers",
			protocols:     map[string][]string{"glm-*": {dto.ModelProtocolChat, dto.ModelProtocolMessages}},
			model:         "glm-5.3",
			relayMode:     relayconstant.RelayModeResponses,
			clientVersion: "2024-06-01",
			clientBeta:    "prompt-caching-2024-07-31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotVersion, gotBeta string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotVersion = r.Header.Get("anthropic-version")
				gotBeta = r.Header.Get("anthropic-beta")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[],"usage":{"input_tokens":1,"output_tokens":1}}`))
			}))
			defer server.Close()

			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(`{"model":"`+tt.model+`"}`))
			context.Request.Header.Set("Content-Type", "application/json")
			if tt.clientVersion != "" {
				context.Request.Header.Set("anthropic-version", tt.clientVersion)
			}
			if tt.clientBeta != "" {
				context.Request.Header.Set("anthropic-beta", tt.clientBeta)
			}

			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelId:         31,
					ChannelBaseUrl:    server.URL,
					ChannelType:       constant.ChannelTypeAstraFlow,
					ApiKey:            "sk-test",
					UpstreamModelName: tt.model,
					ChannelOtherSettings: dto.ChannelOtherSettings{
						ModelProtocols: tt.protocols,
					},
				},
				RequestURLPath:  "/v1/messages",
				RelayFormat:     types.RelayFormatClaude,
				RelayMode:       tt.relayMode,
				OriginModelName: tt.model,
			}

			respAny, err := (&Adaptor{}).DoRequest(context, info, bytes.NewBufferString(`{"model":"`+tt.model+`"}`))
			require.NoError(t, err)
			resp, ok := respAny.(*http.Response)
			require.True(t, ok, "expected *http.Response, got %T", respAny)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, tt.wantVersion, gotVersion)
			assert.Equal(t, tt.wantBeta, gotBeta)
		})
	}
}

// TestAstraflowDowngradesRejectedNativeProtocol 锁定运行时兜底: 配置声明原生
// 支持 messages,但上游用 "not implemented" 明确拒绝时,该组合在进程内被记住,
// 后续请求不再直连,改为降级转换;且探测读取的错误体必须被还原,不影响调用方解析。
func TestAstraflowDowngradesRejectedNativeProtocol(t *testing.T) {
	resetNativeProtocolDowngradeMemoForTest()
	t.Cleanup(resetNativeProtocolDowngradeMemoForTest)

	gin.SetMode(gin.TestMode)

	const upstreamErrorBody = `{"error":{"message":"not implemented","type":"upstream_error"}}`
	const requestBody = `{"model":"glm-5.3","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(upstreamErrorBody))
	}))
	defer server.Close()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(requestBody))

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         7,
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
		OriginModelName: "glm-5.3",
	}
	adaptor := &Adaptor{}

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, server.URL+"/v1/messages", url, "declared native messages must be used before the first failure")

	respAny, err := adaptor.DoRequest(context, info, bytes.NewBufferString(requestBody))
	require.NoError(t, err)
	resp, ok := respAny.(*http.Response)
	require.True(t, ok, "expected *http.Response, got %T", respAny)
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, upstreamErrorBody, string(body), "peeking must restore the response body for the caller")

	url, err = adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, server.URL+"/v1/chat/completions", url, "after a rejection the combination must downgrade to chat")

	converted, err := adaptor.ConvertClaudeRequest(context, info, &dto.ClaudeRequest{Model: "glm-5.3"})
	require.NoError(t, err)
	_, isClaudeRequest := converted.(*dto.ClaudeRequest)
	assert.False(t, isClaudeRequest, "downgraded requests must be converted to chat")
}

// TestAstraflowChannelTestDoesNotDowngradeProtocol 锁定渠道测试的边界: 渠道测试走的是
// 合成请求(header 透传同样对测试请求短路, relay/channel/api_request.go), 它命中的
// "not implemented" 不构成生产流量的证据, 不得把 (渠道, 模型, 协议) 永久降级。
func TestAstraflowChannelTestDoesNotDowngradeProtocol(t *testing.T) {
	resetNativeProtocolDowngradeMemoForTest()
	t.Cleanup(resetNativeProtocolDowngradeMemoForTest)

	gin.SetMode(gin.TestMode)

	server := newRejectingUpstream(t, http.StatusInternalServerError, `{"error":{"message":"not implemented","type":"upstream_error"}}`)

	info := newNativeMessagesInfo(server.URL, 24)
	info.IsChannelTest = true
	adaptor := &Adaptor{}

	respAny, err := adaptor.DoRequest(newMessagesContext(), info, bytes.NewBufferString(`{"model":"glm-5.3"}`))
	require.NoError(t, err)
	resp, ok := respAny.(*http.Response)
	require.True(t, ok, "expected *http.Response, got %T", respAny)
	defer func() { _ = resp.Body.Close() }()

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, server.URL+"/v1/messages", url, "a channel test must not downgrade production traffic")

	_, downgraded := nativeProtocolDowngradeMemo.Load(nativeProtocolMemoKey(info, dto.ModelProtocolMessages))
	assert.False(t, downgraded, "a channel test must not record a downgrade")
}

// TestAstraflowStreamingChatAsksUpstreamForUsage 锁定流式 usage 契约: 渠道类型 59
// 在 streamSupportedChannels 中, 故流式请求会把 stream_options.include_usage 发给
// 上游, 上游返回的 usage 才不用按文本估算; 非流式请求不带该字段。
func TestAstraflowStreamingChatAsksUpstreamForUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newInfo := func(isStream bool) *relaycommon.RelayInfo {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		common.SetContextKey(context, constant.ContextKeyChannelType, constant.ChannelTypeAstraFlow)
		common.SetContextKey(context, constant.ContextKeyOriginalModel, "deepseek-v3")
		common.SetContextKey(context, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{
			ModelProtocols: map[string][]string{"claude-*": {dto.ModelProtocolChat, dto.ModelProtocolMessages}},
		})

		info := &relaycommon.RelayInfo{
			RelayFormat:     types.RelayFormatClaude,
			RelayMode:       relayconstant.RelayModeChatCompletions,
			IsStream:        isStream,
			OriginModelName: "deepseek-v3",
		}
		info.InitChannelMeta(context)
		return info
	}

	adaptor := &Adaptor{}
	maxTokens := uint(16)
	request := &dto.ClaudeRequest{
		Model:     "deepseek-v3",
		MaxTokens: &maxTokens,
		Messages:  []dto.ClaudeMessage{{Role: "user", Content: "hi"}},
	}

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	streamInfo := newInfo(true)
	require.True(t, streamInfo.SupportStreamOptions, "astraflow must be registered in streamSupportedChannels")
	converted, err := adaptor.ConvertClaudeRequest(context, streamInfo, request)
	require.NoError(t, err)
	chatRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok, "expected a chat request, got %T", converted)
	require.NotNil(t, chatRequest.StreamOptions)
	assert.True(t, chatRequest.StreamOptions.IncludeUsage)

	plainInfo := newInfo(false)
	converted, err = adaptor.ConvertClaudeRequest(context, plainInfo, request)
	require.NoError(t, err)
	chatRequest, ok = converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok, "expected a chat request, got %T", converted)
	assert.Nil(t, chatRequest.StreamOptions, "a non-streaming request must not ask for stream usage")
}

// TestAstraflowKeepsNativeProtocolOnUnrelatedUpstreamError 锁定标记范围: 参数级
// 的普通 4xx("unsupported parameter: temperature")不是协议拒绝, 不得让该组合被
// 永久降级, 后续请求仍直连原生端点。
func TestAstraflowKeepsNativeProtocolOnUnrelatedUpstreamError(t *testing.T) {
	resetNativeProtocolDowngradeMemoForTest()
	t.Cleanup(resetNativeProtocolDowngradeMemoForTest)

	gin.SetMode(gin.TestMode)

	const upstreamErrorBody = `{"error":{"message":"unsupported parameter: temperature","type":"invalid_request_error"}}`
	server := newRejectingUpstream(t, http.StatusBadRequest, upstreamErrorBody)

	info := newNativeMessagesInfo(server.URL, 21)
	adaptor := &Adaptor{}

	respAny, err := adaptor.DoRequest(newMessagesContext(), info, bytes.NewBufferString(`{"model":"glm-5.3"}`))
	require.NoError(t, err)
	resp, ok := respAny.(*http.Response)
	require.True(t, ok, "expected *http.Response, got %T", respAny)
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, upstreamErrorBody, string(body))

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, server.URL+"/v1/messages", url, "a parameter-level 4xx must not downgrade the protocol")
}

// TestAstraflowRestoresAndMemoizesLargeErrorBody 锁定超大错误体的处理: 超过窥探上限
// 的响应体读完后必须逐字节完整, 且命中签名时仍要记入降级记忆。
func TestAstraflowRestoresAndMemoizesLargeErrorBody(t *testing.T) {
	resetNativeProtocolDowngradeMemoForTest()
	t.Cleanup(resetNativeProtocolDowngradeMemoForTest)

	gin.SetMode(gin.TestMode)

	upstreamErrorBody := `{"error":{"message":"not implemented","detail":"` +
		strings.Repeat("x", nativeProtocolBodyPeekLimit) + `"}}`
	require.Greater(t, len(upstreamErrorBody), nativeProtocolBodyPeekLimit, "fixture must exceed the peek limit")

	server := newRejectingUpstream(t, http.StatusInternalServerError, upstreamErrorBody)

	info := newNativeMessagesInfo(server.URL, 22)
	adaptor := &Adaptor{}

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, server.URL+"/v1/messages", url)

	respAny, err := adaptor.DoRequest(newMessagesContext(), info, bytes.NewBufferString(`{"model":"glm-5.3"}`))
	require.NoError(t, err)
	resp, ok := respAny.(*http.Response)
	require.True(t, ok, "expected *http.Response, got %T", respAny)
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, upstreamErrorBody, string(body), "the replayed prefix plus the remainder must equal the original body")

	url, err = adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, server.URL+"/v1/chat/completions", url, "a rejection in a large body must still downgrade")
}

// TestAstraflowDowngradeWatermarkKeepsInFlightRequestsNative 锁定水位线语义: 降级
// 只对"记录之后才开始"的请求生效; 记录之前(已在途)的请求保持它发出时的原生决定,
// 否则它的原生响应会被降级后的处理器按 chat 解析。
func TestAstraflowDowngradeWatermarkKeepsInFlightRequestsNative(t *testing.T) {
	resetNativeProtocolDowngradeMemoForTest()
	t.Cleanup(resetNativeProtocolDowngradeMemoForTest)

	gin.SetMode(gin.TestMode)

	const baseURL = "https://api.modelverse.cn"
	adaptor := &Adaptor{}

	info := newNativeMessagesInfo(baseURL, 23)
	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, baseURL+"/v1/messages", url)

	// 模拟另一个请求刚刚因协议拒绝写入记忆。
	nativeProtocolDowngradeMemo.Store(nativeProtocolMemoKey(info, dto.ModelProtocolMessages), time.Now())

	info.StartTime = time.Now().Add(time.Second) // 记录之后才开始
	url, err = adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, baseURL+"/v1/chat/completions", url, "requests started after the recording must downgrade")

	info.StartTime = time.Now().Add(-time.Second) // 记录之前就已发出
	url, err = adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, baseURL+"/v1/messages", url, "requests in flight before the recording must keep the native protocol")
}

// newGeminiTextInfo 构造一次 gemini 原生格式的文本生成会话。
func newGeminiTextInfo(baseURL string, channelID int, protocols map[string][]string, model string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:            channelID,
			ChannelBaseUrl:       baseURL,
			ChannelType:          constant.ChannelTypeAstraFlow,
			ApiKey:               "sk-test",
			UpstreamModelName:    model,
			ChannelOtherSettings: dto.ChannelOtherSettings{ModelProtocols: protocols},
		},
		RequestURLPath:  "/v1beta/models/" + model + ":generateContent",
		RelayFormat:     types.RelayFormatGemini,
		RelayMode:       relayconstant.RelayModeGemini,
		OriginModelName: model,
	}
}

// newGeminiContext 构造一次 POST /v1beta/models/{model}:generateContent 的 gin 上下文。
func newGeminiContext(path string) *gin.Context {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{}`))
	return context
}

// TestConvertGeminiRequestKeepsNativeBodyWhenDeclared 锁定: 声明原生 gemini 的模型,
// gemini 请求体原样转发(与渠道类型 24 同样的形状归一化);未声明时返回转换后的
// OpenAI chat 请求,由 GetRequestURL 指向 chat 端点。
func TestConvertGeminiRequestKeepsNativeBodyWhenDeclared(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context := newGeminiContext("/v1beta/models/gemini-3.8-flash:generateContent")

	newInfo := func(protocols map[string][]string) *relaycommon.RelayInfo {
		return newGeminiTextInfo("https://api.modelverse.cn", 31, protocols, "gemini-3.8-flash")
	}

	adaptor := &Adaptor{}
	request := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "hi"}}}},
	}

	converted, err := adaptor.ConvertGeminiRequest(context, newInfo(map[string][]string{"gemini-*": {dto.ModelProtocolGemini}}), request)
	require.NoError(t, err)
	assert.Same(t, request, converted, "declared-native gemini must be forwarded untouched")

	converted, err = adaptor.ConvertGeminiRequest(context, newInfo(map[string][]string{"gemini-*": {dto.ModelProtocolChat}}), request)
	require.NoError(t, err)
	_, isChatRequest := converted.(*dto.GeneralOpenAIRequest)
	assert.True(t, isChatRequest, "an unmatched model must be downgraded to a chat request, got %T", converted)
}

// TestConvertGeminiRequestDowngradeAsksUpstreamForUsage 锁定降级链路的流式 usage 契约:
// 转成 chat 发出的流式请求要带 stream_options.include_usage(与 messages/responses
// 的降级一致), 否则上游不回 usage、结算只能按文本估算;非流式请求不带该字段。
func TestConvertGeminiRequestDowngradeAsksUpstreamForUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context := newGeminiContext("/v1beta/models/glm-5.3-flash:generateContent")

	protocols := map[string][]string{"glm-*": {dto.ModelProtocolChat}}
	newInfo := func(isStream bool) *relaycommon.RelayInfo {
		info := newGeminiTextInfo("https://api.modelverse.cn", 32, protocols, "glm-5.3-flash")
		info.IsStream = isStream
		// 渠道注册在 streamSupportedChannels 里, 该能力位与是否本次流式无关:
		// 只有两个条件同时成立才该带 include_usage, 所以这里恒为 true, 由 IsStream 区分。
		info.SupportStreamOptions = true
		return info
	}

	adaptor := &Adaptor{}
	request := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "hi"}}}},
	}

	converted, err := adaptor.ConvertGeminiRequest(context, newInfo(true), request)
	require.NoError(t, err)
	streamRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok, "expected a chat request, got %T", converted)
	require.NotNil(t, streamRequest.StreamOptions, "a streaming downgrade must ask the upstream for usage")
	assert.True(t, streamRequest.StreamOptions.IncludeUsage)

	converted, err = adaptor.ConvertGeminiRequest(context, newInfo(false), request)
	require.NoError(t, err)
	plainRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok, "expected a chat request, got %T", converted)
	assert.Nil(t, plainRequest.StreamOptions, "a non-streaming request must not ask for stream usage")
}

// TestConvertGeminiRequestRejectsNonTextActions 锁定 gemini 请求转换的作用范围:
// 只有文本生成 action 才参与"直传 or 转 chat"的分流。`countTokens` 之类的非文本
// action 在两条路上都不通, 必须保持改动前的快速失败——否则 chat 形状的请求体会被
// 发到客户端的 gemini action 路径上, 变成打到上游的垃圾请求。
func TestConvertGeminiRequestRejectsNonTextActions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "hi"}}}},
	}

	tests := []struct {
		name      string
		path      string
		protocols map[string][]string
	}{
		{
			name: "countTokens without a declaration", path: "/v1beta/models/glm-5.3-flash:countTokens",
			protocols: map[string][]string{"glm-*": {dto.ModelProtocolChat}},
		},
		{
			name: "countTokens on an unconfigured channel", path: "/v1beta/models/glm-5.3-flash:countTokens",
		},
		{
			name: "unknown action", path: "/v1beta/models/gemini-3.8-flash:somethingElse",
			protocols: map[string][]string{"gemini-*": {dto.ModelProtocolGemini}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := newGeminiTextInfo("https://api.modelverse.cn", 36, tt.protocols, "glm-5.3-flash")
			info.RequestURLPath = tt.path

			converted, err := (&Adaptor{}).ConvertGeminiRequest(newGeminiContext(tt.path), info, request)
			require.Error(t, err, "a non-text gemini action must fail fast, got %T", converted)
		})
	}
}

// TestGetRequestURLDisablesPingForNativeGeminiStream 锁定 gemini 原生流式的 ping 边界:
// gemini 客户端会逐条 JSON 解析 SSE 事件, 网关注入的 `: PING` 注释只能关掉——渠道类型
// 24 在同一位置做了同样的事(info.DisablePing = true), 非流式与降级会话不受影响。
func TestGetRequestURLDisablesPingForNativeGeminiStream(t *testing.T) {
	protocols := map[string][]string{"gemini-*": {dto.ModelProtocolGemini}}
	newInfo := func(isStream bool) *relaycommon.RelayInfo {
		info := newGeminiTextInfo("https://api.modelverse.cn", 37, protocols, "gemini-3.8-flash")
		info.IsStream = isStream
		if isStream {
			info.RequestURLPath = "/v1beta/models/gemini-3.8-flash:streamGenerateContent"
		}
		return info
	}

	streamInfo := newInfo(true)
	_, err := (&Adaptor{}).GetRequestURL(streamInfo)
	require.NoError(t, err)
	assert.True(t, streamInfo.DisablePing, "a native gemini stream must not receive gateway ping frames")

	plainInfo := newInfo(false)
	_, err = (&Adaptor{}).GetRequestURL(plainInfo)
	require.NoError(t, err)
	assert.False(t, plainInfo.DisablePing)

	downgraded := newGeminiTextInfo("https://api.modelverse.cn", 38, map[string][]string{"glm-*": {dto.ModelProtocolChat}}, "glm-5.3-flash")
	_, err = (&Adaptor{}).GetRequestURL(downgraded)
	require.NoError(t, err)
	assert.False(t, downgraded.DisablePing, "a downgraded chat session keeps the standard stream handling")
}

// TestAstraflowKeepsProtocolWhenBodyPassthrough 锁定透传与会话降级的边界: 请求体
// 透传时上游收到的就是客户端原始报文, 形状改不了, 降级无从谈起——此时不得把
// (渠道, 模型, 协议) 写进降级记忆, 也不该打出"已降级"的日志。
func TestAstraflowKeepsProtocolWhenBodyPassthrough(t *testing.T) {
	resetNativeProtocolDowngradeMemoForTest()
	t.Cleanup(resetNativeProtocolDowngradeMemoForTest)

	gin.SetMode(gin.TestMode)

	server := newRejectingUpstream(t, http.StatusInternalServerError, `{"error":{"message":"not implemented","type":"upstream_error"}}`)

	info := newGeminiTextInfo(server.URL, 39, map[string][]string{"gemini-*": {dto.ModelProtocolGemini}}, "gemini-3.8-flash")
	info.ChannelSetting.PassThroughBodyEnabled = true

	respAny, err := (&Adaptor{}).DoRequest(newGeminiContext(info.RequestURLPath), info, bytes.NewBufferString(`{"contents":[]}`))
	require.NoError(t, err)
	resp, ok := respAny.(*http.Response)
	require.True(t, ok, "expected *http.Response, got %T", respAny)
	defer func() { _ = resp.Body.Close() }()

	_, downgraded := nativeProtocolDowngradeMemo.Load(nativeProtocolMemoKey(info, dto.ModelProtocolGemini))
	assert.False(t, downgraded, "a pass-through body cannot be downgraded, so nothing may be recorded")
}

// TestGetRequestURLForGeminiTextSessions 锁定 gemini 原生格式的端点选择: 声明了
// gemini 的模型按上游模型名重建 /{version}/models/{model}:{action}(流式补 alt=sse),
// 未声明/未配置的模型降级到 chat 端点；embedding 等非文本 action 保持客户端路径。
// 两类路径都不得把客户端的查询串(尤其 ?key=<网关令牌>)转发给上游。
func TestGetRequestURLForGeminiTextSessions(t *testing.T) {
	const baseURL = "https://api.modelverse.cn"
	declared := map[string][]string{"gemini-*": {dto.ModelProtocolGemini}}

	tests := []struct {
		name          string
		protocols     map[string][]string
		path          string
		upstreamModel string
		isStream      bool
		want          string
	}{
		{
			name: "declared native generateContent", protocols: declared,
			path: "/v1beta/models/gemini-3.8-flash:generateContent", upstreamModel: "gemini-3.8-flash",
			want: baseURL + "/v1beta/models/gemini-3.8-flash:generateContent",
		},
		{
			name: "declared native streamGenerateContent forces alt=sse", protocols: declared,
			path: "/v1beta/models/gemini-3.8-flash:streamGenerateContent", upstreamModel: "gemini-3.8-flash", isStream: true,
			want: baseURL + "/v1beta/models/gemini-3.8-flash:streamGenerateContent?alt=sse",
		},
		{
			name: "client alt=sse upgrades generateContent to the streaming action", protocols: declared,
			path: "/v1beta/models/gemini-3.8-flash:generateContent?alt=sse", upstreamModel: "gemini-3.8-flash", isStream: true,
			want: baseURL + "/v1beta/models/gemini-3.8-flash:streamGenerateContent?alt=sse",
		},
		{
			// 声明键匹配 model_mapping 之后的上游名, URL 也必须用上游名。
			name:      "channel model mapping is applied to the upstream path",
			protocols: map[string][]string{"google/gemini-*": {dto.ModelProtocolGemini}},
			path:      "/v1beta/models/gemini-3.8-flash:generateContent", upstreamModel: "google/gemini-3.8-flash",
			want: baseURL + "/v1beta/models/google/gemini-3.8-flash:generateContent",
		},
		{
			name: "client auth query is never forwarded", protocols: declared,
			path: "/v1beta/models/gemini-3.8-flash:generateContent?key=sk-client-token", upstreamModel: "gemini-3.8-flash",
			want: baseURL + "/v1beta/models/gemini-3.8-flash:generateContent",
		},
		{
			name: "non-beta client path is normalized to the configured version", protocols: declared,
			path: "/v1/models/gemini-3.8-flash:generateContent", upstreamModel: "gemini-3.8-flash",
			want: baseURL + "/v1beta/models/gemini-3.8-flash:generateContent",
		},
		{
			// 客户端可能把分隔符转义成 %3A: 路由与请求校验用解码后的路径, 这里也必须解码,
			// 否则原生会话会静默退化成"非文本路径", 把客户端模型名原样发给上游。
			name: "percent-escaped action delimiter is decoded", protocols: declared,
			path: "/v1beta/models/gemini-3.8-flash%3AgenerateContent", upstreamModel: "gemini-3.8-flash",
			want: baseURL + "/v1beta/models/gemini-3.8-flash:generateContent",
		},
		{
			name: "undeclared model downgrades to chat", protocols: map[string][]string{"gemini-*": {dto.ModelProtocolChat}},
			path: "/v1beta/models/gemini-3.8-flash:generateContent", upstreamModel: "gemini-3.8-flash",
			want: baseURL + "/v1/chat/completions",
		},
		{
			name: "unconfigured channel downgrades to chat",
			path: "/v1beta/models/gemini-3.8-flash:generateContent", upstreamModel: "gemini-3.8-flash",
			want: baseURL + "/v1/chat/completions",
		},
		{
			name: "embedding actions keep the client path", protocols: declared,
			path: "/v1beta/models/gemini-embedding-001:embedContent?key=sk-client-token", upstreamModel: "gemini-embedding-001",
			want: baseURL + "/v1beta/models/gemini-embedding-001:embedContent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := newGeminiTextInfo(baseURL, 33, tt.protocols, tt.upstreamModel)
			info.RequestURLPath = tt.path
			info.IsStream = tt.isStream

			url, err := (&Adaptor{}).GetRequestURL(info)
			require.NoError(t, err)
			assert.Equal(t, tt.want, url)
		})
	}
}

// TestAstraflowKeepsNativeGeminiOnTransientRequestModeError 锁定 gemini 的标记范围:
// 星图 gemini 端点的 400 "Invalid param: get request mode" 是上游整体抖动(实测同一
// 模型同一请求几分钟内 200/400 交替, 期间所有模型一起 400), 不是"该模型没有 gemini
// 协议"。这类瞬时故障不得写进降级记忆, 否则一次抖动就把原生 gemini 永久降级成有损转换。
func TestAstraflowKeepsNativeGeminiOnTransientRequestModeError(t *testing.T) {
	resetNativeProtocolDowngradeMemoForTest()
	t.Cleanup(resetNativeProtocolDowngradeMemoForTest)

	gin.SetMode(gin.TestMode)

	const upstreamErrorBody = `{"error":{"message":"[trace_id: x] Invalid param: get request mode","type":"invalid_request_error"}}`
	server := newRejectingUpstream(t, http.StatusBadRequest, upstreamErrorBody)

	info := newGeminiTextInfo(server.URL, 35, map[string][]string{"gemini-*": {dto.ModelProtocolGemini}}, "gemini-3.8-flash")
	adaptor := &Adaptor{}

	respAny, err := adaptor.DoRequest(newGeminiContext(info.RequestURLPath), info, bytes.NewBufferString(`{"contents":[]}`))
	require.NoError(t, err)
	resp, ok := respAny.(*http.Response)
	require.True(t, ok, "expected *http.Response, got %T", respAny)
	defer func() { _ = resp.Body.Close() }()

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, server.URL+"/v1beta/models/gemini-3.8-flash:generateContent", url, "a transient upstream failure must not downgrade a declared native protocol")

	_, downgraded := nativeProtocolDowngradeMemo.Load(nativeProtocolMemoKey(info, dto.ModelProtocolGemini))
	assert.False(t, downgraded, "a transient upstream failure must not be recorded")
}

// TestAstraflowDowngradesRejectedGeminiProtocol 锁定 gemini 的运行时兜底: 声明了原生
// gemini 但上游用 "not implemented" 拒绝时, 该组合在进程内被记住, 后续请求改走
// chat 转换; 探测读取的错误体同样要还原。
func TestAstraflowDowngradesRejectedGeminiProtocol(t *testing.T) {
	resetNativeProtocolDowngradeMemoForTest()
	t.Cleanup(resetNativeProtocolDowngradeMemoForTest)

	gin.SetMode(gin.TestMode)

	const upstreamErrorBody = `{"error":{"message":"not implemented","type":"upstream_error"}}`
	const requestBody = `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`
	server := newRejectingUpstream(t, http.StatusInternalServerError, upstreamErrorBody)

	info := newGeminiTextInfo(server.URL, 34, map[string][]string{"gemini-*": {dto.ModelProtocolGemini}}, "gemini-3.8-flash")
	adaptor := &Adaptor{}

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, server.URL+"/v1beta/models/gemini-3.8-flash:generateContent", url)

	respAny, err := adaptor.DoRequest(newGeminiContext(info.RequestURLPath), info, bytes.NewBufferString(requestBody))
	require.NoError(t, err)
	resp, ok := respAny.(*http.Response)
	require.True(t, ok, "expected *http.Response, got %T", respAny)
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, upstreamErrorBody, string(body), "peeking must restore the response body for the caller")

	url, err = adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, server.URL+"/v1/chat/completions", url, "after a rejection the combination must downgrade to chat")

	converted, err := adaptor.ConvertGeminiRequest(newGeminiContext(info.RequestURLPath), info, &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "hi"}}}},
	})
	require.NoError(t, err)
	_, isChatRequest := converted.(*dto.GeneralOpenAIRequest)
	assert.True(t, isChatRequest, "downgraded requests must be converted to chat, got %T", converted)
}
