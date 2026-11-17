# PaymentAsia 充值支付接入设计

**Date:** 2026-11-17
**Status:** Draft（待评审）
**范围:** 后端 `setting/` `service/` `controller/` `model/` `router/`、前端 `web/`
**相关:** `paymentasia/paymentasia.pdf`（Integration Guide v2.7.6）、`paymentasia/key.png`、`AGENTS.md`、`web/AGENTS.md`

## 1. 背景与目标

PaymentAsia 是香港收单网关，提供：

- Hosted Payment Page：商户把签名后的表单 POST 到 `https://payment.pa-sys.com/app/page/{MerchantToken}`，用户在托管页完成支付。
- 服务端 datafeed（notify）：支付结果 POST 回商户 `notify_url`。
- Payment Query / Refund / Reconciliation API（本期不做）。

本设计在 CubeRouter 的充值链路中新增一个 first-class 支付渠道 `paymentasia`：

1. 用户在钱包页选择 PaymentAsia 的某个通道（Alipay/AlipayHK/WeChat/CreditCard/FPS 等）并输入充值数量。
2. 后端创建 pending 充值订单，返回托管页 URL + 签名表单参数，前端表单 POST 跳转。
3. PaymentAsia datafeed 回调后验签、核对金额/币种，事务内幂等入账。
4. 运营可通过管理端配置凭据、沙箱、币种、单价、最低充值；通道通过现有「支付方式」列表启用/停用。

## 2. 非目标

本版（v1）明确不做：

1. 退款（Refund API）、对账（Reconciliation API）、主动查询（Payment Query API）。退款产品本身不支持；查询留到 phase 2，作为管理员补单/掉单排查的辅助。
2. 订阅支付（subscription plans）走 PaymentAsia。
3. 自动汇率/自动同步 PaymentAsia 结算价；`PaymentAsiaUnitPrice` 由管理员手工配置。
4. 新增数据库表或列；不修改 `TopUp` schema。
5. 在仓库保存任何 PaymentAsia 凭据；`paymentasia/` 目录保持不入库。
6. 默认启用 Octopus/PayMe/CUP；映射保留，是否启用由运营决定。
7. 收集用户地址/电话等风控字段；只发送用户邮箱（可选）与 IP。

## 3. 关键决策

| 决策 | 选择 | 理由 |
|---|---|---|
| 接入形态 | 新增 first-class provider `paymentasia` | 签名、回调字段、状态语义都与 Epay 不同，塞进 Epay 会污染现有路径 |
| 代码骨架 | 复刻 Epay 的「表单跳转 + datafeed 入账」 | 与现有支付链路、幂等/行锁/额度换算完全一致 |
| 通道暴露 | 每个通道一个 `PayMethods` type（`paymentasia_*`） | 复用现有支付方式编辑器与按 type 分发；用户少一次点击；通道是 PaymentAsia 固定枚举 |
| 币种 | 管理端单选 HKD / USD；默认 HKD | 文档：CreditCard 支持 HKD+USD，其它通道仅 HKD |
| 金额换算 | `展示数量 × PaymentAsiaUnitPrice × 分组倍率 × 折扣` | 与 Waffo/Stripe 的 unit-price 模式一致；不引入自动汇率 |
| 结算口径 | 与 Epay 相同：`Amount` 存展示单位、`Money` 存网关实付、CNY 模式快照 `Price` | 复用 `topupAmountToQuotaDecimal` / 现有补单逻辑 |
| 回调安全 | SHA-512 验签 + 金额/币种核对 + 订单行锁幂等 | 文档要求；复用 `RechargeEpay` 的事务模式 |
| 配置存储 | OptionMap（`model/option.go` 注册 + 管理端 UI） | 与 Stripe/Creem/Waffo 一致 |
| 沙箱 | 独立 sandbox token/secret + 开关 | 生产与沙箱凭据不同，不能混用 |

## 4. PaymentAsia 协议摘要

### 4.1 Hosted Payment Page

