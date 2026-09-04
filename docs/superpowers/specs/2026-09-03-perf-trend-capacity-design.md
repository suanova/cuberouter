# 性能趋势与容量分析 — 设计文档

日期：2026-09-03
状态：已与需求方逐节确认（数据模型与采集管道 / 查询 API 与前端 / 薄导出端点与配置迁移测试）

## 1. 目标与范围

在 cuberouter 内置「性能趋势 + 容量分析」，并保留对接第三方运维栈（Prometheus / k8s）的能力：

1. **性能趋势**：每个模型/分组/渠道的延迟分位数（P50/P95/P99）、TTFT、成功率、TPS 随时间的走向，定位上游劣化；
2. **容量分析**：网关级 RPS、在途并发峰值、过载 503 拒绝率，判断何时加实例/调阈值；
3. **薄导出**：同源内存指标以 Prometheus 文本格式暴露 `/api/metrics`，供 k8s/Prometheus 抓取，内置分析不依赖外部组件。

### 范围外（明确不做）

- 不引入外部时序存储；分析数据放 PG/SQLite 主库（沿用 perf_metrics 现状）；
- 不做 Prometheus 精确分位数、不做告警规则投递；
- Dashboard 总览页 v1 不改（已有 24h 摘要面板 PerformanceHealthPanel）；
- 渠道级指标不进入 Prometheus 导出（高基数），仅内置分析可用。

## 2. 现状基础（已核实）

- `pkg/perf_metrics`：内存原子桶（键 model/group/bucket_ts）→ 周期 flush（`flushLoop`，间隔可配，默认 5min）→ DB `perf_metrics` 表按键增量 upsert；查询 = DB 行 + 热点内存桶 + Redis 活跃桶三方合并。
- 桶粒度可配：minute / 5min / hour（默认 hour，`perf_metrics_setting.bucket_time`）；保留 `retention_days`（默认 0 = 不过期），清理挂 flushLoop（`cleanupExpiredMetrics`）。
- Redis 镜像仅做「当前活跃桶」的崩溃兜底（`recordRedis` / `mergeRedisActiveBuckets`，1h TTL，Redis 未启用时全跳过）。
- 采样点 `RecordRelaySample(info, success, outputTokens)` 三个调用点：成功在 `service/quota.go:387`、`service/text_quota.go:543`，失败在 `controller/relay.go:255`。`RelayInfo.ChannelId`（`relay/common/relay_info.go:60`）可取得，失败分支可能为 0（未选到渠道）。
- 过载保护 `middleware/performance.go SystemPerformanceCheck` 挂在所有 relay 路由组上，超阈值返回 503（`system_cpu_overloaded` 等），此时发生在解析 model 之前 → 拒绝无法按 model 归因。
- `seriesSchema` 常量是前端响应格式版本标记，payload 语义变化需核对解码逻辑。

## 3. 数据模型

### 3.1 perf_metrics 表扩展

键与索引：唯一索引 `(model_name, group, channel_id, bucket_ts)`（替换现有 `(model_name, group, bucket_ts)`）。旧行 channel_id = 0，不与新键冲突。

新增列：

| 列 | 类型 | 说明 |
|---|---|---|
| `channel_id` | int | 索引键的一部分 |
| `channel_name` | varchar | flush 时经渠道缓存反查的展示名，改名不追溯 |
| `lat_b0` … `lat_b12` | int64 | 总延迟直方图单元计数（含尾桶） |
| `ttft_b0` … `ttft_b12` | int64 | TTFT 直方图单元计数（含尾桶） |

直方图边界常量 `latencyBucketBoundsSec`（放 `pkg/perf_metrics`，单一定义源）：

```
[0.1, 0.25, 0.5, 1, 2, 4, 8, 16, 32, 64, 128, 240] 秒 + 尾桶 >240s
```

归属规则：值 v 落入第一个满足 `v < bound` 的单元（半开区间 `[bound[i-1], bound[i])`），超过最大边界落入尾桶。每次采样只给所在单元 +1，与现有 sum/count 完全同构地增量 upsert。

### 3.2 capacity_metrics 表（新建）

键：`bucket_ts`（与 perf_metrics 同桶对齐）。

| 列 | 说明 |
|---|---|
| `bucket_ts` | 主键 |
| `attempts` | 进入 relay 路由组的请求数（含未过鉴权/被拒），增量 upsert |
| `rejected_503` | SystemPerformanceCheck 判定过载返回的 503 数，增量 upsert |
| `inflight_peak` | 桶内并发峰值，**取 max** |

