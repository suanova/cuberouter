# Tiered Expression & Time-Based Billing (表达式/分时计费)

> Last Updated: 2026-09-30

本文档讲 cuberouter 的第二种主流计费模式:**表达式计费**——一条表达式完整定义模型的价格。`billing_mode = tiered_expr` 的模型走这条路。**分时定价(波峰波谷、夜间/周末折扣)是表达式里的时间条件**,不是独立配置项;阶梯分档、缓存/图像/音频单独计价、请求参数感知倍率(priority/flex、fast-mode)也都在这套表达式里。

- 与固定费率的关系:**同模型二选一**。标了 `tiered_expr` 的模型,ModelRatio / CompletionRatio / ModelPrice 等倍率表对它全部失效;未标记的模型继续走倍率表(见 `docs/billing/fixed-rate.md`)。
- 设计哲学、完整架构见 `pkg/billingexpr/expr.md`;用 REST 批量导入时配合 `docs/import-models/model-pricing.md`。本文是面向运营/接入的实操总结。
- 视频按秒的"错峰时段表"是另一个功能(视频模型的价格表 + `OffPeakWindow` 选项,不在表达式体系内),别与本文混淆。

## 1. 先记住三条事实

1. **系数是真实 $/1M 价**。`p * 2.5` 就是输入 $2.5/1M tokens——直接抄上游官方价目表,没有固定费率那套 "倍率 1 = $2/1M" 的 ×2 换算。
2. **激活即全量接管,没有"部分配置"**。`billing_setting.billing_mode` 里出现 `{"模型名":"tiered_expr"}`,该模型就完全不看倍率表了。
3. **表达式是定价合同**。写什么执行什么,改价 = 改表达式字符串;版本前缀(`v1:`)留给未来演进。

## 2. 计费公式与换算

内部货币单位仍然是 **quota**(`common/constants.go: QuotaPerUnit = 500 * 1000`,`$1 = 500,000 quota`)。

```
token 表达式:  quota = 表达式输出(USD) / 1,000,000 × QuotaPerUnit × GroupRatio
任务表达式:    quota = 表达式输出(USD) × QuotaPerUnit × GroupRatio
```

- token 表达式的输出 = Σ(单价 $/1M × token 数),先除以 1e6 折成美元再乘 `QuotaPerUnit`(`pkg/billingexpr/settle.go: quotaConversion`,v1 语义)。
- 任务表达式(`u("seconds") * 0.4`)的输出**已经是整单美元成本**,直接 × `QuotaPerUnit`,不再除 1e6——`$0.4/秒 × 10 秒 = $4 = 2,000,000 quota`。
- 分组倍率最后叠加,和固定费率一致。

对照固定费率:那边 `quota = tokens × ModelRatio × GroupRatio`;这边倍率表整个被表达式取代,表达式系数即真实美元价。**定价页/结算展示的美元金额 = quota ÷ 500,000**。

## 3. 配置入口与存储

| 项 | 位置 |
|---|---|
| 后台页面 | 系统管理 → 计费与支付 → **模型定价**(`/system-settings/billing/model-pricing`) |
| 编辑抽屉 | 表格行编辑按钮 → 抽屉里 **"表达式 (Expression)"** 页签(`model-pricing-sheet.tsx:689`) |
| 存储 | options 表两个 key:`billing_setting.billing_mode`(`{"模型":"tiered_expr"}`)、`billing_setting.billing_expr`(`{"模型":"表达式"}`) |
| JSON 模式 | 模型定价页右上角切 JSON,也有 `BillingMode` / `BillingExpr` 两个编辑器(`ratio-settings-card.tsx:129`) |
| REST | 两个 key 都要 PUT;保存时后端逐个编译 + 冒烟测试(`controller/option.go:206`,`setting/billing_setting/tiered_billing.go: SmokeTestExpr`) |

编辑器(`tiered-pricing-editor.tsx`)三种用法:

1. **可视化编辑器**:按档位填每个变量的 **$/1M 单价**(输入/输出/缓存读取/缓存写入/缓存写入1h/图像/音频),条件用 `len`/`p`/`c`;顺带把"分时缓存(Claude 5m/1h)"也做了(那是缓存写入两档价,不是分时计费)。
2. **表达式编辑器(Raw)**:直接写字符串;右上角有 **LLM Prompt 助手**(复制一段给 AI 生成表达式的提示词)。
3. **预设模板**:Flat / Claude / GPT / Qwen / Gemini 等,含 **"含时间条件 (Time-based)"** 分组:夜间 5 折、周末 8 折两套现成规则。