- Live：`POST https://payment.pa-sys.com/app/page/{MerchantToken}`
- Sandbox：`POST https://payment-sandbox.pa-sys.com/app/page/{MerchantToken}`
- Generic：`POST https://payment.pa-sys.com/app/page/generic/{MerchantToken}`（`network=UserDefine`）
- `Content-Type: application/x-www-form-urlencoded`

字段（见 Guide 第 5–6 页）：

| 字段 | 必填 | 说明 |
|---|---|---|
| `merchant_reference` | Y | 商户订单号，≤36，跨请求唯一 |
| `currency` | Y | ISO 4217；CreditCard 支持 HKD/USD，其它通道仅 HKD |
| `amount` | Y | 字符串，2 位小数（Octopus 1 位） |
| `sign` | Y | SHA-512，见 4.3 |
| `return_url` | Y | 支付完成后浏览器回跳地址 |
| `notify_url` | Y | 服务端 datafeed 地址 |
| `customer_ip` | Y | IPv4 格式 |
| `network` | Y | `Alipay` / `Wechat` / `CUP` / `CreditCard` / `Fps` / `Octopus` / `PayMe` / `UserDefine`，大小写敏感 |
| `wallet` | N | 仅 Alipay：`AlipayCN` / `AlipayHK` |
| `subject` | N | 订单名称 |
| `customer_email` 等 | N | 可选风控字段 |

### 4.2 Datafeed（服务端回调）

POST 到 `notify_url`，字段：

- `merchant_reference`、`request_reference`、`currency`、`amount`、`status`、`sign`
- `status`：`0` PENDING / `1` SUCCESS / `2` FAIL / `4` PROCESSING
- 商户必须验签，并核对 `amount`、`currency`、`merchant_reference` 与原订单一致。

### 4.3 签名

文档与 PHP 示例（第 19 页）：

```php
ksort($fields);
$sign = hash('SHA512', http_build_query($fields) . $secret);
```

即：

1. 去掉 `sign` 字段，其余字段按 key 升序排序；
2. `http_build_query`：`key=value` 用 `&` 连接，value 用 PHP `urlencode`（空格→`+`，仅保留 `A-Za-z0-9-_` 和 `.` 不转义，`~`、`*` 等都要百分号编码）；
3. 直接拼接 Merchant Secret（**没有** `&` 或其它分隔符）；
4. SHA-512 hex（小写）。

**文档笔误（已验证）**：文档第 19 页的 sample pre-sign string 与 sample hash 可以精确互相验证——前提是使用该 pre-sign string 中的 `subject=IphoneX`（大写 I、小写 p）。同一页 `$fields` 数组里写的是 `iPhoneX`，两处大小写不一致属于文档笔误。算法本身已由文档向量确认；单测可以直接用文档 pre-sign string + hash 作为编码/哈希阶段的锚点，另用自造字段覆盖特殊字符与大小写。沙箱首笔仍建议抓一次真实请求/回调签名做端到端确认。

### 4.4 查询 API（phase 2）

`POST https://gateway.pa-sys.com/{MerchantToken}/payment/query`，参数 `merchant_reference` + `sign`，返回 JSON 数组（Sale/Refund 记录，含 `status/currency/amount/created_time/completed_time`）。可用于管理员「同步订单状态」或掉单排查，v1 不实现。

## 5. 总体流程

```
用户                     前端                         后端                         PaymentAsia
 |  选择通道/金额          |                            |                              |
 |----------------------->| POST /paymentasia/amount   | 计算实付金额                  |
 |                        |--------------------------->|                              |
 |                        | POST /paymentasia/pay      | 创建 pending TopUp(Local)     |
 |                        |--------------------------->| 生成签名表单参数              |
 |                        |<---------------------------| { url, data }                 |
 |  表单 POST 跳转托管页   |                            |                              |
 |------------------------------------------------------------------------------------------>|
 |                        |                            |                              支付 |
 |<------- return_url 回跳钱包（展示历史，不以此入账） --------------------------------------|
 |                        |                            |<------- datafeed POST --------|
 |                        |                            | 验签/核对/行锁/幂等入账        |
```

