# Model Tags Convention (模型标签规范)

> Last Updated: 2026-09-18
> 状态:定稿(第 5 节两个待定项均已决策)

## 1. 为什么 tag 必须是封闭词表

`models.tags`(逗号分隔字符串)不只是展示文案,它是定价页的**筛选维度**:

- `web/src/features/pricing/components/pricing-sidebar.tsx` — 侧边栏 tag 筛选器,选项从所有模型的 tag 动态聚合(小写去重)
- `web/src/features/pricing/lib/filters.ts` — 搜索框匹配 tag;`parseTags` 解析
- `web/src/features/pricing/components/model-card.tsx` — 模型卡片展示(有数量上限)

词表失控 = 筛选器报废。反面教材:上游 llm-metadata 的 352 个模型仅 context 类 tag 就有 **28 种取值**(128K / 131.1K / 262.1K / 204.8K ……精确 token 数换算),这种列表无法作为筛选项使用。

因此:**封闭词表 + 稀疏标注**。加新词是一次显式决策(改本文档),不许随手造。

## 2. 封闭词表(3 个维度,共 ~11 个词)

### 维度一:模态分类(模型"卖的是什么")

| tag | 何时标 |
|---|---|
| `图像生成` | 图像生成模型 |
| `视频生成` | 视频生成模型(如 vidu 系列) |
| `音频处理` | TTS / 语音转写模型 |
| `向量` | embedding 模型(2026-09-21 随 qwen3-embedding-8b 接入新增) |

**文本对话模型不标**——站上绝大多数模型是文本类,全标等于没标,筛选无意义还增加维护成本。缺省即文本。

### 维度二:上下文窗口(卖点才标)

| tag | 覆盖范围(向下取整) |
|---|---|
| `128K` | 128K ≤ context < 256K(含 131.1K、200K 等) |
| `256K` | 256K ≤ context < 512K(含 262.1K 等) |
| `512K` | 512K ≤ context < 1M |
| `1M` | ≥ 1M |

**< 128K 不标**——没有用户按"小上下文"筛选。每个模型最多 1 个 context tag。

### 维度三:模型能力(影响购买决策)

| tag | 含义 | 对照上游 |
|---|---|---|
| `推理` | 支持 reasoning / thinking | Reasoning |
| `工具` | 支持 function / tool calling | Tools |
| `多模态` | 支持图像等文本以外的输入理解（已定，见 5.1) | Vision |
| `开源` | 开放权重 | Open Weights |

上游的 `Files` **弃用**——那是平台文件上传能力,不是模型能力。

## 3. 标注规则

1. 每个模型典型 2~4 个 tag;维度一、维度二各最多 1 个,维度三最多 3 个
2. 只标卖点与区分度,不标"人人都有"的属性
3. context 一律向下取整到档位,禁止写精确 token 数
4. 新增词 = 修改本文档,视为约定变更

## 4. 与官方上游同步的关系

本地词表与上游（llm-metadata）已分叉。按 [model-metadata-rules.md](./model-metadata-rules.md) 第 9 条，本站**不使用官方同步功能**，所有模型 `sync_official=0`，tag 一律按本文档人工标注。

（若未来重新启用同步：overwrite 永远不要选 `tags` 字段，否则上游的 28 种 context 取值会覆盖本地词表；且同步的"新建"路径不看 `sync_official`，会带入英文描述和上游 tag。）

## 5. 已决事项

### 5.1 `视觉` vs `多模态`

**已定（2026-09-18）：用 `多模态`**，更通俗。语义约定为"支持图像等文本以外的输入理解"，对应上游的 Vision。

### 5.2 tag 存储语言

**已定（2026-09-18）：中文存储**，与 [model-metadata-rules.md](./model-metadata-rules.md) 第 1 条"说明信息使用中文"一致。pricing 页直接渲染原始 tag 字符串，前端不做 i18n；本站主要面向中文用户。context 类（`1M` / `128K`）无语言差异。

## 6. 示例

`kimi-k3`(1M 上下文、推理、工具、多模态、文本对话):

```
推理,工具,多模态,1M
```

对比上游原始 tag:`Reasoning,Tools,Files,Open Weights,Vision,1M`(6 个 → 收敛为 4 个;`Files` 弃用,`Open Weights` 受"能力维度最多 3 个"约束让位)。
