# 2026-09-08 模型定价跟随站点展示货币(视频表 USD/s 化)

实现分支: `feat/model-pricing-currency`(spec: `specs/2026-09-08-model-pricing-currency-design.md`,plan 见 `.superpowers/sdd/2026-09-08-model-pricing-currency/`)

## 变更摘要

系统设置→模型定价编辑器/列表与用户侧定价页的货币计价输入/展示(per-token lanes、per-request 价、分时/阶梯与工具按量编辑器、列表摘要、视频价格表)随站点所选展示货币(USD/CNY/CUSTOM)换算显示与录入;存储与计费始终 USD。视频价格表从固定 ¥/s 语义统一为 USD/s,存量配置启动时一次性 ÷7.3 迁移。

## 关键实现点

- **换算边界**: 只在 UI 字符串边界 ×rate(显示)/÷rate(录入);ratio 家族保持无量纲倍率(lane÷prompt 推导,rate 约去),表达式文本永远美元
- **视频表 USD/s**: `VideoPriceModelPrice` 去掉 `/USD2RMB`;option `VideoPrice` 启动迁移(`migrateVideoPriceUsdToUSD`,标记 `VideoPriceUsdMigrated`,行锁 + OnConflict 两阶段写,多主并发安全)
- **公开定价缓存版本** bump;三库(SQLite/MySQL 8.0.46/PostgreSQL 15.18)迁移矩阵通过(全新/存量/幂等两遍 + 4 并发迁移回归)

## 验证

- 后端: `go build ./...` + `go test ./setting/... ./model/ ./relay/helper/...` 全绿
- 前端: typecheck + 全量 vitest(551)全绿;改动文件 oxlint 干净
- 任务级: 12 任务全部经独立 subagent 审查通过(Task 2/5 各一轮 fix)

## 备注 / 后续

- `features/models/components/drawers/model-mutate-drawer.tsx`(约 1012-1188 行)是另一个可达的模型定价编辑入口(per-request "Fixed price (USD)" + `Cost in USD per request…` key + `$/1M` 输入 + `Calculated price: $…` 预览),编辑同一批 ModelPrice/ModelRatio map,仍写死 USD——已登记为 follow-up:复用 getBillingCurrency/formatBillingCurrencyFromUSD 边界做换算(ratio 模式不换算),需独立任务级评审后再实施
- deferred minors 清单见 SDD ledger(final review 已裁夺)

## CodeRabbit 评审处理(2026-09-09,PR #74)

- **响应式货币源**: 新增 `useBillingCurrency()`(currency.ts,与 `getBillingCurrency` 同归一化出口);`dynamic-pricing-breakdown` / `video-price-table` 改用响应式订阅,挂载中展示货币变化即刷新符号与价格(补行为回归测试 ×2)
- **draft 汇率 rebase(数据正确性)**: `pricing-format.rebaseDisplayPriceDraft` 纯函数;model-pricing-sheet 的 price/promptPrice/lanePrices 与 video-price-editor 草稿在汇率变化时 rebase(保留底层 USD 意图,空/未完成录入原样保留),杜绝跨汇率保存写错价(补纯函数 + 编辑器行为测试)
- 视频价 `/s` 走 `t('s')`;task-usage 价格输入框补可访问名称(aria-label),测试按 accessible name 查询;zh-TW 6 条新增定价文案译繁中并同步 untranslated 报告;nitpick 全清(测试 helper 显式类型、去掉 DOM 顺序断言)
- 验证: typecheck + 全量 vitest(559)全绿;改动文件 oxlint 干净
- model-mutate-drawer 维持 follow-up(见上),CodeRabbit 线程已回复说明
