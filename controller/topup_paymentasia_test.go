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
	"bytes"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type paymentAsiaControllerSettings struct {
	sandbox              bool
	merchantToken        string
	secret               string
	sandboxMerchantToken string
	sandboxSecret        string
	notifyURL            string
	returnURL            string
	currency             string
	unitPrice            float64
	minTopUp             int
	payMethods           []map[string]string
	payAddress           string
	epayID               string
	epayKey              string
	customCallbackURL    string
	quotaPerUnit         float64
	displayType          string
	amountDiscount       map[int]float64
	topupGroupRatioJSON  string
	systemName           string
}

// pinPaymentAsiaControllerFixture 固定本组用例依赖的支付全局配置，并在用例结束后
// 恢复原值：PaymentAsia 凭据/币种/单价/最低充值、PayMethods、Epay 凭据、额度展示
// 类型、QuotaPerUnit、金额折扣、充值分组倍率与站点名。
//
// Epay 凭据刻意设为已配置：若 GetTopUpInfo / RequestEpay 的 paymentasia_* 命名空间
// 守卫失效，PaymentAsia 方法会被当成 Epay 方法放行，测试随即失败。
func pinPaymentAsiaControllerFixture(t *testing.T) {
	t.Helper()
	saved := paymentAsiaControllerSettings{
		sandbox:              setting.PaymentAsiaSandbox,
		merchantToken:        setting.PaymentAsiaMerchantToken,
		secret:               setting.PaymentAsiaSecret,
		sandboxMerchantToken: setting.PaymentAsiaSandboxMerchantToken,
		sandboxSecret:        setting.PaymentAsiaSandboxSecret,
		notifyURL:            setting.PaymentAsiaNotifyUrl,
		returnURL:            setting.PaymentAsiaReturnUrl,
		currency:             setting.PaymentAsiaCurrency,
		unitPrice:            setting.PaymentAsiaUnitPrice,
		minTopUp:             setting.PaymentAsiaMinTopUp,
		payMethods:           operation_setting.PayMethods,
		payAddress:           operation_setting.PayAddress,
		epayID:               operation_setting.EpayId,
		epayKey:              operation_setting.EpayKey,
		customCallbackURL:    operation_setting.CustomCallbackAddress,
		quotaPerUnit:         common.QuotaPerUnit,
		displayType:          operation_setting.GetQuotaDisplayType(),
		amountDiscount:       operation_setting.GetPaymentSetting().AmountDiscount,
		topupGroupRatioJSON:  common.TopupGroupRatio2JSONString(),
		systemName:           common.SystemName,
	}
	t.Cleanup(func() {
		setting.PaymentAsiaSandbox = saved.sandbox
		setting.PaymentAsiaMerchantToken = saved.merchantToken
		setting.PaymentAsiaSecret = saved.secret
		setting.PaymentAsiaSandboxMerchantToken = saved.sandboxMerchantToken
		setting.PaymentAsiaSandboxSecret = saved.sandboxSecret
		setting.PaymentAsiaNotifyUrl = saved.notifyURL
		setting.PaymentAsiaReturnUrl = saved.returnURL
		setting.PaymentAsiaCurrency = saved.currency
		setting.PaymentAsiaUnitPrice = saved.unitPrice
		setting.PaymentAsiaMinTopUp = saved.minTopUp
		operation_setting.PayMethods = saved.payMethods
		operation_setting.PayAddress = saved.payAddress
		operation_setting.EpayId = saved.epayID
		operation_setting.EpayKey = saved.epayKey
		operation_setting.CustomCallbackAddress = saved.customCallbackURL
		common.QuotaPerUnit = saved.quotaPerUnit
		operation_setting.SetQuotaDisplayTypeForTest(saved.displayType)
		operation_setting.GetPaymentSetting().AmountDiscount = saved.amountDiscount
		require.NoError(t, common.UpdateTopupGroupRatioByJSONString(saved.topupGroupRatioJSON))
		common.SystemName = saved.systemName
	})

	setting.PaymentAsiaSandbox = false
	setting.PaymentAsiaMerchantToken = " live_token "
	setting.PaymentAsiaSecret = " live_secret "
	setting.PaymentAsiaSandboxMerchantToken = "sandbox_token"
	setting.PaymentAsiaSandboxSecret = "sandbox_secret"
	setting.PaymentAsiaNotifyUrl = ""
	setting.PaymentAsiaReturnUrl = ""
	setting.PaymentAsiaCurrency = "HKD"
	setting.PaymentAsiaUnitPrice = 7.8
	setting.PaymentAsiaMinTopUp = 1
	operation_setting.PayMethods = []map[string]string{
		{"name": "Alipay HK", "type": "paymentasia_alipayhk", "color": "#1677ff", "min_topup": "1"},
		{"name": "Alipay HK (typo)", "type": "paymentasia_typo", "color": "#000000", "min_topup": "1"},
	}
	operation_setting.PayAddress = "https://pay.example.com"
	operation_setting.EpayId = "epay_id"
	operation_setting.EpayKey = "epay_key"
	operation_setting.CustomCallbackAddress = ""
	common.QuotaPerUnit = 500000
	operation_setting.SetQuotaDisplayTypeForTest(operation_setting.QuotaDisplayTypeUSD)
	operation_setting.GetPaymentSetting().AmountDiscount = map[int]float64{100: 0.9}
	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"default":1,"vip":2}`))
	common.SystemName = "CubeRouter"
}

func TestGetPaymentAsiaPayMoney(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)

	testCases := []struct {
		name        string
		displayType string
		amount      int64
		unitPrice   float64
		group       string
		want        float64
	}{
		{
			name:        "usd display multiplies unit price",
			displayType: operation_setting.QuotaDisplayTypeUSD,
			amount:      2,
			unitPrice:   7.8,
			group:       "default",
			want:        15.6,
		},
		{
			name:        "tokens display converts amount to usd first",
			displayType: operation_setting.QuotaDisplayTypeTokens,
			amount:      1_000_000,
			unitPrice:   7.8,
			group:       "default",
			want:        15.6,
		},
		{
			name:        "cny display charges configured unit price",
			displayType: operation_setting.QuotaDisplayTypeCNY,
			amount:      10,
			unitPrice:   1.08,
			group:       "default",
			want:        10.8,
		},
		{
			name:        "zero unit price falls back to one",
			displayType: operation_setting.QuotaDisplayTypeUSD,
			amount:      2,
			unitPrice:   0,
			group:       "default",
			want:        2,
		},
		{
			name:        "nan unit price falls back to one",
			displayType: operation_setting.QuotaDisplayTypeUSD,
			amount:      2,
			unitPrice:   math.NaN(),
			group:       "default",
			want:        2,
		},
		{
			name:        "infinite unit price falls back to one",
			displayType: operation_setting.QuotaDisplayTypeUSD,
			amount:      2,
			unitPrice:   math.Inf(1),
			group:       "default",
			want:        2,
		},
		{
			name:        "topup group ratio multiplies",
			displayType: operation_setting.QuotaDisplayTypeUSD,
			amount:      2,
			unitPrice:   7.8,
			group:       "vip",
			want:        31.2,
		},
		{
			name:        "amount discount applies to the original amount",
			displayType: operation_setting.QuotaDisplayTypeUSD,
			amount:      100,
			unitPrice:   1,
			group:       "default",
			want:        90,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			operation_setting.SetQuotaDisplayTypeForTest(testCase.displayType)
			setting.PaymentAsiaUnitPrice = testCase.unitPrice
			assert.Equal(t, testCase.want, getPaymentAsiaPayMoney(testCase.amount, testCase.group))
		})
	}
}

func TestFormatPaymentAsiaAmount(t *testing.T) {
	testCases := []struct {
		name        string
		value       float64
		network     string
		wantAmount  string
		wantRounded float64
		wantErr     bool
	}{
		{
			name:        "alipay keeps two decimals",
			value:       12.345,
			network:     "Alipay",
			wantAmount:  "12.35",
			wantRounded: 12.35,
		},
		{
			name:        "octopus keeps one decimal",
			value:       12.345,
			network:     "Octopus",
			wantAmount:  "12.3",
			wantRounded: 12.3,
		},
		{
			name:    "two decimal amount below the gateway minimum is rejected",
			value:   0.001,
			network: "Alipay",
			wantErr: true,
		},
		{
			name:    "octopus amount that rounds to zero is rejected",
			value:   0.04,
			network: "Octopus",
			wantErr: true,
		},
		{
			name:    "non finite amount is rejected instead of panicking",
			value:   math.NaN(),
			network: "Alipay",
			wantErr: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			amount, rounded, err := formatPaymentAsiaAmount(testCase.value, testCase.network)
			if testCase.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testCase.wantAmount, amount)
			assert.Equal(t, testCase.wantRounded, rounded)
		})
	}
}

func TestIsEpayOnlyPaymentMethod(t *testing.T) {
	testCases := []struct {
		name       string
		methodType string
		want       bool
	}{
		{name: "known paymentasia method is not epay", methodType: "paymentasia_alipayhk", want: false},
		{name: "misspelled paymentasia type stays reserved", methodType: "paymentasia_typo", want: false},
		{name: "epay method", methodType: "alipay", want: true},
		{name: "custom epay method", methodType: "custom1", want: true},
		{name: "empty method is rejected", methodType: "", want: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, isEpayOnlyPaymentMethod(testCase.methodType))
		})
	}
}

func TestGetTopUpInfoAttachesPaymentAsiaCurrencyWithoutMutatingConfig(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/topup/info", nil)

	GetTopUpInfo(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Data struct {
			EnablePaymentAsiaTopUp bool                `json:"enable_paymentasia_topup"`
			PaymentAsiaMinTopUp    int64               `json:"paymentasia_min_topup"`
			PayMethods             []map[string]string `json:"pay_methods"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))

	require.Len(t, response.Data.PayMethods, 1)
	assert.Equal(t, "paymentasia_alipayhk", response.Data.PayMethods[0]["type"])
	assert.Equal(t, "HKD", response.Data.PayMethods[0]["currency"])
	assert.True(t, response.Data.EnablePaymentAsiaTopUp)
	assert.Equal(t, int64(1), response.Data.PaymentAsiaMinTopUp)

	for _, method := range operation_setting.PayMethods {
		_, hasCurrency := method["currency"]
		assert.False(t, hasCurrency, "operation_setting.PayMethods must not be mutated")
	}
}

