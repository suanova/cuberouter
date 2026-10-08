package openai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
	if constant.StreamingTimeout == 0 {
		constant.StreamingTimeout = 30
	}
}

func setupOaiStreamTest(t *testing.T, body io.ReadCloser) (*gin.Context, *http.Response, *relaycommon.RelayInfo) {
	t.Helper()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp := &http.Response{Body: body}

	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		RelayFormat: types.RelayFormatOpenAI,
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-4o",
		},
	}

	return c, resp, info
}

// errReader 首次 Read 即返回错误，模拟上游 body 读取失败。
type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("simulated upstream read error")
}

func (errReader) Close() error {
	return nil
}

// blockReader 阻塞读取直到 Close 被调用，模拟上游流持续无数据（用于空闲超时）。
type blockReader struct {
	mu     sync.Mutex
	closed bool
}

func (r *blockReader) Read(p []byte) (int, error) {
	for {
		r.mu.Lock()
		if r.closed {
			r.mu.Unlock()
			return 0, io.EOF
		}
		r.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
}

func (r *blockReader) Close() error {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
	return nil
}

func TestOpenAIStreamResultError_MapsEndReasons(t *testing.T) {
	cases := []struct {
		name       string
		reason     relaycommon.StreamEndReason
		wantCode   types.ErrorCode
		wantStatus int
	}{
		{"idle timeout", relaycommon.StreamEndReasonTimeout, types.ErrorCodeStreamIdleTimeout, http.StatusGatewayTimeout},
		{"client disconnected", relaycommon.StreamEndReasonClientGone, types.ErrorCodeStreamClientClosed, statusClientClosedRequest},
		{"ping fail", relaycommon.StreamEndReasonPingFail, types.ErrorCodeStreamClientClosed, statusClientClosedRequest},
		{"scanner error", relaycommon.StreamEndReasonScannerErr, types.ErrorCodeStreamScannerFailed, http.StatusBadGateway},
		{"panic", relaycommon.StreamEndReasonPanic, types.ErrorCodeStreamScannerFailed, http.StatusBadGateway},
		{"handler stop", relaycommon.StreamEndReasonHandlerStop, types.ErrorCodeStreamDataHandler, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := relaycommon.NewStreamStatus()
			st.SetEndReason(tc.reason, nil)
			apiErr := openAIStreamResultError(st)
			require.NotNil(t, apiErr, "应返回错误")
			assert.Equal(t, tc.wantCode, apiErr.GetErrorCode(), "错误码不匹配")
			assert.Equal(t, tc.wantStatus, apiErr.StatusCode, "HTTP 状态码不匹配")
		})
	}
}