保存校验:编译(语法)+ 冒烟(4 组 token 向量 × 2 组请求样例,结果必须有限且非负);任务模型额外把 `u()` 键对照插件 usage schema 校验。

## 4. 表达式语言速览

变量(单价以 $/1M 计,**未使用的子类别留在 p/c 里按基础价计费**):

| 变量 | 含义 |
|---|---|
| `p` | 输入 token(计价用,**自动排除**已单独计价的子类别) |
| `c` | 输出 token(同上) |
| `len` | **完整输入上下文长度**,只用于档位条件;不受自动排除影响 |
| `cr` / `cc` / `cc1h` | 缓存读取 / 缓存写入(5m)/ 缓存写入(1h,Claude) |
| `img` / `img_o` | 图像输入 / 输出 token |
| `ai` / `ao` | 音频输入 / 输出 token |

自动排除的例子(GPT 语义,prompt_tokens=1000 含 200 缓存):写 `p*3+c*15` 时 `p=1000`;写 `...+cr*0.3` 时 `p=800`,缓存单独算。Claude 语义(`input_tokens` 本就是纯文本)不做减法,系统按上游返回格式自动判断(`service/tiered_settle.go: BuildTieredTokenParams`)。

函数:`tier(name, value)`(必须包住费用,记录命中档位)、`param(路径)`(gjson 读请求 body)、`header(键)`、`has(源, 子串)`、`max/min/abs/ceil/floor`,以及分时用的 `hour/minute/weekday/month/day(时区)`。

**阶梯条件必须用 `len` 而不是 `p`**——缓存命中会把 `p` 扣小,导致高上下文请求掉进低档。

版本前缀 `v1:`(可省略);版本控制编译环境、token 归一化与换算公式。

## 5. 分时计费(重点)

分时 = 表达式里的**时间条件**,服务端按**运行时刻**求值。两种写法:

### 写法 A:时间条件直接写进表达式(波峰/波谷两套价)

```
hour("Asia/Shanghai") >= 9 && hour("Asia/Shanghai") < 21
  ? tier("peak", p * 6 + c * 30)
  : tier("offpeak", p * 3 + c * 15)
```

只写"高峰"条件,其余时段自动落 `else`。缺点:这种条件不是 `p/c/len` 形态,可视化编辑器解析不了、定价页会显示"特殊计费表达式"。

### 写法 B:请求规则 × 倍率(推荐,也是 UI 默认路径)

基础价保持单档,分时作为**条件乘数**叠加(最终价 = 基础价 × 各命中规则倍率之积):

```
(tier("base", p * 3 + c * 15)) * (hour("Asia/Shanghai") >= 21 || hour("Asia/Shanghai") < 6 ? 0.5 : 1)
```

编辑器里就是可视化模式下方的 **"请求规则计费"** 区域(`tiered-pricing-editor.tsx:1831`):规则组 → 条件(来源选 **时间**)→ 时间函数 + 时区 + 匹配方式 + 倍率。预设模板"含时间条件"分组即夜间 5 折 / 周末 8 折。

**时间条件要素**:

| 要素 | 取值 |
|---|---|
| 时间函数 | 小时(0-23)/ 分钟(0-59)/ 星期(0=周日…6=周六)/ 月(1-12)/ 日(1-31) |
| 时区 | IANA 名称(下拉内置 11 个常用时区:`Asia/Shanghai`、`UTC`、`America/New_York`、`Europe/London`、`Asia/Tokyo`…)。**不能写 `+8` 这类偏移量** |
| 匹配方式 | 等于 `==` / 大于等于 `>=` / 小于 `<` / 时间范围(UI 显示"时间范围") |
| 时间范围语义 | 开始 ≤ 结束:当日区间 `fn >= s && fn < e`(左闭右开);开始 > 结束:跨零点 `fn >= s \|\| fn < e`(如 21-6);**开始 = 结束恒不命中** |

