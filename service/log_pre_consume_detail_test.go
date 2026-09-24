package service

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GenerateTextOtherInfo 应把 PriceData 中的预扣计算明细（PreConsumeDetail）
// 顶层输出到 other["pre_consume_detail"]，让消费日志可追溯预扣值的计算过程。
func TestGenerateTextOtherInfoOutputsPreConsumeDetail(t *testing.T) {
	// ChannelMeta 以指针内嵌，提升字段（IsModelMapped 等）在 nil 时会 panic，必须初始化
	relayInfo := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	relayInfo.PriceData = types.PriceData{
		PreConsumeDetail: &types.PreConsumeDetail{
			EstimatedPromptTokens: 15,
			FloorTokens:           0,
			RequestMaxTokens:      0,
			PreConsumedTokens:     15,
			ModelRatio:            0.88,
			GroupRatio:            1,
			UsePrice:              false,
			Quota:                 13,
		},
	}

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	other := GenerateTextOtherInfo(ctx, relayInfo, 0.88, 1, 3.99, 0, 0.1, -1, -1)

	detail, ok := other["pre_consume_detail"].(*types.PreConsumeDetail)
	require.True(t, ok, "expect other[\"pre_consume_detail\"] to be *types.PreConsumeDetail, got %T", other["pre_consume_detail"])
	assert.EqualValues(t, 15, detail.EstimatedPromptTokens)
	assert.EqualValues(t, 15, detail.PreConsumedTokens)
	assert.EqualValues(t, 13, detail.Quota)
	assert.InDelta(t, 0.88, detail.ModelRatio, 1e-9)
	assert.EqualValues(t, 1, detail.GroupRatio)
}

// 未填充明细（旧数据/免费模型路径未构造 detail）时不应输出该键。
func TestGenerateTextOtherInfoOmitsNilPreConsumeDetail(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	other := GenerateTextOtherInfo(ctx, relayInfo, 0.88, 1, 3.99, 0, 0.1, -1, -1)

	_, ok := other["pre_consume_detail"]
	assert.False(t, ok, "expect no pre_consume_detail key when detail is nil")
}
