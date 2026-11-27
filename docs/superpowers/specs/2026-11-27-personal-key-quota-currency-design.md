# 个人 API Key 额度按货币录入 — 设计

日期：2026-11-27
状态：已确认（方案 + 设计已获批准）

## 背景与意图

个人 API Key 新建/编辑抽屉的「原生额度」要求用户填写内部 quota 单位（500,000 quota = 1 USD），
用户难以换算。组织 Key、用户额度、兑换码、订阅等表单早已改为按站点展示货币录入；
本次把个人 Key 补齐，与既有模式一致。

## 现状

- `web/src/features/keys/lib/api-key-form.ts`：`remain_quota` 为原始 quota，默认 5,000,000（=$10）。
- `web/src/features/keys/components/api-keys-mutate-drawer.tsx`：输入框 label「原生额度」，step=1，无货币符号。
- 已有换算 helper（`web/src/lib/format.ts`）：`quotaUnitsToDollars` / `parseQuotaFromDollars` /
  `quotaUnitsToEditableAmount` / `getEditableQuotaStep`，跟随系统 `quota_display_type`
  （USD / CNY / CUSTOM / TOKENS）。cn 站 = CNY（汇率 7.3），com 站 = CUSTOM（HKD）。
- 先例：`organization-token-form.ts` 的组织 Key 表单；兑换码抽屉的 dirty 判断
  （未改额度字段时提交原 quota）。

## 目标

- 表单额度输入/回显跟随站点当前显示货币（¥ / HK$ / $）；TOKENS 模式保持点数。
- 后端、API、数据库不改，仍存原生 quota。
- 现有 Key 零迁移；编辑时未触碰额度字段则原值原样提交。

## 非目标

- 不改组织 Key 表单（已是货币输入）。
- 不改管理端其它仍用原始 quota 的输入（如 Quota Settings）。
- 不改计费/扣费逻辑。

## 方案

表单字段改为 `remain_quota_dollars`（沿用项目现有命名）：

- 默认值：当前显示货币 **10**（`parseQuotaFromDollars(10)`；cn ≈ 684,932 quota，USD = 5,000,000）。
  依赖运行时站点配置，必须在 `getApiKeyFormDefaultValues()` 内计算，不能放模块级常量。
- 回显：`quotaUnitsToEditableAmount(apiKey.remain_quota)`，取整到站点货币精度。
- 提交：`unlimited_quota ? 0 : parseQuotaFromDollars(value)`。
- 编辑保护：`form.getFieldState('remain_quota_dollars').isDirty === false` 时，
  payload 使用最新拉取的 `apiKeyData.data.remain_quota`，避免只改名称导致额度按分漂移。
- UI：沿用兑换码抽屉的既有货币输入模式（`getCurrencyDisplay` / `getCurrencyLabel`）：
  label 用 `Quota ({{currency}})`，placeholder / description 指明货币；
  step 用 `getEditableQuotaStep()`（货币模式 0.0001，TOKENS 模式 1）。
- 校验：非负；`unlimited_quota` 为 true 时跳过额度校验。

## 考虑过的其它方案

- 全精度回显 `quotaUnitsToDollars`（组织 Key 表单现状）：历史 Key 会显示长小数，弃。
- 后端改存/返回货币金额：改动面大且破坏内部计费单位，弃。

## 测试

- 往返一致：显示金额 10 → quota → 回显 10（覆盖 CNY 7.3 与 USD 两种配置）。
- 编辑未改额度：payload 的 `remain_quota` 等于原值（含非整分的历史数据）。
- `unlimited_quota` 为 true：不换算，payload 为 0。
- 默认值随显示货币：USD → 5,000,000；CNY(7.3) → 684,932。

## 影响面

仅 `web/` 前端。验证：受影响 vitest 用例、`bun run typecheck`、涉及文件 lint。
