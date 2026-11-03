# 用户用量页设计（User Usage Page）

**Date:** 2026-11-03
**Status:** Draft — awaiting review
**Branch:** TBD（建议 `feat/user-usage-page`）

## 1. 目标与背景

客户需要一个自助页面，快速回答「我最近用了多少、花了多少、还剩多少」，而不是去逐条翻「用量日志」。

当前现实：

- 站点消费主要发生在**组织账单**上。用户以组织 Key / 组织上下文发起调用，钱由组织出。
- 现有「用量日志」页是逐条明细，面向排查；普通用户不应该被要求去那里做用量查询。
- 组织侧已有「Usage」页（逐条、按 responsible user 过滤），但它是组织管理员视角，不是「我发起的调用」。

因此新增一个用户侧「用量」页，作为客户查看用量的唯一入口；日志页保留，只用于排查明细。

### 口径（已与用户确认）

统计口径是**发起人**：`logs.actor_user_id = 当前用户`，不是现有日志页的 `responsible_user_id` 口径。

- 个人调用：actor = 本人。
- 组织 Key 调用：API Key 不带自然人身份，actor 取该 Key 的责任成员（私有 Key 即持有人本人；公共 Key 下别人用你的 Key 发起的调用也计在你头上）。
- 控制台 / Playground 组织上下文：actor = 登录用户。

这是数据层能做到的最接近「我发起」的口径。

## 2. 已确认的产品决策

1. **时间框**：按天，默认当天。
2. **概览两行**：
   - 个人行：我发起、个人账单的 token / 花费 / 请求数 + 个人剩余额度。
   - 组织行：我发起、所选组织账单的同一组指标 + 我在该组织的可用额度（只显示一个合并后的数）。
   - 组织行带组织下拉，参照钱包 `OrganizationStatsCard`；默认选中当前组织上下文，否则第一个组织。
3. **用量表**：文本 / 图像 / 视频 / 音频 / 向量统一进一张表；按 `日期 × 类型 × 账单归属 × API Key × 模型` 聚合。
4. **表列**：日期、类型、账单归属、API Key、模型、请求数、成功、失败、输入 token、输出 token、缓存 token、消费金额；底部总计行；服务端分页。
5. **表过滤**：模型、API Key、账单归属（个人 / 组织）。
6. **类型来源**：模型标签（导入模型时按 `docs/import-models/model-tags.md` 标注）：
   - `图像生成` → 图像
   - `视频生成` → 视频
   - `音频处理` → 音频
   - `向量` → 向量
   - 无模态标签 → 文本
   - 取不到标签 → 其他
   - MJ 的 `mj_video` → 视频；其余 `mj_*` / `swap_face` → 图像（MJ 模型不在 models 表里）
7. **缓存 token**：`logs` 增加结构化 `cache_tokens` 列，并回填历史数据。
8. **v1 不做**视频专属列（功能类型、使用时长、分辨率）。

## 3. 页面结构

```
时间框（按天，默认今天）
────────────────────────────────────────
概览
  ┌ 个人 ─────────────────────────────┐
  │ 花费 · 请求数 · 输入/输出/缓存 token │
  │ 剩余额度                            │
  └────────────────────────────────────┘
  ┌ 组织 [组织下拉 ▾] ────────────────┐
  │ 花费 · 请求数 · 输入/输出/缓存 token │
  │ 我的可用额度（合并值）              │
  └────────────────────────────────────┘
────────────────────────────────────────
用量表
  过滤：模型 / API Key / 账单归属
  日期 | 类型 | 账单归属 | API Key | 模型 | 请求数 | 成功 | 失败 |
  输入 token | 输出 token | 缓存 token | 消费金额
  总计行
  分页
```

- 路由：`/usage`。
- 导航：General 组新增 `Usage`（用量），放在 `Usage Logs` 之前；日志页保留。
- 移动端：概览卡片纵向堆叠；表格沿用现有 DataTable 的移动端方案。
- 页面状态（时间、过滤、分页）放 URL search，便于刷新/分享，沿用 `useTableUrlState` 模式。

## 4. 时间窗口

