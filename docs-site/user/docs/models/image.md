# 图像生成

> 使用 OpenAI Images 兼容接口进行文生图和图像编辑。

站内图像模型统一走 OpenAI Images 风格接口：文生图用 `POST /v1/images/generations`，图像编辑用 `POST /v1/images/edits`。不同模型支持的参数和返回格式不同，下面按系列说明。

## 端点

| 用途 | 方法与路径 | 请求格式 |
| --- | --- | --- |
| 文生图 | `POST /v1/images/generations` | `application/json` |
| 图像编辑 | `POST /v1/images/edits` | `multipart/form-data` 或 `application/json` |

请求头：

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

图像编辑上传文件时使用 `multipart/form-data`，不要手动设置 boundary，由 HTTP 客户端生成。

## 支持的模型

| 系列 | 站内型号 | 文生图 | 图像编辑 | 备注 |
| --- | --- | :---: | :---: | --- |
| GPT Image | `gpt-image-2.5-flare`、`gpt-image-2.5-flare-b`、`gpt-image-2.5-flare-m`、`gpt-image-2.5-sunburst-b`、`gpt-image-2.5-sunburst-m`、`gpt-image-2-m`、`gpt-image-2-b`、`gpt-image-1.5-m` | ✓ | ✓ | 2.5 系列分速度优先（`flare`）和编辑精度优先（`sunburst`） |
| Gemini Image | `gemini-3.1-flash-image-m`、`gemini-3-pro-image-m` | ✓ | ✓ | Nano Banana 系列，图像生成与编辑 |
| Qwen Image | `qwen-image-2.1-turbo` | ✓ | ✓ | 站内固定输出 PNG |
| Wan Image | `wan2.7-image-a` | ✓ | — | 支持 1K/2K/4K，单次最多 4 张 |
| MOSS Image | `moss-image` | ✓ | — | 512×512 至 2048×2048，支持批量与固定种子 |

> 同一系列的不同后缀是不同模型，参数和可用分组可能不同。调用时使用模型广场中的完整 ID；实际可用型号以模型广场为准。
>
> 表中「图像编辑」表示模型具备编辑能力；具体到某个型号是否开放 `/v1/images/edits`，以实际返回为准。已验证 `gpt-image-2.5-flare-m`、`gpt-image-2.5-sunburst-m` 和 `gpt-image-2-m` 支持编辑接口。

## 文生图

### 请求参数

```json
{
  "model": "gpt-image-2.5-flare-m",
  "prompt": "一只戴宇航头盔的橘猫，扁平插画风格，纯白背景",
  "n": 1,
  "size": "1024x1024",
  "quality": "low",
  "output_format": "png"
}
```

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | :---: | --- |
| `model` | string | 是 | 站内图像模型 ID |
| `prompt` | string | 是 | 图片描述，支持中英文 |
| `n` | integer | 否 | 生成张数，默认 `1`；模型可能有更小的上限 |
| `size` | string | 否 | 尺寸，格式和取值范围因模型而异 |
| `quality` | string | 否 | `low` / `medium` / `high` / `auto`，主要用于 GPT Image |
| `response_format` | string | 否 | `url` 或 `b64_json`；部分模型固定返回 `b64_json` |
| `output_format` | string | 否 | `png` / `jpeg`；`webp` 不一定支持 |
| `background` | string | 否 | `transparent` / `opaque` / `auto`，主要用于 GPT Image |
| `moderation` | string | 否 | `low` / `auto`，主要用于 GPT Image |
| `output_compression` | integer | 否 | `0–100`，仅对 `jpeg` 等有损格式生效 |
| `stream` | boolean | 否 | 是否流式返回预览，仅部分模型支持 |
| `partial_images` | integer | 否 | 流式预览图数量，`0–3`，需要 `stream=true` |
| `seed` | integer | 否 | 随机种子，支持情况因模型而异 |
| `num_inference_steps` | integer | 否 | 采样步数，部分自建模型支持 |
| `watermark` | boolean | 否 | 是否添加水印，支持情况因模型而异 |

