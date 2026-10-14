# 用户 × 模型折扣计价设计

**Date:** 2026-10-14
**Status:** Draft（待评审）
**范围:** 后端 `model/` `service/` `controller/` `router/` `middleware/` `relay/` `types/` `constant/` `i18n/`，前端 `web/`
**相关:** [fixed-rate.md](../../billing/fixed-rate.md)（计费口径）、[logs-and-reconciliation.md](../../billing/logs-and-reconciliation.md)、`AGENTS.md` 计费安全不变量

## 1. 目标

引入新的计价维度：**用户 × 模型折扣**。管理员可为指定用户、针对每个模型单独配置折扣倍率 d；该用户的调用价格 = 既有口径（模型价/倍率 × 分组倍率或专属倍率 × 其他系数）× d。

- **跟随人**：查找键是调用的发起人（`ActorUserId`）。组织成员用组织令牌（扣组织额度）调用时，同样按成员本人的折扣计价，与扣费主体（个人钱包/订阅/组织额度）解耦。
- **配置者**：平台管理员可为任意用户配置；组织所有者/管理员可为本组织在职成员配置。折扣挂在**个人**身上，组织不是计价主体。
- **覆盖范围**：全部按模型计价的路径——文本 token、按次（ModelPrice）、tiered_expr 表达式、图像按张、视频/任务按次或按秒、实时语音（WSS）。订阅与组织账本只是资金来源，价格口径相同，自动继承。
- 未配置的模型 d = 1.0（原价）。

## 2. 非目标（明确不做）

1. 不做加价（d > 1）；折扣值域 (0, 1]。
2. **不允许 0 折**（免费）。需要免费时沿用既有机制（分组倍率 0 / 模型价格为 0）；避免两种"免费"来源造成对账歧义。
3. 不做用户自助配置——用户不能改自己的折扣。
4. 不做组织级计价主体——组织令牌的价格取决于"人"，不是组织。
5. 不改可访问性——折扣只影响价格，不影响模型可用范围（分组/渠道判定不变）、限流、余额校验。
6. 折扣不作用于工具费（`relay/common/tool_usage.go` 的 tool price）。
7. 不支持通配符/前缀模型键，精确匹配计费模型名（`GetBillingModelName()`）。
8. 不新增"我的折扣"自助查询接口；用户通过定价页折后价与使用日志可见（§9）。

## 3. 关键决策

| 决策 | 选择 | 理由 |
|---|---|---|
| 存储 | `users.model_discounts TEXT` JSON 列（`{"模型名": 0.8}`） | 与 `users.setting`（`model/user.go` User 结构）同型先例；随认证缓存零额外往返；三库只需 ADD COLUMN；不新增表 |
| 生效位点 | 折叠进 `GroupRatioInfo.GroupRatio`，插点为 `relay/helper/price.go:46 HandleGroupRatio`（唯一） | 与"专属倍率"折叠先例（`price.go:60-69`）一致；预扣/结算/任务快照/订阅/组织账本全部自动继承。独立字段方案要改约 10 处乘法点，漏一即预扣≠结算 |
| 查找键 | `ActorUserId` | 跟随人：个人=本人、组织令牌=调用者；`middleware/account_context.go:92-104` 恒填充 |
| 快照时机 | 鉴权时载入 context → `GenRelayInfo` 快照进 `relayInfo` → 请求内不再重读；任务另做 `TaskBillingContext` 快照 | 单次请求预扣与结算同值（I3） |
| 罚金 | 随折扣缩放（`service/violation_fee.go:123` 不改） | 现状罚金已随专属倍率缩放；保持一致，文档标注 |
| 0 折 | 写入校验拒绝 d ≤ 0 | 见非目标 2 |
| 组织权限 | 新 capability `OrganizationCapabilityManageMemberDiscounts` | 与 `manage_members` 解耦，可独立授权/收紧 |
| 成员退出组织 | 折扣保留 | 折扣是"人"的属性；自动清除会让同一用户行为随组织关系漂移。平台管理员始终可清；组织侧 UI 提示 |
| API 写语义 | 全量替换（PUT `{"discounts":{...}}`），事务内 `lockForUpdate` 读改写 | 与 JSON 列存储一致；并发覆盖以审计 before/after 留痕兜底 |
| 值域 | (0, 1]，保留 4 位小数 | 校验拒绝 0/负/NaN/±Inf/>1；d = 1 的条目写入时剔除（等价删除） |

