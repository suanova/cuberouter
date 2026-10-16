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
package helper

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 用户×模型折扣在 HandleGroupRatio 折叠进 GroupRatio：预扣（倍率/按次/tiered）
// 与结算同口径；无折扣时全链路与现状一致。

func discountRatioTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return ctx
}

// TestModelPriceHelperAppliesUserModelDiscount：倍率路径，折扣乘进 GroupRatio。
// modelRatio 0.88、default 组(1.0)、折扣 0.5、floor 500 + max_tokens 100
// → 600 × 0.88 × 1 × 0.5 = 264。
func TestModelPriceHelperAppliesUserModelDiscount(t *testing.T) {
	savedRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedRatios)) })
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"discount-ratio-model":0.88}`))

	prevFloor := common.PreConsumedQuota
	common.PreConsumedQuota = 500
	t.Cleanup(func() { common.PreConsumedQuota = prevFloor })

	ctx := discountRatioTestContext(t)
	info := &relaycommon.RelayInfo{
		OriginModelName:    "discount-ratio-model",
		UserGroup:          "default",
		UsingGroup:         "default",
		UserModelDiscounts: map[string]float64{"discount-ratio-model": 0.5},
	}

	priceData, err := ModelPriceHelper(ctx, info, 55, &types.TokenCountMeta{MaxTokens: 100})
	require.NoError(t, err)

	detail := priceData.PreConsumeDetail
	require.NotNil(t, detail)
	assert.EqualValues(t, 600, detail.PreConsumedTokens)
	assert.InDelta(t, 0.5, detail.GroupRatio, 1e-9)
	assert.InDelta(t, 0.5, detail.UserModelDiscount, 1e-9)
	assert.EqualValues(t, 264, detail.Quota, "want 600×0.88×1×0.5")
	assert.EqualValues(t, 264, priceData.QuotaToPreConsume)
	assert.InDelta(t, 0.5, priceData.GroupRatioInfo.GroupRatio, 1e-9)
	assert.InDelta(t, 0.5, priceData.GroupRatioInfo.UserModelDiscount, 1e-9)
	assert.True(t, priceData.GroupRatioInfo.HasUserModelDiscount)
}

// TestModelPriceHelperNoDiscountUnchanged：无折扣快照时与现状一致，且不产生折扣标记。
func TestModelPriceHelperNoDiscountUnchanged(t *testing.T) {
	savedRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedRatios)) })
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"discount-none-model":0.88}`))

	prevFloor := common.PreConsumedQuota
	common.PreConsumedQuota = 500
	t.Cleanup(func() { common.PreConsumedQuota = prevFloor })

	ctx := discountRatioTestContext(t)
	info := &relaycommon.RelayInfo{
		OriginModelName: "discount-none-model",
		UserGroup:       "default",
		UsingGroup:      "default",
	}

	priceData, err := ModelPriceHelper(ctx, info, 55, &types.TokenCountMeta{MaxTokens: 100})
	require.NoError(t, err)

	detail := priceData.PreConsumeDetail
	require.NotNil(t, detail)
	assert.EqualValues(t, 528, detail.Quota, "want 600×0.88")
	assert.EqualValues(t, 528, priceData.QuotaToPreConsume)
	assert.InDelta(t, 1, detail.GroupRatio, 1e-9)
	assert.InDelta(t, 1, detail.UserModelDiscount, 1e-9)
	assert.InDelta(t, 1, priceData.GroupRatioInfo.GroupRatio, 1e-9)
	assert.False(t, priceData.GroupRatioInfo.HasUserModelDiscount)
}

