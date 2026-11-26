# API 密钥：root/admin 全用户视图（短设计）

日期：2026-11-24
状态：待批准

## 意图

管理员帮用户排查（如「我的 Key 不能用」）和做风控/滥用调查时，需要在 API 密钥页看到全站用户的个人 Key：核对配置与状态、复制完整 Key、必要时启停或删除，并且所有跨用户动作留下审计记录。

## 目标

- API 密钥页新增 `全部 / 仅自己` Tabs，仅 admin 及以上可见；默认停在「仅自己」。
- 「全部」列出全站**个人** Key（含禁用/过期/耗尽），带归属用户（用户名/显示名），支持按名称、Key、用户名、状态筛选。
- 可见与可操作范围遵循现有角色层级：admin 只能看/操作角色低于自己的用户（以及自己）；root 全可见。
- 复制完整 Key、启停、删除走新的管理员接口；成功时写手动管理审计，失败时由 `AdminAuth` 的写操作兜底审计记录。
- 其他用户的 Key 可查看配置（模型限制、IP 限制、额度、时间等），但不提供编辑（改额度/分组/模型限制）。

## 非目标

- 不包含组织 Key（组织 Key 仍在组织详情页和「仅自己」里管理）。
- 不做批量删除/批量复制/多选操作。
- 不改普通用户现有接口的行为，不改数据库 schema。
- 不提供编辑他人 Key 的能力；不改 Key 的归属/转移。

## 方案

**A（选用）**：新增 `/api/admin/token` 接口组，统一挂 `AdminAuth`；前端 Tabs 切换数据源。安全边界清晰，写操作自动进入审计兜底；只处理个人 Key，不和组织权限模型混在一起。

**B（不选）**：把组织 Key 也并入全局列表。清单更全，但需要合并两套权限、复制组织 Key 另走组织接口，改动和风险明显更大。

**C（不选）**：在现有 `/api/token/self/all` 上加 `scope=all`。路由最少，但把管理员级读取塞进 UserAuth 接口，失去审计中间件兜底，也更容易出越权缺陷。

## 设计

### 后端路由（全部使用 `middleware.AdminAuth()`）

| 方法 | 路径 | 中间件 | 说明 |
|------|------|--------|------|
| GET | `/api/admin/token/` | `SearchRateLimitWhenQuery` | 全站个人 Key 列表 |
| POST | `/api/admin/token/:id/key` | `CriticalRateLimit` + `DisableCache` | 复制完整 Key |
| PUT | `/api/admin/token/:id/status` | — | 启用/停用 |
| DELETE | `/api/admin/token/:id` | — | 删除单条 |

`SearchRateLimitWhenQuery` 扩展为同时识别 `username` 查询。

### 权限规则

统一用「操作者是否可管理目标用户」判定：

- root：任意用户。
- admin：`targetUser.Role < RoleAdminUser`，或 `targetUser.Id == 操作者 Id`（自己）。
- 其他角色进不来（AdminAuth 拦截）。
- 仅 `scope_type = personal`（含历史空值）可被管理；组织 Key 一律按不存在/无权限处理。
- 列表查询按同一规则过滤（`users` 子查询），admin 看不到其他 admin/root 的 Key，但能看到自己的。

### 查询与模型

- `model.Token` 新增只读字段 `OwnerUsername`、`OwnerDisplayName`（`gorm:"-"`，列表补全用）。
- 新增 `model.AdminPersonalTokensQuery(operatorId, operatorRole)`：个人作用域 + 角色可见性条件，返回 `*gorm.DB`，供 service 叠加过滤。
- 新增 `service.ListAdminPersonalTokens`：复用「仅自己」列表的名称/Key/状态过滤语义，新增 `username` 过滤（`users.username` / `users.display_name` LIKE）；`id desc` 分页；返回前一次性补全 owner 名称并把 Key 掩码。
- 状态过滤复用 `applyOrganizationTokenStatusFilter`（过期按时间判定，对个人 Key 同样成立）。
- 新增 `service.GetAdminManageablePersonalToken(operator, tokenId)`：加载 token + owner，执行权限规则；复制/启停/删除三个动作共用。

### 审计

- 新增 action：`token.key_view`、`token.status_enable`、`token.status_disable`、`token.delete`。
- 参数：`token_id`、`token_name`、`target_user_id`、`target_username`。
- 成功时调用 `recordManageAudit`（自动标记跳过兜底）；失败不标记，由 `AdminAuth` 的写操作兜底记 `generic`。
- 在 `controller/audit.go` 补英文模板，在 `web/src/features/usage-logs/lib/format.ts` 补 `AUDIT_TEMPLATES` 并同步 i18n。
- 复制完整 Key 是敏感读取，必须审计（对齐 `channel.key_view` 先例）。

### 前端

- `ApiKeysProvider` 增加 `viewScope: 'self' | 'all'`（默认 `self`），通过 context 提供。
- `ApiKeys` 页头：`isAdmin(role)` 时渲染 `全部 / 仅自己` Tabs（复用用量日志页样式）；「全部」模式隐藏「创建」按钮（创建始终落在当前账号下）。
- `api.ts` 新增四个管理员接口；`types.ts` 增加 owner 字段、`username` 参数与响应类型。
- 「全部」模式：
  - 数据源切到 `/api/admin/token/`，`queryKey` 带 scope。
  - 列新增「归属用户」（username/display name）；隐藏「计费账户」（全为 Personal）。
  - 工具栏在名称/Key 搜索旁增加「按用户名」输入框（500ms debounce）。
  - 行操作：查看、复制、启停、删除；无编辑。复制完整 Key 由 Key 单元格经 provider 走管理员 reveal 接口。
  - 删除确认文案包含归属用户。
  - 移动端行展示归属用户。
- 「仅自己」模式行为完全不变。

## 约束

- 无数据库 schema/迁移变更；查询需同时兼容 SQLite、MySQL、PostgreSQL（`users` 子查询、`LIKE ... ESCAPE '!'`）。
- 列表中的 Key 始终掩码；完整值只由 reveal 接口返回。
- 新文件带 AGPL 头（`Copyright (C) 2023-2026 QuantumNous` + `Copyright (C) 2026 CubeRouter`）；不整仓跑 copyright。
- 单个 PR 变更文件数 < 30。

## 测试

后端：

- 列表权限矩阵：admin 看不到其他 admin/root 的 Key、能看到 common/ops 与自己的；root 全可见；组织 Key 不出现。
- 动作权限矩阵：admin 不能复制/启停/删除更高或同级角色用户的 Key（自己除外）；root 可以；组织 Key 被拒。
- 过滤：username、名称、Key 前缀 `sk-`、状态与分页；返回 Key 已掩码且 owner 名称已补全。
- 审计：成功的复制/启停/删除各写入一条 `LogTypeManage` 日志，action 与目标用户参数正确。

前端：

- 密钥页测试：非 admin 无 Tabs；admin 有 Tabs；「全部」调用管理员接口并携带筛选参数；行操作在「全部」模式调用管理员接口。
- i18n 同步与 typecheck/build。

## 待定

无。