## 4. 不变式

- **I1 折后价** = 既有口径 × d。
- **I2 跟随人**：查找键恒为 `ActorUserId`。
- **I3 请求内一致**：单请求预扣与结算使用同一折扣值（随 `GroupRatioInfo` 折叠传播）；任务提交时快照进 `TaskBillingContext`，轮询阶段的差额重算沿用；旧任务无快照字段 → 视为 1.0。
- **I4 安全不变量不变**：折扣乘法发生在倍率合成阶段，不引入新的 quota 转换；所有换算仍走 `common.QuotaFromFloatStrict` / `billingexpr.QuotaRoundStrict` 等既有 `*Checked` 通道。
- **I5 罚金缩放**：罚金 = `ViolationDeductionAmount × GroupRatioInfo.GroupRatio`（已含折扣），与专属倍率现状一致。
- **I6 无 0 折**：d ∈ (0, 1] ⇒ `GroupRatio == 0` 只可能来自分组倍率 0 的既有配置，`freeModel` 判定（`price.go:168-184`）不受本功能影响。
- **I7 不外泄**：`model_discounts` 列 `json:"-"`；仅专用接口按权限返回；`GetSelf` / `GetAllUsers` / `ToBaseUser` 不带该字段。

## 5. 数据模型与校验

### 5.1 列与解析

```go
// model/user.go User 结构（仿 Setting 列，type:text;column:setting）
ModelDiscounts string `json:"-" gorm:"type:text;column:model_discounts"`
```

- 空串 = 无折扣；JSON 形状 `{"<计费模型名>": <d>}`。
- 新增 `model/user_model_discount.go`（新文件带 AGPL 头）：
  - `ParseUserModelDiscounts(raw string) map[string]float64` —— 逐条校验（模型名 trim 后非空且 ≤ 255 字节；d 有限且 0 < d ≤ 1），非法条目跳过并 `common.SysError`，不使请求失败、也不静默按全价（跳过行为本身有日志）。
  - 上限常量：条目 ≤ 500、序列化 ≤ 64 KiB。

### 5.2 写入校验（API 层 + service 层双保险）

- 模型名：trim 后非空、≤ 255 字节。
- 折扣：有限浮点；**0 < d ≤ 1**；四舍五入到 4 位小数；d == 1 剔除。
- 超限（条目数 / 体积）或非法值一律 400 拒绝、不落库；错误消息走后端 i18n（`i18n/` en/zh 新增 Msg 键）。
- 读路径防御（§8.3）：`HandleGroupRatio` 查到的值再做一次 `0 < d ≤ 1 && finite` 判定，不满足按 1.0 计 + `common.SysError`。

## 6. 缓存与生命周期

1. `model/user_cache.go:16` `UserBase` 增加 `ModelDiscounts string`；`ToBaseUser`（`model/user.go:129`）带上；`writeUserCache` Lua 脚本（`model/user_cache.go:53-96`）增加 HSET 字段；`userCacheSchemaVersion` 2 → 3（`model/user_cache.go:14`）——旧条目自然失效、一次回源，无需数据迁移。
2. `UserBase.WriteContext`（`model/user_cache.go:29`）把解析后的 map 写入新 context key `ContextKeyUserModelDiscounts`（`constant/context_key.go` 新增）。
3. 新增 `RefreshUserModelDiscountsCache(userId)` —— 仿 `RefreshUserGroupCache`（`model/user_cache.go:202`）：`updateUserCacheFieldAtVersion`（`model/user_auth_cache.go:243`）+ 写读复核循环。
4. 新增 `model.GetUserModelDiscounts(userId)`：缓存优先、DB 兜底（仿 `GetUserGroup`，`model/user.go:1348`），供兜底路径使用。
5. 两处写 API 在事务提交后调用刷新；失败仅 `SysError`（与用户分组同策略），缓存过期后自然收敛。
6. **组织令牌**：`middleware/auth.go:458` 的 `userCache` 是令牌属主；scopeCtx 解析后（:472 起），若 `scopeCtx.ActorUserId != token.UserId`，额外 `GetUserCache(ActorUserId)` 并覆盖 context key（跟随人）。
7. **兜底**：`relayInfo.UserModelDiscounts == nil`（未经过 TokenAuth 的路径，如 Playground / 渠道测试）时，`HandleGroupRatio` 用 `model.GetUserModelDiscounts(ActorUserId || UserId)` 惰性读缓存（`relay/helper` 可以 import `model`，`relay/common` 不行，注意依赖方向）。