- 日期范围选择器，**天粒度**；默认今天（`00:00` 至当前时刻，结束日按全天计算）。
- 后端按站点本地时区解析日期（复用 `parseBillingDateRange` 的语义：`YYYY-MM-DD`，end 当日 23:59:59）。
- 按本地自然日分组：复用 `FLOOR((created_at + 本地 UTC 偏移) / 86400)` 的 day_key 方案（跨 SQLite/MySQL/PostgreSQL 已验证，见 `model/log_billing.go`）。
- **最大跨度 31 天**（与现有账单报表一致）；超出返回 400。
- 日期列直接显示后端返回的 `date`（`YYYY-MM-DD`），不在前端二次本地化，避免服务器/浏览器时区不一致导致日期漂移。

## 5. 概览

### 5.1 数据口径

- actor = 当前用户。
- 个人行：`billing_account_type = personal`。
- 组织行：`billing_account_type = organization AND organization_id = 所选组织`。
- 指标：请求数、输入 token、输出 token、缓存 token、花费（quota）、剩余额度。
- **概览只跟时间框走**，不跟表格的模型 / API Key / 账单归属过滤（见 §13 待确认的调用）。

### 5.2 请求数与 token

- 请求数 = 文本 consume 日志条数 + 任务数 + MJ 任务数；不计违规罚金（fee）。
- 输入 token / 输出 token 来自日志字段；任务/MJ 行没有 token。
- 缓存 token 是输入 token 的子集（输入 token 已包含缓存命中部分），**不要**再加进总量。
- 花费：文本用 consume - refund 净额；任务/MJ 用各自表的最终 quota（详见 §7）。

### 5.3 剩余额度

- 个人：`user.quota`。
- 组织：后端计算并返回一个合并值：
  - 组织余额 `org_remaining = max(0, organization.quota - organization.used_quota)`；
  - 当前用户是 owner：`remaining = org_remaining`（owner 豁免成员限额）；
  - 非 owner 且 `member.quota_limit > 0`：`remaining = max(0, min(member.quota_limit - member.used_quota, org_remaining))`；
  - 非 owner 且 `member.quota_limit = 0`（不限）：`remaining = org_remaining`。
- 同时返回构成字段（`member_quota_limit` / `member_used_quota` / `organization_quota` / `organization_used_quota`），前端放在悬停或副标题里，比如「成员剩余 ¥X / 组织余额 ¥Y」。
- 组织下拉选项来自 `GET /api/organizations`（已带上述字段），默认当前组织上下文，失效时回退第一个组织（与钱包一致）。
- 只有用户属于至少一个组织时才显示组织行。

### 5.4 金额展示

- 接口返回原始 quota，前端复用现有 `formatLogQuota` / `formatQuota` 按站点货币设置展示，避免前后端各算一套。

## 6. 用量表

### 6.1 行定义

一行 = `日期 × 类型 × 账单归属 × API Key（token_id + token_name 快照） × 模型`。

- 账单归属显示 `个人` 或组织名（文本行用日志里的快照 `organization_name`）。
- API Key 按 `token_id` 分组，显示 `token_name` 快照；Key 改名/删除后仍保留历史快照。
- 文本行按日志快照分组：Key 或组织在窗口内改名会拆成两行，各自显示当时的名字；任务/MJ 行没有日志快照，读取时批量补当前名字（见 §7.5）。
- 类型见 §2.6。

### 6.2 列

| 列 | 内容 | 空值规则 |
|---|---|---|
| 日期 | `YYYY-MM-DD` | — |
| 类型 | 文本/图像/视频/音频/向量/其他 | — |
| 账单归属 | 个人 / 组织名 | — |
| API Key | token_name（按 token_id 分组） | 无 Key 时 `-` |
| 模型 | model_name | — |
| 请求数 | 文本 = consume 条数；任务/MJ = 任务数 | — |
| 成功 | 文本 = 请求数；任务/MJ = SUCCESS 数 | fee 行留空 |
| 失败 | 任务/MJ = FAILURE 数 | 文本留空（见 §10/§13） |
| 输入 token | prompt_tokens | 任务/MJ 留空 |
| 输出 token | completion_tokens | 任务/MJ 留空 |
| 缓存 token | cache_tokens | 任务/MJ 留空 |
| 消费金额 | 净额 quota | — |

- 总计行：当前过滤条件下所有行的合计（不是当前页）。
- 排序：日期倒序 → 账单归属 → 类型 → 模型 → API Key。
- 分页：服务端分页，默认 20 行/页；合并三个来源后在内存中排序、分页（31 天窗口下基数可控，见 §7.6）。

