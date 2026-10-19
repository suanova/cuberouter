# 组织成员自建 Key 默认值（Member Key Defaults）设计

**Date:** 2026-10-19
**Status:** Approved（2026-10-19；实现细节在 reviewer 后补充：平台管理员豁免、复用责任成员锁、幂等哈希保持原始请求）
**基线:** `main` `a652efa`（含 #155 成员模型白名单、#156 成员配额上限）
**范围:** `service/` `controller/` `i18n/`、前端 `web/`

## 1. 背景与目标

组织普通成员（无 `CanManageAllTokens`）目前可以创建组织 Key，并自由填写额度、是否无限、有效期、模型限制、分组、IP 白名单和批量数量。前端默认值就是「无限额度 + 永不过期 + 不限制模型」，因此成员默认能造出无限消耗组织额度的长期 Key。

`2026-10-17-org-member-quota-limit-design.md` §11 已把「成员自建 Key 时的额外约束」留作后续。本次目标是：

- 成员仍然可以创建组织 Key，但**只能填写名称**；其余字段由服务端强制为安全默认值。
- 新建 Key 的额度有限、永不过期、不限制模型（实际模型访问仍受成员白名单实时约束）、使用组织分组、私有、责任人为自己。
- 成员不能一次创建多把 Key；批量数量不开放。
- owner / 组织管理员不受影响，仍可完整配置。
- **只影响新建 Key**：存量成员和存量 Key 的字段与行为不变；`quota_limit = 0` 仍表示成员级不限制。

## 2. 非目标

1. **不引入未配置成员的跨 Key 默认 1% 累计上限**。1% 只是新 Key 的初始额度，不是成员级闸门。未配置成员建多把新 Key 时，每把有限，但合计可能超过 1%；组织总额度仍是最终硬约束。需要跨 Key 硬上限时，管理员设置现有成员配额（`quota_limit`）。
2. 不回写、不迁移、不重算存量 Key。存量 `unlimited_quota` Key 保持原样。
3. 不改变成员配额的语义：`quota_limit = 0` 仍是「不限制」，`used_quota` 仍是累计实际消费。
4. 不改变 owner / 组织管理员 / 平台管理员的权限与表单。
5. 不新增数据库列或迁移。
6. 不限制个人 Key；本设计只作用于 `scope_type = organization` 的组织 Key。
7. 不阻止成员反复单把创建 Key；只关闭「一次建多把」。
8. 不改变组织 Key 列表的可见列；成员仍能看到自己 Key 的额度、有效期和模型限制。

## 3. 关键决策

| 决策 | 选择 | 理由 |
|---|---|---|
| 受限对象 | 普通成员：`!actor.Capabilities.CanManageAllTokens && !actor.IsPlatformAdmin` | owner / 组织管理员 / 平台管理员继续完整配置；平台管理员在 workspace 面仍按平台身份豁免，保留现有批量创建行为 |
| 强制位置 | 服务端在 Create / BatchCreate / Update 内归一化；前端隐藏只是体验 | API 可直接伪造字段 |
| Key 额度 | 强制 `UnlimitedQuota=false`，`RemainQuota` 按 §4 计算 | 单把 Key 不再无限 |
| 已配置成员（`quota_limit > 0`） | 默认额度 = `max(1, quota_limit - used_quota)`；剩余 ≤ 0 时拒绝创建 | Key 不超过成员剩余预算；跨 Key 的累计闸门仍由现有成员配额负责 |
| 未配置成员（`quota_limit = 0`） | 默认额度 = `min(org.quota - org.used_quota, org.quota / 100)`，至少 1；组织无剩余时拒绝创建 | 1% 组织总配额，且不超过实际剩余；不写回成员配额 |
| 有效期 | `expired_time = -1`（永不过期） | 用户决定；成员退出/停用已有系统 blocker 兜底 |
| 模型限制 | `model_limits_enabled = false`，`model_limits = ""` | 跟随实时「组织分组模型 ∩ 成员白名单」，避免快照过期 |
| 分组 | `group = ""`（组织分组） | 成员不能选择其他可用分组 |
| IP 白名单 | `allow_ips = ""` | 不限制 |
| 可见性 / 责任人 | `private` / 操作者本人 | 沿用成员现有权限规则 |
| 跨分组重试 | `false` | 只在 `group = auto` 时有意义，成员不开放 |
| 批量数量 | 成员 `token_count > 1` 直接拒绝 | UI 隐藏数量；不静默改成 1 |
| 编辑 | 成员只能改 `name` 和 `status`；其余字段保留库中原值 | 防止「建完再改」绕过创建默认值 |
| 存量数据 | 不迁移、不改写 | 用户明确「只对新 Key 生效」 |

