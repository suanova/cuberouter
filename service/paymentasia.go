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
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"sort"
	"strings"
)

const paymentAsiaMethodTypePrefix = "paymentasia_"

const (
	paymentAsiaLiveGatewayURL    = "https://payment.pa-sys.com/app/page/"
	paymentAsiaSandboxGatewayURL = "https://payment-sandbox.pa-sys.com/app/page/"
)

// PaymentAsiaChannel is the gateway channel behind a paymentasia_* PayMethods
// type. Wallet is only set for Alipay.
type PaymentAsiaChannel struct {
	Network string
	Wallet  string
}

var paymentAsiaChannels = map[string]PaymentAsiaChannel{
	"paymentasia_alipay":     {Network: "Alipay"},
	"paymentasia_alipaycn":   {Network: "Alipay", Wallet: "AlipayCN"},
	"paymentasia_alipayhk":   {Network: "Alipay", Wallet: "AlipayHK"},
	"paymentasia_wechat":     {Network: "Wechat"},
	"paymentasia_cup":        {Network: "CUP"},
	"paymentasia_creditcard": {Network: "CreditCard"},
	"paymentasia_fps":        {Network: "Fps"},
	"paymentasia_octopus":    {Network: "Octopus"},
	"paymentasia_payme":      {Network: "PayMe"},
	"paymentasia_generic":    {Network: "UserDefine"},
}

// paymentAsiaCallbackSignFields is the fixed datafeed signature scope from the
// integration guide. Callback fields outside this allowlist are never signed by
// the gateway, so including them would reject every real callback.
var paymentAsiaCallbackSignFields = []string{
	"amount",
	"currency",
	"request_reference",
	"merchant_reference",
	"status",
}

// IsPaymentAsiaType reports whether a PayMethods type lives in the PaymentAsia
// namespace. The prefix is reserved so a misspelled type is never handled as an
// Epay method.
func IsPaymentAsiaType(methodType string) bool {
	return strings.HasPrefix(methodType, paymentAsiaMethodTypePrefix)
}

// IsPaymentAsiaMethod reports whether methodType is a known PaymentAsia channel.
func IsPaymentAsiaMethod(methodType string) bool {
	_, ok := paymentAsiaChannels[methodType]
	return ok
}

// PaymentAsiaChannelByMethodType resolves the gateway channel of a PaymentAsia
// PayMethods type.
func PaymentAsiaChannelByMethodType(methodType string) (PaymentAsiaChannel, bool) {
	channel, ok := paymentAsiaChannels[methodType]
	return channel, ok
}

// encodePaymentAsiaQueryValue encodes a value exactly like PHP urlencode,
// which PaymentAsia uses through http_build_query: A-Z, a-z, 0-9, '-', '_' and
// '.' stay as-is, a space becomes '+', and every other byte is percent-encoded
// with uppercase hex. Go's url.Values.Encode leaves '~' unescaped, hence this
// local encoder.
func encodePaymentAsiaQueryValue(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for i := 0; i < len(value); i++ {
		char := value[i]
		switch {
		case char >= 'A' && char <= 'Z',
			char >= 'a' && char <= 'z',
			char >= '0' && char <= '9',
			char == '-', char == '_', char == '.':
			builder.WriteByte(char)
		case char == ' ':
			builder.WriteByte('+')
		default:
			const upperHex = "0123456789ABCDEF"
			builder.WriteByte('%')
			builder.WriteByte(upperHex[char>>4])
			builder.WriteByte(upperHex[char&0x0F])
		}
	}
	return builder.String()
}

// BuildPaymentAsiaSign returns the SHA-512 hex signature of fields: the "sign"
// key is ignored, the rest is sorted by key, encoded as a PHP query string,
// and the merchant secret is appended without a separator. The input map is
// not modified.
func BuildPaymentAsiaSign(fields map[string]string, secret string) string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		if key == "sign" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for i, key := range keys {
		if i > 0 {
			builder.WriteByte('&')
		}
		builder.WriteString(encodePaymentAsiaQueryValue(key))
		builder.WriteByte('=')
		builder.WriteString(encodePaymentAsiaQueryValue(fields[key]))
	}
	builder.WriteString(secret)

	sum := sha512.Sum512([]byte(builder.String()))
	return hex.EncodeToString(sum[:])
}

// VerifyPaymentAsiaCallbackSign verifies a datafeed signature. Only the
// documented callback fields participate, any field missing from that set
// fails verification, and the signature comparison is constant time.
func VerifyPaymentAsiaCallbackSign(fields map[string]string, secret string) bool {
	signedFields := make(map[string]string, len(paymentAsiaCallbackSignFields))
	for _, key := range paymentAsiaCallbackSignFields {
		value, ok := fields[key]
		if !ok {
			return false
		}
		signedFields[key] = value
	}

	providedSign := strings.ToLower(fields["sign"])
	expectedSign := BuildPaymentAsiaSign(signedFields, secret)
	return subtle.ConstantTimeCompare([]byte(providedSign), []byte(expectedSign)) == 1
}

// PaymentAsiaGatewayURL returns the hosted payment page URL for a merchant
// token. The generic page serves the UserDefine network.
func PaymentAsiaGatewayURL(sandbox, generic bool, merchantToken string) string {
	gatewayURL := paymentAsiaLiveGatewayURL
	if sandbox {
		gatewayURL = paymentAsiaSandboxGatewayURL
	}
	if generic {
		gatewayURL += "generic/"
	}
	return gatewayURL + merchantToken
}