### 6.3 过滤

- 模型：文本输入（沿用现有日志页的模糊匹配语义）。
- API Key：文本输入（token_name）。
- 账单归属：下拉 `全部 / 个人 / 组织`；选「组织」时再出现组织下拉（当前用户的组织列表）。
- 过滤只作用于表格和总计行，不作用于概览（§13）。

### 6.4 空态 / 加载

- 无数据："该时间范围内没有用量"。
- 无组织：只显示个人行。
- 加载：卡片与表格 skeleton；过滤变化时表格保持旧数据并显示 fetching 状态。

## 7. 数据源与聚合

### 7.1 数据源

| 源 | 表 | 用途 |
|---|---|---|
| 日志 | `logs`（LOG_DB） | 文本、同步图像/语音、违规罚金的请求数 / token / 花费 |
| 任务 | `tasks`（主库 DB） | 视频/图像/音频任务的提交/成功/失败数与最终花费 |
| Midjourney | `midjourneys`（主库 DB） | MJ 的请求数 / 成功失败 / 最终花费 |
| 模型标签 | `models`（主库 DB） | `model_name → tags`，用于类型分类 |

### 7.2 `logs` 表结构变更

新增两列（需要 SQLite / MySQL / PostgreSQL / ClickHouse 兼容）：

1. `cache_tokens INT NOT NULL DEFAULT 0`
   - 写入：`RecordConsumeLogParams.CacheTokens`；在文本结算处把 `summary.CacheTokens` 带进 params。
   - 旧数据从 `other.cache_tokens` 回填。
2. `usage_source VARCHAR(16) NOT NULL DEFAULT 'text'`
   - 值：`text`（普通 API 调用）、`task`（任务插件计费）、`mj`（Midjourney）、`fee`（违规罚金等非请求扣费）。
   - 写入：
     - `RecordConsumeLogParams.UsageSource` 默认 `text`；
     - 任务提交的 consume 日志显式 `task`；
     - MJ 的 consume 日志显式 `mj`；
     - `RecordTaskBillingLogParams.UsageSource` 默认 `task`，MJ 退款显式 `mj`；
     - 违规罚金显式 `fee`。
   - 作用：
     - 文本聚合只取 `text` / `fee`，排除 `task` 和 `mj`，避免和任务表、MJ 表重复计数；
     - `task` / `mj` 日志仅用于回填时的分类与排除，本页不从它们统计（任务/MJ 的花费与状态以各自表为准）；
     - `fee` 计入花费但不计请求数。
   - 注意：`RecordTaskBillingLog` 当前不写 `ActorUserId`；本页不消费 task/mj 日志，所以不影响统计。若以后要从日志统计任务/MJ，需补写 actor 并回填。

ClickHouse 走 `ALTER TABLE logs ADD COLUMN IF NOT EXISTS`，类型分别为 `Int32 DEFAULT 0` 和 `String DEFAULT 'text'`（参照 `model/main.go` 的 `clickHouseLogScopeColumns`）。

### 7.3 回填

一次性、可恢复、分批执行：

- 扫描 `logs`，解析 `other`：
  - `cache_tokens = other.cache_tokens`（缺失为 0）；
  - `usage_source`（按以下顺序判定）：
    1. 模型名 `mj_%` 或 `swap_face` → `mj`；
    2. `other.is_task = true` 或存在 `other.pre_consumed_quota` 或存在 `other.task_id` → `task`；
    3. `other.violation_fee = true` → `fee`；
    4. 其余 → `text`。
  - 注意：任务退款的 `other` 只有 `task_id` + `reason`（`recordTaskRefundLog`），所以 `task_id` 必须进 task 判定；否则旧退款会被当成 text，产生「负花费、零请求」的脏行。
- 幂等：已回填过的行跳过。SQLite/MySQL/PostgreSQL 用 id 水位（`settings`/`option` 表持久化最后处理的 id）分批推进；进程重启后从水位继续，完成后写完成标记。
- 多节点：回填作为后台任务执行，用 Redis 分布式锁保证同一时刻只有一个节点在跑；拿不到锁的节点直接跳过。没有 Redis 的部署（单机 SQLite）不需要锁。
- ClickHouse：`id` 是读时补的展示 id（默认 0），**不能**用 id 水位。改为一次性 `ALTER TABLE logs UPDATE`（`JSONExtractInt` / `JSONExtractBool`）或按 `created_at` 区间分批；这是一次重 mutation，部署时需要在维护窗口执行。
- 回填范围：全量历史（用户已确认「加列并且回填」）。
- 上线顺序：先部署后端并等回填完成标记；回填完成前不发布/不暴露新页面，避免旧任务日志在 text 行里重复计数。接口响应带 `backfill_complete` 字段，便于确认状态。

