# User × Model Discount (用户 × 模型折扣)

> Last Updated: 2026-10-16

本文档讲 cuberouter 的「用户 × 模型折扣」：管理员可以为**单个用户、按单个模型**配置折扣倍率 d，该用户的调用按折后价计费；未配置的模型 d = 1.0（原价）。

- 折扣跟随**人**（调用发起人），与扣费主体（个人钱包/订阅/组织额度）解耦——组织 key 调用按该 key 责任成员的折扣计价（§4）。
- 折扣是**折上折**：乘在分组倍率/专属倍率之上，不替换它们，也不影响可用分组/渠道选择（§1）。
- 计费主干口径（模型倍率、分组倍率、`$1 = 500,000 quota` 等）见 `docs/billing/fixed-rate.md`；本文只讲折扣这一层。

## 1. 口径：折上折、正交叠加

```
最终价 = 模型价/倍率 × [专属倍率 若配置，否则分组倍率] × d × 其他系数
```

- **折上折**：d 乘在组级基准之上，不覆盖、不替换分组倍率。同时配置了专属倍率时两者相乘，例如专属倍率 0.9 × 折扣 0.8 = 0.72。
- **正交**：折扣只改价格——不影响模型可用范围（分组/渠道判定不变）、auto group 选择、限流与余额校验。
- **免费**：折扣不会把付费模型变免费——d 值域 (0, 1] 不含 0；免费组（分组倍率 0）下 `0 × d = 0` 仍免费。要免费请沿用既有机制（分组倍率 0 / 模型价 0），避免两种"免费"来源造成对账歧义。
- **模型键**：精确匹配计费模型名（`GetBillingModelName()`，`relay/common/relay_info.go`），无通配符/前缀匹配。
- **覆盖范围**：全部按模型计价的路径——文本 token、按次（`ModelPrice`）、tiered_expr 表达式、图像按张、视频/任务按次或按秒、实时语音（WSS）。

实现上，d 折叠进 `GroupRatioInfo.GroupRatio`（插点唯一：`relay/helper/price.go` 的 `HandleGroupRatio`，即 `GroupRatio = 基准 × d`），预扣/结算/任务快照/订阅/组织账本自动继承；另有两处自行解析倍率的路径同步乘 d——实时语音逐消息预扣（`service/quota.go` 的 `PreWssConsumeQuota`）与任务异步差额重算（`service/task_billing.go` 的 `RecalculateTaskQuotaByTokens`，用任务提交时的快照值）。同一请求内预扣与结算用的是同一个折叠后的值。

## 2. 值域与写入校验：(0, 1]、4 位小数

- d 必须**有限**且 **0 < d ≤ 1**；写入时四舍五入到 **4 位小数**，校验按舍入后的值做（`model/user_model_discount.go: NormalizeUserModelDiscounts`）——`0.00004` 舍入为 0 → 拒绝；`0.99996` 舍入为 1 → 按 d=1 处理。
- **d = 1 的条目写入时剔除**（等价删除，落库不存 1）。
- 结构上限：模型名 trim 后非空、≤ 255 字节；条目 ≤ 500；序列化 ≤ 64 KiB。超限或非法值一律 400 拒绝、不落库。
- **读路径防御**：存量脏数据（0、>1、NaN/Inf、超长名）不参与计费——逐条校验，非法条目按 1.0 计并记 `SysError`；整列 JSON 无法解析时按无折扣处理。原则是"脏数据按原价，不使请求失败"。
- 存储：`users.model_discounts` TEXT JSON 列（`{"模型名": 0.8}`，与 `users.setting` 同型先例）；空串/NULL = 无折扣；对常规用户对象序列化时 `json:"-"`，不随 `GetSelf` / 用户列表外泄，只经专用接口按权限返回。

## 3. 配置入口（两条路径，写同一份数据）

折扣没有组织级副本：平台与组织两条入口最终都读写同一个 `users.model_discounts`。

### 3.1 平台管理员

