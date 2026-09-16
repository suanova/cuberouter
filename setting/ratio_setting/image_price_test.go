package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateImagePriceValidation(t *testing.T) {
	// 先写入一个合法表,校验失败的更新必须整体回滚,已有配置不受影响
	valid := `{"img-model":{"rows":[{"resolution":"1024x1024","price":0.0625}]}}`
	require.NoError(t, UpdateImagePriceByJSONString(valid))

	tests := []struct {
		name    string
		jsonStr string
	}{
		{"empty_rows", `{"m":{"rows":[]}}`},
		{"nil_table", `{"m":null}`},
		{"empty_resolution", `{"m":{"rows":[{"resolution":"  ","price":0.1}]}}`},
		{"zero_price", `{"m":{"rows":[{"resolution":"1024x1024","price":0}]}}`},
		{"negative_price", `{"m":{"rows":[{"resolution":"1024x1024","price":-0.1}]}}`},
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

func TestImagePriceAnchorAndSizeRatio(t *testing.T) {
	require.NoError(t, UpdateImagePriceByJSONString(`{
		"img-anchor-model": {"rows": [
			{"resolution": "1024x1024", "price": 0.0625},
			{"resolution": "1328x1328", "price": 0.125},
			{"resolution": "1024x1536", "price": 0.1}
		]}
	}`))
	table, ok := GetImagePrice("img-anchor-model")
	require.True(t, ok)

	// 锚点 = 最高价行,保证 size 系数 ≤ 1
	assert.Equal(t, 0.125, ImagePriceAnchor(table))
	assert.Equal(t, 1.0, ImagePriceSizeRatio(table, "1328x1328"))
	assert.Equal(t, 0.5, ImagePriceSizeRatio(table, "1024x1024"))
	// 与测试内同一浮点运算,结果必然一致
	assert.Equal(t, 0.1/0.125, ImagePriceSizeRatio(table, "1024x1536"))
	// 未配置的分辨率没有系数(调用方按锚点计费),空分辨率同理
	assert.Equal(t, 0.0, ImagePriceSizeRatio(table, "2048x2048"))
	assert.Equal(t, 0.0, ImagePriceSizeRatio(table, ""))
	_, ok = GetImagePrice("no-table")
	assert.False(t, ok)
}

func TestNormalizeImageResolution(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"1024x1024", "1024x1024"},
		{"  1328x1328  ", "1328x1328"},
		{"1328X1328", "1328x1328"},
		{"1024*1024", "1024x1024"},
		{"1024 x 1024", "1024x1024"},
		{"", ""},
		{"   ", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, NormalizeImageResolution(tt.in))
	}
	// 归一化后,请求侧写法与配置写法等价
	require.NoError(t, UpdateImagePriceByJSONString(`{"img-norm-model":{"rows":[{"resolution":"1024x1024","price":0.5}]}}`))
	table, ok := GetImagePrice("img-norm-model")
	require.True(t, ok)
	assert.Equal(t, 1.0, ImagePriceSizeRatio(table, "1024*1024"))
	assert.Equal(t, 1.0, ImagePriceSizeRatio(table, " 1024X1024 "))
}

func TestUpdateImagePriceEmptyClears(t *testing.T) {
	require.NoError(t, UpdateImagePriceByJSONString(`{"img-clear-model":{"rows":[{"resolution":"1024x1024","price":0.1}]}}`))
	_, ok := GetImagePrice("img-clear-model")
	require.True(t, ok)
	// 空串清空全部价格表(与 VideoPrice 语义一致)
	require.NoError(t, UpdateImagePriceByJSONString(""))
	_, ok = GetImagePrice("img-clear-model")
	require.False(t, ok)
}
