# Organization Member Model Access (组织成员模型白名单)

> Last Updated: 2026-10-16

本文档讲 cuberouter 的「组织成员模型白名单」：组织所有者/管理员可以为**本组织的单个成员**配置一份可用模型清单，限制该成员**能看、能调**的模型范围；**未配置（默认）即不限**。

- 白名单挂在**组织成员关系**上（`organization_members.model_access` 列），但生效时**跟随人**——成员本人的所有调用与视图一并收窄（§2）。
- 只**收窄**、不放宽：它是在分组/渠道/令牌「可用模型限制」之上的又一个闸门（§3）。
- 多组织成员：各组织的白名单取**交集**，未配置的组织视为全集（§1）。
- 与「用户 × 模型折扣」正交：折扣改价、白名单改可用范围，两者同批引入、互不影响（折扣见 `docs/billing/user-model-discount.md`）。

## 1. 口径

- **挂载点**：`organization_members.model_access` 列（TEXT，存 JSON 字符串数组；NULL/空串 = 未配置）。该列对常规成员对象 `json:"-"`，不随成员列表外泄，只经专用接口按权限返回（§5）。
- **默认未配置 = 不限**：白名单是加法管制的反面——没有配置就什么都不拦。
- **只收窄不放宽**：白名单只能从既有可用集合里再剔除模型，永远不会授予原本不可用的模型。
- **多组织交集**：成员的**有效集** = 他所在各组织「已配置且生效」的白名单的逐条**精确字符串交集**；未配置的组织不参与（视为全集）；一个受限组织都没有 → 不限。注意交集按字符串精确计算（同一个模型的通配/具体写法不会自动合并），给同一批模型配置多个组织时请用同一套写法。
- **平台管理员豁免**：`role >= admin`（`model.IsAdmin`）的账号永远不受限——平台管理面不因组织配置被锁死。
- **组织状态联动**：只有 **active 组织**里的 active 成员关系参与计算。组织被**禁用/解散** → 限制**立即解除**（含该成员的个人 key）；组织重新**启用** → 限制**回归**。解散的组织没有管理员了，无人能再解除限制，所以限制不能比组织活得更久。
- **成员关系联动**：只有 `status = active` 的成员关系参与；成员被禁用/移除/退出后立刻失效。**重新加入会复活同一行**，行上残留的白名单随之恢复生效（§5 的失效点覆盖加入/重新加入）。
- **容量与条目**：单成员清单 ≤ **2000 条**、序列化 ≤ **60 KiB**（受 MySQL TEXT 64 KiB 上限约束，并给审计快照留余量）、单条模型名 trim 后 ≤ 255 字节；写入时 trim、去空、去重。超限或非法一律 400 拒绝、不落库。
- **脏数据 fail-open**：存储列里解析不出内容的值——空串、`null`、`"[]"`、无法解析的 JSON——一律按「未配置」（= 不限制）处理；其中**无法解析的 JSON、被丢弃的超长条目与被截断的超限清单**会记 `SysError` 留痕（空串/`null`/`"[]"` 静默按未配置）。原则是「脏数据不锁死用户」。
- **匹配归一化**：放行判定时，清单条目与请求模型名**都**过 `ratio_setting.FormatMatchingModelName` 再比对（与计费、令牌模型限制同一套归一化：`gpt-4-gizmo-*` / `gpt-4o-gizmo-*`、gemini thinking 预算模型）：存 `gpt-4-gizmo-*`，请求 `gpt-4-gizmo-abc` 放行；反向同样成立——存具体名 `gpt-4-gizmo-abc` 也会先归一到通配，因此同样放行该族其它具体名。归一化只作用于**放行比对**：条目本身与多组织交集仍按精确字符串处理（见「多组织交集」——把 `gpt-4-gizmo-*` 和 `gpt-4-gizmo-abc` 分别配到两个组织上，交集是空集）。

## 2. 生效范围（跟随人）

白名单是**人**的属性，先于「用哪把 key 调用」成立：

| 场景 | 判定用的用户 | 依据 |
|---|---|---|
| 个人 key 调用 | 令牌属主本人（`token.UserId`） | `service/organization_token_auth.go`（认证解析 actor） |
| 组织 key 调用 | key 的**责任成员**（`token.ResponsibleUserId`）——换人负责即换白名单 | 同上 |
| 会话/账号上下文（dashboard 视图） | 登录用户本人 | `middleware/account_context.go` |

