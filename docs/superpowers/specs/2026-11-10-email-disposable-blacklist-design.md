# 邮箱域名限制增强：一次性邮箱拦截与可配置黑名单

- 日期：2026-11-10
- 状态：设计待评审
- 涉及站点：cuberouter.cn（cn 三节点）、cuberouter.com（com 两节点）

## 1. 背景与问题

当前邮箱限制只有白名单模式：

- `EmailDomainRestrictionEnabled` + `EmailDomainWhitelist`：只允许列表内域名，精确匹配（`controller/misc.go:246-261`）。
- `EmailAliasRestrictionEnabled`：拒绝 local part 含 `+` 或 `.`。

这表达不了"保留任意正常域名、只拦一次性邮箱"的需求，而后者是注册防刷的常见做法。目标是在不改动现有白名单语义的前提下，增加两层能力：

1. 内置一次性邮箱域名清单（社区维护，随依赖升级更新，不做运行时更新）。
2. 管理员可配置的域名黑名单（精确域 + `*.` 子域通配）。

## 2. 目标 / 非目标 / 反目标

### 目标

- 管理员可单独开启一次性邮箱检测，使用 AfterShip/email-verifier 内置的 75,258 条域名清单。
- 管理员可配置黑名单，支持 `mailinator.com` 与 `*.mailinator.com` 两种写法。
- 校验覆盖发验证码、注册、绑定邮箱三个入口，关闭"先拿码、后改配置"和"绑定时绕过"的口子。
- 所有新开关默认关闭，升级后行为与现在完全一致。

### 非目标

- 不替换或改变现有白名单语义（仍为精确匹配、仍只在启用时生效）。
- 不引入 SMTP 探测、MX 校验、Gravatar、拼写建议等 AfterShip 的其他能力。
- 不做一次性清单的自动更新/后台刷新任务（见 §5.3）；清单更新依赖升级 `AfterShip/email-verifier`。
- 不拦截 OAuth 首次登录、密码重置、管理员/聚合 API 建号；这些路径涉及既有账号，误杀会把用户锁在门外。
- 不提供清单来源/刷新 URL 配置（本次根本没有运行时下载）。
- 不做清单签名校验、离线文件导入、按组织/分组差异化策略。

### 反目标

- 不能因为新功能导致已有用户无法登录、无法重置密码。
- 不能因为新功能在启动或注册请求上引入网络等待。
- 不能因为新功能改变现有白名单配置的行为。

## 3. 现状与相关代码

| 位置 | 现状 |
| --- | --- |
| `common/constants.go:73-85` | 邮箱域名/别名开关与白名单变量 |
| `controller/misc.go:225-303` | `SendEmailVerification`：唯一执行域名/别名校验的入口 |
| `controller/user.go:235-340` | `Register`：只验验证码，不查域名 |
| `controller/user.go:1623-1660` | `EmailBind`：只验验证码，不查域名 |
| `controller/misc.go:305-341` | `SendPasswordResetEmail`：不查域名 |
| `controller/oauth.go:354` | OAuth 首次建号写入邮箱，不查域名 |
| `model/option.go:40-224` | OptionMap 初始化与默认值 |
| `model/option.go:258-330` | `UpdateOption` / `updateOptionMap`：运行时配置生效 |
| `model/option.go:237-243` | `SyncOptions`：多节点按 `SyncFrequency` 轮询 DB 同步配置 |
| `controller/option.go:106-260` | `GetOptions` / `UpdateOption` 管理端接口与启用校验 |
| `web/src/features/system-settings/auth/` | 基础认证设置页（白名单/别名开关与文本框） |
| `i18n/keys.go` + `i18n/locales/{en,zh-CN,zh-TW}.yaml` | 后端错误文案 |

多节点部署下，管理员保存 option 后各节点会在下一个 `SyncOptions` 周期内生效；新开关沿用同一机制。

## 4. 方案总览

新增 3 个配置项，全部默认关闭：

| 配置键 | 类型 | 默认 | 作用 |
| --- | --- | --- | --- |
| `EmailDisposableCheckEnabled` | bool | `false` | 启用 AfterShip 内置一次性邮箱清单检测 |
| `EmailDomainBlacklistEnabled` | bool | `false` | 启用管理员黑名单 |
| `EmailDomainBlacklist` | string | `""` | 黑名单条目，逗号分隔（前端按行编辑） |

