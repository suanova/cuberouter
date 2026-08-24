package openai

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
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

// 上游 body 读取失败（扫描器错误）时必须返回错误而不是成功 usage——计费被跳过。
func TestOaiStreamHandler_ScannerError_ReturnsErrorNoUsage(t *testing.T) {
	c, resp, info := setupOaiStreamTest(t, errReader{})

	usage, apiErr := OaiStreamHandler(c, info, resp)

	require.NotNil(t, apiErr, "扫描器错误应返回错误")
	assert.Equal(t, types.ErrorCodeStreamScannerFailed, apiErr.GetErrorCode())
	assert.Nil(t, usage, "扫描器错误不应返回 usage")
}

// 空 body（EOF 但无任何响应内容）必须视为不完整流返回错误，不按成功计费。
func TestOaiStreamHandler_EmptyBody_ReturnsStreamIncomplete(t *testing.T) {
	c, resp, info := setupOaiStreamTest(t, io.NopCloser(strings.NewReader("")))

	usage, apiErr := OaiStreamHandler(c, info, resp)

	require.NotNil(t, apiErr, "空 body 应返回错误")
	assert.Equal(t, types.ErrorCodeStreamIncomplete, apiErr.GetErrorCode())
	assert.Nil(t, usage, "空 body 不应返回 usage")
}

// 上游无任何数据且空闲超时：必须返回 stream_idle_timeout 错误，不得合成成功 usage。
func TestOaiStreamHandler_IdleTimeout_ReturnsErrorNoUsage(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 1
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	c, resp, info := setupOaiStreamTest(t, &blockReader{})

	usage, apiErr := OaiStreamHandler(c, info, resp)

	require.NotNil(t, apiErr, "空闲超时应返回错误")
	assert.Equal(t, types.ErrorCodeStreamIdleTimeout, apiErr.GetErrorCode())
	assert.Nil(t, usage, "空闲超时不应返回 usage（不得合成计费）")
}
