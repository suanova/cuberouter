# 邀请时配置成员模型与额度限制设计

**Date:** 2026-11-26

**Status:** Approved（2026-11-27）

**范围:** 后端 `model/` `service/` `controller/` `router/`、前端 `web/src/features/organization/`

**相关:** `2026-10-16-org-member-model-access-design.md`、`2026-10-17-org-member-quota-limit-design.md`、`AGENTS.md`、`web/AGENTS.md`

## 1. 背景与目标

组织成员已有两项个人限制：可用模型白名单（`organization_members.model_access`）和组织内累计消费上限（`organization_members.quota_limit`），但目前只能在成员加入后，通过成员行菜单的 Model Settings / Add quota 配置。

本设计让组织 owner/admin 在**发邀请时**就为被邀请人设置这两项限制：

1. 邀请弹窗内可设置“可用模型白名单”和“成员消费额度上限”。
2. 限制随邀请记录保存；被邀请人接受邀请时，在同一事务里写入成员行并刷新白名单缓存。
3. 接受后限制照常生效，且仍可用现有两个对话框修改；配额对话框新增 Unlimited 开关，用于清除邀请设错的初始额度（见 §6.4）。
4. 未设置限制的邀请、以及全部既有成员行为不变（零变化）。

### 为什么邀请特殊

- 邀请按邮箱发出，被邀请人可能还没有账号，发邀请时**不存在成员行**，限制没有地方挂；只能先存在邀请记录上，接受时再落到成员行。
- 邀请只能由组织 owner/admin 发起（平台管理员会被服务端显式拒绝），而 `manage_member_models` / `manage_member_quotas` 正好只授予 owner/admin，权限面天然一致。

## 2. 非目标（明确不做）

1. 平台管理员在后台“直接添加成员”时配置限制（后续单独讨论，涉及是否给平台管理员扩权）。
2. 邀请上的模型折扣（本版只做模型白名单 + 额度上限）。
3. 待处理邀请的“单独编辑限制”接口；需要改就撤销或重发。
4. 被邀请人在邀请落地页看到限制详情。
5. 邀请列表的完整限制编辑器（只做摘要列）。
6. 引入 `pending` 成员状态或预创建成员行。
7. 成员配额“直接改成任意更低正数”的一步式 UI。保留现有 Add quota 累加语义；降低到指定值仍是“先 Unlimited 清除，再 Add 目标值”两步。

## 3. 方案比较

| 方案 | 做法 | 结论 |
|---|---|---|
| **A. 邀请表加两列（选定）** | `organization_invitations` 增加 `model_access`、`quota_limit`，与成员表同形；接受时搬进成员行 | 改动最小；审计快照天然带上；接受时写入就是两个字段赋值 |
| B. 邀请表加 JSON 列 | 一个 `member_settings` 存 `{models, quota_limit}` | 未来加设置不用迁移，但本版没有其他设置；不可按列查询，列表/审计里是 JSON 文本，校验还要解包。收益不足 |
| C. 预创建成员行 | 发邀请时先建一条 pending 成员关系持有设置，接受时激活 | 邮箱邀请可能没有对应 user，无法建行；新增成员状态会波及列表、计费、白名单交集、配额预算等全部路径，代价远大于收益 |
| D. 接受后再配置 | 邀请不存设置，接受后由管理员手动配置 | 不满足“邀请时配置”；接受与配置之间存在限制真空 |

## 4. 数据模型

`model/organization_invite.go` 的 `OrganizationInvite` 增加两个**可空**列：

```go
ModelAccess *string `json:"model_access" gorm:"type:text;column:model_access"`
QuotaLimit  *int    `json:"quota_limit" gorm:"column:quota_limit"`
```

- `model_access` 的存储格式与成员表完全一致：`model.MarshalMemberModelAccess` 产出的 JSON 数组字符串；`""`（非 NULL）表示**显式不限制**。
- `quota_limit` 单位 quota；`0`（非 NULL）表示**显式不限制**。
- **NULL = 该邀请没有配置这一项**。这是为了区分“没设置”和“显式设为不限制”：
  - 新 UI 创建的邀请总会显式提交两项（哪怕是空列表 / 0），因此是“显式不限制”。
  - 旧版客户端、列表页“重试发送”（只带 email + role）创建的邀请，两项为 NULL，接受时**保留被邀请人原先的成员设置**（新成员则落到默认空值）。
