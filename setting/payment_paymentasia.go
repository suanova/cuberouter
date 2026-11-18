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
package setting

import "strings"

const (
	PaymentAsiaEnvironmentLive    = "live"
	PaymentAsiaEnvironmentSandbox = "sandbox"
)

var (
	PaymentAsiaSandbox              bool
	PaymentAsiaMerchantToken        string
	PaymentAsiaSecret               string
	PaymentAsiaSandboxMerchantToken string
	PaymentAsiaSandboxSecret        string
	PaymentAsiaNotifyUrl            string
	PaymentAsiaReturnUrl            string
	PaymentAsiaCurrency             string  = "HKD"
	PaymentAsiaUnitPrice            float64 = 1.0
	PaymentAsiaMinTopUp             int     = 1
)

// GetPaymentAsiaCredentials returns the merchant token and secret of the
// active environment, verbatim (no trimming). Sandbox mode never falls back
// to the live pair: empty sandbox credentials stay empty.
func GetPaymentAsiaCredentials() (merchantToken, secret string, sandbox bool) {
	if PaymentAsiaSandbox {
		return PaymentAsiaSandboxMerchantToken, PaymentAsiaSandboxSecret, true
	}
	return PaymentAsiaMerchantToken, PaymentAsiaSecret, false
}

// GetPaymentAsiaSecretForEnvironment returns the trimmed merchant secret of the
// named environment. An empty or unknown environment falls back to the
// currently active credential pair. ok is false when that secret is blank.
func GetPaymentAsiaSecretForEnvironment(environment string) (secret string, ok bool) {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case PaymentAsiaEnvironmentSandbox:
		secret = PaymentAsiaSandboxSecret
	case PaymentAsiaEnvironmentLive:
		secret = PaymentAsiaSecret
	default:
		_, secret, _ = GetPaymentAsiaCredentials()
	}
	secret = strings.TrimSpace(secret)
	return secret, secret != ""
}
