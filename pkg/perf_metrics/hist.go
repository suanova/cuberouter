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
// 首个满足 cum >= rank 的单元为跨越单元：rank 严格落在单元内部时取单元下界
// （lo）；恰在单元末尾（rank == cum）时取单元上界（hi），边界处取值无偏；
// 分位点越过全部非尾单元（落在尾桶）时返回 240000（近似上限）。
// total<=0 返回 -1；直方图全零但有计数（升级前旧行有计数无直方图列）同样按
// 无数据返回 -1，避免把缺失的直方图错报成尾桶 240s。
func quantileMs(q float64, hist *[histCellCount]int64, total int64) float64 {
	if total <= 0 || q < 0 || q > 1 {
		return -1
	}
	hasCells := false
	for i := 0; i < histCellCount; i++ {
		if hist[i] != 0 {
			hasCells = true
			break
		}
	}
	if !hasCells {
		return -1
	}
	rank := q * float64(total)
	var cum int64
	for i := 0; i < histCellCount; i++ {
		cum += hist[i]
		if float64(cum) < rank {
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
	// 仅当单元计数总和不足 total（调用方数据不一致，如部分写入的行）时可达；
	// 按尾桶近似兜底，与旧行为一致。
	return 240000
}
