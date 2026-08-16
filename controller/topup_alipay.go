package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
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
	"github.com/smartwalle/alipay/v3"
)

// AlipayPayRequest represents a payment request for Alipay checkout.
type AlipayPayRequest struct {
	Amount        int64  `json:"amount"`
	PaymentMethod string `json:"payment_method"`
}

// GetAlipayClient initializes and returns an Alipay client.
// Returns nil if Alipay is not properly configured.
func GetAlipayClient() *alipay.Client {
	if setting.AlipayAppId == "" || setting.AlipayPrivateKey == "" || setting.AlipayPublicKey == "" {
		return nil
	}

	var client *alipay.Client
	var err error

	// isProduction 为 false 时 SDK 走沙箱网关
	client, err = alipay.New(setting.AlipayAppId, setting.AlipayPrivateKey, !setting.AlipaySandboxEnabled)

	if err != nil {
		logger.LogError(context.Background(), fmt.Sprintf("初始化支付宝客户端失败: %v", err))
		return nil
	}

	// 加载支付宝公钥（公钥模式）
	if err = client.LoadAliPayPublicKey(setting.AlipayPublicKey); err != nil {
		logger.LogError(context.Background(), fmt.Sprintf("加载支付宝公钥失败: %v", err))
		return nil
	}

	return client
}

// getAlipayMinTopup returns the minimum topup amount for Alipay.
func getAlipayMinTopup() int64 {
	minTopup := setting.AlipayMinTopUp
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dMinTopup := decimal.NewFromInt(int64(minTopup))
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		minTopup = int(dMinTopup.Mul(dQuotaPerUnit).IntPart())
	}
	return int64(minTopup)
}

// getAlipayNotifyUrl returns the Alipay async notification URL.
// AlipayNotifyUrl 只存站点根域名（如 https://api.example.com），自动拼接通知路径；
// 为空时回退到 GetCallbackAddress()（ServerAddress 或 CustomCallbackAddress）。
func getAlipayNotifyUrl() string {
	if base := strings.TrimRight(strings.TrimSpace(setting.AlipayNotifyUrl), "/"); base != "" {
		return base + "/api/alipay/notify"
	}
	return strings.TrimRight(service.GetCallbackAddress(), "/") + "/api/alipay/notify"
}

// RequestAlipayPay handles Alipay payment requests.
func RequestAlipayPay(c *gin.Context) {
	var req AlipayPayRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	if req.Amount < getAlipayMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", getAlipayMinTopup())})
		return
	}

	if req.PaymentMethod != model.PaymentMethodAlipay {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "不支持的支付渠道"})
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}
	payMoney := getPayMoney(req.Amount, group) // 复用目标端 getPayMoney
	if payMoney < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	// 生成唯一订单号
	tradeNo := fmt.Sprintf("%s%d", common.GetRandomString(6), time.Now().Unix())
	tradeNo = fmt.Sprintf("ALIUSR%dNO%s", id, tradeNo)

	client := GetAlipayClient()
	if client == nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "当前管理员未配置支付宝支付信息"})
		return
	}

	// 构建回调URL
	notifyUrl := getAlipayNotifyUrl()
	returnUrl := paymentReturnPath("/wallet")

	// 调用支付宝电脑网页支付接口
	p := alipay.TradePagePay{}
	p.NotifyURL = notifyUrl
	p.ReturnURL = returnUrl
	p.Subject = fmt.Sprintf("充值 %d", req.Amount)
	p.OutTradeNo = tradeNo
	p.TotalAmount = strconv.FormatFloat(payMoney, 'f', 2, 64)
	p.ProductCode = "FAST_INSTANT_TRADE_PAY" // PC网页支付产品码

	payUrl, err := client.TradePagePay(p)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("创建支付宝支付订单失败 user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	// 计算实际充值额度（与 EPay 逻辑一致：TOKENS 显示类型下 amount 除以 QuotaPerUnit 存储）
	amount := req.Amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dAmount := decimal.NewFromInt(int64(amount))
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		amount = dAmount.Div(dQuotaPerUnit).IntPart()
	}

	topUp := &model.TopUp{
		UserId:          id,
		Amount:          amount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodAlipay,
		PaymentProvider: model.PaymentProviderAlipay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	err = topUp.Insert()
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("创建支付宝充值订单失败 user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("支付宝 充值订单创建成功 user_id=%d trade_no=%s amount=%d money=%.2f", id, tradeNo, req.Amount, payMoney))
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"pay_url": payUrl.String(),
		},
	})
}

