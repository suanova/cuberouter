# 组织成员配额（Member Quota）

> Last Updated: 2026-10-17

本文档讲 cuberouter 的「组织成员配额」：组织 **Owner**/管理员可以为**本组织的单个成员**设置一个「组织内累计消费上限」，限制该成员作为**责任成员（使用者）**的组织调用总消费；**未配置（默认）即不限**。

- 配额是**消费闸门**，不是预分配资金/冻结余额；组织总额度仍是最终硬约束（§1）。
- 预算按成员切分：已配置配额之和不得超过组织总额度；平台调额、成员重新加入等动作可能造成**超额分配**，按固定规则收敛（§2）。
- **预扣严格拒绝、结算/Realtime 宽松入账**：上限拦的是「新请求」，不是「已经产生的用量」（§4）。
- 清除配额（limit = 0）即恢复不限制，已用值不重置（§6）。

## 1. 口径

- **挂载点**：`organization_members.quota_limit`（上限）与 `organization_members.used_quota`（累计实际消费），单位均为 quota。两列对常规成员对象 `json:"-"`，不随成员列表外泄，只经专用接口按权限返回（§5）。
- **成员配额 = 组织内累计实际消费上限**：只统计该成员作为**责任成员**产生的组织计费调用——即该成员作为「使用者」的全部组织 Key 的消费。个人钱包/个人 Key 的消费不计入。
  - 换责任人即换归集对象：组织 Key 换个使用者后，后续消费记到新责任成员名下；用别人的组织 Key 消费，记在别人的配额里。
  - 成员自建多少组织 Key 都绕不过累计上限——配额挂在人身上，不按 Key 求和。
- **累计口径**：`used_quota` 是累计值，不随月份、Key 轮换、配额调整或清除而重置。
  - **计数起点是功能上线时刻**：所有成员从 0 开始；上线后的全部组织消费都计入（未配置配额的成员同样累计，便于日后随时加限）。上线前的历史消费不追溯。
  - 成员加入组织后从加入时刻开始累计。
- **默认不限制**：`quota_limit = 0`（未配置）时，成员及其组织 Key 不受个人上限约束，消费直接受组织总额度约束——与功能上线前完全一致，存量成员无需回填。
- **闸门不预留资金**：成员配额不锁定、不预留组织余额；未配置成员和已配置成员的消费都受 `organization.quota - used_quota` 限制。成员上限只决定「该成员还能不能发起新请求」，不改变组织总额度的硬约束地位。
- **同事务计数**：成员 `used_quota` 与组织 `used_quota` 在**同一个计费事务**里更新（`service/organization_billing.go` 的唯一写入口）。账本（`organization_billing_session` / `organization_billing_record`）是权威记录；修复协程重放 settle/refund 时成员计数随之修正。

## 2. 预算切分与超额分配

成员配额按组织总额度做预算切分，配置动作受预算校验约束；运行期的余额仍由组织总额度兜底。

- **占用预算的定义**：`allocated` = 同一组织内 **active + disabled**、**非 Owner** 成员的 `quota_limit` 之和；`0`（不限）的成员不占预算。exited/removed 成员的配额退出总和。
  - disabled 成员仍占预算：他们仍可能被重新启用，保留预算可避免启用时的冲突。
- **配置校验**：设置/修改成员配额时，要求 `newSum ≤ organization.quota`（`newSum = 当前总和 - 该成员原上限 + 新上限`）。校验在组织行锁内完成，同一组织的配置动作串行化，并发配置不会写穿总和。
- **超额分配（over-allocation）**：以下**非配置动作**允许让已分配之和超过组织总额度：
  - 平台管理员调低组织总额度；
  - 成员重新加入（恢复原配额）；
  - Owner 变更（成员被降级后原配额重新计入预算）。

  发生超额时：组织余额仍是最终闸门（超额不等于可透支）；成员设置对话框会显示「超额分配」警告；接口读取返回 `over_allocated: true`。
- **超额后的下一次配置动作**（设计 I6）：
  - 当前总和已超组织总额度 → **只允许不增加总和的修改**（降低、清除、无变化）；任何会增加总和的修改都拒绝；
  - 当前总和未超 → 要求新总和 ≤ 组织总额度；
  - **清除（limit = 0）任何时候都允许**，因为它只释放预算。
- **读取口径**：GET 返回的 `organization_quota` 是组织总额度，`allocated` 是上述总和（含该成员当前已存的上限），`over_allocated = allocated > organization_quota`。UI 的「可分配余额」= `organization_quota - allocated`。

## 3. 特殊身份与成员状态

