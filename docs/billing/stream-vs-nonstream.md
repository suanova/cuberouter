# 流式 vs 非流式：统计 / 计费 / 日志差异

> Last Updated: 2026-10-08

记录 2026-10-05 的讨论：同一套中继链路在流式与非流式下，统计、计费、日志上的全部行为差异，以及由此暴露的一个已知漏计点（2026-10-08 已修复，见 §12 落地记录）。相关文档：`docs/billing/logs-and-reconciliation.md`（logs 表与对账）、`docs/billing/fixed-rate.md`、`docs/billing/tiered-expr.md`。

## 1. 总览

| 环节 | 非流式 | 流式 |
|---|---|---|
| 预扣 / 退款 | 相同（请求前预扣，出错退） | 相同 |
| 结算入口 | `PostTextConsumeQuota` | 同一入口，但在整个流转发完才调用 |
| usage 来源 | 上游响应 JSON 的 `usage` 字段；缺失时本地估算 | 最后一个 SSE 分片里的 `usage`；没有则按累积文本估算 |
| 失败是否计费 | 报错即错误 → 退预扣 | 另有"流健康度"守卫：超时/断连/不完整流 → **跳过计费与消费日志**（修复前；2026-10-08 起改为部分结算，见 §6 表头说明与 §12） |
| 客户端断连 | 不感知，照常读完并计费 | 感知（client_gone），关上游、**按已掌握用量部分结算**（2026-10-08 前为跳过计费；OpenAI 路径） |
| 日志 | `is_stream=false`，`frt` 是哨兵值 | `is_stream=true`，多 `stream_status`、真实 `frt`、TTFT 指标 |

## 2. 共同的骨架（两者一致）

- 预估 prompt token → `PreConsumeBilling` 预扣（`controller/relay.go:177`），重试循环里只预扣一次。
- 任何错误返回 → `Billing.Refund` + 违规扣费兜底（`controller/relay.go:183-192`）。
- 成功 → 同一个 `service.PostTextConsumeQuota`（`service/text_quota.go:396`）算 quota、`SettleBilling`、写消费日志；日志只有 `IsStream` 字段区分（`service/text_quota.go:549`）。

## 3. usage 来源差异（计费/统计的核心）

**非流式**：解析完整响应体的 `usage`（`OpenaiHandler`，`relay/channel/openai/relay-openai.go:357`）。Responses 协议先经 `NormalizeResponsesUsage` 把 `input_tokens`/`output_tokens` 归一成 prompt/completion（`relaykit/relayconvert/internal/oai_responses/to_oai_chat_resp.go:195-233`）。

**流式**：`OaiStreamHandler` 从最后一条 SSE 分片取 `usage`（`handleLastResponse`，`relay/channel/openai/helper.go:184`），另有"完整 usage 附在倒数第二个事件"的兼容回退（`relay-openai.go:230-252`）。上游始终不给 usage 时按累积文本估算（见 §4.3），并置 `ContextKeyLocalCountTokens` → 日志 `admin_info.local_count_tokens`（`service/usage_helpr.go:23`、`service/billing_usage.go:53`）。

配套差异：

- 请求侧默认希望上游带 usage：`includeUsage := true`（用户显式关掉除外）；只有渠道支持 StreamOptions 且 `constant.ForceStreamOption` 时才注入 `stream_options.include_usage=true`（`relay/compatible_handler.go:50-68`）。
- 客户端要 usage 但上游没给时，网关会**自己合成**一个 usage 分片发出去（`relay/channel/openai/helper.go:203-208`）——客户端看到的 usage 与计费口径一致但可能是估算值。
- Claude / Gemini / Responses 协议流式各有提取路径（`message_delta`、`usageMetadata`、`SetStreamUsage` 载体），非流式统一读响应体 usage。

## 4. 非流式：usage 字段清单与缺失回退

### 4.1 解析落点

