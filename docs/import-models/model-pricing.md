# Model Pricing

> 完整设计文档 `pkg/billingexpr/expr.md`。
> 本文是接入新模型时的**选型 + 配置速查**,按计费模式组织。

## 0. 计费模式总览(一模型一档)

cuberouter 一个模型同一时刻只走一种计费模式,判定顺序固定(`relay/helper/price.go` → `ModelPriceHelper` / `relay/relay_task.go`):

| 优先级 | 模式 | 触发条件 | 配置载体 |
|---|---|---|---|
| 1 | 表达式 tiered_expr | `billing_setting.billing_mode` 里模型值为 `tiered_expr` | `billing_setting.billing_expr` |
| 2 | 图片按张价目表 | 模型在 `ImagePrice` 表里 | option `ImagePrice` |
| 3 | 视频按秒价目表(仅 Task 渠道) | 模型在 `VideoPrice` 表里 | option `VideoPrice` |
| 4 | 按次 ModelPrice | 模型在 `ModelPrice` 表里(或内置 `defaultModelPrice`,如 dall-e-3) | option `ModelPrice` |
| 5 | 按 token 倍率(默认) | 模型在 `ModelRatio` 表里 | option `ModelRatio` + `CompletionRatio`/`CacheRatio`/`CreateCacheRatio`/`ImageRatio`/`AudioRatio` |
| — | 未配置 | 两张表都没有 | 请求直接报错「模型价格未配置」(除非开自用模式或用户开了 `AcceptUnsetRatioModel`) |

- 文本中继(chat/messages/responses/embeddings)走 `ModelPriceHelper`; Task/视频/MJ (Midjourney) 走 `ModelPriceHelperPerCall`(在 4/5 之外还看视频表)。
- 所有模式之上乘 **分组系数 GroupRatio**(用户组→模型组的特殊系数 `GroupGroupRatio` 优先于普通组系数)。
- 价格体系是**计费身份**(BillingModelName)查表,与渠道路由解耦;默认等于客户端请求的模型名。
- 系数表全部存**美元**;定价页显示时按站点展示货币(`quota_display_type`,CNY 默认汇率 7.3)换算,不影响扣费。

## 1. 按 token 固定倍率(最常用)

计费公式(`service/text_quota.go`,quota 单位:`$1 = 500,000 quota`):

```
费用 = [ (prompt_tokens - 缓存/图像/音频等单独计价部分)
       + 缓存读取 × CacheRatio
       + 缓存写入 × CreateCacheRatio(5m;1h = ×1.6 由 6/3.75 推导)
       + 图像 × ImageRatio + 音频输入另行计价
       + 输出 × CompletionRatio ] × ModelRatio × GroupRatio
```

| 配置项 | option key | 未配置时的兜底 |
|---|---|---|
| 模型倍率 | `ModelRatio` | 无兜底 → 报错(**倍率 1 = $2/1M token** 的祖传换算) |
| 输出倍率 | `CompletionRatio` | 硬编码表(gpt-4o 4、gpt-5 8、claude-3/4/5 系 5、gemini 4…未知模型 1) |
| 缓存读取 | `CacheRatio` | **1**(不打折,缓存白送的时候设 0.x) |
| 缓存写入 | `CreateCacheRatio` | **1.25**(仅 Claude 语义响应才产生该项) |
| 图像 token | `ImageRatio` | 1 |
| 音频 | `AudioRatio` / `AudioCompletionRatio` | 1(gemini 原生音频模型另有 $/1M 常量) |

换算速查:**输入价($/1M) = ModelRatio × 2**;输出价 = ModelRatio × CompletionRatio × 2。例:kimi-k3 ModelRatio=1.3699 → 输入 $2.74/1M。

模型名匹配为**精确匹配**(`RWMap.Get`),无通配符;仅 gemini-2.5*-thinking-* 与 gpt-*-gizmo-* 有代码级名称归一(`FormatMatchingModelName`)。

配置方式:后台「系统设置 → 模型价格 → 模型倍率」各表单,或 REST `PUT /api/option/` body `{"key":"ModelRatio","value":"{\"model\":0.547}"}`(整体替换该表,先 GET 再合并写回)。

## 2. 按次 ModelPrice

模型配了 `ModelPrice`(USD/次)就整个请求一口价,忽略 token。适合 MJ、部分 task、suno 等。预扣与结算同一价 × QuotaPerUnit × GroupRatio。

## 3. 表达式 tiered_expr(分时/阶梯/缓存写入 1h)