### 尺寸（`size`）

- **GPT Image 2.5 / 2**：只接受像素值 `宽x高` 或 `auto`，**不接受** `1K`/`2K`/`4K`、`16:9` 等字符串。
  - 宽高需要是 16 的倍数；`1920x1080` 会 400，`1024x1024`、`1536x1024`、`1024x1536` 可用。
  - 总像素有上下限：`768x768`、`1024x576`、`4096x4096` 会被拒绝。
  - `auto` 实测返回 `1254x1254`。
  - 比例需要自己换算成像素尺寸，例如 3:2 用 `1536x1024`，2:3 用 `1024x1536`。
- **Wan 2.7 Image**：支持 1K/2K/4K 档位和多种宽高比，具体写法以模型广场说明为准。
- **MOSS Image**：支持 512×512 至 2048×2048，未传时默认 `1024x1024`。
- **Qwen Image / Gemini Image**：使用上游接受的尺寸写法；常见 `1024x1024`，其它尺寸以实际返回为准。

### 返回格式

```json
{
  "created": 1792496626,
  "data": [
    {"b64_json": "iVBORw0KGgo..."}
  ]
}
```

或：

```json
{
  "created": 1792496626,
  "data": [
    {"url": "https://..."}
  ]
}
```

- GPT Image、Qwen Image、MOSS Image 站内固定返回 `b64_json`，需要自行 Base64 解码。
- 部分模型可能在 `usage` 中返回 token 用量；以响应为准。
- Base64 图片体积较大（1MB 图片约 200KB Base64），不要写入普通日志。

## 图像编辑

### multipart 上传

```bash
curl -sS https://<base_url>/v1/images/edits \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -F "model=gpt-image-2.5-sunburst-m" \
  -F "prompt=把背景换成纯白色，保留主体细节" \
  -F "size=1024x1024" \
  -F "quality=low" \
  -F "image=@input.png;type=image/png"
```

带蒙版：

```bash
curl -sS https://<base_url>/v1/images/edits \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -F "model=gpt-image-2.5-sunburst-m" \
  -F "prompt=把蒙版区域替换成蓝色圆形" \
  -F "size=1024x1024" \
  -F "quality=low" \
  -F "image=@input.png;type=image/png" \
  -F "mask=@mask.png;type=image/png"
```

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | :---: | --- |
| `image` | file | 是 | 待编辑图片，png/jpeg/webp；可重复该字段传多张参考图 |
| `mask` | file | 否 | PNG，带 alpha 通道；透明区域为可编辑区域，尺寸必须与 `image` 一致 |
| `prompt` | string | 是 | 编辑指令 |
| `n` | integer | 否 | 生成张数 |
| `size` / `quality` / `background` / `output_format` / `output_compression` / `moderation` | — | 否 | 同文生图 |

::: warning
GPT Image 上游不支持 `input_fidelity`，传了会返回 400。不需要蒙版时直接省略 `mask`，不要传空文件或空值。
:::

### JSON + `image_url`

GPT Image 2.5 和 2 支持用 JSON 传参考图，图片需要是上游可直接访问的公网 URL：

```bash
curl -sS https://<base_url>/v1/images/edits \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2.5-sunburst-m",
    "prompt": "在背景里加一道彩虹",
    "images": [
      {"image_url": "https://example.com/input.png"}
    ]
  }'
```

注意：

- `prompt` 必须放在顶层。
- 参考图写成 `images: [{"image_url": "..."}]`，不要写成 `images: ["https://..."]`。
- 不要用顶层 `image_url` 或 `image` 字段。
- 需要鉴权的图片请先下载，再用 multipart 上传。

## 各系列差异