// RequestAlipayAmount handles Alipay amount calculation requests.
func RequestAlipayAmount(c *gin.Context) {
	var req AmountRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	if req.Amount < getAlipayMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", getAlipayMinTopup())})
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": strconv.FormatFloat(payMoney, 'f', 2, 64)})
}

// AlipayNotify handles Alipay async payment notification callbacks.
func AlipayNotify(c *gin.Context) {
	// webhook 启用守卫（目标端既有模式，参照 EpayNotify）
	if !isAlipayWebhookEnabled() {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝 webhook 被拒绝 reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	// 解析通知参数
	if err := c.Request.ParseForm(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝回调解析参数失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	client := GetAlipayClient()
	if client == nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝回调失败: 未找到配置信息 path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	// 验证签名
	if err := client.VerifySign(context.Background(), c.Request.Form); err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝回调验签失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	// 获取交易状态
	tradeStatus := c.Request.FormValue("trade_status")
	outTradeNo := c.Request.FormValue("out_trade_no")
	tradeNo := c.Request.FormValue("trade_no")

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("支付宝回调 path=%q client_ip=%s out_trade_no=%s trade_no=%s trade_status=%s", c.Request.RequestURI, c.ClientIP(), outTradeNo, tradeNo, tradeStatus))

	if tradeStatus == "TRADE_SUCCESS" || tradeStatus == "TRADE_FINISHED" {
		// VerifySign 只能证明是支付宝签名的通知，还要确认属于本应用
		if appId := c.Request.FormValue("app_id"); appId != setting.AlipayAppId {
			logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝回调 app_id 不匹配 out_trade_no=%s app_id=%s client_ip=%s", outTradeNo, appId, c.ClientIP()))
			_, _ = c.Writer.Write([]byte("fail"))
			return
		}

		LockOrder(outTradeNo)
		defer UnlockOrder(outTradeNo)

		topUp := model.GetTopUpByTradeNo(outTradeNo)
		if topUp == nil {
			logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝回调未找到订单 out_trade_no=%s client_ip=%s", outTradeNo, c.ClientIP()))
			_, _ = c.Writer.Write([]byte("fail"))
			return
		}

		if topUp.PaymentProvider != model.PaymentProviderAlipay {
			logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝回调订单支付网关不匹配 out_trade_no=%s order_provider=%s client_ip=%s", outTradeNo, topUp.PaymentProvider, c.ClientIP()))
			_, _ = c.Writer.Write([]byte("fail"))
			return
		}

		// 实付金额不得低于订单金额，防止按低金额回调给全额额度
		paidAmount, parseErr := decimal.NewFromString(c.Request.FormValue("total_amount"))
		if parseErr != nil || paidAmount.LessThan(decimal.NewFromFloat(topUp.Money)) {
			logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝回调金额不匹配 out_trade_no=%s total_amount=%s order_money=%.2f client_ip=%s", outTradeNo, c.Request.FormValue("total_amount"), topUp.Money, c.ClientIP()))
			_, _ = c.Writer.Write([]byte("fail"))
			return
		}

		if topUp.Status == common.TopUpStatusPending {
			err := model.RechargeAlipay(outTradeNo, c.ClientIP())
			if err != nil {
				logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝回调充值失败 out_trade_no=%s client_ip=%s error=%q", outTradeNo, c.ClientIP(), err.Error()))
				_, _ = c.Writer.Write([]byte("fail"))
				return
			}
			logger.LogInfo(c.Request.Context(), fmt.Sprintf("支付宝回调充值成功 out_trade_no=%s client_ip=%s", outTradeNo, c.ClientIP()))
		}
	} else {
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("支付宝回调忽略事件 out_trade_no=%s trade_status=%s client_ip=%s", outTradeNo, tradeStatus, c.ClientIP()))
	}

	// 返回 success 告知支付宝已处理
	_, _ = c.Writer.Write([]byte("success"))
}
