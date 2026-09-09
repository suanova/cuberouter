# 模型定价界面跟随站点展示货币(视频价格表 USD/s 化)— 设计规格

日期: 2026-09-08
状态: 已批准(2026-09-08 对话确认,范围: 三块边界输入统一处理 + 视频价格表统一为 USD/s)

## 1. 背景与目标

现状: 站点已有展示货币设置(`quota_display_type`: USD / CNY / TOKENS / CUSTOM,`general_setting.go`),余额、充值、消费记录、用户侧 token 价格预估均已跟随(前端 `formatCurrencyFromUSD`)。但**管理端模型定价界面整体写死美元**(`$`、`$/1M`、`USD price per 1M tokens`、`Cost in USD per request`),站点选 CNY 时管理员仍填美元,与其他界面不一致。视频价格表更是「人民币岛」:库内 ¥/s、编辑 ¥/s、用户定价页原样展示 ¥/s,只有计费预扣锚点处 `/7.3` 折成美元。

目标:
1. 管理端模型定价全部**以货币计价**的输入与展示(per-token lanes、per-request 价格、分时/阶梯表达式 lane、工具按量 lane、列表摘要、预览行)随站点所选展示货币换算显示与录入;
2. 视频价格表从固定 ¥/s 语义统一为 **USD/s** 语义(与其余定价一致:库内美元、界面随货币换算),含存量配置一次性迁移;
3. 存储与计费始终 USD,计费引擎语义除视频表锚点去掉固定 ÷7.3 外零改动。

需求来源(用户 2026-09-08 确认):「改成随着选的货币变」→ 管理端定价编辑器 → 展示+录入都换算(推荐)→ 表达式/Task/视频表三块统一 → 视频表也统一为 USD/s。

## 2. 不变量与硬边界

- **存储与计费永远 USD**;换算只发生在 UI 边界(显示 = USD × rate;录入 = 输入 ÷ rate 后走现有逻辑)。
- **汇率来源**: 沿用站点展示货币配置 —— USD→rate=1;CNY→`usdExchangeRate`(即 `Price`,默认 7.3);CUSTOM→`customCurrencyExchangeRate` + `customCurrencySymbol`;TOKENS 无货币意义,定价界面回落 USD(`$`、rate=1)。
- **表达式文本永远美元**: 后端解析的表达式字符串不做文本改写,只换算生成它的 lane 输入与界面预览/摘要;手工粘贴/查看原文的表达式在帮助文案中注明单位为美元。
- **不换算的字段**: `GroupRatio` 等纯倍率输入(乘数语义);`upstream-ratio-sync` 的官方美元对比列(工具属性,保持美元源数值)。

## 3. 现状关键点(实现依据)

- 编辑器数值现状: per-token 模式 ratio 1 ↔ 输入价 `$2/1M`(ratio×2,`ratioToBasePrice`);lane 价格 = ratio × 输入价(`deriveLanePrice`);保存时由 lane/输入价反推 ratio(`deriveLaneRatio`)。因此换算与现有推导可线性复合: **汇率只作用于字符串进出点**(加载 ×rate、提交/输入 ÷rate),ratio 推导、冲突检测(基于存储 ratio 的 USD 比较)全部不动。
- 视频价格表消费链(迁移语义核对):
  - `setting/ratio_setting/video_price.go` `VideoPriceModelPrice = VideoPriceAnchor(t) / USD2RMB` —— 唯一做 ¥→USD 折算处;
  - `relay/helper/video_price.go` `ComputeVideoPriceRatios` 只用**相对比值**(`normal[res]/anchor`、`off_peak/normal`),币种无关,不受语义翻转影响;
  - `relay/helper/price.go:197-199` 预扣锚点(USD per-call);
  - `relay/channel/task/jsplugin/adaptor.go:143` 仅存在性判断;
  - `model/pricing.go:423` 公开定价缓存原样暴露行值(`Pricing.VideoPrices`),用户侧据此展示;
  - 持久化: option key `VideoPrice`(`model/option.go:663`),更新后失效公开定价缓存。
