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

	// USD/CUSTOM 模式:金额单位是美元 → 2 × 500,000
	operation_setting.SetQuotaDisplayTypeForTest(operation_setting.QuotaDisplayTypeUSD)
	require.Equal(t, int64(1000000), topupAmountToQuotaDecimal(2).IntPart())

	// CNY 模式:金额单位是元 → 2 ÷ 7.3 × 500,000 ≈ 136,986
	operation_setting.SetQuotaDisplayTypeForTest(operation_setting.QuotaDisplayTypeCNY)
	require.Equal(t, int64(136986), topupAmountToQuotaDecimal(2).IntPart())

	// CNY 模式汇率非法(0)时回退 1,保持旧行为
	operation_setting.Price = 0
	require.Equal(t, int64(1000000), topupAmountToQuotaDecimal(2).IntPart())
}