`dto.Usage`（`relaykit/dto/openai_response.go:228`）。OpenAI chat 在 `OpenaiHandler` 整体 unmarshal（`relay-openai.go:354`）；之后 `applyUsagePostProcessing` 做渠道级归一化（`relay/channel/openai/usage.go:10-51`）：DeepSeek `prompt_cache_hit_tokens`、智谱/Moonshot 非标准位置、llama.cpp `timings.cache_n` → 统一进 `prompt_tokens_details.cached_tokens`。

### 4.2 结算实际消费的字段（`calculateTextQuotaSummary`，`service/text_quota.go:230-380`）

两层优先：

1. **优先 `usage.billing_usage` 快照**（`effectiveBillingUsage` → `CanonicalUsage`，`service/billing_usage.go:16-21`、`relaykit/dto/billing_usage.go:242`）。存在且语义可识别（openai/anthropic/gemini）时用原始上游方言重算出的 canonical usage 结算，避免转换丢 cache/模态明细；估算补过的带 `Estimated=true`。
2. **否则看顶层 flat 字段**：
   - `prompt_tokens`（或 `input_tokens`）、`completion_tokens`（或 `output_tokens`）——计费主体；
   - `prompt_tokens_details.cached_tokens` → 缓存折扣；`prompt_cache_hit_tokens` 归一到此；
   - `prompt_tokens_details.cached_creation_tokens` / `cache_write_tokens` → 缓存写；
   - `claude_cache_creation_5_m_tokens` / `_1_h_tokens` → Claude 5m/1h 缓存写分开计价；
   - `prompt_tokens_details.image_tokens` → 按 image_ratio 计；`prompt_tokens_details.audio_tokens` → Gemini 音频输入独立单价；
   - `total_tokens` 只进统计展示，不参与 quota 计算；completion 明细（reasoning/text/audio/image）同样只展示；
   - `usage_semantic` 决定 Claude 还是 OpenAI 计价语义（`text_quota.go:386-394`）。

### 4.3 缺失判定与本地估算（OpenAI chat）

触发条件是 **`usage.prompt_tokens == 0`**（不是 usage 为 nil——JSON 里没有 `usage` 也是零值；`relay-openai.go:385`）：

```
completion = usage.completion_tokens
           ?: Σ_choice CountTextToken(message.content + reasoning_content, model)
prompt     = info.GetEstimatePromptTokens()   // 请求阶段已算好的估算
total      = prompt + completion
```

- **completion 估算**：`CountTextToken`（`service/token_counter.go:426-438`）——OpenAI 系模型走 tiktoken 精确数；非 OpenAI 模型走 `EstimateTokenByModel`（`service/token_estimator.go:216`），按厂商（OpenAI/Claude/Gemini 三套系数）的逐字符状态机估算：单词、数字串、CJK、emoji、数学符号、URL 分隔符、换行、空格各权重，最后 `ceil` 求和。
- **prompt 估算**：请求进来时 `EstimateRequestToken` 算好挂在 `RelayInfo`（`controller/relay.go:154-160` → `service/token_counter.go:191-326`）：文本走 `CountTextToken`（或 `TokenTypeTextNumber` 时数字符数），OpenAI 格式再 `+ tools×8 + messages×3 + names×3 + 3`，媒体按图片 tile（上限 1536）/音频 256/视频 8192/文件 4096 加总。**`constant.CountToken` 关闭时估算直接返回 0**（`token_counter.go:182-184`）——此时回退路径 prompt 计 0，只剩 completion 估算。
- 估算结果会**回写进响应体**再发给客户端（`usageModified`，仅 `RelayFormatOpenAI` 分支）。
- 边界：该回退**只**在 prompt==0 时触发；上游给了 prompt>0 但 completion=0 时不再补（completion 记 0 少扣）。流式（`toolCount*7` 兜底）和 Gemini（`patchGeminiZeroCompletionUsage`）才有补丁。