- 旧设计文档 `2026-09-01-video-per-second-pricing-design.md` 原意是「anchor ¥/秒 ÷ **系统展示汇率**」,实现时写死 `USD2RMB=7.3`。本次 USD/s 化后,存储为显式美元,**计费锚点不再依赖汇率常量**(更稳定),显示换算走展示汇率 —— 语义上更贴合系统「库内美元」原则。

## 4. 前端换算基础设施(新增)

`web/src/lib/currency.ts` 现有 `formatCurrencyFromUSD`(仅格式化)。新增数值换算(建议同文件或相邻小模块,纯函数、可单测):

- `usdToDisplayValue(usd: number): number` —— USD × rate(TOKENS 时按 rate=1,不格式化);
- `displayToUsdValue(local: number): number` —— ÷ rate;
- `getPricingCurrency(): { symbol: string; rate: number; kind: 'currency' | 'custom' | 'tokens' | 'usd' }` —— 包装 `getCurrencyDisplay()`,编辑器据此取符号/汇率,TOKENS 回落 `$`/1;
- 显示数字经现有 `formatPricingNumber`/`snapFloatDrift` 归整(防 0.06849…×7.3 往返抖动);显示保留足够小数位(沿用现有 12 位内有效数字策略,输入解析不做主动取整,靠现有浮点归整)。

编辑器内部状态(表单值、lane state)**始终存 USD**;换算发生在:
1. 加载: `editData`/存量 ratio → 输入框显示值(×rate);
2. 输入: onChange 解析显示值 → USD(÷rate)存入状态;
3. 提交: 状态中已是 USD,现有 assemble/ratio 推导不变。

## 5. 管理端定价界面改造清单

`web/src/features/system-settings/models/`(编辑 `web/` 前需先读 `web/AGENTS.md`):

1. **`model-pricing-inputs.tsx`**: `PriceInput` 的 `$` addon 与 `$/1M` 后缀换成 `getPricingCurrency()` 符号与单位文案;`PriceLane` 帮助文案(`USD price per 1M tokens.` → 动态「按站点展示货币」语义,保留 i18n key 结构)。
2. **`model-pricing-sheet.tsx` / `model-pricing-core.ts`**: per-request 价格输入与 per-token 输入价/lane 的加载/输入/提交三处边界换算;预览行(`buildPreviewRows` 中 `$${promptPrice}` 等)改为货币格式;写死 `USD price per 1M input tokens.` / `Cost in USD per request` 等文案动态化。
3. **`model-pricing-snapshots.ts` / `model-ratio-table-columns.tsx`**: `getPriceSummary`/`getPriceDetail` 中 `$xx / request`、`$xx / 1M` 等按货币格式化(复用 `formatCurrencyFromUSD`;倍率单位如 `x1.25` 不动)。消费者不止列表页,改后统一。
4. **`tiered-pricing-editor.tsx`**: `PRICE_SUFFIX = '$/1M tokens'`、`Fixed price` 组及所有单价输入(含 `priceToUnitCost` 解析处)按边界换算;表达式文本仍美元;帮助文案注明「表达式文本以美元存储」。
5. **`task-usage-pricing-editor.tsx`**: 单价输入(`$ / unit`、token 字段 `$/1M`)、行摘要(`… × $x/unit`)、帮助文案(`Task usage prices are USD per declared unit…`)同上处理;`/1000000` 写入表达式的机制不变(状态已是 USD)。
6. **`video-price-editor.tsx`**: 行输入 addon `¥` 与标签 `(¥/s)` 换成动态货币符号与单位;数值走同一边界换算(此时存储语义已为 USD/s,见 §6);`video-price-drafts.ts` 归一化逻辑(非负/数值校验)不变。