## 4. 默认额度计算

记：

- `L = organization_members.quota_limit`
- `U = organization_members.used_quota`
- `Q = organizations.quota`
- `QU = organizations.used_quota`

### 4.1 管理员已配置成员额度（`L > 0`）

- `remaining = L - U`
- `remaining <= 0` → 拒绝创建，返回 `ErrOrganizationMemberQuotaExhausted`
- 否则 `keyQuota = remaining`

### 4.2 管理员未配置成员额度（`L = 0`）

- `orgRemaining = Q - QU`
- `Q <= 0 || orgRemaining <= 0` → 拒绝创建，返回 `ErrOrganizationQuotaUnavailable`
- `keyQuota = min(orgRemaining, Q / 100)`
- `keyQuota < 1` → `keyQuota = 1`（仅在 `orgRemaining >= 1` 时成立）

### 4.3 说明

- 单位为 quota，整数除法；`Q / 100` 向下取整。
- 默认额度在创建时快照；之后组织额度或成员配额变化不重算存量 Key。
- 1% 不写入 `organization_members.quota_limit`，不占「成员配额预算总和」。
- 已配置成员的 Key 默认额度按「创建时剩余成员额度」计算；多把 Key 在创建时可能各自拿到相同的剩余值，但实际累计消费仍由成员配额闸门卡住。
- 未配置成员的 1% 是单 Key 默认值，不是跨 Key 总额度；见 §2 非目标 1。
- 若计算值超过现有单 Key 上限（`maxTokenQuota()` / `validateOrganizationTokenRequest` 的上限），按现有上限截断；实现时复用现有上限常量，不新增魔法数。

## 5. 服务端强制点

### 5.1 受限判定

只有普通成员应用：`!actor.Capabilities.CanManageAllTokens && !actor.IsPlatformAdmin`。owner、组织管理员、平台管理员（包括在 workspace 面以成员身份进入的平台管理员）不受影响；后者保留现有批量创建能力。责任人是别人（管理员代建）时也不应用默认值；是否应用只看**操作者**。

### 5.2 CreateOrganizationToken

在组织行锁和 actor 解析之后：

1. `responsibleUserId = operatorUserId`（成员不能替他人建 Key，现有检查已拒绝）。
2. 复用 `ensureOrganizationTokenResponsibleUserWithTx` 已有的责任成员行 `FOR UPDATE` 读：把该函数改为返回锁定后的 `*model.OrganizationMember`，用其中的 `quota_limit`、`used_quota` 计算；**不要新增第二次成员行锁**，否则会改变 `TestOrganizationTokenResponsibleMemberWritesLockTargetMember` 的锁计数契约。组织余额使用 `actor.Organization`（在组织行锁之后读取，计费写组织行会与此锁串行）。
3. 按 §4 计算 `keyQuota`；无法计算时返回哨兵错误，整个事务回滚。
4. 用以下有效值构造 `model.Token`，忽略客户端传的对应字段：
   - `Name` = trim 后的名称（必填、≤50）
   - `Status` = enabled
   - `ExpiredTime` = -1
   - `RemainQuota` = `keyQuota`
   - `UnlimitedQuota` = false
   - `ModelLimitsEnabled` = false
   - `ModelLimits` = ""
   - `AllowIps` = 指向空字符串
   - `Group` = ""
   - `CrossGroupRetry` = false
   - `Visibility` = private
   - `ResponsibleUserId` = operator
5. 审计记录写入实际生效的值。

### 5.3 BatchCreateOrganizationTokens

- 成员 `TokenCount > 1` → 返回错误（建议新增 i18n 文案「成员不能一次创建多把 Key」）。
- `TokenCount == 1` 时走与单把创建相同的默认值路径。
- 默认额度在循环外计算一次；批量路径本来就给每把 Key 相同额度，行为一致。
- 幂等哈希保持按原始请求计算（现有行为）；成员 UI 只发送名称，重放稳定；默认额度不参与哈希。

### 5.4 UpdateOrganizationToken

对普通成员：

- 允许修改：`name`、`status`。
- 从库中现有 Token 原样保留：`expired_time`、`remain_quota`、`unlimited_quota`、`model_limits_enabled`、`model_limits`、`allow_ips`、`group`、`cross_group_retry`、`visibility`、`responsible_user_id`、`user_id`。
- 现有的「成员不能改责任人」「不能发布 public」检查继续生效；成员尝试把 private 改成 public 仍然报 `permission denied`，不静默保留。
- 现有的模型白名单校验只在 limits 或责任人实际变化时触发；保留原值不会误触发。
- 管理员/owner 路径保持完整字段更新。

### 5.5 校验顺序

实现上把 `validateOrganizationTokenRequest` 从事务外移到 actor 解析之后、对**有效请求**校验：

