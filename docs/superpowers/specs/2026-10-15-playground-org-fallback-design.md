# 游乐场组织额度兜底设计

**Date:** 2026-10-15
**Status:** Draft（待评审）
**范围:** 后端 `middleware/`、`service/`、`controller/`、`router/`、`setting/`、`relaykit/dto/`；前端零改动；无 DB 结构变更（`users.setting` 为 JSON 列，加字段零迁移）
**相关:** [组织计费账本](../../../service/organization_billing.go)、[组织令牌鉴权](../../../service/organization_token_auth.go)、[游乐场入口](../../../controller/playground.go)、[组织分组](../../../service/organization_group.go)

## 1. 背景与目标

游乐场 `/pg/*` 目前是「登录会话 + 个人临时 token」的纯个人钱包计费（`controller/playground.go:72` `preparePlaygroundContext`）：个人额度不足时直接报「用户额度不足」，即使该用户是组织成员、组织有余额、账本链路（`OrganizationFunding`）完全现成。

**目标（2026-10-15 确认）**：个人钱包余额低于阈值时，游乐场请求自动改以**组织身份**计费：

- 语义：选组织、不选 key——后端校验调用者是该组织活跃成员后，整笔记组织账；
- 权限：全部活跃成员可用（API 世界里拿 key 是唯一门槛，游乐场把门槛换成成员身份）；
- 触发：自动兜底、用户无感，不弹窗、不提供选择器；
- 选组织：上次兜底用过 > 剩余额度最多（并列取组织 id 最小）；
- 覆盖：全部 `/pg` 入口（chat / images / video）。

**方式（2026-10-15 确认）**：方案 B——后端在 Distribute 之前预判并切换。原方案 A（错误驱动重试）被否，不实施。

## 2. 非目标（明确不做）

1. **不做错误驱动重试**：「余额 ≥ 阈值但单请求更贵」的场景保持原报错，不自动兜底（将来可在此基础上叠加，见 §9）。
2. **不做手动选择**：不加「额度来源」选择器，不加 `organization_id` 请求字段。
3. **有活跃订阅的用户不参与兜底**：订阅是他们的有效资金来源，不静默越级到组织（判定见 §4.1）。
4. **阈值不做成设置项**：单常量，日后需要再提。
5. **不改计费内核**：不新增 quota 计算/转换，不触 `OrganizationFunding` 的事务、账本、幂等、修复退款逻辑——全部复用。
6. **不做组织 key 语义的检查**：blocker / visibility / responsible-owner-admin 校验都是 key 语义（`service/organization_token_auth.go:77-119`），游乐场无 key，不适用。

## 3. 关键决策

| 决策 | 选择 | 理由 |
|---|---|---|
| 触发条件 | 钱包为有效资金来源 **且** 钱包余量 `< 阈值`（严格小于） | 等于阈值不切，行为可预期 |
| 阈值 | **¥0.2 ≈ 13,700 quota**（= 0.2 ÷ 7.3 × `QuotaPerUnit`，`common/constants.go:22` 默认 50 万/USD） | 2026-10-15 确认按人民币表达。遵循「存储与计费永远 USD」原则（2026-09-08 货币设计）：折算为美元锚 ≈ $0.0274 后固定，**不随展示汇率变化**；`QuotaPerUnit` 在调用时参与换算，保证 USD 语义 |
| 选组织 | 活跃成员 ∩ `org.status=active` ∩ `quota-used>0`；last-used 命中则用，否则剩余最多（并列 id 最小） | 与"上次用过 > 额度最多"定案一致；全自动、事后可解释 |
| last-used 存储 | `users.setting` JSON 新字段 `playground_last_organization_id`（`relaykit/dto/user_settings.go`） | JSON 列加字段零迁移；随用户设置 Redis 缓存、多节点共享；仅值变化时写回 |
| 切换位置 | 新中间件，注册在 **UserAuth 之后、Distribute 之前** | 渠道选择与分组倍率必须先看到组织分组（与组织 key 语义一致，`middleware/auth.go:474-498`） |
| 失败姿态 | **fail-open**：选组织/写回失败 → 降级走个人路径 + `SysLog` | 兜底是增强路径，不该让 DB 抖动挡住正常请求 |
| 作用域归属 | actor / creator / responsible = **发起人本人** | 账本与日志可审计到具体是谁花的；组织 key 场景 responsible=key 责任人，游乐场无 key |
| 视频轮询 | fetch 按 task_id **无作用域**加载 → 校验 `user_id` → 从任务行回填作用域 | 组织兜底的视频任务，轮询若不还原作用域会查不到（`AsyncTaskScope` 隔离） |

