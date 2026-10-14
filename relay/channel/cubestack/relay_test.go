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
	"github.com/QuantumNous/new-api/setting/security_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	chatRequestBody = `{"model":"qwen38-27b","messages":[{"role":"user","content":"hi"}]}`
	chatUpstreamBody = `{"id":"chatcmpl-1","object":"chat.completion","created":1700000000,"model":"qwen38-27b",` +
		`"choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],` +
		`"usage":{"prompt_tokens":12,"completion_tokens":8,"total_tokens":20}}`

	responsesRequestBody = `{"model":"qwen38-27b","input":"hi"}`
	responsesUpstreamBody = `{"id":"resp_1","object":"response","created_at":1700000000,"model":"qwen38-27b","status":"completed",` +
		`"output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed",` +
		`"content":[{"type":"output_text","text":"pong","annotations":[]}]}],` +
		`"usage":{"input_tokens":5,"output_tokens":3,"total_tokens":8}}`

	messagesRequestBody = `{"model":"qwen38-27b","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`
	messagesUpstreamBody = `{"id":"msg_1","type":"message","role":"assistant","model":"qwen38-27b",` +
		`"content":[{"type":"text","text":"pong"}],"stop_reason":"end_turn",` +
		`"usage":{"input_tokens":3,"output_tokens":2}}`
)

// relayChainCase 定义一次完整非任务中继链路断言：假上游记录收到的请求，断言方法、
// 上游路径、鉴权头与 Claude 协议头，请求体逐字节一致，随后用适配器 DoResponse 解析
// 假上游返回体并确认 usage 与回写内容。
type relayChainCase struct {
	name                 string
	path                 string
	relayMode            int
	relayFormat          types.RelayFormat
	upstreamModel        string
	protocols            map[string][]string // 渠道 model_protocols 声明; nil 表示不配置
	apiKey               string
	requestBody          string
	upstreamBody         string
	isStream             bool
	wantUpstreamPath     string
	skipRequestEquality  bool
	wantPromptTokens     int
	wantCompletionTokens int
	wantBodyContains     string
	// wantAnthropicVersion 为空表示"上游请求里不该出现 anthropic-version"：
	// 该头只在原生 Anthropic 直连时才有意义。
	wantAnthropicVersion string
	channelID            int
}

// TestCubeStackDefaultsToNativeProtocols 锁定 CubeStack 文本侧的核心契约: 未声明
// model_protocols 的模型，三条文本协议**全部**按客户端原路径直达上游，不做任何转换。
// 这是本渠道与 AstraFlow 的基线差异(后者未声明时 messages 转 chat)，也是
// "配好渠道就能被 Claude Code / Codex 直接用"的前提。
func TestCubeStackDefaultsToNativeProtocols(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []relayChainCase{
		{
			name: "chat completions", path: "/v1/chat/completions",
			relayMode: relayconstant.RelayModeChatCompletions, relayFormat: types.RelayFormatOpenAI,
			upstreamModel: "qwen38-27b", apiKey: "sk-test", channelID: 201,
			requestBody: chatRequestBody, upstreamBody: chatUpstreamBody,
			wantUpstreamPath: "/v1/chat/completions",
			wantPromptTokens: 12, wantCompletionTokens: 8,
			wantBodyContains: `"chatcmpl-1"`,
		},
		{
			name: "responses stay on the responses endpoint", path: "/v1/responses",
			relayMode: relayconstant.RelayModeResponses, relayFormat: types.RelayFormatOpenAIResponses,
			upstreamModel: "qwen38-27b", apiKey: "sk-test", channelID: 202,
			requestBody: responsesRequestBody, upstreamBody: responsesUpstreamBody,
			wantUpstreamPath: "/v1/responses",
			wantPromptTokens: 5, wantCompletionTokens: 3,
			wantBodyContains: `"resp_1"`,
		},
		{
			name: "messages stay on the anthropic endpoint", path: "/v1/messages",
			relayMode: relayconstant.RelayModeChatCompletions, relayFormat: types.RelayFormatClaude,
			upstreamModel: "qwen38-27b", apiKey: "sk-test", channelID: 203,
			requestBody: messagesRequestBody, upstreamBody: messagesUpstreamBody,
			wantUpstreamPath: "/v1/messages",
			wantPromptTokens: 3, wantCompletionTokens: 2,
			wantBodyContains:     `"pong"`,
			wantAnthropicVersion: "2023-06-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runRelayChain(t, tt)
		})
	}
}

