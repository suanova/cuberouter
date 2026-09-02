package ratio_setting

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// beijing returns a fixed time instant expressed in Asia/Shanghai, so tests
// stay deterministic regardless of the machine's local timezone.
func beijing(layout, value string) time.Time {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	t, err := time.ParseInLocation(layout, value, loc)
	if err != nil {
		panic(err)
	}
	return t
}

func TestUpdateVideoPriceValidation(t *testing.T) {
	// 先写入一个合法表,校验失败的更新必须整体回滚,已有配置不受影响
	valid := `{"viduq3-pro":{"rows":[{"resolution":"1080p","normal_price":0.75,"off_peak_price":0.625}]}}`
	require.NoError(t, UpdateVideoPriceByJSONString(valid))

	tests := []struct {
		name    string
		jsonStr string
	}{
		{"empty_rows", `{"m":{"rows":[]}}`},
		{"nil_table", `{"m":null}`},
		{"empty_resolution", `{"m":{"rows":[{"resolution":"  ","normal_price":0.75,"off_peak_price":0.625}]}}`},
		{"zero_normal_price", `{"m":{"rows":[{"resolution":"1080p","normal_price":0,"off_peak_price":0.625}]}}`},
		{"negative_normal_price", `{"m":{"rows":[{"resolution":"1080p","normal_price":-1,"off_peak_price":0.625}]}}`},
		{"negative_off_peak_price", `{"m":{"rows":[{"resolution":"1080p","normal_price":0.75,"off_peak_price":-0.1}]}}`},
		{"zero_off_peak_price", `{"m":{"rows":[{"resolution":"1080p","normal_price":0.75,"off_peak_price":0}]}}`},
		{"malformed_json", `not-json`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, UpdateVideoPriceByJSONString(tt.jsonStr))
			// 失败不写入:新模型 m 不存在,已配置的 viduq3-pro 原样保留
			_, ok := GetVideoPrice("m")
			require.False(t, ok)
			table, ok := GetVideoPrice("viduq3-pro")
			require.True(t, ok)
			require.Equal(t, 0.75, table.Rows[0].NormalPrice)
		})
	}
}

func TestVideoPriceRoundTrip(t *testing.T) {
	jsonStr := `{
		"viduq3-pro": {"rows": [
			{"resolution": "1080p", "normal_price": 0.75, "off_peak_price": 0.625},
			{"resolution": "720p", "normal_price": 0.625, "off_peak_price": 0.5},
			{"resolution": "540p", "normal_price": 0.28125, "off_peak_price": 0.25}
		]},
		"viduq3-turbo": {"rows": [
			{"resolution": "1080p", "normal_price": 1.3, "off_peak_price": 0.9}
		]}
	}`
	require.NoError(t, UpdateVideoPriceByJSONString(jsonStr))

	table, ok := GetVideoPrice("viduq3-pro")
	require.True(t, ok)
	require.Len(t, table.Rows, 3)
	// 精度保留:0.75 / 0.625 / 0.28125 均可被 float64 精确表示
	require.Equal(t, 0.75, table.Rows[0].NormalPrice)
	require.Equal(t, 0.625, table.Rows[0].OffPeakPrice)
	require.Equal(t, 0.625, table.Rows[1].NormalPrice)
	require.Equal(t, 0.28125, table.Rows[2].NormalPrice)

	_, ok = GetVideoPrice("viduq3-turbo")
	require.True(t, ok)
	// 未配置的模型不存在
	_, ok = GetVideoPrice("no-such-model")
	require.False(t, ok)
}

func TestVideoPriceAnchor(t *testing.T) {
	tests := []struct {
		name string
		rows []VideoPriceRow
		want float64
	}{
		{
			name: "ordered_max_first",
			rows: []VideoPriceRow{
				{Resolution: "1080p", NormalPrice: 0.75},
				{Resolution: "720p", NormalPrice: 0.625},
				{Resolution: "540p", NormalPrice: 0.28125},
			},
			want: 0.75,
		},
		{
			name: "unordered_still_max",
			rows: []VideoPriceRow{
				{Resolution: "540p", NormalPrice: 0.28125},
				{Resolution: "1080p", NormalPrice: 0.75},
				{Resolution: "720p", NormalPrice: 0.625},
			},
			want: 0.75,
		},
		{
			name: "single_row",
			rows: []VideoPriceRow{
				{Resolution: "1080p", NormalPrice: 0.4},
			},
			want: 0.4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, VideoPriceAnchor(&VideoPriceTable{Rows: tt.rows}))
		})
	}
}

