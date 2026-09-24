package helper

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ModelPriceHelper 应把预扣计算中间值填入 PreConsumeDetail：
// 预扣 tokens = max(估算 prompt, 保底) + 请求 max_tokens；预扣额度 = tokens × 模型倍率 × 分组倍率。
func TestModelPriceHelperFillsPreConsumeDetail(t *testing.T) {
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"detail-test-model": 0.88}`), "set model ratio")
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
	require.NoError(t, err, "ModelPriceHelper")

	d := priceData.PreConsumeDetail
	require.NotNil(t, d, "expect PreConsumeDetail filled")
	// max(55, 500) + 100 = 600 tokens; 600 × 0.88 × 1 = 528
	assert.EqualValues(t, 55, d.EstimatedPromptTokens)
	assert.EqualValues(t, 500, d.FloorTokens)
	assert.EqualValues(t, 100, d.RequestMaxTokens)
	assert.EqualValues(t, 600, d.PreConsumedTokens)
	assert.InDelta(t, 0.88, d.ModelRatio, 1e-9)
	assert.EqualValues(t, 1, d.GroupRatio)
	assert.EqualValues(t, 528, d.Quota, "want 528")
	assert.EqualValues(t, priceData.QuotaToPreConsume, d.Quota, "want d.Quota == QuotaToPreConsume")
}