- 不加默认值、不回填；存量行升级后为 NULL，行为与现状一致。
- `AutoMigrate` 加列；三库（SQLite / MySQL / PostgreSQL）fresh、升级、二次启动幂等验证。

## 5. 后端行为

### 5.1 创建 / 重发邀请

`POST /api/organizations/:id/invitations` 请求体增加可选字段：

```json
{
  "email": "target@example.com",
  "role": "member",
  "force_rotate": false,
  "models": ["gpt-6", "claude-opus-5-1"],
  "quota_limit": 1000000
}
```

- `models` 缺省（字段不存在）= NULL（不配置）；`[]` = 显式不限制。
- `quota_limit` 缺省 = NULL；`0` = 显式不限制。
- Controller / service 请求结构使用 `*[]string`、`*int` 保留“缺省”与“显式”的区别。

校验（在组织行锁事务内完成）：

1. `models` 非 nil：`model.ValidateMemberModelAccess(*models)`（逐条非空、≤255B、≤2000 条、序列化 ≤60KiB）。
2. `quota_limit` 非 nil：`>= 0`。
3. `quota_limit` 非 nil 且 > 0 时按“是否增加承诺”校验：
   - **新建邀请 / 原邀请该项为 NULL**：`allocated := sumOrganizationMemberQuotaLimits(tx, organization)`；要求 `allocated + *quota_limit <= organization.Quota`。
   - **更新已有邀请且原值非 NULL**：新值 ≤ 原值时直接放行（降低/不变不增加承诺）；只有新值 > 原值时才按上面的公式校验。
   - `quota_limit = 0` 任何时候放行。
   - 与成员配额配置同一套预算规则（quota 设计 I6：“只拒绝增加”）。注意不能像成员配额那样把原值当 `currentLimit` 传：pending 邀请不计入 `allocated`，减掉原值会低估总和；上面的“大于原值才校验”达到同一目的且不会误拒不变的重发。
   - 待处理邀请**不计入** allocated；多个 pending 邀请可以各自通过，接受时允许超额（见 §7）。
4. 能力：`models` 非 nil 需要 `manage_member_models`，`quota_limit` 非 nil 需要 `manage_member_quotas`；owner/admin 均持有，平台角色此前已被拒绝。

持久化：

- **新建邀请**：按请求写入；缺省项保持 NULL。
- **已存在邀请（重开/重发）**：只有请求提供了某项时才更新该列；缺省项保留原值。
  - 列表页“重试发送”只带 email + role，因此保留原限制。
  - 弹窗里的“重新发送”（force rotate）带当前表单值，因此按表单更新。
- **force rotate 幂等哈希**必须包含两个设置字段（用清洗后的 models + quota，nil 与空/0 区分），同一 `Idempotency-Key` 换设置重放视为冲突。

### 5.2 接受邀请

`AcceptInvite` 在既有事务里把邀请上的设置传给 `ReactivateOrganizationMember`：

- 新建成员行：非 NULL 的项写入；NULL 项保持默认（空 / 0）。
- 恢复既有成员行（removed/exited 后再次加入）：
  - 非 NULL 的项**覆盖**旧值（邀请是加入的条款）。
  - NULL 的项**保留**旧值（只可能来自不提供设置的旧客户端/重试路径）。
- **已是 active 成员**（邀请创建后又被加入规则自动加入，或被平台管理员直接添加）：只把邀请标记为 accepted，**不改成员行、不应用邀请设置**。旧邀请不应静默改掉已经加入的成员；这同时修正现有“接受邀请会覆盖角色”的潜在问题。审计仍记录邀请接受。
- disabled 成员：保持现有拒绝（`organization member disabled`），邀请保持 pending。
- 接受时**不再校验预算**：沿用 quota 设计 I6，“成员重新加入等非配置动作允许超额”，避免平台调低组织额度后邀请无法接受。组织总额度仍是最终硬约束。
- 事务提交后继续调用 `invalidateUserModelAllowListCaches(userId)`（已有逻辑）；白名单缓存失效后下一次读取回源 DB。
- 接受审计沿用 `organization.invite.accept`，邀请快照包含两项设置。