func TestLastOpenAIStreamResponseFinished(t *testing.T) {
	cases := []struct {
		name string
		data string
		want bool
	}{
		{"empty", "", false},
		{"invalid json", "not-json", false},
		{"no choices", `{"id":"x","choices":[]}`, false},
		{"no finish reason", `{"id":"x","choices":[{"delta":{"content":"hi"}}]}`, false},
		{"explicit stop", `{"id":"x","choices":[{"delta":{},"finish_reason":"stop"}]}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, lastOpenAIStreamResponseFinished(tc.data), "data=%s", tc.data)
		})
	}
}

// 上游 body 读取失败（扫描器错误）时必须返回错误而不是成功 usage；
// 计费改由异常结束的部分结算承担（见 TestOaiStreamHandler_ZeroEvidenceSettlesZeroQuota）。
func TestOaiStreamHandler_ScannerError_ReturnsErrorNoUsage(t *testing.T) {
	setupOpenAIStreamSettleDB(t)
	c, resp, info := setupOaiStreamTest(t, errReader{})

	usage, apiErr := OaiStreamHandler(c, info, resp)

	require.NotNil(t, apiErr, "扫描器错误应返回错误")
	assert.Equal(t, types.ErrorCodeStreamScannerFailed, apiErr.GetErrorCode())
	assert.True(t, types.IsSkipRetryError(apiErr), "部分结算后不得允许换渠道重试")
	assert.Nil(t, usage, "扫描器错误不应返回 usage")
}

// 空 body（EOF 但无任何响应内容）必须视为不完整流返回错误，不向调用方返回成功 usage。
func TestOaiStreamHandler_EmptyBody_ReturnsStreamIncomplete(t *testing.T) {
	setupOpenAIStreamSettleDB(t)
	c, resp, info := setupOaiStreamTest(t, io.NopCloser(strings.NewReader("")))

	usage, apiErr := OaiStreamHandler(c, info, resp)

	require.NotNil(t, apiErr, "空 body 应返回错误")
	assert.Equal(t, types.ErrorCodeStreamIncomplete, apiErr.GetErrorCode())
	assert.True(t, types.IsSkipRetryError(apiErr), "部分结算后不得允许换渠道重试")
	assert.Nil(t, usage, "空 body 不应返回 usage")
}

// 上游无任何数据且空闲超时：必须返回 stream_idle_timeout 错误，不得合成成功 usage。
func TestOaiStreamHandler_IdleTimeout_ReturnsErrorNoUsage(t *testing.T) {
	setupOpenAIStreamSettleDB(t)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 1
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	c, resp, info := setupOaiStreamTest(t, &blockReader{})

	usage, apiErr := OaiStreamHandler(c, info, resp)

	require.NotNil(t, apiErr, "空闲超时应返回错误")
	assert.Equal(t, types.ErrorCodeStreamIdleTimeout, apiErr.GetErrorCode())
	assert.True(t, types.IsSkipRetryError(apiErr), "部分结算后不得允许换渠道重试")
	assert.Nil(t, usage, "空闲超时不应返回 usage（不得合成计费）")
}

const abnormalSettleContentChunk = "data: {\"id\":\"chatcmpl_1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello world\"}}]}\n\n"

// 上游出流后卡死（空闲超时）不再全额免单：按已收文本估算部分结算，
// 写一条 is_stream=true 的消费日志，钱包实际扣减，错误仍保持 SkipRetry。
func TestOaiStreamHandler_AbnormalEndSettlesPartialUsage(t *testing.T) {
	setupOpenAIStreamSettleDB(t)
	user := createOpenAIStreamSettleUser(t, 1_000_000)

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 1
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	c, resp, info := newAbnormalSettleStreamTest(t, &oneChunkThenBlockReader{data: abnormalSettleContentChunk}, user)

	usage, apiErr := OaiStreamHandler(c, info, resp)

	require.NotNil(t, apiErr, "空闲超时仍应返回错误")
	assert.Equal(t, types.ErrorCodeStreamIdleTimeout, apiErr.GetErrorCode())
	assert.True(t, types.IsSkipRetryError(apiErr), "部分结算后不得允许换渠道重试")
	assert.Nil(t, usage, "异常结束仍不向调用方返回 usage")

	entry := latestConsumeLog(t)
	assert.True(t, entry.IsStream)
	assert.Equal(t, 100, entry.PromptTokens, "估算 prompt 应取请求期估算值")
	assert.Greater(t, entry.CompletionTokens, 0, "已收文本应产生 completion 估算")
	assert.Greater(t, entry.Quota, 0, "异常结束必须产生实际计费")
	assert.Contains(t, entry.Other, "partial_settled")
	assert.Contains(t, entry.Other, "timeout")

	var updated model.User
	require.NoError(t, model.DB.First(&updated, user.Id).Error)
	assert.Equal(t, 1_000_000-entry.Quota, updated.Quota, "钱包实际扣减应与日志一致")
}

// 末帧带 usage 的异常结束：按上游 usage 结算，不得退回本地估算。
func TestOaiStreamHandler_AbnormalEndPrefersUpstreamUsage(t *testing.T) {
	setupOpenAIStreamSettleDB(t)
	user := createOpenAIStreamSettleUser(t, 1_000_000)

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 1
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	usageChunk := "data: {\"id\":\"chatcmpl_1\",\"object\":\"chat.completion.chunk\",\"choices\":[],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":9,\"total_tokens\":14}}\n\n"
	c, resp, info := newAbnormalSettleStreamTest(t, &oneChunkThenBlockReader{data: usageChunk}, user)

	_, apiErr := OaiStreamHandler(c, info, resp)
	require.NotNil(t, apiErr)

	entry := latestConsumeLog(t)
	assert.Equal(t, 5, entry.PromptTokens, "上游 usage 优先于请求期估算（100）")
	assert.Equal(t, 9, entry.CompletionTokens)
}

// 客户端断开（client_gone）与超时共用同一条守卫：同样必须部分结算。
func TestOaiStreamHandler_ClientGoneSettlesPartialUsage(t *testing.T) {
	setupOpenAIStreamSettleDB(t)
	user := createOpenAIStreamSettleUser(t, 1_000_000)

	c, resp, info := newAbnormalSettleStreamTest(t, &oneChunkThenBlockReader{data: abnormalSettleContentChunk}, user)
	// 请求上下文已取消：扫描器主循环立即判定 client_gone（body 永不 EOF，结果确定）。
	reqCtx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Request = c.Request.WithContext(reqCtx)

	_, apiErr := OaiStreamHandler(c, info, resp)

	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeStreamClientClosed, apiErr.GetErrorCode())
	assert.Equal(t, statusClientClosedRequest, apiErr.StatusCode)
	assert.True(t, types.IsSkipRetryError(apiErr))

	entry := latestConsumeLog(t)
	assert.Greater(t, entry.Quota, 0)
	assert.Contains(t, entry.Other, "client_gone")
}

// 零证据（CountToken 关闭 → prompt 估算为 0、无已收文本）：等价旧行为全额退，
// 只留一条 quota=0 的消费日志，钱包一分不动。
func TestOaiStreamHandler_ZeroEvidenceSettlesZeroQuota(t *testing.T) {
	setupOpenAIStreamSettleDB(t)
	user := createOpenAIStreamSettleUser(t, 1_000_000)

	c, resp, info := newAbnormalSettleStreamTest(t, errReader{}, user)
	info.SetEstimatePromptTokens(0) // CountToken 关闭时请求期估算为 0

	_, apiErr := OaiStreamHandler(c, info, resp)
	require.NotNil(t, apiErr)

	entry := latestConsumeLog(t)
	assert.Equal(t, 0, entry.Quota)
	assert.Contains(t, entry.Other, "partial_settled")

	var updated model.User
	require.NoError(t, model.DB.First(&updated, user.Id).Error)
	assert.Equal(t, 1_000_000, updated.Quota, "零证据时不得扣费")
}

// 正常结束（只有 [DONE] 的空流）：handler 不结算、不写消费日志，
// 计费仍由调用方按返回的 usage 处理（本改动不得触碰正常路径）。
func TestOaiStreamHandler_NormalEmptyStreamDoesNotSettle(t *testing.T) {
	setupOpenAIStreamSettleDB(t)
	user := createOpenAIStreamSettleUser(t, 1_000_000)

	c, resp, info := newAbnormalSettleStreamTest(t, io.NopCloser(strings.NewReader("data: [DONE]\n\n")), user)

	usage, apiErr := OaiStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 100, usage.PromptTokens, "正常路径仍按请求期估算返回 usage")
	assert.Equal(t, int64(0), consumeLogCount(t), "正常结束由调用方结算，handler 不写消费日志")
}