## 6. 后端设计

### 6.1 配置项（`setting/payment_paymentasia.go`，新增）

| Option key | 类型 | 默认 | 说明 |
|---|---|---|---|
| `PaymentAsiaSandbox` | bool | false | 沙箱开关 |
| `PaymentAsiaMerchantToken` | string | "" | 生产 Merchant Token |
| `PaymentAsiaSecret` | string | "" | 生产 Merchant Secret |
| `PaymentAsiaSandboxMerchantToken` | string | "" | 沙箱 Merchant Token |
| `PaymentAsiaSandboxSecret` | string | "" | 沙箱 Merchant Secret |
| `PaymentAsiaCurrency` | string | `HKD` | 收款币种，HKD/USD |
| `PaymentAsiaUnitPrice` | float64 | 1.0 | 1 个展示单位折合的支付币种金额（CNY 站点+HKD 约 1.08；USD 站点+HKD 约 7.8） |
| `PaymentAsiaMinTopUp` | int | 1 | 全局最低充值数量（展示单位） |
| `PaymentAsiaNotifyUrl` | string | "" | 覆盖默认回调地址 |
| `PaymentAsiaReturnUrl` | string | "" | 覆盖默认回跳地址 |

启用判定（新增到 `controller/payment_webhook_availability.go`）：

```go
func isPaymentAsiaTopUpEnabled() bool   // 当前环境（沙箱/生产）的 token+secret 均非空
func isPaymentAsiaWebhookEnabled() bool // 同 isPaymentAsiaTopUpEnabled
```

### 6.2 常量与通道映射

`model/topup.go` 新增：

```go
PaymentProviderPaymentAsia = "paymentasia"
```

`service/paymentasia.go`（新增）定义通道映射：

| PayMethod type | network | wallet | 备注 |
|---|---|---|---|
| `paymentasia_alipay` | `Alipay` | — | 默认 Alipay |
| `paymentasia_alipaycn` | `Alipay` | `AlipayCN` | 仅大陆 Alipay |
| `paymentasia_alipayhk` | `Alipay` | `AlipayHK` | 推荐 |
| `paymentasia_wechat` | `Wechat` | — | 推荐 |
| `paymentasia_cup` | `CUP` | — | |
| `paymentasia_creditcard` | `CreditCard` | — | 唯一支持 USD 的通道；推荐 |
| `paymentasia_fps` | `Fps` | — | 推荐 |
| `paymentasia_octopus` | `Octopus` | — | 金额只能 1 位小数 |
| `paymentasia_payme` | `PayMe` | — | |
| `paymentasia_generic` | `UserDefine` | — | 通用收银台，使用 `/app/page/generic/` |

```go
type PaymentAsiaChannel struct {
    Network string
    Wallet  string // 仅 Alipay
}

func IsPaymentAsiaMethod(methodType string) bool
func PaymentAsiaChannelByMethodType(methodType string) (PaymentAsiaChannel, bool)
```

「推荐」仅写入文档/管理端提示，不自动写入 `PayMethods`；是否展示由运营在管理端添加。

### 6.3 签名与网关工具（`service/paymentasia.go`）

```go
func BuildPaymentAsiaSign(fields map[string]string, secret string) string
func VerifyPaymentAsiaSign(fields map[string]string, secret string) bool // 常数时间比较
func PaymentAsiaGatewayURL(sandbox, generic bool, token string) string
```

实现要点：

- 排序用 `sort.Strings`（key 均为 ASCII）；
- **PHP 兼容 urlencode**：自实现 `encodePHPQueryValue`——`A-Za-z0-9`、`-`、`_`、`.` 原样，空格→`+`，其余字节百分号编码（大写 hex）。不要直接用 `url.Values.Encode()`（Go 不转义 `~`，与 PHP 不一致）；
- 拼接后直接加 secret（无分隔符），`sha512.Sum512` → `hex.EncodeToString`；
- 验签时先 `strings.ToLower` 再 `subtle.ConstantTimeCompare`；
- **签名覆盖范围**：Hosted page 请求签名覆盖我们发送的全部字段；datafeed 验签只使用文档固定的五个字段（`amount` / `currency` / `request_reference` / `merchant_reference` / `status`）作为 allowlist——文档 p.11 的验签示例就是这五个字段。若网关回调附加新字段，不参与验签，否则任何新增字段都会导致所有真实回调被拒；
- 不记录 secret / 完整签名以外的敏感信息。

