package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

// 估算应同时返回构成分解（文本分词 / 消息格式开销 / 工具开销 / 名称开销 / 基础开销），
// 且分解之和等于返回的 tokens 总数，供日志展示预扣估算的来龙去脉。
func TestEstimateRequestTokenBreakdown(t *testing.T) {
	prevCountToken := constant.CountToken
	constant.CountToken = true
	defer func() { constant.CountToken = prevCountToken }()

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "Minimax-M2.5")

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
	if err != nil {
		t.Fatalf("EstimateRequestToken: %v", err)
	}

	detail := GetLastTokenEstimateBreakdown()
	if detail == nil {
		t.Fatalf("expect breakdown captured, got nil")
	}
	if detail.Total != total {
		t.Fatalf("breakdown total %d != returned %d", detail.Total, total)
	}
	// OpenAI 格式：1 条消息 → 消息开销 3；无工具/名称；基础 3
	if detail.MessagesOverhead != 3 {
		t.Errorf("MessagesOverhead: want 3, got %d", detail.MessagesOverhead)
	}
	if detail.ToolsOverhead != 0 || detail.NamesOverhead != 0 {
		t.Errorf("ToolsOverhead/NamesOverhead: want 0/0, got %d/%d", detail.ToolsOverhead, detail.NamesOverhead)
	}
	if detail.BaseOverhead != 3 {
		t.Errorf("BaseOverhead: want 3, got %d", detail.BaseOverhead)
	}
	if detail.TextTokens+detail.MessagesOverhead+detail.ToolsOverhead+detail.NamesOverhead+detail.BaseOverhead+detail.MediaTokens != total {
		t.Errorf("parts (%d+%d+%d+%d+%d+%d) != total %d",
			detail.TextTokens, detail.MessagesOverhead, detail.ToolsOverhead,
			detail.NamesOverhead, detail.BaseOverhead, detail.MediaTokens, total)
	}
	if detail.Total != 15 {
		t.Errorf("Total: want 15 (9 text + 3 msg + 3 base), got %d", detail.Total)
	}
}