### 4.4 其他协议的非流式回退对照（不一致）

- **Claude Messages 非流式**：`ClaudeHandler` 无估算回退（`relay/channel/claude/relay-claude.go:396-416`）。usage 缺失 → 全 0 → `hasBillableUsage()==false` → 日志"上游没有返回计费信息，无法扣费（可能是上游超时）"，quota 记 0（`service/text_quota.go:377-381, 442-444`）。补估算为流式专有（`HandleStreamFinalResponse`，`relay-claude.go:235-253`）。
- **Responses 非流式**：无回退，`OaiResponsesHandler` 直接返回归一化 usage（`relay/channel/openai/relay_responses.go:21-65`），零值走上面同一条。
- **Gemini 非流式**：有回退——`usageMetadata` 缺失 → `ResponseText2Usage`；metadata 有但 completion 为 0 → `patchGeminiZeroCompletionUsage` 补估算并回填 `BillingUsage`（`relay/channel/gemini/relay-gemini.go:40-65, 111-121`）。

## 5. "解析失败"的准确定义（非流式错误出口）

指**响应报文层面的失败**，handler 返回 `*types.NewAPIError`。以 `OpenaiHandler` 为例（`relay-openai.go:332-448`）：

1. body 读不出来（连接中断/超时）→ `ErrorCodeReadResponseBodyFailed`；上游 5xx 返回 HTML/纯文本也落这条；
2. body 不是合法 JSON → `ErrorCodeBadResponseBody`；
3. body 里是错误对象（`GetOpenAIError().Type != ""`）→ `WithOpenAIError`，即使 HTTP 200 也算错——即上游 4xx/5xx 的**主要识别方式是 body**，这条文本路径没有独立的 `resp.StatusCode >= 400` 检查（状态码只原样透传）；
4. OpenRouter enterprise 包装 `success=false` 或内层解析失败；
5. 罕见分支：估算回写时二次 unmarshal 失败、`force_format` 重新 marshal 失败。

Claude/Responses 同构（`GetClaudeError` / `GetOpenAIError`）。

错误之后的处理顺序：回到 retry loop（`controller/relay.go:241-254`）→ 可重试的**先换渠道重试**（预扣不退、复用）→ 只有最终 `newAPIError != nil` 才由 defer 执行 `Billing.Refund`（幂等，仅未结算时生效，`service/billing_session.go:97-142`）。

**关键区分**：usage 缺失不是解析失败——不是错误，正常结算，`hasBillableUsage()==false` → `Quota=0` → `Settle(0)` 差额全额退回（`billing_session.go:62-80`），日志"上游没有返回计费信息，无法扣费"。

## 6. 流式：结束判定 → 是否计费

流式有 `StreamStatus` 状态机（`relay/common/stream_status.go:12-22`）：done / eof / timeout / client_gone / scanner_error / handler_stop / panic / ping_fail。

**有"异常结束 → 跳过计费与消费日志"守卫的 handler**（2026-10-08 起这些守卫已改为"按已掌握用量部分结算"，见 §12）：

| handler | 位置 | 备注 |
|---|---|---|
| OpenAI chat (`OaiStreamHandler`) | `relay/channel/openai/relay-openai.go:260-271` | 另有"干净 EOF 但无 `[DONE]`/`finish_reason` = 不完整流"一档（:266-271） |
| OpenAI Responses | `relay/channel/openai/relay_responses.go:141-151` | 未收 `response.completed` 的 EOF 同判 |
| Chat→Responses 桥 | `relay/channel/openai/chat_via_responses.go:305-316` | |
| 图片流 | `relay/channel/openai/relay_image.go:144-160` | 语义不同（`upstreamFinished`），单独评估 |

