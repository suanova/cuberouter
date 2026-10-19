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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// pricingDiscountTestModel 是价目表里用来挂折扣的测试模型，带上 zz- 前缀避免
// 和真实模型重名，也不会命中默认供应商规则。
const pricingDiscountTestModel = "zz-pricing-discount-model"

// pricingPageResponse 只需要断言用到的字段：顶层折扣表与 data.pricings。
type pricingPageResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Pricings []model.Pricing `json:"pricings"`
	} `json:"data"`
	// 指针/映射类型刻意不加 omitempty：字段缺失时反序列化为 nil，
	// 便于断言「字段恒存在」这一契约。
	UserModelDiscounts map[string]float64 `json:"user_model_discounts"`
}

// setupPricingModelDiscountTestDB 建好 GetPricing 整条链路要用的表：价目表重建
// 要能力/渠道/模型/供应商，分组解析要用户/组织上下文。SQL 日志静音：分组解析会
// 故意查不存在的账号上下文行，那是正常分支，不该混进测试输出。
func setupPricingModelDiscountTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Organization{}, &model.OrganizationMember{},
		&model.UserAccountContext{}, &model.OrganizationDisableRecord{},
		&model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{},
	))
	// 价目表缓存是包级全局对象：先失效一次，保证本次按本测试自己的库重建；
	// 测试结束再失效，免得把测试模型留给后面的用例。
	model.InvalidatePricingCache()

	t.Cleanup(func() {
		model.InvalidatePricingCache()
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// createPricingDiscountTestUser 建一个 default 分组的普通用户，并按原样写入
// 模型折扣表（空串表示没有折扣）。
func createPricingDiscountTestUser(t *testing.T, db *gorm.DB, username string, modelDiscounts string) model.User {
	t.Helper()

	user := createPricingTestUser(t, db, username, "default")
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", user.Id).
		Update("model_discounts", modelDiscounts).Error)
	return user
}

// seedDiscountPricingModel 让测试模型出现在价目表里：一条 default 分组的启用能力。
func seedDiscountPricingModel(t *testing.T, db *gorm.DB) {
	t.Helper()

	require.NoError(t, db.Create(&model.Ability{
		Group: "default", Model: pricingDiscountTestModel, ChannelId: 1, Enabled: true,
	}).Error)
}

// performGetPricingRequest 直调 handler；viewerId <= 0 模拟未登录（不设 id）。
func performGetPricingRequest(t *testing.T, viewerId int) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	if viewerId > 0 {
		context.Set("id", viewerId)
	}
	GetPricing(context)
	return recorder
}

func decodePricingPageResponse(t *testing.T, recorder *httptest.ResponseRecorder) pricingPageResponse {
	t.Helper()

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload pricingPageResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	return payload
}

// TestGetPricingIncludesViewerModelDiscounts 登录用户的定价响应要带上他自己的模型
// 折扣，前端据此展示折后价；字段与 group_ratio 同层。
func TestGetPricingIncludesViewerModelDiscounts(t *testing.T) {
	db := setupPricingModelDiscountTestDB(t)
	seedDiscountPricingModel(t, db)
	viewer := createPricingDiscountTestUser(t, db, "pricing-discount-viewer",
		`{"`+pricingDiscountTestModel+`":0.8}`)

	payload := decodePricingPageResponse(t, performGetPricingRequest(t, viewer.Id))

	require.Equal(t, map[string]float64{pricingDiscountTestModel: 0.8}, payload.UserModelDiscounts)
}

// TestGetPricingAnonymousHasEmptyDiscounts 未登录时字段恒存在且为空对象：匿名访问
// 没有折扣表，但前端不该为字段是否存在做分支。
func TestGetPricingAnonymousHasEmptyDiscounts(t *testing.T) {
	db := setupPricingModelDiscountTestDB(t)
	seedDiscountPricingModel(t, db)

	payload := decodePricingPageResponse(t, performGetPricingRequest(t, 0))

	require.NotNil(t, payload.UserModelDiscounts, "user_model_discounts 必须始终以对象形式存在")
	require.Empty(t, payload.UserModelDiscounts)
}

// TestGetPricingDiscountsDoNotLeakAcrossViewers 折扣只属于当前登录用户：先请求带折扣的
// A，再请求没有折扣的 B，B 必须为空；同时 model.GetPricing 返回的是共享缓存切片，
// 折扣不能改写它的元素（否则后续所有用户的价目表都会带上别人的折扣价）。
func TestGetPricingDiscountsDoNotLeakAcrossViewers(t *testing.T) {
	db := setupPricingModelDiscountTestDB(t)
	seedDiscountPricingModel(t, db)
	viewerA := createPricingDiscountTestUser(t, db, "pricing-discount-viewer-a",
		`{"`+pricingDiscountTestModel+`":0.8}`)
	viewerB := createPricingDiscountTestUser(t, db, "pricing-discount-viewer-b", "")

	expectedRatio, _, _ := ratio_setting.GetModelRatio(pricingDiscountTestModel)
	cachedBefore, ok := pricingByModelName(model.GetPricing())[pricingDiscountTestModel]
	require.True(t, ok, "测试模型必须出现在价目表缓存里")
	require.Equal(t, expectedRatio, cachedBefore.ModelRatio)

	payloadA := decodePricingPageResponse(t, performGetPricingRequest(t, viewerA.Id))
	require.Equal(t, map[string]float64{pricingDiscountTestModel: 0.8}, payloadA.UserModelDiscounts)
	// 折扣只走顶层字段：价目表条目仍是基准倍率，否则前端会在折后价上再叠一次折扣。
	responsePricing, ok := pricingByModelName(payloadA.Data.Pricings)[pricingDiscountTestModel]
	require.True(t, ok)
	require.Equal(t, expectedRatio, responsePricing.ModelRatio)

	payloadB := decodePricingPageResponse(t, performGetPricingRequest(t, viewerB.Id))
	require.NotNil(t, payloadB.UserModelDiscounts)
	require.Empty(t, payloadB.UserModelDiscounts)

	cachedAfter, ok := pricingByModelName(model.GetPricing())[pricingDiscountTestModel]
	require.True(t, ok)
	require.Equal(t, expectedRatio, cachedAfter.ModelRatio)
}