| 项 | 值 |
|---|---|
| 读 | `GET /api/user/:id/model-discounts`（ops 及以上读权限面；DB 直读） |
| 写 | `PUT /api/user/:id/model-discounts`（admin）；body `{"discounts": {"模型名": 0.8}}` |
| 后台 UI | 用户列表 → 编辑用户抽屉 → 「Model Discounts」区 |
| 审计 | 管理审计 `user.model_discounts`（before/after） |

- **全量替换**语义：PUT 整体替换该用户的折扣表，`{"discounts": {}}` 即清空；事务内 `lockForUpdate` 读改写，审计留 before/after 兜底并发覆盖。
- 角色护栏沿用既有规则（不能修改自己无权管理的用户）；读取接口返回 `{"success":true,"data":{"discounts":{...}}}`。

### 3.2 组织所有者/管理员

| 项 | 值 |
|---|---|
| 读 | `GET /api/organizations/:id/members/:userId/model-discounts`（与成员列表同权限面） |
| 写 | `PUT /api/organizations/:id/members/:userId/model-discounts`（能力 `can_manage_member_discounts`，与 `manage_members` 解耦） |
| 组织 UI | 组织成员列表 → 成员行操作菜单「Model Discounts」 |
| 审计 | 组织审计 `organization.member.model_discounts.update`（target=member，before/after） |

- 目标必须是本组织**在职（active）**成员；对 owner 成员的操作沿用既有等级校验。成员行入口同样受行级约束（owner 行与不低于操作者等级的行不可操作）。
- **成员退出组织后折扣保留**：折扣是"人"的属性，自动清除会让同一用户的行为随组织关系漂移；平台管理员始终可清。
- 提示：组织 key 是共享凭证，"给一把 key 统一折扣"等价于给该 key 的责任成员配置（§4）。

## 4. 跟随人与归因：查找键 = ActorUserId

| 调用方式 | 用谁的折扣表 | 依据 |
|---|---|---|
| 个人 key | `token.UserId`（令牌属主本人） | `service/organization_token_auth.go` |
| 组织 key | `token.ResponsibleUserId`（该 key 的**责任成员**；须为组织 active 成员，否则调用直接被拒） | 同上 |
| 会话/账号上下文（dashboard 切组织） | 登录用户本人 | `middleware/account_context.go` |

组织 key 是共享凭证、没有调用者登录态，"谁在用"由 key 的责任成员确定：同一成员无论用个人 key 还是归他负责的组织 key，都按同一份折扣；换人负责 key 即换折扣表。**扣谁的额度（个人钱包/订阅/组织额度）不影响折扣表**——折扣与扣费主体解耦。

## 5. 生效与缓存

- **请求内一致（快照）**：鉴权时随认证缓存载入 → `GenRelayInfo` 快照进 `RelayInfo` → 单次请求内不再重读，预扣与结算用同一个 d。任务在提交时把 d 快照进 `TaskBillingContext`（`model/task.go`），轮询阶段的差额重算沿用；**旧任务没有该字段（缺省 0）按 1.0**，不追溯。
- **缓存承载**：折扣随认证缓存下发（`UserBase.ModelDiscounts`，`model/user_cache.go`），认证缓存 schema 版本 **2 → 3**；升级后旧条目整体失效、一次回源，无需数据迁移。
- **写入后同步刷新**：两条写接口在事务提交后调用 `RefreshUserModelDiscountsCache`（把 DB 权威值写回缓存 + 读复核循环）。折扣**不推进 auth_version**，所以必须显式刷新；刷新失败只记日志，缓存过期/未命中时回源 DB，自然收敛（自愈）。
- **兜底与 fail-open**：未经 TokenAuth 的路径（Playground、渠道测试等）折扣表未载入时，按 ActorUserId 惰性读缓存（先缓存后 DB）；读取失败按原价计，不阻断请求。

## 6. 罚金、工具费与图片价表

