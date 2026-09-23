# 组织自动加入（注册邮箱白名单）设计

**Date:** 2026-09-23
**Status:** Draft（待评审）
**范围:** 后端 `model/` `service/` `controller/` `router/` `i18n/`，平台面前端 `web/`
**相关:** [组织管理移植设计](2026-09-20-organization-management-port-design.md)、[ORGANIZATION_TECHNICAL_DESIGN.md](../../ORGANIZATION_TECHNICAL_DESIGN.md)

## 1. 目标

管理员（root）为组织配置一组邮箱匹配规则；用户用邮箱注册并通过邮箱验证后，服务端按**已验证的邮箱**匹配规则，命中即把该用户以 `member` 身份自动加入对应组织。

规则支持两类：

- **域名规则**：`enterprise.com`（仅裸域名）或 `*.enterprise.com`（裸域名 + 所有子域）
- **地址规则**：`user-a@enterprise.com`（严格全等，用于具体人员）

注册接口**不新增任何客户端参数**：组织完全由服务端按邮箱推导，客户端无法指定或影响归属。

## 2. 非目标（明确不做）

1. **不追溯**：新增规则不会把已注册用户补进组织。
2. **不在 OAuth、邮箱绑定/换绑、管理员直加路径触发**，只在密码注册且邮箱验证通过时触发（理由见 §8.3）。
3. **不支持 `member` 以外的角色**，自动加入者一律 `member`。
4. **不做负向规则**（"禁止加入"），需要排除某人时不给它任何匹配规则即可。
5. **不改平台级 `common.EmailDomainWhitelist` 的语义**（`controller/misc.go:236` 的精确域名匹配保持原样）。平台级白名单是注册闸门，先于本功能生效；组织规则只在注册已成立的前提下决定归属。
6. 不支持正则、裸 `*`、CIDR、IP 字面量。

## 3. 关键决策

| 决策 | 选择 | 理由 |
|---|---|---|
| 规则存储 | 独立表 `organization_join_rules`，`pattern_normalized` 全局唯一索引 | 跨组织互斥必须由数据库保证，不能靠应用层自觉；也避免把权限数据放进 option 存储（test 站曾出现 `PUT /api/option/` 返回 success 但不落库） |
| 域名规则之间 | **配置时互斥**：相同域名、互为父子域一律拒绝 | 运行时只有一个候选，无需仲裁；"A 配了 `*.enterprise.com` 后再给 B 配 `mail.enterprise.com`"必须被拦住，否则互斥被包含关系绕过 |
| 地址规则 vs 域名规则 | **地址规则优先** | 支持"整个公司域进 A、几个具体人员进 B"这类真实例外；全等匹配是管理员显式意图，不可能是误配 |
| 同一组织多条规则 | 允许 | 例如 `*.enterprise.com` + `partner.com` |
| 未开启邮箱验证 | **拒绝创建规则** | 避免配好一堆规则却静默失效 |
| 配置方式 | 批量：一次请求多行，类型自动判别 | 配几十个具体人员时不用逐条点 |
| 冲突时的配置校验 | 域名 vs 域名 → 拒绝；地址 vs 域名 → 仅提示不拦截 | 见上两行 |
| 加入失败 | DB 错误 → 注册整体回滚；组织非 active → 跳过加入但注册成功 | 见 §8.4 |
| 组织行锁 | 不加锁，只读状态 | 见 §8.5 |

## 4. 不变式

- **I1**：对任意邮箱，运行时最多命中一条**生效**规则。由 `pattern_normalized` 唯一索引（地址规则）+ 域名规则配置时互斥共同保证。
- **I2**：只有经 `common.VerifyCodeWithKey(email, code, common.EmailVerificationPurpose)` 验证通过的邮箱才可能触发加入。任何未经邮箱验证的创建路径都不得触发。
- **I3**：自动加入只发生在注册事务内，角色恒为 `member`，且必然伴随一条组织审计记录。
- **I4**：任何写入规则表的路径都必须先过冲突检查；唯一索引是并发下的最后兜底，不是唯一防线。

## 5. 数据模型

