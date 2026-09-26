# 组织成员随账号删除收口设计

**Date:** 2026-09-24
**Status:** Draft（待评审）
**范围:** 后端 `model/` `service/` `controller/` `i18n/`，平台面前端 `web/`
**相关:** [组织自动加入设计](2026-09-23-organization-auto-join-design.md)、[组织管理移植设计](2026-09-20-organization-management-port-design.md)、[ORGANIZATION_TECHNICAL_DESIGN.md](../../ORGANIZATION_TECHNICAL_DESIGN.md)

## 1. 问题

用户被**硬删除**后，其在 `organization_members` 里的行（无论 `active` 还是 `removed`/`exited` 墓碑）会留在库中，而 `users.id` 在 SQLite 上会被复用（建表语句为 `id integer PRIMARY KEY`，无 `AUTOINCREMENT`，删掉最大行后新插入拿回同一 id）。下一个拿到该 id 的新账号会与这条陈旧成员行"对接"，产生两类后果：

- 成员行是 `active` → 新账号**继承**该组织的成员身份（见 §5.4 D1）
- 成员行是墓碑 → 注册自动加入被静默跳过（见 §5.4 D2，本机实测症状）

根因是**成员关系以可复用的 `user_id` 作为唯一身份凭据，且账号销毁时没有收口**。本设计把"账号存在"确立为成员关系的前提，并在账号销毁路径上收费。

## 2. 非目标（明确不做）

1. **不改表结构**（无 DDL，见 §6）：不加表、不加列、不加索引、不改类型、不动 AutoMigrate。
2. **不采用"人级"墓碑**：不把墓碑 key 到归一化邮箱。邮箱在系统内可变可解绑（`User.ClearBinding("email")`），域名规则本就放行同域任意邮箱（见 [组织自动加入设计](2026-09-23-organization-auto-join-design.md) §3 的公共邮箱服务商提示），因此它买不到真屏障，却要引入无法回填历史的迁移（已删账号的邮箱无从恢复）。
3. **不改软删路径的语义**（`DeleteSelf` / `ManageUser(action=delete)`）：软删不产生 id 复用与邮箱释放，不可能触发 D1/D2（证明见 §5.3），保持现状。
4. **不追溯收口以外的组织数据**：审计、配额调整、账单记录等历史一行不动（§5.4 三分类）。
5. **不自动转移组织所有权、不自动移交 key**：删除时只做"拒绝并说明"，不做静默处置（§7.2）。

## 3. 关键决策

| 决策 | 选择 | 理由 |
|---|---|---|
| 身份模型 | **账号级**：成员关系以"账号存在于 `users`"为前提 | 现状的"账号级"行为在 MySQL/PG 上偶然成立（id 不复用）、在 SQLite 上偶然失效；本设计把它收敛为明确规则，而非引入新语义 |
| 收口范围 | 该用户在**每个未解散组织**里的成员行，**所有状态**（active/disabled/removed/exited） | active 行产生继承，墓碑产生误判，disabled 行会参与政策判定；账号既已不存在，任何成员状态都不成立 |
| 未解散组织 + 用户是该组织 `owner_user_id` | **拒绝删除** | 组织不变式是"恰好一个 active owner"（`service/organization_member.go:266`、`:310`），而 owner 本身不可被成员操作移除（`ensureOrganizationMemberTargetAllowed`，`service/organization_member.go:614`）。静默转移所有权是发明政策 |
| 未解散组织 + 用户持有该组织 key | **拒绝删除**，点名 blocker | 该用户是组织 token 的 `user_id` 或 `responsible_user_id`（`service/organization_token.go:525`）。现流程对这些 key 有专门的移交语义（`transferOrganizationKeysWithTx`，`service/organization_member.go:841`）：public 可见的转移、非 public 的删除并逐条审计。删除用户不能绕过它 |
| 已解散（`dissolved`）组织 | 直接收口，不拒绝 | 解散组织的成员在 `evaluateWorkspaceOrganizationPolicy` 下什么都拿不到，key 也已无实际效力；否则"曾拥有过已解散组织"会让人永远无法删除 |
| 收口落点 | `service` 层新增入口，事务内先收口再调 `model` 的 `HardDeleteWithTx(tx)`（§8） | 收口要写组织审计（`service.recordOrganizationAudit`），而 `model` 不能反向依赖 `service`；放在 controller 会漏掉第二个硬删入口 |
| 拒绝时的错误与留痕 | 复用 `OrganizationOperationBlockedError` + `recordOrganizationMemberKeyTransferBlockedAudit` | 现成机制已能表达"操作被 blocker 拦下"，并在组织审计里留痕（`service/organization_audit.go:145`，仅对 `*OrganizationOperationBlockedError` 生效） |
| 陈旧行兜底 | 只做**可证明**的判定（账号行不存在）用于读取与修复；`created_at` 比较**只记日志** | 多节点下墙钟可能失真（§12），据此拒绝会误伤合法成员 |
| 存量修复 | 启动期一次性清理"账号不存在的成员行"，分批 + 幂等 | 可证明安全：账号不存在 → 成员关系不可能成立。修复前本机已手工执行同类清理 |

