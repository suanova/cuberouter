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

package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateImagePriceValidation(t *testing.T) {
	// 先写入一个合法表,校验失败的更新必须整体回滚,已有配置不受影响
	valid := `{"img-model":{"rows":[{"tier":"fast","price":0.0625}]}}`
	require.NoError(t, UpdateImagePriceByJSONString(valid))

	tests := []struct {
		name    string
		jsonStr string
	}{
		{"empty_rows", `{"m":{"rows":[]}}`},
		{"nil_table", `{"m":null}`},
		{"empty_tier", `{"m":{"rows":[{"tier":"  ","price":0.1}]}}`},
		{"unknown_tier", `{"m":{"rows":[{"tier":"1024x1024","price":0.1}]}}`},
		{"zero_price", `{"m":{"rows":[{"tier":"fast","price":0}]}}`},
		{"negative_price", `{"m":{"rows":[{"tier":"fast","price":-0.1}]}}`},
		{"duplicate_tier", `{"m":{"rows":[{"tier":"fast","price":0.1},{"tier":"FAST","price":0.2}]}}`},
		{"malformed_json", `not-json`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, UpdateImagePriceByJSONString(tt.jsonStr))
			// 失败不写入:新模型 m 不存在,已配置的 img-model 原样保留
			_, ok := GetImagePrice("m")
			require.False(t, ok)
			table, ok := GetImagePrice("img-model")
			require.True(t, ok)
			require.Equal(t, 0.0625, table.Rows[0].Price)
		})
	}
}

func TestImagePriceAnchorAndTierRatio(t *testing.T) {
	require.NoError(t, UpdateImagePriceByJSONString(`{
		"img-anchor-model": {"rows": [
			{"tier": "fast", "price": 0.0625},
			{"tier": "high", "price": 0.125},
			{"tier": "standard", "price": 0.1}
		]}
	}`))
	table, ok := GetImagePrice("img-anchor-model")
	require.True(t, ok)

	// 锚点 = 最高价行,保证档位系数 ≤ 1
	assert.Equal(t, 0.125, ImagePriceAnchor(table))
	assert.Equal(t, 1.0, ImagePriceTierRatio(table, "high"))
	assert.Equal(t, 0.5, ImagePriceTierRatio(table, "fast"))
	// 与测试内同一浮点运算,结果必然一致
	assert.Equal(t, 0.1/0.125, ImagePriceTierRatio(table, "standard"))
	// 未配置的档位没有系数(调用方按锚点计费),空档位同理
	assert.Equal(t, 0.0, ImagePriceTierRatio(table, ""))
	_, ok = GetImagePrice("no-table")
	assert.False(t, ok)
}

func TestNormalizeImagePriceTier(t *testing.T) {
	assert.Equal(t, "fast", NormalizeImagePriceTier("Fast"))
	assert.Equal(t, "standard", NormalizeImagePriceTier("  STANDARD "))
	assert.Equal(t, "high", NormalizeImagePriceTier("High"))
	assert.Equal(t, "", NormalizeImagePriceTier("   "))

	// 归一化后,请求侧写法与配置写法等价
	require.NoError(t, UpdateImagePriceByJSONString(`{"img-norm-model":{"rows":[{"tier":" fast ","price":0.5}]}}`))
	table, ok := GetImagePrice("img-norm-model")
	require.True(t, ok)
	assert.Equal(t, "fast", table.Rows[0].Tier)
	assert.Equal(t, 1.0, ImagePriceTierRatio(table, "FAST"))
	assert.Equal(t, 1.0, ImagePriceTierRatio(table, " fast "))
}

func TestUpdateImagePriceEmptyClears(t *testing.T) {
	require.NoError(t, UpdateImagePriceByJSONString(`{"img-clear-model":{"rows":[{"tier":"high","price":0.1}]}}`))
	_, ok := GetImagePrice("img-clear-model")
	require.True(t, ok)
	// 空串清空全部价格表(与 VideoPrice 语义一致)
	require.NoError(t, UpdateImagePriceByJSONString(""))
	_, ok = GetImagePrice("img-clear-model")
	assert.False(t, ok)
}