### 6.4 接口

#### `POST /api/user/paymentasia/amount`

请求：`{ "amount": <int 展示单位> }`

响应：`{ "message": "success", "data": "<实付金额字符串，2 位小数>" }`

校验顺序与 Epay 一致：最低充值 → `rejectInvalidTopUpQuota` → 取用户分组 → `getPaymentAsiaPayMoney` → 实付 ≥ 0.01。

#### `POST /api/user/paymentasia/pay`

请求：`{ "amount": <int>, "payment_method": "paymentasia_xxx" }`

处理：

1. 校验 `isPaymentAsiaTopUpEnabled()`、金额、额度上限、`service.IsPaymentAsiaMethod`，并且要求该 type 已在 `operation_setting.ContainsPayMethod` 中（与 `RequestEpay` 一致：没在管理端启用的通道不能被直接调用）。
2. 解析通道；校验币种/通道兼容（USD 仅 CreditCard）；Octopus 金额按 1 位小数。
3. 计算 `payMoney`：
   ```
   base = amount
   if TOKENS: base = amount / QuotaPerUnit
   payMoney = round(base × PaymentAsiaUnitPrice × groupRatio × discount, 2)  // Octopus 1 位
   ```
4. 生成 `trade_no`：`PA{uid}NO{6位随机}{unix}`，校验 ≤36；插入 `TopUp`：
   - `Amount`：与 Epay 相同（TOKENS 模式先 ÷QuotaPerUnit 取整）
   - `Money`：四舍五入后的实付金额（网关币种）
   - `Rate`：CNY 展示模式下快照 `operation_setting.Price`，否则 0
   - `PaymentMethod`：`paymentasia_xxx`
   - `PaymentProvider`：`paymentasia`
   - `Status`：`pending`
5. 组装字段：`merchant_reference=trade_no`、`currency`、`amount`、`return_url`、`notify_url`、`customer_ip`、`network`、`wallet?`、`subject`、`customer_email?`。
   - `notify_url` 默认 `service.GetCallbackAddress() + "/api/paymentasia/notify"`，可覆盖。
   - `return_url` 默认 `paymentReturnPath("/wallet?show_history=true")`，可覆盖。
   - `customer_ip`：`c.ClientIP()` 非 IPv4 时回退 `"0.0.0.0"` 并 `LogWarn`。
6. `sign = BuildPaymentAsiaSign(fields, secret)`；返回：
   ```json
   {
     "message": "success",
     "data": { "...fields including sign": "..." },
     "url": "https://payment.pa-sys.com/app/page/{token}"
   }
   ```
   前端沿用现有 `submitPaymentForm(url, data)`。

> 共享 `PayMethods` 的隔离：`paymentasia_*` 进入支付方式列表后，`RequestEpay`（`controller/topup.go`）与订阅的 `SubscriptionRequestEpay`（`controller/subscription_payment_epay.go`）都必须显式拒绝 `paymentasia_*`，否则会被当成 Epay type 拉起易支付订单。`GetTopUpInfo` 的 paymentasia 分支也只认 `service.IsPaymentAsiaMethod` 返回 true 的 type，避免管理端错拼的 `paymentasia_*` 变成死按钮。

#### `POST /api/paymentasia/notify`（公开，`anonymousRequestBodyLimit`）

处理：

1. `isPaymentAsiaWebhookEnabled()` 拒绝未配置。
2. `ParseForm`；取 POST 字段；验签失败 → 400 + 日志。
3. `LockOrder(merchant_reference)` + `defer UnlockOrder`；查 `TopUp`：
   - 不存在 → 日志 + 200（不重试未知订单）
   - `PaymentProvider != paymentasia` → 日志 + 200（不让其它渠道的订单被误结算）
