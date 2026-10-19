# CubeRouter 预扣费机制

> Last Updated: 2026-09-30

## 一、预扣费机制全景

### 1.1 要解决的问题

LLM 中继的费用在请求开始时未知（输出 token 数、流式时长未发生），但"付得起吗"必须在转发前判断。不预扣则余额 1 分的用户可发起百元消费的请求。

预扣 = 转发前按**预估费用**预先划扣，结束后按**实际费用**多退少补，失败时全额退还。

### 1.2 估价（`relay/helper/price.go` `ModelPriceHelper`）

| 模式 | 预扣额 |
| :--- | :--- |
| 按量（ratio） | `(promptTokens + max_tokens × completionRatio) × modelRatio × groupRatio`，`max_tokens`（补全份额）缺省按 `defaultPreConsumeMaxTokens=8192` 兜底（落点 `relay/helper/price.go:133-141`）；`PreConsumedQuota=500` 仍是 prompt 份额的下限（`common/constants.go:143`） |
| 按次（price） | `modelPrice × QuotaPerUnit × groupRatio` + `BillingRatios`（经 `AddOtherRatio` 拒绝非正/NaN） |
| 表达式估价（tiered_expr）| 客户端未传 max_tokens 时用 `defaultPreConsumeMaxTokens=8192` 兜底（`relay/helper/price.go:43`，tiered 用法 `:342`）|
| 免费模型（ratio/price 为 0）| 在 `EnableFreeModelPreConsume` 关闭时估价为 0 并跳过预扣（`controller/relay.go:166-173`）。换算全部走 `QuotaFromFloatStrict`/`QuotaRoundStrict` 饱和转换。|

### 1.3 编排：`BillingSession`（`service/billing_session.go`）

入口 `service.PreConsumeBilling`（`service/billing.go:20`）两道防御：
- `QuotaClamp` 非空（估价已饱和）→ 400；
- 预扣额为负 → 400。随后 `NewBillingSession`（`:357`）按用户计费偏好选**资金来源**：

- 钱包 `WalletFunding` / 订阅 `SubscriptionFunding`（`service/funding_source.go`）；
- `wallet_first`：钱包不足 → 回退订阅；`subscription_first`（默认）：订阅不足且允许 overflow → 回退钱包；
- 钱包前置检查：`userQuota <= 0` 或 `userQuota - preConsumedQuota < 0` → 403（`:370-381`）。

**信任旁路**（`shouldTrust`，`:297`）：非异步任务、阈值 $10（`common/quota.go:3`）、令牌额度（或 unlimited）> $10、钱包 > $10 → `effectiveQuota=0` 跳过预扣；结算无条件扣、余额可为负（`:246-250` 注释）。订阅永不旁路（订阅预扣依赖 `amount>0` 的 requestId 幂等记录锁定订阅）。

**顺序与回滚**（`preConsume`，`:187`）：先扣令牌 → 再扣资金来源；资金来源失败回滚令牌；钱包不足映射 `ErrInsufficientWalletQuota` → `ErrorCodeInsufficientUserQuota`（偏好回退赖此生效）。

### 1.4 原子核心（`model/quota_reserve.go`）

`TryReserveUserQuota` / `TryReserveTokenQuota` 三态结果 OK / Insufficient / Miss：

1. Redis 命中：Lua 原子"校验条目（Id/CacheSchema/字段存在）→ 余额不足返回 0 → `HINCRBY` 扣减返回 1"（`:20-55`）。脚本单线程执行，并发不可能同时通过检查后双双扣负。
2. Miss： hydration（`GetUserCache` / `GetTokenByKey(key, true)`）后**重试一次**。
3. Redis 异常/ hydration失败：降级 DB 条件更新 `reserveUserQuotaDB`（`WHERE id=? AND quota >= ?` + `quota - ?`，`:144-160`），可用性优先。
4. 扣减成功后**同一路径**立即持久化增量：批量模式入队、直写模式 UPDATE（`persistUserQuotaDelta`/`persistTokenQuotaDelta`，`:108-142`）——增量在**预扣时刻**即脱离 Redis。
5. 持久化失败 → **补偿**：`cacheApplyUserQuotaDelta` 把缓存刚扣的加回去，请求失败（`:191-197`）。缓存与 DB 永不单向漂移。