### 7.4 文本日志聚合

对 `logs` 做一条聚合 SQL（时间窗口 + actor 过滤）：

```
WHERE actor_user_id = ?
  AND created_at BETWEEN ? AND ?
  AND usage_source IN ('text','fee')
  AND type IN (2,6)                 -- consume, refund；error 不参与
GROUP BY day_key, billing_account_type, billing_account_id,
         organization_id, organization_name,
         token_id, token_name, model_name
```

- `request_count = SUM(CASE WHEN type=2 AND usage_source <> 'fee' THEN 1 ELSE 0 END)`
- `prompt_tokens / completion_tokens / cache_tokens = SUM(CASE WHEN type=2 THEN ... END)`
- `quota = SUM(CASE WHEN type=2 THEN quota WHEN type=6 THEN -quota ELSE 0 END)`（净额）
- 成功 = 请求数（文本不拆失败，见 §13）。
- 模型标签在 Go 里用一次 `SELECT model_name, tags FROM models` 映射，避免逐行查询。

### 7.5 任务 / MJ 聚合

**任务（`tasks`）**

```
WHERE actor_user_id = ?
  AND COALESCE(NULLIF(submit_time,0), created_at) BETWEEN ? AND ?
```

- 读取：id、task_id、token_id、token_name、billing_account_type、billing_account_id、organization_id、organization_name、status、quota、submit_time、properties。
- 模型：解析 `properties.origin_model_name`（Go 侧解析，任务量远小于日志，不做额外的 schema 变更）；为空时归「其他」，模型列显示 `-`。
- 名字：`tasks` 已有 `token_name` / `organization_name` 字段，但当前创建路径没有写。实现时在 `InitTask` 里按 `TokenId` / `OrganizationId` 补快照；历史行读取时按 id 批量查 `tokens` / `organizations` 补当前名字（不追溯改名历史）。
- 按 `day_key × 账单归属 × token × 模型` 聚合：
  - `request_count` = 任务数；
  - `success_count` = `status = SUCCESS`；
  - `failure_count` = `status = FAILURE`；
  - 其他状态（SUBMITTED / IN_PROGRESS / QUEUED / NOT_START）计入请求数，不计成功/失败；
  - `quota` = SUM(task.quota)（最终值，含预扣调整；失败任务已退款则为 0）。

**Midjourney（`midjourneys`）**

MJ 的时间字段是**毫秒**（`submit_time` / `start_time` / `finish_time`），与 `tasks` 的秒不同：

```
WHERE actor_user_id = ?
  AND COALESCE(NULLIF(submit_time,0), NULLIF(start_time,0), finish_time) BETWEEN ? AND ?
```

（参数为 `startTs*1000` / `endTs*1000`；day_key 用选中的毫秒值除以 1000 后再按本地日截断。）

- 模型名：`CovertMjpActionToModelName(action)`（`mj_<action>` / `swap_face`）。
- 状态：SUCCESS / FAILURE 同任务口径；其他状态计入请求数。
- `quota` = SUM(task.quota)（退款后为 0）。
- 名字：读取时批量补 `token_name` / 组织名（表里有字段，缺失则留空）。
- 类型：`mj_video` → 视频；其余 `mj_*` / `swap_face` → 图像。

**类型**：任务模型用模型标签映射；任务模型名应都能在 models 表中找到。

### 7.6 合并与总计

- 三个来源（日志、任务、MJ）各产出一组 `UsageRow`；按 `(date, type, billing_account_type, organization_id, organization_name, token_id, token_name, model_name)` 合并，指标相加。
- 文本行按日志快照分组，Key/组织改名会拆行；任务/MJ 行用读取时补的名字，同一窗口内只按 id 合并。合并键带上名字，保证两类行的行为一致且可预测。
- 合并后在内存中排序、分页，并计算过滤后总计。
- `model_name` / `token_name` 过滤在合并后统一应用（对三个来源都生效）。
- 31 天窗口内聚合行数 = 天数 × Key 数 × 模型数 × 账单归属数，预计在数百到数千行；若实际出现极端基数，再考虑下推到 SQL 分页。

