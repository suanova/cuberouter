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
- zh-TW 新增 6 key 由 i18n:sync 以 en 回填,如需繁体文案需单独翻译
- deferred minors 清单见 SDD ledger(final review 已裁夺)
