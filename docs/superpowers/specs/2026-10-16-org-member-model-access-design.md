# 组织成员模型白名单（Member Model Access）设计

**Date:** 2026-10-16
**Status:** Draft（用户已授权按理解推进）
**范围:** 后端 `model/` `service/` `controller/` `router/` `middleware/` `constant/` `i18n/`，前端 `web/`
**相关:** 用户×模型折扣设计（同批组织治理面）、`docs/ORGANIZATION_PRD.md`、`AGENTS.md`

## 1. 目标

组织管理员为组织成员选择**可用/可见模型**（成员级白名单）：

- 默认（未配置）成员可见可用该组织全部模型——零行为变化。
- 配置后：1) 成员只**看到**被选模型（定价页、Playground、建 key 模型下拉、模型列表接口）；2) 成员**自建 key** 能用的模型也受同样限制；3) 运行时调用非白名单模型被拒绝。

动机：企业/组织场景下收窄特定成员的模型范围（成本、合规、防误用），且与刚刚落地的组织面治理（成员折扣）形成同一套「组织管成员」的能力族。

## 2. 非目标（明确不做）

1. 不做组织级/组织 Key 级模型限制（本版只做成员级）。
2. 不做平台侧给任意普通用户设模型限制（平台管理员走组织入口或用分组；通用用户级白名单留待将来）。
3. 不展示限制原因/申请流程；不做模型级用量报表联动。
4. 不改写存量令牌的 `model_limits` 数据（运行时交集天然覆盖，见 I3）。
5. 白名单只做**收窄**，绝不放宽任何既有约束（分组/可用组/令牌限制/渠道可用性）。

## 3. 关键决策

| 决策 | 选择 | 理由 |
|---|---|---|
| 归属 | 挂在**组织成员关系**上（`organization_members.model_access`，org × member 一行） | 权限主体与配置主体一致；退出组织 = 该行非 active → 限制自动解除，无需清理逻辑 |
| 生效范围 | **跟随人**：成员本人所有调用与视图（个人 key、他负责的组织 key、他发起的任务），与折扣功能同先例 | 「成员自己创建 key 也受限」意味着个人上下文生效；组织管理员管的是"这个人" |
| 多组织组合 | **交集**：有效集 = 其 user 的**全部 active memberships** 授予集之交；未配置的组织视为"全部"（对交集无影响）；无 active membership = 不限制 | 每个管理员的显式限制都必须成立，不被无关的第二个组织静默作废；单组织场景两种语义等价 |
| 平台管理员豁免 | 用户 role ≥ admin（10）时**不适用**白名单（读取器直接返回 nil） | 防组织管理员连锁限制平台运维人员；防自锁 |
| 存储 | `organization_members.model_access` TEXT（JSON 数组；空串 = 不限制）；上限 2000 条 / 60 KiB（MySQL TEXT 64 KiB 上限，含审计快照余量） | 与折扣列同型；成员数为量级可控 |
| 有效性判定 | 只统计 `status = active` 的成员关系 | 退出/被禁用后限制不再生效（disabled 成员本就无法使用组织资源；个人资源不受组织 disable 影响） |
| 读取路径 | 新增缓存 getter `GetUserModelAllowList(userId)`（Redis 缓存优先、DB 兜底；nil = 不限制），写后显式失效 | 运行时每请求需要；绝大多数用户无限制（nil 快路径） |
| PUT 语义 | body `{"models":[...]}`；**非空 = 白名单；空数组 = 解除限制**（回到默认全量） | "零模型"无意义（应禁用成员），空数组天然表达"取消限制" |
| 能力 | 新 capability `manage_member_models`（owner/admin，与 `manage_members` 同级授予） | 与 `manage_member_discounts` 同模式，可独立授权 |
| 审计 | `organization.member.model_access.update`（target_type=member，before/after） | 与成员其他治理操作同口径 |

## 4. 语义与不变式

