# 流式异常结束部分结算设计

**Date:** 2026-10-08
**Status:** Draft（待评审）
**范围:** 后端 `service/`、`relay/channel/openai/`、`relay/channel/gemini/`、`relay/common/`；无 DB 结构变更，不触 `relaykit/`
**相关:** [流式 vs 非流式差异](../../billing/stream-vs-nonstream.md)（§6 守卫、§11 倒贴面、§12 待决策）、[logs 与对账](../../billing/logs-and-reconciliation.md)

## 1. 问题

流式请求在**异常结束**时（客户端断开、空闲超时、扫描器错误、panic、不完整流），OpenAI 家族的三个 handler 直接返回错误，**跳过计费与消费日志**并全额退还预扣。但此时上游通常已经收到请求、消耗了至少 prompt 的算力并按上游口径计费——差额是纯亏损（`stream-vs-nonstream.md` §11/§12）。

非流式没有这个洞（客户端断连后照常读完并扣费），Claude 原生流与 Gemini 流也没有（它们已经"收到多少算多少"地结算）。**洞只在 OpenAI 家族的异常结束守卫路径上**：`client_gone` 的来源（用户取消、前置 nginx 超时断连、断网）在真实流量里远高于"上游拒绝"，这是唯一系统性、高频、且连日志都不留的出血点。

目标口径（2026-10-05 确认方向）：尽量贴近上游计费，**宁多收不漏收**。

## 2. 非目标（明确不做）

1. **不改非流式**：非流式 usage 缺失的结构性回退（Claude 非流式无估算、OpenAI 非流式只补 `prompt==0`，`stream-vs-nonstream.md` §4.4 / §11）本次不动，留待后续。
2. **不动 `streamErr != nil` 早退**（`chat_via_responses.go:293`、`responses_via_chat.go:167`、`relay-gemini.go:236`）：这一类混合"上游 error 对象"（应退款）与"本地解析/转换失败"（应部分结算）两种语义，逐处判别风险大于收益，本次保持现状（§8 留存差异 D1）。
3. **不动图片流**（`relay/channel/openai/relay_image.go:144-160`）：语义不同（`upstreamFinished`），`stream-vs-nonstream.md` §6 已注明单独评估。
4. **不动 `preconsume-risk-analysis.md` 的待决策补丁**（预扣超扣窗口、批量落库重试）。
5. **不改表结构**：不新增列/索引，不做迁移。
6. **不引入新的计费口径**：结算一律复用 `service.PostTextConsumeQuota`（`service/text_quota.go:396`）现有链路，不新增 quota 计算或转换。

## 3. 现状核实（与 `stream-vs-nonstream.md` §6 表格的差异）

实现与 §6 表格不一致，本设计以代码为准：

| 路径 | §6 表格说法 | 实际行为 | 是否出血 |
|---|---|---|---|
| `OaiStreamHandler` | 有跳过计费守卫 | ✅ 属实：`relay-openai.go:222-231` 两个 return 直接返回错误 | **是** |
| `OaiResponsesStreamHandler` | 有守卫 | ✅ 属实：`relay_responses.go:143-151` 两个 return | **是** |
| `OaiResponsesToChatStreamHandler` | 有守卫 | ✅ 属实：`chat_via_responses.go:299-307` 两个 return | **是** |
| Gemini 流（chat / responses） | 有守卫 | ❌ **不属实**：`geminiStreamHandler` 在 `!IsNormalEnd()` 时只 `LogWarn`（`relay-gemini.go:239-241`）然后照常 `return usage, nil`；`GeminiHelper` 无条件 `PostTextConsumeQuota`（`relay/gemini_handler.go:155`）。**已经按已收内容结算**，只是零分片时 usage 全 0 → `Settle(0)` 全额退（仍写一条 quota=0 的消费日志） | 仅"零分片不收 prompt 估算"一处 |
| Claude 原生流 | 无守卫 | ✅ 属实（`HandleStreamFinalResponse` 补估算后正常结算） | 否 |
| 桥 `OaiChatToResponsesStreamHandler` | 未列 | 无 `StreamStatus` 守卫，照常估算结算 | 否 |

结论：**改动面 = 三个 OpenAI handler 的 6 个守卫 return + Gemini 零分片一处**。

三处守卫里**结构上不可能出现"上游明确拒绝"**：

- HTTP ≥ 400 在进 handler 之前就被拦截并正确退款（`relay/compatible_handler.go:201-206`、`relay/gemini_handler.go:141-146`）。
- 流内 error 对象在两座桥里由 `streamErr != nil` 早退（先于守卫）处理。
- 三个 handler 的扫描回调只用 `sr.Error`（软错误）记录，不用 `sr.Stop`，因此守卫里不会有 `handler_stop`。

即策略 A 中"能确认上游明确拒绝 → 仍退款"这条在本改动面上**由结构保证**，不需要额外判别逻辑。

## 4. 关键决策

