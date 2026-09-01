/**
此文件为旧版支付设置文件，如需增加新的参数、变量等，请在 payment_setting.go 中添加
This file is the old version of the payment settings file. If you need to add new parameters, variables, etc., please add them in payment_setting.go
*/

package operation_setting

import (
	"github.com/QuantumNous/new-api/common"
)

var PayAddress = ""
var CustomCallbackAddress = ""
var EpayId = ""
var EpayKey = ""
var Price = 7.3
var MinTopUp = 1
var USDExchangeRate = 7.3

// 各支付网关的独立支付汇率(USD→本地货币)。0 表示未单独配置,回退全局 Price。
// 与 Price 的区别:Price 还用于定价展示与渠道余额换算,这两个字段只影响
// 对应网关的支付金额。
var EpayRate = 0.0
var AlipayRate = 0.0

// GetEpayRate 返回 Epay 支付汇率;未单独配置时回退全局 Price。
func GetEpayRate() float64 {
	if EpayRate > 0 {
		return EpayRate
	}
	return Price
}

// GetAlipayRate 返回支付宝支付汇率;未单独配置时回退全局 Price。
func GetAlipayRate() float64 {
	if AlipayRate > 0 {
		return AlipayRate
	}
	return Price
}

var PayMethods = []map[string]string{
	{
		"name": "支付宝",
		"icon": "SiAlipay",
		"type": "alipay",
	},
	{
		"name": "微信",
		"icon": "SiWechat",
		"type": "wxpay",
	},
	{
		"name":      "自定义1",
		"icon":      "LuCreditCard",
		"type":      "custom1",
		"min_topup": "50",
	},
}

func UpdatePayMethodsByJsonString(jsonString string) error {
	PayMethods = make([]map[string]string, 0)
	return common.Unmarshal([]byte(jsonString), &PayMethods)
}

func PayMethods2JsonString() string {
	jsonBytes, err := common.Marshal(PayMethods)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}

func ContainsPayMethod(method string) bool {
	for _, payMethod := range PayMethods {
		if payMethod["type"] == method {
			return true
		}
	}
	return false
}