新文件 `model/organization_join_rule.go`（新文件需带 AGPL 头，两行版权 `2023-2026 QuantumNous` + `2026 CubeRouter`）。

```go
const (
    OrganizationJoinRuleMatchTypeDomain = "domain"
    OrganizationJoinRuleMatchTypeEmail  = "email"
)

type OrganizationJoinRule struct {
    Id                int    `json:"id"`
    OrganizationId    int    `json:"organization_id" gorm:"not null;index"`
    MatchType         string `json:"match_type" gorm:"type:varchar(16);not null"`
    Pattern           string `json:"pattern" gorm:"type:varchar(255);not null"`            // 管理员原样输入，仅展示
    PatternNormalized string `json:"pattern_normalized" gorm:"type:varchar(191);not null;uniqueIndex"` // 归一化后，唯一
    CreatedBy         int    `json:"created_by" gorm:"not null"`
    CreatedAt         int64  `json:"created_at" gorm:"bigint;index"`
    UpdatedAt         int64  `json:"updated_at" gorm:"bigint;index"`
}
```

- 表名 `organization_join_rules`；在 `model/main.go` 的 AutoMigrate 列表里与 `Organization`、`OrganizationMember` 同处注册。
- `pattern_normalized` 用 `varchar(191)`：utf8mb4 下 191×4 = 764 ≤ 767 字节，兼容 MySQL 5.7.8 的 COMPACT/REDUNDANT 行格式索引上限；与仓库现有 `Organization.NameNormalized varchar(128) uniqueIndex` 的做法一致。校验阶段同样限制归一化后长度 ≤ 191。
- 无 `DeletedAt`：删除是物理删除，靠组织审计留痕。

## 6. 归一化与校验

`NormalizeJoinRulePattern(matchType, raw string) (normalized string, err error)`：

- **domain**：`strings.TrimSpace` → 小写 → 去掉一个结尾 `.` → 保留开头的 `*.`（`*.` 是语义的一部分：`*.enterprise.com` 与 `enterprise.com` 是两条不同规则）。
  - 去掉 `*.` 后必须匹配 `^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z0-9]([a-z0-9-]*[a-z0-9])?$`（至少一个点、无空标签、无首尾连字符）。
  - 拒绝：含 `@`、`*` 出现在非开头位置、含其他非 ASCII 字符（国际化域名要求写 punycode，例如 `xn--fiq228c.com`）、超长。
- **email**：`model.NormalizeEmail(raw)`（小写 + trim），并过 `common.Validate.Var(pattern, "required,email")`（与 `controller/misc.go:222` 同一套校验）。
- 两者归一化后长度均须 ≤ 191。
- `matchType` 由调用方判定：输入含 `@` 为 email 规则，否则为 domain 规则（管理 API 侧），或显式传入。

匹配用 base（`strings.TrimPrefix(normalized, "*.")`）与归一化后的完整 pattern 两个值；base 不单独存储，由 pattern 推导。

## 7. 匹配与冲突判定

**同一个函数同时服务运行时匹配和配置时冲突检查，禁止两处各写一套语义。**

### 7.1 单条规则命中 `MatchJoinRule(rule, normalizedEmail) bool`

令 `d` = 邮箱 `@` 之后的部分：

- `email` 规则：`normalizedEmail == rule.PatternNormalized`
- `domain` 规则，`base = TrimPrefix(pattern_normalized, "*.")`：
  - 裸域名规则：`d == base`
  - 通配规则：`d == base || strings.HasSuffix(d, "."+base)`

必须用 `HasSuffix(d, "."+base)`，**不能**用 `HasSuffix(d, base)` —— 后者会让 `evil-enterprise.com` 命中 `enterprise.com`。也不使用正则。

### 7.2 两条规则冲突 `JoinRulesOverlap(a, b) bool`

- email vs email：归一化后相等。
- domain vs domain：`base_a == base_b || HasSuffix(base_a, "."+base_b) || HasSuffix(base_b, "."+base_a)`（同域或互为父子域；已由 §3 的互斥决策一并覆盖）。
- email vs domain：该 email 的域是否被 domain 规则命中（即对 `d` 走 §7.1 的 domain 判定）。

调用点不同，处理不同：