// TestCubeStackDeclarationOverridesTheNativeDefault 锁定声明仍然压过基线: 渠道把模型
// 声明成只有 chat 时，responses 与 messages 都要转成 chat 发出去、响应再转回各自形状，
// 并且此时不带 Claude 协议头——上游在那条路径上按 OpenAI 协议解析。
func TestCubeStackDeclarationOverridesTheNativeDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)

	chatOnly := map[string][]string{"qwen38-27b": {dto.ModelProtocolChat}}

	tests := []relayChainCase{
		{
			name: "declared chat-only responses are converted", path: "/v1/responses",
			relayMode: relayconstant.RelayModeResponses, relayFormat: types.RelayFormatOpenAIResponses,
			upstreamModel: "qwen38-27b", protocols: chatOnly, apiKey: "sk-test", channelID: 204,
			requestBody: responsesRequestBody, upstreamBody: chatUpstreamBody,
			wantUpstreamPath: "/v1/chat/completions", skipRequestEquality: true,
			wantPromptTokens: 12, wantCompletionTokens: 8,
			wantBodyContains: `"object":"response"`,
		},
		{
			name: "declared chat-only messages are converted", path: "/v1/messages",
			relayMode: relayconstant.RelayModeChatCompletions, relayFormat: types.RelayFormatClaude,
			upstreamModel: "qwen38-27b", protocols: chatOnly, apiKey: "sk-test", channelID: 205,
			requestBody: messagesRequestBody, upstreamBody: chatUpstreamBody,
			wantUpstreamPath: "/v1/chat/completions", skipRequestEquality: true,
			wantPromptTokens: 12, wantCompletionTokens: 8,
			wantBodyContains: `"type":"message"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runRelayChain(t, tt)
		})
	}
}

// TestCubeStackAcceptedWithoutCredentials 锁定 keyless 上游契约(与视频侧
// task/cubestack 一致): 渠道 key 为空时不得把空 Bearer 发到上游，也不该因此少发
// 请求；配置了 key 时照常带 Bearer。
func TestCubeStackAcceptedWithoutCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)

	runRelayChain(t, relayChainCase{
		name: "keyless upstream", path: "/v1/chat/completions",
		relayMode: relayconstant.RelayModeChatCompletions, relayFormat: types.RelayFormatOpenAI,
		upstreamModel: "qwen38-27b", apiKey: "", channelID: 206,
		requestBody: chatRequestBody, upstreamBody: chatUpstreamBody,
		wantPromptTokens: 12, wantCompletionTokens: 8,
	})
}

// TestRelayChainStreamingNativeMessages 锁定 Claude Code 实际走的那条链路: 未声明
// 任何协议的模型，流式 /v1/messages 直接收发 Anthropic SSE，usage 取自报文的
// input_tokens/output_tokens，不按文本估算。
func TestRelayChainStreamingNativeMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// StreamScannerHandler 需要正数 StreamingTimeout，测试环境由 main 的 InitEnv
	// 才会设置，这里按既有流式测试的模式保存/恢复。
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 300
	defer func() { constant.StreamingTimeout = oldStreamingTimeout }()

	const requestBody = `{"model":"qwen38-27b","max_tokens":16,"messages":[{"role":"user","content":"hi"}],"stream":true}`
	const upstreamBody = "" +
		"event: message_start\n" +
		`data: {"type":"message_start","message":{"id":"msg_s1","type":"message","role":"assistant","model":"qwen38-27b","content":[],"usage":{"input_tokens":7,"output_tokens":1}}}` + "\n\n" +
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
			ChannelId:         207,
			ChannelBaseUrl:    server.URL,
			ChannelType:       constant.ChannelTypeCubeStack,
			ApiKey:            "sk-test",
			UpstreamModelName: "qwen38-27b",
		},
		RequestURLPath:  "/v1/messages",
		RelayFormat:     types.RelayFormatClaude,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		IsStream:        true,
		OriginModelName: "qwen38-27b",
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
	assert.Contains(t, recorder.Body.String(), "content_block_delta", "native anthropic events must pass through untouched")
}

// TestCubeStackDowngradesRejectedProtocol 锁定运行时兜底在这个渠道上同样生效: 未声明
// 的模型默认按原生 messages 发出去，上游用"协议不支持"拒绝时该组合被记入进程内记忆，
// 后续请求改走 chat 转换；探测读取的错误体必须被还原，不影响调用方解析。
func TestCubeStackDowngradesRejectedProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const upstreamErrorBody = `{"error":{"message":"unsupported protocol","type":"invalid_request_error"}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(upstreamErrorBody))
	}))
	defer server.Close()

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(messagesRequestBody))

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         208,
			ChannelBaseUrl:    server.URL,
			ChannelType:       constant.ChannelTypeCubeStack,
			ApiKey:            "sk-test",
			UpstreamModelName: "qwen38-27b",
		},
		RequestURLPath:  "/v1/messages",
		RelayFormat:     types.RelayFormatClaude,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		OriginModelName: "qwen38-27b",
	}
	adaptor := &Adaptor{}

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, server.URL+"/v1/messages", url, "an undeclared model must start out native")

	respAny, err := adaptor.DoRequest(context, info, bytes.NewBufferString(messagesRequestBody))
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
}