4. 核对 `currency`（与当前配置一致，大小写不敏感）与 `amount`（decimal 精确等于 `Money` 四舍五入值）；不一致 → 400 + 日志。
5. 按 `status`：
   - `"1"`：若 pending → `model.RechargePaymentAsia(tradeNo, c.ClientIP())`；成功/幂等 → 200。
   - `"2"`：`model.UpdatePendingTopUpStatus(tradeNo, model.PaymentProviderPaymentAsia, common.TopUpStatusFailed)`；200。
   - `"0"` / `"4"`：仅记录日志，200。
6. 内部错误（DB/额度）→ 500，让 PaymentAsia 有机会重试；具体重试策略未知，幂等保证重复回调安全。

> 配置切换策略（有意为之）：验签只使用**当前环境**的 secret，币种只与**当前配置**比对。沙箱→生产切换后，旧沙箱 pending 订单的回调会被拒绝（测试单不应进生产额度）；生产中改币种同理。运维规则：存在待支付订单时不要切环境/改币种，遗留订单走管理员补单。

### 6.5 结算（`model/topup.go`）

新增 `RechargePaymentAsia(tradeNo, callerIp)`，复制 `RechargeEpay` 的事务骨架：

- `lockForUpdate` 行锁；provider 校验；`success` → `alreadyDone`；非 `pending` → 错误；
- `quotaToAdd = WalletQuotaFromDecimalStrict(topupAmountToQuotaDecimal(Amount, topUpRate(topUp)))`；
- 更新订单 `success` / `CompleteTime`，`creditTopUpQuota` 同一事务；
- 事务外 `syncCreditUserQuotaCache` + `RecordTopupLog`。

`topUpCreditedQuota` 增加显式分支（避免落到旧订单启发式）：

```go
case PaymentProviderPaymentAsia:
    return common.QuotaFromDecimal(topupAmountToQuotaDecimal(topUp.Amount, topUpRate(topUp)))
```

`ManualCompleteTopUp` 对非 Stripe 渠道已按 `Amount` 语义结算，无需改动，管理员补单天然可用。

### 6.6 GetTopUpInfo（`controller/topup.go`）

- 支付方式过滤：`strings.HasPrefix(method["type"], "paymentasia_")` → `isPaymentAsiaTopUpEnabled()`；其它未知 type 仍走 Epay 判定。
- 返回给前端的 paymentasia 方法对象附加 `"currency": setting.PaymentAsiaCurrency`（用于前端正确显示 HK$/US$）。
- 新增 data 字段：`enable_paymentasia_topup`（启用且至少有一个 paymentasia 方法）、`paymentasia_min_topup`。

### 6.7 Option 注册（`model/option.go`）

- `InitOptionMap` 写入 10 个 key 的默认值；
- `UpdateOption` switch 增加对应赋值（bool/float/int 按现有模式解析）。

### 6.8 路由（`router/api-router.go`）

- 用户侧：`POST /api/user/paymentasia/amount`、`POST /api/user/paymentasia/pay`（`middleware.CriticalRateLimit()`）。
- 公开回调：`POST /api/paymentasia/notify`（`anonymousRequestBodyLimit`，与 `/api/alipay/notify` 同级）。

### 6.9 可观测性与交易追踪

关联键只有一个：`merchant_reference = trade_no`。PaymentAsia 的 `request_reference`（网关侧唯一号）在回调时记录到应用日志与 topup 审计信息。

**四层记录，同一笔交易可交叉对齐：**

| 视角 | 内容 | 查询方式 |
|---|---|---|
| `topups` 表 | `trade_no`、`user_id`、`amount`、`money`、`payment_method`、`payment_provider`、`status`、`create_time`/`complete_time`、`rate` | 管理端充值记录 / `GET /api/user/topup?keyword=<trade_no>` |
| 应用日志 | 每个阶段一行；`logger.LogInfo(ctx, ...)` 自动带 request id（`X-Oneapi-Request-Id`） | `grep '<trade_no>'` 日志文件 |
| `logs` 表（type=topup） | 用户可见充值日志 + `admin_info` 审计字段 | 用量日志详情（管理员可见 `admin_info`） |
| PaymentAsia 后台 | `merchant_reference` + `request_reference` | 商户后台；Query API（phase 2） |

