# 模型广场管理员全量视图（短设计）

日期：2026-11-23
状态：待批准

## 意图

root/admin 在模型广场（价目表）应能看到站点当前在服务的全部模型，而不是被自己账号所在分组的可用分组限制；普通用户维持按可用分组过滤。

## 目标

- `GET /api/pricing` 对 `role >= RoleAdminUser`：返回全量价目行，`usable_group` 展开为价目表里出现过的全部业务分组（带描述），`group_ratio` 保留这些分组。
- 普通用户、匿名用户、非管理员组织上下文：行为完全不变。
- 前端零改动：侧边栏分组筛选直接读 `usable_group`，管理员会自然看到全部分组。

## 非目标

- 不加 `全部 / 我的分组` 开关（用户已选方案 A）。
- 不把 `models.status != 1`（元数据禁用）或没有任何启用渠道的模型放进价目表——它们仍属于「模型管理」页。
- 不改 `/v1/models`、Playground、令牌分组等调用侧的分组限制。
- 不改管理员账号自身的 `auto_groups` / 模型折扣展示。

## 方案

**A（选用）**：管理员始终看全站。后端单点分支，前端零改动；Model Square 是目录页，分组筛选本身已能收窄。
**B（不选）**：加 `全部 / 我的分组` 开关。交互与用量页一致，但公共价目页要引入 scope 状态、默认值，并让价格展示跟随 scope；对目录页是额外复杂度。

## 设计

在 `controller/pricing.go` 的 `GetPricing` 中：

- 新增 `collectPricingGroups(pricing []model.Pricing) map[string]string`：汇总价目表所有行的 `EnableGroup`，跳过 `""` 与 `"all"`，描述优先 `setting.GetUsableGroupDescription(group)`。
- `role >= common.RoleAdminUser` 时：
  - `usableGroup = collectPricingGroups(pricing)`
  - 跳过 `filterPricingByUsableGroups`
- 否则保持现状：`usableGroup = service.GetAccountUsableGroups(group)`，再过滤价目表。
- 组织成员白名单过滤保持原样（管理员 allow 为 nil，天然跳过）。
- `group_ratio` 的末尾裁剪保持现状：管理员保留价目表里出现的全部分组中配置了倍率的项。

行为说明：管理员在组织上下文时也按平台管理员看全站（角色优先于组织分组）；管理员账号自身的 `GroupGroupRatio` 特例仍会折叠进 `group_ratio`（沿用现有计算顺序与 #178 的参考实现）。

## 约束

- 无数据库/schema 变更；纯读路径。
- 不改共享价目缓存元素；只按请求重建切片。
- 前端不需要新增 i18n 文案。

## 测试

- `TestCollectPricingGroupsExpandsAdminView`（从 #178 回退的 commit 取回）：`all` 不进入分组集合；已配置描述沿用；未配置描述退回分组名；空价目表返回空集合。
- 继续跑 `controller` 现有价目表/组织上下文测试，确认普通用户路径未变。
- 前端 `pricing` 相关测试与 typecheck（无代码改动，跑一遍确认）。

## 待定

无。