## 7. 计费链改造点

### 7.1 折叠点（唯一新增）

`relay/helper/price.go:46 HandleGroupRatio`：

```go
base := groupRatioInfo.GroupRatio                    // 现状：专属倍率或分组倍率
d := LookupUserModelDiscount(relayInfo)              // GetBillingModelName() 精确匹配；缺省 1
groupRatioInfo.GroupRatio = base * d                 // 折叠：所有下游自动继承
groupRatioInfo.UserModelDiscount, groupRatioInfo.HasUserModelDiscount = d, d != 1
```

`types/price_data.go`：
- `GroupRatioInfo`（:10）增加 `UserModelDiscount float64` + `HasUserModelDiscount bool`（展示/审计用，不参与再次乘法）；
- `PreConsumeDetail`（:37）增加 `UserModelDiscount float64`（随消费日志输出，对账可追溯）。

**自动继承、不改代码（测试覆盖即可）**：`ModelPriceHelper` ratio 路（`price.go:141`）、`ModelPriceHelperPerCall`（`:206`，含 MJ）、`modelPriceHelperTiered`（`:369` 预扣与 `:385` 快照）、`relay/relay_task.go:300` 任务预扣与 `:303` TieredBillingSnapshot、`service/quota.go:182/315` 结算、`service/text_quota.go:239`、`service/tiered_settle.go:101 refreshTieredBillingGroup`、`controller/relay.go:366` 自动分组重试（重跑 `HandleGroupRatio`，折扣随组重解析自动刷新）。

### 7.2 必须同步改的"自行解析倍率"点（3 处）

1. **`service/quota.go:87 PreWssConsumeQuota`**（实时语音逐消息预扣）—— :105-122 自行解析分组倍率，需 `actualGroupRatio *= LookupUserModelDiscount(relayInfo)`（同一 helper，同一模型名口径）。
2. **`service/task_billing.go:417 RecalculateTaskQuotaByTokens`**（任务异步差额结算）—— :443-451 从 `task.Group` 重新解析分组倍率，会丢折扣：`model/task.go:222 TaskBillingContext` 增加 `UserModelDiscount float64`（`json:"user_model_discount,omitempty"`；读取时 `<= 0` 视为 1.0，兼容旧任务）；写入点 `controller/relay.go:776`（提交时从 `relayInfo.PriceData.GroupRatioInfo.UserModelDiscount` 快照）；重算公式乘上该值。
3. （核对项，无需改）`controller/relay.go:366` 自动分组重试、`service/tiered_settle.go` 的 snapshot 刷新：两侧都取折叠后的 `GroupRatio`，天然一致。

### 7.3 展示与日志

- `service/log_info_generate.go:322` 与 `service/task_billing.go:52`：`HasUserModelDiscount` 为真时写 `other["user_model_discount"]`（与 `user_group_ratio` 同型）。
- 前端 `web/src/features/usage-logs/components/dialogs/details-dialog.tsx:302`、`components/columns/common-logs-columns.tsx:273`：加"用户折扣"展示。
- `relay/mjproxy_handler.go:299,690` 等把 `GroupRatioInfo.GroupRatio` 当"分组倍率"展示的文案：补折扣行或改标签（折叠后该值已是有效倍率）。

### 7.4 边界说明

- **罚金**（`service/violation_fee.go:123`）：不改，随折扣缩放；`docs/billing/` 写明。
- **工具费**（`relay/common/tool_usage.go:49`）：非目标，不打折。
- **渠道测试**（`controller/channel-test.go:563`）：按测试者本人折扣（通常无配置），无影响。

## 8. API / 权限 / 审计

### 8.1 平台管理员