**必须打的日志点（全部带 `trade_no`，使用 `logger` 的 ctx 版本）：**

1. 创建订单：`paymentasia 订单创建 trade_no= user_id= amount= money= currency= method= network= request_id=`
2. 回调收到：`paymentasia 回调收到 trade_no= request_reference= status= amount= currency= client_ip= sign_ok= request_id=`
3. 拒绝原因（验签失败/金额不符/币种不符/订单不存在/provider 不匹配/通道禁用）：`LogWarn` + 原始字段；**绝不打印 secret，sign 只记录 `sign_ok`**
4. 结算：`paymentasia 充值成功 trade_no= user_id= quota_to_add= money= request_reference=`；重复回调 → `幂等跳过`；失败 → `LogError`（500 让网关重试）
5. `status=2`：`paymentasia 回调标记失败 trade_no= status= request_reference=`

**审计日志扩展：** 现有 `RecordTopupLog(userId, content, callerIp, paymentMethod, callbackPaymentMethod)` 只写支付方式；新增 `RecordTopupLogWithMeta(..., meta map[string]any)`（`RecordTopupLog` 委托传 `nil`）。PaymentAsia 结算把 `trade_no`、`currency`、`request_reference`、`channel` 写进 `admin_info`；用量日志详情弹窗 `web/src/features/usage-logs/components/dialogs/details-dialog.tsx` 的 topup 审计区追加展示这些字段。

**掉单排查路径（v1）：**

- 订单 pending 且无回调 → 按 `trade_no` 查应用日志（创建是否有、回调是否收到/被拒）；网关侧状态用 Payment Query API 手工 curl（phase 2 做成管理端「同步状态」按钮）。
- 确认已付款但回调丢失 → 管理员补单 `AdminCompleteTopUp`，现有审计日志会记录操作人与 caller IP。

**已知限制：** `TopUp` 表没有 `request_reference` 列（v1 不改 schema），所以不能按网关侧单号在管理端充值列表直接搜索；需要时在应用日志/`admin_info` 中检索。若后续运营强依赖该检索，再评估加列。

## 7. 前端设计

### 7.1 钱包充值

- `constants.ts`：`PAYMENT_TYPES` 增加 10 个 `paymentasia_*`；新增
  ```ts
  export function isPaymentAsiaPayment(paymentType: string): boolean {
    return paymentType.startsWith('paymentasia_')
  }
  ```
- `api.ts`：`calculatePaymentAsiaAmount`、`requestPaymentAsiaPayment`（分别 POST amount/pay）。
- `hooks/use-payment.ts`：`PaymentAmountCalculators` 增加 `paymentAsia`；按 `isPaymentAsiaPayment` 选择 calculator；支付分支走现有 regular 表单提交（响应含 `url` + `data`）。
- `types.ts`：`PaymentMethod` 增加可选 `currency?: 'HKD' | 'USD'`；`TopupInfo` 增加 `enable_paymentasia_topup?`、`paymentasia_min_topup?`。
- `hooks/use-topup-info.ts`：`parsePaymentMethods` 透传 `currency`。
- `lib/payment.ts`：`getMinTopupAmount` 增加 `enable_paymentasia_topup` → `paymentasia_min_topup` 分支；每个 PaymentAsia 方法按钮的可用下限取 `max(method.min_topup, paymentasia_min_topup)`，避免页面初始化在 1 而接口拒绝。
- `components/recharge-form-card.tsx`：
  - `hasConfigurableTopup` 纳入 `enable_paymentasia_topup`；
  - 选中 paymentasia 方法时，Pay 金额符号按 `method.currency` 显示 `HK$` / `US$`，其余情况维持现有本地币种符号。
  - 预设金额卡片上的 "Pay" 估算沿用 Epay 汇率（与 Stripe/Waffo 现有行为一致，属于既有跨网关限制）；选择 PaymentAsia 后以 "Amount to pay" 与确认弹窗为准，v1 不为它新增价格 API。
