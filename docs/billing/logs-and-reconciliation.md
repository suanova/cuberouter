# Logs Table & Reconciliation (logs 表与对账)

> Last Updated: 2026-09-30

本文记录 `logs` 表的结构、日志内容的存储方式，以及"用它和上游/渠道对账"的能力边界。用途：设计对账流程、做对账导出、排查账目差异时先读这一篇。

相关文档：计费公式见 `docs/billing/fixed-rate.md`、`docs/billing/tiered-expr.md`；表达式细节见 `pkg/billingexpr/expr.md`。

## 1. 一张表装全部日志

`logs` 是全部日志的唯一落库表（GORM 默认复数表名，无自定义 `TableName`）。日志类型（`model/log.go:156-166`，刻意不用 iota，值不可变）：

| type | 含义 | 主要写入方 |
|---|---|---|
| 0 | Unknown | — |
| 1 | Top-up 充值 | `RecordTopupLog`（`model/log.go:342`，`model/topup.go` 调用） |
| 2 | **Consume 消费** | `RecordConsumeLog`（`model/log.go:589`）——中继请求计费主路径 |
| 3 | Manage 管理审计 | `RecordOperationAuditLog`（`model/log.go:317`，`middleware/audit.go`、`controller/audit.go`） |
| 4 | System 系统 | `model.RecordLog`（注册赠送、签到、2FA 等） |
| 5 | Error 错误 | `RecordErrorLog`（`model/log.go:375`，`controller/relay.go:417` 起） |
| 6 | Refund 退款 | `RecordTaskBillingLog`（`model/log.go:711`，任务/MJ 退款路径） |
| 7 | Login 登录 | `RecordLoginLog`（`model/log.go:291`） |

两个写入开关：

- `LogConsumeEnabled=false` 时**消费日志根本不写**（`model/log.go:593`）。对账前必须确认这段时间它是开着的。
- 用户设置 `RecordIpLog` 关闭时 `ip` 列留空（`model/log.go:609-615`）。

## 2. Schema（`model/log.go:59-103`）

关键列（完整清单见结构体）：

| 列 | 说明 |
|---|---|
| `created_at` | Unix 秒 |
| `type` | 见上表 |
| `user_id` / `username` | 归属用户（快照） |
| `token_id` / `token_name` | 令牌（快照） |
| `channel_id` | 渠道（有索引）；`channel_name` 是 `gorm:"->"` 查询时补齐，不入库 |
| `model_name` | **计费模型名**（`GetBillingModelName`，虚拟定价别名优先，`service/text_quota.go:232`），不等于上游收到的模型名 |
| `prompt_tokens` / `completion_tokens` | 两个独立列，见 §4 语义 |
| `quota` | 本站实扣（quota 单位，$1 = 500,000 quota） |
| `use_time` / `is_stream` | 耗时（秒）/ 流式标记 |
| `group` | 使用分组 |
| `request_id` | 本系统请求 ID（`idx_logs_request_id`） |
| `upstream_request_id` | 上游请求 ID，**只有上游回了 `X-Oneapi-Request-Id` 头才有**（`common/constants.go:197`，`service/http.go:37`）——new-api 系上游才有，直连 Anthropic/OpenAI 基本为空 |
| `other` | JSON 字符串，类型相关细节，见 §3 |
| `scope_type` / `scope_id` | personal / organization |
| `billing_account_type` / `billing_account_id` | 谁付钱（组织请求为 organization） |
| `organization_id` / `organization_name` | 组织（名称是快照） |
| `actor_user_id` / `creator_user_id` / `creator_name` / `responsible_user_id` / `responsible_name` | 操作人/创建人/责任人快照 |
| `organization_billing_session_id` / `_key` | 组织计费会话 |
| `billing_event_key` | 幂等键，唯一索引 `idx_logs_billing_event_key`；同一笔账只落一条，冲突静默跳过（`model/log.go:578-587`） |

组织查询专用复合索引：`idx_logs_org_billing_type_time`、`idx_logs_org_billing_responsible_time`（与唯一索引一起在 `ensureOrganizationBillingLogIndexes` 显式幂等创建，`model/main.go:650`——不用 `uniqueIndex` tag，避免 SQLite AutoMigrate 每次启动重建整张表）。