func TestGetTopUpInfoRequiresEnabledCredentialsForPaymentAsiaTopUp(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	setting.PaymentAsiaMerchantToken = ""
	setting.PaymentAsiaSecret = ""

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/topup/info", nil)

	GetTopUpInfo(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Data struct {
			EnablePaymentAsiaTopUp bool                `json:"enable_paymentasia_topup"`
			PayMethods             []map[string]string `json:"pay_methods"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))

	// 凭据缺失时 PaymentAsia 方法整体隐藏，不会退化展示为 Epay 方法
	assert.False(t, response.Data.EnablePaymentAsiaTopUp)
	assert.Empty(t, response.Data.PayMethods)
}

// newPaymentAsiaCheckoutDB 为下单用例换上一套独立的 SQLite 库（User/TopUp），
// 用例结束后恢复 model.DB 并关闭连接。
func newPaymentAsiaCheckoutDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}))
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			_ = sqlDB.Close()
		}
	})
}

func createPaymentAsiaPayer(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:       42,
		Username: "paymentasia_payer",
		Email:    "payer@example.com",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
}

func TestRequestPaymentAsiaPayCreatesSignedPendingOrder(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	newPaymentAsiaCheckoutDB(t)
	createPaymentAsiaPayer(t)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 42)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/user/paymentasia/pay",
		strings.NewReader(`{"amount":2,"payment_method":"paymentasia_alipayhk"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	RequestPaymentAsiaPay(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Message string            `json:"message"`
		URL     string            `json:"url"`
		Data    map[string]string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "success", response.Message)

	// 商户 token 使用 TrimSpace 后的值拼装托管页地址
	assert.Equal(t, "https://payment.pa-sys.com/app/page/live_token", response.URL)
	assert.Equal(t, "HKD", response.Data["currency"])
	assert.Equal(t, "15.60", response.Data["amount"])
	assert.Equal(t, "Alipay", response.Data["network"])
	assert.Equal(t, "AlipayHK", response.Data["wallet"])
	assert.Equal(t, "192.0.2.1", response.Data["customer_ip"])
	assert.Equal(t, "payer@example.com", response.Data["customer_email"])
	assert.Equal(t, "CubeRouter", response.Data["subject"])
	assert.Equal(t, "http://localhost:3000/api/paymentasia/notify", response.Data["notify_url"])
	assert.Equal(t, "http://localhost:3000/wallet?show_history=true", response.Data["return_url"])

	tradeNo := response.Data["merchant_reference"]
	require.True(t, strings.HasPrefix(tradeNo, "PA42NO"))
	assert.LessOrEqual(t, len(tradeNo), 36)

	// 签名只覆盖返回给网关的字段，且使用 TrimSpace 后的 secret
	signedFields := make(map[string]string, len(response.Data))
	for key, value := range response.Data {
		if key == "sign" {
			continue
		}
		signedFields[key] = value
	}
	assert.Equal(t, service.BuildPaymentAsiaSign(signedFields, "live_secret"), response.Data["sign"])

	topUp := model.GetTopUpByTradeNo(tradeNo)
	require.NotNil(t, topUp)
	assert.Equal(t, 42, topUp.UserId)
	assert.Equal(t, int64(2), topUp.Amount)
	assert.Equal(t, 15.6, topUp.Money)
	assert.Equal(t, "paymentasia_alipayhk", topUp.PaymentMethod)
	assert.Equal(t, model.PaymentProviderPaymentAsia, topUp.PaymentProvider)
	assert.Equal(t, common.TopUpStatusPending, topUp.Status)
}

func TestRequestPaymentAsiaPayRejectsUSDAmountOnNonCardNetwork(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	newPaymentAsiaCheckoutDB(t)
	createPaymentAsiaPayer(t)
	setting.PaymentAsiaCurrency = "USD"

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 42)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/user/paymentasia/pay",
		strings.NewReader(`{"amount":2,"payment_method":"paymentasia_alipayhk"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	RequestPaymentAsiaPay(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"message":"error","data":"该支付通道不支持 USD 支付"}`, recorder.Body.String())

	var orderCount int64
	require.NoError(t, model.DB.Model(&model.TopUp{}).Count(&orderCount).Error)
	assert.Zero(t, orderCount)
}

// newPaymentAsiaNotifyDB 为 datafeed 用例换上独立的 SQLite 库（User/TopUp/Log）。
// RechargePaymentAsia 在事务外会写充值日志，因此 LOG_DB 必须指向同一套库；
// Redis 与内存缓存关闭，避免测试依赖外部缓存。
func newPaymentAsiaNotifyDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousDatabaseType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	previousMemoryCache := common.MemoryCacheEnabled
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.Log{}))
	model.DB = db
	model.LOG_DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetMainDatabaseType(previousDatabaseType)
		common.RedisEnabled = previousRedis
		common.MemoryCacheEnabled = previousMemoryCache
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			_ = sqlDB.Close()
		}
	})
}