### 7.7 概览聚合

- 同一套来源，但只按账单账户汇总：
  - 个人：`personal`；
  - 组织：所选组织的 `organization`。
- 不做模型/Key/账单归属过滤。
- 组织可用额度公式在 Go 侧计算（§5.3）。

## 8. 后端接口

`GET /api/usage/self`（`middleware.UserAuth()`）

### 8.1 请求参数

| 参数 | 说明 |
|---|---|
| `start` / `end` | `YYYY-MM-DD`，必填；最大 31 天 |
| `model_name` | 可选，模糊匹配 |
| `token_name` | 可选 |
| `billing_account_type` | 可选：`personal` / `organization` |
| `organization_id` | 可选，配合账单归属过滤 |
| `overview_organization_id` | 可选，组织概览行选中的组织 |
| `p` / `page_size` | 分页，默认 1 / 20 |

### 8.2 响应

```json
{
  "success": true,
  "data": {
    "backfill_complete": true,
    "range": {
      "start": "2026-11-03",
      "end": "2026-11-03",
      "start_timestamp": 0,
      "end_timestamp": 0
    },
    "overview": {
      "personal": {
        "request_count": 0,
        "prompt_tokens": 0,
        "completion_tokens": 0,
        "cache_tokens": 0,
        "quota": 0,
        "remaining_quota": 0
      },
      "organization": {
        "id": 1,
        "name": "",
        "request_count": 0,
        "prompt_tokens": 0,
        "completion_tokens": 0,
        "cache_tokens": 0,
        "quota": 0,
        "remaining_quota": 0,
        "member_quota_limit": 0,
        "member_used_quota": 0,
        "organization_quota": 0,
        "organization_used_quota": 0
      }
    },
    "rows": {
      "page": 1,
      "page_size": 20,
      "total": 0,
      "items": [
        {
          "date": "2026-11-03",
          "type": "text",
          "billing_account_type": "personal",
          "organization_id": 0,
          "organization_name": "",
          "token_id": 1,
          "token_name": "",
          "model_name": "",
          "request_count": 0,
          "success_count": 0,
          "failure_count": 0,
          "prompt_tokens": 0,
          "completion_tokens": 0,
          "cache_tokens": 0,
          "quota": 0
        }
      ]
    },
    "totals": {
      "request_count": 0,
      "success_count": 0,
      "failure_count": 0,
      "prompt_tokens": 0,
      "completion_tokens": 0,
      "cache_tokens": 0,
      "quota": 0
    }
  }
}
```

- `type` 用稳定枚举（`text` / `image` / `video` / `audio` / `vector` / `other`）；类型文案由前端 i18n，不依赖后端文案。
- 金额一律返回原始 quota。

### 8.3 校验与权限

- 只返回 `actor_user_id = 当前用户` 的行；不接受任何 `user_id` 参数。
- 组织概览：必须是当前成员；否则 403。
- 表格的组织过滤：API 接受任意 `organization_id`（行本身属于本人，历史组织也算），但前端下拉只列当前所在组织；已退出组织的用量在「全部」里仍可见。
- 日期非法 / 跨度超 31 天：400。
- 频率限制：`CriticalRateLimit()`（与现有日志查询一致）。

## 9. 前端设计

### 9.1 目录

```
web/src/features/usage/
  index.tsx
  api.ts
  types.ts
  components/
    usage-overview.tsx        # 个人行 + 组织行
    usage-organization-card.tsx
    usage-table.tsx
    usage-filter-bar.tsx
    usage-columns.tsx
  lib/
    format.ts
    time-window.ts
```

- 路由：`web/src/routes/_authenticated/usage/index.tsx`。
- 导航：`web/src/hooks/use-sidebar-data.ts` General 组新增项。

### 9.2 复用