| 决策 | 选择 | 理由 |
|---|---|---|
| 零分片策略 | **A：分情况兜底**（2026-10-08 确认） | 客户端断开 / 空闲超时 / 连接中断 / 不完整流 → 至少收 prompt 估算；明确拒绝 → 退款（本面结构性成立）。B 会把"识别不出的拒绝"也收；C 在长思考模型 + 断连场景继续漏计 |
| 结算入口落点 | **handler 内显式调用**共享函数 `service.SettleAbnormalStreamEnd` | `TextHelper` 在 `err != nil` 时丢弃 usage（`relay/compatible_handler.go:209-214`），框架层结算必须依赖"usage+error"语义，会连带结算非流式的 `usage+error`（如 `relay/channel/gemini/relay_responses.go:37-52` 的 blocked/empty 返回 `&usage, err`），风险外溢 |
| usage 来源优先级 | 上游已给的 usage（末帧 / 倒数第二帧回退）→ 已收文本估算 + `toolCount*7` | 与成功路径同一口径（`stream-vs-nonstream.md` §12.2） |
| usage 构建代码 | **把成功路径的尾部提取逻辑抽成函数，正常/异常两条路径共用一份** | 避免两套口径漂移；异常路径不重复实现"末帧 usage → 倒数第二帧 → 估算" |
| 零证据时的行为 | 仍调用 `PostTextConsumeQuota`，由 `hasBillableUsage()==false` → `Settle(0)` 全额退（等价现状，但**多留一条消费日志**） | 与成功路径行为一致（免费模型、`CountToken` 关闭时也会写 quota=0 的消费日志）；同时消灭"连一条 type=2 都没有"的盲区 |
| 结构化标记 | `relayInfo.AbnormalStreamSettled bool` → `other.stream_status.partial_settled = true` | 对账需要区分"异常结束但收了钱"与"正常结算"；沿用既有 `other.stream_status`（`service/log_info_generate.go:153`）与 `RelayInfo` 字段（同 `QuotaClamp`）做法，不改 `PostTextConsumeQuota` 签名（10 个调用点） |

## 5. 不变式

- **I1（无二次退款）**：部分结算完成后，`controller/relay.go:183-192` 的 defer `Billing.Refund` 必须是 no-op。依赖 `BillingSession.Refund` 的 `if s.settled || s.refunded` 短路（`service/billing_session.go:97-104`），实施时不得改动该短路。
- **I2（不重试）**：触发部分结算的守卫错误必须保持 `ErrOptionWithSkipRetry`。重试会复用同一份预扣再次结算 → 双份账。三个 handler 的守卫错误全部来自 `openAIStreamResultError`（`relay-openai.go:107-122`）与 `incompleteOpenAIStreamError`（`:124-132`），均带 SkipRetry，实施时加回归断言防回退。
- **I3（口径单一）**：usage 构建与 quota 计算不得出现第二实现。异常路径复用成功路径抽出的 usage 函数与 `PostTextConsumeQuota` 全链路（`QuotaFromDecimalChecked` / `noteQuotaClamp` / `attachQuotaSaturation`），不引入裸 cast 或新转换（AGENTS.md 计费安全约束）。
- **I4（零证据等价现状）**：没有任何可计费证据时（`CountToken` 关闭导致估算为 0、usage 全 0）→ `Settle(0)` 全额退回，与现状资金结果一致。
- **I5（结算幂等语义不变）**：`Settle` / `Refund` 的既有幂等与状态机不改；本设计只在"是否调用 Settle"上做文章。

## 6. 设计

### 6.1 结算入口（新文件 `service/stream_abnormal_settle.go`）

```go
// SettleAbnormalStreamEnd 在流式异常结束时按现有口径做部分结算。
// usage 为调用方已掌握的最好证据（上游 usage 或已收文本估算）；完全无证据时
// 退化为 Settle(0) 全额退回（等价旧行为），但仍留一条消费日志。
func SettleAbnormalStreamEnd(c *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage)
```

行为：

1. `info.AbnormalStreamSettled = true`（供日志标记）。
2. `PostTextConsumeQuota(c, info, usage, []string{"流式异常结束（<end_reason>），按已掌握用量部分结算"})`。
3. 另打一条 `logger.LogWarn`，带 `end_reason`、估算/上游 usage 来源与最终 quota，便于 ops grep。

零分片口径由调用方自然满足：0 分片时调用方传入的就是 `ResponseText2Usage(c, "", model, info.GetEstimatePromptTokens())`（prompt 估算 + 0 completion）；`CountToken` 关闭时估算为 0 → I4。

### 6.2 站点改动