// TestModelPriceHelperPerCallAppliesUserModelDiscount：模型固定价格路径。
// 单价 0.04 USD、default 组、折扣 0.5 → 0.04 × 500000 × 1 × 0.5 = 10000。
// 按次路径(Quota)与按量预扣路径(QuotaToPreConsume)必须按同一折扣缩放。
func TestModelPriceHelperPerCallAppliesUserModelDiscount(t *testing.T) {
	savedPrices := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedPrices)) })
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"discount-price-model":0.04}`))

	ctx := discountRatioTestContext(t)
	discounts := map[string]float64{"discount-price-model": 0.5}

	perCallInfo := &relaycommon.RelayInfo{
		OriginModelName:    "discount-price-model",
		UserGroup:          "default",
		UsingGroup:         "default",
		UserModelDiscounts: discounts,
	}
	perCallData, err := ModelPriceHelperPerCall(ctx, perCallInfo)
	require.NoError(t, err)
	assert.True(t, perCallData.UsePrice)
	assert.EqualValues(t, 10000, perCallData.Quota, "want 0.04×500000×0.5")
	assert.InDelta(t, 0.5, perCallData.GroupRatioInfo.GroupRatio, 1e-9)
	assert.True(t, perCallData.GroupRatioInfo.HasUserModelDiscount)

	ratioInfo := &relaycommon.RelayInfo{
		OriginModelName:    "discount-price-model",
		UserGroup:          "default",
		UsingGroup:         "default",
		UserModelDiscounts: discounts,
	}
	ratioData, err := ModelPriceHelper(ctx, ratioInfo, 0, &types.TokenCountMeta{})
	require.NoError(t, err)
	assert.True(t, ratioData.UsePrice)
	assert.EqualValues(t, 10000, ratioData.QuotaToPreConsume, "want 0.04×500000×0.5")
	assert.InDelta(t, 0.5, ratioData.PreConsumeDetail.UserModelDiscount, 1e-9)
}

// TestModelPriceHelperTieredAppliesUserModelDiscount：tiered_expr 路径。
// p=1000、expr p*2 → 折前额度 2000/1e6×500000 = 1000，折扣 0.5 → 预扣 500；
// 快照里的 GroupRatio 必须是折后值，折前额度保持原始成本。
func TestModelPriceHelperTieredAppliesUserModelDiscount(t *testing.T) {
	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode":    `{"discount-tiered-model":"tiered_expr"}`,
		"billing_setting.billing_expr":    `{"discount-tiered-model":"tier(\"base\", p * 2)"}`,
		"group_ratio_setting.group_ratio": `{"default":1}`,
	}))
	require.Equal(t, billing_setting.BillingModeTieredExpr, billing_setting.GetBillingMode("discount-tiered-model"))

	ctx := discountRatioTestContext(t)
	info := &relaycommon.RelayInfo{
		OriginModelName:    "discount-tiered-model",
		UserGroup:          "default",
		UsingGroup:         "default",
		UserModelDiscounts: map[string]float64{"discount-tiered-model": 0.5},
		BillingRequestInput: &billingexpr.RequestInput{
			Body: []byte(`{}`),
		},
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)

	assert.EqualValues(t, 500, priceData.QuotaToPreConsume, "want 1000×0.5")
	assert.InDelta(t, 0.5, priceData.GroupRatioInfo.GroupRatio, 1e-9)
	assert.True(t, priceData.GroupRatioInfo.HasUserModelDiscount)

	snapshot := info.TieredBillingSnapshot
	require.NotNil(t, snapshot)
	assert.EqualValues(t, 1000, snapshot.EstimatedQuotaBeforeGroup)
	assert.InDelta(t, 0.5, snapshot.GroupRatio, 1e-9)
	assert.EqualValues(t, 500, snapshot.EstimatedQuotaAfterGroup)
}

// TestModelPriceHelperIgnoresInvalidStoredDiscount：脏数据(>1)按原价计费并留审计日志。
func TestModelPriceHelperIgnoresInvalidStoredDiscount(t *testing.T) {
	savedRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedRatios)) })
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"discount-invalid-model":0.88}`))

	prevFloor := common.PreConsumedQuota
	common.PreConsumedQuota = 500
	t.Cleanup(func() { common.PreConsumedQuota = prevFloor })

	var logBuffer bytes.Buffer
	common.LogWriterMu.Lock()
	previousWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = previousWriter
		common.LogWriterMu.Unlock()
	})

	ctx := discountRatioTestContext(t)
	info := &relaycommon.RelayInfo{
		OriginModelName:    "discount-invalid-model",
		UserGroup:          "default",
		UsingGroup:         "default",
		UserModelDiscounts: map[string]float64{"discount-invalid-model": 1.5},
	}

	priceData, err := ModelPriceHelper(ctx, info, 55, &types.TokenCountMeta{MaxTokens: 100})
	require.NoError(t, err)

	assert.EqualValues(t, 528, priceData.QuotaToPreConsume, "want 600×0.88 at full price")
	assert.InDelta(t, 1, priceData.GroupRatioInfo.GroupRatio, 1e-9)
	assert.InDelta(t, 1, priceData.GroupRatioInfo.UserModelDiscount, 1e-9)
	assert.False(t, priceData.GroupRatioInfo.HasUserModelDiscount)
	assert.Contains(t, logBuffer.String(), "discount-invalid-model")
}