> 修正（2026-10-08）：**Gemini 不在上表**。`geminiStreamHandler` 在 `!IsNormalEnd()` 时只 `LogWarn`（`relay-gemini.go:245-249`），随后照常 `return usage, nil`（`relay-gemini.go:246-250`），`GeminiHelper`（`relay/gemini_handler.go:155`）无条件 `PostTextConsumeQuota` —— 它一直是"收到多少算多少"，只是**零分片**时 usage 全 0 → `Settle(0)` 全额退（另留一条 quota=0 的消费日志）。2026-10-08 已给该零分片分支补上 prompt 估算兜底。

这些错误全部带 `ErrOptionWithSkipRetry`（如 `relay-openai.go:107-122`）→ 不会换渠道重试。非流式没有这一层：body 读全且能解析即算成功。

**已经按"收到多少算多少"结算的**（不在改动面）：原生 Claude 流（无守卫，用 `message_delta` 的 usage 结算，`relay/channel/claude/relay-claude.go:288-309`）、baidu / xai / dify / aws / cloudflare（无守卫，返回估算 usage 后正常结算）。**同一场景 Claude 收费、OpenAI 不收费，协议间已经不一致。**

## 7. 客户端断连

- **流式**：扫描器主循环监听 `c.Request.Context().Done()` → `client_gone`，立即关上游 `resp.Body` 停止上游生成（`relay/helper/stream_scanner.go:303-307`）；已开始写数据后错误以 SSE error 事件收尾（`returnOpenAIStreamError`，`relay-openai.go:149-176`）。
- **非流式**：上游请求不绑客户端 ctx（`relay/channel/api_request.go:319` 用 `http.NewRequest`；只有 Task 路径用 `NewRequestWithContext`，`:595`）。全仓 `c.Request.Context().Done()` 监听点只有流扫描器、SSE ping、plugin 协议桥、task 媒体代理——普通非流式转发一个都没有。客户端断开后网关照常等上游返回、照常扣费并写消费日志（`relay/compatible_handler.go:222` 无条件 `PostTextConsumeQuota`），只是响应写不回（`service/http.go:74-77` 仅记日志）。

## 8. 日志 / 统计字段差异

- `frt`（首字延迟）：`SetFirstResponseTime` 只在流式路径被调用（`stream_scanner.go:266`、图片流、realtime、aws）；非流式保持 `StartTime-1s` 哨兵（`relay/common/relay_info.go:633`），`frt` 恒为 -1000ms（`service/log_info_generate.go:82`）。
- `other.stream_status`（status/end_reason/error_count）仅 `IsStream` 时写入（`service/log_info_generate.go:153-181`）。
- TTFT 性能指标只对流式记录（`pkg/perf_metrics/metrics.go:34`：`info.IsStream && info.HasSendResponse()`）。
- `use_time` 两者都在结算时刻算（`service/text_quota.go:234`）；流式包含整个传输时长，消费日志落库时刻 = 流结束时刻。
- 上游分片计数 `ReceivedResponseCount` 仅流式有（`stream_scanner.go:267`）。
- 组织计费租约续租 `TouchOrganizationBillingSession` 只在流扫描器按 chunk 心跳（`stream_scanner.go:270`；修复协程语义见 `service/organization_billing.go:832-864`）。

## 9. 超时与保活

- 流式：以"两条数据间隔"计的空闲超时（`StreamingTimeout` ticker，`stream_scanner.go:299-300`）、SSE ping 保活、每片延长 write deadline；空闲超时 → `StreamEndReasonTimeout` → 部分结算（2026-10-08 前为跳过计费）。
- 非流式：只有 `http.Client.Timeout`（`common.RelayTimeout`，默认 0 = 不超时，`service/http_client.go:129-130`）和 `RelayResponseHeaderTimeout`（只卡等响应头，`common/constants.go:178-186`）。

## 10. 重试语义

