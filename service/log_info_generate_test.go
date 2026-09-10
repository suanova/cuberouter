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
	snap := other.Snapshot()
	require.Equal(t, "search,weather", snap["plugin_slugs"])
	require.Equal(t, 3, snap["plugin_tool_calls"])
}

// TestGenerateTextOtherInfoNoPluginMarkers verifies non-plugin requests do
// not gain the keys (a zero round-0 count is still recorded when the loop set
// the key, but ordinary relay traffic never sets it).
func TestGenerateTextOtherInfoNoPluginMarkers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)

	other := GenerateTextOtherInfo(ctx, testRelayInfo(), 1, 1, 1, 0, 1, 0, 1)
	snap := other.Snapshot()
	_, ok := snap["plugin_slugs"]
	assert.False(t, ok)
	_, ok = snap["plugin_tool_calls"]
	assert.False(t, ok)
}