// insertPaymentAsiaNotifyOrder 插入唯一用户与一条待支付的 PaymentAsia 订单。
func insertPaymentAsiaNotifyOrder(t *testing.T, tradeNo string, money float64) (*model.User, *model.TopUp) {
	t.Helper()
	user := &model.User{
		Id:       7,
		Username: "paymentasia_notify_user",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, model.DB.Create(user).Error)
	topUp := &model.TopUp{
		UserId:          user.Id,
		Amount:          2,
		Money:           money,
		TradeNo:         tradeNo,
		PaymentMethod:   "paymentasia_alipayhk",
		PaymentProvider: model.PaymentProviderPaymentAsia,
		CreateTime:      common.GetTimestamp(),
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, topUp.Insert())
	return user, topUp
}

// postPaymentAsiaDatafeed 以表单 POST 调用 datafeed handler，返回响应记录器。
func postPaymentAsiaDatafeed(t *testing.T, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	for key, value := range fields {
		form.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodPost, "/api/paymentasia/notify", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx.Request = request

	PaymentAsiaNotify(ctx)

	return recorder
}

func paymentAsiaNotifyUserQuota(t *testing.T, userId int) int {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("quota").Where("id = ?", userId).First(&user).Error)
	return user.Quota
}

func TestPaymentAsiaCallbackAmountMatches(t *testing.T) {
	testCases := []struct {
		name           string
		orderMoney     float64
		network        string
		callbackAmount string
		want           bool
	}{
		{name: "two decimal channel matches", orderMoney: 15.6, network: "Alipay", callbackAmount: "15.60", want: true},
		{name: "two decimal channel mismatch", orderMoney: 15.6, network: "Alipay", callbackAmount: "15.61", want: false},
		{name: "octopus compares one decimal", orderMoney: 12.3, network: "Octopus", callbackAmount: "12.30", want: true},
		{name: "octopus rejects a different amount", orderMoney: 12.3, network: "Octopus", callbackAmount: "12.31", want: false},
		{name: "non numeric callback amount", orderMoney: 15.6, network: "Alipay", callbackAmount: "not-a-number", want: false},
		{name: "non finite order money", orderMoney: math.NaN(), network: "Alipay", callbackAmount: "15.60", want: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, paymentAsiaCallbackAmountMatches(testCase.orderMoney, testCase.network, testCase.callbackAmount))
		})
	}
}