越界/小数的边界(如 hour 写 24、9.5)在编辑器生成表达式时被丢弃——单条件的规则组整条不会写进表达式;域校验见 `billing-expr.ts: TIME_FUNC_RANGES`。

**求值时点(容易踩)**:时区解析走 Go `time.LoadLocation`(`pkg/billingexpr/run.go: timeInZone`),空串或非法时区**静默回退 UTC**;宿主必须带 tzdata(Docker 镜像已装 `tzdata`;裸机/自定义镜像缺失时所有分时规则会按 UTC 算)。预扣和结算**各自取当时的服务器时间**再跑一遍表达式——跨时段边界的请求可能"按旧价预扣、按新价结算",差额在结算里多退少补。

**命中追踪**:编译时引擎给"`探针条件 ? 数字字面量 : 1`"形态的三目插埋点(不改变数值结果),结算时把实际命中的规则写进消费日志 `request_rules`;短路跳过的规则保持"未命中"。写法 A(条件在基础表达式里)不进 `request_rules`,但 `tier()` 仍会记录 `matched_tier`。

## 6. 请求规则(全集)

规则组 = 若干条件 `&&` 连接,条件来源三种:

| 来源 | 表达式形态 | 支持的匹配 |
|---|---|---|
| Body param | `param("service_tier")`(gjson 路径) | 等于 / 包含 / 存在 / > / ≥ / < / ≤ |
| Header | `header("anthropic-beta")` | 等于 / 包含 / 存在 |
| 时间 | `hour("Asia/Shanghai")` 等 | 等于 / ≥ / < / 时间范围 |

多个规则组**独立相乘**(如 GPT `service_tier=priority ×2`、`=flex ×0.5`);存储形态就是若干 `(条件 ? 倍率 : 1)` 相乘,没有额外分隔语法。追踪埋点仅识别 `条件 ? 数字字面量 : 1`(fallback 必须为 1,条件至少引用一个 `param/header/hour/minute/weekday/month/day`);其他三目照常求值但不进日志。`_trace`/`_trace_int` 是保留标识符,表达式里不能使用(`pkg/billingexpr/compile.go:34`)。

请求输入在**预扣时冻结**(`BillingSnapshot` + `BillingRequestInput`),结算复用同一份 headers/body,防止中途篡改。

## 7. 预扣 → 结算

1. **预扣**(`relay/helper/price.go: modelPriceHelperTiered`):用本地估算的 prompt tokens + `max_tokens`(**缺省按 8192**,`defaultPreConsumeMaxTokens`——2026-10-08 起该常量也供按量路径共用,原 `defaultTieredPreConsumeMaxTokens`)跑一遍表达式折成 quota 冻结;免费分组(GroupRatio=0)预扣为 0,关闭"免费模型预消耗"开关时更会跳过预扣会话。此时缓存等子类别按 0 估——即"全按未命中缓存"预扣。
2. **自动分组重试**:换到更贵分组时先把预扣提到该分组估价,换到更便宜分组只在结算后退差额(`service/tiered_settle.go: PrepareTieredBillingForSelectedGroup`)。
3. **结算**(`service/text_quota.go` → `TryTieredSettle`):按上游实际 usage 归一化 token → 同一快照重跑表达式 → `QuotaRoundChecked`(四舍五入离零,饱和钳制会记入日志 `admin_info.quota_saturation`)→ 多退少补。表达式跑挂时**保留预扣值**,不退款不回滚。
4. **任务模型**(视频/图像任务):提交时把请求侧 usage 事实与表达式一起冻结进快照;完成回调时用实测值逐键覆盖再重算(`relay/relay_task.go`、`service/task_polling.go`),中途换档会产生结算差额。

## 8. 日志与展示

- 消费日志 `other` 字段(`service/log_info_generate.go: InjectTieredBillingInfo`):`billing_mode="tiered_expr"`、`expr_b64`(表达式原文 base64)、`matched_tier`、`request_rules[]`(`{cond, multiplier, matched}`)。
- **日志详情**对话框渲染"条件乘数"列表:每条规则显示友好描述 + `倍率x`,实际命中的绿色高亮 + "已命中"(`dynamic-pricing-breakdown.tsx:566`;数据优先用日志里的 `request_rules`,没有才回落解析表达式)。
- **定价页模型详情**共用同一渲染:解析 `tier(...)` 出价表,时间规则显示为如"小时 21:00~6:00 (Asia/Shanghai) ×0.5"。
- 解析不了的表达式(如写法 A 的时间档)显示"特殊计费表达式"+原始字符串。
- 编辑器内置**成本估算器**:输入 prompt/completion token 数预览基础价成本(不含请求规则倍率)。