## 4. 不变式

- **I1**：成员行存在的必要条件是其 `user_id` 在 `users` 里存在（硬删收口与 orphan 判定共同保证）。
- **I2**：账号销毁不得改变任何组织的 owner 唯一性，也不得绕过组织 key 的移交语义。做不到时**删除失败**，而不是静默处置。
- **I3**：每一行被收口的成员行都必须伴随一条组织审计（action `organization.member.account_deleted`），组织台账不得出现无出处的成员消失。
- **I4**：收口与用户硬删在**同一事务**内完成：要么账号还在且成员行还在，要么两者都不在。不得出现"账号已删、成员行还在"的中间态。
- **I5**：收口不得触碰审计、配额调整、账单等历史表（只读历史，允许悬空 user_id）。
- **I6**：`created_at` 比较类判定在任何路径上都不作为访问控制或数据删除的依据（可误报，见 §12）。

## 5. 现状：删除流程与缺陷

### 5.1 四个用户删除入口

| 入口 | 路由 | 实现 | 类型 |
|---|---|---|---|
| `controller/user.go:1071` `DeleteUser` | `DELETE /api/user/:id/` | `model.HardDeleteUserById` | **硬删** |
| `controller/user.go:1215` `ManageUser` `case "delete"`（`:1246`） | `POST /api/user/manage` | `user.Delete()` | 软删 |
| `controller/user.go:1102` `DeleteSelf` | `DELETE /api/user/self` | `model.DeleteUserById` → `Delete()` | 软删 |
| `controller/aggregated_api.go:834` `AggregatedDeleteUser` | `POST /api/v2/users/:user_id/delete` | `model.HardDeleteUserById` | **硬删** |

两个硬删入口都必须经过本设计的收口（这是收口放在 `service` 而非 controller 的原因）。

### 5.2 硬删清理清单

`model/user.go:1011` `HardDelete()` → `deleteUserAuthenticationData`（`:1048`）。事务内：`auth_version` 自增并发布 tombstone → `external_identity_claims` 释放 → `two_fa_backup_codes` / `two_fas` / `user_sessions` / `auth_flows` / `passkey_credentials` → **`tokens`（按 `user_id` 全删，含组织 token）** → `user_oauth_bindings` → users 行本身。事务外：失效 token/user 缓存。

**未清理**：`organization_members`（本次缺陷）、`user_account_contexts`、`organization_token_system_blockers` 的关联行、以及全部历史表。

### 5.3 软删为什么不在范围内（可证明）

软删只置 `users.deleted_at`（`model/user.go:987`）并吊销会话。三个后果使它不可能触发 D1/D2：

1. users 行**仍占据该 id** → SQLite 下一个 rowid 是 `max+1`，不会复用；
2. `CheckUserExistOrDeleted`（`model/user.go:295`）用 `Unscoped()` 查询用户名与邮箱 → 该用户名/邮箱**永久不可再注册**，不会出现"同邮箱新账号"；
3. 全仓库没有把 `deleted_at` 置回的恢复路径 → 该账号不会复活。