- **违规罚金**（`service/violation_fee.go: ChargeViolationFeeIfNeeded`）：`罚金 = 金额 × QuotaPerUnit × 有效倍率`，乘的是折叠后的有效倍率，因此**随 d 缩放**——与专属倍率的现状一致（本功能未改该文件）。
- **工具调用附加费**（`service/text_quota.go: calculateTextToolCallSurcharge`）：`附加费 = 配置价($/1K calls) × 次数 ÷ 1000 × 有效倍率 × QuotaPerUnit`。它乘的同样是折叠后的有效倍率，与罚金同机制，**随 d 缩放**；工具价格的**配置**（$/1K）本身是基准价，不因折扣变化（日志里 `tool_surcharges[].price` 记的就是该基准价，见 §7）。
- **图片价表**：定价页的图片价表按固定 USD/张展示**基准价**，不乘折扣（该表本就不做 viewer 折算的既有形态）。

## 7. 展示、日志与对账字段

定价页（`/api/pricing`）响应顶层追加 `user_model_discounts`（对登录用户返回其折扣表；匿名用户为空表；**不写入** `model.GetPricing()` 的共享缓存对象）。前端对命中折扣的用户展示折后价与「Discount N%」徽标，口径是**徽标只在同屏主价格已随 d 折算的界面出现**：

- 卡片/列表：token 价、按次固定价、tiered/dynamic 摘要按 d 折算、显示徽标；**视频/图片摘要按基准价展示**（分辨率/档位明细本就在详情弹窗），这类卡片不显示徽标——基准价与徽标并排会被读成「这些价格已折后」。
- 详情弹窗：价格区已折后（token/按次/动态/视频价表均乘 d），徽标保留；其中图片按张价目表仍按基准价展示（§6），不影响该弹窗的徽标。

日志与对账相关字段：

| 字段 | 位置 | 写入条件 |
|---|---|---|
| `other.user_model_discount` | 消费日志、MJ 日志、任务消费日志、任务差额/退款日志（`service/log_info_generate.go`、`service/task_billing.go`） | 仅真实折扣（d < 1）时写入；d = 1 不写 |
| `other.pre_consume_detail.user_model_discount` | 消费日志的预扣明细（`types/price_data.go`；仅 token/倍率计费路径有该明细） | 明细存在时总是带（1 = 无折扣） |
| `user_model_discounts` | 定价页响应（顶层） | 登录用户 |

- 消费日志的 `group_ratio` 是**折叠后**的有效倍率（含 d）；`user_group_ratio` 仍是专属倍率的原始值。用 `other` 重算金额时 `group_ratio` 不要再乘一遍 d。
- 使用日志 UI：详情弹窗与列表片段显示「User Model Discount 80%」（百分比、去尾零），仅 0 < d < 1 时展示。

## 8. 示例

用户 alice 在模型 M 上配置了 d = 0.8；M 的 `ModelRatio` = 1.3699、所属分组倍率 1。一次请求输入 10,000 tokens（无缓存、无输出）：

| 项 | 计算 | quota |
|---|---|---|
| 原价 | 10,000 × 1.3699 | 13,699 |
| 折后（d = 0.8） | 13,699 × 0.8 | 10,959 |

若 M 在该分组另有专属倍率 0.9：组级基准取 0.9（而不是分组倍率 1），折后单价 = `1.3699 × 0.9 × 0.8`——d 与专属倍率各自独立生效、直接相乘。

## 9. 回滚

- **数据**：功能不依赖表结构变化——`model_discounts` 列留空即原价；代码回滚后列保留、不再被读取，无需删列。
- **缓存**：认证缓存 schema 版本号随代码回滚自然失效一次（3 → 2），条目回源重建。
- **行为**：折扣折叠在 `HandleGroupRatio` 单点，回滚代码即恢复原价口径；历史日志里的 `user_model_discount` 字段保留，可继续用于解释历史账单。

## 10. 相关

- 组织成员的**模型白名单**（收窄成员可见/可调的模型范围，同批引入、与折扣正交）：`docs/org-member-model-access.md`