## 9. 示例:夜间 5 折的一笔账

模型表达式(写法 B):

```
(tier("base", p * 3 + c * 15 + cr * 0.3)) * (hour("Asia/Shanghai") >= 21 || hour("Asia/Shanghai") < 6 ? 0.5 : 1)
```

北京时间 22:30 的一笔请求:输入 10,000 tokens(其中 2,000 命中缓存),输出 2,000 tokens;GroupRatio = 1。GPT 语义下自动排除 `cr` 后 `p=8,000`。

| 部分 | 计算 | USD |
|---|---|---|
| 普通输入 | 8,000 × $3 / 1M | 0.024 |
| 缓存输入 | 2,000 × $0.3 / 1M | 0.0006 |
| 输出 | 2,000 × $15 / 1M | 0.03 |
| 基础表达式输出 | | **0.0546** |
| × 夜间倍率 0.5 | | **0.0273** |
| **quota** | 0.0273 × 500,000 | **13,650 quota** |

同请求在白天是 27,300 quota;这笔夜间请求省了 50%。

## 10. 常见坑与边界

- **`|||when(...)` 记法不存在**。`pkg/billingexpr/expr.md` 与早前导入文档里写的"`|||` 分隔请求规则"是历史记法;当前实现直接把它编译失败(`unexpected token "|"`)。**唯一有效的存储形态**是 `(基础表达式) * (条件 ? 倍率 : 1)`(可多个相乘),编辑器生成和解析的都是这一形态——写表达式时以编辑器和本文为准。
- 阶梯条件用 `len`,不用 `p`(缓存命中会误判档位)。
- 时间范围"开始 = 结束"恒不命中;越界边界会被编辑器丢弃,但如果你直接写 raw 表达式,后端只做编译与冒烟测试(不校验语义),`hour >= 25` 这种恒假条件会静默存进去。
- 时区必须 IANA 名;`"+8"`、拼错的时区会**静默回退 UTC**,分时折扣会按 UTC 时段生效。
- 预扣与结算各自取样"当前时间";跨时段边界的请求按结算时刻的价格重算并差额结算。
- 表达式编译结果按 SHA-256 缓存(上限 256 条,超限整体清空);改表达式 = 换 hash = 重新编译,即时生效。
- 激活后倍率表全部失效;REST 配置必须同时 PUT `billing_setting.billing_mode` 和 `billing_setting.billing_expr` 两个 key。
- 表达式里的数字是美元价,不是 quota,也不是倍率;任务表达式的输出是"整单美元"而不是 $/1M。

## 11. 文件地图

| 层 | 文件 |
|---|---|
| 表达式引擎 | `pkg/billingexpr/compile.go`(编译/缓存/埋点)、`run.go`(运行环境/时间函数)、`settle.go`(换算)、`types.go`(快照/追踪结构) |
| 存储与校验 | `setting/billing_setting/tiered_billing.go`、`controller/option.go:206` |
| 预扣 | `relay/helper/price.go:327`、`relay/helper/billing_expr_request.go` |
| 结算 | `service/tiered_settle.go`、`service/text_quota.go:416`、`service/quota.go:160,296`;任务:`relay/relay_task.go`、`service/task_polling.go` |
| 日志注入 | `service/log_info_generate.go:333` |
| 编辑器 | `web/src/features/system-settings/models/tiered-pricing-editor.tsx`、`model-pricing-sheet.tsx`、`task-usage-pricing-editor.tsx` |
| 表达式构造/解析(前端) | `web/src/features/pricing/lib/billing-expr.ts`、`tier-expr.ts` |
| 展示 | `web/src/features/pricing/components/dynamic-pricing-breakdown.tsx`、`web/src/features/usage-logs/components/dialogs/details-dialog.tsx` |
| 设计文档 | `pkg/billingexpr/expr.md`;导入侧:`docs/import-models/model-pricing.md` |