因此软删路径下的残留成员行只造成"成员列表里有一位无法登录的成员"以及计数偏差，属于独立议题（§16 风险表），不在本次范围。

### 5.4 缺陷清单（按严重度）

| # | 缺陷 | 机制 | 后果 |
|---|---|---|---|
| D1 | **权限继承** | 用户被硬删时未先从组织移除 → 成员行仍 `active`；新账号复用 id 后，`availableAccountContexts`（`service/account_context.go:133-136`）与 `middleware/account_context.go:97-104` 仅按 `user_id + status=active` 判定 | 新账号可切换到该组织上下文，使用组织额度与 API Key |
| D2 | **自动加入被静默跳过** | 成员行是 `removed`/`exited` 墓碑；新账号复用 id 后命中 `JoinOrganizationByJoinRuleWithTx` 的"已存在则跳过"分支（`service/organization_join_rule.go:306-311`） | 已实测：日志 `user 6 already belongs to organization 2, join rule 1 skipped` |
| D3 | **组织 owner 悬空** | 硬删路径对 `owner_user_id` **零引用**（`controller/user.go`、`model/user.go`、`controller/aggregated_api.go` 均无组织检查）；owner 保护只存在于成员操作路径 | 组织残留一个指向已删账号的 owner；`organizations.owner_user_id` 悬空 |
| D4 | **组织 key 被静默删除** | 组织 token 的 `user_id = responsibleUserId`（`service/organization_token.go:525`），`HardDelete` 按 `user_id` 全删 token | 组织 key 在无组织审计、无移交的情况下消失；仅担任 `responsible_user_id` 的 key 反而不被删，但责任人指向空号 |
| D5 | **审计/日志重新归属** | 历史表的 `user_id` 指向被复用的 id | 组织审计、消费日志的历史被归属到另一人 |
| D6 | **成员计数含无主行** | `ActiveMemberCount` 只统计 `organization_members`，不 join `users`（`service/organization_management.go:179`） | 组织成员数与实际可用成员不符 |

数据三分类（本设计只处理第一类）：

| 类别 | 表 | 处理 |
|---|---|---|
| 活状态 | `organization_members` | 随账号收口（§7） |
| 历史（自带快照，只读） | `organization_audit_logs`、`organization_quota_adjustments`、`organization_billing_records`、`logs` | 一行不动 |
| 悬空引用 | `organizations.owner_user_id`、`tokens.responsible_user_id`、`user_account_contexts` | 拒绝式保护（§7.2）/ 随账号删除 |

### 5.5 组织侧的对照流程

`RemoveOrganizationMember`（`service/organization_member.go:466`）与 `ExitOrganization`（`:539`）的完整语义：幂等键 → 锁组织 → 锁成员行 → 权限与 owner 保护 → **`transferOrganizationKeysWithTx`（公钥转移 + 非 public token 删除，逐条审计）** → 置 `removed`/`exited` + 时间戳（**墓碑，行不删**）→ 组织审计 → key 缓存失效。

本设计不改变上述语义：墓碑对**在世账号**仍然阻止自动加入（现注释的意图原样保留），改变的是"账号不存在时它不再算数"。

## 6. 表结构影响：无 DDL

新增行为全部落在现成表上，语句性质为 DML：

| 动作 | 表 | 语句 |
|---|---|---|
| 收口成员行 | `organization_members` | `DELETE` |
| 补组织审计 | `organization_audit_logs` | `INSERT` |
| 清理成员相关 blocker | `organization_token_system_blockers`（按 `ref_type='member' AND ref_id=userId AND status='active'`）| `UPDATE`（置 cleared）|
| 清理被删 token 的 blocker | `organization_token_system_blockers`（按 `token_id`）| `UPDATE` |
| 删除账号上下文 | `user_account_contexts` | `DELETE` |
| 存量孤儿修复 | `organization_members` | `DELETE`（分批）|