- `GET /api/user/:id/model-discounts` —— 挂 `userReadRoute`（`router/api-router.go:154`，`OpsAuth` 读权限面），返回 `{"success":true,"data":{"discounts":{...}}}`。
- `PUT /api/user/:id/model-discounts` —— 挂 `adminRoute`（`router/api-router.go:173`，`AdminAuth`），body `{"discounts":{"模型":0.8}}`，全量替换。
- 写入流程：解析校验（§5.2）→ 事务内 `lockForUpdate(tx)` 读用户行 → 写 `model_discounts` → 提交后 `RefreshUserModelDiscountsCache` → `recordManageAuditFor(c, id, "user.model_discounts", {before, after})`（`controller/audit.go:99`）。
- 等级校验复用现有规则（不能修改自己无权管理的角色，参考 `controller/user.go` `ManageUser` 的 `canManageTargetRole`）。
- **路由注册注意**：`/:id` 具名路由的先后顺序（参见 `router/api-router.go:163` 附近注释与 `router/user_invitees_route_test.go` 的回归约束）。

### 8.2 组织管理员

- `GET /api/organizations/:id/members/:userId/model-discounts` —— `middleware.OrganizationReadAccessAuth(service.OrganizationCapabilityViewMembersFull)`（与成员列表同权限面）。
- `PUT /api/organizations/:id/members/:userId/model-discounts` —— `middleware.OrganizationManagementAuth(service.OrganizationCapabilityManageMemberDiscounts)`。
- 新 capability：`service/organization_policy.go:41` 常量区加 `OrganizationCapabilityManageMemberDiscounts = "manage_member_discounts"`，并在角色→能力授予表（同文件 `:342-392`，owner/admin 段）与 `manage_members` 同级授予；`service/organization_access.go:35` Capabilities 结构加 `CanManageMemberDiscounts`（`:192` 映射处同步）。
- 服务层校验：目标 userId 是本组织**在职（active）**成员；对 owner 成员的操作沿用 `UpdateOrganizationMember` 的现有等级校验。
- 审计：`service/organization_audit.go:42` 常量区新增 action `organization.member.model_discounts.update`，`target_type=member`、before/after，metadata 注明"折扣作用于该成员个人，对其全部调用生效（含个人钱包）"。
- 路由挂点：`router/organization-router.go:43-46`。

### 8.3 读路径防御

见 §5.2 末条：折叠前对 d 再做 `0 < d ≤ 1 && finite` 判定，非法按 1.0 + `SysError`（AGENTS.md 乘数先验证）。

## 9. 前端改造点

1. **后台用户抽屉**：`web/src/features/users/components/users-mutate-drawer.tsx:352`（"分组与额度"区）下方新增"模型折扣"编辑器——模型下拉（数据源用现有定价模型列表 hooks，不新增接口）+ 折扣输入（百分比 1–100，落库 4 位小数），保存走 §8.1 PUT，打开时 GET 拉取。
2. **组织成员侧**：`web/src/features/organization/components/organization-members-section.tsx` 与 `organization-member-dialogs.tsx` 成员详情同款编辑器，入口按 `CanManageMemberDiscounts` 控制；文案明确"作用于该成员的所有调用"。
3. **定价页折后价**：`controller/pricing.go GetPricing`（约 :56）响应顶层追加 `user_model_discounts`（对登录用户返回其 (0,1) 区间条目；**不得修改** `model.GetPricing()` 缓存的 Pricing 对象，参考现有 groupRatio 的按请求折叠方式）。前端：
   - `web/src/features/pricing/lib/model-helpers.ts`（`getViewerGroupRatio`，:109）旁合并折扣因子；
   - `web/src/features/pricing/lib/price.ts:63 calculateTokenPrice` 增加折扣参数并乘入（调用点 :159/:193）；
   - `components/model-details.tsx:883`、`components/pricing-columns.tsx`、`components/model-card.tsx` 展示折后价 + 折扣徽标。
4. **i18n**：`web/src/i18n/locales/*.json` 新增 key（`bun run i18n:sync`）；后端 `i18n/`（en/zh）新增错误消息键。
5. 新文件带 AGPL 头（`web/` 新文件跑 `bun run copyright`）。

## 10. 测试计划

**Go（testify `require`/`assert`，表测为主）：**

| 用例 | 位置（建议） |
|---|---|
| 解析/校验：合法、0、负、NaN/Inf、>1、超长模型名、超条目数、剔除 1 | `model/user_model_discount_test.go` |
| 三路折叠：ratio / per-call / tiered 的 quota 与 `GroupRatioInfo` 字段 | `relay/helper/price_test.go` |
| 预扣 = 结算一致（同一折扣） | `service/text_quota_test.go`（仿现有表测风格） |
| PreWss 折扣 | `service/quota_test.go` |
| 任务异步重算保留快照折扣；旧任务（无字段）= 1.0 | `service/task_billing_test.go` |
| 鉴权载入：个人（actor = 属主）/ 组织令牌（actor ≠ 属主，覆盖 context） | middleware 层测试或集成测试 |
| API 权限矩阵：admin / org owner / org member / 非成员；读权限 | controller 测试（复用 `service/organization_test_support_test.go`） |
| 审计落库：`user.model_discounts` 与 `organization.member.model_discounts.update` | controller/service 测试 |

