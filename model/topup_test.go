package model

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/require"
)

func TestTopupAmountToQuotaDecimalRMB(t *testing.T) {
	// 默认展示类型(USD/CNY):10 元按系统汇率 7.3 折算 → 10/7.3 × 500,000 ≈ 684,931
	// (元不再被当成美元,额度不再多给汇率倍)
	original := operation_setting.USDExchangeRate
	t.Cleanup(func() { operation_setting.USDExchangeRate = original })
	operation_setting.USDExchangeRate = 7.3

	got := topupAmountToQuotaDecimal(10)
	require.Equal(t, int64(684931), got.IntPart())

	// 100 元 → 100/7.3 × 500,000 ≈ 6,849,315
	got = topupAmountToQuotaDecimal(100)
	require.Equal(t, int64(6849315), got.IntPart())
}

func TestTopupAmountToQuotaDecimalExchangeRateFallback(t *testing.T) {
	// 汇率非法(0)时回退 1,保持旧行为:10 元 → 10 × 500,000
	original := operation_setting.USDExchangeRate
	t.Cleanup(func() { operation_setting.USDExchangeRate = original })
	operation_setting.USDExchangeRate = 0

	require.Equal(t, int64(5000000), topupAmountToQuotaDecimal(10).IntPart())
}