## 4. 流程

### 4.1 POST 生成请求（/pg/chat/completions、/pg/images/generations、/pg/images/edits、/pg/video/generations）

1. `UserAuth()`：会话鉴权，写 `group`/`user_group` = 用户分组（`middleware/auth.go:202-215`）。
2. **`PlaygroundOrganizationFallback()`（新中间件）**：
   - **skip 条件**：非 POST；`use_access_token`（PAT，控制器本就会拒绝）；上下文已设 `billing_account_type=organization`。
   - **有效资金来源判定**：`NormalizeBillingPreference(userSetting.BillingPreference)`（`service/billing_session.go:450`）+ `model.HasActiveUserSubscription`。`subscription_first` / `subscription_only` 且有活跃订阅 → 订阅来源 → **skip**；否则钱包来源 → 比较 `model.GetUserQuota(userId, false) < threshold`。
   - **`service.SelectPlaygroundOrganization(userId)`**：
     1. 候选 = 活跃成员（复用 `activeOrganizationMemberWithEnabledUserQuery`，`service/organization_token_auth.go:44`）∩ 组织 `status=active` ∩ 剩余 `quota-used>0`；
     2. last-used（`model.GetUserSetting(userId, false)`）命中候选 → 取之；否则取剩余最多，并列取 id 最小；
     3. 无候选 / 查询失败 → 返回未命中（调用方降级）。
   - **命中后写入上下文**：
     - 构造临时组织 token：`model.Token{UserId: 发起人, Name: "playground-org-<group>", Group: 组织分组, ScopeType: organization, OrganizationId: orgId}`（Id=0、Key 为空，纯粹承载上下文与日志名）；
     - 构造 `TokenScopeContext`（ScopeType/BillingAccountType=organization，BillingAccountId/OrganizationId=orgId，Actor/Creator/Responsible=发起人，AccountGroup=`normalizeOrganizationGroup(org.Group)`，OrganizationQuota=quota-used）；
     - `middleware.SetupContextForTokenWithScopeContext(c, tempToken, scopeCtx)` 写入 id/token_*/scope 全部上下文键；
     - 覆写 `c.Set("group", orgGroup)` 与 `ContextKeyUserGroup` = 组织分组（对齐 `middleware/auth.go:474-498` 的 accountGroup/usingGroup 语义）；
     - last-used 变化时 `model.UpdateUserSetting` 写回（`model/user.go:192`，含 Redis 写穿）。
   - **失败姿态**：以上任一步出错 → `SysLog` + 继续（个人路径），不中断请求。