`inflight_peak` 跨实例合并是 `(bucket_ts)` 上的多写，取 max 采用「条件 UPDATE（`WHERE inflight_peak < ?`），影响行数为 0 时重读决定是否 INSERT/跳过」，不用 `GREATEST`/`max()`（避开 SQLite/MySQL/PG 方言差异，见 AGENTS.md 跨库约束）。峰值为近似值（采样粒度 2s），语义在文档注明。

### 3.3 内存结构

- `bucketKey` 增加 `channelId int`；`counters` 与 `atomicBucket` 增加 `latHist [13]int64`、`ttftHist [13]int64`（add / drain / snapshot / addCounters / mergeCounters 逐元素对称扩展）。
- 网关级容量桶：进程内单桶（当前 bucket_ts 一个），结构与表列一致，2s 采样循环把 in-flight gauge CAS 进峰值，flushLoop drain 后按上述语义 upsert。
- Redis 镜像同步扩展（键含 channel、hash 字段含直方图各单元），仅 Redis 启用时生效，保持对称。

## 4. 采集管道

1. **采样**：`Sample` 增加 `ChannelId`。三个既有 `RecordRelaySample` 调用点不变；失败分支 channel=0 归入「未分渠道」（channel_id=0 行仍参与 model/group 汇总，渠道视图隐藏）。
2. **并发 gauge**：新 `middleware.RelayCapacity()` 挂 relay 路由组根（与现有 StatsMiddleware 同层）：进入 `inflight.Add(1)`，结束 defer `-1` 且 `attempts.Add(1)`。不细分 per-channel 并发（见范围外）。
3. **503 计数**：`SystemPerformanceCheck` 返回过载错误前回调 `perfmetrics.RecordOverloadReject()`（middleware 引 perfmetrics，无循环依赖）。
4. **flush**：现有 `flushLoop` 扩展为同时 drain perf 桶与容量桶、清理过期数据（`cleanupExpiredMetrics` 覆盖两张表）。
5. **直方图始终采集**：无开关，每次采样为两次边界查找 + 原子加。

## 5. 查询 API

### 5.1 既有端点加法扩展（向后兼容）

- `GET /api/perf-metrics?model=&group=&hours=`：
  - `BucketPoint` 保留旧字段，新增：`request_count`、`p50_latency_ms`、`p95_latency_ms`、`p99_latency_ms`、`p95_ttft_ms`（无数据为 -1）；
  - `GroupResult` 新增 `channels: [{ channel_id, channel_name, series: BucketPoint[] }]`。
- `GET /api/perf-metrics/summary?hours=`：形状不变。
- 分位数采用**单元跨越估计**（`pkg/perf_metrics/quantile.go`，落在 `pkg/perf_metrics/hist.go`）：对累积计数首次 ≥ rank 的单元，rank 严格落在单元内部 → 返回单元下界；rank 恰为单元末累计 → 返回单元上界；分位点越过全部非尾单元 → 返回 240000。误差 ≤ 单元宽（同值样本集中于单元下界时精确）。不做单元内线性插值——插值会在真实分布集中于单元边界时系统性偏移（如全 1000ms 的样本会报出 1500ms）。
- plan 阶段核对前端对 `seriesSchema` 的缓存解码逻辑，必要时 bump。

### 5.2 新端点

- `GET /api/capacity-metrics?hours=` → `{ series: [{ ts, attempts, rejected_503, inflight_peak }] }`。`hours` 沿用 30 天上限。RPS = attempts / 桶秒数，由前端换算。
- `GET /api/metrics`（见 §6），独立路由，不走后台登录鉴权。

## 6. 薄 Prometheus 导出

- **进程级累积注册表**（与桶同一边界的直方图累积计数 + 计数器，`Record()` 时同步累加）；重启归零符合 counter 语义。多实例由 Prometheus 按 pod 分别抓取。
- **手写 exposition 渲染器**，零新依赖（不引 client_golang）：`# HELP` / `# TYPE` + `name{labels} value` 行，Content-Type `text/plain; version=0.0.4`。数据直读共享注册表，不维护两套记录器。
- 指标集：
  - `cuberouter_relay_requests_total{model,group}`
  - `cuberouter_relay_latency_seconds_bucket{model,group}` / `_sum` / `_count`
  - `cuberouter_relay_ttft_seconds_bucket{model,group}` / `_sum` / `_count`
  - `cuberouter_inflight_requests`（gauge）
  - `cuberouter_overload_rejects_total`
  - `cuberouter_relay_attempts_total`
  - Go runtime 子集：goroutines、MemStats 关键项
  - 导出不含 channel 维度（高基数，见范围外）