迁移：`migrateLOGDB`（`model/main.go:620`）。LOG_DB 默认=主库；配 ClickHouse 时走 `migrateClickHouseLogDB`（`model/main.go:713-821`，MergeTree，`PARTITION BY toYYYYMM(created_at)`，`ORDER BY (created_at, request_id)`）。

## 3. `other` 列存什么（按类型）

前端类型定义是完整清单：`web/src/features/usage-logs/types.ts:116`（`LogOtherData`）。

**消费（type=2）**，由 `service/log_info_generate.go` 组装（`GenerateTextOtherInfo:72`，Claude 变体 `:298`，音频/实时 `:286/:274`，阶梯叠加 `:333`）：

- 计费倍率：`model_ratio`、`completion_ratio`、`group_ratio`、`user_group_ratio`、`cache_ratio`、`model_price`、`cache_tokens`、`pre_consume_detail`
- 阶梯计费：`billing_mode="tiered_expr"`、`expr_b64`（base64 表达式）、`matched_tier`、`request_rules`
- 缓存写入（Claude）：`cache_creation_tokens`、`_5m`、`_1h` 及对应 ratio；`cache_write_tokens` 归一化合计（`service/text_quota.go:519-524`）
- 请求特征：`request_path`、`request_conversion`（协议转换链）、`is_model_mapped` + `upstream_model_name`、`reasoning_effort`、`po`（参数覆盖审计）、`claude`、`is_system_prompt_overwritten`
- 流式健康：`stream_status`（status/end_reason/error_count/errors）、`frt`（首字延迟 ms）
- 订阅计费：`billing_source`、`subscription_*`、`wallet_quota_deducted`
- 任务/工具/图片：`is_task`、`task_id`、`usage_facts`、`tool_surcharges`、`web_search*`、`file_search*`、`image*`、`audio_*`
- 违规罚金：`violation_fee`、`violation_fee_code/marker`、`fee_quota`
- 语义标记：`usage_semantic="anthropic"`（仅 Claude 行，`service/text_quota.go:485`）

**admin_info（管理员可见；非管理员查询整块剥离，`model/log.go:189`）**：`use_channel`（重试过的渠道链）、`billing_model`、`conversion_diagnostics`、`is_multi_key`/`multi_key_index`（多 key 渠道的 key 序号）、`channel_affinity`、`usage_billing_path`、`quota_saturation`。

**root_info**：仅 root 可见（非 root 经 `FormatAdminLogs` 剥离，`model/log.go:211`）。

**错误（type=5）**（`controller/relay.go:425-451`）：`request_path`、`error_type`、`error_code`、`status_code`、`channel_id/name/type` + `admin_info`。

**审计/登录/充值**：`op`（action+params，前端 i18n 渲染）、`audit_info`（method/route/status）；充值另有 `admin_info.{server_ip,node_name,caller_ip,payment_method,callback_payment_method,version}`（`model/log.go:342-368`）。

## 4. 输入/输出/缓存 token 的存储与语义

- **输入/输出**：`prompt_tokens` / `completion_tokens` 两列。来源是上游 usage 归一化（Claude `input_tokens`/`output_tokens` → `relay/channel/claude/relay-claude.go:325-326`）。
- **缓存读取**：**没有独立列**，在 `other.cache_tokens`（= `usage.PromptTokensDetails.CachedTokens`，`service/text_quota.go:259`）。来源：OpenAI `prompt_tokens_details.cached_tokens`、Claude `cache_read_input_tokens`、DeepSeek `prompt_cache_hit_tokens`（`relay/channel/openai/usage.go:17`）、智谱/Moonshot 非标准位置提取。
- **缓存写入**：`other.cache_creation_tokens` / `_5m` / `_1h`（Claude 分档）、`other.cache_write_tokens`（归一化合计）。
- **用户累计**：`users.total_prompt_tokens` / `total_completion_tokens` / **`total_cache_tokens`**（每次 LLM 成功请求累加，`model/user.go:1587`）。

### 4.1 口径差异（对账最易踩的坑）