- **I1 默认零变化**：会员未配置（nil）时，可见性/创建/运行时/定价页行为与现状完全一致（回归硬要求）。
- **I2 跟随人**：限制以 user 为键生效；同一 user 无论用哪把 key（个人/组织/责任成员身份）都被限制。
- **I3 只收窄**：运行时有效可用集 = 组织白名单 ∩ 令牌自身 model_limits（若启用）∩ 既有分组/渠道可用性；建 key 时校验 `model_limits ⊆ 白名单`，越界 400。存量令牌不改写。
- **I4 交集组合**：多组织 = 各 active membership 授予集之交；未配置=全集（不影响交集）。
- **I5 管理员豁免**：role ≥ admin 的用户不适用（读取器返回 nil）。
- **I6 平台/组织隔离**：白名单不做跨用户归属；除成员本人外，仅平台管理员与组织管理面可见成员的白名单配置。
- **I7 配置面守卫**：PUT/GET 目标须为**本组织 active 成员**，沿用 `ensureOrganizationMemberTargetAllowed` 的目标等级校验（与折扣/成员更新一致）。

## 5. 数据与缓存

1. `model/organization_member.go`（或组织成员模型文件）：`OrganizationMember.ModelAccess string`（`gorm:"type:text;column:model_access"`，空串=不限制，`json:"-"` 或按需）。
2. `model` 层：`ParseMemberModelAccess(raw) []string`（宽松解析：JSON 数组、去重、逐条校验非空 ≤255B、上限 2000 条；非法整体 → nil + SysError）；`MarshalMemberModelAccess([]string) string`（排序、空 → `""`）。
3. `model.GetUserModelAllowList(userId int) []string`：
   - role ≥ admin → nil（I5）；
   - 查询 `organization_members` 中该 user 全部 `status=active` 行的 `model_access`：全部为空 → nil；否则逐行求交（空行 = 全集，跳过）；交集为空 → 返回**空列表**（表示"不允许任何模型"，与 nil 语义区分）；
   - Redis 缓存（键如 `user:model_allowlist:<id>`，JSON 存 `{"nil":true}` 或数组；TTL 沿用用户缓存惯例），命中原样返回。
4. 失效点（写后显式刷新/删除该 user 的缓存；失败仅 SysError，TTL 自然收敛）：
   - 成员白名单 PUT（本功能）；
   - 成员新增/移除/退出/禁用/启用（`service/organization_member.go` 既有路径）；
   - 组织禁用/解散（成员整体失效）；
   - 加入规则自动加入（新增 membership）。
   计划阶段逐点核对实际函数名。
5. 所有读写走 `common.*` JSON 封装；三库矩阵 + 迁移测试（fresh / 升级加列 / 二次启动幂等）。

## 6. 强制点（全部为"过滤/收窄"，不新增放行逻辑）

| # | 位置（当前行号为 25aeff4 附近） | 行为 |
|---|---|---|
| 1 | `middleware/distributor.go:82-96`（令牌模型限制块，选渠前） | 追加白名单判定：`matchName` 不在有效集 → 拒绝（i18n 消息：模型不在组织允许范围；错误码沿用权限类 403）。仅在 allow-list 非 nil 时执行（快路径零成本） |
| 2 | `middleware/distributor.go:565`（第二处 model-limit 检查） | 计划阶段核实该分支语义后同样处理 |
| 3 | `controller/user.go:703 GetUserModels` | 返回集 ∩ 白名单（nil 直通）。覆盖：Playground、个人密钥创建/编辑模型下拉、组织 token 表单、概览页 |
| 4 | `controller/pricing.go GetPricing` | `pricings` 列表按白名单过滤（与 usable-group 过滤并列；不影响缓存对象——在按请求副本上过滤） |
| 5 | 令牌创建/更新（个人）：计划阶段定位 controller/token.go 的 model_limits 校验点 | 请求带 `model_limits` 时校验 ⊆ 白名单，否则 400（i18n） |
| 6 | 组织令牌创建/更新（`service/organization_token.go:318/543/673` 一带） | 同上：由责任成员视角校验 |
| 7 | `/api/user/models` 的 `:id` 变体（管理员查某用户） | 同样过滤（保持与本人一致视图） |

**不做**的运行时点：计费/任务提交之外的旁路（渠道测试等）不引入白名单（测试者是管理员，且 I5 已豁免）。

## 7. API / 权限 / 审计