- 成员：先归一化（只保留 name/status 与服务端默认值），再校验名称/状态。
- 管理员/平台管理员：按原请求完整校验。
- 分组校验同样移到归一化之后，避免被忽略的客户端 `group` 触发错误。
- 批量路径的 `token_count` 上限检查同样在 actor 解析后按角色处理；成员 `>1` 直接拒绝，不做静默截断。

### 5.6 并发与一致性

- `lockOrganizationTokenWriteWithTx` 保持现状（锁组织行）；不需要扩展它的 `SELECT`，因为 `actor.Organization` 是在该锁之后、同一事务内读取的，行在此期间不会变化。
- 成员配额行复用 `ensureOrganizationTokenResponsibleUserWithTx` 的 `FOR UPDATE` 读，把函数改为返回锁定的成员行；不要新增额外锁，避免改变现有锁计数测试。
- 计费侧的 `updateOrganizationUsedQuotaTx` 先更新组织行、再更新成员行；建 Key 持有组织行锁期间计费会等待，因此锁后读到的组织余额和成员计数器都是一致的。
- 不在事务外使用 `actor.Organization` 或 `actor.Member` 的旧快照计算额度。

### 5.7 错误面

新增 service 哨兵错误：

- `ErrOrganizationMemberQuotaExhausted`：`"organization member quota is exhausted"`
- `ErrOrganizationQuotaUnavailable`：`"organization quota is unavailable"`

`writeOrganizationError` 映射为 `403`，复用现有错误码：

- `types.ErrorCodeInsufficientOrganizationMemberQuota`
- `types.ErrorCodeInsufficientOrganizationQuota`

前端 `ORGANIZATION_TOKEN_ERROR_MESSAGE_KEYS` 为这两个 code 增加可读文案（en 源 key，再同步各语言）：

- 「成员配额已用尽，无法创建新的组织 Key。请联系组织管理员调整成员配额。」
- 「组织额度不可用，无法创建新的组织 Key。」

这两类是业务拒绝，不记录系统错误日志。

## 6. API / 权限

- 不新增路由，不改现有 API 形状；默认值由 `POST /api/organizations/:id/tokens`、`POST /api/organizations/:id/token-batches`、`PATCH /api/organizations/:id/tokens/:tokenId` 内部处理。
- 不新增 capability；沿用 `CanManageAllTokens` 区分普通成员与管理角色。
- 成员仍可创建/编辑/删除自己的组织 Key（现有 `organizationPolicyCanManageTokenResource` 规则不变）。
- 成员创建成功后 API 返回完整 Token（含实际 `remain_quota`、`expired_time`），前端据此展示结果。

## 7. 前端

### 7.1 组织 Key 创建/编辑对话框

`web/src/features/organization/components/organization-token-form-dialog.tsx` 已接收 `canManageAllTokens`。按角色分叉：

- **普通成员 · 创建**
  - 只渲染「名称」输入框和一行只读说明：「额度、有效期和模型由组织策略自动设置。」
  - 隐藏：创建数量、可见性、责任人、无限额度、额度、有效期、模型限制、IP 白名单、分组、跨分组重试。
  - 提交只发送 `{ name }`；服务端填默认值。
  - 始终调用单把创建接口，不调用批量接口。
  - 创建结果弹窗展示服务端返回的实际额度与「永不过期」，让成员知道拿到的规格。
- **普通成员 · 编辑**
  - 只渲染名称；启停沿用列表行现有操作；其余字段以只读摘要展示（额度、有效期、模型限制、分组）。
  - 保存时表单把库中原值回传（现有 `transformOrganizationTokenFormToPayload`）；服务端仍会保留受限字段。状态切换走现有 `buildOrganizationTokenStatusPayload`。
- **管理员 / owner**
  - 现有表单完全不变（数量、可见性、责任人、额度、有效期、模型、IP、分组、跨分组重试都可配）。

### 7.2 成员配额设置提示

在组织的 Member Settings / Quota Limit 区块增加说明：

- 未设置（0）时：成员新建组织 Key 的默认额度为组织总配额的 1%，且不超过组织剩余额度；这不代表成员跨 Key 的硬上限。
- 已设置时：成员新建组织 Key 的默认额度为剩余成员配额；跨 Key 累计消费仍受成员配额约束。

提示只影响文案，不改变 `quota_limit = 0` 的现有语义。

### 7.3 i18n

新增文案使用英文源 key，写入 `web/src/i18n/locales/en.json` 等文件；运行 `bun run i18n:sync` 补齐其他语言。后端沿用现有英文错误文案，由前端按错误码映射，不需要新增后端 i18n 资源。

## 8. 测试计划

