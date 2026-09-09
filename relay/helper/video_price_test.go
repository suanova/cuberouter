package helper

import (
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
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

// seedVideoPrice 写入与 vidu 价目表一致的管理员配置(分辨率 × 正常/错峰 USD/s)。
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

func TestVideoResolutionTier(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "tier_360p", raw: "360p", want: "360p"},
		{name: "tier_480p", raw: "480p", want: "480p"},
		{name: "tier_540p", raw: "540p", want: "540p"},
		{name: "tier_720p", raw: "720p", want: "720p"},
		{name: "tier_1080p", raw: "1080p", want: "1080p"},
		{name: "tier_4k", raw: "4k", want: "4k"},
		{name: "tier_2160p_alias", raw: "2160p", want: "4k"},
		{name: "tier_uppercase_and_space", raw: " 720P ", want: "720p"},
		{name: "px_1080p", raw: "1920x1080", want: "1080p"},
		{name: "px_asterisk", raw: "1920*1080", want: "1080p"},
		{name: "px_portrait_1080p", raw: "1080x1920", want: "1080p"},
		{name: "px_4k", raw: "3840x2160", want: "4k"},
		{name: "px_ultrawide_4k", raw: "4096x2304", want: "4k"},
		{name: "px_720p", raw: "1280x720", want: "720p"},
		{name: "px_480p", raw: "854x480", want: "480p"},
		{name: "px_540p", raw: "960x540", want: "540p"},
		{name: "px_360_content_480_tier", raw: "640x360", want: "480p"},
		{name: "px_360p", raw: "320x240", want: "360p"},
		{name: "empty", raw: "", want: ""},
		{name: "garbage_label", raw: "fhd", want: ""},
		{name: "garbage_single_dimension", raw: "1920", want: ""},
		{name: "garbage_no_width", raw: "x1080", want: ""},
		{name: "garbage_bad_width", raw: "abcx1080", want: ""},
		{name: "garbage_negative", raw: "-1920x1080", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, VideoResolutionTier(tt.raw))
		})
	}
}

func TestComputeVideoPriceRatiosForTaskSubmit(t *testing.T) {
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
			name:  "no_table_nil",
			req:   relaycommon.TaskSubmitReq{Duration: 5},
			model: "viduq3-pro-fast",
			now:   peak,
			want:  nil,
		},
		{
			name:  "metadata_duration_float64_wins",
			req:   relaycommon.TaskSubmitReq{Duration: 3, Metadata: map[string]interface{}{"duration": float64(10)}},
			model: "viduq3-pro",
			now:   peak,
			want:  map[string]float64{"seconds": 10, "size": 5.0 / 6.0},
		},
		{
			name:  "metadata_duration_int_wins",
			req:   relaycommon.TaskSubmitReq{Duration: 3, Metadata: map[string]interface{}{"duration": int(12)}},
			model: "viduq3-pro",
			now:   peak,
			want:  map[string]float64{"seconds": 12, "size": 5.0 / 6.0},
		},
		{
			name:  "metadata_duration_string_wins",
			req:   relaycommon.TaskSubmitReq{Duration: 3, Metadata: map[string]interface{}{"duration": "8"}},
			model: "viduq3-pro",
			now:   peak,
			want:  map[string]float64{"seconds": 8, "size": 5.0 / 6.0},
		},
		{
			name:  "metadata_duration_invalid_ignored",
			req:   relaycommon.TaskSubmitReq{Duration: 6, Metadata: map[string]interface{}{"duration": "abc"}},
			model: "viduq3-pro",
			now:   peak,
			want:  map[string]float64{"seconds": 6, "size": 5.0 / 6.0},
		},
		{
			name:  "metadata_resolution_overrides_top_and_size",
			req:   relaycommon.TaskSubmitReq{Duration: 5, Resolution: "720p", Size: "1920x1080", Metadata: map[string]interface{}{"resolution": "540p"}},
			model: "viduq3-pro",
			now:   peak,
			want:  map[string]float64{"seconds": 5, "size": 3.0 / 8.0},
		},
		{
			name:  "top_resolution_beats_size",
			req:   relaycommon.TaskSubmitReq{Duration: 5, Resolution: "720p", Size: "1920x1080"},
			model: "viduq3-pro",
			now:   peak,
			want:  map[string]float64{"seconds": 5, "size": 5.0 / 6.0},
		},
		{
			name:  "size_pixels_anchor_row",
			req:   relaycommon.TaskSubmitReq{Duration: 5, Size: "1920x1080"},
			model: "viduq3-pro",
			now:   peak,
			want:  map[string]float64{"seconds": 5},
		},
		{
			name:  "size_pixels_540p_tier",
			req:   relaycommon.TaskSubmitReq{Duration: 5, Size: "960x540"},
			model: "viduq3-pro",
			now:   peak,
			want:  map[string]float64{"seconds": 5, "size": 3.0 / 8.0},
		},
		{
			name:  "size_garbage_conservative_no_size_ratio",
			req:   relaycommon.TaskSubmitReq{Size: "fhd"},
			model: "viduq3-pro",
			now:   peak,
			want:  map[string]float64{"seconds": 5},
		},
		{
			name:  "seconds_string_fallback",
			req:   relaycommon.TaskSubmitReq{Seconds: "7", Size: "720p"},
			model: "viduq3-pro",
			now:   peak,
			want:  map[string]float64{"seconds": 7, "size": 5.0 / 6.0},
		},
		{
			name:  "duration_saturated_at_max",
			req:   relaycommon.TaskSubmitReq{Metadata: map[string]interface{}{"duration": float64(99999)}},
			model: "viduq3-pro",
			now:   peak,
			want:  map[string]float64{"seconds": relaycommon.MaxTaskDurationSeconds, "size": 5.0 / 6.0},
		},
		{
			name:  "offpeak_default_720p_half",
			req:   relaycommon.TaskSubmitReq{},
			model: "viduq3-pro",
			now:   offpeak,
			want:  map[string]float64{"seconds": 5, "size": 5.0 / 6.0, "time": 0.5},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeVideoPriceRatiosForTaskSubmit(tt.req, tt.model, tt.now)
			assert.Equal(t, tt.want, got)
		})
	}
}