- 新增 **domain** 规则时，与任何已存在的 domain 规则冲突 → **拒绝**，返回冲突对象（组织名 + pattern）；若它覆盖了**其他组织**已有的 email 规则 → **允许**，返回提示（地址规则更具体，仍会优先进原组织）。
- 新增 **email** 规则时，若被**其他组织**的 domain 规则覆盖 → **允许**，返回提示「该地址当前会被组织 A 的 `*.enterprise.com` 命中，新增后将优先进入组织 B」。

提示（notice）只在本组织与**其他组织**的规则相互覆盖时产生；同一组织内部的覆盖不提示（例如本组织已配 `*.enterprise.com` 再补一条冗余的 `mail.enterprise.com` 地址规则）。

### 7.3 运行时解析 `ResolveOrganizationJoinRuleWithTx(tx, normalizedEmail)`

不做全表扫描：

1. 先查 email 规则：`WHERE match_type='email' AND pattern_normalized = ?`，命中即返回（地址规则优先）。
2. 再查 domain 规则：把 `d` 的所有后缀（`enterprise.com`、`b.enterprise.com`、`a.b.enterprise.com`…，含 `d` 自身）拼成 `IN (...)` 列表，对 `pattern_normalized` 唯一索引做等值查询；结果里再用 §7.1 的 bare/wildcard 语义确认。
3. 若 domain 侧命中多于一条（配置检查被绕过或历史脏数据）：**放弃加入**，记 `common.SysError` 并带上所有命中规则 id，不猜。I1 被破坏时失败方向必须是"不加入"。

## 8. 注册流程

### 8.1 触发位置

`controller/user.go` 的 `Register`，**仅在 `common.EmailVerificationEnabled` 分支内、`VerifyCodeWithKey` 与 `EnsureEmailAvailable` 通过之后**取 `user.Email` 作为唯一依据。

### 8.2 时序

沿用现有注册事务（`controller/user.go:311-322` 的 `model.DB.Transaction`）：在 `cleanUser.InsertWithTx(tx, inviterId)` 之后（此时 `cleanUser.Id` 已生成），调用

```go
service.JoinOrganizationByJoinRuleWithTx(tx, &cleanUser, verifiedEmail)
```

该函数：解析规则（§7.3）→ 无命中则返回 nil → 读组织状态 → 非 `active` 则跳过 → 在**同一事务**内插入成员行 + 写审计（§10）。

成员行字段照 `service.AddOrganizationMember` 的写法（`Role = member`、`Status = active`、`CreatedAt`/`UpdatedAt = now`），唯一差别是 `InvitedBy = 0`。新用户不可能已存在成员行（`EnsureEmailAvailable` 走 `Unscoped`，已删除用户的邮箱也算占用，同邮箱无法二次注册），因此不需要 `ReactivateOrganizationMember`，直接 `Create`。

### 8.3 为什么必须锁死在注册分支内

`controller/oauth.go:353-357` 的 OAuth 创建路径把 provider 返回的邮箱直接写入 `user.Email`，只查唯一性、**不验证邮箱归属**。一旦本功能出现在 `FinishInsert`/`FinalizeOAuthUserCreation` 这类公共创建路径上，或日后被顺手加到 OAuth，攻击者用 GitHub/OIDC 填 `ceo@enterprise.com` 即可免邮箱控制进入组织。实现时须在该调用点写明这条原因。

### 8.4 失败语义

| 情况 | 行为 |
|---|---|
| 无规则命中 | 静默通过，与现状一致 |
| 命中，但组织 `disabled`/`dissolved` | **跳过加入，注册照常成功**，记 `common.SysError`（含组织 id、规则 id、邮箱域）。不能让一条失效规则把整个域的注册全部卡死 |
| 加入写库报错 | **整个注册事务回滚**，按现有 `common.ApiError(c, err)` 返回。验证码在注册路径上未被消费（用的是 `VerifyCodeWithKey` 而非 Claim），用户可在 10 分钟内用同一验证码重试 |

### 8.5 不加组织行锁