校验顺序（`service.ValidateEmailRestrictions`）：

```
1. 别名限制（若开启）：local part 含 '+' 或 '.' → 拒绝
2. 白名单（若开启）：
     域名在列表内 → 通过第 2 步，跳过第 3、4 步
     不在列表内 → 拒绝
3. 黑名单（若开启）：精确域或 *.子域命中 → 拒绝
4. 一次性邮箱（若开启）：内置清单命中 → 拒绝
5. 通过
```

关键语义：**白名单是显式放行，开启时黑名单与一次性检测都不参与**。这样管理员可以用白名单豁免被上游误判的域名，也避免三个开关同时开启时语义互相打架。UI 需要写明这一点。

## 5. 详细设计

### 5.1 域名匹配规则

统一先 `model.NormalizeEmail`（小写 + 去空格），取最后一个 `@` 之后的 domain。

- 精确域：`mailinator.com` 只匹配 `mailinator.com`。
- 子域通配：`*.mailinator.com` 匹配任意深度子域（`a.mailinator.com`、`a.b.mailinator.com`），**不匹配** `mailinator.com` 本身。
- 不支持其他通配形式（`mail*`、`*inator.com`），也不支持正则。
- 条目在保存时规范化：trim、转小写、去空行；校验长度 ≤ 253、不含 `@ / 空格 :`、`*.` 只能在开头；最多 10,000 条。
- 一次性邮箱清单按精确域匹配（与 AfterShip `IsDisposable` 行为一致）。上游清单本身收录了大量子域，但无法覆盖任意子域；需要覆盖时由管理员在黑名单里加 `*.` 条目。
- IDN/Unicode 域名：v1 按 ASCII 小写比较，不做 IDNA 转换；现有白名单同样如此。作为已知限制记录。

### 5.2 校验入口

新增 `service/email_restriction.go`，导出：

```go
var (
    ErrEmailDomainNotWhitelisted error
    ErrEmailDomainBlacklisted    error
    ErrEmailDomainDisposable     error
    ErrEmailAliasRestricted      error
)

func ValidateEmailRestrictions(email string) error
```

调用点：

| 入口 | 变更 |
| --- | --- |
| `SendEmailVerification` | 用 `ValidateEmailRestrictions` 替换现有白名单/别名内联逻辑；错误映射到 i18n |
| `Register` | 当 `EmailVerificationEnabled` 时，在验证码校验前再校验一次最终邮箱 |
| `EmailBind` | 在验证码校验前校验目标邮箱 |

不新增的调用点及理由：

- `SendPasswordResetEmail`：涉及已有账号，且该接口刻意不泄露账号是否存在；加校验会变成"域名被封 → 无法重置"。
- OAuth 首次建号：同样影响既有用户登录；新用户若用一次性邮箱注册 OAuth 账号，属于可接受的漏口。
- 管理员/聚合 API 建号：管理员显式操作，不拦。

`Register` 与 `EmailBind` 的二次校验用于关闭两个竞态：

1. 用户在配置变更前拿到验证码，配置变更后仍可完成注册。
2. 验证码已发出，绑定接口不重新检查域名。

### 5.3 一次性清单：只用内置清单，不做运行时更新

**依赖**：`github.com/AfterShip/email-verifier v1.5.0`（MIT）。

只使用 `emailverifier.NewVerifier().IsDisposable(domain)`，不调用 `Verify()`、`CheckSMTP`、`CheckMX`，也不调用 `EnableAutoUpdateDisposable()`。包 `init()` 会把内置 75,258 条域名装入 `sync.Map`，约增加数 MB 常驻内存；不随开关变化，属于固定成本。实现完成后实测 RSS 增量并记录，若超过 20MB 再评估替代方案。

**为什么不做运行时更新**（2026-11-10 实测）：

- 库自带 `EnableAutoUpdateDisposable()` 硬编码 5s 超时；cn 三台容器拉取同一 1.9MB 文件耗时 8.4–28.7s，每天必然失败。
- 它的更新直接改写包级全局 `sync.Map`，与站点配置生命周期脱钩，无法按站点开关。
- 自研刷新需要额外的后台任务、超时/重试、内存快照与测试；内置清单已覆盖 75,258 条，v1.5.0 于 2026-09 做过一次全量清洗，短期够用。
- 需要补漏时，管理员用黑名单（含 `*.` 子域）处理；清单整体更新随依赖升级完成。