| 系列 | 差异 |
| --- | --- |
| GPT Image 2.5 | 文生图 + 编辑；`size` 只接受像素值；`quality`、`background`、`moderation`、`output_compression` 可用；`output_format=webp` 在已验证型号上会 400，建议使用 png/jpeg；固定返回 `b64_json`；支持 `stream` + `partial_images` 流式预览。 |
| GPT Image 2 | 文生图 + 编辑；`size` 规则与 2.5 一致；支持 JSON `image_url` 编辑；固定返回 `b64_json`。 |
| GPT Image 1.5 | 文生图 + 编辑；参数以 OpenAI Images 标准为主，具体支持以模型广场为准。 |
| Gemini Image | 文生图 + 编辑；适合需要 Gemini 图像理解/生成能力的场景；尺寸和比例写法以上游支持为准。 |
| Qwen Image | 文生图 + 编辑；站内固定 `output_format=png`、`response_format=b64_json`；客户端传 `jpeg` 也可能被覆盖为 PNG。 |
| Wan Image | 文生图；支持 1K/2K/4K 和多种比例，单次最多 4 张；不支持编辑。 |
| MOSS Image | 文生图；支持 512–2048 分辨率和批量生成；站内固定 30 步采样、默认 `1024x1024`、返回 `b64_json`；不支持编辑。 |

## 代码示例

### Python（OpenAI SDK）

```python
import base64
import os

from openai import OpenAI

client = OpenAI(
    api_key=os.environ["CUBEROUTER_API_KEY"],
    base_url="https://<base_url>/v1",
)

resp = client.images.generate(
    model="gpt-image-2.5-flare-m",
    prompt="一只戴宇航头盔的橘猫，扁平插画风格，纯白背景",
    size="1024x1024",
    quality="low",
    n=1,
)

with open("out.png", "wb") as f:
    f.write(base64.b64decode(resp.data[0].b64_json))
```

### 图像编辑（Python）

```python
with open("input.png", "rb") as image:
    resp = client.images.edit(
        model="gpt-image-2.5-sunburst-m",
        image=image,
        prompt="把背景换成纯白色，保留主体细节",
        size="1024x1024",
        quality="low",
    )

with open("edited.png", "wb") as f:
    f.write(base64.b64decode(resp.data[0].b64_json))
```

### 文生图（curl + jq 解码）

```bash
curl -sS https://<base_url>/v1/images/generations \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2.5-flare-m",
    "prompt": "一只戴宇航头盔的橘猫，扁平插画风格，纯白背景",
    "size": "1024x1024",
    "quality": "low",
    "n": 1,
    "output_format": "png"
  }' \
  | jq -r '.data[0].b64_json' \
  | base64 -d > out.png
```

## 常见问题

### Q1: 返回 400 `Invalid request parameters`？

优先检查：

- `size` 是否用了 GPT Image 不接受的写法（`1K`、`16:9`、`1920x1080`）。
- 是否传了 `output_format=webp` 或 `input_fidelity`。
- JSON 编辑是否把 `prompt` 放在了顶层、参考图是否写成 `images: [{"image_url": "..."}]`。
- `mask` 是否与 `image` 同尺寸的 PNG。

上游对这类错误通常只返回通用文案，需要逐项排除。

### Q2: 为什么响应里没有 URL？

GPT Image、Qwen Image、MOSS Image 固定返回 `data[].b64_json`，需要自行解码。不要依赖 `response_format=url`。

### Q3: 调用超时？

首次生成通常需要 10–60 秒，高质量、大尺寸会更慢。客户端超时建议不低于 60 秒；流式预览只能提前看到中间图，最终仍需等待完成事件。

### Q4: `n` 可以传多大？

服务端对 `n` 有上限保护，但每个模型的实际上限更小。GPT Image 建议 1–10；Wan 2.7 单次最多 4 张；超出模型限制可能返回 400 或只返回部分结果。

### Q5: 模型名带 `-m`、`-b` 有什么区别？

它们是不同站内模型，可能对应不同上游和可用分组。例如 `gpt-image-2.5-flare-m` 和 `gpt-image-2.5-flare-b` 不是同一个模型，不能互相替代。以模型广场中的说明为准。