`prompt_tokens` 是否含缓存命中，取决于上游语义（`service/text_quota.go:307-311`）：

- **Claude/Anthropic 语义**：`input_tokens` **不含**缓存读/写，缓存单独计。所以会出现"Cache↓ 144,000 > prompt 1,168"——144k 是缓存命中的前缀，1,168 只是新输入。
- **OpenAI 语义**：`prompt_tokens` **已含**缓存命中，计费时先减去缓存部分再按 `cache_ratio` 单算。
- OpenRouter 转 Claude 的特殊路径还会把缓存从 prompt 里减掉（`service/text_quota.go:270-280`）。

界面上（使用日志 Tokens 列，`web/src/features/usage-logs/components/columns/common-logs-columns.tsx:657-697`）显示为 `prompt / completion`，缓存有值时下面追加一行 `Cache↓ {读} ↑ {写}`。

## 5. 读取路径与权限

- `GET /api/log`（OpsAuth，全量）与 `GET /api/log/self`（UserAuth，个人）——`router/api-router.go:375-383`；聚合在 `model.GetAllLogs` / `model.GetUserLogs`（`model/log.go:777/873`），查 `LOG_DB`。
- 个人视图用 `personalLogScopeQuery` 排除非 personal 作用域（`model/log.go:147-153`）——**组织消费不会出现在个人视图**。对渠道/模型总账用全量接口，不走 self。
- 组织消费明细：`GET /api/organizations/:id/logs`（`service/organization_log.go`），查的是**同一张 `logs` 表**，过滤 `billing_account_type='organization' AND billing_account_id=orgId AND organization_id=orgId`（`:191`），且接口会清空 `content`（`:71`）。
- 组织审计日志是**另一张表** `organization_audit_logs`（`service/organization_audit_query.go:71`，主库 `DB`），与消费日志无关。

## 6. 保留与清理

**默认永久保留，无自动清理。** 删除只有两个途径：

1. 手动：系统设置 → 日志维护 → 清理历史日志（root 触发一次性任务，`POST /api/system-task/log-cleanup`，`controller/system_task.go:14`；任务类型 non-scheduled，`service/system_task.go:77`）。删除目标时间戳前的**全部类型**日志（`model.DeleteOldLogBatch`，`model/log.go:1041`），不可恢复。UI 选择器默认预选"30 天前"，仅为默认值。
2. ClickHouse 日志库且有 `LOG_SQL_CLICKHOUSE_TTL_DAYS>0`（`model/main.go:758`）；默认 0 = 无 TTL。

手动清理只删 `logs`；`quota_data`（用量看板）、组织账本等表不受影响。

## 7. 和上游/渠道对某个模型对账：现有 logs 够不够

目标：给定时间段，对某个 `channel_id` 的某个 `model_name`，核出输入/输出/缓存 token 及对应计费。

### 7.1 直接满足（列/SQL）

```sql
-- type=2 消费日志；channel_id、model_name、created_at 均有索引
SELECT model_name, channel_id,
       COUNT(*)                   AS requests,
       SUM(prompt_tokens)         AS prompt_tokens,
       SUM(completion_tokens)     AS completion_tokens,
       SUM(quota)                 AS quota
FROM logs
WHERE type = 2
  AND channel_id = ?
  AND model_name = ?
  AND created_at BETWEEN ? AND ?
GROUP BY model_name, channel_id;
```

过滤口径提醒：`model_name` 是**计费模型名**；上游收到的名称在 `other.upstream_model_name`（仅映射时记录，`service/log_info_generate.go:89-92`），或查渠道 model_mapping 配置反推。

### 7.2 需要解析 `other`（不能 SQL 聚合）

缓存读/写 token 和各类倍率都在 `other` JSON 里。**跨库（SQLite/MySQL/PG）JSON 提取被刻意避免**——`model/log_billing.go` 的账单聚合（`GetUserBillingAgg`）因此把 `cache_tokens` 留 0 占位，调用方不应把它当实测值。对账需要拉行后在应用层解析 `other`，或做专门的导出作业。

### 7.3 计费拆分：部分满足

