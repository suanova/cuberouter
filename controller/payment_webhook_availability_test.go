package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestStripeWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	originalAPISecret := setting.StripeApiSecret
	originalWebhookSecret := setting.StripeWebhookSecret
	originalPriceID := setting.StripePriceId
	t.Cleanup(func() {
		setting.StripeApiSecret = originalAPISecret
		setting.StripeWebhookSecret = originalWebhookSecret
		setting.StripePriceId = originalPriceID
	})

	setting.StripeWebhookSecret = ""
	setting.StripeApiSecret = "sk_test_123"
	setting.StripePriceId = "price_123"
	require.False(t, isStripeWebhookEnabled())

	setting.StripeWebhookSecret = "whsec_test"
	require.True(t, isStripeWebhookEnabled())

	setting.StripePriceId = ""
	require.False(t, isStripeWebhookEnabled())
}

func TestCreemWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	originalAPIKey := setting.CreemApiKey
	originalProducts := setting.CreemProducts
	originalWebhookSecret := setting.CreemWebhookSecret
	t.Cleanup(func() {
		setting.CreemApiKey = originalAPIKey
		setting.CreemProducts = originalProducts
		setting.CreemWebhookSecret = originalWebhookSecret
	})

	setting.CreemWebhookSecret = ""
	setting.CreemApiKey = "creem_api_key"
	setting.CreemProducts = `[{"productId":"prod_123"}]`
	require.False(t, isCreemWebhookEnabled())

	setting.CreemWebhookSecret = "creem_secret"
	require.True(t, isCreemWebhookEnabled())

	setting.CreemProducts = "[]"
	require.False(t, isCreemWebhookEnabled())
}

func TestWaffoWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	originalEnabled := setting.WaffoEnabled
	originalSandbox := setting.WaffoSandbox
	originalAPIKey := setting.WaffoApiKey
	originalPrivateKey := setting.WaffoPrivateKey
	originalPublicCert := setting.WaffoPublicCert
	originalSandboxAPIKey := setting.WaffoSandboxApiKey
	originalSandboxPrivateKey := setting.WaffoSandboxPrivateKey
	originalSandboxPublicCert := setting.WaffoSandboxPublicCert
	t.Cleanup(func() {
		setting.WaffoEnabled = originalEnabled
		setting.WaffoSandbox = originalSandbox
		setting.WaffoApiKey = originalAPIKey
		setting.WaffoPrivateKey = originalPrivateKey
		setting.WaffoPublicCert = originalPublicCert
		setting.WaffoSandboxApiKey = originalSandboxAPIKey
		setting.WaffoSandboxPrivateKey = originalSandboxPrivateKey
		setting.WaffoSandboxPublicCert = originalSandboxPublicCert
	})

	setting.WaffoEnabled = true
	setting.WaffoSandbox = false
	setting.WaffoApiKey = ""
	setting.WaffoPrivateKey = "private"
	setting.WaffoPublicCert = "public"
	require.False(t, isWaffoWebhookEnabled())

	setting.WaffoApiKey = "api"
	require.True(t, isWaffoWebhookEnabled())

	setting.WaffoEnabled = false
	require.False(t, isWaffoWebhookEnabled())

	setting.WaffoEnabled = true
	setting.WaffoSandbox = true
	setting.WaffoSandboxApiKey = ""
	setting.WaffoSandboxPrivateKey = "sandbox_private"
	setting.WaffoSandboxPublicCert = "sandbox_public"
	require.False(t, isWaffoWebhookEnabled())

	setting.WaffoSandboxApiKey = "sandbox_api"
	require.True(t, isWaffoWebhookEnabled())
}

func TestWaffoPancakeWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	originalMerchantID := setting.WaffoPancakeMerchantID
	originalPrivateKey := setting.WaffoPancakePrivateKey
	originalProductID := setting.WaffoPancakeProductID
	t.Cleanup(func() {
		setting.WaffoPancakeMerchantID = originalMerchantID
		setting.WaffoPancakePrivateKey = originalPrivateKey
		setting.WaffoPancakeProductID = originalProductID
	})

	// Presence of all three credentials enables the gateway. Webhook public
	// keys are bundled in the SDK and there is no separate Enabled toggle —
	// clear any of the three fields to disable.
	setting.WaffoPancakeMerchantID = ""
	setting.WaffoPancakePrivateKey = "private"
	setting.WaffoPancakeProductID = "product"
	require.False(t, isWaffoPancakeWebhookEnabled())

	setting.WaffoPancakeMerchantID = "merchant"
	require.True(t, isWaffoPancakeWebhookEnabled())

	setting.WaffoPancakeProductID = ""
	require.False(t, isWaffoPancakeWebhookEnabled())

	setting.WaffoPancakeProductID = "product"
	setting.WaffoPancakePrivateKey = ""
	require.False(t, isWaffoPancakeWebhookEnabled())
}

func TestEpayWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	originalPayAddress := operation_setting.PayAddress
	originalEpayID := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	originalPayMethods := operation_setting.PayMethods
	t.Cleanup(func() {
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		operation_setting.PayMethods = originalPayMethods
	})

	operation_setting.PayAddress = "https://pay.example.com"
	operation_setting.EpayId = "epay_id"
	operation_setting.EpayKey = ""
	operation_setting.PayMethods = []map[string]string{{"type": "alipay"}}
	require.False(t, isEpayWebhookEnabled())

	operation_setting.EpayKey = "epay_key"
	require.True(t, isEpayWebhookEnabled())

	operation_setting.PayMethods = nil
	require.False(t, isEpayWebhookEnabled())
}

