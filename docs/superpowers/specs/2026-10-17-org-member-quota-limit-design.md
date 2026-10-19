# 组织成员配额（Member Quota）设计

**Date:** 2026-10-17
**Status:** Approved（2026-10-17）
**基线:** `feat/org-member-models`（`3f666a0`）→ 本 PR 分支 `feat/org-member-quota-limit`；等前者合并到 upstream 后 rebase 到最新 main
**范围:** `model/` `service/` `controller/` `router/` `i18n/`、前端 `web/`

## 1. 目标

组织 owner/admin 可以为**单个成员**配置一个「组织内累计消费上限」：

- 默认（未配置）：成员及其组织 Key 都不设个人上限，消费直接受组织总额度约束——共享组织池子（现有行为，零变化）。
- 配置后：该成员在组织内的**累计实际消费**（其作为责任成员的所有组织 Key）不得超过成员配额；用尽后拒绝其后续组织调用。
- 所有已配置成员的配额之和 ≤ `organization.quota`，按预算切分管理。
- 成员配额是**消费闸门**，不是预扣资金/钱包；组织总额度仍是最终硬约束。

## 2. 非目标

1. 不做预分配资金/冻结余额；成员配额不锁定组织余额。
2. 不做月度/周期重置；配额是累计值（与 `organizations.used_quota` 同口径）。
3. 不改组织总额度的硬约束逻辑（`organization.quota - used_quota` 仍是最终闸门）。
4. 不做跨组织合并；配额挂在组织成员关系上，按组织独立。
5. 不在本版约束「成员自建 Key 时填写的 Key 额度」——运行时累计上限已经覆盖，Key 上的 `remain_quota` 不参与本规则。成员自建 Key 的额外约束留作后续（§11）。
6. owner 不可配置配额（沿用成员治理目标等级校验）；owner 始终不限。
7. 不改平台管理员的组织额度调整流程（允许调低到低于已分配之和，见 §4 I5）。

## 3. 关键决策

| 决策 | 选择 | 理由 |
|---|---|---|
| 配额语义 | 成员在组织内的**累计实际消费**上限 | 组织 Key 默认 `unlimited_quota`，按 Key 数字求和没有意义；实际消费才是管理目标 |
| 归属 | 组织 Key 的**责任成员**（`responsible_user_id`） | 现有组织计费 actor 即责任成员；换责任人后后续消费记新责任人 |
| 默认 | 不限制，共享组织总额度 | 存量行为零变化，存量成员无需回填 |
| 配置之和 | active + disabled 成员的已配置配额之和 ≤ `organization.quota`；exited/removed 释放；owner 不计入 | 预算切分；disabled 成员仍可能回来，保留预算避免启用冲突 |
| 超额事件 | 允许发生（平台调低总额度、成员重新加入），组织显示「超额分配」；组织余额兜底 | 规则简单，不阻断平台运维和邀请流程；只在配置动作时校验总和 |
| 强制时点 | 预扣/加码严格拒绝；结算允许一次超额，之后拒绝新请求 | 用量已产生，结算失败会形成未结算坏账；上限是准入闸门，不是每笔结算的硬断言 |
| 存储 | `organization_members.quota_limit`、`organization_members.used_quota` | 与 `model_access` 同挂成员关系；`0 = 不限制` |
| 计数 | 与组织 `used_quota` 在**同一事务**里更新；账本（`organization_billing_record` / `session`）为权威，可重算 | 复用组织计费唯一写入口，避免并发超扣 |
| 能力 | 新 capability `manage_member_quotas`（owner/admin） | 与 `manage_member_models` / `manage_member_discounts` 同模式，可独立授权 |
| 审计 | `organization.member.quota.update`（target_type=member，before/after） | 与成员其他治理操作同口径 |
| 前端入口 | 并入现有「Member Settings」合并对话框，新增 Quota Limit 区块 | 成员治理在同一屏；按 capability 显隐 |

## 4. 语义与不变式

