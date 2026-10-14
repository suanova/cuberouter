# Fixed-Rate Per-Token Billing (固定费率按 token 计费)

> Last Updated: 2026-09-30

本文档讲 cuberouter 最常用的一种计费模式:**按 token 用量计费、费率固定**。kimi-k3 用的就是这种。

其他计费模式(按次固定价 ModelPrice、表达式/分时计费、视频按秒、图像按张)不在本文范围;表达式/分时计费见 `docs/billing/tiered-expr.md`(设计细节见 `pkg/billingexpr/expr.md`)。

## 1. 计费单位

系统内部货币单位是 **quota**:

```
$1 = 500,000 quota        (common/constants.go: QuotaPerUnit)
```

所有费用先算成 quota 整数,再从用户余额扣减。展示层再换算回美元。

## 2. 单位换算:quota ↔ 倍率 ↔ 美元

整套体系只有两个常数,其余全是推导:

```
common/constants.go:                   QuotaPerUnit = 500 * 1000   // $1 = 500,000 quota
setting/ratio_setting/model_ratio.go:  USD = 500                  // 祖宗常数:1 unit = $0.002 → $1 = 500 unit
                                                                   // quota = unit × 1000
```

### 2.1 倍率的物理含义:1 token = ModelRatio 个 quota

计费式里 token 数是直接乘倍率的(`service/text_quota.go`),所以

```
1 个输入 token 的费用 = ModelRatio 个 quota
                     = ModelRatio / 500,000 美元
                     = ModelRatio × $2 / 1M tokens
```

即 **`ModelRatio = 单价($/1M) ÷ 2`**。所谓"倍率 1 = $2/1M",和"1 token = 1 quota"是同一句话;那个 `2` 就是 `QuotaPerUnit` 的倒数,不是约定俗成的魔法数。

### 2.2 换算表

| 从 | 到 | 公式 | 代码 |
|---|---|---|---|
| 倍率 r | $/1M | `r × 2` | `features/pricing/lib/price.ts: calculateTokenPrice` |
| $/1M | 倍率 | `÷ 2` | `model-pricing-core.ts: basePriceToRatio`(先 `displayPriceToUsd` 折回美元) |
| 倍率 r | $/token | `r / 500,000` | `service/quota.go: CalcOpenRouterCacheCreateTokens` |
| USD | quota | `× 500,000` | `relay/helper/price.go`(按次路径 `modelPrice × QuotaPerUnit`) |
| $/1M | quota(每 token) | `价 / 1e6 × QuotaPerUnit` | `pkg/billingexpr`: `rawCost / 1_000_000 * QuotaPerUnit` |
| lane 倍率 | lane 单价 | `lane 单价 ÷ 输入单价` | `model-pricing-core.ts: createInitialLaneState` |

### 2.3 三个容易踩的点

- **倍率与汇率无关**。显示货币(`usdExchangeRate`、自定义货币)只影响前端输入框里那个数字;落库的倍率是纯倍数,换显示货币既不改倍率也不改历史账——前端在 ÷2 之前先把显示价折回美元。
- **quota 不等于用户看到的"钱"**。展示类型有 `USD / CNY / TOKENS / CUSTOM`(`web/src/lib/currency.ts`),`TOKENS` 直接把 quota 当 token 显示(500,000 tokens = $1);充值那边 1 USD 卖多少本地货币由 `Price`(充值价)决定,和 `usdExchangeRate` 不是一回事。
- **1K 口径 vs 1M 口径**。老代码注释写 `1 === $0.002 / 1K tokens`(一句话里 token 的基准量),新 UI 和本文用 $/1M,两者同源(`$0.002/1K = $2/1M`),别当成两套约定。

## 3. 配置项

固定费率模式由几张倍率表决定。**后台入口:系统管理 → 计费与支付**(不是"运维/运营"分区——那里只有系统行为、监控告警、SMTP、Worker、日志、性能):

| 后台页面 | 前端路径 | 管什么 |
|---|---|---|
| 模型定价 | `/system-settings/billing/model-pricing` | `ModelRatio` / `CompletionRatio` / `CacheRatio` / `CreateCacheRatio`(以及 Image/Audio 系列、`ModelPrice`、表达式计费) |
| 分组定价 | `/system-settings/billing/group-pricing` | `GroupRatio`(还有 `TopupGroupRatio`、可选用组、组间覆盖、auto group) |

配置项本身:

| 配置项 | 含义 | 换算规则 |
|---|---|---|
| `ModelRatio` 模型倍率 | 输入 token 的基准价 | **倍率 1 = $2 / 1M tokens**(祖传约定) |
| `CompletionRatio` 补全倍率 | 输出价格 = 输入价格 × 此倍率 | 输出通常比输入贵 3~5 倍,跟随上游官方定价 |
| `CacheRatio` 缓存倍率 | 命中 prompt 缓存的输入 token 打几折 | 0.1 = 缓存部分 1 折 |
| `CreateCacheRatio` 缓存写入倍率 | 显式写缓存(如 Claude)的输入 token 单价倍率 | 未设置则不产生该项费用 |
| `GroupRatio` 分组倍率 | 按用户分组整体加价/折扣 | 1 = 无影响 |