## 6. 视频价格表 USD/s 化(后端)

### 6.1 语义与改动点

- `setting/ratio_setting/video_price.go`: `VideoPriceModelPrice` 去掉 `/USD2RMB`,返回 `VideoPriceAnchor(t)`(单位注释改为 USD/s);`USD2RMB` 常量仍供 model_ratio 默认值使用,不删除。
- `relay/helper/price.go:195-199` 注释更新(「锚点 ¥/秒 → USD per-call」→「锚点 USD/s 作为按次预扣锚」)。
- 其余消费点(相对系数、存在性判断)零改动。
- 公开定价缓存 `PricingVersion` 哈希(`model/pricing.go:446`)需 bump(数据语义变化);迁移后立即 `InvalidatePricingCache()`。

### 6.2 存量迁移(幂等、多节点并发安全、三库)

option 行 `VideoPrice`(JSON:`{model: {rows: [{resolution, normal_price, off_peak_price}]}}`)内的每个 `normal_price` / `off_peak_price` ÷7.3 一次。

- 位置: 启动期 options 加载**之前**(对齐既有先例: c362ec9 类迁移,`model/main.go` `migrateDB`/`migrateDBFast` 两条路径都调用同一迁移函数)。
- 算法(事务内):
  1. `lockForUpdate` 锁定 `VideoPrice` option 行(多节点并发时后者等待;SQLite 无锁语句,由单写者+标记兜底);
  2. 读迁移标记 option(新 key,如 `VideoPriceUsdMigrated`),已存在 → 跳过;
  3. 解析 JSON,每个价格 ÷7.3(普通 float 除法,不涉及配额换算辅助函数——非 quota 路径),写回 option 行,写入标记;
  4. 提交。
- 全新库: 无 `VideoPrice` 行(或默认空) → 直接置标记,无实际改写。
- 迁移后内存态: 迁移在 options 加载前完成,`UpdateVideoPriceByJSONString` 读到即 USD 值,无需二次处理。
- 此迁移不涉及表结构/索引/约束,仅 option 表行内 JSON 值;按 AGENTS.md 数据库规矩仍需真实 SQLite / MySQL / PostgreSQL 各跑: 全新库、存量库升级、幂等两次启动。

### 6.3 迁移等价性说明(对账依据)

迁移前计费锚 = 存储 ¥/s ÷ 7.3(固定);迁移后 = 存储(¥/7.3)= USD/s,计费锚数值**不变**。管理员界面: 显示 = USD × 展示汇率(CNY 默认 7.3 时与迁移前填的 ¥/s 数字一致);此后若调整展示汇率,界面数值随汇率显示变化而**计费锚不变**(旧实现的计费锚随填写的 ¥ 值,汇率只影响展示——这是本次语义统一的收益点)。

## 7. 用户侧定价页(`web/src/features/pricing/`)

- 视频价格展示(`video-price-table.tsx`、`model-card`、`pricing-columns`、`dynamic-pricing-breakdown` 等): 当前「verbatim ¥/s + ¥ 符号」改为按 `formatCurrencyFromUSD` 随货币显示(值已为 USD/s)。统一做法: 各调用方把 USD/s 数值先换算/格式化后传入展示组件;`lib/video-price.ts` `formatVideoPrice` 保留纯数字字符串化(输入已是换算后数值),仅更新注释与单测——保证与定价页其他金额走同一格式管线。
- 错峰窗口展示不变(纯时间)。

## 8. i18n

写死的 `USD`/`¥` 文案全部改为动态单位/货币符号 + 新增/调整 `web/src/i18n/locales/en.json` 与 `zh.json` key(英文原串为 key,按 `web/AGENTS.md`),`bun run i18n:sync` 同步。后端如无面向用户文案则不动(计费报错文案与货币无关)。

## 9. 测试与验证