- **I1 默认零变化**：`quota_limit = 0`（未配置）时，准入、计费、Key 行为与现状完全一致。
- **I2 只按责任成员归集**：成员消费 = 其作为 `responsible_user_id` 的组织计费调用；用别人的组织 Key 计数在别人的名下。成员自建多少 Key 都绕不过累计上限。
- **I3 累计口径**：`organization_members.used_quota` 是累计值，不随月份/Key 重置；调高/调低配额、清除限制都不重置已用值。计数起点为**功能上线时刻**：所有成员从 0 开始，上线后的全部组织消费都计入（未配置配额的成员同样累计）；上线前的历史消费不追溯。
- **I4 准入严格**：`quota_limit > 0` 时，预扣请求必须满足 `used_quota + 本次预扣 ≤ quota_limit`，否则在调用上游前拒绝。
- **I5 结算允许超额**：结算与任务提交后的加价（`ReserveMoreWithToken`）正差额把 `used_quota` 推过 `quota_limit` 时照常入账（不失败、不进修复重试）；之后该成员的预扣请求全部拒绝。超额如实记录。
- **I6 预算和**：配置/修改成员配额时，同一组织内 active + disabled 非 owner 成员的 `quota_limit` 之和不得超过 `organization.quota`（0 = 不限的成员不占预算）。平台调低总额度、成员重新加入等非配置动作允许突破该和，UI 标记「超额分配」；下一次配置动作重新校验：**当前和已超总额度时，只允许不增加总和的修改（降低/清除），任何增加都拒绝；未超时要求新总和 ≤ 总额度**。
- **I7 owner 豁免**：owner 不可配置配额（沿用 `ensureOrganizationMemberTargetAllowed`）。豁免按**当前** owner 身份/角色动态判定（计费与预算计算读取 `organizations.owner_user_id`），不要求在转移时清空 `quota_limit`；成员被提升为 owner 后其存量配额暂停生效，被降级后原配额恢复生效。
- **I8 成员状态联动**：disabled 成员不能调用（现有认证）；其配额仍计入总和、已用值保留。exited/removed 成员的配额退出总和；重新加入时恢复原配额（可能造成超额，按 I6 标记）。
- **I9 降额语义**：管理员把配额降到低于已用值时允许；该成员立即无法发起新调用，已用值不变。
- **I10 组织总额度仍是最终闸门**：未配置成员和已配置成员的消费都受 `organization.quota - used_quota` 限制；已配置配额不预留余额。

## 5. 数据与计数

### 5.1 列

- `organization_members.quota_limit`：`int not null default 0`，单位 quota；`0 = 不限制`。
- `organization_members.used_quota`：`int not null default 0`，累计实际消费。
- 迁移：`AutoMigrate` 加列；旧行保持 0（= 不限制、未消费），升级不丢数据；三库 fresh/升级/二次启动幂等验证。

### 5.2 计数写入口

组织计费的唯一写入口是 `service/organization_billing.go` 的 `updateOrganizationUsedQuotaTx`（6 个调用点：funding、ReserveMore、settle、refund、billing repair 等）。实现时在同一事务内同步成员计数：

- 调用方传入责任成员 `responsibleUserId`（`relayInfo.ResponsibleUserId` 或 `session.ResponsibleUserId`）。
- 正差额且是**预扣**（funding）：条件更新 `WHERE organization_id=? AND user_id=? AND (quota_limit = 0 OR used_quota + ? <= quota_limit)`；0 行 → 返回 `ErrOrganizationMemberQuotaExceeded`，整个组织计费事务回滚。
- 正差额且是**结算或提交后加价（ReserveMore）**：无条件 `used_quota = used_quota + ?`（允许 I5 超额）。
- 负差额（退款/结算退款）：`used_quota = max(0, used_quota + delta)`。
- 责任成员行为缺失（用户已彻底删除等）：跳过成员计数，不影响组织计费；记 `SysError`。
- owner：按当前 owner 身份动态豁免（不依赖 `quota_limit` 为 0）；仍记录 `used_quota` 便于展示/审计。