补充:缓存写入只有 `CreateCacheRatio` 一个配置项;1h 档不是独立配置,由代码按 ×1.6(6/3.75)从 5m 档派生(`relay/helper/price.go: claudeCacheCreation1hMultiplier`)。

### 3.1 页面上怎么改(两种入口)

模型定价页默认是**可视化**模式,看不到倍率,看到的是价格:

- 表头只有 模型名称 / 模式 / 价格摘要;点某一行的编辑按钮 → 侧边抽屉,按"计费模式"分别填 **输入价格、补全价格、缓存读取价格、缓存写入价格、图像/音频价格**,单位是显示货币的 **$/1M**(`model-pricing-core.ts: laneConfigs`)。
- 抽屉保存时按 `basePriceToRatio` 反推:主价 ÷2 → `ModelRatio`,各 lane 价 ÷ 主价 → `CompletionRatio` / `CacheRatio` / `CreateCacheRatio`…(`model-pricing-core.ts:227`)。**倍率与汇率无关**,所以换显示货币不影响落库的倍率。
- 想直接改倍率 JSON:同一页右上角 **"切换到 JSON"**,才会出现 8 个 JSON 编辑器——`模型倍率`、`提示缓存倍率`、`创建缓存倍率`、`补全倍率`、`模型固定定价`、`图片倍率`、`音频倍率`、`音频补全倍率`(`model-ratio-form.tsx: modelJsonFields`)。

分组定价页同理:可视化模式是"定价分组"表格,`GroupRatio` = **倍率**列,`TopupGroupRatio` = **充值倍率**列;右上角可切 JSON。

两个页面保存的都还是 `/api/option/` 里的同名 option(可视化与 JSON 只是同一份数据的两种前端)。

## 4. 计费公式

代码位置:`service/text_quota.go`,`calculateTextQuotaSummary` 里的 `promptQuota` / `completionQuota` 一段(图像/音频 token 与工具调用附加费是额外的加项,本式只列固定费率的主干)。

```
费用(quota) = [ 未命中缓存的输入token
             + 缓存命中token × CacheRatio
             + 输出token × CompletionRatio ]
             × ModelRatio × GroupRatio
```

由公式直接可得单价表:

| 费用项 | 单价($ / 1M tokens) |
|---|---|
| 输入(未命中缓存) | 2 × ModelRatio |
| 输入(缓存命中) | 2 × ModelRatio × CacheRatio |
| 输出 | 2 × ModelRatio × CompletionRatio |

## 5. 示例:kimi-k3

### 5.1 配置与单价

kimi-k3 在 test 站的配置:

| 配置项 | 值 |
|---|---|
| ModelRatio | 1.3699(= 1/0.73) |
| CompletionRatio | 5 |
| CacheRatio | 0.1 |
| GroupRatio | 1(test 站所有分组均为 1) |

代入单价表:

| 费用项 | 单价 |
|---|---|
| 输入 | 2 × 1.3699 = **$2.74 / 1M** |
| 输入(缓存命中) | 2.74 × 0.1 = **$0.27 / 1M** |
| 输出 | 2.74 × 5 = **$13.70 / 1M** |

### 5.2 一笔具体的账

用户发起一次请求:**输入 10,000 tokens(其中 6,000 命中缓存),输出 2,000 tokens**:

| 部分 | 计算 | quota |
|---|---|---|
| 普通输入 | 4,000 × 1.3699 | 5,479 |
| 缓存输入 | 6,000 × 0.1 × 1.3699 | 822 |
| 输出 | 2,000 × 5 × 1.3699 | 13,699 |
| **合计** | | **20,000 quota ≈ $0.04** |

若 6,000 tokens 未命中缓存,需多付 6,000 × 0.9 × 1.3699 ≈ 7,397 quota——缓存命中为本请求省了约 37% 的输入费用。

## 6. 扣费流程:预扣 → 结算

1. **预扣(PreConsume)**:请求开始前按预估量(含 max_tokens 等上限)冻结一笔 quota;余额不足直接拒绝请求,不会事后欠费
2. **结算(Settle)**:请求结束后按上游返回的实际 usage 重算,多退少补

## 7. 定价页展示

定价页(`/api/pricing` + 前端)把倍率换算成 $/1M 展示:输入价 = ModelRatio × $2 × 分组倍率(`web/src/features/pricing/lib/price.ts: calculateTokenPrice`,`base = model.model_ratio * 2 * ratio`)。后台模型定价表的价格摘要走 `web/src/features/system-settings/models/model-pricing-snapshots.ts`(`getPriceSummary`),同一条 ×2 规则。运营改倍率表,定价页价格随之自动更新;模型的描述/图标/tag 来自元数据表(`docs/model-tags.md`),两者在定价接口处 join。
