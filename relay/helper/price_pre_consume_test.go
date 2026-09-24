package helper

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

// ModelPriceHelper 应把预扣计算中间值填入 PreConsumeDetail：
// 预扣 tokens = max(估算 prompt, 保底) + 请求 max_tokens；预扣额度 = tokens × 模型倍率 × 分组倍率。
func TestModelPriceHelperFillsPreConsumeDetail(t *testing.T) {
	if err := ratio_setting.UpdateModelRatioByJSONString(`{"detail-test-model": 0.88}`); err != nil {
		t.Fatalf("set model ratio: %v", err)
	}
	defer func() { _ = ratio_setting.UpdateModelRatioByJSONString("{}") }()

	prevFloor := common.PreConsumedQuota
	common.PreConsumedQuota = 500
	defer func() { common.PreConsumedQuota = prevFloor }()

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		OriginModelName: "detail-test-model",
		UserGroup:       "default",
		UsingGroup:      "default",
	}
	meta := &types.TokenCountMeta{MaxTokens: 100}

	priceData, err := ModelPriceHelper(ctx, info, 55, meta)
	if err != nil {
		t.Fatalf("ModelPriceHelper: %v", err)
	}

	d := priceData.PreConsumeDetail
	if d == nil {
		t.Fatalf("expect PreConsumeDetail filled, got nil")
	}
	// max(55, 500) + 100 = 600 tokens; 600 × 0.88 × 1 = 528
	if d.EstimatedPromptTokens != 55 {
		t.Errorf("EstimatedPromptTokens: want 55, got %d", d.EstimatedPromptTokens)
	}
	if d.FloorTokens != 500 {
		t.Errorf("FloorTokens: want 500, got %d", d.FloorTokens)
	}
	if d.RequestMaxTokens != 100 {
		t.Errorf("RequestMaxTokens: want 100, got %d", d.RequestMaxTokens)
	}
	if d.PreConsumedTokens != 600 {
		t.Errorf("PreConsumedTokens: want 600, got %d", d.PreConsumedTokens)
	}
	if d.ModelRatio != 0.88 || d.GroupRatio != 1 {
		t.Errorf("ratios: want 0.88/1, got %v/%v", d.ModelRatio, d.GroupRatio)
	}
	if d.Quota != 528 || d.Quota != priceData.QuotaToPreConsume {
		t.Errorf("Quota: want 528 (== QuotaToPreConsume), got %d (QuotaToPreConsume=%d)", d.Quota, priceData.QuotaToPreConsume)
	}
}
