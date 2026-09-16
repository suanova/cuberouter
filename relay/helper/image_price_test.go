package helper

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedImagePrice 写入图片按张价格表。
// 档位价均为精确二进制小数(0.0625=1/16, 0.125=1/8),
// 与 QuotaPerUnit(500000) 相乘无浮点截断误差,断言可精确到整数 quota。
func seedImagePrice(t *testing.T) {
	t.Helper()
	require.NoError(t, ratio_setting.UpdateImagePriceByJSONString(`{
		"img-price-model": {"rows": [
			{"resolution": "1024x1024", "price": 0.0625},
			{"resolution": "1328x1328", "price": 0.125}
		]}
	}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateImagePriceByJSONString(""))
	})
}

func TestModelPriceHelperImagePriceTable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	seedImagePrice(t)

	newInfo := func(model string) (*gin.Context, *relaycommon.RelayInfo) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Set("group", "default")
		return ctx, &relaycommon.RelayInfo{
			OriginModelName: model,
			UserGroup:       "default",
			UsingGroup:      "default",
		}
	}

	t.Run("size and count ratios multiply anchor price", func(t *testing.T) {
		ctx, info := newInfo("img-price-model")
		meta := &types.TokenCountMeta{
			BillingRatios: map[string]float64{"n": 2},
			ImageSize:     "1024x1024",
		}
		priceData, err := ModelPriceHelper(ctx, info, 1000, meta)
		require.NoError(t, err)
		require.True(t, priceData.UsePrice)
		// 锚点 0.125 × 500000 × n=2 × size=0.5 = 62500
		require.Equal(t, 62500, priceData.QuotaToPreConsume)
		assert.Equal(t, float64(2), priceData.OtherRatios()["n"])
		assert.Equal(t, 0.5, priceData.OtherRatios()["size"])
		// 结算复用同一 PriceData:OtherRatios 与 info.PriceData 一致
		assert.Equal(t, priceData.OtherRatios(), info.PriceData.OtherRatios())
	})

	t.Run("anchor resolution row carries no size ratio", func(t *testing.T) {
		ctx, info := newInfo("img-price-model")
		meta := &types.TokenCountMeta{
			BillingRatios: map[string]float64{"n": 1},
			ImageSize:     "1328x1328",
		}
		priceData, err := ModelPriceHelper(ctx, info, 1000, meta)
		require.NoError(t, err)
		require.Equal(t, 62500, priceData.QuotaToPreConsume)
		assert.False(t, priceData.HasOtherRatio("size"))
	})

	t.Run("unknown resolution bills at anchor", func(t *testing.T) {
		ctx, info := newInfo("img-price-model")
		meta := &types.TokenCountMeta{
			BillingRatios: map[string]float64{"n": 2},
			ImageSize:     "2048x2048",
		}
		priceData, err := ModelPriceHelper(ctx, info, 1000, meta)
		require.NoError(t, err)
		// 锚点 0.125 × 500000 × n=2 = 125000
		require.Equal(t, 125000, priceData.QuotaToPreConsume)
		assert.False(t, priceData.HasOtherRatio("size"))
	})

	t.Run("normalized resolution matches table row", func(t *testing.T) {
		ctx, info := newInfo("img-price-model")
		meta := &types.TokenCountMeta{
			BillingRatios: map[string]float64{"n": 1},
			ImageSize:     " 1024*1024 ",
		}
		priceData, err := ModelPriceHelper(ctx, info, 1000, meta)
		require.NoError(t, err)
		require.Equal(t, 31250, priceData.QuotaToPreConsume)
		assert.Equal(t, 0.5, priceData.OtherRatios()["size"])
	})

	t.Run("legacy ImagePriceRatio is not stacked on the table", func(t *testing.T) {
		ctx, info := newInfo("img-price-model")
		meta := &types.TokenCountMeta{
			BillingRatios:   map[string]float64{"n": 1},
			ImageSize:       "1328x1328",
			ImagePriceRatio: 2,
		}
		priceData, err := ModelPriceHelper(ctx, info, 1000, meta)
		require.NoError(t, err)
		// 若 ImagePriceRatio 被叠加,结果会是 125000
		require.Equal(t, 62500, priceData.QuotaToPreConsume)
	})

	t.Run("empty size bills at anchor", func(t *testing.T) {
		ctx, info := newInfo("img-price-model")
		meta := &types.TokenCountMeta{
			BillingRatios: map[string]float64{"n": 1},
		}
		priceData, err := ModelPriceHelper(ctx, info, 1000, meta)
		require.NoError(t, err)
		require.Equal(t, 62500, priceData.QuotaToPreConsume)
		assert.False(t, priceData.HasOtherRatio("size"))
	})
}

func TestHasModelBillingConfigImagePriceTable(t *testing.T) {
	seedImagePrice(t)
	// 仅配置图片价格表(无 ModelPrice/ModelRatio)也算有计费配置
	assert.True(t, HasModelBillingConfig("img-price-model"))
	assert.False(t, HasModelBillingConfig("img-unknown-model"))
}
