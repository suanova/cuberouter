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
	"os"
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
	serviceChargeEnabled bool
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
		serviceChargeEnabled: setting.PaymentAsiaServiceChargeEnabled,
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
		setting.PaymentAsiaServiceChargeEnabled = saved.serviceChargeEnabled
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
	setting.PaymentAsiaServiceChargeEnabled = false
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

// initPaymentAsiaControllerColumnNames 触发 model 的保留字列初始化：controller
// 测试没有走 model.InitDB，group 列未初始化会让 GetUserGroup 生成非法 SQL。
// 这里借一次不落盘的 SQLite InitDB 完成初始化，随后恢复被改写的全局状态。
func initPaymentAsiaControllerColumnNames(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	originalIsMasterNode := common.IsMasterNode
	originalSQLitePath := common.SQLitePath
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")
	t.Cleanup(func() {
		model.DB = previousDB
		common.IsMasterNode = originalIsMasterNode
		common.SQLitePath = originalSQLitePath
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		if hadSQLDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalSQLDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	})

	common.IsMasterNode = false
	common.SQLitePath = fmt.Sprintf("file:%s_cols?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, os.Setenv("SQL_DSN", "local"))
	require.NoError(t, model.InitDB())
	sqlDB, err := model.DB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	model.DB = previousDB
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
	assert.Equal(t, "HKD", topUp.Currency, "订单必须快照结算币种")
	assert.Equal(t, setting.PaymentAsiaEnvironmentLive, topUp.PaymentEnvironment, "订单必须快照创建时的环境")
}

func TestRequestPaymentAsiaPaySnapshotsSandboxEnvironment(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	newPaymentAsiaCheckoutDB(t)
	createPaymentAsiaPayer(t)
	setting.PaymentAsiaSandbox = true

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
	assert.Equal(t, "https://payment-sandbox.pa-sys.com/app/page/sandbox_token", response.URL)

	topUp := model.GetTopUpByTradeNo(response.Data["merchant_reference"])
	require.NotNil(t, topUp)
	assert.Equal(t, "HKD", topUp.Currency)
	assert.Equal(t, setting.PaymentAsiaEnvironmentSandbox, topUp.PaymentEnvironment)
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

// setPaymentAsiaNotifyOrderSnapshot persists the currency/environment snapshot
// fields on an order created by insertPaymentAsiaNotifyOrder.
func setPaymentAsiaNotifyOrderSnapshot(t *testing.T, topUp *model.TopUp, currency, environment string) {
	t.Helper()
	require.NoError(t, model.DB.Model(topUp).Updates(map[string]interface{}{
		"currency":            currency,
		"payment_environment": environment,
	}).Error)
	topUp.Currency = currency
	topUp.PaymentEnvironment = environment
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
	newPaymentAsiaNotifyDB(t)
	gin.SetMode(gin.TestMode)

	// 回调先按 merchant_reference 查订单再验签：必须先有订单，验签失败才对
	// 未知订单以外的请求回 400。
	_, topUp := insertPaymentAsiaNotifyOrder(t, "PA-NOTIFY-BADSIGN", 15.6)

	recorder := postPaymentAsiaDatafeed(t, map[string]string{
		"amount":             "15.60",
		"currency":           "HKD",
		"request_reference":  "PA-REF-BADSIGN",
		"merchant_reference": topUp.TradeNo,
		"status":             "1",
		"sign":               "bad",
	})

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "OK")

	reloaded := model.GetTopUpByTradeNo(topUp.TradeNo)
	require.NotNil(t, reloaded)
	assert.Equal(t, common.TopUpStatusPending, reloaded.Status)
}

func TestPaymentAsiaNotifyLogFieldsCannotBeForged(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	newPaymentAsiaNotifyDB(t)
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
	forgedTradeNo := "PA-NOTIFY\n" + forgedLine
	// 先查订单再验签：订单号里带换行也必须按原样查得到，验签失败才回 400。
	insertPaymentAsiaNotifyOrder(t, forgedTradeNo, 15.6)
	recorder := postPaymentAsiaDatafeed(t, map[string]string{
		"amount":             "15.60",
		"currency":           "HKD",
		"request_reference":  "PA-REF\rFORGED",
		"merchant_reference": forgedTradeNo,
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

// TestPaymentAsiaNotifyUsesOrderSnapshotAfterSettingsChange 下单后切换环境与币种，
// 旧订单的回调必须仍按订单快照（live secret + HKD）验签与核对，而不是当前配置。
func TestPaymentAsiaNotifyUsesOrderSnapshotAfterSettingsChange(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	newPaymentAsiaNotifyDB(t)
	gin.SetMode(gin.TestMode)

	user, topUp := insertPaymentAsiaNotifyOrder(t, "PANOTIFYSNAPSHOT", 15.6)
	setPaymentAsiaNotifyOrderSnapshot(t, topUp, "HKD", setting.PaymentAsiaEnvironmentLive)

	setting.PaymentAsiaSandbox = true
	setting.PaymentAsiaCurrency = "USD"

	fields := map[string]string{
		"amount":             "15.60",
		"currency":           "HKD",
		"request_reference":  "PA-REF-SNAPSHOT",
		"merchant_reference": topUp.TradeNo,
		"status":             "1",
	}
	fields["sign"] = service.BuildPaymentAsiaSign(fields, "live_secret")

	recorder := postPaymentAsiaDatafeed(t, fields)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
	assert.Equal(t, 1_000_000, paymentAsiaNotifyUserQuota(t, user.Id))

	reloaded := model.GetTopUpByTradeNo(topUp.TradeNo)
	require.NotNil(t, reloaded)
	assert.Equal(t, common.TopUpStatusSuccess, reloaded.Status)
}

// TestPaymentAsiaNotifyRejectsCurrencyOutsideOrderSnapshot 当前配置切到 USD 后，
// 快照为 HKD 的订单只接受 HKD 回调；来自切换后配置的 USD 回调必须拒绝。
func TestPaymentAsiaNotifyRejectsCurrencyOutsideOrderSnapshot(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	newPaymentAsiaNotifyDB(t)
	gin.SetMode(gin.TestMode)

	user, topUp := insertPaymentAsiaNotifyOrder(t, "PANOTIFYSNPCURRENCY", 15.6)
	setPaymentAsiaNotifyOrderSnapshot(t, topUp, "HKD", setting.PaymentAsiaEnvironmentLive)

	setting.PaymentAsiaCurrency = "USD"

	fields := map[string]string{
		"amount":             "15.60",
		"currency":           "USD",
		"request_reference":  "PA-REF-SNAPSHOT-CURRENCY",
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

// TestPaymentAsiaNotifyRejectsOrderEnvironmentWithoutSecret 订单属于沙箱环境，
// 但沙箱 secret 已被清空：即使生产 secret 让回调整体可达，也必须回 500 让网关
// 重试，而不是用错误环境的 secret 验签后回 400 或错误入账。
func TestPaymentAsiaNotifyRejectsOrderEnvironmentWithoutSecret(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	newPaymentAsiaNotifyDB(t)
	gin.SetMode(gin.TestMode)

	user, topUp := insertPaymentAsiaNotifyOrder(t, "PANOTIFYNOENVSECRET", 15.6)
	setPaymentAsiaNotifyOrderSnapshot(t, topUp, "HKD", setting.PaymentAsiaEnvironmentSandbox)

	setting.PaymentAsiaSandboxMerchantToken = ""
	setting.PaymentAsiaSandboxSecret = ""

	fields := map[string]string{
		"amount":             "15.60",
		"currency":           "HKD",
		"request_reference":  "PA-REF-NO-ENV-SECRET",
		"merchant_reference": topUp.TradeNo,
		"status":             "1",
	}
	fields["sign"] = service.BuildPaymentAsiaSign(fields, "unused_secret")

	recorder := postPaymentAsiaDatafeed(t, fields)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)

	reloaded := model.GetTopUpByTradeNo(topUp.TradeNo)
	require.NotNil(t, reloaded)
	assert.Equal(t, common.TopUpStatusPending, reloaded.Status)
	assert.Zero(t, paymentAsiaNotifyUserQuota(t, user.Id))
}

// postPaymentAsiaAmount 以 JSON POST 调用报价 handler，用户固定为 fixture 的 42。
func postPaymentAsiaAmount(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 42)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/user/paymentasia/amount",
		strings.NewReader(body),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")
	RequestPaymentAsiaAmount(ctx)
	return recorder
}

func TestRequestPaymentAsiaAmountTokenAmountMultiple(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	initPaymentAsiaControllerColumnNames(t)
	newPaymentAsiaCheckoutDB(t)
	createPaymentAsiaPayer(t)
	operation_setting.SetQuotaDisplayTypeForTest(operation_setting.QuotaDisplayTypeTokens)

	// 750000 不是 QuotaPerUnit(500000) 的整数倍：报价必须拒绝，否则同一笔
	// 充值会按截断后的 1 美元报价、再按 2 美元落库/扣费。
	recorder := postPaymentAsiaAmount(t, `{"amount":750000}`)
	assert.JSONEq(t, `{"message":"error","data":"充值数量必须为 500000 的整数倍"}`, recorder.Body.String())

	// 整数倍按 token 数 ÷ QuotaPerUnit 换算成 2 美元 → 15.60
	recorder = postPaymentAsiaAmount(t, `{"amount":1000000}`)
	assert.JSONEq(t, `{"message":"success","data":"15.60"}`, recorder.Body.String())
}

func TestRequestPaymentAsiaPayTokenAmountMultiple(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	newPaymentAsiaCheckoutDB(t)
	createPaymentAsiaPayer(t)
	operation_setting.SetQuotaDisplayTypeForTest(operation_setting.QuotaDisplayTypeTokens)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 42)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/user/paymentasia/pay",
		strings.NewReader(`{"amount":750000,"payment_method":"paymentasia_alipayhk"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	RequestPaymentAsiaPay(ctx)

	assert.JSONEq(t, `{"message":"error","data":"充值数量必须为 500000 的整数倍"}`, recorder.Body.String())
	var orderCount int64
	require.NoError(t, model.DB.Model(&model.TopUp{}).Count(&orderCount).Error)
	assert.Zero(t, orderCount)

	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Set("id", 42)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/user/paymentasia/pay",
		strings.NewReader(`{"amount":1000000,"payment_method":"paymentasia_alipayhk"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	RequestPaymentAsiaPay(ctx)

	var response struct {
		Message string            `json:"message"`
		Data    map[string]string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "success", response.Message)
	assert.Equal(t, "15.60", response.Data["amount"])

	topUp := model.GetTopUpByTradeNo(response.Data["merchant_reference"])
	require.NotNil(t, topUp)
	assert.Equal(t, int64(2), topUp.Amount)
	assert.Equal(t, 15.6, topUp.Money)
}

func TestGetTopUpInfoFiltersPaymentAsiaMethodsByCurrency(t *testing.T) {
	cases := []struct {
		name              string
		currency          string
		wantTypes         []string
		wantEnablePayment bool
	}{
		{
			name:              "HKD keeps every known paymentasia method",
			currency:          "HKD",
			wantTypes:         []string{"paymentasia_alipayhk", "paymentasia_creditcard", "alipay"},
			wantEnablePayment: true,
		},
		{
			name:              "USD keeps only credit card channels",
			currency:          "USD",
			wantTypes:         []string{"paymentasia_creditcard", "alipay"},
			wantEnablePayment: true,
		},
		{
			name:              "unknown currency hides every paymentasia method",
			currency:          "EUR",
			wantTypes:         []string{"alipay"},
			wantEnablePayment: false,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			pinPaymentAsiaControllerFixture(t)
			setting.PaymentAsiaCurrency = testCase.currency
			operation_setting.PayMethods = []map[string]string{
				{"name": "Alipay HK", "type": "paymentasia_alipayhk", "color": "#1677ff", "min_topup": "1"},
				{"name": "Credit Card", "type": "paymentasia_creditcard", "color": "#333333", "min_topup": "1"},
				{"name": "Alipay", "type": "alipay", "color": "#1677ff", "min_topup": "1"},
			}

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

			gotTypes := make([]string, 0, len(response.Data.PayMethods))
			for _, method := range response.Data.PayMethods {
				gotTypes = append(gotTypes, method["type"])
				if service.IsPaymentAsiaMethod(method["type"]) {
					assert.Equal(t, testCase.currency, method["currency"])
				}
			}
			assert.Equal(t, testCase.wantTypes, gotTypes)
			assert.Equal(t, testCase.wantEnablePayment, response.Data.EnablePaymentAsiaTopUp,
				"enable_paymentasia_topup 必须与过滤后的列表一致")
		})
	}
}

func TestRequestPaymentAsiaAmountHonorsPaymentMethod(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	initPaymentAsiaControllerColumnNames(t)
	newPaymentAsiaCheckoutDB(t)
	createPaymentAsiaPayer(t)
	setting.PaymentAsiaUnitPrice = 1.04

	cases := []struct {
		name   string
		method string
		want   string
	}{
		{name: "alipay keeps two decimals", method: "paymentasia_alipay", want: "1.04"},
		{name: "octopus keeps one decimal", method: "paymentasia_octopus", want: "1.0"},
		{name: "no method keeps two decimals", method: "", want: "1.04"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := postPaymentAsiaAmount(t, fmt.Sprintf(`{"amount":1,"payment_method":%q}`, testCase.method))
			assert.JSONEq(t, fmt.Sprintf(`{"message":"success","data":%q}`, testCase.want), recorder.Body.String())
		})
	}

	recorder := postPaymentAsiaAmount(t, `{"amount":1,"payment_method":"paymentasia_typo"}`)
	assert.JSONEq(t, `{"message":"error","data":"支付方式不存在"}`, recorder.Body.String())

	setting.PaymentAsiaCurrency = "USD"
	recorder = postPaymentAsiaAmount(t, `{"amount":1,"payment_method":"paymentasia_alipay"}`)
	assert.JSONEq(t, `{"message":"error","data":"该支付通道不支持 USD 支付"}`, recorder.Body.String())
}

func TestPaymentAsiaNotifyLookupFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	signedFields := func(tradeNo string) map[string]string {
		fields := map[string]string{
			"amount":             "15.60",
			"currency":           "HKD",
			"request_reference":  "PA-REF-LOOKUP",
			"merchant_reference": tradeNo,
			"status":             "1",
		}
		fields["sign"] = service.BuildPaymentAsiaSign(fields, "live_secret")
		return fields
	}

	// 订单不存在是网关重试也无法解决的数据问题：回 200 停止重试。
	t.Run("unknown_order_is_acknowledged", func(t *testing.T) {
		pinPaymentAsiaControllerFixture(t)
		newPaymentAsiaNotifyDB(t)

		recorder := postPaymentAsiaDatafeed(t, signedFields("PA-NOTIFY-MISSING"))

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, "OK", recorder.Body.String())
	})

	// 数据库查询失败是暂时性错误：必须回 500 让网关重试，不能像订单不存在一样吞掉。
	t.Run("lookup_database_error_returns_500", func(t *testing.T) {
		pinPaymentAsiaControllerFixture(t)
		newPaymentAsiaNotifyDB(t)
		require.NoError(t, model.DB.Migrator().DropTable(&model.TopUp{}))

		recorder := postPaymentAsiaDatafeed(t, signedFields("PA-NOTIFY-DBERROR"))

		require.Equal(t, http.StatusInternalServerError, recorder.Code)
	})
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

// TestPaymentAsiaServiceChargeRateForMethod 固定手续费费率表：信用卡最高档
// (3.7% + HK$2)，银联 2.3%，其余钱包/FPS/Octopus/PayMe/未知通道 1.6%。
func TestPaymentAsiaServiceChargeRateForMethod(t *testing.T) {
	testCases := []struct {
		method    string
		wantRate  float64
		wantFixed float64
	}{
		{method: "paymentasia_alipay", wantRate: 0.016},
		{method: "paymentasia_alipaycn", wantRate: 0.016},
		{method: "paymentasia_alipayhk", wantRate: 0.016},
		{method: "paymentasia_wechat", wantRate: 0.016},
		{method: "paymentasia_fps", wantRate: 0.016},
		{method: "paymentasia_octopus", wantRate: 0.016},
		{method: "paymentasia_payme", wantRate: 0.016},
		{method: "paymentasia_generic", wantRate: 0.016},
		{method: "paymentasia_cup", wantRate: 0.023},
		{method: "paymentasia_creditcard", wantRate: 0.037, wantFixed: 2},
		{method: "", wantRate: 0.016},
		{method: "paymentasia_unknown", wantRate: 0.016},
	}

	for _, testCase := range testCases {
		t.Run(testCase.method, func(t *testing.T) {
			charge := paymentAsiaServiceChargeRateForMethod(testCase.method)
			assert.Equal(t, testCase.wantRate, charge.Percent)
			assert.Equal(t, testCase.wantFixed, charge.FixedHKD)
		})
	}
}

// TestCalculatePaymentAsiaServiceFee 覆盖网关手续费与到账额度的换算：
// 实付 base 不变，手续费按网关币种计算后折回展示单位，净额再按订单额度语义入账。
func TestCalculatePaymentAsiaServiceFee(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	origPrice := operation_setting.Price
	t.Cleanup(func() { operation_setting.Price = origPrice })
	operation_setting.Price = 7.3

	testCases := []struct {
		name           string
		currency       string
		unitPrice      float64
		displayType    string
		requestAmount  int64
		storedAmount   int64
		method         string
		network        string
		wantFee        float64
		wantFeeFixed   float64
		wantFeeRate    float64
		wantFeeDisplay string
		wantNetDisplay string
		wantNetQuota   int
	}{
		{
			name:           "hkd display alipay charges 1.6 percent",
			currency:       "HKD",
			unitPrice:      1,
			displayType:    operation_setting.QuotaDisplayTypeUSD,
			requestAmount:  100,
			storedAmount:   100,
			method:         "paymentasia_alipayhk",
			network:        "Alipay",
			wantFee:        1.6,
			wantFeeRate:    0.016,
			wantFeeDisplay: "1.6000",
			wantNetDisplay: "98.4000",
			wantNetQuota:   49_200_000,
		},
		{
			name:           "hkd display credit card charges 3.7 percent plus fixed 2",
			currency:       "HKD",
			unitPrice:      1,
			displayType:    operation_setting.QuotaDisplayTypeUSD,
			requestAmount:  100,
			storedAmount:   100,
			method:         "paymentasia_creditcard",
			network:        "CreditCard",
			wantFee:        5.7,
			wantFeeFixed:   2,
			wantFeeRate:    0.037,
			wantFeeDisplay: "5.7000",
			wantNetDisplay: "94.3000",
			wantNetQuota:   47_150_000,
		},
		{
			name:           "cny display converts the gateway fee back to yuan",
			currency:       "HKD",
			unitPrice:      1.08,
			displayType:    operation_setting.QuotaDisplayTypeCNY,
			requestAmount:  100,
			storedAmount:   100,
			method:         "paymentasia_alipayhk",
			network:        "Alipay",
			wantFee:        1.728,
			wantFeeRate:    0.016,
			wantFeeDisplay: "1.6000",
			wantNetDisplay: "98.4000",
			wantNetQuota:   6_739_726,
		},
		{
			name:           "usd display converts the fixed hkd fee at 7.8",
			currency:       "USD",
			unitPrice:      1,
			displayType:    operation_setting.QuotaDisplayTypeUSD,
			requestAmount:  100,
			storedAmount:   100,
			method:         "paymentasia_creditcard",
			network:        "CreditCard",
			wantFee:        3.9564102564102565,
			wantFeeFixed:   0.2564102564102564,
			wantFeeRate:    0.037,
			wantFeeDisplay: "3.9564",
			wantNetDisplay: "96.0436",
			wantNetQuota:   48_021_800,
		},
		{
			name:           "tokens display keeps the order amount unit",
			currency:       "HKD",
			unitPrice:      1,
			displayType:    operation_setting.QuotaDisplayTypeTokens,
			requestAmount:  1_000_000,
			storedAmount:   2,
			method:         "paymentasia_alipayhk",
			network:        "Alipay",
			wantFee:        0.032,
			wantFeeRate:    0.016,
			wantFeeDisplay: "0.0320",
			wantNetDisplay: "1.9680",
			wantNetQuota:   984_000,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			setting.PaymentAsiaCurrency = testCase.currency
			setting.PaymentAsiaUnitPrice = testCase.unitPrice
			operation_setting.SetQuotaDisplayTypeForTest(testCase.displayType)
			operation_setting.GetPaymentSetting().AmountDiscount = nil

			breakdown, err := calculatePaymentAsiaServiceFee(
				testCase.requestAmount,
				testCase.storedAmount,
				"default",
				testCase.method,
				testCase.network,
			)
			require.NoError(t, err)
			assert.InDelta(t, testCase.wantFee, breakdown.Fee, 1e-9)
			assert.InDelta(t, testCase.wantFeeFixed, breakdown.FeeFixed, 1e-9)
			assert.Equal(t, testCase.wantFeeRate, breakdown.FeeRate)
			assert.Equal(t, testCase.wantFeeDisplay, breakdown.FeeDisplay.StringFixed(4))
			assert.Equal(t, testCase.wantNetDisplay, breakdown.NetDisplay.StringFixed(4))
			assert.Equal(t, testCase.wantNetQuota, breakdown.NetQuota)
		})
	}
}

// TestCalculatePaymentAsiaServiceFeeRejectsFeeAtOrAboveAmount 手续费大于等于
// 实付金额时不得下单：否则会给出零或负额度。
func TestCalculatePaymentAsiaServiceFeeRejectsFeeAtOrAboveAmount(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	operation_setting.SetQuotaDisplayTypeForTest(operation_setting.QuotaDisplayTypeUSD)
	operation_setting.GetPaymentSetting().AmountDiscount = nil
	setting.PaymentAsiaCurrency = "HKD"
	setting.PaymentAsiaUnitPrice = 1

	// 信用卡固定费 HK$2 已超过 1 元实付。
	_, err := calculatePaymentAsiaServiceFee(1, 1, "default", "paymentasia_creditcard", "CreditCard")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "手续费")
}

