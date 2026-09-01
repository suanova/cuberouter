package helper

import (
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

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

// seedVideoPrice 写入与 vidu 价目表一致的管理员配置(分辨率 × 正常/错峰 ¥/秒)。
func seedVideoPrice(t *testing.T) {
	t.Helper()
	err := ratio_setting.UpdateVideoPriceByJSONString(`{
	  "viduq3-pro": {"rows": [
	    {"resolution":"1080p","normal_price":0.75,"off_peak_price":0.375},
	    {"resolution":"720p","normal_price":0.625,"off_peak_price":0.3125},
	    {"resolution":"540p","normal_price":0.28125,"off_peak_price":0.15625}]},
	  "viduq3-turbo": {"rows": [
	    {"resolution":"1080p","normal_price":0.40625,"off_peak_price":0.21875},
	    {"resolution":"720p","normal_price":0.375,"off_peak_price":0.1875},
	    {"resolution":"540p","normal_price":0.21875,"off_peak_price":0.125}]}
	}`)
	require.NoError(t, err)
}

func TestComputeVideoPriceRatiosUnconfiguredModel(t *testing.T) {
	// 未配置视频价格表的模型:不产生任何系数(计费按模型基础价,与插件默认行为一致)
	require.Nil(t, ComputeVideoPriceRatios(relaycommon.TaskSubmitReq{}, "viduq3-pro-fast", time.Now()))
}

func TestComputeVideoPriceRatios(t *testing.T) {
	seedVideoPrice(t)
	peak := beijing("2006-01-02 15:04:05", "2026-09-01 12:00:00")
	offpeak := beijing("2006-01-02 15:04:05", "2026-09-01 23:00:00")

	tests := []struct {
		name  string
		req   relaycommon.TaskSubmitReq
		model string
		now   time.Time
		want  map[string]float64
	}{
		{
			name:  "defaults_pro_720p",
			req:   relaycommon.TaskSubmitReq{},
			model: "viduq3-pro",
			now:   peak,
			want:  map[string]float64{"seconds": 5, "size": 5.0 / 6.0},
		},
		{
			name:  "defaults_turbo_720p",
			req:   relaycommon.TaskSubmitReq{},
			model: "viduq3-turbo",
			now:   peak,
			want:  map[string]float64{"seconds": 5, "size": 12.0 / 13.0},
		},
		{
			name:  "explicit_1080p_no_size_key",
			req:   relaycommon.TaskSubmitReq{Duration: 10, Size: "1080p"},
			model: "viduq3-pro",
			now:   peak,
			want:  map[string]float64{"seconds": 10},
		},
		{
			name:  "turbo_540p",
			req:   relaycommon.TaskSubmitReq{Duration: 5, Size: "540p"},
			model: "viduq3-turbo",
			now:   peak,
			want:  map[string]float64{"seconds": 5, "size": 7.0 / 13.0},
		},
		{
			name:  "uppercase_resolution_normalized",
			req:   relaycommon.TaskSubmitReq{Duration: 5, Size: "720P"},
			model: "viduq3-pro",
			now:   peak,
			want:  map[string]float64{"seconds": 5, "size": 5.0 / 6.0},
		},
		{
			name:  "resolution_field_fallback_when_size_empty",
			req:   relaycommon.TaskSubmitReq{Duration: 5, Resolution: "1080p"},
			model: "viduq3-turbo",
			now:   peak,
			want:  map[string]float64{"seconds": 5},
		},
		{
			name:  "duration_saturated_at_max",
			req:   relaycommon.TaskSubmitReq{Duration: 99999},
			model: "viduq3-pro",
			now:   peak,
			want:  map[string]float64{"seconds": relaycommon.MaxTaskDurationSeconds, "size": 5.0 / 6.0},
		},
		{
			name:  "negative_duration_falls_back",
			req:   relaycommon.TaskSubmitReq{Duration: -5},
			model: "viduq3-pro",
			now:   peak,
			want:  map[string]float64{"seconds": 5, "size": 5.0 / 6.0},
		},
		{
			name:  "seconds_string_fallback",
			req:   relaycommon.TaskSubmitReq{Seconds: "8", Size: "540p"},
			model: "viduq3-pro",
			now:   peak,
			want:  map[string]float64{"seconds": 8, "size": 3.0 / 8.0},
		},
		{
			name:  "unknown_resolution_conservative_1",
			req:   relaycommon.TaskSubmitReq{Duration: 5, Size: "4k"},
			model: "viduq3-pro",
			now:   peak,
			want:  map[string]float64{"seconds": 5},
		},
		{
			name:  "offpeak_pro_720p_half",
			req:   relaycommon.TaskSubmitReq{Duration: 5, Size: "720p"},
			model: "viduq3-pro",
			now:   offpeak,
			want:  map[string]float64{"seconds": 5, "size": 5.0 / 6.0, "time": 0.5},
		},
		{
			name:  "offpeak_pro_540p_not_half",
			req:   relaycommon.TaskSubmitReq{Duration: 5, Size: "540p"},
			model: "viduq3-pro",
			now:   offpeak,
			want:  map[string]float64{"seconds": 5, "size": 3.0 / 8.0, "time": 5.0 / 9.0},
		},
		{
			name:  "offpeak_turbo_1080p_not_half",
			req:   relaycommon.TaskSubmitReq{Duration: 5, Size: "1080p"},
			model: "viduq3-turbo",
			now:   offpeak,
			want:  map[string]float64{"seconds": 5, "time": 7.0 / 13.0},
		},
		{
			name:  "offpeak_turbo_540p_not_half",
			req:   relaycommon.TaskSubmitReq{Duration: 5, Size: "540p"},
			model: "viduq3-turbo",
			now:   offpeak,
			want:  map[string]float64{"seconds": 5, "size": 7.0 / 13.0, "time": 4.0 / 7.0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeVideoPriceRatios(tt.req, tt.model, tt.now)
			assert.Equal(t, tt.want, got)
		})
	}
}