// TestHandleGroupRatioIsIdempotentAcrossRetries：auto 分组重试会重复调用
// HandleGroupRatio，每次都从基准倍率重新合成，折扣不得叠加翻倍。
func TestHandleGroupRatioIsIdempotentAcrossRetries(t *testing.T) {
	ctx := discountRatioTestContext(t)
	info := &relaycommon.RelayInfo{
		OriginModelName:    "discount-retry-model",
		UserGroup:          "default",
		UsingGroup:         "default",
		UserModelDiscounts: map[string]float64{"discount-retry-model": 0.5},
	}

	first := HandleGroupRatio(ctx, info)
	second := HandleGroupRatio(ctx, info)

	assert.InDelta(t, 0.5, first.GroupRatio, 1e-9)
	assert.InDelta(t, 0.5, second.GroupRatio, 1e-9, "second call must not fold the discount twice")
	assert.InDelta(t, 0.5, second.UserModelDiscount, 1e-9)
	assert.True(t, second.HasUserModelDiscount)
}

func setupModelDiscountTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.User{}))

	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// TestModelPriceHelperLoadedEmptySnapshotSkipsFallback：令牌快照已载入且为空
// （无折扣用户）时以快照为准，不得回源——即使 DB 里该用户有折扣也不查。
func TestModelPriceHelperLoadedEmptySnapshotSkipsFallback(t *testing.T) {
	db := setupModelDiscountTestDB(t)
	user := model.User{
		Username: "discount-empty-snapshot-user", Password: "password", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default",
		ModelDiscounts: `{"discount-empty-snap-model":0.5}`,
	}
	require.NoError(t, db.Create(&user).Error)

	savedRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedRatios)) })
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"discount-empty-snap-model":0.88}`))

	prevFloor := common.PreConsumedQuota
	common.PreConsumedQuota = 500
	t.Cleanup(func() { common.PreConsumedQuota = prevFloor })

	ctx := discountRatioTestContext(t)
	info := &relaycommon.RelayInfo{
		OriginModelName:    "discount-empty-snap-model",
		UserGroup:          "default",
		UsingGroup:         "default",
		ActorUserId:        user.Id,
		UserModelDiscounts: map[string]float64{},
	}

	priceData, err := ModelPriceHelper(ctx, info, 55, &types.TokenCountMeta{MaxTokens: 100})
	require.NoError(t, err)

	assert.EqualValues(t, 528, priceData.QuotaToPreConsume, "want 600×0.88 at full price from the loaded snapshot")
	assert.False(t, priceData.GroupRatioInfo.HasUserModelDiscount)
}

// TestModelPriceHelperNilMapFallsBackToCache：令牌未带折扣快照（nil）时按
// ActorUserId 回源数据库；存在 ActorUserId 时不看属主(UserId)。
func TestModelPriceHelperNilMapFallsBackToCache(t *testing.T) {
	db := setupModelDiscountTestDB(t)
	user := model.User{
		Username: "discount-cache-user", Password: "password", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default",
		ModelDiscounts: `{"discount-cache-model":0.5}`,
	}
	require.NoError(t, db.Create(&user).Error)

	savedRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedRatios)) })
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"discount-cache-model":0.88}`))

	prevFloor := common.PreConsumedQuota
	common.PreConsumedQuota = 500
	t.Cleanup(func() { common.PreConsumedQuota = prevFloor })

	ctx := discountRatioTestContext(t)
	info := &relaycommon.RelayInfo{
		OriginModelName: "discount-cache-model",
		UserGroup:       "default",
		UsingGroup:      "default",
		ActorUserId:     user.Id,
		UserId:          user.Id + 999, // 属主不存在：折扣必须按 actor 取
	}

	priceData, err := ModelPriceHelper(ctx, info, 55, &types.TokenCountMeta{MaxTokens: 100})
	require.NoError(t, err)

	assert.EqualValues(t, 264, priceData.QuotaToPreConsume, "want 600×0.88×1×0.5 from the actor's stored discount")
	assert.InDelta(t, 0.5, priceData.GroupRatioInfo.UserModelDiscount, 1e-9)
	assert.True(t, priceData.GroupRatioInfo.HasUserModelDiscount)
}