- 开始出流**之前**失败：两者一样走 `shouldRetry` 换渠道重试，共用同一份预扣（`controller/relay.go:204-254`）；SSE 头会被清掉以便返回普通 HTTP 错误（`relay/helper/common.go:64-75`）。
- 出流**之后**：流式错误全部 SkipRetry，不换渠道、退预扣；非流式失败仍可正常重试。

## 11. 倒贴面盘点：哪些路径会净亏损

判断标准：该请求上游**已经开工或已经完成**（上游会把成本记到我们的渠道账户上），而本站最终**记 0 或全额退款**——差额即净亏损。反之，上游明确拒绝且未计费的请求，退款是正确的、不亏。

| 路径 | 上游侧实际发生 | 本站动作 | 是否倒贴 |
|---|---|---|---|
| 非流式：上游 4xx/5xx / body 是 error 对象（§5 第 3 条） | 拒绝请求，通常不计费 | 报错 → 退预扣 | **否**（退款正确） |
| 非流式：读 body 失败 / 非法 JSON（连接中途断、5xx 返 HTML） | 多数未计费；连接中断类可能已部分计费 | 报错 → 退预扣 | 可能小亏（低频、单次） |
| 非流式：成功之后才失败（估算回写二次 unmarshal、`force_format` 重 marshal，`relay-openai.go:384-406`） | 已完成并计费 | 报错 → 退预扣 | **是**（罕见分支） |
| 非流式：usage 缺失 →"上游没有返回计费信息，无法扣费"（§4.4；Claude 无回退，OpenAI 只补 prompt==0 的情况） | 已完成并计费 | `Settle(0)` 全额退（`text_quota.go:377-381`） | **是**（结构性） |
| 流式：client_gone / ping_fail / 空闲超时 / scanner_error / 不完整流（§6 守卫） | 已开工，至少 prompt 被计费；多数按已生成部分计费（口径随上游） | **修复前**：跳过计费与消费日志 + 全额退预扣，**连一条 type=2 都没有**；**2026-10-08 起（PR #134）**：按已掌握用量部分结算并落 type=2 | **是（系统性、高频）→ 修复前**；修复后只剩 §12 的留存差异 |

要点：

- **单次亏损量级** = 上游对该请求的收费（至少 prompt；通常 prompt + 断连前已生成的输出）。总量由频率决定：`client_gone` 的来源（用户取消、前置 nginx/网关超时断连、断网）在真实流量里远高于"上游拒绝"，所以流式守卫是主要出血点。
- **不是垫资，是"该收没收"**：预扣是先扣后退，用户侧没有便宜可占；亏损表现为上游账单有、本站 `logs` 里没有对应 type=2——就是对账文档 `logs-and-reconciliation.md` §7.4 第 2 条写的差额。
- **第三类（等价）：估算不足**。上游给了 usage 但我们语义识别不了（canonical 化失败退回 flat 字段）、或按文本估算少于真实（reasoning/缓存 token 我们看不到）——不算退款，但结果同样"上游计费 > 本站记录"。`billing_usage` 快照机制兜底，不保证 100%。
- **安全的部分**：上游明确拒绝类（错误对象 / 4xx / 5xx）退款不亏，这类占"解析失败"的大多数；非流式的洞主要不在"解析失败"本身，而在"上游完成后才失败"的罕见分支与 usage 缺失的结构性回退。

**精确集合**：会倒贴 = "上游已开工或已完成，但本站最终记 0 / 全额退款"的所有路径。其中流式守卫（含零分片的 client_gone）是唯一系统性、高频、且连日志都不留的一条——**2026-10-08 已由 PR #134 修复（见 §12）**，修复后残余见 §12 的留存差异。

## 12. 已知漏计点（**已修复**，2026-10-08，PR #134 已合并）