| 文件 | 位置 | 改动 |
|---|---|---|
| `relay/channel/openai/relay-openai.go` | 守卫 `:222-231`（2 个 return） | 先构建 best-effort usage（**抽出成功路径 `:233-272` 的尾部提取逻辑**：`handleLastResponse` 末帧 usage → 倒数第二帧回退 → `ResponseText2Usage(responseTextBuilder, estimatePrompt)` + `toolCount*7` → `applyUsagePostProcessing`），再 `SettleAbnormalStreamEnd`，最后照常 `returnOpenAIStreamError` |
| `relay/channel/openai/relay_responses.go` | 守卫 `:143-151`（2 个 return） | 抽出 `:153-170` 的补齐逻辑（`CountTextToken` 补 completion、`prompt==0` 补估算、`CloneBillingUsageWithEstimatedCompletion`）供两条路径共用，再结算 |
| `relay/channel/openai/chat_via_responses.go` | 守卫 `:299-307`（2 个 return） | 复用 `:309-313`：`state.Usage()` 为空/零 → `ResponseText2Usage(c, state.UsageText(), ...)`，再结算 |
| `relay/channel/gemini/relay-gemini.go` | `geminiStreamHandler:220-231` | `!hasBillableUsageMetadata` 且 `info.ReceivedResponseCount == 0` **且流异常结束**（`!info.StreamStatus.IsNormalEnd()`）时，用 prompt 估算替代空 usage；**正常结束 + 0 分片（合法空响应）保持 0**。`GeminiResponsesStreamHandler`（`relay/channel/gemini/relay_responses.go:156-161`）只做 `return usage, nil`，自动继承 |
| `relay/common/relay_info.go` | `RelayInfo` | 新增 `AbnormalStreamSettled bool` |
| `service/log_info_generate.go` | `appendStreamStatus:153-178` | `AbnormalStreamSettled` 为真时输出 `partial_settled: true` |

异常路径**不**向客户端补发最终 usage 帧（客户端已断开或流已损坏，错误照常返回）。

### 6.3 日志与审计

- 消费日志（type=2）必定落库：`quota > 0` 是"部分结算"，`quota == 0` 是"无证据、全额已退"。
- `other.stream_status`：既有 `status=error` + `end_reason`；新增 `partial_settled=true` 标识部分结算路径。
- `admin_info.local_count_tokens=true`：`ResponseText2Usage` 既有标记，表示 usage 来自本地估算。
- 估算口径不保证严格 ≥ 上游（断连后上游可能继续生成、缓存 token 我们看不到），只能保证**不再全额退还**。

## 7. 测试

- **service 层**：`SettleAbnormalStreamEnd` 的证据选择——nil/空 usage 时按 prompt 估算兜底、`CountToken` 关闭时为 0（等价现状）、标记位与 `other.stream_status.partial_settled` 落库。复用仓库既有 DB fixture 模式。
- **handler 层**：复用 `relay/channel/openai/relay_openai_stream_test.go`、`relay/channel/openai/relay_responses_billing_test.go` 的 harness，构造 `client_gone` / 空闲超时 / 不完整 EOF，断言：错误仍带 SkipRetry、消费日志落库且 quota 符合预期、0 分片也能收 prompt 估算。
- **回归**：守卫错误的 SkipRetry 属性（防 I2 回退）；正常结束路径的 usage 不变（抽函数后行为等价）。
- 全部使用 `testify` require/assert，表驱动，不写覆盖率型测试（AGENTS.md 后端测试质量）。

数据库：无 schema/迁移变更，不涉及三库矩阵；但结算链路写入 `logs` 表，测试需覆盖真实 SQLite（既有测试基建）。

## 8. 风险与留存差异

- **D1**：`streamErr != nil` 早退（上游 error 对象 / 本地解析失败）仍全额退款——其中"本地解析失败但上游已开工"仍会漏计。
- **D2**：分片级附加计费在异常结束时可能丢失（如 Responses 图片生成计数只在 `response.completed` 提交、未提交的 tool surcharge）。
- **D3**：估算本身可能低于上游真实用量（缓存/reasoning token 不可见），方向是"少收"而非"倒贴"；不保证严格 ≥ 上游。
- **D4**：免费模型与 `CountToken` 关闭场景下会新增 quota=0 的消费日志（与成功路径一致，属可接受的行为变化）。
- **D5**：`stream-vs-nonstream.md` §6 表格需按 §3 修正（Gemini 不是跳过计费），§12 标注为已实现。

## 9. 验收标准

1. `client_gone` / 空闲超时 / `scanner_error` / panic / 不完整 EOF（≥1 分片）→ 消费日志落库、`quota > 0`、`stream_status.partial_settled=true`。
2. 0 分片异常结束（`CountToken` 开启）→ 至少按 prompt 估算收费；`CountToken` 关闭 → 全额退回且资金结果与现状一致。
3. 正常结束（含合法空响应）→ 计费行为与本改动前逐字节一致。
4. Gemini 异常结束 0 分片 → 收 prompt 估算；正常空响应 → 不收。
5. 所有守卫错误保持 `ErrOptionWithSkipRetry`，不触发换渠道重试；结算后无二次退款。
6. `go build ./...`、`cd relaykit && GOWORK=off go build ./...`、相关测试包全绿。