- `lib/ui.tsx`：`getPaymentIcon` 增加 paymentasia_alipay* → `SiAlipay`、paymentasia_wechat → `SiWechat`、paymentasia_creditcard/generic → `CreditCard`、paymentasia_fps → `Landmark`，其余默认。
- `components/subscription-plans-card.tsx`：`getEpayMethods` 过滤掉 `paymentasia_*`（v1 订阅不支持 PaymentAsia）。

### 7.2 管理端设置

- 新增 `web/src/features/system-settings/integrations/paymentasia-settings-section.tsx`：
  - 字段：沙箱开关、生产/沙箱 Token & Secret、币种（HKD/USD）、单价、最低充值、Notify/Return URL 覆盖；
  - 只读提示：默认回调地址 `{ServerAddress}/api/paymentasia/notify`（ServerAddress 为空时用 `window.location.origin`）；
  - 保存走现有 `updateOption` 批量接口。
- `payment-settings-section.tsx`：挂载新 section；zod schema / 默认值 / 保存 payload 增加 10 个字段。
- `payment-methods-visual-editor.tsx` + `payment-method-dialog.tsx`：增加 paymentasia 通道模板与 type 选项，默认图标；更新 type 说明文案（paymentasia_* 不再透传 Epay）。
- `web/src/features/system-settings/types.ts`、`billing/index.tsx`、`billing/section-registry.tsx`：补齐新字段默认值/传递。
- i18n：新增文案走 `t('...')`，提交前 `bun run i18n:sync`（在 `web/` 下）。

## 8. 安全

- **只信任签名**：datafeed 无其它鉴权；验签必须常数时间比较；secret 不进日志、不进前端。
- **金额/币种核对**：防止错单/部分金额；签名已保证完整性，核对是纵深防御。
- **幂等**：进程内 `LockOrder` 只是优化，正确性由 `RechargePaymentAsia` 的事务 + 行锁保证；多实例部署安全。
- **Provider 校验**：只允许 `paymentasia` 订单被该回调结算。
- **回调响应**：验签/核对失败 400；业务成功/幂等/忽略 200；内部错误 500（便于重试）。PaymentAsia 的具体重试策略需联调确认。
- **凭据管理**：只存 OptionMap（DB），`paymentasia/key.png`、`paymentasia.pdf` 不入库；生产与沙箱凭据分离。
- **IP**：IPv6 回退 `0.0.0.0` 并告警（文档要求 IPv4）。
- **限额**：复用 `common.WalletQuotaFromDecimalStrict`、`MaxWalletQuota`；金额换算用 decimal，禁止裸 float 累乘入账。

## 9. 测试与验收

### 9.1 自动化测试

- `service/paymentasia_test.go`（新增）：
  - PHP 兼容 urlencode（空格、逗号、`@`、`~`、`*`）；
  - 使用文档 pre-sign string（`IphoneX` 笔误值）+ secret 精确复现 sample hash；另用自造字段向量断言排序与拼接；
  - datafeed allowlist：多出未文档化字段不影响验签；
  - 验签成功 / 篡改字段 / 篡改签名 / 大小写 hex；
  - 通道映射：每个 type → network/wallet；未知 type 拒绝。
- 支付方式隔离：Epay 与订阅支付接口拒绝 `paymentasia_*`；`GetTopUpInfo` 不展示未知 `paymentasia_*`（把 type 判定抽成小函数后做表驱动测试）。
- `controller/payment_webhook_availability_test.go`：`isPaymentAsiaTopUpEnabled` 在 live/sandbox 凭据组合下的真假值。
- `model/topup_test.go`：`TestTopUpCreditedQuotaByProvider` 增加 paymentasia 行（CNY/USD 两种展示类型）。
- 前端：`web/src/features/wallet/__tests__/` 增加 `isPaymentAsiaPayment` / Pay 币种符号的行为测试；受影响组件测试按 `web/AGENTS.md` 3.14 补充。
- 命令：
  - 后端：`go test ./service/... ./model/... ./controller/... ./router/... -race`（按本地全量测试规程）；
  - 前端：`cd web && bun run typecheck && bun run lint`，受影响测试文件。

