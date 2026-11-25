# 组织成员新建 Key 的默认额度（可配置）设计

**Date:** 2026-11-25
**Status:** Draft（待评审）
**基线:** `main` `62c3d81`
**范围:** `model/` `service/` `controller/`、前端 `web/`
**取代:** `2026-10-19-org-member-key-defaults-design.md` §2 非目标 1 与 §4（成员建 Key 默认额度）；其余默认值规则继续有效

## 1. 背景与目标

当前普通成员（无 `CanManageAllTokens`）创建组织 Key 时，服务端强制把 Key 额度写成有限值
（`service/organization_token.go` `normalizeOrganizationMemberTokenRequest`）：

- 成员已配置个人配额（`organization_members.quota_limit > 0`）→ Key 额度 = 剩余成员配额；
- 未配置 → Key 额度 = `min(组织剩余额度, 组织总额度 / 100)`，即组织总额度的 1%，每把 Key 各自一份；
- 成员配额耗尽或组织额度不可用时，直接拒绝创建。

1% 是硬编码策略：管理员既不能选择「不限」，也不能选一个别的默认值；成员每把 Key 只能用组织额度的
1%，想继续用只能反复建 Key。本次改为可配置：

- 组织设置新增 **成员新建 Key 默认额度**（`organizations.default_member_token_quota`，0 = 不限）；
- 普通成员新建的组织 Key 取该值：0 → 无限额；N → `remain_quota = N`；
- 删除旧的「剩余成员配额 / 1%」额度推导；
- **保留创建期可用性检查**：成员配额或组织额度不足时仍然拒绝建 Key（沿用现有 403 错误码与文案），
  避免成员拿到一把注定会失败的 Key；
- 成员累计配额（`organization_members.quota_limit`）完全不变，仍是独立的跨 Key 消费闸门。

## 2. 非目标

1. 不改变成员累计配额（`organization_members.quota_limit`）的语义、接口与预扣强制；
   本设计只改「新建 Key 的初始额度」。
2. 不迁移、不回写存量 Key。设置改变只影响之后新建的 Key。
3. 不改变管理员 / Owner / 平台管理员建 Key 的表单与行为（仍可显式指定额度或无限）。
4. 不引入「组织级默认成员累计限额」（三态继承、预算公式、预扣解析）——该方案已评估并否决，
   见 §3。
5. 不改变预扣严格、结算宽松的既定口径（`docs/org-member-quota.md` §4）。
6. 不改变个人（非组织）Key 的任何行为。
7. 不改变创建期拒绝的两个错误码与前端文案（`insufficient_organization_member_quota` /
   `insufficient_organization_quota`），只调整检查条件。

## 3. 方案对比

| 方案 | 内容 | 结论 |
|---|---|---|
| X：组织默认「成员累计限额」 | 成员配额三态（继承/自定义/不限），预扣时动态解析；需要 2 列、预算公式、预扣改造、成员侧展示改造 | 否决：约束更强但复杂度过高；默认值一设会立即约束存量成员，语义重 |
| Y：组织配置「成员新建 Key 的默认额度」 | 1 列；建 Key 时快照进 `remain_quota`；0 = 不限；保留创建期可用性检查 | **选定**：直接命中需求，改动面小 |
| 维持现状 | 保留硬编码 1% | 否决：不可配置，成员体验差 |

Y 的已知取舍（写进文档与文案）：**每把 Key 的额度可以通过多建 Key 绕过**，它不是每人累计上限；
需要「每人最多花多少」时，继续使用现有成员配额（按成员配置，预扣时累计校验）。
默认值取 0（不限）是产品选择：单把 Key 可花光组织额度，由组织总额度兜底。

## 4. 数据模型

- `organizations.default_member_token_quota int not null default 0`
  （JSON: `default_member_token_quota`），单位 quota；`0` = 不限。
- 快照语义：在**创建 Key 那一刻**读取并写入 Key 的 `remain_quota` / `unlimited_quota`；
  之后修改组织设置不回溯、不影响存量 Key。
- 不新增成员表列，不改任何现有列。

## 5. 配置入口与 API

复用现有组织更新接口，不新增路由：

- `PATCH /api/organizations/:id`（`service.UpdateOrganization`）请求新增可选字段
  `default_member_token_quota`（`*int`）：
  - 缺省 = 不修改；
  - 必须是 `0 ≤ value ≤ maxOrganizationTokenQuotaValue()`，否则 400；
  - 更新列时加入 `Select` 字段列表；审计沿用 `organization.update`（before/after 已包含整个组织对象）。