func TestVideoPriceModelPrice(t *testing.T) {
	// 按次计费美元价格 = 锚点 ¥/秒 ÷ 7.3(1 USD = 7.3 RMB)
	tests := []struct {
		name   string
		rows   []VideoPriceRow
		anchor float64
	}{
		{"pro_1080p", []VideoPriceRow{{Resolution: "1080p", NormalPrice: 0.75}}, 0.75},
		{"max_of_mixed", []VideoPriceRow{
			{Resolution: "540p", NormalPrice: 0.28125},
			{Resolution: "1080p", NormalPrice: 0.75},
			{Resolution: "720p", NormalPrice: 0.625},
		}, 0.75},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.InDelta(t, tt.anchor/USD2RMB, VideoPriceModelPrice(&VideoPriceTable{Rows: tt.rows}), 1e-12)
		})
	}
}

func TestIsOffPeakHour(t *testing.T) {
	// 错峰窗口:北京时间 [22:00, 次日 08:00),半开区间
	shanghai := OffPeakWindow{StartHour: 22, EndHour: 8, Timezone: "Asia/Shanghai"}
	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"peak_0800_boundary", beijing("2006-01-02 15:04:05", "2026-09-01 08:00:00"), false},
		{"offpeak_0759", beijing("2006-01-02 15:04:05", "2026-09-01 07:59:59"), true},
		{"offpeak_2200_boundary", beijing("2006-01-02 15:04:05", "2026-09-01 22:00:00"), true},
		{"peak_2159", beijing("2006-01-02 15:04:05", "2026-09-01 21:59:59"), false},
		{"offpeak_midnight", beijing("2006-01-02 15:04:05", "2026-09-01 00:00:00"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsOffPeakHour(tt.now, shanghai))
		})
	}

	// StartHour == EndHour(如 8/8)视为无错峰:全天任何时刻都不是错峰
	noOffPeak := OffPeakWindow{StartHour: 8, EndHour: 8, Timezone: "Asia/Shanghai"}
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	for _, hour := range []int{0, 7, 8, 12, 22, 23} {
		now := time.Date(2026, 9, 1, hour, 0, 0, 0, loc)
		assert.False(t, IsOffPeakHour(now, noOffPeak), "hour=%d", hour)
	}
}

func TestGetOffPeakWindowDefault(t *testing.T) {
	// 无任何配置时返回默认错峰窗口 {22, 8, Asia/Shanghai}
	w := GetOffPeakWindow()
	require.Equal(t, 22, w.StartHour)
	require.Equal(t, 8, w.EndHour)
	require.Equal(t, "Asia/Shanghai", w.Timezone)
}

func TestUpdateOffPeakWindowValidation(t *testing.T) {
	// 先注册恢复:测试中途 FailNow 也不会把全局错峰窗口泄漏给同包其他测试
	original := GetOffPeakWindow()
	t.Cleanup(func() {
		UpdateOffPeakWindowByJSONString(
			fmt.Sprintf(`{"start_hour":%d,"end_hour":%d,"timezone":%q}`, original.StartHour, original.EndHour, original.Timezone),
		)
	})

	// 空时区补默认;无法加载的时区直接拒绝(错峰价不会静默失效)
	require.NoError(t, UpdateOffPeakWindowByJSONString(`{"start_hour":23,"end_hour":7,"timezone":""}`))
	w := GetOffPeakWindow()
	assert.Equal(t, 23, w.StartHour)
	assert.Equal(t, "Asia/Shanghai", w.Timezone)

	require.Error(t, UpdateOffPeakWindowByJSONString(`{"start_hour":22,"end_hour":8,"timezone":"Not/AZone"}`))
	// 拒绝后保留上一次的合法配置
	w = GetOffPeakWindow()
	assert.Equal(t, 23, w.StartHour)
}
