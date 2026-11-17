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

// paymentAsiaCallbackAmountMatches 判断 datafeed 金额是否与本地订单一致。订单
// Money 在下单时已按通道精度四舍五入（Octopus 1 位小数，其它 2 位），回调金额
// 解析成 decimal 后精确比较，不做浮点字符串比较。
func paymentAsiaCallbackAmountMatches(orderMoney float64, network, callbackAmount string) bool {
	if math.IsNaN(orderMoney) || math.IsInf(orderMoney, 0) {
		return false
	}
	places := int32(2)
	if network == "Octopus" {
		places = 1
	}
	callbackValue, err := decimal.NewFromString(strings.TrimSpace(callbackAmount))
	if err != nil {
		return false
	}
	return decimal.NewFromFloat(orderMoney).Round(places).Equal(callbackValue)
}

// PaymentAsiaNotify 处理 PaymentAsia datafeed 回调：当前环境凭据验签，核对订单、
// 币种与金额后按状态结算。未知订单/网关不匹配/脏数据回 200 避免无限重试，
// 验签或核对失败回 400，结算内部错误回 500 让网关重试（入账本身幂等）。
func PaymentAsiaNotify(c *gin.Context) {
	if !isPaymentAsiaWebhookEnabled() {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("paymentasia 回调被拒绝 reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	if err := c.Request.ParseForm(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("paymentasia 回调表单解析失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	params := make(map[string]string, len(c.Request.PostForm))
	for key := range c.Request.PostForm {
		params[key] = c.Request.PostForm.Get(key)
	}
	if len(params) == 0 {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("paymentasia 回调参数为空 path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// 凭据按配置原样返回，验签前去掉可能的首尾空白（与启用判断口径一致）。
	_, secret, _ := setting.GetPaymentAsiaCredentials()
	signOK := service.VerifyPaymentAsiaCallbackSign(params, strings.TrimSpace(secret))
	logFields := fmt.Sprintf("trade_no=%s request_reference=%s status=%s amount=%s currency=%s client_ip=%s sign_ok=%t",
		params["merchant_reference"], params["request_reference"], params["status"], params["amount"], params["currency"], c.ClientIP(), signOK)
	if !signOK {
		logger.LogWarn(c.Request.Context(), "paymentasia 回调验签失败 "+logFields)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	logger.LogInfo(c.Request.Context(), "paymentasia 回调收到 "+logFields)

	tradeNo := params["merchant_reference"]
	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)

	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil {
		logger.LogWarn(c.Request.Context(), "paymentasia 回调订单不存在 "+logFields)
		c.String(http.StatusOK, "OK")
		return
	}
	if topUp.PaymentProvider != model.PaymentProviderPaymentAsia {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("paymentasia 回调订单支付网关不匹配 %s order_provider=%s", logFields, topUp.PaymentProvider))
		c.String(http.StatusOK, "OK")
		return
	}
	if !strings.EqualFold(strings.TrimSpace(params["currency"]), paymentAsiaCurrency()) {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("paymentasia 回调币种不一致 %s configured_currency=%s", logFields, paymentAsiaCurrency()))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	channel, ok := service.PaymentAsiaChannelByMethodType(topUp.PaymentMethod)
	if !ok {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("paymentasia 回调订单支付方式未知 %s payment_method=%s", logFields, topUp.PaymentMethod))
		c.String(http.StatusOK, "OK")
		return
	}
	if !paymentAsiaCallbackAmountMatches(topUp.Money, channel.Network, params["amount"]) {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("paymentasia 回调金额不一致 %s order_money=%.2f", logFields, topUp.Money))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	switch params["status"] {
	case "1":
		if topUp.Status == common.TopUpStatusPending {
			alreadyDone, err := model.RechargePaymentAsia(tradeNo, model.PaymentAsiaRechargeMeta{
				RequestReference: params["request_reference"],
				Currency:         params["currency"],
				Channel:          topUp.PaymentMethod,
			}, c.ClientIP())
			if err != nil {
				logger.LogError(c.Request.Context(), fmt.Sprintf("paymentasia 回调充值失败 %s user_id=%d error=%q", logFields, topUp.UserId, err.Error()))
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			if alreadyDone {
				logger.LogInfo(c.Request.Context(), fmt.Sprintf("paymentasia 回调幂等跳过 %s user_id=%d", logFields, topUp.UserId))
			}
		} else if topUp.Status == common.TopUpStatusSuccess {
			logger.LogInfo(c.Request.Context(), fmt.Sprintf("paymentasia 回调订单已完成，跳过入账 %s user_id=%d", logFields, topUp.UserId))
		} else {
			logger.LogError(c.Request.Context(), fmt.Sprintf("paymentasia 回调订单状态异常需人工排查 %s order_status=%s user_id=%d", logFields, topUp.Status, topUp.UserId))
		}
	case "2":
		err := model.UpdatePendingTopUpStatus(tradeNo, model.PaymentProviderPaymentAsia, common.TopUpStatusFailed)
		switch {
		case err == nil:
			logger.LogInfo(c.Request.Context(), fmt.Sprintf("paymentasia 回调标记失败 %s user_id=%d", logFields, topUp.UserId))
		case errors.Is(err, model.ErrTopUpNotFound) || errors.Is(err, model.ErrTopUpStatusInvalid):
			logger.LogInfo(c.Request.Context(), fmt.Sprintf("paymentasia 回调标记失败忽略 order_status=%s %s", topUp.Status, logFields))
		default:
			logger.LogError(c.Request.Context(), fmt.Sprintf("paymentasia 回调标记失败错误 %s user_id=%d error=%q", logFields, topUp.UserId, err.Error()))
		}
	default:
		logger.LogInfo(c.Request.Context(), "paymentasia 回调忽略状态 "+logFields)
	}
	c.String(http.StatusOK, "OK")
}