只做无锁读。竞态后果是"加入了一个几毫秒后被解散的组织"，而解散组织的成员在 `evaluateWorkspaceOrganizationPolicy` 下本来什么都拿不到，不值得为此在注册热路径上加锁，也避开与 `AddOrganizationMember`（先锁组织行再写成员）之间的锁序问题。

### 8.6 响应不泄露

注册响应保持 `{"success": true}`，不带任何组织信息。未认证调用者不应能通过注册接口探测组织是否存在、白名单是否命中。用户登录后在账户上下文里看到自己已属于某组织。

## 9. 管理 API（root 专属）

挂在现有平台面分组下（`router/organization-router.go` 的 `adminOrganizationRoute`，父组 `middleware.AdminAuth()` 保留），子组再套 `middleware.RootAuth()`（`middleware/auth.go:112`，仅 `common.RoleRootUser` 通过）：

```
GET    /api/admin/organizations/:id/join-rules
POST   /api/admin/organizations/:id/join-rules    {patterns: [...], reason}
DELETE /api/admin/organizations/:id/join-rules/:ruleId?reason=
```

- **POST 批量语义**：`patterns` 为字符串数组，支持多行粘贴。服务端逐行 trim、丢弃空行、批内去重、按含 `@` 与否判别类型、逐行校验（§6）→ 与现有规则做冲突检查（§7.2）→ **整批在同一事务内插入**。
  - 任何一行硬失败（格式非法、domain 规则的互斥冲突、pattern 已被任一组织占用）→ **整批拒绝**，逐行返回原因，不产生半批脏数据。错误体形如：
    `data: { line_errors: [{ line, pattern, reason }], notices: [{ pattern, organization_name, conflicting_pattern }] }`
  - **批内重复静默去重**（同一次粘贴里的重复行不是错误）；`pattern_normalized` 已被**其他**组织占用才是硬失败——全局唯一索引是全局的，必须让管理员看到占用的组织名。
  - `notices` 用于"地址规则被其他组织域名规则覆盖"这类提示（§7.2），不阻断写入。
  - 全部成功 → 返回创建的规则列表。
- **创建前置校验**：`common.EmailVerificationEnabled == false` 时直接拒绝，提示"未开启邮箱验证，规则不会生效"。
- **DELETE 必填 `reason`**（与组织其他破坏性操作一致），走审计。
- 规则 id 与组织 id 必须匹配（`WHERE id = ? AND organization_id = ?`），避免跨组织删规则。

## 10. 审计与可观测

沿用 `recordOrganizationAudit`（`service/organization_audit.go`），命名与现有常量形态一致：

| action | target | 内容 |
|---|---|---|
| `organization.member.auto_join` | `member` | 自动加入。operator = 新用户 id，operatorRole = `organizationAuditOperatorRoleSystem`，`reason` = `join_rule:<pattern_normalized>`。让组织成员能在审计里查到"这个人为什么在我组织里" |
| `organization.join_rule.create` | `join_rule` | 规则新增，带 after 快照与 operator（root） |
| `organization.join_rule.delete` | `join_rule` | 规则删除，带 before 快照与 reason |

规则增删同时 `common.SysLog` 一条，便于平台侧排查（规则是跨组织的全局约束，冲突信息要能离开组织审计被看到）。

## 11. 前端与 i18n

- 平台面（`web/src/features/organization-admin/`）组织详情增加"加入规则"一节：规则列表（类型 / pattern / 创建人 / 时间）+ 新增表单（多行文本框批量粘贴 + reason）+ 删除确认（必填 reason）。
- 新增表单**实时回显"将匹配"**：提交前在客户端本地展开每个 pattern 的示例（`*.enterprise.com` → 展示会命中裸域名与子域两种形态），避免管理员写了通配符却以为只匹配子域这类静默误解。纯前端计算，不新增接口。
- 按 root 角色显隐该入口（后端为强制点）。
- 注册页**零改动**。
- 新增文案同步 7 个语言包（`web/src/i18n/locales/*.json`，`bun run i18n:sync`）；后端新增错误信息走 `i18n/` 的 en/zh key，用 `common.ApiErrorI18n`。
- 前端新文件用 `bun run copyright` 补头（`web/AGENTS.md`）。

## 12. 测试与三库验证