**注意**："无 DDL"不等于"无需三库验证"：本次改动落在**事务、行锁、并发删除**上，按 AGENTS.md 仍必须完成 SQLite / MySQL / PostgreSQL 三库实机验证（§14.3）。

## 7. 收口规则

### 7.1 判定与动作

对目标用户所在的**每一个组织**（含其成员行的全部状态）：

| 条件 | 动作 |
|---|---|
| 组织 `status = dissolved` | **收口**：删除成员行 + 写审计 |
| 未解散，且 `organizations.owner_user_id = 目标用户` | **拒绝**：`OrganizationOperationBlockedError`，点名组织，提示"先转让所有权（`TransferOrganizationOwner`）或先解散组织（`DissolveOrganization`）" |
| 未解散，且该用户持有该组织 key（`scope_type='organization' AND organization_id=? AND (user_id=? OR responsible_user_id=?)`，visibility 不限） | **拒绝**：`OrganizationOperationBlockedError`，点名 blocker，提示"先用成员移除/禁用流程带 `transfer_to_user_id` 移交 key" |
| 其余（普通成员，无 key，含 removed/exited/disabled 墓碑） | **收口** |

收口动作的完整内容（同一事务内）：

1. `DELETE` 成员行
2. 写审计 `organization.member.account_deleted`（§10）
3. 将该成员相关的 active blocker 行置为 cleared（`ref_type='member'`、`ref_id=userId`）

被删除账号名下的 token 由 `HardDelete` 现有逻辑删除；其 blocker 行按 `token_id` 一并置 cleared，避免留下永不解除的 blocker。

拒绝是**整请求失败**（不部分收口）：一个用户属于多个组织时，任一组织拒绝则整个删除失败，管理员按提示逐个处理。I4 要求不存在中间态。

### 7.2 为什么是拒绝而不是自动处置

- **owner**：自动转移等于替管理员挑选新 owner；唯一 active owner 是组织不变式，`TransferOrganizationOwner` 连平台 root 都要走一个显式的"活跃成员目标"分支（`service/organization_member.go:238-244`）。删除用户不是推翻它的地方。
- **key**：`transferOrganizationKeysWithTx` 对 public key 是转移、对非 public 是**删除**。删除凭证必须是显式带 `reason` 的组织操作，才会进入组织审计；由"删用户"顺手完成等于把一次凭证销毁藏在一个无关操作里。
- 管理员始终有出路：转让所有权 / 带 `transfer_to_user_id` 移除成员 / 解散组织 / 先删 key。拒绝消息必须把这四条路径写清楚。

## 8. 落点与架构

`model` 不能导入 `service`（现有依赖方向是 `service → model`），而收口要写组织审计（`service.recordOrganizationAudit`）。因此：

1. **`model/user.go`**：把 `HardDelete()` 拆为 `HardDeleteWithTx(tx *gorm.DB) error` + 现有包装（仓库既有先例：`InsertWithTx`、`UpdateWithTx`、`EditWithTx`、`GetUserGroupByIdTx`）。包外行为不变。
2. **`service/`**（新文件，如 `service/user_account_deletion.go`）：新增单入口
   `DeleteUserAccount(operatorUserId, targetUserId int, auditMetadata ...OrganizationAuditRequestMetadata) error`
   在同一事务内：锁并判定该用户的全部组织成员行（§7）→ 有任何拒绝即返回错误、事务回滚 → 收口 → 调 `model.HardDeleteWithTx(tx, userId)` → 提交后失效缓存（`invalidateTokensCache` / `invalidateUserCache` / `invalidateOrganizationTokenCaches`）。
3. **`controller/user.go:1087`** 与 **`controller/aggregated_api.go:857`**：改调该 service 入口，删除原本直接调 `model.HardDeleteUserById` 的语句。两处都要保留现有的角色校验与 `recordManageAuditFor`。
4. 拒绝错误经现有错误映射返回稳定 code（`types.ErrorCode*`），前端可本地化。