### 9.2 手工验收

1. 沙箱凭据配置后，钱包页出现启用的通道；金额计算与预期一致。
2. 点击支付 → 新标签打开托管页；表单字段与签名可被抓包核对。
3. 沙箱完成支付 → datafeed 命中 `/api/paymentasia/notify` → 订单 success，额度增加一次；重复回调不重复入账。
4. 篡改签名/金额/币种的回调不产生入账。
5. 生产最小金额（如 HKD 1）真实支付一笔，核对订单、额度、日志；用 Payment Query API（手工 curl）确认状态。
6. 关闭凭据后通道消失，接口不可拉起支付。
7. 给定一个 `trade_no`，能从 `topups` 表、应用日志、`logs` 表（`admin_info`）和 PaymentAsia 后台四个视角对齐同一笔交易。

### 9.3 数据库

无 schema 变更；结算复用现有 `lockForUpdate`/`creditTopUpQuota`。按规程运行真实 SQLite/MySQL/PostgreSQL 的 model 测试与启动迁移两遍确认（避免回归 GORM 行为）。

## 10. 上线步骤

1. 代码合并后先不配置凭据（dark）：支付方式列表不出现，行为与现在一致。
2. 申请到沙箱账号后，在管理端填沙箱凭据 + 沙箱开关，添加推荐通道（AlipayHK/WeChat/CreditCard/FPS）做端到端测试。
3. 沙箱通过后切生产凭据（关闭沙箱），用最小金额真实支付验证。
4. 观察订单、回调日志、额度入账；确认无异常后再向用户开放。
5. 若发现签名/回调语义与文档不符，保留日志中的原始字段（脱敏 secret）以便与 PaymentAsia 对账。

## 11. 风险与未决项

| 风险/未决 | 影响 | 处理 |
|---|---|---|
| 文档 `iPhoneX` / `IphoneX` 大小写笔误 | 照抄错误值做测试会误判 | 已验证 pre-sign string 与 hash 自洽；单测固定用 `IphoneX` 向量，沙箱首笔再抓真实签名核对 |
| 待支付期间切换环境/币种 | 旧订单回调被拒绝 | 文档化运维规则；管理员补单兜底（`ManualCompleteTopUp`） |
| `API Key` 用途未知 | 可能需要额外鉴权字段 | 申请沙箱邮件一并询问；v1 不存该字段 |
| datafeed 重试策略/期望响应体未知 | 失败时可能丢单或反复重试 | 幂等 + 500 触发重试 + 日志；phase 2 加查询 API 兜底 |
| 币种/通道兼容配置错误 | 网关拒单 | 支付前校验（USD 仅 CreditCard）+ 管理端提示 |
| `PaymentAsiaUnitPrice` 配错 | 多收/少收 | 管理端文案说明；上生产前用小额核对 |
| 生产凭据在 `paymentasia/key.png` | 泄露风险 | 不入库；若曾泄露则联系 PaymentAsia 轮换 |
| IPv6 客户端 | 回调/支付参数不合规 | 回退 `0.0.0.0` + 告警 |

## 12. 备选方案与否决理由

- **A. 每通道一个 PayMethods type（选中）**：复用现有编辑器与 type 分发；通道是固定枚举，不需要动态 JSON。
- **B. Waffo 式单 type + `PaymentAsiaMethods` 子列表**：灵活但需要新的子列表 UI、解析和保存链路；本场景收益低。
- **C. 抽象 Epay 网关接口后复用**：会改动线上 Epay 路径，签名/字段/状态语义差异大，风险大于收益。
- **D. 只做一个 `UserDefine` 通用收银台**：作为 `paymentasia_generic` 保留，但不作为唯一形态；否则无法在站点控制展示哪些通道。