func TestPaymentAsiaNotifyRejectsBadSign(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	gin.SetMode(gin.TestMode)

	recorder := postPaymentAsiaDatafeed(t, map[string]string{
		"amount":             "15.60",
		"currency":           "HKD",
		"request_reference":  "PA-REF-BADSIGN",
		"merchant_reference": "PA-NOTIFY-BADSIGN",
		"status":             "1",
		"sign":               "bad",
	})

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "OK")
}

func TestPaymentAsiaNotifyLogFieldsCannotBeForged(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	gin.SetMode(gin.TestMode)

	// 回调值来自未认证的 POST 表单，必须被转义后才能进入日志：否则换行可伪造
	// 一条完整的"充值成功"日志行。
	var logs bytes.Buffer
	common.LogWriterMu.Lock()
	previousWriter := gin.DefaultWriter
	previousErrorWriter := gin.DefaultErrorWriter
	gin.DefaultWriter = &logs
	gin.DefaultErrorWriter = &logs
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter = previousWriter
		gin.DefaultErrorWriter = previousErrorWriter
		common.LogWriterMu.Unlock()
	})

	forgedLine := "[INFO] 2026/11/17 - 00:00:00 | SYSTEM | paymentasia 回调充值成功 user_id=7"
	recorder := postPaymentAsiaDatafeed(t, map[string]string{
		"amount":             "15.60",
		"currency":           "HKD",
		"request_reference":  "PA-REF\rFORGED",
		"merchant_reference": "PA-NOTIFY\n" + forgedLine,
		"status":             "1",
		"sign":               "bad",
	})

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	output := logs.String()
	require.Contains(t, output, "sign_ok=false")
	// %q 把换行/回车转义成字面量，整条日志仍只有末尾一个换行，注入的内容留在引号内。
	assert.Equal(t, 1, strings.Count(output, "\n"), "callback log must stay on a single line: %s", output)
	assert.Zero(t, strings.Count(output, "\r"), "callback log must not contain raw carriage returns: %s", output)
	assert.Contains(t, output, `trade_no="PA-NOTIFY\n`+forgedLine+`"`)
	assert.Contains(t, output, `request_reference="PA-REF\rFORGED"`)
	assert.NotContains(t, output, "\n"+forgedLine)
}