// TestRequestPaymentAsiaAmountReturnsServiceFee 报价 data 仍是用户实付 base，
// 启用手续费时额外返回 fee/fee_rate/fee_fixed/net_amount/net_quota。
func TestRequestPaymentAsiaAmountReturnsServiceFee(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	initPaymentAsiaControllerColumnNames(t)
	newPaymentAsiaCheckoutDB(t)
	createPaymentAsiaPayer(t)
	setting.PaymentAsiaServiceChargeEnabled = true
	setting.PaymentAsiaUnitPrice = 1
	operation_setting.GetPaymentSetting().AmountDiscount = nil

	recorder := postPaymentAsiaAmount(t, `{"amount":100,"payment_method":"paymentasia_alipayhk"}`)
	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Message   string  `json:"message"`
		Data      string  `json:"data"`
		Fee       string  `json:"fee"`
		FeeRate   float64 `json:"fee_rate"`
		FeeFixed  float64 `json:"fee_fixed"`
		NetAmount string  `json:"net_amount"`
		NetQuota  int     `json:"net_quota"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "success", response.Message)
	assert.Equal(t, "100.00", response.Data, "报价 data 必须保持用户实付 base")
	assert.Equal(t, "1.6000", response.Fee)
	assert.Equal(t, 0.016, response.FeeRate)
	assert.Zero(t, response.FeeFixed)
	assert.Equal(t, "98.4000", response.NetAmount)
	assert.Equal(t, 49_200_000, response.NetQuota)

	// 关闭开关后报价与旧行为完全一致：不返回任何手续费字段。
	setting.PaymentAsiaServiceChargeEnabled = false
	recorder = postPaymentAsiaAmount(t, `{"amount":100,"payment_method":"paymentasia_alipayhk"}`)
	var raw map[string]interface{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &raw))
	assert.Equal(t, "100.00", raw["data"])
	for _, key := range []string{"fee", "fee_rate", "fee_fixed", "net_amount", "net_quota"} {
		assert.NotContains(t, raw, key)
	}
}

// TestRequestPaymentAsiaPaySnapshotsServiceFee 下单时把手续费与净额快照到订单，
// 实付 Money 与用户填写 Amount 均保持不变。
func TestRequestPaymentAsiaPaySnapshotsServiceFee(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	newPaymentAsiaCheckoutDB(t)
	createPaymentAsiaPayer(t)
	setting.PaymentAsiaServiceChargeEnabled = true
	setting.PaymentAsiaUnitPrice = 1
	operation_setting.GetPaymentSetting().AmountDiscount = nil

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 42)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/user/paymentasia/pay",
		strings.NewReader(`{"amount":100,"payment_method":"paymentasia_alipayhk"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	RequestPaymentAsiaPay(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Message string            `json:"message"`
		Data    map[string]string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "success", response.Message)
	assert.Equal(t, "100.00", response.Data["amount"], "支付表单金额保持用户实付 base")

	topUp := model.GetTopUpByTradeNo(response.Data["merchant_reference"])
	require.NotNil(t, topUp)
	assert.Equal(t, int64(100), topUp.Amount)
	assert.Equal(t, 100.0, topUp.Money)
	assert.Equal(t, 1.6, topUp.Fee)
	assert.Equal(t, 0.016, topUp.FeeRate)
	assert.Zero(t, topUp.FeeFixed)
	assert.Equal(t, 98.4, topUp.NetAmount)
	assert.Equal(t, 49_200_000, topUp.NetQuota)
}

// TestRequestPaymentAsiaPayWithoutServiceChargeKeepsLegacySnapshot 开关关闭时
// 订单不写手续费快照，结算按全额入账（NetQuota=0 回退）。
func TestRequestPaymentAsiaPayWithoutServiceChargeKeepsLegacySnapshot(t *testing.T) {
	pinPaymentAsiaControllerFixture(t)
	newPaymentAsiaCheckoutDB(t)
	createPaymentAsiaPayer(t)
	setting.PaymentAsiaUnitPrice = 1
	operation_setting.GetPaymentSetting().AmountDiscount = nil

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 42)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/user/paymentasia/pay",
		strings.NewReader(`{"amount":100,"payment_method":"paymentasia_alipayhk"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	RequestPaymentAsiaPay(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Message string            `json:"message"`
		Data    map[string]string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "success", response.Message)

	topUp := model.GetTopUpByTradeNo(response.Data["merchant_reference"])
	require.NotNil(t, topUp)
	assert.Zero(t, topUp.Fee)
	assert.Zero(t, topUp.FeeRate)
	assert.Zero(t, topUp.FeeFixed)
	assert.Zero(t, topUp.NetAmount)
	assert.Zero(t, topUp.NetQuota)
}