后端(表驱动,`testify`):
- 更新 `setting/ratio_setting/video_price_test.go`、`relay/helper/video_price_test.go`: `VideoPriceModelPrice` 新语义(返回 anchor 本身);相对系数用例不变。
- 新增迁移测试: 存量 JSON ÷7.3 结果、标记幂等(二次执行无变化)、空/缺省值。
- 三库矩阵(SQLite/MySQL/PostgreSQL 真实实例): 全新库 + 存量库升级 + 启动迁移跑两遍,记录引擎版本与命令(AGENTS.md 强制)。
- relaykit 模块不受影响(无 relaykit/ 依赖变更),无需 `GOWORK=off` 专项构建,但 root 构建须过。

前端(`web/AGENTS.md` 约定):
- 新增 `pricing-currency`/`currency` 换算单测: USD↔CNY 往返、rate=1(USD 直通)、CUSTOM、TOKENS 回落、浮点归整。
- 扩展既有 `model-pricing-core`/`__tests__` 与定价页测试: 加载换算、输入反向换算、提交仍为 USD 载荷、TOKENS 回落、视频表新语义展示。
- i18n key 完整性。

手动验证: 编辑器在 USD/CNY/CUSTOM 三态下打开/编辑/保存同一模型,载荷与后端存储值不变(除视频表一次迁移)。

## 10. 风险与缓解

| 风险 | 缓解 |
|---|---|
| 迁移是计费语义变更,出错价格变 7.3 倍 | 行锁+标记幂等;三库全新/存量/双跑验证;迁移与缓存失效同仓提交 |
| 多节点同时启动各跑一次迁移 | `lockForUpdate` 行锁 + 完成后标记;标记检查在锁内 |
| 录入反向换算浮点/舍入漂移(如 0.5÷7.3×7.3≠0.5) | 显示用 `snapFloatDrift` 归整;状态存 USD,显示派生 |
| 汇率调整后历史配置显示价变化 | 已接受:存 USD、显示时换算,与全站其他金额行为一致 |
| 表达式 lane 换算后与表达式文本(美元)不一致观感 | 帮助文案注明表达式以美元存储;只换 lane 输入与预览 |
| 现网已配视频价格表 | 迁移后数值 ÷7.3;CNY@7.3 站点界面数字不变(§6.3) |

## 11. 文件清单

后端:
- `setting/ratio_setting/video_price.go`(去 ÷7.3 + 注释)
- `relay/helper/price.go`(注释)
- `model/main.go`(迁移函数挂载,`migrateDB`/`migrateDBFast`)
- 新迁移函数文件(建议 `model/migration_video_price_usd.go` 或并入现有模式)
- `model/pricing.go`(`PricingVersion` bump)
- 测试: `setting/ratio_setting/video_price_test.go`、`relay/helper/video_price_test.go`、迁移测试

前端(`web/src/`):
- `lib/currency.ts`(或相邻新文件): 数值换算纯函数
- `features/system-settings/models/`: `model-pricing-inputs.tsx`、`model-pricing-sheet.tsx`、`model-pricing-core.ts`、`model-pricing-snapshots.ts`、`model-ratio-table-columns.tsx`、`tiered-pricing-editor.tsx`、`task-usage-pricing-editor.tsx`、`video-price-editor.tsx`、`pricing-format.ts`(如需要)
- `features/pricing/`: `lib/video-price.ts`、`components/video-price-table.tsx`、`model-card`、`pricing-columns`、`dynamic-pricing-breakdown` 等视频价格展示处
- `i18n/locales/en.json`、`zh.json`(+sync)
- 各相关 `__tests__`

## 12. 范围外(本轮不做)

- 表达式**文本**解析/改写为本地货币
- `upstream-ratio-sync` 界面(官方美元对照工具)
- `GroupRatio`/group 倍率等纯乘数输入
- 计费引擎其他路径、余额/充值(已跟随)
- 历史日志/消费快照中的美元数值回显
- 其他 task 平台接入视频按秒(沿用现状)