func TestPaymentAsiaNotifyRejectsWhenDisabled(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	setting.PaymentAsiaMerchantToken = ""
	setting.PaymentAsiaSecret = ""
	setting.PaymentAsiaSandboxMerchantToken = ""
	setting.PaymentAsiaSandboxSecret = ""
	gin.SetMode(gin.TestMode)

	recorder := postPaymentAsiaDatafeed(t, map[string]string{
		"amount":             "15.60",
		"currency":           "HKD",
		"request_reference":  "PA-REF-DISABLED",
		"merchant_reference": "PA-NOTIFY-DISABLED",
		"status":             "1",
		"sign":               "bad",
	})

	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "OK")
}

func TestPaymentAsiaNotifyCreditsOnSuccessOnce(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	newPaymentAsiaNotifyDB(t)
	gin.SetMode(gin.TestMode)

	user, topUp := insertPaymentAsiaNotifyOrder(t, "PANOTIFYSUCCESS", 15.6)

	fields := map[string]string{
		"amount":             "15.60",
		"currency":           "HKD",
		"request_reference":  "PA-REF-SUCCESS",
		"merchant_reference": topUp.TradeNo,
		"status":             "1",
	}
	// fixture 的凭据带首尾空格：验签必须使用 TrimSpace 后的 secret
	fields["sign"] = service.BuildPaymentAsiaSign(fields, "live_secret")

	recorder := postPaymentAsiaDatafeed(t, fields)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())

	reloaded := model.GetTopUpByTradeNo(topUp.TradeNo)
	require.NotNil(t, reloaded)
	assert.Equal(t, common.TopUpStatusSuccess, reloaded.Status)
	assert.NotZero(t, reloaded.CompleteTime)
	assert.Equal(t, 1_000_000, paymentAsiaNotifyUserQuota(t, user.Id))

	// 重复回调：订单已 success，只回 200 不再入账
	recorder = postPaymentAsiaDatafeed(t, fields)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "OK", recorder.Body.String())
	assert.Equal(t, 1_000_000, paymentAsiaNotifyUserQuota(t, user.Id))
}

