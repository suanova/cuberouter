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

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	hosttypes "github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCalculateTextQuotaSummaryUsesFoldedUserModelDiscount：结算读取折叠后的
// GroupRatioInfo（与 ModelPriceHelper 的预扣同一口径）。
// relay/helper 依赖 service，service 测试无法直接调用 ModelPriceHelper，
// 因此按它的产出形态装配折后 PriceData：GroupRatio = 分组倍率 1 × 折扣 d。
func TestCalculateTextQuotaSummaryUsesFoldedUserModelDiscount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const modelName = "discount-quota-model"
	discount := model.LookupModelDiscount(map[string]float64{modelName: 0.5}, modelName)
	require.Equal(t, 0.5, discount)

	tests := []struct {
		name      string
		d         float64
		wantQuota int
		wantRatio float64
	}{
		{name: "discounted request settles on the folded ratio", d: discount, wantQuota: 264, wantRatio: 0.5},
		{name: "full-price request settles unchanged", d: 1, wantQuota: 528, wantRatio: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			relayInfo := &relaycommon.RelayInfo{
				RelayFormat:     types.RelayFormatOpenAI,
				OriginModelName: modelName,
				PriceData: hosttypes.PriceData{
					ModelRatio:      0.88,
					CompletionRatio: 1,
					GroupRatioInfo: hosttypes.GroupRatioInfo{
						GroupRatio:           1 * tt.d,
						UserModelDiscount:    tt.d,
						HasUserModelDiscount: tt.d != 1,
					},
				},
				StartTime: time.Now(),
			}
			// 600 prompt tokens,无补全：600 × 0.88 × 1 × d，与预扣 600×0.88×1×d 同口径。
			summary := calculateTextQuotaSummary(ctx, relayInfo, &dto.Usage{PromptTokens: 600})

			assert.InDelta(t, tt.wantRatio, summary.GroupRatio, 1e-9)
			assert.Equal(t, tt.wantQuota, summary.Quota)
		})
	}
}