后续如确实遇到清单滞后，再单独立项做刷新（需要长超时、后台异步、失败保留旧清单；cn 实测 8–29s，不能沿用库的 5s 超时）。

### 5.4 配置项接入

- `common/constants.go`：新增 3 个变量及默认值。
- `model/option.go`：
  - `InitOptionMap` 写入默认值；
  - `updateOptionMap` 增加 3 个 case（bool 变量走现有 `HasSuffix("Enabled")` 分支，黑名单字符串单独 case）；
  - DB 中不存在的键保持默认值，无需迁移。
- `controller/option.go` / `model.validateOptionValue` 保存校验：
  - 每次保存 `EmailDomainBlacklist` 都校验条目格式（§5.1），非法条目直接拒绝；允许空列表（清空后黑名单不生效，属于安全的 no-op）。
  - `EmailDomainBlacklistEnabled` 不做非空校验：开启时列表为空只是不生效，避免依赖"先存列表、再开开关"的提交顺序，以及多节点最长 60s 的 `SyncOptions` 同步窗口。
  - 开启白名单的现有校验保持不变（白名单为空会拦截所有人，属于危险配置，必须拦住）。
- `GetOptions`/`UpdateOption` 无需改动，新键自动进入现有 option 列表；前端刷新后可见。

### 5.5 错误文案与 i18n

新增后端 key（`i18n/keys.go` + `en.yaml` / `zh-CN.yaml` / `zh-TW.yaml`）：

| key | 英文 | 中文 |
| --- | --- | --- |
| `user.email_domain_not_allowed` | `Your email domain is not allowed. Please use another email address.` | `该邮箱域名不在允许的域名列表中，请更换邮箱。` |
| `user.email_domain_blacklisted` | `Your email domain has been blocked by the administrator.` | `该邮箱域名已被管理员禁止使用。` |
| `user.email_domain_disposable` | `Disposable email addresses are not allowed.` | `不允许使用一次性邮箱地址。` |
| `user.email_alias_restricted` | `Email addresses containing '+' or '.' in the mailbox name are not allowed.` | `邮箱地址的用户名部分不允许包含 '+' 或 '.'。` |

`SendEmailVerification` 现有硬编码英文/中文文案统一改为 i18n（行为变化：随请求语言返回）。响应结构保持 `{success:false, message}` 不变。

### 5.6 前端

保持现有 "Basic Authentication" 分区，新增 3 个字段，避免拆分页面带来额外回归面：

- 两个开关：
  - `EmailDomainBlacklistEnabled`：文案 "Email Domain Blacklist" / 描述 "Block specific email domains"。
  - `EmailDisposableCheckEnabled`：文案 "Block Disposable Emails" / 描述 "Use a built-in list of disposable email domains"。
- 一个文本域 `EmailDomainBlacklist`：
  - placeholder：`mailinator.com\n*.tempmail.com`；
  - 描述："One domain per line; use *.example.com to block all subdomains"。
- 在域名区域增加一段说明：白名单开启时只允许白名单内域名，黑名单与一次性检测不生效。

实现要点：

- `types.ts`、`auth/index.tsx` 默认值、`section-registry.tsx` 透传、`basic-auth-section.tsx` schema/字段全部补齐。
- 现有白名单的"换行 ↔ 逗号"转换扩展为两个列表共用；抽一个小函数（例如 `normalizeDomainListField`）避免复制逻辑。
- i18n：新增文案以 `t('English source')` 形式出现，运行 `bun run i18n:sync` 同步到 `web/src/i18n/locales/*.json`，手工补 zh / zh-TW。
- 保存仍走现有 `useUpdateOption`，逐项提交；不新增 API。

## 6. 数据、兼容性与部署

- 无数据库 schema 变更，不新增表/列/索引；新 option 仅在管理员首次保存时插入 `options` 表。
- 因此不触发 AGENTS.md 的"三库矩阵"要求；仍会在 SQLite 上跑相关 service/controller 测试。
- 默认值全部关闭，滚动升级期间新旧节点行为一致；配置同步走现有 `SyncOptions` 轮询。
- 回滚：关闭 3 个开关即可恢复原行为，无数据清理。

