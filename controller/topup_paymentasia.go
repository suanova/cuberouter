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
package controller

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// paymentAsiaCurrency returns the gateway settlement currency: the configured
// value upper-cased and trimmed, HKD when unset.
func paymentAsiaCurrency() string {
	currency := strings.ToUpper(strings.TrimSpace(setting.PaymentAsiaCurrency))
	if currency == "" {
		return "HKD"
	}
	return currency
}

// paymentAsiaMinimumTopUp returns the effective minimum recharge amount: the
// configured PaymentAsia minimum, never below the site-wide minimum. A
// non-positive configuration falls back to the site minimum.
func paymentAsiaMinimumTopUp() int64 {
	minimum := getMinTopup()
	if configured := int64(setting.PaymentAsiaMinTopUp); configured > minimum {
		minimum = configured
	}
	return minimum
}

// getPaymentAsiaPayMoney 计算网关实付金额:amount(展示单位)× PaymentAsiaUnitPrice
// (网关币种单价)× 分组充值倍率 × 折扣。金额语义沿用 Epay:TOKENS 模式下 amount
// 是 token 数,先 ÷ QuotaPerUnit 换成美元;CNY 模式的元金额不再换算,直接乘单价。
func getPaymentAsiaPayMoney(amount int64, group string) float64 {
	dAmount := decimal.NewFromInt(amount)
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		if common.QuotaPerUnit <= 0 {
			return 0
		}
		dAmount = dAmount.Div(decimal.NewFromFloat(common.QuotaPerUnit))
	}

	unitPrice := setting.PaymentAsiaUnitPrice
	if math.IsNaN(unitPrice) || math.IsInf(unitPrice, 0) || unitPrice <= 0 {
		unitPrice = 1
	}
	topupGroupRatio := common.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}
	discount := 1.0
	if ds, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(amount)]; ok && ds > 0 {
		discount = ds
	}

	payMoney := dAmount.
		Mul(decimal.NewFromFloat(unitPrice)).
		Mul(decimal.NewFromFloat(topupGroupRatio)).
		Mul(decimal.NewFromFloat(discount))
	return payMoney.InexactFloat64()
}

// formatPaymentAsiaAmount rounds value to the decimals the channel accepts
// (Octopus 1, others 2) and rejects amounts below the gateway minimum. The
// rounded float64 is the value stored on the order.
func formatPaymentAsiaAmount(value float64, network string) (string, float64, error) {
	places := int32(2)
	minimum := decimal.NewFromFloat(0.01)
	if network == "Octopus" {
		places = 1
		minimum = decimal.NewFromFloat(0.1)
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "", 0, errors.New("充值金额无效")
	}
	rounded := decimal.NewFromFloat(value).Round(places)
	if rounded.LessThan(minimum) {
		return "", 0, errors.New("充值金额过低")
	}
	return rounded.StringFixed(places), rounded.InexactFloat64(), nil
}

func RequestPaymentAsiaAmount(c *gin.Context) {
	var req AmountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	if minimum := paymentAsiaMinimumTopUp(); req.Amount < minimum {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", minimum)})
		return
	}
	id := c.GetInt("id")
	if rejectInvalidTopUpQuota(c, id, req.Amount) {
		return
	}
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}
	// 报价阶段还不知道用户会选哪个通道，按非 Octopus 的 2 位小数格式化。
	amountStr, _, err := formatPaymentAsiaAmount(getPaymentAsiaPayMoney(req.Amount, group), "")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": amountStr})
}