- 权限沿用 `update_organization`（Owner / 组织管理员；平台管理员按其既有规则，需要 reason）。
- 前端：组织中心 → 设置页新增第二张卡「成员 Key 默认额度」，与 Profile 共用同一个表单和保存按钮：
  - 输入以 CNY 展示（`quotaUnitsToEditableAmount` / `parseQuotaFromDollars`），空或 0 = 不限；
  - 共享表单 schema 增加该字段；`transformOrganizationFormToPayload` 只在设置界面标记
    `includeDefaultMemberTokenQuota` 时发送它，平台编辑/创建抽屉不发送（避免把已有值误清零）。
- 平台管理台的编辑抽屉、创建抽屉行为不变。

## 6. 建 Key 默认值

普通成员（`isRestrictedOrganizationTokenActor`）创建 / 批量创建单个组织 Key 时：

- `RemainQuota = organization.DefaultMemberTokenQuota`
- `UnlimitedQuota = organization.DefaultMemberTokenQuota == 0`
- 其余强制默认不变：`ExpiredTime = -1`、`ModelLimitsEnabled = false`、`ModelLimits = ""`、
  `AllowIps = ""`、`Group = ""`、`CrossGroupRetry = false`、`Visibility = private`、
  `ResponsibleUserId = 操作者`；
- 删除 `organizationMemberTokenDefaultQuota` 的额度推导，替换为纯可用性检查
  `ensureOrganizationMemberTokenUsable`（在组织行锁内、读取锁定成员行之后执行），
  检查顺序与运行期预扣一致（先组织、后成员）：
  1. `organization.Quota <= 0 || organization.Quota - organization.UsedQuota <= 0` →
     `ErrOrganizationQuotaUnavailable`；
  2. `member.QuotaLimit > 0 && member.UsedQuota >= member.QuotaLimit` →
     `ErrOrganizationMemberQuotaExhausted`；
  - 两个哨兵错误与 `controller/organization.go` 的 403 映射保留；
  - 与现状的差异：现状仅在成员**未配置**个人限额时检查组织额度；新规则在配置了个人限额时
    也检查组织额度（组织额度为 0 时 Key 同样无法使用）。理由是「成员剩余」并不保证
    「组织剩余」——成员限额总和允许超额分配、平台也可能调低组织额度；该窗口里建出来的 Key
    注定无法使用。顺序与运行期一致，保证同样的情形在建 Key 和调用时返回同一个错误码。
- 检查通过后写入 `RemainQuota = organization.DefaultMemberTokenQuota`、
  `UnlimitedQuota = DefaultMemberTokenQuota == 0`；
  组织额度不足时 Key 额度不再被 1% 或剩余额度截断（配置值即上限，实际消费仍受组织余额兜底）；
- 创建不再因「成员配额耗尽 / 组织额度不可用」以外的原因被拒绝；组织已解散 / 停用等既有检查保留；
- 批量路径 `token_count = 1` 走同一归一化；成员 `token_count > 1` 仍拒绝；
- 成员编辑 Key 仍只能改名称/启停，受限字段保留库中原值（不变）；
- 管理员 / Owner / 平台管理员建 Key 行为完全不变。

运行期约束不变：有限额 Key 按现有令牌额度扣减停止；无限额 Key 由成员累计配额（若配置）与
组织总额度约束。

## 7. 前端

### 7.1 设置页

- 新增「成员 Key 默认额度」Card（标题、说明、CNY 输入、超额/合法值校验）。
- 说明文案：该值只影响**之后**由普通成员新建的组织 Key；0 = 无限额；管理员建 Key 不受影响；
  每把 Key 独立计算，不构成每人累计上限。
- 保存调用现有 `updateOrganization`，成功后刷新组织详情。

### 7.2 成员配额对话框

- 删除「成员新建 Key 使用剩余成员配额 / 未设置时用组织额度 1%」的说明（不再成立），
  保留「成员配额限制跨 Key 累计消费」的说明。

### 7.3 组织 Key 表单

- 成员的只读说明「额度、有效期和模型由组织策略自动设置」仍然成立，无需改文案；
- 创建结果、编辑摘要、列表中的额度展示沿用现有 `unlimited_quota` 路径（自动显示 Unlimited / 金额）；
- 更新相关测试的期望值（原 1% 数字改为不限或配置值）。