// TestGetRequestURLRejectsCleartextUpstream 锁定发送前防线：安全开关开启时，
// 非回环的明文上游在 GetRequestURL 阶段即被拒绝，Bearer 凭证不会发出（CWE-319）。
// 其余用例使用 httptest 回环地址，不受影响。
func TestGetRequestURLRejectsCleartextUpstream(t *testing.T) {
	old := security_setting.GetSecuritySetting().RequireHTTPSChannelBaseURL
	security_setting.GetSecuritySetting().RequireHTTPSChannelBaseURL = true
	defer func() { security_setting.GetSecuritySetting().RequireHTTPSChannelBaseURL = old }()

	gin.SetMode(gin.TestMode)

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "http://upstream.example.com",
			ChannelType:       constant.ChannelTypeCubeStack,
			ApiKey:            "sk-test",
			UpstreamModelName: "qwen38-27b",
		},
		RequestURLPath: "/v1/chat/completions",
	}

	_, err := (&Adaptor{}).GetRequestURL(info)
	require.ErrorContains(t, err, "must use HTTPS")
}

// runRelayChain 执行一次完整链路：DoRequest 打向假上游并断言请求契约，
// DoResponse 解析响应并断言 usage 与回写内容。
func runRelayChain(t *testing.T, tt relayChainCase) {
	t.Helper()

	// StreamScannerHandler 需要正数 StreamingTimeout，测试环境由 main 的 InitEnv
	// 才会设置，这里按既有流式测试的模式保存/恢复。
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 300
	defer func() { constant.StreamingTimeout = oldStreamingTimeout }()

	var gotMethod, gotPath, gotAuth, gotAnthropicVersion string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotAnthropicVersion = r.Header.Get("anthropic-version")
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
			ChannelId:            tt.channelID,
			ChannelBaseUrl:       server.URL,
			ChannelType:          constant.ChannelTypeCubeStack,
			ApiKey:               tt.apiKey,
			UpstreamModelName:    tt.upstreamModel,
			ChannelOtherSettings: dto.ChannelOtherSettings{ModelProtocols: tt.protocols},
		},
		RequestURLPath:  tt.path,
		RelayFormat:     tt.relayFormat,
		RelayMode:       tt.relayMode,
		IsStream:        tt.isStream,
		OriginModelName: tt.upstreamModel,
	}

	respAny, err := adaptor.DoRequest(context, info, bytes.NewBufferString(tt.requestBody))
	require.NoError(t, err)
	resp, ok := respAny.(*http.Response)
	require.True(t, ok, "expected *http.Response, got %T", respAny)
	defer func() { _ = resp.Body.Close() }()

	wantPath := tt.wantUpstreamPath
	if wantPath == "" {
		wantPath = tt.path
	}
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, wantPath, gotPath)
	assert.Equal(t, wantAuthHeader(tt.apiKey), gotAuth)
	assert.Equal(t, tt.wantAnthropicVersion, gotAnthropicVersion)
	if !tt.skipRequestEquality {
		assert.Equal(t, tt.requestBody, string(gotBody))
	}

	usageAny, apiErr := adaptor.DoResponse(context, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usageAny)
	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok, "expected *dto.Usage, got %T", usageAny)

	if tt.wantPromptTokens > 0 {
		assert.Equal(t, tt.wantPromptTokens, usage.PromptTokens)
	}
	if tt.wantCompletionTokens > 0 {
		assert.Equal(t, tt.wantCompletionTokens, usage.CompletionTokens)
	}
	if tt.wantBodyContains != "" {
		assert.Contains(t, recorder.Body.String(), tt.wantBodyContains)
	}
}

// wantAuthHeader 给出该渠道在给定 key 下应发到上游的 Authorization 值: 空 key
// 意味着上游不校验凭证，不该出现空的 Bearer。
func wantAuthHeader(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	return "Bearer " + apiKey
}
