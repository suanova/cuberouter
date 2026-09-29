# Model & Vendor Icons (模型/厂商图标)

> Last Updated: 2026-09-18

## 1. 字段含义

`models.icon` 与 `vendors.icon` **不是自由文本,也不是图片 URL**,而是 [`@lobehub/icons`](https://github.com/lobehub/lobe-icons) 图标库的**组件路径**。前端按名字动态查库渲染 SVG 品牌 logo。

- 图标画廊(选名字用):**https://icons.lobehub.com**
- 当前依赖版本:`@lobehub/icons` v5.14.0(`web/`)
- 渲染入口:`web/src/lib/lobe-icon.tsx` 的 `getLobeIcon(iconName, size)`

## 2. 取值语法

| 写法 | 效果 |
|---|---|
| `"Moonshot"` | 基础图标 |
| `"OpenAI.Color"` | 彩色变体(`.` 后接子组件名) |
| `"Claude.Avatar.type={'platform'}"` | 链式属性,可传 `type` / `shape` / `size` 等 |

注意:

- **大小写敏感**——`moonshot` 查不到 `Moonshot`,会走降级
- 未设置或名字不存在时,降级为灰圆圈 + 名字首字母(如 "M")
- 本地自定义图标(不走 lobe 库):`Sub2API`、`Astraflow`,定义在 `lobe-icon.tsx` 的 `CUSTOM_ICONS`

## 3. 消费位置与 fallback 链

定价页模型卡片(`web/src/features/pricing/components/model-card.tsx`):

```
model.icon  →  vendor_icon  →  灰圆圈首字母
```

即**模型级留空时自动继承厂商图标**。厂商图标在厂商管理里维护。

## 4. 本站约定

1. **优先用厂商品牌图标,不用产品品牌**。例:kimi-k3 填 `Moonshot`(月之暗面),不填 `Kimi`——同一厂商的模型在定价页图标统一
2. **与上游元数据对齐**:llm-metadata 官方同步给的 icon 值可直接采用(上游用的就是 lobe 图标名)
3. **模型级可以留空**:厂商图标已正确时,留空走 fallback 即可;显式填写的好处是以后换厂商图标不影响该模型
4. 填新名字前先在 https://icons.lobehub.com 确认存在;或本地查:

```bash
ls web/node_modules/@lobehub/icons/es/ | grep -i <名字>
```

## 5. 示例

kimi-k3(test 站,2026-09-18 配置):

| 字段 | 值 | 说明 |
|---|---|---|
| `models.icon` | `Moonshot` | 与上游元数据、厂商图标一致 |
| `vendors.icon`(Moonshot, id=9) | `Moonshot` | 厂商级同一图标 |