预扣入口（`PreConsumeWithToken`）在同一事务内的条件更新就是准入闸门；错误在事务回滚后以 `ErrOrganizationMemberQuotaExceeded` 返回。`PreConsumeOrganizationQuota` 当前没有生产调用点，不在此处重复实现。

### 5.3 账本与修复

- `organization_billing_session` 已记录 `responsible_user_id`、`pre_consumed_quota`、`settled_quota`、`refunded_quota`；账本记录 `organization_billing_record` 也带 `responsible_user_id` 与 usage。
- 修复协程重放 settle/refund 时走同一写入口，成员计数随之修正。
- 提供按 `responsible_user_id` 的账本重算能力（运维/测试辅助），入参 `since`（默认功能上线时刻）；上线前的账本记录不计入。日常一致性靠同事务更新与修复重放保证，不依赖重算。

## 6. 计费强制点

| 路径 | 行为 |
|---|---|
| `PreConsumeWithToken`（funding，正差额） | 成员上限严格拒绝；组织计费事务回滚，不调上游 |
| `ReserveMoreWithToken`（正差额） | 允许超额入账（I5）；之后新请求被拒 |
| `settleWithTokenActual`（正差额） | 允许超额入账（I5），不回滚、不进修复 |
| `settleWithTokenActual`（负差额） | 退回成员计数（clamp 0） |
| `RefundWithToken` | 退回成员计数（clamp 0） |
| 违规罚金 / 异步任务 / Playground 组织兜底 | 都走 `OrganizationFunding`/`updateOrganizationUsedQuotaTx`，自动覆盖 |
| `consumeOrganizationAndTokenQuotaWithRequestCount`（realtime、`SettleBilling` fallback、任务重算） | 事后扣费按结算口径（允许超额入账）；`/v1/realtime` 另加会话开始准入检查（§5.4） |

### 5.4 `/v1/realtime` 会话开始准入（预扣之后）

`/v1/realtime` 是 WebSocket 长连接，但请求并非没有预扣：`Relay` 在 `WssHelper` 之前已经调用 `service.PreConsumeBilling` 按估算预扣（`controller/relay.go`），会话结束后再由 `PostWssConsumeQuota` 按实际用量结算。会话开始检查因此运行在预扣之后：

- 建立上游连接前（`WssHelper` 调上游之前）检查：`quota_limit > 0 && used_quota >= quota_limit` → 拒绝，不建立上游连接，本次预扣随请求失败退回；
- 因为预扣已经计入 `used_quota`，预扣后恰好到达上限（`used_quota == quota_limit`）的会话会被拒绝；这比 I4「`used + 本次预扣 ≤ quota_limit`」的等号规则更保守，可以接受；
- 会话结束后的 `PostWssConsumeQuota` 走事后台账，按 I5 允许超额；
- 并发会话可能同时通过检查并各自超额（检查不预留），与「上限是准入闸门」的口径一致，接受。

其他事后路径（`SettleBilling` fallback、`RecalculateTaskQuota` 无会话任务）同样按结算口径入账，不另设准入检查。

错误面：新增 `ErrOrganizationMemberQuotaExceeded`，映射为请求拒绝（沿用组织额度不足的 HTTP 状态与错误格式），i18n 文案区分「组织额度不足」与「成员配额不足」。

## 7. API / 权限 / UI

- `GET /api/organizations/:id/members/:userId/quota`
  - 权限：`OrganizationReadAccessAuth(ViewMembersFull)`（与成员名单/白名单同面）
  - 目标：active 或 disabled 成员（disabled 仍占预算，需要可读）
  - 响应：`{limit, used, organization_quota, allocated, over_allocated}`；`allocated` = active+disabled 非 owner 的限额之和
- `PUT /api/organizations/:id/members/:userId/quota`
  - 权限：新 capability `manage_member_quotas`（owner/admin）
  - body：`{"limit": 0}`（0 = 解除限制）
  - 校验：目标为 active 或 disabled 成员（exited/removed 不可操作；disabled 必须可降额/清除以释放预算）+ `ensureOrganizationMemberTargetAllowed`；`limit >= 0`；预算和校验（持组织锁）；写审计 `organization.member.quota.update`（before/after）
