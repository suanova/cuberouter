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

// paymentAsiaUnitPrice returns the configured gateway price of one display
// unit, falling back to 1 when the option is missing or invalid.
func paymentAsiaUnitPrice() float64 {
	unitPrice := setting.PaymentAsiaUnitPrice
	if math.IsNaN(unitPrice) || math.IsInf(unitPrice, 0) || unitPrice <= 0 {
		return 1
	}
	return unitPrice
}

// paymentAsiaOrderRateSnapshot returns the CNY/CUSTOM exchange rate to snapshot
// on an order; zero means "USD/TOKENS or no snapshot" and settlement falls back
// to the current rate.
func paymentAsiaOrderRateSnapshot() float64 {
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeCNY:
		return operation_setting.Price
	case operation_setting.QuotaDisplayTypeCustom:
		return operation_setting.GetUsdToCurrencyRate(operation_setting.Price)
	default:
		return 0
	}
}

// paymentAsiaStoredAmount converts a user-entered amount to the unit stored on
// TopUp.Amount: TOKENS mode stores amount/QuotaPerUnit, other modes store the
// entered amount. Callers must validate the TOKENS multiple first.
func paymentAsiaStoredAmount(amount int64) int64 {
	if operation_setting.GetQuotaDisplayType() != operation_setting.QuotaDisplayTypeTokens {
		return amount
	}
	if common.QuotaPerUnit <= 0 || math.IsNaN(common.QuotaPerUnit) || math.IsInf(common.QuotaPerUnit, 0) {
		return amount
	}
	return decimal.NewFromInt(amount).Div(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
}

// paymentAsiaEnvironmentForSandbox names the credential environment of an
// order. It is snapshotted on creation and also serves as the legacy-order
// fallback when the snapshot column is empty.
func paymentAsiaEnvironmentForSandbox(sandbox bool) string {
	if sandbox {
		return setting.PaymentAsiaEnvironmentSandbox
	}
	return setting.PaymentAsiaEnvironmentLive
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
// 是 token 数,先 ÷ QuotaPerUnit 换成美元;CNY/CUSTOM 模式的金额已是展示货币,
// 不再除以汇率,直接乘单价(单价即网关币种/展示货币的换算率)。
func getPaymentAsiaPayMoney(amount int64, group string) float64 {
	dAmount := decimal.NewFromInt(amount)
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		if common.QuotaPerUnit <= 0 {
			return 0
		}
		dAmount = dAmount.Div(decimal.NewFromFloat(common.QuotaPerUnit))
	}

	unitPrice := paymentAsiaUnitPrice()
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

// validatePaymentAsiaAmountMultiple requires TOKENS-mode amounts to be whole
// multiples of QuotaPerUnit: the pay path stores amount/QuotaPerUnit as an
// integer, so a partial multiple would quote one amount and then store and
// charge a truncated one.
func validatePaymentAsiaAmountMultiple(amount int64) error {
	if operation_setting.GetQuotaDisplayType() != operation_setting.QuotaDisplayTypeTokens {
		return nil
	}
	if common.QuotaPerUnit <= 0 || math.IsNaN(common.QuotaPerUnit) || math.IsInf(common.QuotaPerUnit, 0) {
		return errors.New("充值额度配置错误")
	}
	quotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
	if !decimal.NewFromInt(amount).Mod(quotaPerUnit).IsZero() {
		return fmt.Errorf("充值数量必须为 %s 的整数倍", quotaPerUnit.String())
	}
	return nil
}

// paymentAsiaServiceChargeRate 是 PaymentAsia 一个通道的手续费率。FixedHKD 以
// 港币计价，USD 结算时按固定汇率换算。
type paymentAsiaServiceChargeRate struct {
	Percent  float64
	FixedHKD float64
}

// paymentAsiaFixedFeeHKDToUSD 是换算信用卡固定 HK$2 手续费用的固定汇率。
const paymentAsiaFixedFeeHKDToUSD = 7.8

// paymentAsiaServiceChargeRateForMethod 返回通道费率：信用卡最高档
// (3.7% + HK$2)，银联 2.3%，其余钱包/FPS/Octopus/PayMe/通用收银台及未知
// 通道按 1.6% 计费。
func paymentAsiaServiceChargeRateForMethod(methodType string) paymentAsiaServiceChargeRate {
	switch methodType {
	case "paymentasia_creditcard":
		return paymentAsiaServiceChargeRate{Percent: 0.037, FixedHKD: 2}
	case "paymentasia_cup":
		return paymentAsiaServiceChargeRate{Percent: 0.023}
	default:
		return paymentAsiaServiceChargeRate{Percent: 0.016}
	}
}

var errPaymentAsiaServiceFeeExceedsAmount = errors.New("充值金额过低，无法覆盖手续费")

// paymentAsiaFeeBreakdown 是一笔订单的手续费快照。Fee/FeeFixed 为网关币种，
// FeeDisplay/NetDisplay 为展示单位，NetQuota 为实际入账额度。
type paymentAsiaFeeBreakdown struct {
	Fee        float64
	FeeRate    float64
	FeeFixed   float64
	FeeDisplay decimal.Decimal
	NetDisplay decimal.Decimal
	NetQuota   int
}

// calculatePaymentAsiaServiceFee 按用户实际支付的网关金额计算手续费，再把
// 手续费折回展示单位，从到账额度中扣除。requestAmount 是用户输入数量（用于
// 计算实付），storedAmount 是落库的 Amount（TOKENS 模式已 ÷QuotaPerUnit）。
// 报价与下单都调用本函数，保证展示与实际到账一致。
func calculatePaymentAsiaServiceFee(requestAmount int64, storedAmount int64, group string, methodType string, network string, currency string, rate float64) (paymentAsiaFeeBreakdown, error) {
	charge := paymentAsiaServiceChargeRateForMethod(methodType)
	_, baseGateway, err := formatPaymentAsiaAmount(getPaymentAsiaPayMoney(requestAmount, group), network)
	if err != nil {
		return paymentAsiaFeeBreakdown{}, err
	}

	fixedGateway := decimal.NewFromFloat(charge.FixedHKD)
	if currency == "USD" {
		fixedGateway = fixedGateway.Div(decimal.NewFromFloat(paymentAsiaFixedFeeHKDToUSD))
	}
	feeGateway := decimal.NewFromFloat(baseGateway).
		Mul(decimal.NewFromFloat(charge.Percent)).
		Add(fixedGateway)
	// 展示口径统一到 4 位小数：报价展示、订单快照和 NetQuota 都由该值推导，
	// 避免"看到的"与"到账的"出现极小偏差。
	feeDisplay := feeGateway.Div(decimal.NewFromFloat(paymentAsiaUnitPrice())).Round(4)
	netDisplay := decimal.NewFromInt(storedAmount).Sub(feeDisplay)
	if !netDisplay.IsPositive() || feeDisplay.GreaterThanOrEqual(decimal.NewFromInt(storedAmount)) {
		return paymentAsiaFeeBreakdown{}, errPaymentAsiaServiceFeeExceedsAmount
	}

	grossQuota := model.TopupAmountToQuotaDecimal(decimal.NewFromInt(storedAmount), rate)
	feeQuota := model.TopupAmountToQuotaDecimal(feeDisplay, rate)
	netQuota, quotaErr := common.WalletQuotaFromDecimalStrict(grossQuota.Sub(feeQuota))
	if quotaErr != nil || netQuota <= 0 {
		return paymentAsiaFeeBreakdown{}, errPaymentAsiaServiceFeeExceedsAmount
	}

	return paymentAsiaFeeBreakdown{
		Fee:        feeGateway.InexactFloat64(),
		FeeRate:    charge.Percent,
		FeeFixed:   fixedGateway.InexactFloat64(),
		FeeDisplay: feeDisplay,
		NetDisplay: netDisplay,
		NetQuota:   netQuota,
	}, nil
}

// PaymentAsiaAmountRequest is the quote request. payment_method is optional:
// when set, the quote uses that channel's amount precision.
type PaymentAsiaAmountRequest struct {
	Amount        int64  `json:"amount"`
	PaymentMethod string `json:"payment_method"`
}

func RequestPaymentAsiaAmount(c *gin.Context) {
	var req PaymentAsiaAmountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	if minimum := paymentAsiaMinimumTopUp(); req.Amount < minimum {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", minimum)})
		return
	}
	if err := validatePaymentAsiaAmountMultiple(req.Amount); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}
	// 本请求只读取一次结算币种：通道校验、报价与手续费换算必须同源。
	currency := paymentAsiaCurrency()
	rate := paymentAsiaOrderRateSnapshot()
	network := ""
	if req.PaymentMethod != "" {
		channel, ok := service.PaymentAsiaChannelByMethodType(req.PaymentMethod)
		if !ok {
			c.JSON(http.StatusOK, gin.H{"message": "error", "data": "支付方式不存在"})
			return
		}
		if currency == "USD" && channel.Network != "CreditCard" {
			c.JSON(http.StatusOK, gin.H{"message": "error", "data": "该支付通道不支持 USD 支付"})
			return
		}
		network = channel.Network
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
	// 未指定通道时按非 Octopus 的 2 位小数格式化。
	amountStr, _, err := formatPaymentAsiaAmount(getPaymentAsiaPayMoney(req.Amount, group), network)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}
	response := gin.H{"message": "success", "data": amountStr}
	if setting.PaymentAsiaServiceChargeEnabled {
		breakdown, feeErr := calculatePaymentAsiaServiceFee(
			req.Amount,
			paymentAsiaStoredAmount(req.Amount),
			group,
			req.PaymentMethod,
			network,
			currency,
			rate,
		)
		if feeErr != nil {
			c.JSON(http.StatusOK, gin.H{"message": "error", "data": feeErr.Error()})
			return
		}
		response["fee"] = breakdown.FeeDisplay.StringFixed(4)
		response["fee_rate"] = breakdown.FeeRate
		response["fee_fixed"] = breakdown.FeeFixed
		response["net_amount"] = breakdown.NetDisplay.StringFixed(4)
		response["net_quota"] = breakdown.NetQuota
	}
	c.JSON(http.StatusOK, response)
}

func RequestPaymentAsiaPay(c *gin.Context) {
	// 凭据只读取一次：可用性判定、环境快照与本次签名必须来自同一份配置。
	// 读取后立即校验，避免管理员在判定与下单之间清空凭据时生成无法支付的订单。
	merchantToken, secret, sandbox := setting.GetPaymentAsiaCredentials()
	merchantToken = strings.TrimSpace(merchantToken)
	secret = strings.TrimSpace(secret)
	if merchantToken == "" || secret == "" {
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
	if err := validatePaymentAsiaAmountMultiple(req.Amount); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": err.Error()})
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

	amount := paymentAsiaStoredAmount(req.Amount)
	// CNY 模式下快照当前汇率到订单,结算按快照换算,防止待支付期间改汇率
	topUpRate := paymentAsiaOrderRateSnapshot()
	// 所有新订单都快照入账额度，包括免手续费订单：结算不再依赖回调时的展示类型。
	netQuota, quotaErr := common.WalletQuotaFromDecimalStrict(
		model.TopupAmountToQuotaDecimal(decimal.NewFromInt(amount), topUpRate),
	)
	if quotaErr != nil || netQuota <= 0 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值额度无效"})
		return
	}
	var feeBreakdown paymentAsiaFeeBreakdown
	if setting.PaymentAsiaServiceChargeEnabled {
		feeBreakdown, err = calculatePaymentAsiaServiceFee(req.Amount, amount, user.Group, req.PaymentMethod, channel.Network, currency, topUpRate)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"message": "error", "data": err.Error()})
			return
		}
		netQuota = feeBreakdown.NetQuota
	}

	tradeNo := fmt.Sprintf("PA%dNO%s%d", id, common.GetRandomString(6), time.Now().Unix())
	if len(tradeNo) > 36 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "订单号生成失败"})
		return
	}

	topUp := &model.TopUp{
		UserId:             id,
		Amount:             amount,
		Money:              roundedMoney,
		Rate:               topUpRate,
		TradeNo:            tradeNo,
		PaymentMethod:      req.PaymentMethod,
		PaymentProvider:    model.PaymentProviderPaymentAsia,
		CreateTime:         time.Now().Unix(),
		Status:             common.TopUpStatusPending,
		Currency:           currency,
		PaymentEnvironment: paymentAsiaEnvironmentForSandbox(sandbox),
		Fee:                feeBreakdown.Fee,
		FeeRate:            feeBreakdown.FeeRate,
		FeeFixed:           feeBreakdown.FeeFixed,
		NetAmount:          feeBreakdown.NetDisplay.InexactFloat64(),
		NetQuota:           netQuota,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("paymentasia 创建充值订单失败 user_id=%d trade_no=%s payment_method=%s amount=%d error=%q", id, tradeNo, req.PaymentMethod, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

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

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("paymentasia 订单创建 trade_no=%s user_id=%d amount=%d money=%.2f currency=%s method=%s network=%s fee_rate=%.4f fee=%.4f fee_fixed=%.4f net_amount=%.4f net_quota=%d",
		tradeNo, id, req.Amount, roundedMoney, currency, req.PaymentMethod, channel.Network,
		feeBreakdown.FeeRate, feeBreakdown.Fee, feeBreakdown.FeeFixed, feeBreakdown.NetDisplay.InexactFloat64(), feeBreakdown.NetQuota))
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

// PaymentAsiaNotify 处理 PaymentAsia datafeed 回调：先按 merchant_reference 查
// 订单，再用订单快照的环境选 secret 验签、按快照币种核对，最后按状态结算。
// 未知订单/网关不匹配/脏数据回 200 避免无限重试，验签或核对失败回 400，结算
// 与凭据缺失等内部错误回 500 让网关重试（入账本身幂等）。任何状态变更前都
// 必须验签成功。
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

	tradeNo := params["merchant_reference"]
	baseLogFields := fmt.Sprintf("trade_no=%q request_reference=%q status=%q amount=%q currency=%q client_ip=%s",
		tradeNo, params["request_reference"], params["status"], params["amount"], params["currency"], c.ClientIP())
	if strings.TrimSpace(tradeNo) == "" {
		logger.LogWarn(c.Request.Context(), "paymentasia 回调缺少订单号 "+baseLogFields)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)

	topUp, err := model.GetTopUpByTradeNoWithError(tradeNo)
	if err != nil {
		if errors.Is(err, model.ErrTopUpNotFound) {
			logger.LogWarn(c.Request.Context(), "paymentasia 回调订单不存在 "+baseLogFields)
			c.String(http.StatusOK, "OK")
			return
		}
		logger.LogError(c.Request.Context(), fmt.Sprintf("paymentasia 回调查询订单失败 %s error=%q", baseLogFields, err.Error()))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if topUp.PaymentProvider != model.PaymentProviderPaymentAsia {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("paymentasia 回调订单支付网关不匹配 %s order_provider=%s", baseLogFields, topUp.PaymentProvider))
		c.String(http.StatusOK, "OK")
		return
	}

	// 环境与币种都以订单快照为准；旧订单（两列为空，无数据回填）回退当前配置。
	orderEnvironment := strings.TrimSpace(topUp.PaymentEnvironment)
	environmentSnapshotMissing := orderEnvironment == ""
	if environmentSnapshotMissing {
		_, _, sandbox := setting.GetPaymentAsiaCredentials()
		orderEnvironment = paymentAsiaEnvironmentForSandbox(sandbox)
	}
	secret, ok := setting.GetPaymentAsiaSecretForEnvironment(orderEnvironment)
	if !ok {
		logger.LogError(c.Request.Context(), fmt.Sprintf("paymentasia 回调订单环境缺少 secret %s order_environment=%s", baseLogFields, orderEnvironment))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	signOK := service.VerifyPaymentAsiaCallbackSign(params, secret)

	orderCurrency := strings.TrimSpace(topUp.Currency)
	currencySnapshotMissing := orderCurrency == ""
	if currencySnapshotMissing {
		orderCurrency = paymentAsiaCurrency()
	}
	logFields := fmt.Sprintf("%s sign_ok=%t order_currency=%q order_environment=%s order_fee=%.4f order_net_quota=%d",
		baseLogFields, signOK, orderCurrency, orderEnvironment, topUp.Fee, topUp.NetQuota)
	if !signOK {
		logger.LogWarn(c.Request.Context(), "paymentasia 回调验签失败 "+logFields)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if environmentSnapshotMissing {
		logger.LogWarn(c.Request.Context(), "paymentasia 回调订单缺少环境快照，回退当前环境 "+logFields)
	}
	if currencySnapshotMissing {
		logger.LogWarn(c.Request.Context(), "paymentasia 回调订单缺少币种快照，回退当前配置 "+logFields)
	}
	logger.LogInfo(c.Request.Context(), "paymentasia 回调收到 "+logFields)

	if !strings.EqualFold(strings.TrimSpace(params["currency"]), orderCurrency) {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("paymentasia 回调币种不一致 %s", logFields))
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
			// 返回 500 让网关重试：200 会被视为已送达，失败状态可能永远不落库。
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	default:
		logger.LogInfo(c.Request.Context(), "paymentasia 回调忽略状态 "+logFields)
	}
	c.String(http.StatusOK, "OK")
}