### 5.3 仅供邀请弹窗的只读接口

新增：

```
GET /api/organizations/:id/member-restriction-options
```

- 权限：`OrganizationReadAccessAuth(ViewMembersFull)`（与成员白名单/配额读取同面）。
- 响应：

```json
{
  "success": true,
  "data": {
    "models": ["gpt-6", "..."],
    "organization_quota": 10000000,
    "allocated": 1000000,
    "over_allocated": false
  }
}
```

- `models`：组织基组可见模型全集，`service.GetGroupsEnabledModels([NormalizeOrganizationGroup(organization.Group)])`，与现有 Model Settings 对话框同一口径。
- `organization_quota` / `allocated` / `over_allocated`：与 `GET /:id/members/:userId/quota` 同一口径（active + disabled 非 owner 的限额之和）。

### 5.4 权限与审计

| 动作 | 权限 | 审计 |
|---|---|---|
| 创建/重发邀请（带设置） | `manage_invitations` + 对应 `manage_member_models` / `manage_member_quotas` | 既有 `organization.invite.create/resend/reopen`，before/after 快照含 `model_access` 原始字符串与 `quota_limit` |
| 接受邀请 | 邮箱匹配（既有） | 既有 `organization.invite.accept` |
| 读取选项接口 | `ViewMembersFull` | 无 |

- 接受时不再单独记一条成员设置审计：邀请快照就是本次设置的记录；与直接添加成员不单独记设置审计同口径。

## 6. 前端

### 6.1 API / 类型

- `createOrganizationInvite` 请求类型增加 `models?: string[]`、`quota_limit?: number`。
- 新增 `getOrganizationMemberRestrictionOptions(organizationId)` 与 `MemberRestrictionOptions` 类型。
- `OrganizationInviteRow` 增加 `model_access?: string | null`、`quota_limit?: number | null`（列表摘要用；`model_access` 是 JSON 数组字符串，前端防御式解析，空/非法按“无限制”）。

### 6.2 邀请弹窗

`organization-invite-dialog.tsx`：

- 在 email / role 下方增加一个默认折叠的 **Model and quota restrictions** 区块（`Collapsible`，已有 `web/src/components/ui/collapsible.tsx`）。
- 区块内容：
  - 模型多选（`MultiSelect`），候选来自选项接口的 `models`；空 = 不限制。
  - 额度上限输入（绝对值，展示货币；复用 `parseQuotaFromDollars` / `formatQuota`）；空或 0 = 不限制。
  - 预算提示：组织额度、已分配、可分配；`over_allocated` 时显示既有超额警告。
  - 文案说明“这是加入时的初始限制，加入后仍可在成员页调整”。
- 区块按 `can_manage_member_models` / `can_manage_member_quotas` 分别渲染；两者都无时不显示、也不请求选项接口。owner/admin 发邀请时两者恒有。
- 提交：form 始终显式发送 `models`（数组）与 `quota_limit`（转换后的 quota；空 = 0）。
- 客户端校验：额度金额必须是有限正数或留空；转换后为 0 且输入非空 → “金额过小”；超过可分配额度 → 阻止提交并提示（后端仍是最终裁判）。
- force rotate 的 `pendingResend` 保存完整表单值，确认重发时带上同样设置。
- 列表页“重试发送”继续只发 `{email, role}`，后端据缺省语义保留原限制。

### 6.3 邀请列表

`organization-invites-section.tsx` 增加一个“Restrictions”列（默认可见，可随列可见性菜单隐藏）。列必须区分 NULL（未配置，接受时保留成员原设置）与显式空 / 0（接受时清除限制），否则会误导：

- 两项都为 NULL → `Not configured`。
- `model_access` 为 NULL → `Models unchanged`；显式空列表 → `All models`；非空 → `{{count}} models`。
- `quota_limit` 为 NULL → `Quota unchanged`；0 → `No limit`；> 0 → `formatQuota(quota_limit)`。
- 用 ` · ` 连接；`model_access` 解析失败（非法 JSON）按 `Models unchanged` 处理。