> 落地记录：spec `docs/superpowers/specs/2026-10-08-stream-abnormal-settle-design.md`（随 PR 提交）、plan `docs/superpowers/plans/2026-10-08-stream-abnormal-settle.md`（本地工作产物）、PR #134（已合并）。零分片策略**采纳 A**。实现要点：新增 `service.SettleAbnormalStreamEnd`（打标 + 复用 `PostTextConsumeQuota` 全链路），三个 OpenAI handler 的守卫下移到 usage 计算之后并调用它，Gemini 零分片异常结束补 prompt 估算；消费日志新增 `stream_status.partial_settled=true` 供对账区分。产出复核（CodeRabbit + 独立 reviewer）后追加：结算提交失败时消费日志带 `other.settle_failed=true` +"结算失败，额度未实际扣减"文案——覆盖的是 `PostTextConsumeQuota` 一直存在的"settle 失败仍写 quota 日志、而 controller 退款 defer 会把预扣全额退回"的日志/资金漂移（钱是对的，日志会虚记收入）。**留存差异**：`streamErr != nil` 早退（上游 error 对象 / 本地解析失败，`chat_via_responses.go:293`、`responses_via_chat.go:167`、`relay-gemini.go:242`）仍全额退款；非流式结构性回退（§4.4）与图片流未动；"结算失败是否干脆不写消费日志"留待对账口径单独决策。

### 12.1 问题

流式异常结束时（§6 守卫），**（修复前的行为：）跳过计费 + 全额退预扣**，但上游此时通常已收到请求、消耗了（至少 prompt 的）算力并计费——差额 = 纯亏损，完整倒贴面见 §11。用户举的例子：客户端 60s 超时断连、上游还在跑。注意非流式没有这个洞（§7，照常扣费），洞只在流式守卫路径。**2026-10-08 已修复（PR #134）**。

**目标口径（2026-10-05 确认方向）**：尽量贴近上游计费，**宁多收不漏收**（多收走系统外投诉消化，不让平台倒贴）。

### 12.2 改造方向（**已按此实现**，2026-10-08）

在异常结束分支里先按现有口径部分结算、再把错误照常返回：

- **部分 usage 来源**：优先上游已给的 usage（Claude `message_delta`、Gemini `usageMetadata` 中途就有）；OpenAI chat 通常拿不到（usage 在最后一帧）→ 用 `ResponseText2Usage`（请求期 prompt 估算 + 已收文本估算）+ `toolCount*7` 兜底（`relay-openai.go:250-252` 已有同款）。
- **不会有二次退款**：`BillingSession.Refund` 在已结算时是 no-op（`service/billing_session.go:99、151-155`），`controller/relay.go` 退款 defer 不用改。
- **usage 完全不可用时自动等价现状**：`hasBillableUsage()==false` → `Settle(0)` 全额退回（`service/text_quota.go:377-381`）。
- **错误必须保持 `ErrOptionWithSkipRetry`**：已开始出流的错误绝不能换渠道重试，否则下一次尝试复用同一份预扣会出双份账。

### 12.3 零分片策略（2026-10-05 记录，2026-10-08 采纳 **A**）

一个数据分片都没收到就异常结束（长思考模型还没吐字、客户端已断连）时，是否仍按 prompt 估算兜底结算？

- **A. 分情况兜底**（讨论时倾向）：客户端断开 / 空闲超时 / 连接中断 / 不完整流 → 至少收 prompt 估算；上游明确拒绝（HTTP ≥ 400 或 body 是可解析的 error 对象）→ 仍退款。避免"流模式下所有上游 4xx/5xx 都误收"。
- **B. 一律兜底收**：只要请求发到过上游且不是明确成功/失败判定，就至少收 prompt 估算。最简单，配置错误类场景（key 错、模型不存在）会系统性多收。
- **C. 零分片就退**：只有收到 ≥1 个分片才部分计费。最保守，长思考 + 断连场景仍然漏计。

配套口径（一并确认）：估算口径下**无法保证严格 ≥ 上游**（上游可能在我们断连后继续生成、缓存 token 我们看不到），只能保证不再全额退还。
