package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRelayInfo returns a RelayInfo with non-zero timestamps and an
// initialized ChannelMeta; the zero time.Time crashes time.Time.UnixMilli
// inside GenerateTextOtherInfo, and the embedded *ChannelMeta is dereferenced
// for the model-mapping fields.
func testRelayInfo() *relaycommon.RelayInfo {
	now := time.Now()
	return &relaycommon.RelayInfo{StartTime: now, FirstResponseTime: now, ChannelMeta: &relaycommon.ChannelMeta{}}
}

// TestGenerateTextOtherInfoPluginMarkers verifies the plugin loop's
// observability markers land at the top level of the consume log's other map
// (non-sensitive slugs/counts, so not nested under admin_info).
func TestGenerateTextOtherInfoPluginMarkers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	common.SetContextKey(ctx, constant.ContextKeyPluginSlugs, "search,weather")
	common.SetContextKey(ctx, constant.ContextKeyPluginToolCalls, 3)

	other := GenerateTextOtherInfo(ctx, testRelayInfo(), 1, 1, 1, 0, 1, 0, 1)
	require.Equal(t, "search,weather", other["plugin_slugs"])
	require.Equal(t, 3, other["plugin_tool_calls"])
}

// TestGenerateTextOtherInfoNoPluginMarkers verifies non-plugin requests do
// not gain the keys (a zero round-0 count is still recorded when the loop set
// the key, but ordinary relay traffic never sets it).
func TestGenerateTextOtherInfoNoPluginMarkers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)

	other := GenerateTextOtherInfo(ctx, testRelayInfo(), 1, 1, 1, 0, 1, 0, 1)
	_, ok := other["plugin_slugs"]
	assert.False(t, ok)
	_, ok = other["plugin_tool_calls"]
	assert.False(t, ok)
}

// TestGenerateTextOtherInfoMarksAbnormalStreamSettle 锁定对账标记：异常结束但
// 仍结算的请求，消费日志的 stream_status 必须带 partial_settled，普通流式不写。
func TestGenerateTextOtherInfoMarksAbnormalStreamSettle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)

	abnormal := testRelayInfo()
	abnormal.IsStream = true
	abnormal.StreamStatus = relaycommon.NewStreamStatus()
	abnormal.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, nil)
	abnormal.AbnormalStreamSettled = true

	other := GenerateTextOtherInfo(ctx, abnormal, 1, 1, 1, 0, 1, 0, 1)
	streamStatus, ok := other["stream_status"].(map[string]interface{})
	require.True(t, ok, "stream_status 应存在")
	assert.Equal(t, "error", streamStatus["status"])
	assert.Equal(t, "client_gone", streamStatus["end_reason"])
	assert.Equal(t, true, streamStatus["partial_settled"])

	normal := testRelayInfo()
	normal.IsStream = true
	normal.StreamStatus = relaycommon.NewStreamStatus()
	normal.StreamStatus.SetEndReason(relaycommon.StreamEndReasonDone, nil)

	normalOther := GenerateTextOtherInfo(ctx, normal, 1, 1, 1, 0, 1, 0, 1)
	normalStatus, ok := normalOther["stream_status"].(map[string]interface{})
	require.True(t, ok, "stream_status 应存在")
	_, hasPartial := normalStatus["partial_settled"]
	assert.False(t, hasPartial, "正常结束不应带 partial_settled")
}
