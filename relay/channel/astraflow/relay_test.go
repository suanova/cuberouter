package astraflow

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/security_setting"

	"github.com/gin-gonic/gin"
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
	requestBody          string
	upstreamBody         string
	isStream             bool
	wantPromptTokens     int // -1 表示不断言
	wantCompletionTokens int // -1 表示不断言
	wantBodyContains     string
}

// TestRelayChainNonTaskModes 锁定多模态非任务链路契约：chat/responses/
// embeddings/image 四种模式都走同一 OpenAI 直传链路——上游路径沿用客户端
// 路径、携带 Bearer 鉴权、请求体原样转发、响应经 OpenaiHandler 解析回写。
func TestRelayChainNonTaskModes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []relayChainCase{
		{
			name:          "chat completions",
			path:          "/v1/chat/completions",
			relayMode:     relayconstant.RelayModeChatCompletions,
			relayFormat:   types.RelayFormatOpenAI,
			model:         "deepseek-v3",
			requestBody:   `{"model":"deepseek-v3","messages":[{"role":"user","content":"hi"}]}`,
			upstreamBody:  `{"id":"chatcmpl-1","object":"chat.completion","created":1700000000,"model":"deepseek-v3","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":8,"total_tokens":20}}`,
			wantPromptTokens: 12, wantCompletionTokens: 8,
			wantBodyContains: `"chatcmpl-1"`,
		},
		{
			name:         "responses",
			path:         "/v1/responses",
			relayMode:    relayconstant.RelayModeResponses,
			relayFormat:  types.RelayFormatOpenAIResponses,
			model:        "deepseek-v3",
			requestBody:  `{"model":"deepseek-v3","input":"hi"}`,
			upstreamBody: `{"id":"resp_1","object":"response","created_at":1700000000,"model":"deepseek-v3","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"pong"}]}],"usage":{"input_tokens":5,"output_tokens":3,"total_tokens":8}}`,
			// responses 响应体没有 prompt_tokens，OpenaiHandler 走估算路径，
			// usage 非 nil 且响应体原样回写即可（该模式真实链路如此）。
			wantPromptTokens: -1, wantCompletionTokens: -1,
			wantBodyContains: `"resp_1"`,
		},
		{
			name:          "embeddings",
			path:          "/v1/embeddings",
			relayMode:     relayconstant.RelayModeEmbeddings,
			relayFormat:   types.RelayFormatEmbedding,
			model:         "text-embedding-3-large",
			requestBody:   `{"model":"text-embedding-3-large","input":"hi"}`,
			upstreamBody:  `{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"model":"text-embedding-3-large","usage":{"prompt_tokens":8,"total_tokens":8}}`,
			wantPromptTokens: 8, wantCompletionTokens: 0,
			wantBodyContains: `"embedding"`,
		},
		{
			name:          "image generation",
			path:          "/v1/images/generations",
			relayMode:     relayconstant.RelayModeImagesGenerations,
			relayFormat:   types.RelayFormatOpenAIImage,
			model:         "gpt-image-1",
			requestBody:   `{"model":"gpt-image-1","prompt":"a cat","n":1,"size":"1024x1024"}`,
			upstreamBody:  `{"created":1700000000,"data":[{"url":"https://cdn.example.com/cat.png"}]}`,
			wantPromptTokens: -1, wantCompletionTokens: -1,
			wantBodyContains: `https://cdn.example.com/cat.png`,
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
			ChannelBaseUrl:   server.URL,
			ChannelType:      constant.ChannelTypeAstraFlow,
			ApiKey:           "sk-test",
			UpstreamModelName: tt.model,
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
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, tt.path, gotPath)
	assert.Equal(t, "Bearer sk-test", gotAuth)
	assert.Equal(t, tt.requestBody, string(gotBody))

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
