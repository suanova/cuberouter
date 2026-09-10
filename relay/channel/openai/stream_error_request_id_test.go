package openai

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestReturnOpenAIStreamErrorAppendsRequestIDIdempotently 锁定流式错误出口的
// request id 补齐:不完整流(502 stream_incomplete 等)可能经不经过
// controller.Relay defer 的桥出口直接序列化,此前报文体没有 request id;
// 补齐需幂等——重复经过出口或后续再走 Relay defer 都不产生重复后缀。
func TestReturnOpenAIStreamErrorAppendsRequestIDIdempotently(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(common.RequestIdKey, "req-20260909-abc")
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI}

	apiErr := incompleteOpenAIStreamError()
	_, _ = returnOpenAIStreamError(c, info, apiErr, false)
	assert.Contains(t, apiErr.Error(), "(request id: req-20260909-abc)")

	// 再次经过出口(等价于错误再经 Relay defer):后缀不重复。
	_, _ = returnOpenAIStreamError(c, info, apiErr, false)
	assert.Equal(t, 1, strings.Count(apiErr.Error(), "(request id: "))
}
