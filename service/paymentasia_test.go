/*
Copyright (C) 2023-2026 QuantumNous
Copyright (C) 2026 CubeRouter

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PaymentAsia Integration Guide v2.7.6 p.19 sample signature vector.
const (
	paymentAsiaGuideSecret = "127f7830-b856-4ddf-92b4-a6478e38547b"
	paymentAsiaGuideSign   = "a36026912f25eb4ef4ea23d6f9760fa0e664f7561fa993976b44b3036cd251ab53c6d7401e7b0f2f492867d4d4d59a0abee931bb9040ae8a5b0b0516a7f0923b"
)

func paymentAsiaGuideFields() map[string]string {
	return map[string]string{
		"merchant_reference":  "1234567890",
		"currency":            "HKD",
		"amount":              "100.00",
		"customer_ip":         "123.123.123.123",
		"customer_first_name": "John",
		"customer_last_name":  "Doe",
		"customer_address":    "1, Bay Street",
		"customer_phone":      "0123123123",
		"customer_email":      "someone@gmail.com",
		"customer_state":      "NY",
		"customer_country":    "US",
		"return_url":          "https://demo.shop.com/payment/return",
		"network":             "Alipay",
		// The guide's field array spells this "iPhoneX", but its own pre-sign
		// string (and the sample hash) use "IphoneX"; the pre-sign string wins.
		"subject": "IphoneX",
	}
}

func TestBuildPaymentAsiaSignMatchesGuideVector(t *testing.T) {
	fields := paymentAsiaGuideFields()

	assert.Equal(t, paymentAsiaGuideSign, BuildPaymentAsiaSign(fields, paymentAsiaGuideSecret))
}

func TestPaymentAsiaQueryEncodingMatchesPHPUrlencode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"spaces tilde star comma at", "a b~c*d,e@f", "a+b%7Ec%2Ad%2Ce%40f"},
		{"unreserved characters", "AZaz09-_.", "AZaz09-_."},
		{"address", "1, Bay Street", "1%2C+Bay+Street"},
		{"url", "https://demo.shop.com/payment/return", "https%3A%2F%2Fdemo.shop.com%2Fpayment%2Freturn"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, encodePaymentAsiaQueryValue(tt.input))
		})
	}
}

func TestPaymentAsiaSignIsOrderIndependent(t *testing.T) {
	ascending := map[string]string{
		"amount":             "10.00",
		"currency":           "HKD",
		"merchant_reference": "PA1NO123456",
		"network":            "Alipay",
	}
	descending := map[string]string{
		"network":            "Alipay",
		"merchant_reference": "PA1NO123456",
		"currency":           "HKD",
		"amount":             "10.00",
	}
	sign := BuildPaymentAsiaSign(ascending, "order-secret")
	assert.Equal(t, sign, BuildPaymentAsiaSign(descending, "order-secret"))

	// A caller-supplied "sign" never participates in signing, and the input
	// map must be left untouched.
	withSign := map[string]string{
		"amount":             "10.00",
		"currency":           "HKD",
		"merchant_reference": "PA1NO123456",
		"network":            "Alipay",
		"sign":               "stale-value",
	}
	assert.Equal(t, sign, BuildPaymentAsiaSign(withSign, "order-secret"))
	assert.Len(t, withSign, 5)
	assert.Equal(t, "stale-value", withSign["sign"])
}

func TestVerifyPaymentAsiaCallbackSignIgnoresUnknownFields(t *testing.T) {
	secret := "callback-secret"
	fields := map[string]string{
		"amount":             "100.00",
		"currency":           "HKD",
		"merchant_reference": "PA1NO123456",
		"request_reference":  "req-7f7d",
		"status":             "1",
	}
	fields["sign"] = BuildPaymentAsiaSign(fields, secret)

	require.True(t, VerifyPaymentAsiaCallbackSign(fields, secret))

	fields["notify_type"] = "datafeed"
	fields["provider_reference"] = "4200000693172837987150090397"
	assert.True(t, VerifyPaymentAsiaCallbackSign(fields, secret),
		"undocumented callback fields must not affect verification")

	fields["amount"] = "1.00"
	assert.False(t, VerifyPaymentAsiaCallbackSign(fields, secret),
		"tampered amount must fail verification")

	fields["amount"] = "100.00"
	fields["sign"] = strings.ToUpper(fields["sign"])
	assert.True(t, VerifyPaymentAsiaCallbackSign(fields, secret),
		"uppercase hex signature must verify")

	fields["sign"] = strings.Repeat("0", len(fields["sign"]))
	assert.False(t, VerifyPaymentAsiaCallbackSign(fields, secret),
		"tampered signature must fail verification")

	delete(fields, "request_reference")
	assert.False(t, VerifyPaymentAsiaCallbackSign(fields, secret),
		"missing signed field must fail verification")
}

func TestPaymentAsiaChannelMapping(t *testing.T) {
	tests := []struct {
		methodType string
		want       PaymentAsiaChannel
	}{
		{"paymentasia_alipay", PaymentAsiaChannel{Network: "Alipay"}},
		{"paymentasia_alipaycn", PaymentAsiaChannel{Network: "Alipay", Wallet: "AlipayCN"}},
		{"paymentasia_alipayhk", PaymentAsiaChannel{Network: "Alipay", Wallet: "AlipayHK"}},
		{"paymentasia_wechat", PaymentAsiaChannel{Network: "Wechat"}},
		{"paymentasia_cup", PaymentAsiaChannel{Network: "CUP"}},
		{"paymentasia_creditcard", PaymentAsiaChannel{Network: "CreditCard"}},
		{"paymentasia_fps", PaymentAsiaChannel{Network: "Fps"}},
		{"paymentasia_octopus", PaymentAsiaChannel{Network: "Octopus"}},
		{"paymentasia_payme", PaymentAsiaChannel{Network: "PayMe"}},
		{"paymentasia_generic", PaymentAsiaChannel{Network: "UserDefine"}},
	}
	for _, tt := range tests {
		t.Run(tt.methodType, func(t *testing.T) {
			channel, ok := PaymentAsiaChannelByMethodType(tt.methodType)
			require.True(t, ok)
			assert.Equal(t, tt.want, channel)
			assert.True(t, IsPaymentAsiaMethod(tt.methodType))
			assert.True(t, IsPaymentAsiaType(tt.methodType))
		})
	}

	for _, unknown := range []string{"", "alipay", "paymentasia", "paymentasia_typo", "paymentasia_"} {
		_, ok := PaymentAsiaChannelByMethodType(unknown)
		assert.False(t, ok, "unknown type %q must not resolve a channel", unknown)
		assert.False(t, IsPaymentAsiaMethod(unknown), "unknown type %q is not a PaymentAsia method", unknown)
	}

	// The namespace check stays broad so a misspelled type never falls into Epay.
	assert.True(t, IsPaymentAsiaType("paymentasia_typo"))
	assert.True(t, IsPaymentAsiaType("paymentasia_"))
	assert.False(t, IsPaymentAsiaType("paymentasia"))
	assert.False(t, IsPaymentAsiaType("alipay"))
	assert.False(t, IsPaymentAsiaType(""))
}

func TestPaymentAsiaGatewayURL(t *testing.T) {
	tests := []struct {
		name    string
		sandbox bool
		generic bool
		want    string
	}{
		{"live normal", false, false, "https://payment.pa-sys.com/app/page/TOKEN123"},
		{"live generic", false, true, "https://payment.pa-sys.com/app/page/generic/TOKEN123"},
		{"sandbox normal", true, false, "https://payment-sandbox.pa-sys.com/app/page/TOKEN123"},
		{"sandbox generic", true, true, "https://payment-sandbox.pa-sys.com/app/page/generic/TOKEN123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, PaymentAsiaGatewayURL(tt.sandbox, tt.generic, "TOKEN123"))
		})
	}
}