## 7. 安全与隐私

- 本次不引入任何运行时下载或新的出站请求：一次性清单编译进二进制，黑名单来自管理端 option。
- 管理员可随时通过白名单豁免误判域名。
- 日志不记录用户邮箱；错误响应不暴露内部匹配细节。

## 8. 可观测性

- 校验拒绝：沿用现有 HTTP 200 + `success:false` 响应，不额外打点（避免注册接口日志噪声）。
- 本次不新增后台任务，无新增周期性日志。

## 9. 测试计划

后端：

- `service/email_restriction_test.go`（表驱动）：
  - 白名单关闭时：黑名单精确命中、`*.` 子域命中、大小写、apex 不被 `*.` 命中；
  - 白名单开启时：列表内域名跳过黑名单/一次性检测；列表外拒绝；
  - 别名限制：`+`、`.`、正常 local part；
  - 空邮箱、无 `@`、多 `@` 的输入不 panic；
  - 依赖契约：`mailinator.com` 判定为 disposable，`gmail.com` 判定为非 disposable（防止依赖升级静默改变行为）。
- `controller` 侧：黑名单保存校验（非法条目拒绝、空列表允许）；开启开关不依赖列表是否已先保存。
- 手工 e2e（测试站）：分别开启各开关，用 `x@mailinator.com` 走发码/注册/绑定，确认拒绝文案；关闭后恢复。
- 运行：`go test ./service/... ./controller/... ./common/...`，以及本地全量测试规程中的 CI 等价命令。

前端：

- `web/src/features/system-settings/auth/__tests__/domain-list.test.ts`：换行/逗号互转、空行过滤、保持不变时不提交。
- `bun run typecheck`、涉及文件 lint；手工验证保存后刷新页面回显正确。

不做：库的 `Verify()`/SMTP 路径测试（不使用）；运行时清单下载测试（本次没有下载逻辑）。

## 10. 风险与缓解

| 风险 | 缓解 |
| --- | --- |
| 上游清单误杀正常域名 | 白名单显式豁免；库自身维护 6 条误判豁免；可关闭一次性检测 |
| 任意子域绕过精确匹配 | 黑名单支持 `*.`；文档说明 |
| 内置清单随时间陈旧 | 随 `AfterShip/email-verifier` 升级更新；管理员用黑名单补漏 |
| 依赖包启动内存增加 | 只导入必要包；实测 RSS 并记录；超过 20MB 再评估 |
| 管理员误开黑名单导致注册中断 | 启用时条目校验；默认关闭；白名单优先 |
| 配置多节点传播延迟 | 沿用现有 `SyncOptions` 周期；注册/绑定二次校验保证最终一致 |

## 11. 明确留给实现者的判断

以下为实现细节，不改变产品行为，由实现者按项目惯例决定：

- 黑名单解析放在每次请求时还是缓存到 option 更新时（条目上限 10k，建议缓存）。
- 错误哨兵的具体命名与包内组织。
- 是否把列表格式化逻辑抽成前端 helper 并配套单测（建议抽）。
- 日志字段的具体拼装方式（本次几乎没有新增日志）。
- 黑名单条目格式校验函数的放置位置（controller 或 service）。

## 12. 决策记录（供评审否决）

1. 白名单开启时独占放行，黑名单/一次性检测不参与。
2. 黑名单与一次性检测各自独立开关，默认关闭。
3. 本次不实现清单自动更新/后台刷新；只用依赖内置清单，更新靠升级依赖。
4. 校验覆盖发码、注册、绑定；不覆盖 OAuth 登录、密码重置、管理员建号。
5. 使用 AfterShip/email-verifier 的 `IsDisposable`，不使用其自动更新与 SMTP 能力。
6. 黑名单只在保存时做格式校验，允许空列表；开启开关时不做非空校验（规避提交顺序与多节点同步窗口）。白名单的启用非空校验保持原样。
7. 现有硬编码错误文案统一迁移到 i18n。

## 13. 后续可选（不在本次范围）

动态刷新一次性清单：需要自研后台任务（60s 超时、每日一次、失败保留旧清单），cn 容器实测 8–29s；届时应单独立项，避免把未验证的后台任务带进本次变更。