// seedVideoPriceContextModel 写入 off_peak == normal 的独立模型,让依赖
// time.Now() 的 FromTaskContext 断言在任意运行时刻(含北京时间 22:00–8:00 的
// 错峰窗口)都确定。表整体 ReplaceAll,各测试自行 seed 后使用,互不依赖顺序。
func seedVideoPriceContextModel(t *testing.T) {
	t.Helper()
	err := ratio_setting.UpdateVideoPriceByJSONString(`{
	  "ut-video-price-context": {"rows": [
	    {"resolution":"1080p","normal_price":1.0,"off_peak_price":1.0},
	    {"resolution":"720p","normal_price":0.8,"off_peak_price":0.8},
	    {"resolution":"540p","normal_price":0.4,"off_peak_price":0.4}]}
	}`)
	require.NoError(t, err)
}

func TestVideoPriceRatiosFromTaskContext(t *testing.T) {
	seedVideoPriceContextModel(t)
	const model = "ut-video-price-context"

	t.Run("request_from_context", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("task_request", relaycommon.TaskSubmitReq{Duration: 10, Size: "1920x1080"})
		got := VideoPriceRatiosFromTaskContext(c, model)
		assert.Equal(t, map[string]float64{"seconds": 10}, got)
	})

	t.Run("metadata_overlay", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("task_request", relaycommon.TaskSubmitReq{
			Duration: 2,
			Metadata: map[string]interface{}{"duration": float64(9), "resolution": "540p"},
		})
		got := VideoPriceRatiosFromTaskContext(c, model)
		assert.Equal(t, map[string]float64{"seconds": 9, "size": 0.4}, got)
	})

	t.Run("no_table_nil", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("task_request", relaycommon.TaskSubmitReq{Duration: 5})
		got := VideoPriceRatiosFromTaskContext(c, "ut-video-price-context-unconfigured")
		assert.Nil(t, got)
	})

	t.Run("missing_task_request_defaults_without_panic", func(t *testing.T) {
		// 构造上不可达(适配器 Validate* 总是写入请求);断言护栏按缺省参数推导而非 panic。
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		got := VideoPriceRatiosFromTaskContext(c, model)
		assert.Equal(t, map[string]float64{"seconds": 5, "size": 0.8}, got)
	})
}