判定点统一在 `middleware/distributor.go`：读 `actor_user_id`（缺失时退回登录/令牌属主）后查询该用户的有效白名单。

- **视图同样受限**：受限成员自己打开 Playground、建 key、概览、定价页时，看到的模型清单就已收窄（§4）。
- **任务/状态查询**：视频任务的状态类端点（GET）不解析请求模型，distributor 会从已存任务**回填** `OriginModelName` 再做白名单判定；查不到任务记录时保持空模型（与令牌限制同路径行为）。

## 3. 运行时行为

- **单点拦截 + 交集**：模型检查集中在 `middleware/distributor.go` 的检查块。先查令牌的「可用模型限制」（既有），再查成员白名单；两道都通过才放行——效果是**令牌限制 ∩ 成员白名单**。
- **403 语义**：被成员白名单拒绝时返回 403，消息键 `distributor.member_model_forbidden`（三语：`模型 {{.Model}} 不在组织为你的账号设置的允许范围内`）；令牌限制的 403 仍是既有 `distributor.token_model_forbidden`，两者可从文案区分。
- **空模型跳过**：请求模型为空时跳过白名单检查——空模型只出现在状态/查询类路径（如 MJ 任务轮询，记录里没有模型来源）；提交类路径要么被紧随其后的「必须有模型」400 拦下，要么由下面的 remix 补判覆盖。
- **remix 补判**：视频 remix（`POST /v1/videos/:video_id/remix`）是**空模型的提交类路径**，上面的空模型跳过覆盖不到，由 `relay/relay_task.go` 的 `ResolveOriginTask` 在**源任务模型**上单点补判（同样 403 `member_model_forbidden`）。
- **pin 分支豁免（已知边界）**：渠道绑定的调用走 distributor 的 pin 分支（`constraints.ResolvedPin()` 命中，来源为渠道绑定令牌 `token` 或 task-plugin 续传/源任务 `origin_task`）——该分支直接锁定 pin 的渠道，**不进入模型检查块**，令牌「可用模型限制」与成员白名单都不查（既有行为：pin 分支本就不看令牌限制，成员检查与它同形）。受限成员若持有 pin 到固定渠道的 key，仍可调用该渠道上的任意模型；登记在案。

## 4. 可见性与建 key 校验

受限成员「看不到」的模型，也不能通过 API 使用：

| 面 | 端点/路径 | 行为 |
|---|---|---|
| Playground / 建 key 下拉 / 概览 | `GET /api/user/models` | 只返回白名单内的模型；未受限用户响应逐字节不变 |
| API 模型列表（SDK 发现面） | `GET /v1/models`（按请求头分派 OpenAI/Anthropic/Gemini 形态）、`/v1beta/models`、`/v1beta/openai/models` | 只返回分组可见 ∩ 白名单内的模型；未受限用户响应逐字节不变 |
| 定价页 | `GET /api/pricing` 的 `data.pricings` | 只返回白名单内的模型价目；只为本请求重建切片，绝不改写共享缓存 |
| 建/改个人令牌 | `POST /api/token/`、`PUT /api/token/` | `model_limits` 启用且超出白名单 → 400（i18n `token.model_outside_member_access`） |
| 建/改/批建组织 Key | `POST /api/organizations/:id/tokens`、`PATCH /api/organizations/:id/tokens/:tokenId`、`POST /api/organizations/:id/token-batches` | 同上，按 key 的**责任成员**校验，超界 → 400（组织错误通道） |

- 不启用或为空的 `model_limits` 按「未设置」放行；白名单为「不限」一律放行。
- 建 key 校验只是把越界**提前**反馈；运行时 distributor 的交集才是最终闸门（存量 key 的越界 limits 会在调用时被拦）。
- 组织 Key 表单的「模型限制」候选：组织中心（member surface）按所选使用者（责任成员）的白名单收窄候选，切换使用者时同步裁剪已选；平台管理员面（`/api/admin/organizations/...`）没有该读取的镜像路由，候选保持全量——保存仍由服务端按使用者白名单校验（越界 400），运行时交集兜底。
- `GET /v1/models/:model`（单模型元数据查询）保持既有行为、不按白名单收窄：它的数据源是全站静态模型表，与分组、令牌限制同样无关；收窄的是列表发现面，单查面维持原样（已知边界，与 pin 分支豁免同批登记）。

## 5. 配置入口