func RequestPaymentAsiaPay(c *gin.Context) {
	if !isPaymentAsiaTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "当前管理员未配置支付信息"})
		return
	}

	var req EpayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	if minimum := paymentAsiaMinimumTopUp(); req.Amount < minimum {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", minimum)})
		return
	}
	id := c.GetInt("id")
	if rejectInvalidTopUpQuota(c, id, req.Amount) {
		return
	}
	if !service.IsPaymentAsiaMethod(req.PaymentMethod) || !operation_setting.ContainsPayMethod(req.PaymentMethod) {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "支付方式不存在"})
		return
	}
	channel, ok := service.PaymentAsiaChannelByMethodType(req.PaymentMethod)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "支付方式不存在"})
		return
	}
	currency := paymentAsiaCurrency()
	if currency != "HKD" && currency != "USD" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "支付币种配置错误"})
		return
	}
	if currency == "USD" && channel.Network != "CreditCard" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "该支付通道不支持 USD 支付"})
		return
	}

	user, err := model.GetUserById(id, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "用户不存在"})
		return
	}
	amountStr, roundedMoney, err := formatPaymentAsiaAmount(getPaymentAsiaPayMoney(req.Amount, user.Group), channel.Network)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}

	tradeNo := fmt.Sprintf("PA%dNO%s%d", id, common.GetRandomString(6), time.Now().Unix())
	if len(tradeNo) > 36 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "订单号生成失败"})
		return
	}
	amount := req.Amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dAmount := decimal.NewFromInt(req.Amount)
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		amount = dAmount.Div(dQuotaPerUnit).IntPart()
	}
	// CNY 模式下快照当前汇率到订单,结算按快照换算,防止待支付期间改汇率
	topUpRate := 0.0
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeCNY {
		topUpRate = operation_setting.Price
	}
	topUp := &model.TopUp{
		UserId:          id,
		Amount:          amount,
		Money:           roundedMoney,
		Rate:            topUpRate,
		TradeNo:         tradeNo,
		PaymentMethod:   req.PaymentMethod,
		PaymentProvider: model.PaymentProviderPaymentAsia,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("paymentasia 创建充值订单失败 user_id=%d trade_no=%s payment_method=%s amount=%d error=%q", id, tradeNo, req.PaymentMethod, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	merchantToken, secret, sandbox := setting.GetPaymentAsiaCredentials()
	merchantToken = strings.TrimSpace(merchantToken)
	secret = strings.TrimSpace(secret)

	customerIP := c.ClientIP()
	if parsedIP := net.ParseIP(customerIP); parsedIP == nil || parsedIP.To4() == nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("paymentasia 客户 IP 非 IPv4，回退 0.0.0.0 user_id=%d trade_no=%s client_ip=%q", id, tradeNo, customerIP))
		customerIP = "0.0.0.0"
	}
	returnURL := strings.TrimSpace(setting.PaymentAsiaReturnUrl)
	if returnURL == "" {
		returnURL = paymentReturnPath("/wallet?show_history=true")
	}
	notifyURL := strings.TrimSpace(setting.PaymentAsiaNotifyUrl)
	if notifyURL == "" {
		notifyURL = service.GetCallbackAddress() + "/api/paymentasia/notify"
	}
	subject := strings.TrimSpace(common.SystemName)
	if subject == "" {
		subject = "CubeRouter"
	}

	fields := map[string]string{
		"merchant_reference": tradeNo,
		"currency":           currency,
		"amount":             amountStr,
		"return_url":         returnURL,
		"notify_url":         notifyURL,
		"customer_ip":        customerIP,
		"network":            channel.Network,
		"subject":            subject,
	}
	if channel.Wallet != "" {
		fields["wallet"] = channel.Wallet
	}
	if email := strings.TrimSpace(user.Email); email != "" {
		fields["customer_email"] = email
	}
	fields["sign"] = service.BuildPaymentAsiaSign(fields, secret)

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("paymentasia 订单创建 trade_no=%s user_id=%d amount=%d money=%.2f currency=%s method=%s network=%s", tradeNo, id, req.Amount, roundedMoney, currency, req.PaymentMethod, channel.Network))
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data":    fields,
		"url":     service.PaymentAsiaGatewayURL(sandbox, channel.Network == "UserDefine", merchantToken),
	})
}