**前端**：折后价纯函数测试（`model-helpers` / `price.ts`），与现有测试风格一致时补充。

**回归**：现有计费测试全绿；折扣缺省（无配置）时全链路行为与现状完全一致是硬要求。

## 11. 数据库验证（AGENTS.md 强制）

- 三库矩阵 SQLite / MySQL / PostgreSQL：`AutoMigrate` 在 **fresh 库**与**上一发布版本升级库**上各跑一次、启动两次验证幂等；确认新列默认值、既有索引/约束不受影响；折扣 JSON 三库读写与解析结果一致。
- Redis 开/关两种形态：缓存 schema 2→3 失效回源、刷新函数、组织令牌 actor（≠属主）载入。
- 记录精确版本、命令、结果到 PR。

## 12. 文档与回滚

- `docs/billing/` 新增 `user-model-discount.md`（口径、配置入口、叠加关系、罚金缩放说明、与分组倍率/专属倍率的关系），并在 `fixed-rate.md` §3 配置项表加一行引用。
- **回滚**：功能不依赖表结构（新列留空即原价）；代码回滚后列保留但不生效；缓存 schema 版本号随代码回滚自然失效一次。

## 附录 A：改造点清单（按文件）

| 文件 | 动作 |
|---|---|
| `model/user.go` | User 加 `ModelDiscounts` 列；`ToBaseUser` 带上；新增 `GetUserModelDiscounts`（缓存优先） |
| `model/user_model_discount.go`（新） | 解析/校验/上限常量（AGPL 头） |
| `model/user_cache.go` | UserBase 字段、Lua 字段、schema 版本 2→3、`WriteContext`、`RefreshUserModelDiscountsCache` |
| `model/task.go` | `TaskBillingContext.UserModelDiscount` |
| `constant/context_key.go` | `ContextKeyUserModelDiscounts` |
| `relay/common/relay_info.go` | `RelayInfo.UserModelDiscounts`；`GenRelayInfo` 从 context 快照 |
| `relay/helper/price.go` | `HandleGroupRatio` 折叠 + 兜底读取 |
| `types/price_data.go` | `GroupRatioInfo` 两字段；`PreConsumeDetail.UserModelDiscount` |
| `middleware/auth.go` | 组织令牌 actor（≠属主）载入并覆盖 context |
| `service/quota.go` | `PreWssConsumeQuota` 折扣 |
| `service/task_billing.go` | 重算乘快照折扣；任务日志字段 |
| `service/log_info_generate.go` | 消费日志字段 |
| `controller/relay.go` | `TaskBillingContext` 快照写入 |
| `controller/user.go` / `router/api-router.go` | 管理端 GET/PUT 端点 |
| `controller/organization_member.go` / `router/organization-router.go` | 组织端 GET/PUT 端点 |
| `service/organization_policy.go` / `service/organization_access.go` | 新 capability |
| `service/organization_audit.go` | 新审计 action |
| `controller/pricing.go` | 响应附 `user_model_discounts` |
| `i18n/`（en/zh） | 校验错误消息 |
| `web/`（见 §9） | 编辑器 / 展示 / i18n |
| `docs/billing/` | 新文档 |

## 附录 B：验收标准

1. 配置折扣后：同一用户、同一模型，quota 按 d 等比下降；预扣与结算差额为 0（同请求一致性）。
2. 组织成员用组织令牌调用按成员本人折扣计价；非成员、非本组织管理员无法读写他人折扣。
3. 未配置折扣的用户全链路行为与现状完全一致（回归全绿）。
4. 三库矩阵与 Redis 开关组合通过；两类审计记录可见（管理审计 + 组织审计）。
5. 定价页对配置了折扣的用户显示折后价与折扣徽标。
6. 非法输入（0 折、>1、NaN、超限）一律 400，不落库；读路径遇脏数据按 1.0 计并留日志。