| 项 | 值 |
|---|---|
| 读 | `GET /api/organizations/:id/members/:userId/model-access`（与成员列表同权限面 `view_members_full`）→ `{"access": [...], "models": [...]}`（`access` = 当前白名单，`models` = 可选全集，取自本组织分组的可见模型） |
| 写 | `PUT /api/organizations/:id/members/:userId/model-access`（能力 `manage_member_models`，owner/admin；body `{"models": ["模型名", ...]}`） |
| 组织 UI | 组织中心（Organizations）→ 成员列表 → 成员行操作菜单「Model Access」（中文界面：模型访问）→ 对话框「Member Model Access」 |
| 审计 | 组织审计 `organization.member.model_access.update`（target=member，含 before/after） |

- **全量替换**语义：PUT 整体替换该成员的清单；**空选择 = 解除限制**（不是"禁止一切"）——对话框的 Clear（清空选择）即此语义。
- 目标必须是本组织**在职（active）**成员；对 owner 行与不低于操作者等级的行沿用既有等级校验。
- 能力与 `manage_members` **解耦**：`manage_member_models` 单独授予（owner/admin），响应里以 `can_manage_member_models` 下发控制 UI 显隐。

## 6. 缓存与失效

- **键与值**：Redis 键 `user:model_allowlist:<userId>`；值 JSON——`{"u":true}` = 不限，`{"m":[...]}` = 受限集（`{"m":[]}` = 一个模型都不允许，与"不限"不同）。存储列里的空表示「未配置」，而受限交集为空是一个真实状态，在缓存里由 `{"m":[]}` 承载。
- **读路径**：先缓存后 DB，DB 是权威。TTL 复用用户缓存 TTL（`SYNC_FREQUENCY` 环境变量，缺省 60 秒）自然收敛。**每次判定 = 1 次裸 `IsAdmin` SELECT（`SELECT role`，未缓存）+ 1 次白名单读取**：Redis 开、命中 = 1 SELECT + 1 GET；未命中 = 1 SELECT + 2 GET（多读一次栅栏键）；Redis 关 = 2 SELECT（读取直落 DB）。视频任务状态类 GET 在令牌「可用模型限制」未启用时会先回填、再判定，getter 共两次（即再多一组 SELECT/GET）。
- **写后显式失效**：白名单**不推进认证版本**，所以必须显式失效——配置写入后、以及成员关系变更后（加入/重新加入、移除、退出、禁用/启用）都失效该成员的缓存；组织**禁用/启用/解散**时失效该组织全部 active 成员的缓存（组织级刷新 best-effort，N 次串行原子脚本）。
- **回填栅栏**：失效用原子脚本同时推进 `user:model_allowlist:ver:<userId>` 并删除缓存键；未命中的回填在读取 DB **之前**捕获栅栏值，写入时再用原子脚本比对，栅栏已变则放弃写入。这样「读请求读到旧快照 → 管理员写入并失效 → 读请求把旧值写回」的竞态不会把旧值钉住整个 TTL。栅栏键持久保留（每次失效递增），只在发生失效的用户上产生。
- **失败方向**：DB 读失败 → 按不限处理（fail-open，记 `SysError`），且**不写入缓存**（避免把「不限」钉住 TTL）；失效失败只记日志，缓存未命中会回源 DB、TTL 到期自然收敛。

## 7. 存储与迁移

- 列：`organization_members.model_access` TEXT（GORM tag `type:text;column:model_access`，`json:"-"`）；空串/NULL = 未配置。
- 迁移：`AutoMigrate` 新增列，**旧行保持 NULL**（= 不限），升级不丢数据、唯一约束（`idx_org_user`）与既有索引不受影响。
- 三库已验证（SQLite 3.50.4 / MySQL 5.7.44 / PostgreSQL 15.19）：fresh 建表含列、升级加列、重启幂等；列存在前插入的旧行（`model_access` 为 NULL）在迁移后按「不限」参与 getter。

## 8. 回滚

- **数据**：列留空即恢复原行为——旧行本就是 NULL；代码回滚后列保留但**不再被读写**，无需删列。
- **缓存**：`user:model_allowlist:*` 键随 TTL 自然过期（回滚后的代码不再读写它）；无需手工清理。
- **行为**：白名单判定集中在 distributor/remix 单点与几处可见性过滤点，回滚代码即恢复全部模型可见可调。

## 9. 相关

- 「用户 × 模型折扣」（改价，同批引入）：`docs/billing/user-model-discount.md`
- 组织体系总览（成员/角色/能力模型）：`docs/organization-system.md`