详细语法见本文下半部分与 `pkg/billingexpr/expr.md`。核心:

- 激活:`billing_setting.billing_mode` 里 `{"模型名":"tiered_expr"}`;表达式存 `billing_setting.billing_expr`。**REST 配置时这两个 key 都要 PUT**。
- 系数是**真实 $/1M token 价**(不经 ratio ×2 换算)。
- 扣费 = 表达式输出 / 1,000,000 × QuotaPerUnit × GroupRatio。
- 变量 `p`/`c` 自动排除已单独计价的子类(仅 GPT 语义;Claude 语义 input_tokens 本就纯文本);**阶梯条件用 `len`**(完整输入长度),不用 `p`。
- 该模式没有"部分配置"——一旦激活,ModelRatio 等表对该模型全部失效。

### 模板一:平峰单档(无时间条件)

```
tier("默认", p * <输入价> + c * <输出价> + cr * <缓存读取价> + cc * <写入5m价> + cc1h * <写入1h价>)
```

### 模板二:波峰波谷(时间条件)

```
(高峰条件)
  ? tier("peak", p * <高峰输入价> + c * <高峰输出价> + cr * <高峰缓存价> + cc * <...> + cc1h * <...>)
  : tier("offpeak", p * <空闲输入价> + c * <空闲输出价> + cr * <空闲缓存价> + cc * <...> + cc1h * <...>)
```

只写"高峰"条件作为真分支,其余(周末、夜间、午间)自动落入 `else`——空闲时段不需要写条件。

### 模板三:按输入长度分档(阶梯)

用 `len`(完整输入上下文长度)做条件,链式三目从短到长排,最后一段无条件兜底:

```
len <= 32000 ? tier("short", p * 1.2 + c * 6 + cr * 0.24 + cc * 1.5)
: len <= 128000 ? tier("mid", p * 2.4 + c * 12 + cr * 0.48 + cc * 3)
: tier("long", p * 3 + c * 15 + cr * 0.6 + cc * 3.75)
```

条件也可组合输出长度(`c`),如 doubao seed 的"短上下文+短输出折价":

```
len <= 32000 && c <= 200 ? tier("discount", ...) : len <= 32000 ? tier("short", ...) : tier("long", ...)
```

注意:分档条件**必须用 `len` 而非 `p`**——`p` 会因缓存/图像单独计价被扣除,缓存命中多的请求会被误判到低档;`len` 始终反映完整输入长度(GPT 语义 = prompt_tokens;Claude 语义 = 文本输入+缓存读取+缓存创建)。

### 模板四:多模态/图像模型

图像输入 `img`、图像输出 `img_o`、音频输入 `ai`、音频输出 `ao` 都是可选变量——**不用就留在 p/c 里按基础价计费,用了就单独计价并自动从 p/c 扣除**:

```
# 图像输入(gpt-image 类,输入图像 token 单独计价)
tier("base", p * 2 + c * 8 + img * 2.5)

# 图像输出(gemini-3-pro-image 类,产出图像 token 单独计价)
tier("base", p * 2 + c * 12 + img_o * 120)

# 音频输入(gemini 实时音频类)
tier("base", p * 0.3 + c * 2.5 + cr * 0.03 + ai * 1.0)

# 全多模态(qwen omni 类,图像+音频输入输出全拆)
tier("base", p * 0.43 + c * 3.06 + img * 0.78 + ai * 3.81 + ao * 15.11)
```

### 模板五:请求感知倍率(加价/折扣)

表达式后接 `|||` 与 `when(...)` 规则,同一表达式的价格上叠加系数:

```
tier("base", p * 5 + c * 25 + cr * 0.5 + cc * 6.25 + cc1h * 10)
|||when(header("anthropic-beta") has "fast-mode") * 6
```

可视化编辑器里就是"请求规则"组(header/param/时间条件 × 倍率);多个规则各自独立命中相乘,如 GPT priority ×2、flex ×0.5。时间型规则也可以走这条路做**整体折扣**(而非模板二那种两套价格的写法):`tier("base", p*3 + c*15)|||when(hour 在 21:00-次日06:00) * 0.5`。

### 编辑器内置预设速查

可视化编辑器的"Preset templates"下拉与以上模板一一对应,可直接套改:

