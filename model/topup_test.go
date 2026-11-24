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
	origCustomRate := operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate
	t.Cleanup(func() {
		operation_setting.SetQuotaDisplayTypeForTest(origType)
		operation_setting.Price = origPrice
		common.QuotaPerUnit = origQuotaPerUnit
		operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = origCustomRate
	})
	operation_setting.Price = 7.3
	common.QuotaPerUnit = 500000

	// USD 模式:金额单位是美元 → 2 × 500,000(汇率参数不影响)
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

	// CUSTOM 模式:金额单位是自定义货币 → 2 ÷ 7.3 × 500,000 ≈ 136,986
	operation_setting.SetQuotaDisplayTypeForTest(operation_setting.QuotaDisplayTypeCustom)
	operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = 7.3
	require.Equal(t, int64(136986), topupAmountToQuotaDecimal(2, 7.3).IntPart())

	// CUSTOM 模式订单快照优先于当前展示汇率 → 2 ÷ 7.8 × 500,000 ≈ 128,205
	require.Equal(t, int64(128205), topupAmountToQuotaDecimal(2, 7.8).IntPart())

	// CUSTOM 模式汇率非法时回退 1
	require.Equal(t, int64(1000000), topupAmountToQuotaDecimal(2, 0).IntPart())
}

func TestTopUpRateSnapshotFallback(t *testing.T) {
	origPrice := operation_setting.Price
	origType := operation_setting.GetQuotaDisplayType()
	origCustomRate := operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate
	t.Cleanup(func() {
		operation_setting.Price = origPrice
		operation_setting.SetQuotaDisplayTypeForTest(origType)
		operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = origCustomRate
	})
	operation_setting.Price = 7.3

	// 无快照(旧订单/USD 模式)→ 回退当前 Price
	require.Equal(t, 7.3, topUpRate(&TopUp{}))

	// 有快照 → 用快照
	require.Equal(t, 7.5, topUpRate(&TopUp{Rate: 7.5}))

	// CUSTOM 展示无快照 → 回退当前自定义货币汇率，而不是 Price
	operation_setting.SetQuotaDisplayTypeForTest(operation_setting.QuotaDisplayTypeCustom)
	operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = 7.8
	require.Equal(t, 7.8, topUpRate(&TopUp{}))
	require.Equal(t, 7.5, topUpRate(&TopUp{Rate: 7.5}))
}

func TestTopUpCreditedQuotaByProvider(t *testing.T) {
	origType := operation_setting.GetQuotaDisplayType()
	origPrice := operation_setting.Price
	origCustomRate := operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate
	origQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		operation_setting.SetQuotaDisplayTypeForTest(origType)
		operation_setting.Price = origPrice
		operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = origCustomRate
		common.QuotaPerUnit = origQuotaPerUnit
	})
	operation_setting.Price = 7.3
	operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = 7.3
	common.QuotaPerUnit = 500000

	cases := []struct {
		name        string
		displayType string
		topUp       TopUp
		want        int
	}{
		{"epay cny new order", operation_setting.QuotaDisplayTypeCNY, TopUp{PaymentProvider: PaymentProviderEpay, PaymentMethod: "payment", Amount: 10, Money: 10, Rate: 7.3}, 684932},
		{"epay cny migrated quota amount", operation_setting.QuotaDisplayTypeCNY, TopUp{PaymentProvider: PaymentProviderEpay, PaymentMethod: "payment", Amount: 684932, Money: 10}, 684932},
		{"epay migrated quota under usd display", operation_setting.QuotaDisplayTypeUSD, TopUp{PaymentProvider: PaymentProviderEpay, PaymentMethod: "payment", Amount: 684932, Money: 10}, 684932},
		{"alipay cny", operation_setting.QuotaDisplayTypeCNY, TopUp{PaymentProvider: PaymentProviderAlipay, PaymentMethod: PaymentMethodAlipay, Amount: 1, Money: 1}, 68493},
		{"stripe uses money", operation_setting.QuotaDisplayTypeUSD, TopUp{PaymentProvider: PaymentProviderStripe, Money: 2}, 1000000},
		{"creem amount is quota", operation_setting.QuotaDisplayTypeUSD, TopUp{PaymentProvider: PaymentProviderCreem, Amount: 12345}, 12345},
		{"waffo amount is usd", operation_setting.QuotaDisplayTypeUSD, TopUp{PaymentProvider: PaymentProviderWaffo, Amount: 2}, 1000000},
		{"waffo pancake amount is usd", operation_setting.QuotaDisplayTypeUSD, TopUp{PaymentProvider: PaymentProviderWaffoPancake, Amount: 2}, 1000000},
		{"paymentasia cny", operation_setting.QuotaDisplayTypeCNY, TopUp{PaymentProvider: PaymentProviderPaymentAsia, PaymentMethod: "paymentasia_alipayhk", Amount: 10, Money: 10.8, Rate: 7.3}, 684932},
		{"paymentasia usd", operation_setting.QuotaDisplayTypeUSD, TopUp{PaymentProvider: PaymentProviderPaymentAsia, PaymentMethod: "paymentasia_alipayhk", Amount: 2, Money: 15.6}, 1000000},
		{"epay custom new order", operation_setting.QuotaDisplayTypeCustom, TopUp{PaymentProvider: PaymentProviderEpay, PaymentMethod: "payment", Amount: 10, Money: 10, Rate: 7.3}, 684932},
		{"paymentasia custom legacy", operation_setting.QuotaDisplayTypeCustom, TopUp{PaymentProvider: PaymentProviderPaymentAsia, PaymentMethod: "paymentasia_alipayhk", Amount: 10, Money: 10.8, Rate: 7.3}, 684932},
		{"paymentasia custom fee-free", operation_setting.QuotaDisplayTypeCustom, TopUp{PaymentProvider: PaymentProviderPaymentAsia, PaymentMethod: "paymentasia_alipayhk", Amount: 100, Money: 100, Rate: 7.3}, 6849315},
		// 有手续费快照的订单按 NetQuota 报告实际到账额度。
		{"paymentasia net quota snapshot", operation_setting.QuotaDisplayTypeUSD, TopUp{PaymentProvider: PaymentProviderPaymentAsia, PaymentMethod: "paymentasia_alipayhk", Amount: 100, Money: 100, Fee: 1.6, FeeRate: 0.016, NetAmount: 98.4, NetQuota: 49200000}, 49200000},
		// 大额 USD 订单会超过单请求 int32 饱和边界（2,147,483,647），但钱包容许到
		// MaxWalletQuota；报表口径必须返回完整额度而不是被截断的饱和值。
		{"paymentasia large usd", operation_setting.QuotaDisplayTypeUSD, TopUp{PaymentProvider: PaymentProviderPaymentAsia, PaymentMethod: "paymentasia_alipayhk", Amount: 100000, Money: 780000}, 50000000000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			operation_setting.SetQuotaDisplayTypeForTest(tc.displayType)
			require.Equal(t, tc.want, topUpCreditedQuota(&tc.topUp))
		})
	}
}