### 6.4 成员配额对话框：Unlimited 开关

改造 `organization-member-quota-dialog.tsx`，**保留现有“Amount to add”累加语义**，另加一个 API Key 风格的开关（`organization-token-form-dialog.tsx` 已有同款交互）：

- 开关标签 `Unlimited Quota`，描述复用 token 表单的 `Usage is still limited by what the organization has left.`。
- 打开开关：隐藏/禁用“Amount to add”，保存发送 `limit = 0`（清除个人上限；组织总额度仍是最终上限）。
- 关闭开关：保持现状——输入追加金额，保存发送 `当前上限 + 追加金额`。
- 初始状态：当前 `limit === 0` 时开关打开，否则关闭。
- 保存按钮可用条件：开关打开且当前 `limit !== 0`，或开关关闭且输入了有效正金额。
- 降到某个更低的正值仍是两步：先打开开关保存（清零），再关闭开关 Add 目标值。这是本版接受的取舍（§2 非目标 7）。
- 后端接口不变：`PUT /:id/members/:userId/quota` 本来就接收绝对值，`0` 合法且预算校验放行。

### 6.5 i18n

新增文案走 `useTranslation()`，英文原文作 key，同步 `web/src/i18n/locales/{en,zh,zh-TW}.json`（`bun run i18n:sync`）。复用现有 key：`No limit`、`Remaining allocatable`、`Select allowed models; leave empty for no limit`、超额警告等。

## 7. 不变式与边界

- **I1 零变化**：未配置限制的邀请（NULL/NULL）与所有既有成员行行为不变。
- **I2 邀请只是“加入时的初始值”**：接受后仍通过现有 PUT 修改；邀请不锁死后续配置。
- **I3 显式 vs 缺省**：NULL = 不配置（接受时保留旧值）；空列表 / 0 = 显式不限制（接受时清空旧限制）。
- **I4 预算**：发邀请时占用预算校验；接受时允许超额。多个 pending 邀请可同时承诺同一份组织额度，接受后 UI 显示 over-allocated；组织总额度仍是最终硬约束。
- **I5 模型全集口径**：与现有成员 Model Settings 一致，取组织基组（`organization.group`）的可用模型；不是每个 token 可选组的并集。
- **I6 权限一致性**：带设置的发邀请同时受 `manage_member_models` / `manage_member_quotas` 约束；不持有对应能力时拒绝（不是静默忽略）。
- **I7 状态边界**：组织 dissolved/disabled、邀请过期/已接受、邮箱不匹配等既有拒绝逻辑不变；邀请过期即丢弃设置。
- **I8 返回语义**：邀请列表/创建响应的 `model_access` 为存储原文（JSON 数组字符串），与成员表存储同构；`quota_limit` 为 quota 整数。

## 8. 测试计划

### 后端

- **model/迁移**：`organization_invitations` 新列存在（扩展 `TestOrganizationFoundationAutoMigrateCreatesRequiredColumns`）；三库 fresh / 旧表升级 / 二次启动幂等。
- **service/邀请**：
  - 新建邀请带 models/quota 正确落库；缺省项为 NULL。
  - 非法 models（超长、超量、过大）拒绝；负 quota 拒绝；超预算拒绝；`quota_limit = 0` 通过。
  - 重发预算：原值不变或降低放行；提高且超预算拒绝；原值为 NULL 的更新按新建规则校验。
  - 重发（不提供设置）保留原列；force rotate（提供设置）更新列；同一幂等键换设置 → 冲突。
  - 平台角色仍不能发邀请（既有用例回归）。
- **service/接受**：
  - 新成员接受后成员行获得设置。
  - 曾经成员（removed/exited）接受后，非 NULL 项覆盖旧值、NULL 项保留旧值。
  - 已是 active 成员时接受：邀请变 accepted，成员角色与设置不变。
  - disabled 成员接受仍被拒绝，邀请保持 pending。
  - 组织额度已调低时接受仍成功（允许超额）。
  - 白名单缓存失效路径（Redis 开关两形态）。