- 日期范围：优先复用日期选择组件；若现有 `CompactDateTimeRangePicker` 只能按分钟，封装一个天粒度的包装（不新增第三方依赖）。
- 表格：`DataTablePage` + `useDataTable` + `useTableUrlState`（与组织 Usage 页一致）。
- 金额/数字：`formatLogQuota` / `formatNumber`。
- 组织下拉：参照钱包 `OrganizationStatsCard` + `useWalletOrganizations` 的数据获取方式。
- 过滤栏：`LogsFilterToolbar` / `LogsFilterField`。

### 9.3 i18n

- 新增文案全部走 `useTranslation()`，加到 `web/src/i18n/locales/{en,zh,zh-TW}.json`；类型标签用枚举 + i18n key，不直接渲染后端中文。
- `bun run i18n:sync` 检查缺失 key。

## 10. 边界与已知限制

- **文本类失败数**：错误日志目前不写 `actor_user_id` / 账单归属（`controller/relay.go` 的 `RecordErrorLog` 只传 user/token/model），按 actor 聚合会漏掉，且重试会产生多条错误日志。因此 v1 文本行不展示失败数；任务/MJ 从各自表的 status 取，可靠。
- **LogConsumeEnabled 关闭**：文本依赖 consume 日志，关闭时文本行会空；任务/MJ 表不受影响。页面不做特殊提示（与日志页一致）。
- **旧数据**：`actor_user_id = 0` 的历史行不纳入（组织作用域迁移已把旧行 actor 回填为 user_id）。
- **时区**：分组按服务器本地日；日期列直接显示后端日期，不做前端二次转换。浏览器与服务端时区不同时，「今天」的边界以服务端为准。
- **任务模型名缺失**：`properties.origin_model_name` 为空时归「其他」，模型列显示 `-`。
- **待处理任务**：计入请求数，不计成功/失败；quota 为当前预扣值。
- **退款**：文本花费按 consume - refund 净额（fee 计入）；任务/MJ 用各自表的最终 quota。
- **违规罚金**：`fee` 源计入花费，不计请求数。
- **MJ**：作为任务类来源（`midjourneys` 表），有成功/失败和最终花费；`mj_video` 归视频，其余归图像。
- **任务/MJ 的 Key/组织名**：历史行读取时补当前名字，不追溯改名历史；文本行保留日志快照。
- **模型标签查找**：包含软删除模型（历史用量仍能分类），按精确名匹配；v1 不处理 `NameRule` 通配规则，匹配不到归「其他」。
- **回填期间**：`usage_source` 未回填完的旧任务日志会被当成 text，页面不应在回填完成前发布（§7.3、§13）。
- **模型标签缺失**：归「其他」；不影响金额/数量口径。

## 11. 非目标（v1）

- 视频专属列：功能类型（文生/图生）、使用时长、分辨率。
- 单请求明细下钻（日志页继续承担排查）。
- 导出（Excel/CSV）。
- 跨组织合计行。
- 文本失败数。
- 实时刷新 / 自动轮询。

## 12. 反目标（做成这样就失败）

- 用户为了看基本用量还得去日志页。
- 汇总数字与账单/日志对不上（重复计数、退款没扣、缓存 token 重复加总）。
- 组织行余额与钱包页不一致（owner / 成员限额口径分叉）。
- 页面在 31 天窗口下慢到不可用，或查询打爆日志库。
- 用户能看到别人的用量（actor 过滤或组织校验写错）。

## 13. 待确认的默认值（审阅时可直接否掉）

1. **时间范围上限 31 天**：与现有账单报表一致，防止长跨度聚合压垮日志库。
2. **概览不跟随表格过滤**：概览固定显示个人 + 所选组织的全量汇总；表格过滤只影响表格和总计行。
3. **文本不显示失败数**：原因见 §10；若要做，需要先修错误日志的 actor/账单归属，并处理重试重复计数。
4. **MJ 作为任务类来源**：从 `midjourneys` 表统计成功/失败和最终花费，不再从日志净额（避免退款归属问题）；`mj_video` 归视频。
5. **v1 不做导出**。
6. **已退出组织的用量**：表格「账单归属=组织」的下拉只列当前所在组织；已退出组织的行在「全部」里仍可见，但不能单独筛选。
7. **改名拆行**：文本行按日志快照分组，Key/组织在窗口内改名会拆成两行；任务/MJ 读取时补当前名字。
8. **类型查找**：包含软删除模型，精确名匹配；不处理通配模型规则，匹配不到归「其他」。
9. **回填上线顺序**：先部署后端并等回填完成，再发布页面；回填用 Redis 锁只在单节点执行，ClickHouse 用一次性 mutation。