- 前端：
  - 现有「Member Settings」对话框新增 **Quota Limit** 区块：输入金额（CNY，复用 quota/dollar 转换），空/0 = 不限制；显示已用/上限、组织可分配余额；超额分配时显示警告
  - 成员行操作入口在调用者拥有 `manage_member_models` / `manage_member_discounts` / `manage_member_quotas` 任一能力时显示；对话框按 capability 组合显示三个区块（模型访问 / 折扣 / 配额），只渲染有权限的区块
  - 保存顺序与现有对话框一致：独立 PUT；失败不关闭，保留编辑
- 可选：成员列表增加「配额」列（used/limit），仅管理面显示

## 8. 测试计划

- **model/迁移**：加列、旧行默认 0、三库 fresh/升级/重启幂等；`used_quota` 与 `quota_limit` 读写。
- **service/配置**：设置/清除/降额；预算和矩阵（active/disabled/exited/removed、owner、并发配置）；目标等级校验；审计落库；超额分配状态。
- **计费**：funding 严格拒绝（成员上限）、settle/ReserveMore 超额入账、refund 回退、clamp 0、owner 豁免、disabled 成员计数、多 Key 归集、任务/Playground/罚金路径；`/v1/realtime` 会话开始拒绝（配额已用尽）与事后超额入账；与组织额度不足的错误区分；并发预扣不超限。
- **修复**：账本会话重放后成员计数一致。
- **前端**：对话框读取/保存/清除、金额转换、超额提示、capability 门控、失败不关闭。
- **三库**：SQLite / MySQL 5.7.44 / PostgreSQL 15.19。

## 9. 回滚

- 数据：把 `quota_limit` 清零即恢复不限制；代码回滚后列保留但不生效。
- 计数：列保留，无外部依赖；组织计费不读它时不影响现有逻辑。

## 10. 改动面预判

| 面 | 文件 |
|---|---|
| 数据 | `model/organization_member.go`（两列）、迁移测试 |
| 服务 | `service/organization_member.go`（GET/PUT quota）、`service/organization_billing.go`（计数与强制）、`service/organization_policy.go`/`organization_access.go`（capability）、`service/organization_audit.go`（action） |
| 计费错误 | `service/organization_billing.go` + i18n（错误键/文案） |
| API | `controller/organization_member.go`、`router/organization-router.go` |
| 前端 | `features/organization/{api,types,lib}`、成员设置对话框（新增 Quota 区块）、成员行/列、locales |
| 测试/文档 | 各包测试、`docs/org-member-quota.md`（口径/预算/超额/回滚） |

## 11. 后续（不在本 PR）

成员在组织中**自己创建 Key** 时的约束：本版运行时累计上限已经覆盖「多建 Key 绕过」；若还需要在创建时限制 Key 的 `remain_quota` 或禁止 `unlimited_quota`，作为下一版单独讨论。

## 附录 A：验收标准

1. 未配置成员：全链路与现状一致（回归全绿）。
2. 配置成员配额后：该成员所有责任组织 Key 的累计消费到上限即拒绝新请求；其他成员不受影响。
3. 管理员配置配额时，所有 active+disabled 非 owner 成员配额之和超过组织总额度 → 400；已超额时只拒绝增加总和的修改，降低/清除始终允许。
4. 结算超额：一次请求可把 used 推过上限，随后请求被拒；账本金额正确；`/v1/realtime` 在配额用尽后不能建立新连接，进行中的会话按实际用量入账。
5. owner 不受限；disabled 成员计数与预算占用正确；exited 释放、重新加入恢复并标记超额。
6. 平台调低组织总额度到低于已分配之和：允许，UI 显示超额分配。
7. 三库矩阵 + Redis 开关（本功能不依赖缓存）通过；审计可见 before/after。