### 7.4 i18n

- 新增/修改英文源 key 写入 `web/src/i18n/locales/*.json`，运行 `bun run i18n:sync` 补齐。

## 8. 迁移与三库验证

- `AutoMigrate` 增加 `organizations.default_member_token_quota` 一列；无数据回填、无索引变化。
- 按 `AGENTS.md`：
  - SQLite（项目内置版本）、MySQL 5.7.44、PostgreSQL 15.19；
  - 新库建表、旧库升级（用最新发布版本建库后升级）、连续启动两次幂等；
  - 覆盖组织更新、建 Key 读取默认值；
  - 在交付说明/PR 中记录实际版本、命令与结果。

## 9. 测试计划

### 9.1 Service

1. 组织默认 0：成员建 Key → `unlimited_quota = true`、`remain_quota = 0`。
2. 组织默认 N：成员建 Key → `unlimited_quota = false`、`remain_quota = N`。
3. 其他强制默认字段不变（有效期、模型、IP、分组、可见性、责任人）。
4. 成员配额耗尽 / 组织额度不足时仍拒绝创建（现有哨兵与错误码）；
   已配置成员限额但组织额度为 0 也拒绝（新增口径）；两边都不足时返回组织错误（与运行期一致）。
5. 管理员 / Owner / 平台管理员建 Key 保持显式字段（含无限额度）。
6. 修改组织默认值不影响存量 Key；新建 Key 使用新值。
7. 组织更新：字段缺省不变、0 清除、N 设置、负数/超上限拒绝。
8. 成员配额语义回归：预扣仍按 `quota_limit` 累计强制。

### 9.2 Controller / Router

- 组织更新请求绑定新字段；删除的哨兵错误映射单测更新；无新增路由。

### 9.3 前端

- 设置页：字段初始化/换算、空=不限、保存 payload 只在设置界面包含该字段、
  平台编辑/创建抽屉不发送该字段。
- 成员配额对话框：1% 文案删除后的渲染与测试。
- 组织 Key 表单：成员创建结果/摘要显示 Unlimited 或配置值。

### 9.4 全量

- 后端：CI 等价 `go test`（排除 main 包）+ `-race`。
- 前端：typecheck、lint、相关测试。
- 三库矩阵见 §8。

## 10. 回滚

- 代码回滚即可：旧代码不读取新列，成员建 Key 恢复 1% / 剩余成员配额逻辑。
- 已创建的无限额或有限额 Key 保持原值（不回写）；旧代码可正常处理无限额 Key。
- 无数据迁移，回滚不产生数据差异。

## 11. 改动面预判

| 面 | 文件 |
|---|---|
| 模型 | `model/organization.go`（一列） |
| Service | `service/organization.go`（更新请求/校验/写入）、`service/organization_token.go`（建 Key 默认值、删除推导与哨兵） |
| Controller | `controller/organization.go`（请求字段；保留两个错误映射分支） |
| 前端 | `organization-settings-section.tsx`（新 Card）、`lib/organization-form.ts`（schema/defaults/payload 标记）、`api.ts`（payload 类型）、`types.ts`（zod 字段）、`organization-member-quota-dialog.tsx`（删除 1% 文案）、i18n、相关测试 |
| 文档 | 本文档；在 `2026-10-19-org-member-key-defaults-design.md` 标注 §4 与 §2.1 被取代 |

## 12. 验收标准

1. 组织设置为 0（默认）：普通成员新建组织 Key 为无限额；设置为 N：Key 额度 = N。
2. API 伪造 `unlimited_quota` / `remain_quota` 对普通成员不生效，仍按组织设置归一化。
3. 成员配额耗尽或组织额度不足时拒绝建 Key（403 + 现有错误码）；已配置成员限额但组织额度为 0
   也拒绝；检查在创建接口的服务层、组织行锁内完成。
4. 管理员 / Owner / 平台管理员建 Key、成员累计配额、预扣/结算口径均不变。
5. 存量 Key 不受影响；设置修改只影响之后新建的 Key。
6. 设置页字段可读写，平台编辑/创建抽屉不会误清零该字段。
7. 三库迁移与测试矩阵、Go 全量 + `-race`、前端 typecheck/lint/测试全部通过。