### 12.1 单元测试（`require` + `assert`，确定性表驱动）

`MatchJoinRule` 与 `JoinRulesOverlap` 的恶意/边界样例：

- `evil-enterprise.com` 不命中 `enterprise.com` / `*.enterprise.com`
- `enterprise.com.evil.com` 不命中（后缀必须锚定在 `.` 上）
- `*.enterprise.com` 命中裸域名与多级子域；`enterprise.com` 不命中子域
- 大小写混写、前后空格、结尾点、`xn--` punycode 形式
- 地址规则严格全等：`user-a@enterprise.com` 不命中 `user-a+1@enterprise.com`、`xuser-a@enterprise.com`
- 非法输入：裸 `*`、`*` 在中间、含 `@` 的 domain 规则、无点的 domain 规则、超过 191 字符
- 冲突：同域、父子域两个方向、地址规则落在域名规则范围内

### 12.2 控制器/服务测试

- 白名单邮箱注册 → 成员行（`member`/`active`/`InvitedBy=0`）+ 审计行（`organization.member.auto_join`，reason 含命中的 pattern）
- 非白名单邮箱注册 → 无成员行，注册成功
- 组织 `disabled` → 无成员行，注册仍成功
- 地址规则与域名规则同时命中（不同组织）→ 进地址规则所属组织
- 提示语义：地址规则被其他组织域名规则覆盖 → 写入成功并返回 notice；新增域名规则覆盖其他组织已有地址规则 → 写入成功并返回 notice；同一组织内部覆盖 → 不产生 notice
- 加入写库失败 → 注册失败，用户未创建
- OAuth 注册即使邮箱命中规则也不加入
- 路由鉴权：root 通过、平台管理员（非 root）403（可参照 `router/organization_create_router_test.go` 的用例形态）

### 12.3 数据库

按 AGENTS.md，新表与唯一索引必须在 **SQLite / MySQL / PostgreSQL** 三库验证：

- 新建库、从最新发布版升级各跑一遍；AutoMigrate 连续执行两次证明幂等
- 验证唯一索引在三库下都真实生效（重复 pattern 插入必须失败）
- 记录确切版本、命令与结果到 PR

### 12.4 前端

`web/AGENTS.md` 的构建与 lint 必须通过；规则表单的校验与 preview 逻辑有组件测试。

## 13. 交付物

| 层 | 文件 |
|---|---|
| 模型 | `model/organization_join_rule.go`（新）、`model/main.go`（AutoMigrate 注册） |
| 服务 | `service/organization_join_rule.go`（新：归一化、校验、匹配、冲突、解析、加入） |
| 控制器 | `controller/organization_join_rule.go`（新：列表/批量创建/删除） |
| 注册接入 | `controller/user.go`（`Register` 事务内一处调用） |
| 路由 | `router/organization-router.go`（root 子组三条路由） |
| 审计 | `service/organization_audit.go`（三个 action 常量） |
| i18n | `i18n/`（en/zh 新增 key） |
| 前端 | `web/src/features/organization-admin/`（列表/表单/删除）、`web/src/i18n/locales/*.json` |
| 测试 | `service/organization_join_rule_test.go`、`controller/` 对应测试、`router/` 鉴权测试 |

## 14. 风险

| 风险 | 缓解 |
|---|---|
| 规则配错导致范围过大（例如误配 `*.com`） | 创建时回显匹配示例；域名规则之间的父子域冲突被拒绝；root 独占 + 审计 |
| 陌生人经白名单进入组织后**可读到组织全部 API Key**（普通 `member` 角色自带 `ViewOrganizationTokens`，`service/organization_token.go:234` 注释明确"完整 secret 在列表和详情每次都返回"） | 本功能把白名单视作凭证级控制：root 独占配置、只认已验证邮箱、不追溯、全量审计 |
| 规则存在但静默不生效 | 未开启邮箱验证时拒绝创建；多组织命中时只取地址规则且配置期给出提示 |
| 并发写规则导致互斥失效 | 唯一索引兜底；冲突检查在同一事务内执行 |
| 注册热路径被拖慢 | 解析是唯一索引等值查询 + 极少数后缀值，无全表扫描，无行锁 |