- 鉴权：`perf_metrics_setting` 增 `export_enabled`（默认 false）、`export_token`（默认空）；token 非空时校验 `Authorization: Bearer`。无 token 时文档注明依赖网络层限制。

## 7. 配置

`perf_metrics_setting`（沿用现有设置注册机制）新增：

- `export_enabled`：bool，默认 false
- `export_token`：string，默认空

桶粒度、flush 间隔、保留天数沿用现有配置；容量采样周期固定 2s 不设配置。

## 8. 迁移与保留

- `model/main.go` 迁移流程追加（全部遵循跨库模式，SQLite 走 `ADD COLUMN` 路径，索引用 `commonGroupCol` 等）：
  1. `perf_metrics` 加列：`channel_id`、`channel_name`、`lat_b0..lat_b12`、`ttft_b0..ttft_b12`；
  2. drop 旧唯一索引 `idx_perf_model_group_bucket`，建新 `(model_name, group, channel_id, bucket_ts)`；
  3. 建 `capacity_metrics` 表。
- 保留清理：`cleanupExpiredMetrics(retentionDays)` 同时清理两表。文档建议生产设置 `retention_days`（现状默认 0 = 永久保留）。
- 迁移幂等性随测试验证（重复执行无副作用）。

## 9. 前端

性能监控页（`web/src/features/performance-metrics`）扩展，不加新导航，文案走 zh/en i18n：

1. **趋势图增强**：模型级趋势线固定叠加 P95/P99 两条附线（不加开关/偏好机制——P50 与均值视觉近似，需要时再补；图表库沿用该图表现有 VChart 封装并遵守 web/AGENTS 约定）；
2. **渠道维度**：管理端（admin 门控）「渠道明细」卡片——渠道名与可用性属内部信息，不得出现在对公开访客开放的模型广场（pricing）页面；卡片内模型下拉 + 同模型各渠道成功率、延迟、TPS 明细表；
3. **容量区块**：同页新增 RPS / 在途并发峰值 / 503 拒绝率三条时间线，叠加过载阈值参考线（阈值来自现有 `/api/performance/stats` 的 monitor 配置）。

Dashboard 总览页 v1 不改。

## 10. 测试

- `pkg/perf_metrics`：直方图单元归属（半开区间、尾桶）、drain/snapshot/merge 对称、分位数插值表驱动（已知分布 → 期望近似区间）、空桶/全尾桶。
- model 层：upsert 冲突累加含新列；capacity 峰值条件 UPDATE 的取 max 语义；迁移幂等。
- 导出渲染：行格式正则校验（`name{labels} value`）、数值正确、counter 语义。
- controller/router：新 handler 参数校验（hours 上限）、`/api/metrics` 鉴权分支；仿现有 `router/*_test.go` 风格（testify require/assert）。
- 前端：确认测试设施后仅补纯函数（格式化）测试；图表组件跟随既有模式。

## 11. 关键文件清单

| 文件 | 改动 |
|---|---|
| `pkg/perf_metrics/types.go` | Sample/键/计数器/原子桶扩展、边界常量 |
| `pkg/perf_metrics/metrics.go` | Record、Query 合并、bucketKey |
| `pkg/perf_metrics/flush.go` | flushLoop、容量桶 drain、双表清理 |
| `pkg/perf_metrics/quantile.go` | 新：分位数插值 |
| `pkg/perf_metrics/export.go` | 新：进程注册表 + exposition 渲染 |
| `controller/perf_metrics.go` | 新查询参数与响应扩展 |
| `controller/capacity_metrics.go` | 新 handler |
| `controller/metrics_export.go` | 新：/api/metrics handler |
| `model/perf_metric.go` | 新列、新索引、capacity upsert、GetCapacityMetrics |
| `model/main.go` | 迁移步骤 |
| `middleware/performance.go` | 503 计数回调 |
| `middleware/*`（新 `relay_capacity.go`） | RelayCapacity 并发计数 |
| `router/api-router.go` | capacity-metrics 路由、/api/metrics 路由 |
| `controller/perf_metrics.go` | 新查询参数与响应扩展 |
| `controller/capacity_metrics.go` | 新：/api/capacity-metrics handler |
| `setting/perf_metrics_setting/config.go` | 两个新配置键 |
| `service/quota.go` `service/text_quota.go` `controller/relay.go` | Sample 增加 ChannelId 透传 |
| `web/src/features/performance-metrics/*` | 趋势图、渠道、容量区块 |
| `web/src/features/*/api.ts types.ts` | 新响应类型 |