func TestPaymentAsiaWebhookEnabledRequiresActiveEnvironmentCredentials(t *testing.T) {
	originalSandbox := setting.PaymentAsiaSandbox
	originalMerchantToken := setting.PaymentAsiaMerchantToken
	originalSecret := setting.PaymentAsiaSecret
	originalSandboxMerchantToken := setting.PaymentAsiaSandboxMerchantToken
	originalSandboxSecret := setting.PaymentAsiaSandboxSecret
	t.Cleanup(func() {
		setting.PaymentAsiaSandbox = originalSandbox
		setting.PaymentAsiaMerchantToken = originalMerchantToken
		setting.PaymentAsiaSecret = originalSecret
		setting.PaymentAsiaSandboxMerchantToken = originalSandboxMerchantToken
		setting.PaymentAsiaSandboxSecret = originalSandboxSecret
	})

	setting.PaymentAsiaSandbox = false
	setting.PaymentAsiaMerchantToken = ""
	setting.PaymentAsiaSecret = ""
	setting.PaymentAsiaSandboxMerchantToken = ""
	setting.PaymentAsiaSandboxSecret = ""
	require.False(t, isPaymentAsiaTopUpEnabled())
	require.False(t, isPaymentAsiaWebhookEnabled())

	setting.PaymentAsiaMerchantToken = "live_token"
	require.False(t, isPaymentAsiaTopUpEnabled())
	require.False(t, isPaymentAsiaWebhookEnabled())

	setting.PaymentAsiaSecret = "live_secret"
	require.True(t, isPaymentAsiaTopUpEnabled())
	require.True(t, isPaymentAsiaWebhookEnabled())

	// 切到沙箱后只看沙箱凭据，生产凭据不兜底
	setting.PaymentAsiaSandbox = true
	require.False(t, isPaymentAsiaTopUpEnabled())
	require.False(t, isPaymentAsiaWebhookEnabled())

	setting.PaymentAsiaSandboxMerchantToken = "sandbox_token"
	setting.PaymentAsiaSandboxSecret = "sandbox_secret"
	require.True(t, isPaymentAsiaTopUpEnabled())
	require.True(t, isPaymentAsiaWebhookEnabled())

	setting.PaymentAsiaSandboxSecret = "   "
	require.False(t, isPaymentAsiaTopUpEnabled())
	require.False(t, isPaymentAsiaWebhookEnabled())
}

func TestGetPaymentAsiaCredentialsReturnsActiveEnvironmentPairVerbatim(t *testing.T) {
	originalSandbox := setting.PaymentAsiaSandbox
	originalMerchantToken := setting.PaymentAsiaMerchantToken
	originalSecret := setting.PaymentAsiaSecret
	originalSandboxMerchantToken := setting.PaymentAsiaSandboxMerchantToken
	originalSandboxSecret := setting.PaymentAsiaSandboxSecret
	t.Cleanup(func() {
		setting.PaymentAsiaSandbox = originalSandbox
		setting.PaymentAsiaMerchantToken = originalMerchantToken
		setting.PaymentAsiaSecret = originalSecret
		setting.PaymentAsiaSandboxMerchantToken = originalSandboxMerchantToken
		setting.PaymentAsiaSandboxSecret = originalSandboxSecret
	})

	setting.PaymentAsiaSandbox = false
	setting.PaymentAsiaMerchantToken = " live_token "
	setting.PaymentAsiaSecret = "live_secret"
	setting.PaymentAsiaSandboxMerchantToken = "sandbox_token"
	setting.PaymentAsiaSandboxSecret = "sandbox_secret"

	merchantToken, secret, sandbox := setting.GetPaymentAsiaCredentials()
	require.False(t, sandbox)
	require.Equal(t, " live_token ", merchantToken)
	require.Equal(t, "live_secret", secret)

	// 沙箱凭据为空时原样返回空值，不回退到生产凭据
	setting.PaymentAsiaSandbox = true
	setting.PaymentAsiaSandboxMerchantToken = ""
	setting.PaymentAsiaSandboxSecret = ""
	merchantToken, secret, sandbox = setting.GetPaymentAsiaCredentials()
	require.True(t, sandbox)
	require.Empty(t, merchantToken)
	require.Empty(t, secret)
}

func TestAlipayWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	originalAppID := setting.AlipayAppId
	originalPrivateKey := setting.AlipayPrivateKey
	originalPublicKey := setting.AlipayPublicKey
	originalSandbox := setting.AlipaySandboxEnabled
	t.Cleanup(func() {
		setting.AlipayAppId = originalAppID
		setting.AlipayPrivateKey = originalPrivateKey
		setting.AlipayPublicKey = originalPublicKey
		setting.AlipaySandboxEnabled = originalSandbox
	})

	setting.AlipayAppId = ""
	setting.AlipayPrivateKey = ""
	setting.AlipayPublicKey = ""
	require.False(t, isAlipayTopUpEnabled())
	require.False(t, isAlipayWebhookEnabled())

	setting.AlipayAppId = "app_id"
	setting.AlipayPrivateKey = "private_key"
	require.False(t, isAlipayTopUpEnabled())
	require.False(t, isAlipayWebhookEnabled())

	setting.AlipayPublicKey = "public_key"
	require.True(t, isAlipayTopUpEnabled())
	require.True(t, isAlipayWebhookEnabled())

	// 沙箱开关不影响启用判定，只决定 SDK 连沙箱还是正式网关
	setting.AlipaySandboxEnabled = true
	require.True(t, isAlipayTopUpEnabled())
	require.True(t, isAlipayWebhookEnabled())

	setting.AlipayPrivateKey = ""
	require.False(t, isAlipayTopUpEnabled())
	require.False(t, isAlipayWebhookEnabled())
}