- **选项接口**：模型全集、组织额度/已分配/over-allocated；`ViewMembersFull` 权限矩阵；无成员也返回全集。
- **审计**：邀请 create/resend/reopen 的 before/after JSON 含两项设置。

### 前端

- 邀请弹窗：区块展开/折叠；模型多选与额度输入提交正确载荷；空额度发 0；金额过小/超可分配阻止提交；服务端失败对话框不关闭、保留值；force resend 带设置。
- 邀请列表：摘要列渲染（两项 NULL / 单项 NULL / 显式空列表 / 0 / 有值）；列表“重试”请求体不含 `models` / `quota_limit`。
- 成员配额对话框：Unlimited 开关初始状态（`limit` 为 0 / 非 0）；打开后保存 0 并禁用金额输入；关闭后保持累加；当前为 0 时关闭并输入金额保存该金额；无有效变更时保存不可用。
- `bun run typecheck`；涉及文件 `bun run lint`。

### 三库验证

按 `AGENTS.md`：SQLite、MySQL（≥5.7.8）、PostgreSQL（≥9.6）各至少一个真实实例；新列加列、存量行默认 NULL、迁移幂等；外部库用例沿用 `TEST_MYSQL_DSN` / `TEST_POSTGRES_DSN` 约定。将版本、命令、结果写进 PR。

## 9. 迁移与回滚

- 迁移：`AutoMigrate` 加两列，无回填；存量邀请为 NULL（= 不配置）。
- 回滚：代码回滚后两列保留但不生效；已用设置发起的待处理邀请，在旧代码下接受会丢失设置（可接受）。
- 数据清理：把两列置 NULL 即回到“未配置”。

## 10. 改动面预判

| 面 | 文件 |
|---|---|
| 数据模型 | `model/organization_invite.go` |
| 邀请服务 | `service/organization_invite.go`（请求归一化、预算校验、持久化、幂等哈希） |
| 成员服务 | `service/organization_member.go`（`ReactivateOrganizationMember` 增加设置参数；新增选项读取函数） |
| 控制器 | `controller/organization_invite.go`（请求字段）、`controller/organization_member.go`（选项接口） |
| 路由 | `router/organization-router.go`（新只读路由） |
| 后端测试 | `model/organization_test.go`、`service/organization_invite_test.go`、`service/organization_member_test.go`、`controller/organization_invite_test.go` 等 |
| 前端 API/类型 | `web/src/features/organization/{api,types}.ts` |
| 前端组件 | `organization-invite-dialog.tsx`、`organization-invites-section.tsx`、`organization-sections.tsx`、`organization-member-quota-dialog.tsx`（Unlimited 开关） |
| 前端测试 | `web/src/features/organization/components/__tests__/organization-invite-dialog.test.tsx`、`organization-invites-section.test.tsx`、`organization-member-quota-dialog.test.tsx`（扩展现有） |
| i18n | `web/src/i18n/locales/{en,zh,zh-TW}.json` |

## 11. 已定决策与留给实现者的部分

**已定（本设计）：**

- 方案 A：邀请表加两列。
- 两列可空，用 NULL 区分“未配置”与“显式不限制”。
- 发邀请时校验预算；接受时允许超额。
- 已存在邀请的重发：新 quota ≤ 原值直接放行，只有提高时才校验 `allocated + 新值 ≤ 组织额度`。
- 接受时非 NULL 设置覆盖旧成员值，NULL 保留旧值；已是 active 成员则只标记邀请接受、不改成员行。
- 模型全集取组织基组；新增只读选项接口供弹窗使用。
- 邀请列表摘要列区分“未配置 / 显式不限制”。
- 配额对话框保留 Add quota 累加语义，另加 Unlimited Quota 开关（置 0）作为本版清除入口；降到任意更低正值仍需两步。
- 不做待处理邀请的单独编辑接口。

**留给实现者：**

- 邀请弹窗内区块的具体布局、折叠控件细节与文案措辞。
- 是否把额度输入抽成与 Add quota 对话框共享的组件（若抽取，注意现有对话框“追加金额”与本处“初始绝对值”的语义差异）。
- 测试文件拆分与命名、fixture 形态。
- 后端选项读取函数放在 `organization_member.go` 还是邻近文件。
