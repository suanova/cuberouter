package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func estimateBreakdownTestContext(t *testing.T) *gin.Context {
	t.Helper()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "Minimax-M2.5")
	return ctx
}

// 估算应同时写入构成分解（文本分词 / 消息格式开销 / 工具开销 / 名称开销 / 基础开销），
// 且分解之和等于返回的 tokens 总数，供日志展示预扣估算的来龙去脉。
func TestEstimateRequestTokenBreakdown(t *testing.T) {
	prevCountToken := constant.CountToken
	constant.CountToken = true
	defer func() { constant.CountToken = prevCountToken }()

	ctx := estimateBreakdownTestContext(t)

	meta := &types.TokenCountMeta{
		TokenType:     types.TokenTypeTokenizer,
		CombineText:   "user\n用一句话介绍四季",
		MessagesCount: 1,
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
	info.RelayFormat = types.RelayFormatOpenAI

	total, err := EstimateRequestToken(ctx, meta, info)
	require.NoError(t, err)

	detail := GetTokenEstimateBreakdown(ctx)
	require.NotNil(t, detail, "expect breakdown captured in request context")
	assert.Equal(t, total, detail.Total)
	// OpenAI 格式：1 条消息 → 消息开销 3；无工具/名称；基础 3
	assert.Equal(t, 3, detail.MessagesOverhead)
	assert.Zero(t, detail.ToolsOverhead)
	assert.Zero(t, detail.NamesOverhead)
	assert.Equal(t, 3, detail.BaseOverhead)
	assert.Equal(t, total, detail.TextTokens+detail.MessagesOverhead+
		detail.ToolsOverhead+detail.NamesOverhead+detail.BaseOverhead+detail.MediaTokens)
	assert.Equal(t, 15, detail.Total, "want 15 (9 text + 3 msg + 3 base)")
}

// 估算分解是请求作用域的：其他请求（未估算）读不到、也不会读到别的请求的数据。
func TestTokenEstimateBreakdownIsRequestScoped(t *testing.T) {
	prevCountToken := constant.CountToken
	constant.CountToken = true
	defer func() { constant.CountToken = prevCountToken }()

	ctxA := estimateBreakdownTestContext(t)
	meta := &types.TokenCountMeta{
		TokenType:     types.TokenTypeTextNumber,
		CombineText:   "hello",
		MessagesCount: 1,
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	info.RelayFormat = types.RelayFormatOpenAI
	_, err := EstimateRequestToken(ctxA, meta, info)
	require.NoError(t, err)
	require.NotNil(t, GetTokenEstimateBreakdown(ctxA))

	// 未经过估算的请求 context 必须为 nil（不能复用其他请求的数据）
	ctxB := estimateBreakdownTestContext(t)
	assert.Nil(t, GetTokenEstimateBreakdown(ctxB))

	// CountToken 关闭时不写分解
	constant.CountToken = false
	ctxC := estimateBreakdownTestContext(t)
	_, err = EstimateRequestToken(ctxC, meta, info)
	require.NoError(t, err)
	assert.Nil(t, GetTokenEstimateBreakdown(ctxC))
}
