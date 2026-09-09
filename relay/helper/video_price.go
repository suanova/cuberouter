package helper

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
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
		// 配置行分辨率同样 trim + 小写归一,避免 " 720p " 这类值匹配不上请求的 "720p"
		if strings.ToLower(strings.TrimSpace(row.Resolution)) != resolution {
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

// VideoResolutionTier 把 OpenAI 风格的尺寸描述("1920x1080" / "1920*1080")
// 或分辨率档位字面量归一到表行使用的档位
// (360p/480p/540p/720p/1080p/4k,另含部分上游特有的 768p/2k 直出档)。
// 尺寸按长边分档:≥3840→4k,≥1920→1080p,≥1280→720p,≥960→540p,≥640→480p,
// 其余→360p。无法解析时返回 ""(调用方保持原值/缺省,按未知分辨率保守计费)。
func VideoResolutionTier(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "360p", "480p", "540p", "720p", "768p", "1080p", "2k", "4k":
		return s
	case "2160p": // 2160p 与 4k 同档
		return "4k"
	}
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == 'x' || r == '*' })
	if len(parts) != 2 {
		return ""
	}
	width, errW := strconv.Atoi(parts[0])
	height, errH := strconv.Atoi(parts[1])
	if errW != nil || errH != nil || width <= 0 || height <= 0 {
		return ""
	}
	switch max := max(width, height); {
	case max >= 3840:
		return "4k"
	case max >= 1920:
		return "1080p"
	case max >= 1280:
		return "720p"
	case max >= 960:
		return "540p"
	case max >= 640:
		return "480p"
	default:
		return "360p"
	}
}

// VideoPriceRatiosFromTaskContext 供 Go 任务适配器(doubao/astraflow)在
// EstimateBilling 中调用:模型命中视频按秒表时,从请求上下文
// (Validate 阶段 storeTaskRequest 写入)推导计费系数;未命中返回 nil。
// 请求缺失(构造上不可达)时按缺省参数推导并 SysError 审计——不能回落 nil,
// 那会退化成"1 秒锚点价"漏计,而按次计费(PerCallBilling)成功任务不做差额结算。
func VideoPriceRatiosFromTaskContext(c *gin.Context, modelName string) map[string]float64 {
	if _, ok := ratio_setting.GetVideoPrice(modelName); !ok {
		return nil
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		common.SysError(fmt.Sprintf("video price estimate: task request missing: %v", err))
		req = relaycommon.TaskSubmitReq{}
	}
	return ComputeVideoPriceRatiosForTaskSubmit(req, modelName, time.Now())
}

// ComputeVideoPriceRatiosForTaskSubmit 是 ComputeVideoPriceRatios 的 Ark 风格
// 包装:除 TaskSubmitReq 顶层字段外,还按 Ark 覆盖语义读取 metadata 中的
// duration/resolution(与 doubao/astraflow convertToRequestPayload 的优先级
// 一致:metadata 覆盖 > 顶层 duration/resolution > size 归一),之后委托
// ComputeVideoPriceRatios 推导计费系数。模型未配置视频按秒表时返回 nil。
func ComputeVideoPriceRatiosForTaskSubmit(req relaycommon.TaskSubmitReq, model string, now time.Time) map[string]float64 {
	if _, ok := ratio_setting.GetVideoPrice(model); !ok {
		return nil
	}
	if duration := metadataDurationSeconds(req.Metadata); duration > 0 {
		req.Duration = duration
	}
	resolution := strings.TrimSpace(metadataResolution(req.Metadata))
	if resolution == "" {
		resolution = strings.TrimSpace(req.Resolution)
	}
	if resolution == "" {
		resolution = VideoResolutionTier(req.Size)
	}
	if resolution != "" {
		// ComputeVideoPriceRatios 优先读 size 字段,把生效档位写回 size 并清空
		// resolution,避免残留顶层值干扰匹配。
		req.Size = resolution
		req.Resolution = ""
	}
	return ComputeVideoPriceRatios(req, model, now)
}

// metadataDurationSeconds 读取 metadata["duration"],兼容 JSON 数值(float64)、
// int/int64 与数字字符串;解析失败或 ≤0 视为未提供(调用方走下一优先级)。
// 上限由 ComputeVideoPriceRatios 内部按 MaxTaskDurationSeconds 饱和。
func metadataDurationSeconds(metadata map[string]any) int {
	raw, ok := metadata["duration"]
	if !ok {
		return 0
	}
	switch v := raw.(type) {
	case float64:
		if v > 0 {
			return int(v)
		}
	case float32:
		if v > 0 {
			return int(v)
		}
	case int:
		if v > 0 {
			return v
		}
	case int64:
		if v > 0 {
			return int(v)
		}
	case string:
		if seconds, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && seconds > 0 {
			return seconds
		}
	}
	return 0
}

// metadataResolution 读取 metadata["resolution"],仅接受非空字符串。
func metadataResolution(metadata map[string]any) string {
	resolution, _ := metadata["resolution"].(string)
	return resolution
}