| 身份 / 状态 | 能否配置配额 | 占用预算（allocated） | 是否继续累计 used | 说明 |
|---|---|---|---|---|
| **Owner** | 否（PUT 拒绝） | 否 | 是（仅展示/审计） | Owner 始终不受成员配额限制。豁免按**当前** `organizations.owner_user_id` 动态判定：成员被提升为 Owner 后其存量配额暂停生效，被降级后原配额恢复生效，无需在转移时清空。配额记录本身仍可读。 |
| **active（在职成员）** | 是 | 是 | 是 | 正常参与准入与计数。 |
| **disabled（组织内禁用）** | 是（可降额/清除以释放预算） | 是 | 是（已有/在途用量照常入账） | 被禁用后不能发起调用（现有认证）；已用值保留，配额仍占预算。 |
| **disabled（被平台禁用）** | 否（既有平台治理限制） | 是 | 是 | 组织管理员不能修改其配额；需平台恢复账号后再处理。其配额在恢复前一直占预算。 |
| **exited / removed（已退出/被移除）** | 否（GET/PUT 均拒绝） | 否（释放预算） | 是（行与已用值保留） | 重新加入（同一用户）会复活同一行，**原配额与已用值一并恢复**；恢复后可能造成超额分配（§2）。 |

- 目标必须是本组织 **active 或 disabled** 成员；exited/removed 不可读、不可写。
- 沿用既有成员治理等级校验：组织管理员只能配置普通成员（不能配置 Owner 或其他管理员）；Owner 可配置管理员与普通成员；Owner 本人的配额永远不可配置。
- 降低配额到低于已用值是允许的：该成员**立即**无法发起新调用，已用值不变（设计 I9）。

## 4. 运行时强制：预扣严格、结算宽松

| 路径 | 口径 | 行为 |
|---|---|---|
| 预扣（funding，`PreConsumeWithToken` 正差额） | **严格** | 必须满足 `used_quota + 本次预扣 ≤ quota_limit`，否则整个组织计费事务回滚，**不调用上游**。上限判定写在 UPDATE 的 WHERE 条件里由数据库原子裁决，并发预扣不会超限。 |
| 提交后加价（`ReserveMoreWithToken` 正差额） | **宽松** | 无条件入账，允许把 `used_quota` 推过上限；之后该成员的预扣请求全部拒绝。 |
| 结算（`settleWithTokenActual` 正差额） | **宽松** | 按实际用量入账，允许超额；不回滚、不进修复重试。已在途的多个请求仍会按实际用量结算，可能继续推高 `used_quota`。 |
| 结算/退款（负差额、`RefundWithToken`） | 回退 | 减少成员计数并夹在 0，`used_quota` 不会为负。 |
| `/v1/realtime` 会话开始 | 准入检查（预扣之后） | realtime 请求同样先经 `PreConsumeBilling` 按估算预扣，再进入 `WssHelper`，所以会话开始检查发生在**预扣之后**：`quota_limit > 0 && used_quota >= quota_limit` 时拒绝建立上游连接，本次预扣随请求失败退回。预扣已计入 `used_quota`，因此预扣后恰好到达上限（`used_quota == quota_limit`）的会话会被拒绝，比 I4 的等号规则更保守。会话结束后按实际用量入账（宽松口径）。并发会话可能同时通过检查并各自超额（检查不预留），与「上限是准入闸门」的口径一致。 |
| 其他事后路径（`SettleBilling` fallback、无会话任务重算等） | 宽松 | 按结算口径入账，不另设准入检查。 |
| 违规罚金 / 异步任务 / Playground 组织额度兜底 | 同一写入口 | 都走组织计费唯一写入口，自动计数并遵守同一规则。 |

- **上限是准入闸门，不是每笔结算的硬断言**：结算失败会形成未结算坏账，所以已产生的用量必须照常入账；超额如实记录。
- **owner 豁免**：准入判定按当前 Owner 身份动态豁免，但 `used_quota` 仍照常累计（用于展示/审计）。
- **成员行缺失**：用户被彻底删除等导致成员行不存在时，跳过成员计数、记 `SysError`，不影响组织计费。
- **错误面**：成员配额不足是**确定性业务拒绝**（跳过渠道重试、不记错误日志），错误码 `insufficient_organization_member_quota`，前端文案「该成员在组织内的配额已用尽。」，与组织额度不足的 `insufficient_organization_quota`（「组织额度不足」）明确区分。普通 HTTP 请求返回 **403**；`/v1/realtime` 的 WebSocket 在检查前已完成升级，同一拒绝以 WS 错误事件送达，403 状态仍用于跳过重试与错误日志抑制。只有确定的成员配额超限才映射为该拒绝；会话开始检查中的数据库等临时错误按普通请求失败处理，保留重试与错误日志。

## 5. 配置入口与 API