unlimited 令牌跳过余额判定但记账照走；playground 不扣令牌。

\* Hydration

缓存术语：缓存条目缺失（冷/过期/丢失）时，**从 DB 事实源读取快照回填缓存**的标准路径。关键词是**按需重建**——缓存条目全部可从 DB 推导，丢失不需要任何专门恢复逻辑，下一次用到它的请求顺手重建，与日常缓存过期走同一条代码路径。

- 用户：`model/user_cache.go:88` `GetUserCache`——miss/读失败 → `GetUserById` → `populateUserCache` 回填。附加防护：auth version fence（`:102-106`），DB 快照比缓存已记录版本还旧时不允许 hydration，防止把过期快照灌回"复活"本该被限制的用户。
- 令牌：`model/token.go:279` `GetTokenByKey`——miss → DB `First` → `cacheInitToken` 回填。守卫（`:293-295` 注释）：**只在条目不存在时初始化；已存在的 hash 只刷新 TTL，绝不用 DB 快照覆盖**——已存在的余额可能刚被 Lua 原子预扣，覆盖等于把刚扣的钱变回来。 hydration是"只填空、不覆写"。

\* 批量模式

配额变更**不逐笔同步写 DB，先在应用内存累积，由后台协程定期汇总落库**。开关 `BATCH_UPDATE_ENABLED=true`（`main.go:160-164`，默认关），周期 `BATCH_UPDATE_INTERVAL`（默认 5s，`common/init.go:112`）。

- 队列：`model/utils.go:28-36` 按类型分的 8 张 map（用户余额、令牌余额、已用额度、渠道消耗、请求数、prompt/completion/cache tokens 统计），`addNewRecord`（`:47`）锁内累加。
- 落库：`InitBatchUpdater`（`:38-45`）每周期 `batchUpdate()`（`:68`）——先整体换出 map（双缓冲），再把每对 `(id, 净增量)` 写成一条 `quota = quota + ?`。
- 安全前提：**权威余额在 Redis 缓存**（扣减即时 `HINCRBY`），批量只延迟 DB 账本追平。恒等式：`缓存余额 = DB 余额 − 未落库增量`。
- 代价：DB 视图滞后 ≤ 周期；应用崩溃丢队列（未落库增量消失，余额在 hydration时"凭空退回"，方向利用户）；`batchUpdate` 落库失败不重试只记 SysLog（`:103`）。
- 逃生口：`IncreaseUserQuota(id, quota, db bool)` 的 `db=true` 强制直写（充值等必须立刻到账的场景）。


### 1.5 追加预扣 `Reserve`（幂等上调，`:153-179`）

- 分层计费换组：`PrepareTieredBillingForSelectedGroup`（`service/tiered_settle.go:125`）——已有 session 补差，免费组切付费组补建 session；
- 异步任务按上游真实报价补差：`controller/relay.go:755`（`Billing.Reserve`），刻意放在任务落库屏障前（插入失败仍全额可退）。

钱包路径用**无条件全额扣**（不足记欠费不中断），保证日志预扣额与余额实际变动对账一致。订阅追加量单独记 `extraReserved`（退款单独回滚）。

### 1.6 结算与退款

- `SettleBilling`（`service/billing.go:51`）：`delta = actualQuota - preConsumed`，正补扣/负退还/零不动；`Settle` **两阶段提交**（`billing_session.go:42-80`）——先资金后令牌，令牌失败只记日志并标记 settled 防 Refund 误退已提交资金。tiered 求值失败回退按 `FinalPreConsumedQuota` 结算。结算后按剩余额度发预警通知。
- `Refund`（`:83`）：`controller/relay.go:175-184` defer 兜底，幂等、异步；钱包 `IncreaseUserQuota` **非幂等不重试**（多退即资损），订阅 `RefundSubscriptionPreConsume(requestId)` 幂等可重试 3 次。
- 结算/退款侧写路径与预扣对称（`model/user.go:1343+`、`model/token.go:376+`）：缓存 `HINCRBY` + DB 增量入队/直写；token 侧守卫"hash 不存在时跳过"；钱包回充有 `MaxWalletQuota` 上限（`user.go:1372`）。

### 2.7 异步任务