## 9. 锁顺序与并发

- 按组织 id **升序**加锁，避免与并发组织操作形成反向锁序。
- 组织子系统既有顺序是"先锁组织行、再锁成员行"（`lockedOrganizationManagementDecisionWithTx` → `model.LockForUpdate`）。收口沿用同一顺序；对每个组织先锁组织行再锁成员行，而不是一次性锁全部成员行。
- 与注册路径（`InsertWithTx` 后插成员行）存在 user→member 与 member→user 的相反顺序，理论上有死锁面：实现时需在事务开头先对 users 行加锁（`lockForUpdate`）以固定顺序，并在三库测试中覆盖并发用例（§14.3）。此点必须在实现时实测确认，不能只靠推理。

## 10. 审计

沿用 `recordOrganizationAudit`（`service/organization_audit.go:79`）。新增 action 常量：

| action | target | 内容 |
|---|---|---|
| `organization.member.account_deleted` | `member` | 成员行因账号被删除而收口。`operator_user_id` = 触发删除的管理员（有则填），`operatorRole` = `organizationAuditOperatorRoleSystem`，`reason` = `account_deleted:<targetUserId>`；before 快照 = 被删成员行，保底让组织成员能查到"这个人为什么从组织里消失了" |

拒绝路径复用 `recordOrganizationMemberKeyTransferBlockedAudit`（`service/organization_audit.go:145`），自动带上 `blockers`。

同时 `common.SysLog` 一条收口汇总（组织数、成员行数），便于平台侧排查（收口是跨组织的平台级动作）。

## 11. 存量修复（启动期一次性）

按仓库既有启动期修复的先例（`model/organization_member_migration.go`）新增一个幂等修复函数：

- **删除**：`organization_members` 中 `user_id` 在 `users` 里**不存在**的行（可证明安全：账号不存在 → 成员关系不可能成立）。分批处理（每批 500，循环至空），禁止 `DELETE ... WHERE user_id NOT IN (SELECT id FROM users)` 一把梭——MySQL 上大表删除会长时间持锁。
- **只报告不删除**：`created_at` 早于对应账号 `created_at` 的行（可能为 id 复用残留，也可能为跨节点时钟偏差，见 §12）。记 `common.SysError` 并输出计数，供运维人工核查。
- **幂等**：第二次启动必须处理 0 行（三库各跑两次验证）。
- 本机已手工执行同类清理（org 2 一条错配行、org 1 一条孤儿行），修复函数上线后应在同类库上得到相同结果。

## 12. 陈旧行判定：哪些结论可信

| 判据 | 可信度 | 用途 |
|---|---|---|
| 账号行不存在（orphan） | **可证明**：账号不存在则成员关系必不成立 | 可用于读取路径判定、启动期删除、运维修复 |
| 成员行 `created_at` < 账号 `created_at` | **不可证明**：`common.GetTimestamp()` 是各节点自己的墙钟（`service/organization_member.go:142`），管理员在 B 节点为 A 节点创建的用户加成员时，两个时间戳来自不同时钟。生产为多节点（3 节点 + 共享 PG/Redis） | **只记日志**，不得作为拒绝访问或自动删除的依据（I6） |

> 更正记录：讨论中曾判断 `created_at` 比较"零误报"（依据是注册与自动加入同事务同节点）。该结论对注册路径成立，但对管理员加成员等跨节点路径不成立，故本设计将其降级为观测信号。

对 pre-fix 库中已存在的"active 幽灵行 + id 复用"（读取路径无法用 orphan 判据识别）：由启动期修复的人工核查项 + 运行时日志（命中 `created_at` 判据时记录）暴露，人工清理。本机这一次即以人工方式清理。

## 13. 前端与 i18n

- 用户删除确认框在收到拒绝时**原样展示 blocker 文案**（点名组织与处理建议），不得退化为通用"删除失败"。
- 拒绝文案走 `web/src/lib/server-error-message.ts` 的 code→文案映射（与 join rule 拒绝同一形态），同步 7 个语言包。
- 后端新增错误信息走 `i18n/` 的 en/zh key，用 `common.ApiErrorI18n`。
- 组织成员列表对"账号已不存在"的行（`user_status` 为空）给出明确标注，避免 D6 类困惑；此项为可选增强，与收口解耦。

