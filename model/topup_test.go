package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/require"
)

func TestTopupAmountToQuotaDecimalByDisplayType(t *testing.T) {
	origType := operation_setting.GetQuotaDisplayType()
	origPrice := operation_setting.Price
	origQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		operation_setting.SetQuotaDisplayTypeForTest(origType)
		operation_setting.Price = origPrice
		common.QuotaPerUnit = origQuotaPerUnit
	})
	operation_setting.Price = 7.3
	common.QuotaPerUnit = 500000

	// USD/CUSTOM 模式:金额单位是美元 → 2 × 500,000(汇率参数不影响)
	operation_setting.SetQuotaDisplayTypeForTest(operation_setting.QuotaDisplayTypeUSD)
	require.Equal(t, int64(1000000), topupAmountToQuotaDecimal(2, 7.3).IntPart())

	// CNY 模式:金额单位是元 → 2 ÷ 7.3 × 500,000 ≈ 136,986
	operation_setting.SetQuotaDisplayTypeForTest(operation_setting.QuotaDisplayTypeCNY)
	require.Equal(t, int64(136986), topupAmountToQuotaDecimal(2, 7.3).IntPart())

	// CNY 模式汇率快照优先于全局 Price:订单快照 7.5 → 2 ÷ 7.5 × 500,000 ≈ 133,333
	operation_setting.Price = 7.3
	require.Equal(t, int64(133333), topupAmountToQuotaDecimal(2, 7.5).IntPart())

	// CNY 模式汇率非法(0/NaN)时回退 1,保持旧行为
	operation_setting.Price = 0
	require.Equal(t, int64(1000000), topupAmountToQuotaDecimal(2, 0).IntPart())
}

func TestTopUpRateSnapshotFallback(t *testing.T) {
	origPrice := operation_setting.Price
	t.Cleanup(func() { operation_setting.Price = origPrice })
	operation_setting.Price = 7.3

	// 无快照(旧订单/USD 模式)→ 回退当前 Price
	require.Equal(t, 7.3, topUpRate(&TopUp{}))

	// 有快照 → 用快照
	require.Equal(t, 7.5, topUpRate(&TopUp{Rate: 7.5}))
}