- `quota` 是该请求**实际扣费的总额**，精确。
- 标准倍率计费的请求，可用 `other` 里的倍率重算各类别金额：输出 = `completion × completion_ratio × ratio`；缓存读 = `cache_tokens × cache_ratio × ratio`；输入 = （按语义剔除缓存后的）`prompt × ratio`；其中 `ratio = model_ratio × group_ratio`。
- **拆不出的三类**：
  1. `billing_mode=tiered_expr`：按表达式整体计费，只有 `expr_b64`/`matched_tier` 快照，要重算需按表达式再跑一遍；
  2. `model_price>0` 按次计费：没有按 token 的单价；
  3. 附加费：`tool_surcharges`、图片/音频单独价、违规罚金——各自独立字段。
- **`quota` 是本站售价**（含分组倍率）。对上游账单（上游成本）时最稳的做法：**token 数直接对；金额用上游价目 × 本站记录的 token 数重算后再对**，不要拿 `quota` 直接比上游金额。

### 7.4 其他对账差异来源

1. **多 key 渠道**：上游按 key 出账，logs 只到渠道级；key 序号在 `other.admin_info.multi_key_index`（admin 字段）。
2. **重试**：一次请求试过多个渠道时，只有最终成交渠道记 `quota`；失败渠道是 type=5 错误日志（quota=0）。上游若对中断流计费，会产生差额——**2026-10-08 起流式异常结束改为按已掌握用量部分结算并落 type=2**（PR #134，见 `stream-vs-nonstream.md` §12），这条差额只剩两个留存口：`streamErr != nil` 早退（上游 error 对象/本地解析失败）与非流式 usage 缺失回退；另注意新增的 `other.stream_status.partial_settled=true`（异常结束但已计费）与 `other.settle_failed=true`（结算提交失败、钱未收到，日志会虚记收入）两个标记。
3. **逐笔核对钩子弱**：`upstream_request_id` 仅 new-api 系上游回传，直连上游为空，一般只能聚合对，不能逐笔对。
4. **组织请求**：总量不受影响；按用户维度切分时要按 `billing_account_type='organization'` 过滤，别按 `user_id`。
5. **前置条件**：`LogConsumeEnabled` 开着、时间段在保留期内（否则行缺失）。
6. **时区**：`created_at` 是 Unix 秒；上游账单一般按 UTC 日，按需换算。

### 7.5 与其它账表的关系（谁是权威源）

| 对什么 | 权威来源 | logs 的角色 |
|---|---|---|
| 每笔请求扣了多少、谁花的 | **logs（type=2）** | 主依据 |
| 组织余额变动 | `organization_billing_records`（带 `quota_before/after` 前后余额快照，`record_key` 唯一幂等）+ `organization_billing_sessions`（`model/organization_foundation.go:155`、`:116`） | 同一笔账的镜像；组织是先结算、后由日志回填 token 数（`syncOrganizationBillingRecordUsageFromConsumeLog`，`model/log.go:544`） |
| 个人钱包 | `users.quota` 变动 + `topups`（支付订单） | type=1 只是事件记录，不是收款凭证 |
| 用量看板聚合 | `quota_data`（DataExport，`RecordConsumeLog` 内 `LogQuotaData`） | 聚合副本，非账本 |

## 8. 做渠道×模型对账的推荐姿势

1. **SQL 聚合**（§7.1）：按 `channel_id + model_name + created_at` 出 requests/prompt/completion/quota。
2. **解析 `other`**：同批行导出后在应用层汇总 `cache_tokens`、`cache_creation_tokens*`、各类倍率与 `is_model_mapped`/`upstream_model_name`、`billing_mode`；按 `usage_semantic` 选择"非缓存输入"的算法。
3. **金额**：token 数与上游逐项对齐；金额用上游价目重算后再对上游账单。拆分金额时先按 §7.3 排除 tiered/按次/附加费行，单独处理。
4. **要做成常规能力**：仿照 `GetUserBillingAgg`（`model/log_billing.go`）加"按渠道+模型+日"的导出/聚合接口，把缓存与倍率字段带出；缓存列若需 SQL 可聚合，需要考虑落独立列（当前 schema 没有）。
