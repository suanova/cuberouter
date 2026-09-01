# 视频按秒定价(通用能力,先接 vidu) — 设计规格

日期: 2026-09-01
状态: 已批准(2026-09-01 对话确认)

## 1. 背景与目标

现状: 视频/任务渠道(如 vidu)按次计费 —— 模型基础价 × 适配器硬编码的系数(时长/分辨率/错峰)。系数表写在代码里(上一轮实现的 `viduResolutionRatios` / `viduOffPeakRatios`),管理员无法配置,用户看定价页也不直观。

目标: 管理员在模型价格页直接配置「分辨率 × 正常价(¥/秒)× 错峰价(¥/秒)」,系统自动推导计费系数(管理员不算比例);用户定价页直接看到该表。做成通用能力,本轮先接 vidu。

需求来源(用户 2026-09-01 确认):
- 配置入口: 模型价格页内编辑(新增「视频按秒」计费模式)
- 错峰窗口: 全局可配(默认 北京时间 22:00-次日 08:00)
- 适用范围: 通用能力 + 先接 vidu

## 2. 配置模型(后端存储)

### 2.1 每模型视频价格表

新增 option key `VideoPrice`(JSON 字符串,平行于现有 `ModelPrice` option):

```json
{
  "viduq3-pro": {
    "rows": [
      {"resolution": "1080p", "normal_price": 0.75,    "off_peak_price": 0.375},
      {"resolution": "720p",  "normal_price": 0.625,   "off_peak_price": 0.3125},
      {"resolution": "540p",  "normal_price": 0.28125, "off_peak_price": 0.15625}
    ]
  }
}
```

- 价格单位为 **¥/秒,管理员填写值原样存储,不做换算**
- 存储:`setting/ratio_setting` 新增 `videoPriceMap`(RWMap[string, VideoPriceTable]),API 与 `modelPriceMap` 平行:
  - `UpdateVideoPriceByJSONString(jsonStr)`(option 更新路径,`model/option.go` switch 新增 `case "VideoPrice"`)
  - `GetVideoPrice(model) (*VideoPriceTable, bool)`
  - 校验: 行数 ≥1、resolution 非空、normal_price > 0、off_peak_price > 0(系数系统无法表达 0 比例,零价会静默按正常价计费;非法整表拒绝并报错)
- 校验时自动同步:`anchor = normal_price 最高行`;同步写入 `modelPriceMap`(`modelPrice = anchor ¥/秒 ÷ 系统 USD 汇率`,汇率复用现有展示汇率配置,未配置默认 7.3),保证计费引擎零改动

### 2.2 全局错峰窗口

新增 option key `OffPeakWindow`(JSON,默认值即 22:00-08:00 Asia/Shanghai):

```json
{"start_hour": 22, "end_hour": 8, "timezone": "Asia/Shanghai"}
```

- 半开区间 `[start_hour, end_hour)`,支持跨零点(start > end)
- 存储:`setting/ratio_setting` 或 `operation_setting` 新增 `GetOffPeakWindow() *OffPeakWindow`(读 option,带默认)

### 2.3 公开定价 API 扩展

`GET /api/pricing`(`model.GetPricing()` → `[]Pricing`):
- `Pricing` struct 新增 `VideoPrices *VideoPriceTable`(json `video_prices,omitempty`),仅当模型配置了表时输出
- `Pricing` struct 新增全局 `OffPeakWindow`(可放响应顶层或每个 Pricing 内;顶层更合适,见 §5)

## 3. 计费引擎

### 3.1 系数推导(系统算,管理员不接触)

锚点规则: `anchor = 表内 normal_price 最高的行`(保证所有 size 系数 ≤ 1)。

| 系数 | 公式 | 说明 |
|---|---|---|
| `seconds` | 请求时长(缺省 5s,饱和 MaxTaskDurationSeconds) | 不变,沿用上轮实现 |
| `size` | `normal[res] / anchor` | 分辨率系数,由表推导 |
| `time` | `off_peak[res] / normal[res]`(仅错峰时段) | 错峰系数,由表推导 |

扣费校验: `charge = anchor × size × time × seconds = 该分辨率该时段 ¥/秒 × 时长`(对用户 12 格价格表逐格成立)。

缺省分辨率: 请求未传时 720p(沿用上轮);未知模型/分辨率 → 系数 1.0 保守计费。

### 3.2 vidu 适配器改动

- 删除硬编码 `viduResolutionRatios` / `viduOffPeakRatios`
- `EstimateBilling` / `AdjustBillingOnSubmit` 改为:模型有表 → 从 `ratio_setting.GetVideoPrice(model)` 推导 size/time;无表 → 只报 seconds(退化为纯按次,与改动前行为兼容)
- 错峰窗口判断改读 `GetOffPeakWindow()`(替代写死的 22/8)
- `seconds`、缺省、饱和、回显结算链路全部保留

### 3.3 通用能力边界

推导逻辑放在适配器可复用的位置(`relay/channel/task/taskcommon` 或 `relay/helper`),本轮仅 vidu 调用;其他 task 平台(ali/kling 等)后续各自接入。

## 4. 前端改动

### 4.1 模型价格页编辑器(`web/src/features/system-settings/models/`)

- `PricingMode` 新增 `'video-per-second'`(`model-pricing-core.ts`),`model-pricing-sheet.tsx` 加第 4 个 Tab「视频按秒」
- 新模式编辑器(新组件,如 `video-price-editor.tsx`):
  - 分辨率行列表,可增删行: 分辨率 / 正常价(¥/秒)/ 错峰价(¥/秒)
  - 保存 → 后端 option 保存路径(与现有模型价格保存一致)
  - 该模式下 per-request 输入隐藏(modelPrice 由后端自动生成)
- 加载逻辑: 模型已有视频表 → 自动切到该模式并载入表
- 错峰窗口: 模型价格页顶部全局配置块(读/写 `OffPeakWindow` option)

### 4.2 用户定价页(`web/src/features/pricing/`)

- 视频模型(有 `video_prices`)显示「分辨率 | 视频时长(¥/秒) | 错峰价(¥/秒)」表,不显示系数
- 错峰窗口展示在表旁(「错峰时段: 22:00-次日 08:00」)
- i18n: 新增文案按 `web/AGENTS.md` 约定加入 `web/src/i18n/locales/{lang}.json`

## 5. API 契约

| 路径 | 方法 | 说明 |
|---|---|---|
| `/api/pricing` | GET | 响应 `[]Pricing` 增加 `video_prices` 字段;全局错峰窗口随响应返回(顶层 `off_peak_window`) |
| option 保存 | PUT | 现有 option 保存路径,新增 `VideoPrice` / `OffPeakWindow` 两个 key(沿用 `model/option.go` switch 模式) |

## 6. 测试

后端(确定性表驱动):
- 表解析/校验(非法行拒绝)、锚点选择(最高 normal 行)
- 系数推导逐格核对(覆盖用户 12 格价格表)
- 错峰窗口边界(跨零点、时区、默认值)
- vidu 适配器: 有表/无表退化、未知分辨率/模型保守 1.0、seconds 饱和(沿用现有测试结构调整)
- 汇率换算: anchor ¥/秒 → modelPrice 的换算与汇率配置联动

前端(按 `web/AGENTS.md` 约定):
- 编辑器: 行增删、模式切换、保存载荷
- 定价页: 视频表渲染、无表模型不显示

## 7. 范围外(本轮不做)

- 其他 task 平台接入(通用能力就绪后各自接)
- `viduq3-pro-fast` 计费与「仅图生」限制
- 参考图生视频的 viduq2 模型约束(保持现状)