func TestPaymentAsiaNotifyRejectsCurrencyMismatch(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	newPaymentAsiaNotifyDB(t)
	gin.SetMode(gin.TestMode)

	user, topUp := insertPaymentAsiaNotifyOrder(t, "PANOTIFYCURRENCY", 15.6)

	fields := map[string]string{
		"amount":             "15.60",
		"currency":           "USD",
		"request_reference":  "PA-REF-CURRENCY",
		"merchant_reference": topUp.TradeNo,
		"status":             "1",
	}
	fields["sign"] = service.BuildPaymentAsiaSign(fields, "live_secret")

	recorder := postPaymentAsiaDatafeed(t, fields)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "OK")

	reloaded := model.GetTopUpByTradeNo(topUp.TradeNo)
	require.NotNil(t, reloaded)
	assert.Equal(t, common.TopUpStatusPending, reloaded.Status)
	assert.Zero(t, paymentAsiaNotifyUserQuota(t, user.Id))
}

func TestPaymentAsiaNotifyRejectsAmountMismatch(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	newPaymentAsiaNotifyDB(t)
	gin.SetMode(gin.TestMode)

	user, topUp := insertPaymentAsiaNotifyOrder(t, "PANOTIFYAMOUNT", 15.6)

	fields := map[string]string{
		"amount":             "15.61",
		"currency":           "HKD",
		"request_reference":  "PA-REF-AMOUNT",
		"merchant_reference": topUp.TradeNo,
		"status":             "1",
	}
	fields["sign"] = service.BuildPaymentAsiaSign(fields, "live_secret")

	recorder := postPaymentAsiaDatafeed(t, fields)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "OK")

	reloaded := model.GetTopUpByTradeNo(topUp.TradeNo)
	require.NotNil(t, reloaded)
	assert.Equal(t, common.TopUpStatusPending, reloaded.Status)
	assert.Zero(t, paymentAsiaNotifyUserQuota(t, user.Id))
}
