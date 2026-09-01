package helper

import (
	"strconv"
	"strings"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// 视频按秒定价的缺省参数(与请求透传缺省保持一致):
// 请求未传时长/分辨率时,计费按 5 秒 / 720p 锚定。
const (
	videoDefaultDurationSeconds = 5
	videoDefaultResolution      = "720p"
)

// ComputeVideoPriceRatios 把任务请求折算为计费系数(seconds × size × time),
// 系数全部来自管理员配置的视频价格表(分辨率表 + 全局错峰窗口):
//   - seconds:请求时长,缺省 5 秒,一律按 MaxTaskDurationSeconds 饱和
//     (时长是用户可控的计费乘子,metadata 等旁路可能绕过请求校验)
//   - size:分辨率相对系数 = 该分辨率正常价 / 锚点(最高正常价行)
//   - time:错峰时段内 = 错峰价 / 正常价(按分辨率),窗口来自 GetOffPeakWindow
//
// 不在配置表内的模型/分辨率按 1.0 保守计费(不产生 size/time 系数)。
func ComputeVideoPriceRatios(req relaycommon.TaskSubmitReq, model string, now time.Time) map[string]float64 {
	table, ok := ratio_setting.GetVideoPrice(model)
	if !ok {
		return nil
	}
	ratios := make(map[string]float64, 3)

	duration := req.Duration
	if duration <= 0 {
		if seconds, err := strconv.Atoi(strings.TrimSpace(req.Seconds)); err == nil {
			duration = seconds
		}
	}
	if duration <= 0 {
		duration = videoDefaultDurationSeconds
	}
	ratios["seconds"] = float64(min(duration, relaycommon.MaxTaskDurationSeconds))

	// 分辨率字段兼容:优先 size(OpenAI 风格),缺省时回退 resolution 字段
	resolution := strings.ToLower(strings.TrimSpace(req.Size))
	if resolution == "" {
		resolution = strings.ToLower(strings.TrimSpace(req.Resolution))
	}
	if resolution == "" {
		resolution = videoDefaultResolution
	}

	anchor := ratio_setting.VideoPriceAnchor(table)
	if anchor <= 0 {
		return ratios
	}
	var sizeRatio, offPeakRatio float64
	for _, row := range table.Rows {
		if strings.ToLower(row.Resolution) != resolution {
			continue
		}
		sizeRatio = row.NormalPrice / anchor
		if ratio_setting.IsOffPeakHour(now, ratio_setting.GetOffPeakWindow()) {
			offPeakRatio = row.OffPeakPrice / row.NormalPrice
		}
		break
	}
	if sizeRatio > 0 && sizeRatio != 1.0 {
		ratios["size"] = sizeRatio
	}
	if offPeakRatio > 0 && offPeakRatio != 1.0 {
		ratios["time"] = offPeakRatio
	}
	return ratios
}