## 14. 测试与三库验证

### 14.1 服务/控制器测试（`require` + `assert`，确定性表驱动）

- 普通成员（无 key）被硬删 → 成员行消失、审计 `organization.member.account_deleted` 存在、账号行消失（I4）
- 墓碑（`removed`/`exited`）成员被硬删 → 成员行消失；随后同邮箱重注册**正常自动加入**（D2 回归）
- 成员行 `active` 被硬删 → 不残留；复用 id 的新账号**拿不到**该组织上下文（D1 回归，可用"删号→重建同 id 账号"逼近）
- 目标是未解散组织 owner → 删除失败、成员行与 owner 未变、错误点名组织（I2）
- 目标持有组织 key（public / 非 public 各一）→ 删除失败、token 未被删、key 转移 blocker 审计存在（D4 回归）
- 组织已解散 + 目标是 owner → 删除成功、成员行收口（§7.1 例外分支）
- 多组织：一个拒绝则整请求失败，任何组织都不产生收口（I4）
- 软删路径行为不变（成员行留存）
- 收口后 `ActiveMemberCount` 不再包含已删成员（D6）

### 14.2 陈旧行断言

- orphan 判据：账号不存在时读取路径视为非成员（零误报）
- `created_at` 判据只写日志、不影响结果（I6）——用"成员行时间戳被人为置早"的夹具断言访问仍正常

### 14.3 数据库（AGENTS.md 强制）

改动落在事务与行锁上，必须三库实机验证：

- SQLite / MySQL / PostgreSQL 各跑：新建库、从最新发布版升级、启动两次证明修复函数幂等
- 并发用例：删除用户与成员移除/注册并发，验证无死锁、无中间态（I4）、锁顺序符合 §9
- 记录确切数据库版本、命令与结果到 PR；任一库无法验证须显式报告，不得声称"已兼容"

## 15. 交付物

| 层 | 文件 |
|---|---|
| 模型 | `model/user.go`（`HardDelete` 拆出 `HardDeleteWithTx`）、`model/organization_member_migration.go` 或同级新文件（启动期修复）|
| 服务 | 新文件（如 `service/user_account_deletion.go`：判定、收口、审计、缓存失效）|
| 控制器 | `controller/user.go:1087`、`controller/aggregated_api.go:857` 改调 service 入口 |
| 审计 | `service/organization_audit.go`（新增 action 常量）|
| i18n | `i18n/`（en/zh 新增 key）|
| 前端 | `web/src/lib/server-error-message.ts`、`web/src/i18n/locales/*.json`（拒绝文案）|
| 测试 | 服务/控制器回归测试（§14.1、§14.2）|

## 16. 风险

| 风险 | 缓解 |
|---|---|
| 拒绝删除改变现有运维习惯（今天删谁都成功） | 拒绝文案列出四条出路（转让/移交/解散/先删 key）；PR 说明这是有意的 fail-closed 变更 |
| 收口与删除用户产生新的锁序，与组织操作/注册并发时死锁 | §9 固定锁序 + §14.3 并发用例实测；必要时先锁 users 行 |
| 存量 active 幽灵行无法用 orphan 判据识别 | 启动期修复的人工核查项 + 运行时日志；生产 MySQL/PG 不复用 id，实际发生面极小 |
| 软删成员的残留（成员列表可见、计数偏高、软删 owner 需 root 修复） | 记录为独立议题，不在本次范围；如需处理，应单独设计"软删是否等同账号销毁" |
| 收口把"组织移除过这个人"的记忆一并抹掉 | 这是 §3 决策的已知代价：账号级语义下，删除账号后同邮箱重注册会重新自动加入。需要更强阻拦时应使用移除 join rule / 邮箱黑名单（独立功能），不能依赖成员行墓碑 |