| 项 | 值 |
|---|---|
| 读 | `GET /api/organizations/:id/members/:userId/quota`（权限面 `view_members_full`，与成员名单/白名单同级）→ `{limit, used, organization_quota, allocated, over_allocated}` |
| 写 | `PUT /api/organizations/:id/members/:userId/quota`（能力 `manage_member_quotas`，Owner/组织管理员）body `{"limit": N}` |
| 组织 UI | 组织中心（Organizations）→ 成员列表 → 成员行操作菜单「模型设置」（Model Settings）→ 对话框「成员设置」（Member Settings）→「配额上限」（Quota Limit）区块（输入框「累计消费上限」） |
| 审计 | 组织审计 `organization.member.quota.update`（target=member，含 before/after `{limit, used}`） |

- **PUT 必填**：请求体必须始终包含 `limit`；省略该字段会按 0 绑定，等于**清除上限**。
- **单位**：API 的 `limit` 是 **quota 单位**，`0` = 解除限制；负数拒绝。UI 输入框以 **CNY** 展示与录入（复用 quota/dollar 转换），空或 0 = 不限制。
- **UI 行为**：区块显示已用/上限与「可分配余额」；`over_allocated` 为真时显示超额分配警告。保存是独立 PUT（在模型访问、折扣之后），失败不关闭对话框并保留编辑。入口在调用者拥有 `manage_member_models` / `manage_member_discounts` / `manage_member_quotas` 任一能力且该行允许操作时出现；对话框只渲染有权限的区块。
- **能力**：`manage_member_quotas` 单独授予 Owner/组织管理员（与 `manage_member_models` / `manage_member_discounts` 同模式，可独立授权），响应里以 `can_manage_member_quotas` 下发控制 UI 显隐。
- **目标限制**：GET/PUT 仅接受本组织 active 或 disabled 成员；Owner 的配额可读但不可写；exited/removed 不可读写。
- **平台管理面**：`/api/admin/organizations/...` 没有成员配额路由；本版成员配额由组织 Owner/管理员在组织中心配置，平台管理员不通过该能力管理成员配额（沿用成员模型白名单的同一模式）。

## 6. 回滚与解除

- **清除上限 = 恢复不限制**：把该成员的 `limit` 设为 `0` 即解除限制；之后的预扣不再做成员判定，已用值保留（不清零、不重置），该成员的配额退出预算总和。在超额分配时清除也允许。
- **降低上限**：降到低于已用值允许，效果是该成员立即无法发起新调用；已用值不变。要恢复调用就重新调高上限或清零。
- **代码回滚**：`quota_limit` / `used_quota` 两列保留但旧代码不再读写；组织计费恢复原有行为，无需删列、无外部依赖。
- **停用功能**：把所有成员配额清零即可；`used_quota` 残留值不影响任何计费逻辑。

## 7. 存储、迁移与三库验证

- 列：`organization_members.quota_limit` 与 `organization_members.used_quota`，均为 `int not null default 0`（GORM tag 见 `model/organization_member.go`）；旧行升级后为 0（= 不限制、未消费）。
- 迁移：`AutoMigrate` 加列；**新库建表、旧库升级、二次启动幂等**三种场景均验证；`(organization_id, user_id)` 唯一索引与既有数据在升级后保持不变。
- 三库验证（本功能开发期实测）：
  - **SQLite 3.50.4**（驱动 `github.com/glebarez/sqlite v1.9.0` → `modernc.org/sqlite v1.40.1`，linux/amd64）；
  - **MySQL 5.7.44**；
  - **PostgreSQL 15.19**。

  覆盖：迁移的 fresh / upgrade / 重启幂等；配额读取与写入、预算总和（`SUM`）、行锁（`FOR UPDATE`）、预算拒绝与审计落库；计费计数的严格预扣、宽松结算、退款夹 0。外部库用例通过 `TEST_MYSQL_DSN` / `TEST_POSTGRES_DSN` 开启：**迁移用例只需 DSN**（在一次性 scratch 库/schema 中执行）；**service 外部用例还需 `ORGANIZATION_TEST_ALLOW_DESTRUCTIVE=1`**（在 DSN 指向的**测试库**内建表、插数据并在用例结束时清理，不要指向生产库）；两个 DSN 同时设置时，service 外部用例需再用 `ORGANIZATION_TEST_DB_TYPE=mysql|postgres` 选定引擎，否则跳过。

## 8. 相关

- 设计文档：`docs/superpowers/specs/2026-10-17-org-member-quota-limit-design.md`
- 组织成员模型白名单（同样挂在成员关系上、跟随人）：`docs/org-member-model-access.md`
- 用户 × 模型折扣（改价，正交）：`docs/billing/user-model-discount.md`
- 组织体系总览（成员/角色/能力模型）：`docs/organization-system.md`