- `GET /api/organizations/:id/members/:userId/model-access` — `OrganizationReadAccessAuth(ViewMembersFull)`；响应 `{"success":true,"data":{"access":[...],"models":[<org 组可见模型全集>]}}`（一次拉齐对话框数据；`access` 为空数组 = 当前不限制）。
- `PUT /api/organizations/:id/members/:userId/model-access` — `OrganizationManagementAuth(manage_member_models)`；body `{"models":[...]}`（空数组=解除）；校验：目标 active 成员 + `ensureOrganizationMemberTargetAllowed` + 各模型 ≤255B、≤2000 条、体积上限；写入事务内 + `recordOrganizationAudit(organization.member.model_access.update, target_type=member, before/after)`；提交后失效该 user 缓存。
- org 模型全集 = `service.GetGroupsEnabledModels([normalizeOrganizationGroup(organization.Group)])`。
- i18n（后端）：新增消息键（校验类 + 运行时拒绝类），en/zh。

## 8. 前端

1. 组织中心成员行操作区（`organization-members-section.tsx`，与「Model Discounts」入口同级）新增「Model Access」入口，门控 `can_manage_member_models`（capability schema + prop 链同折扣功能先例）。
2. 新对话框 `organization-member-model-access-dialog.tsx`：搜索框 + 全选/清空 + 复选列表（候选=GET 返回的 `models`，已选=`access`）+ 「限制全部/解除限制」快捷；保存 PUT，成功 toast + refetch；空选择 = 解除限制（文案明确）。全量替换语义，GET 未落地前禁存。
3. i18n：`Model Access`、`Member Model Access`、`All models are available by default.`、`Clear to remove the restriction` 等，`bun run i18n:sync`。
4. 不新做成员侧提示（成员视图就是被过滤后的结果）。

## 9. 测试计划

- Go：解析/序列化（宽松、去重、上限、空=不限）、`GetUserModelAllowList`（admin 豁免 / 无组织 nil / 单组织 / 多组织交集 / 全空行=全集 / 交集为空=空列表 / 缓存刷新）、distributor 运行时拒绝（白名单 × 令牌限制 交集矩阵：allowed+no-token-limit → 403；allowed+token-limit 内 → 放行；nil → 原行为）、GetUserModels/GetPricing 过滤（nil 直通 bit 级回归）、令牌创建校验（越界 400/子集通过/不带 limits 通过）、组织端 API 权限矩阵 + 审计落库 + 缓存失效、i18n 键。
- 前端：对话框交互（加载/勾选/清空解除/保存载荷/失败不关闭）、入口 capability 门控、`__tests__/` 放置。
- 三库矩阵：新列迁移（fresh/升级/幂等）+ 读写；Redis 开关两形态。

## 10. 文档与回滚

- `docs/billing/` 或 `docs/` 新增 `org-member-model-access.md`（口径、生效范围、交集语义、运行时行为、与令牌限制的关系）；`docs/ORGANIZATION_*` 若有成员治理章节则加引用。
- 回滚：清空列 = 恢复原行为；代码回滚后列保留但不生效；缓存键随代码停用自然过期。

## 附录 A：改动面预判（计划阶段细化）

| 面 | 文件（预判） |
|---|---|
| 模型/存储 | `model/organization_member.go`（+列/解析）、新 `model/user_model_allowlist.go`（getter+缓存） |
| 服务 | `service/organization_member.go`（GET/PUT service fns + 失效点接入）、`service/organization_audit.go`（action）、`service/organization_policy.go`/`organization_access.go`（capability） |
| 运行时 | `middleware/distributor.go`（两处）、`controller/user.go`（GetUserModels）、`controller/pricing.go` |
| 令牌校验 | `controller/token.go`（计划阶段定位）、`service/organization_token.go` |
| 路由/控制器 | `router/organization-router.go`、`controller/organization_member.go` |
| 前端 | `features/organization/{api,types,components/organization-member-model-access-dialog.tsx,organization-members-section.tsx,organization-sections.tsx}`、`lib/account-context.ts`（capability）、locales |
| 测试/文档 | 各包 `__tests__` / `_test.go`、迁移测试、docs |

## 附录 B：验收标准

1. 未配置成员：全链路与现状一致（回归全绿）。
2. 组织管理员为成员配置 `["m1","m2"]` 后：该成员 `/api/user/models`、定价页、Playground、建 key 下拉只见 m1/m2；调用 m3 → 403；建带 m3 限制的 key → 400；带 m1 限制的 key 正常。
3. 多组织：成员在 A(白名单)/B(未配置) → 有效 = A 的白名单；A/B 都有 → 交集。
4. 退出/被移除/组织解散后：限制解除（缓存正确失效）。
5. 平台管理员（admin/root）不受任何白名单影响。
6. 三库矩阵 + Redis 开关通过；组织审计可见 before/after。