任务类提交前 `ForcePreConsume=true` + 全额预扣（`relay/relay_task.go:341`），禁用信任旁路；轮询完成后 `service/task_billing.go` 结差额，失败全额退。

### 2.8 固化的不变式

| 不变式 | 落点 |
| :--- | :--- |
| 检查与扣减原子，并发不超扣 | Lua / DB 条件更新 |
| 负数、饱和预扣入口即拒 | `PreConsumeBilling` 防御 + model 层 `quota<0` 拒绝 |
| 每步失败必有回滚/补偿 | funding 滚令牌、persist 补缓存、Reserve 互滚、Settle 两阶段、Refund 幂等 |
| 钱包永不溢出 | `MaxWalletQuota` 条件更新 |
| 订阅预扣/退款幂等 | `SubscriptionPreConsumeRecord`（requestId） |
| 全程可审计 | `FinalPreConsumedQuota`/`pre_consumed_quota` 进 consume log；饱和进 `admin_info.quota_saturation` |

## 二、管理员配置手册

### 2.1 配置清单

```
系统设置 → 安全 → 速率限制：
  ModelRequestRateLimitEnabled  = true        # 关键！默认关
  ModelRequestRateLimitDurationMinutes = 1
  ModelRequestRateLimitCount    = 60          # 总请求数/分钟（含失败）必须设，见 6.2
  ModelRequestRateLimitSuccessCount = 60      # 成功数/分钟——信任授信敞口的旋钮
  ModelRequestRateLimitGroup    = {"default":[60,60], "vip":[600,300]}

环境变量：
  RELAY_TIMEOUT=600                   # 单请求总时长上限（http.Client.Timeout，含流式响应体）；默认 0=不限制；不宜过短（§10.5）
  RELAY_RESPONSE_HEADER_TIMEOUT=1800  # 等待上游响应头的上限（默认 1800s，0=无限等）；与 RELAY_TIMEOUT 是两个旋钮
  STREAMING_TIMEOUT=300               # 保持默认：块间空闲超时（每 chunk 即 Reset，不封总时长）
  BATCH_UPDATE_INTERVAL=5             # DB 滞后界：决定 §3.5 ①②③ 窗口的量级（不只是通道②）
  SYNC_FREQUENCY=60                   # 保持默认：= 用户/令牌配额缓存的 TTL，调大直接放大 §3.5-① 的窗口时长
```

**前置条件**：`BATCH_UPDATE_ENABLED=true` 必须配 Redis——代码不校验（`main.go:164-168`），配错会让超扣窗口变常态。

**分组策略**：低余额/低信任分组只放行 CR≈1、输出窗口 ≤8192 的模型（令牌 `model_limits` + 分组渠道白名单），高 CR 大窗口模型留给 VIP。

**限流取值口径**：取值按"不挡正常用量 + 封信任授信"定：

| 分组 | 面向 | 建议 [总数, 成功] | 信任授信敞口（≈ 限流量/分 × 平均时长 × 单笔成本） |
| :--- | :--- | :--- | :--- |
| default | 正常付费 | **[60,60]** | 最坏数十在途 ≈ $6~12，典型明显更低 |
| vip | 重度/团队 | [600,300]（保持） | 设计内 |
| 低信任/新人（示例组） | 试用 | [10,10]（即旧口径） | ≈ $1~2 |

总请求数是含失败的硬帽，重试敏感场景可放宽到 120 而保持成功数 60——**成功数才是敞口旋钮**（失败不产生实际成本）。

### 2.2 观测与告警

| 信号 | 含义 | 动作 |
| :--- | :--- | :--- |
| 日志 `user quota cache reserve unavailable, falling back to database`（`model/quota_reserve.go:184`） | Redis 不可达、预扣正降级走 DB 门——§3.5-② 窗口正在发生 | 出现即关注；持续数秒以上应告警（敞口随降级时长线性放大） |
| 批量落库 `SysError` | DB 写失败、增量已回填等下轮 | 持续出现告警；长期不消说明 DB 写路径有系统性问题 |
| `admin_info.quota_saturation` 日志 | 估价饱和被夹紧，防御生效 | 抽查对应请求的 max_tokens 与参数 |
| 负余额名单 | §4 的应收欠费显性信号 | 定期拉取、追缴或封停（余额 ≤0 自动 403 已内建） |