| 分组 | 预设 | 形态 |
|---|---|---|
| Fixed price | Flat / Claude Opus 4.6 / GPT-5.4 | 模板一、模板三(双档) |
| Tiered | Claude Sonnet 4.5 / Qwen3 Max / GLM-4.5 Air / Doubao Seed 1.8 | 模板三(2~4 档,含 `len`+`c` 组合条件) |
| Multimodal | GPT Image 1 Mini / Gemini 2.5 Flash / Gemini 3 Pro Image / Qwen3 Omni Flash | 模板四(img/img_o/ai/ao) |
| Request rule | Claude Opus 4.6 Fast / GPT-5.4 Priority·Flex | 模板五(header/param 倍率) |
| Time-based | Night discount (50%) / Weekend discount (80%) | 模板五的时间型规则(整体折扣) |

### 变量

| 变量 | 含义 | 界面标签(zh) |
|---|---|---|
| `p` | 输入(缓存未命中/兜底) | 输入 |
| `c` | 输出 | 文本输出 |
| `len` | 完整输入上下文长度(仅用于阶梯条件,不受子类排除影响) | — |
| `cr` | 缓存命中(读取) | 缓存读取 |
| `cc` | 缓存写入(5m) | 缓存写入 (5m) |
| `cc1h` | 缓存写入(1h) | 缓存写入 (1h) |
| `img` / `ai` / `ao` / `img_o` | 图片输入 / 音频输入 / 音频输出 / 图片输出 | 对应中文标签 |
| `u("key")` | task 插件计费事实(见 expr.md Task Usage 节) | — |

### 条件构建规则

| 需求 | 写法 |
|---|---|
| 时区(北京时间) | `hour("Asia/Shanghai")` / `weekday("Asia/Shanghai")` — 必须 IANA 名,不要写 "UTC+8" 或数字偏移 |
| 半开区间 [9,12) | `hour(...) >= 9 && hour(...) < 12` |
| 工作日 周一至周五 | `weekday(...) >= 1 && weekday(...) <= 5`(weekday: 0=周日) |
| 跨零点区间 18:00-次日08:00 | `hour(...) >= 18 \|\| hour(...) < 8` |
| 多时段合并化简 | 条件内用 `\|\|` 连接;如 08:00-09:00 ∪ 18:00-次日08:00 = `hour < 9 \|\| hour >= 18` |
| 请求感知倍率 | 表达式后接 `\|\|\|` + `when(...) * N` 规则(`param()`/`header()`/时间探针),如 fast-mode 加价 |

### Task(视频生成等)的表达式差异

task 表达式输出**已是该请求的美元总价**,不再 /1M:`u("seconds") * 0.4` = $0.40/秒;token 型字段用 `u("tokens") * <$/1M> / 1000000` 的规范形状。保存时按插件 `meta.usageSchema` 校验 `u()` 键。

## 4. 视频按秒价目表(VideoPrice)

option `VideoPrice`:`{"模型名":{"rows":[{"resolution":"720p","normal_price":0.05,"off_peak_price":0.025}]}}`,USD/秒。

- 计费 = **锚点(表内最高 normal 价)× seconds 系数 × size 系数(该分辨率价/锚点)× time 系数(错峰价/正常价,仅错峰时段)**(`relay/helper/video_price.go`)。
- 分辨率档位:360p/480p/540p/720p/768p/1080p/2k/4k;`1920x1080` 这类尺寸写法会归一到 `1080p` 档;表内匹配不到 → 按锚点(最贵档)保守计费。
- 时长缺省 5 秒,一律按 `MaxTaskDurationSeconds`(3600)饱和。
- 错峰窗口 option `OffPeakWindow`(默认 22:00-08:00 Asia/Shanghai):`{"start_hour":22,"end_hour":8,"timezone":"Asia/Shanghai"}`。
- 表内价格全部 USD/s(2026-09-08 迁移过 ¥→USD);改表立即失效定价缓存。

## 5. 图片按张价目表(ImagePrice)

option `ImagePrice`:`{"模型名":{"rows":[{"tier":"standard","price":0.04}]}}`,USD/张,固定档位 `fast`/`standard`/`high`。

- 计费 = **锚点(最高价档)× quality 系数(该档价/锚点)× n(张数)**。
- 请求 quality 缺省或表外 → 按锚点计费;`n` 上限 `MaxImageN=128`。
- 表存在时旧 `ImageRatio`(dall-e 尺寸/画质系数)不再叠加。

## 6. 工具调用附加费

与计费模式无关的额外项(`service/text_quota.go` → `operation_setting.GetToolPriceForModel`):内置工具(web_search、file_search、google_search、image_generation 等)按 **$/1K 次调用** 计价,叠加在基础费用上。option key `tool_price_setting.prices`,键格式 `工具名` 或 `工具名:模型前缀*`(最长前缀匹配)。内置默认:web_search $10、file_search $2.5、google_search $14、image_generation $150、web_search_preview $25(限 gpt-4o*/4.1* 前缀)。