## 14. 验证计划

### 后端

- 单测：
  - `usage_source` 分类与回填规则（is_task / pre_consumed_quota / task_id / violation_fee / mj 前缀）。
  - 文本聚合：consume/refund 净额、fee 不计请求、缓存 token 汇总、actor 过滤。
  - 任务 / MJ 聚合：SUCCESS / FAILURE / 进行中状态计数，quota 取最终值，Token/组织名补水。
  - 概览可用额度：owner、成员限额 > 0、成员限额 = 0 三种情形。
  - 类型映射：标签 / 缺省文本 / MJ 前缀（含 mj_video）/ 软删除模型 / 缺失标签。
  - 回填：分批推进、水位恢复、幂等、多节点锁跳过。
- 数据库矩阵：SQLite、MySQL、PostgreSQL 跑迁移（新增列、回填幂等、重启两次）与聚合查询；有 ClickHouse 环境时验证 DDL 与查询分支。
- 接口测试：日期校验、31 天上限、actor 隔离、组织权限。

### 前端

- `bun run typecheck`、`bun run build`、改动文件 oxlint。
- 组件测试：组织下拉默认值/回退、类型标签、空态、过滤参数、总计行。
- 手工 e2e（本地测试站）：
  - 一个同时有个人消费和两个组织消费的用户；
  - 固定某一天，页面数字与日志页/账单报表对得上；
  - 组织余额与钱包组织卡一致；
  - 缓存 token 与日志详情里的缓存 token 一致。

## 15. 实施顺序建议

1. `logs` 列 + 写入路径 + 回填（含 ClickHouse）；任务/MJ 名字快照。
2. 聚合 model/service + 接口 + 单测（三库）。
3. 前端页面（概览 + 表格 + 过滤）+ i18n。
4. 本地 e2e 对账 + 文档更新。

## 附录 A：Builder check 问题与处理

独立 reviewer 以「拿到文档就要排期」的视角提了 8 个问题，处理如下：

1. **MJ 退款没有 `actor_user_id`**：`RecordTaskBillingLog` 不写 actor，`applyMidjourneyBillingScope` 也不写；按 actor 聚合会漏掉 MJ 退款、高估花费。
   → **处理**：MJ 不再从日志净额统计，改为从 `midjourneys` 表统计（表里有 actor、status、最终 quota）。日志里的 `usage_source=mj` 只用于分类/排除。因此不需要给退款日志补 actor。
2. **任务退款日志会被回填成 text**：任务退款的 `other` 只有 `task_id` + `reason`，原判定会漏。
   → **处理**：`other.task_id` 已加入 `task` 判定，避免旧退款变成「负花费、零请求」的 text 行。
3. **任务行的 Key/组织名是空的**：`tasks` 有 `token_name` / `organization_name` 字段，但创建路径没写。
   → **处理**：创建任务时补快照；历史任务读取时按 id 批量查 `tokens` / `organizations` 补当前名字（不追溯改名历史）。MJ 同理。
4. **回填怎么跑、要不要 gate 页面**：多节点部署不能每个节点都跑。
   → **处理**：后台任务 + Redis 分布式锁，id 水位持久化；先完成后端回填再发布页面；接口返回 `backfill_complete`。
5. **已退出组织能不能筛选**：组织列表只返回当前成员。
   → **处理**：前端下拉只列当前组织；已退出组织的行在「全部」里仍可见，但不能单独筛选。
6. **软删除 / 通配模型怎么分类**：`models` 有 `DeletedAt` 和 `NameRule`。
   → **处理**：类型查找包含软删除模型（历史用量仍能分类），精确名匹配，活跃行优先；v1 不处理 `NameRule` 通配规则，匹配不到归「其他」。
7. **改名后的行身份**：原合并键漏了 `token_name` / `organization_name`。
   → **处理**：文本行按日志快照分组（改名拆行）；任务/MJ 读取时补当前名字；合并键包含名字。
8. **ClickHouse 回填**：`id` 是读时展示 id，不能用 id 水位。
   → **处理**：ClickHouse 走一次性 `ALTER TABLE ... UPDATE`（或按 `created_at` 分批），在维护窗口执行；id 水位只用于 SQLite/MySQL/PostgreSQL。
