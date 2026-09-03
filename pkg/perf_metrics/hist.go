package perfmetrics

// 延迟直方图单元数 = 边界数 + 1 个尾桶。
const histCellCount = 13

// latencyBoundsMs 延迟直方图边界（毫秒）。第 13 单元收纳 >240000ms 的尾部分布。
// 边界为线性扫描规模（13 项），勿改成长表：尾桶分位数近似为 240000。
var latencyBoundsMs = [histCellCount - 1]int64{100, 250, 500, 1000, 2000, 4000, 8000, 16000, 32000, 64000, 128000, 240000}

// histIndex 返回 ms 所属单元：半开区间 [bound[i-1], bound[i])，超过最大边界归尾桶。
func histIndex(ms int64) int {
	if ms <= 0 {
		return 0
	}
	for i, b := range latencyBoundsMs {
		if ms < b {
			return i
		}
	}
	return histCellCount - 1
}

// quantileMs 由单元计数累积分布估算分位数（毫秒）。
// rank 命中单元内部（prev < rank < cum）时取单元下界（lo）作近似；
// 恰好落在单元末尾（rank == cum）时取单元上界（hi），即边界处取值无偏。
// total<=0 返回 -1；分位点落在尾桶时返回 240000（近似上限）。
func quantileMs(q float64, hist *[histCellCount]int64, total int64) float64 {
	if total <= 0 || q < 0 || q > 1 {
		return -1
	}
	rank := q * float64(total)
	var cum int64
	for i := 0; i < histCellCount; i++ {
		cum += hist[i]
		if float64(cum) < rank || hist[i] <= 0 {
			continue
		}
		if i == histCellCount-1 {
			return 240000
		}
		if rank == float64(cum) {
			return float64(latencyBoundsMs[i])
		}
		if i == 0 {
			return 0
		}
		return float64(latencyBoundsMs[i-1])
	}
	return 240000
}