## 7. 预扣与结算

- **按 token**:预扣按 `max(实际输入, 500) + max_tokens` 估;结算按上游 usage 实算多退少补。
- **按次/表**:预扣=锚点价×QuotaPerUnit×GroupRatio,系数(seconds/size/n/quality)在结算时由实际请求/响应推导。
- **表达式**:预扣用估算 token(客户端没给 max_tokens 时按 8192 输出估算),结算时快照冻结表达式+请求态重跑。
- 余额不足在预扣阶段直接拒绝,不会事后欠费。计费为 0 的模型(free)默认跳过预扣(可配)。

## 关键事实

- **按 token(per-token)模式没有"缓存写入(1h)"价格项**(价格线只有:输入、输出、缓存读取、缓存写入 5m、图片、音频)。价格表含 1h 缓存写入价时必须用表达式模式。
- **界面显示本地化标签,不显示裸变量名**:消费日志详情弹窗显示"缓存写入 (5m) / 缓存写入 (1h)",定价页档位明细的 cc 显示"缓存写入"(无 5m 后缀);`p`/`c`/`cr`/`cc`/`cc1h` 只存在于表达式字符串与日志原始 JSON。
- **tier 名无 i18n**:档位标签 = `tier("xxx", ...)` 的字符串,界面与日志原样显示;要显示中文直接写中文(如 `tier("高峰", ...)`)。改档位名只影响之后的新请求,历史日志保持旧名。
- **日志核对**:消费日志 `other` 里有 `model_ratio`/`completion_ratio`/`cache_ratio`/`group_ratio`(按 token)、`billing_mode`+`matched_tier`+`request_rules`(表达式)、`use_price`(按次)——核对扣费以日志为准。

## 踩坑清单

1. **前端"Token estimator"预览报 `hour is not defined`** —— 浏览器本地预估器不支持时间函数。**不是表达式错误,直接保存**,后端编译校验完整支持 `hour()`。平峰表达式(无时间函数)预估器正常工作。
2. **跨零点区间写成 `&&` 恒为 false**(`h >= 18 && h < 8` 不可能成立)→ 谷价永不生效;同日内区间写成 `||` 恒为 true(`h >= 8 || h < 9` 覆盖全天)→ 全部按谷价。规则:**跨零点用 `||`,同日用 `&&`**。
3. **展示页数字 ×汇率** —— 定价页把系数当美元 × 系统 USD 汇率(默认 7.3)显示;实际扣费只按 `系数/1,000,000 × QuotaPerUnit × 分组比例`,不乘汇率。核对扣费看消费日志,别信展示页。
4. **保存后编辑器自动切到"表达式"原始模式**(含时间条件时)—— 可视化编辑器解析不了时间条件,属正常。
5. **系数单位** = 每百万 token,与按 token 计费模式填法一致(但表达式模式不经过 ×2 祖传换算,直接填 $/1M)。
6. **表达式激活后按 token 表全部失效** —— 同一模型不能"表达式 + ModelRatio 并存",billing_mode 是开关不是叠加。
7. **视频/图片表优先于 ModelPrice/ModelRatio** —— 配了表就别再配倍率,倍率对表模型不生效(锚点即价格)。
8. **REST 整表替换** —— `PUT /api/option/` 的 value 是**整张表**,漏合并旧条目会把别人配的价格抹掉;务必先 GET 再合并。

## Common Mistakes

- 用 `if/else` 或裸 `&&` 拼分支 —— expr-lang 只有 `? :` 三目
- 忘记 `tier()` 包裹 —— 日志没有档位记录
- 把空闲时段也写进条件 —— 冗余且容易写反
- 高峰条件写成"非高峰"取反 —— 直接写高峰命中,不要取反
- 用按 token 模式配 1h 缓存写入价 —— 该模式没有此价格项,必须用表达式模式
- 按倍率表换算表达式价格(×2)—— 表达式系数就是 $/1M 原价,倍率表那套 ×2 换算不适用
- 给视频/图片表模型又配 ModelPrice/ModelRatio —— 表优先,倍率无效

## Verification

- 保存即校验:后端 `CompileFromCache` 编译 + 样例向量冒烟测试,语法错会直接报
- 线上核对:发一条请求 → 消费日志 → `matched_tier` 与实际扣费(表达式);`model_ratio` 等系数(按 token);`use_price`(按次)