### 8.1 Service 测试（新增 `service/organization_token_member_defaults_test.go`）

1. 未配置成员（`quota_limit = 0`），组织额度 10000 / 已用 0 → Key 额度 100。
2. 未配置成员，组织额度 10000 / 已用 9950 → Key 额度 50（`min(50, 100)`）。
3. 未配置成员，组织额度 50 / 已用 0 → Key 额度 1（整数除法为 0，按最小值 1）。
4. 未配置成员，组织额度 0 或剩余 ≤ 0 → 拒绝。
5. 已配置成员（`quota_limit = 500`，`used_quota = 200`）→ Key 额度 300。
6. 已配置成员，`used_quota >= quota_limit` → 拒绝。
7. 成员创建时 `unlimited_quota / expired_time / model_limits / allow_ips / group / cross_group_retry / visibility / responsible_user_id` 全部被强制为默认值。
8. 管理员/owner 创建同样的请求 → 字段原样保留（含无限额度、有效期、模型、分组、IP）。
9. 成员批量创建 `token_count > 1` → 拒绝；`token_count == 1` → 正常默认值。
10. 成员编辑 Key：尝试改额度/有效期/模型/分组/IP/无限额度 → 库中原值不变；改名称/启停成功。
11. 管理员编辑 Key → 完整字段可改。
12. 并发：建 Key 与计费同时发生，读取到的成员 `used_quota` 是最新值（锁行为测试）。
13. 存量 Key 创建后不因新规则改变（可直接断言建 Key 前的行不变）。

### 8.2 前端测试

- 成员创建对话框只渲染名称与说明；不渲染数量/额度/有效期/模型等字段；提交只带名称。
- 成员编辑对话框只渲染名称与启停，受限字段只读。
- 管理员对话框保持原字段与批量创建。
- 创建结果弹窗展示实际额度/永不过期。
- Member Settings 的默认额度提示。
- 错误码 `insufficient_organization_member_quota` / `insufficient_organization_quota` 的文案映射。

### 8.3 数据库与全量验证

本功能不新增列/迁移，但新增了 `FOR UPDATE` 行锁与事务内读取，属于数据库行为改动。按 AGENTS.md：

- 跑 SQLite / MySQL 5.7.44 / PostgreSQL 15.19 的 service 测试矩阵，覆盖建 Key 的锁路径与并发测试。
- 跑 `go test`（排除 main 包的 CI 等价命令）+ `-race`；前端跑类型检查、lint、相关测试。
- 不需要 `AutoMigrate` 升级验证（无 schema 变化），但需确认启动不产生额外迁移 SQL。

## 9. 回滚

- 代码回滚即可；无数据迁移、无新列。
- 回滚后新规则失效，已创建的有限额度 Key 保持有限（不自动变回无限），不影响使用；成员可删除后重建。
- 存量 Key 从未被改写，回滚不产生数据差异。

## 10. 改动面预判

| 面 | 文件 |
|---|---|
| 服务 | `service/organization_token.go`（默认值计算、Create/Batch/Update 归一化、锁内读取）、`service/organization_token_test.go` 等现有测试 |
| 错误 | `service/` 新哨兵错误、`controller/organization.go` 的 `writeOrganizationError` 映射、`types/organization_error.go`（如需新码则不加，复用现有） |
| 控制器 | `controller/organization_token.go`（校验顺序调整如涉及） |
| i18n | `web/src/i18n/locales/*.json` |
| 前端 | `web/src/features/organization/components/organization-token-form-dialog.tsx`、`organization-tokens-section.tsx`（批量入口显隐）、相关 `lib` 与测试 |
| 成员配额提示 | `web/src/features/organization/components/organization-member-dialogs.tsx` 或配额区块组件 |
| 文档 | 本设计文档 |

## 11. 验收标准

1. 普通成员创建组织 Key 时 UI 只有名称；API 直接伪造额度/有效期/模型/分组/IP/数量也不会生效。
2. 未配置成员的新 Key 额度 = `min(组织剩余额度, 组织总配额 / 100)`，至少 1；组织无剩余时拒绝并给出明确错误。
3. 已配置成员的新 Key 额度 = 创建时剩余成员配额；成员配额用尽时拒绝。
4. 新 Key 永不过期、私有、责任人自己、组织分组、无模型限制、无 IP 限制、无跨分组重试。
5. 成员编辑只能改名称/启停；受限字段保持原值。
6. 管理员/owner 创建与编辑行为完全不变。
7. 存量成员和存量 Key 无任何字段/行为变化。
8. 三库（SQLite / MySQL 5.7.44 / PostgreSQL 15.19）事务与锁测试通过；Go 全量测试 + `-race` 通过；前端类型检查/lint/相关测试通过。
