package perfmetrics

import (
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
)

func flushLoop() {
	for {
		interval := perf_metrics_setting.GetFlushIntervalMinutes()
		time.Sleep(time.Duration(interval) * time.Minute)
		setting := perf_metrics_setting.GetSetting()
		if !setting.Enabled {
			continue
		}
		flushCompletedBuckets()
		flushCompletedCapacityBuckets()
		cleanupExpiredMetrics(setting.RetentionDays)
	}
}

func flushCompletedBuckets() {
	currentBucket := bucketStart(time.Now().Unix())
	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.bucketTs >= currentBucket {
			return true
		}

		bucket := value.(*atomicBucket)
		drained := bucket.drain()
		if drained.requestCount == 0 {
			deleteOldEmptyBucket(k, key)
			return true
		}

		var channelName string
		if k.channelId > 0 {
			// 经渠道缓存反查展示名（spec §3.1），查不到留空，前端回退 #<id>。
			if channel, err := model.CacheGetChannel(k.channelId); err == nil && channel != nil {
				channelName = channel.Name
			}
		}
		metric := &model.PerfMetric{
			ModelName:      k.model,
			Group:          k.group,
			ChannelId:      int64(k.channelId),
			ChannelName:    channelName,
			BucketTs:       k.bucketTs,
			RequestCount:   drained.requestCount,
			SuccessCount:   drained.successCount,
			TotalLatencyMs: drained.totalLatencyMs,
			TtftSumMs:      drained.ttftSumMs,
			TtftCount:      drained.ttftCount,
			OutputTokens:   drained.outputTokens,
			GenerationMs:   drained.generationMs,
		}
		metric.SetLatHist(drained.latHist)
		metric.SetTtftHist(drained.ttftHist)
		err := model.UpsertPerfMetric(metric)
		if err != nil {
			bucket.addCounters(drained)
			common.SysError(fmt.Sprintf("failed to flush perf metric bucket model=%s group=%s channel=%d bucket=%d: %s", k.model, k.group, k.channelId, k.bucketTs, err.Error()))
			return true
		}

		deleteOldEmptyBucket(k, key)
		return true
	})
}

func deleteOldEmptyBucket(k bucketKey, rawKey any) {
	if k.bucketTs < bucketStart(time.Now().Add(-24*time.Hour).Unix()) {
		hotBuckets.Delete(rawKey)
	}
}

func cleanupExpiredMetrics(retentionDays int) {
	if retentionDays <= 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	if err := model.DeletePerfMetricsBefore(cutoff); err != nil {
		common.SysError("failed to cleanup expired perf metrics: " + err.Error())
	}
	if err := model.DeleteCapacityBefore(cutoff); err != nil {
		common.SysError("failed to cleanup expired capacity metrics: " + err.Error())
	}
}

// redisCounters 解析 Redis 镜像 hash：直方图单元为字段 l{i}/t{i}（i=0..12）。
func redisCounters(values map[string]string) counters {
	c := counters{
		requestCount:   parseRedisInt(values["req"]),
		successCount:   parseRedisInt(values["ok"]),
		totalLatencyMs: parseRedisInt(values["lat"]),
		ttftSumMs:      parseRedisInt(values["ttft"]),
		ttftCount:      parseRedisInt(values["ttft_n"]),
		outputTokens:   parseRedisInt(values["out"]),
		generationMs:   parseRedisInt(values["gen_ms"]),
	}
	for i := 0; i < histCellCount; i++ {
		c.latHist[i] = parseRedisInt(values["l"+strconv.Itoa(i)])
		c.ttftHist[i] = parseRedisInt(values["t"+strconv.Itoa(i)])
	}
	return c
}

func parseRedisInt(value string) int64 {
	if value == "" {
		return 0
	}
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}
