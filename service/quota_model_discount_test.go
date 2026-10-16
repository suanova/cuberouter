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
package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 实时语音（WSS）预扣不走 HandleGroupRatio，折扣必须在 PreWssConsumeQuota
// 内自行应用，否则这条路径永远按原价扣费。这里用一组写死的账本固定它：
// 模型倍率 2、default 组倍率 1、文本输入 500 + 文本输出 300（补全倍率按未知模型
// 取 1），即原价 quota = (500 + 300*1) * (1*2) = 1600；折扣 0.5 时为 800。
const (
	wssDiscountTestModel      = "wss-discount-test-model"
	wssDiscountSeedQuota      = 100000
	wssDiscountFullPriceQuota = 1600
	wssDiscountHalfPriceQuota = 800
)

type wssDiscountFixture struct {
	user      model.User
	token     model.Token
	relayInfo *relaycommon.RelayInfo
}

// setupWssDiscountFixture 建独立测试库、固定模型与分组倍率，并造出用户与令牌。
// 折扣快照由调用方决定：nil = 未载入（按 ActorUserId/UserId 回源），
// 非 nil 空 map = 已载入且无折扣。
func setupWssDiscountFixture(t *testing.T, name string, discounts map[string]float64) wssDiscountFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	setupServiceTestDB(t)

	previousModelRatios := ratio_setting.ModelRatio2JSONString()
	previousGroupRatios := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"`+wssDiscountTestModel+`":2}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(previousModelRatios))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousGroupRatios))
	})

	user := createServiceTestUser(t, name+"-user", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).
		Update("quota", wssDiscountSeedQuota).Error)

	token := model.Token{
		UserId:      user.Id,
		Key:         common.GetUUID(),
		Name:        name,
		Status:      common.TokenStatusEnabled,
		RemainQuota: wssDiscountSeedQuota,
		ExpiredTime: -1,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(&token).Error)

	return wssDiscountFixture{
		user:  user,
		token: token,
		relayInfo: &relaycommon.RelayInfo{
			UserId:             user.Id,
			TokenId:            token.Id,
			TokenKey:           token.Key,
			OriginModelName:    wssDiscountTestModel,
			UsingGroup:         "default",
			UserModelDiscounts: discounts,
		},
	}
}

func (f wssDiscountFixture) preWssConsume(t *testing.T) {
	t.Helper()
	usage := &dto.RealtimeUsage{
		TotalTokens:  800,
		InputTokens:  500,
		OutputTokens: 300,
		InputTokenDetails: dto.InputTokenDetails{
			TextTokens: 500,
		},
		OutputTokenDetails: dto.OutputTokenDetails{
			TextTokens: 300,
		},
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.NoError(t, PreWssConsumeQuota(ctx, f.relayInfo, usage))
}

// assertCharged 断言预扣恰好扣掉 wantQuota：用户钱包与令牌余额同步减少。
func (f wssDiscountFixture) assertCharged(t *testing.T, wantQuota int) {
	t.Helper()
	var storedUser model.User
	require.NoError(t, model.DB.First(&storedUser, f.user.Id).Error)
	assert.Equal(t, wssDiscountSeedQuota-wantQuota, storedUser.Quota)

	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, f.token.Id).Error)
	assert.Equal(t, wssDiscountSeedQuota-wantQuota, storedToken.RemainQuota)
	assert.Equal(t, wantQuota, storedToken.UsedQuota)
}

func TestPreWssConsumeQuotaAppliesUserModelDiscount(t *testing.T) {
	fixture := setupWssDiscountFixture(t, "wss-discount-applied", map[string]float64{wssDiscountTestModel: 0.5})

	fixture.preWssConsume(t)

	fixture.assertCharged(t, wssDiscountHalfPriceQuota)
}

func TestPreWssConsumeQuotaWithoutDiscountUnchanged(t *testing.T) {
	fixture := setupWssDiscountFixture(t, "wss-discount-absent", map[string]float64{})

	fixture.preWssConsume(t)

	fixture.assertCharged(t, wssDiscountFullPriceQuota)
}

func TestPreWssConsumeQuotaLoadsDiscountWhenSnapshotMissing(t *testing.T) {
	fixture := setupWssDiscountFixture(t, "wss-discount-reloaded", nil)
	stored := model.MarshalUserModelDiscounts(map[string]float64{wssDiscountTestModel: 0.5})
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", fixture.user.Id).
		Update("model_discounts", stored).Error)

	fixture.preWssConsume(t)

	fixture.assertCharged(t, wssDiscountHalfPriceQuota)
}
