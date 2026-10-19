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
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	hosttypes "github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 消费日志只在真实折扣生效时写 other.user_model_discount，
// 无折扣（全价）的日志与现状完全一致（不新增字段）。

// discountLogRelayInfo 构造折叠后的价格数据：基准分组倍率 1 × 折扣，
// 与 ModelPriceHelper 产出同形（GroupRatio 已含折扣）。
func discountLogRelayInfo(hasDiscount bool, discount float64) *relaycommon.RelayInfo {
	now := time.Now()
	return &relaycommon.RelayInfo{
		OriginModelName:   "discount-log-model",
		StartTime:         now,
		FirstResponseTime: now,
		ChannelMeta:       &relaycommon.ChannelMeta{},
		PriceData: hosttypes.PriceData{
			ModelRatio: 0.88,
			GroupRatioInfo: hosttypes.GroupRatioInfo{
				GroupRatio:           1 * discount,
				UserModelDiscount:    discount,
				HasUserModelDiscount: hasDiscount,
			},
		},
	}
}

func TestGenerateTextOtherInfoIncludesUserModelDiscount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("discounted request records the discount", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		other := GenerateTextOtherInfo(ctx, discountLogRelayInfo(true, 0.5), 0.88, 0.5, 1, 0, 0, 0, 0)

		assert.Equal(t, 0.5, other["user_model_discount"])
		raw, err := common.Marshal(other)
		require.NoError(t, err)
		assert.Contains(t, string(raw), `"user_model_discount":0.5`)
	})

	t.Run("full-price request omits the discount", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		other := GenerateTextOtherInfo(ctx, discountLogRelayInfo(false, 1), 0.88, 1, 1, 0, 0, 0, 0)

		assert.NotContains(t, other, "user_model_discount")
		raw, err := common.Marshal(other)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), "user_model_discount")
	})
}

func TestGenerateMjOtherInfoIncludesUserModelDiscount(t *testing.T) {
	info := discountLogRelayInfo(true, 0.5)
	other := GenerateMjOtherInfo(info, info.PriceData)

	assert.Equal(t, 0.5, other["user_model_discount"])
	assert.Equal(t, 0.5, other["group_ratio"])

	fullPrice := discountLogRelayInfo(false, 1)
	assert.NotContains(t, GenerateMjOtherInfo(fullPrice, fullPrice.PriceData), "user_model_discount")
}
