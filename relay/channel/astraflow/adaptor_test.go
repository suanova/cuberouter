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