3. **`Distribute()`**：`usingGroup` = 组织分组；请求体 `group` 覆盖检查走组织可用分组（`middleware/distributor.go:117-124` 的 `GroupInUserUsableGroups` 以 accountGroup 为键，此时即组织分组）；按组织分组倍率选渠道。
4. **`preparePlaygroundContext`（改造）**：PAT 拒绝保留（`:73-76`）；检测 `ContextKeyBillingAccountType == organization` → **跳过个人临时 token 分支**（不造 `playground-<group>`，不覆盖已写作用域），保留 `userCache.WriteContext`；个人路径完全保持现状。已核实 `GenRelayInfo` 无上下文副作用（`relay/common/relay_info.go:683-786` 无任何 `c.Set`/`SetContextKey`），组织分支可安全跳过其调用（该调用只为个人临时 token 命名服务）。
5. **计费（零内核改动）**：`Relay()` → `GenRelayInfo` 读上下文（`relay_info.go:596-642`）→ `NewBillingSession` 组织短路（`service/billing_session.go:437-448`）→ `OrganizationFunding`：
   - 预扣：组织 `used_quota` 原子条件更新（`organization_billing.go:231`：`used_quota + delta <= quota`）+ 账本会话落库（幂等键 = request id）；
   - **令牌额度全程不动**：`IsPlayground` 在 `decreaseTokenQuotaTx` 里直接短路（`service/quota.go:471`），预扣与结算双路径均如此（`organizationBillingTokenQuotaRelayInfo` 复制 `IsPlayground`，`organization_billing.go:972-981`）；
   - 结算/退款/崩溃修复：账本现成；流式心跳续租现成（`TouchOrganizationBillingSession`）。
6. **日志（零改动）**：`RecordConsumeLog` 自动带 `organization_id`/组织名 + `actor_user_id`/`creator_user_id`/`responsible_user_id` 三段 id 与名字快照（`model/log.go:415-429`）；组织日志视图按 `organization_id` 筛出。

### 4.2 视频轮询（GET /pg/video/generations/:task_id）

1. `PlaygroundVideoFetch` 前置改造（`controller/playground.go:123`）：按 task_id **无作用域**加载任务（新增 model 函数，仅内部使用）：
   - 任务存在且 `task.UserId == 调用者` → 按任务行回填 scope/billing 上下文（组织任务还原组织作用域，个人任务还原个人）；
   - 不属于调用者 / 不存在 → 不回填，走既有 not-found 行为（与现在可观察行为一致）。
2. `preparePlaygroundContext`：组织任务命中组织分支（跳过个人 token）；个人任务同现状。
3. `RelayTaskFetch`：`AsyncTaskScope` 按回填的作用域查到任务，正常回传统一 TaskResponse。

## 5. 不变式

1. **钱只从组织账出**：组织请求的预扣/结算/退款只动组织 `used_quota` 与账本；个人钱包、订阅一律不参与（`NewBillingSession` 组织短路是现行保证，本次不放松）。
2. **playground 请求全程不写令牌额度**：预扣与结算双侧都有 `IsPlayground` 守卫（`service/quota.go:471` 及 settle 侧），本次依赖而非新增。
3. **扣费原子性沿用账本**：组织余额判定在 SQL WHERE 里原子裁决；崩溃残留由修复协程按租约退款——本次不新增任何写路径。
4. **权限每请求校验**：成员身份校验不走跨请求缓存，不引入「一次校验长期有效」。
5. **fail-open 不改变个人路径**：切换失败/无候选 = 与现状完全相同的个人流程与原报错。
6. **前端零改动**：请求/响应形状不变，用户无感。

## 6. 改动清单

| 文件 | 改动 |
|---|---|
| `relaykit/dto/user_settings.go` | +`PlaygroundLastOrganizationId int`（`json:"playground_last_organization_id,omitempty"`）；需 `cd relaykit && GOWORK=off go build ./...` 独立构建验证 |
| `setting/`（新文件或并入现有文件） | 阈值 ¥0.2：以函数在**调用时**按 `0.2 / 7.3 * QuotaPerUnit` 换算为 int quota（避免包初始化顺序踩到 `QuotaPerUnit` 的加载时机）；注释说明「¥0.2 @ 展示汇率 7.3，美元锚定、不随展示汇率变」 |
| `service/playground_organization.go`（新） | `SelectPlaygroundOrganization(userId)`（候选筛选 + last-used 回退 + 剩余最多）与组织 `TokenScopeContext` 构造；复用 `activeOrganizationMemberWithEnabledUserQuery` 与 `normalizeOrganizationGroup` |
| `middleware/playground_organization.go`（新） | `PlaygroundOrganizationFallback()`：skip 条件、资金来源判定、选组织、上下文写入、last-used 写回、fail-open |
| `controller/playground.go` | `preparePlaygroundContext` 组织分支（跳过个人 token，保留 PAT 拒绝）；`PlaygroundVideoFetch` 前置（无作用域加载 + 归属校验 + 作用域回填） |
| `model/task.go` 或邻近文件 | 按 task_id 无作用域加载任务的内部函数（含 `user_id` 供校验） |
| `router/relay-router.go` | `/pg` 组（`:66`）与 `/pg/video` 组（`:83`）在 `UserAuth()` 之后、`Distribute()` 之前注册新中间件 |

**新 Go 文件按项目规范带 AGPL 双版权头**（`Copyright (C) 2023-2026 QuantumNous` + `Copyright (C) 2026 CubeRouter`）。

## 7. 错误处理

| 场景 | 表现 |
|---|---|
| 无可用组织 / 查询失败 / 写回失败 | fail-open：走个人路径，原报错或成功；`SysLog` 记录异常 |
| 组织切换后、预扣前组织被停用/解散 | `organization is not active` → `ErrorCodeOrganizationDisabled`（`billing_session.go:421-423` 现成映射） |
| 组织余额不足 | `ErrorCodeInsufficientOrganizationQuota`（现成映射） |
| 账本幂等键冲突 | `ErrorCodeOrganizationBillingSessionConflict`（现成映射） |
| 视频任务的 task_id 不属于调用者 | not-found（与现状一致） |

## 8. 测试计划

- `service` 表测（选择逻辑，显式 fixture，`require`/`assert`）：
  - last-used 命中候选 / last-used 已失效（退化为剩余最多）/ 多组织剩余并列取 id 最小 / 无候选（成员被禁用、组织 disabled、组织余额为 0）；
  - 作用域构造：actor/creator/responsible = 发起人、AccountGroup = 组织分组、OrganizationQuota = quota-used。
- `middleware` 表测（阈值与守卫）：
  - 边界：余量 = 阈值不切、余量 < 阈值切、余量 > 阈值不切；
  - 资金来源：`subscription_first`/`subscription_only` 且有活跃订阅不切；无订阅时按钱包判定；pref 为 wallet 系时直接按钱包；
  - skip：GET、PAT、已设组织作用域；
  - 效果：`group`/`user_group` 覆写为组织分组、scope 上下文键齐备、last-used 仅在变化时写；
  - fail-open：选组织查询报错时请求继续走个人路径。
- `controller`：组织分支跳过个人 token（不覆盖作用域）；PAT 仍被拒绝；`PlaygroundVideoFetch` 归属校验（他人任务不改写上下文、组织任务正确回填）。
- `router`：扩展 `relay_router_test.go` 的入口注册断言——`/pg` 与 `/pg/video` 的中间件顺序为 UserAuth → PlaygroundOrganizationFallback → Distribute（`/pg/video` 中 AllowTaskPluginChannel 仍先于 Distribute）。
- 构建：`cd relaykit && GOWORK=off go build ./...`；本地全量按 `local-full-test-runbook`（排除 main 包 + `-race`）。
- 无 DB 结构变更；新增查询沿用现有 GORM 模式（成员 join、settings JSON 读取），三库差异面为零。

## 9. 已知边界（接受）

1. **余额 ≥ 阈值但单请求更贵**：保持原「预扣费额度失败」报错，不兜底。将来若要闭合，在此设计上叠加错误驱动重试（原方案 A 的前端部分）即可，后端 `organization_id` 支持是天然基础。
2. **有活跃订阅的用户**：不兜底；订阅耗尽时报订阅侧错误（同上，将来可叠加）。
3. **任务可见性**：studio 视频历史为本地存储，不受影响；若后续存在服务端任务列表且按个人作用域查询，组织兜底任务不会出现在其中——与组织 key 任务现行行为一致。
4. **last-used 不可读/写失败**：退化为「额度最多」/不记忆，不影响正确性。
5. **阈值**：¥0.2（USD 锚 ≈ $0.0274，≈ 13,700 quota）；单常量，调整需发版，不提为设置项。
