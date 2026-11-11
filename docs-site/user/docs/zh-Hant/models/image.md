# 圖像生成

> 使用 OpenAI Images 兼容接口進行文生圖和圖像編輯。

站內圖像模型統一走 OpenAI Images 風格接口：文生圖用 `POST /v1/images/generations`，圖像編輯用 `POST /v1/images/edits`。不同模型支援的參數和返回格式不同，下面按系列說明。

## 端點

| 用途 | 方法與路徑 | 請求格式 |
| --- | --- | --- |
| 文生圖 | `POST /v1/images/generations` | `application/json` |
| 圖像編輯 | `POST /v1/images/edits` | `multipart/form-data` 或 `application/json` |

請求頭：

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

圖像編輯上傳文件時使用 `multipart/form-data`，不要手動設置 boundary，由 HTTP 客戶端生成。

## 支援的模型

| 系列 | 站內型號 | 文生圖 | 圖像編輯 | 備註 |
| --- | --- | :---: | :---: | --- |
| GPT Image | `gpt-image-2.5-flare`、`gpt-image-2.5-flare-b`、`gpt-image-2.5-flare-m`、`gpt-image-2.5-sunburst-b`、`gpt-image-2.5-sunburst-m`、`gpt-image-2-m`、`gpt-image-2-b`、`gpt-image-1.5-m` | ✓ | ✓ | 2.5 系列分速度優先（`flare`）和編輯精度優先（`sunburst`） |
| Gemini Image | `gemini-3.1-flash-image-m`、`gemini-3-pro-image-m` | ✓ | ✓ | Nano Banana 系列，圖像生成與編輯 |
| Qwen Image | `qwen-image-2.1-turbo` | ✓ | ✓ | 站內固定輸出 PNG |
| Wan Image | `wan2.7-image-a` | ✓ | — | 支援 1K/2K/4K，單次最多 4 張 |
| MOSS Image | `moss-image` | ✓ | — | 512×512 至 2048×2048，支援批量與固定種子 |

> 同一系列的不同後綴是不同模型，參數和可用分組可能不同。調用時使用模型廣場中的完整 ID；實際可用型號以模型廣場爲準。
>
> 表中「圖像編輯」表示模型具備編輯能力；具體到某個型號是否開放 `/v1/images/edits`，以實際返回爲準。已驗證 `gpt-image-2.5-flare-m`、`gpt-image-2.5-sunburst-m` 和 `gpt-image-2-m` 支援編輯接口。

## 文生圖

### 請求參數

```json
{
  "model": "gpt-image-2.5-flare-m",
  "prompt": "一隻戴宇航頭盔的橘貓，扁平插畫風格，純白背景",
  "n": 1,
  "size": "1024x1024",
  "quality": "low",
  "output_format": "png"
}
```

| 參數 | 類型 | 必填 | 說明 |
| --- | --- | :---: | --- |
| `model` | string | 是 | 站內圖像模型 ID |
| `prompt` | string | 是 | 圖片描述，支援中英文 |
| `n` | integer | 否 | 生成張數，默認 `1`；模型可能有更小的上限 |
| `size` | string | 否 | 尺寸，格式和取值範圍因模型而異 |
| `quality` | string | 否 | `low` / `medium` / `high` / `auto`，主要用於 GPT Image |
| `response_format` | string | 否 | `url` 或 `b64_json`；部分模型固定返回 `b64_json` |
| `output_format` | string | 否 | `png` / `jpeg`；`webp` 不一定支援 |
| `background` | string | 否 | `transparent` / `opaque` / `auto`，主要用於 GPT Image |
| `moderation` | string | 否 | `low` / `auto`，主要用於 GPT Image |
| `output_compression` | integer | 否 | `0–100`，僅對 `jpeg` 等有損格式生效 |
| `stream` | boolean | 否 | 是否流式返回預覽，僅部分模型支援 |
| `partial_images` | integer | 否 | 流式預覽圖數量，`0–3`，需要 `stream=true` |
| `seed` | integer | 否 | 隨機種子，支援情況因模型而異 |
| `num_inference_steps` | integer | 否 | 採樣步數，部分自建模型支援 |
| `watermark` | boolean | 否 | 是否添加水印，支援情況因模型而異 |

### 尺寸（`size`）

- **GPT Image 2.5 / 2**：只接受像素值 `寬x高` 或 `auto`，**不接受** `1K`/`2K`/`4K`、`16:9` 等字符串。
  - 寬高需要是 16 的倍數；`1920x1080` 會 400，`1024x1024`、`1536x1024`、`1024x1536` 可用。
  - 總像素有上下限：`768x768`、`1024x576`、`4096x4096` 會被拒絕。
  - `auto` 實測返回 `1254x1254`。
  - 比例需要自己換算成像素尺寸，例如 3:2 用 `1536x1024`，2:3 用 `1024x1536`。
- **Wan 2.7 Image**：支援 1K/2K/4K 檔位和多種寬高比，具體寫法以模型廣場說明爲準。
- **MOSS Image**：支援 512×512 至 2048×2048，未傳時默認 `1024x1024`。
- **Qwen Image / Gemini Image**：使用上游接受的尺寸寫法；常見 `1024x1024`，其它尺寸以實際返回爲準。

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

- GPT Image、Qwen Image、MOSS Image 站內固定返回 `b64_json`，需要自行 Base64 解碼。
- 部分模型可能在 `usage` 中返回 token 用量；以響應爲準。
- Base64 圖片體積較大（1MB 圖片約 200KB Base64），不要寫入普通日誌。

## 圖像編輯

### multipart 上傳

```bash
curl -sS https://<base_url>/v1/images/edits \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -F "model=gpt-image-2.5-sunburst-m" \
  -F "prompt=把背景換成純白色，保留主體細節" \
  -F "size=1024x1024" \
  -F "quality=low" \
  -F "image=@input.png;type=image/png"
```

帶蒙版：

```bash
curl -sS https://<base_url>/v1/images/edits \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -F "model=gpt-image-2.5-sunburst-m" \
  -F "prompt=把蒙版區域替換成藍色圓形" \
  -F "size=1024x1024" \
  -F "quality=low" \
  -F "image=@input.png;type=image/png" \
  -F "mask=@mask.png;type=image/png"
```

| 參數 | 類型 | 必填 | 說明 |
| --- | --- | :---: | --- |
| `image` | file | 是 | 待編輯圖片，png/jpeg/webp；可重複該字段傳多張參考圖 |
| `mask` | file | 否 | PNG，帶 alpha 通道；透明區域爲可編輯區域，尺寸必須與 `image` 一致 |
| `prompt` | string | 是 | 編輯指令 |
| `n` | integer | 否 | 生成張數 |
| `size` / `quality` / `background` / `output_format` / `output_compression` / `moderation` | — | 否 | 同文生圖 |

::: warning
GPT Image 上游不支援 `input_fidelity`，傳了會返回 400。不需要蒙版時直接省略 `mask`，不要傳空文件或空值。
:::

### JSON + `image_url`

GPT Image 2.5 和 2 支援用 JSON 傳參考圖，圖片需要是上游可直接訪問的公網 URL：

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

- `prompt` 必須放在頂層。
- 參考圖寫成 `images: [{"image_url": "..."}]`，不要寫成 `images: ["https://..."]`。
- 不要用頂層 `image_url` 或 `image` 字段。
- 需要鑑權的圖片請先下載，再用 multipart 上傳。

## 各系列差異

| 系列 | 差異 |
| --- | --- |
| GPT Image 2.5 | 文生圖 + 編輯；`size` 只接受像素值；`quality`、`background`、`moderation`、`output_compression` 可用；`output_format=webp` 在已驗證型號上會 400，建議使用 png/jpeg；固定返回 `b64_json`；支援 `stream` + `partial_images` 流式預覽。 |
| GPT Image 2 | 文生圖 + 編輯；`size` 規則與 2.5 一致；支援 JSON `image_url` 編輯；固定返回 `b64_json`。 |
| GPT Image 1.5 | 文生圖 + 編輯；參數以 OpenAI Images 標準爲主，具體支援以模型廣場爲準。 |
| Gemini Image | 文生圖 + 編輯；適合需要 Gemini 圖像理解/生成能力的場景；尺寸和比例寫法以上游支援爲準。 |
| Qwen Image | 文生圖 + 編輯；站內固定 `output_format=png`、`response_format=b64_json`；客戶端傳 `jpeg` 也可能被覆蓋爲 PNG。 |
| Wan Image | 文生圖；支援 1K/2K/4K 和多種比例，單次最多 4 張；不支援編輯。 |
| MOSS Image | 文生圖；支援 512–2048 分辨率和批量生成；站內固定 30 步採樣、默認 `1024x1024`、返回 `b64_json`；不支援編輯。 |

## 代碼示例

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
    prompt="一隻戴宇航頭盔的橘貓，扁平插畫風格，純白背景",
    size="1024x1024",
    quality="low",
    n=1,
)

with open("out.png", "wb") as f:
    f.write(base64.b64decode(resp.data[0].b64_json))
```

### 圖像編輯（Python）

```python
with open("input.png", "rb") as image:
    resp = client.images.edit(
        model="gpt-image-2.5-sunburst-m",
        image=image,
        prompt="把背景換成純白色，保留主體細節",
        size="1024x1024",
        quality="low",
    )

with open("edited.png", "wb") as f:
    f.write(base64.b64decode(resp.data[0].b64_json))
```

### 文生圖（curl + jq 解碼）

```bash
curl -sS https://<base_url>/v1/images/generations \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2.5-flare-m",
    "prompt": "一隻戴宇航頭盔的橘貓，扁平插畫風格，純白背景",
    "size": "1024x1024",
    "quality": "low",
    "n": 1,
    "output_format": "png"
  }' \
  | jq -r '.data[0].b64_json' \
  | base64 -d > out.png
```

## 常見問題

### Q1: 返回 400 `Invalid request parameters`？

優先檢查：

- `size` 是否用了 GPT Image 不接受的寫法（`1K`、`16:9`、`1920x1080`）。
- 是否傳了 `output_format=webp` 或 `input_fidelity`。
- JSON 編輯是否把 `prompt` 放在了頂層、參考圖是否寫成 `images: [{"image_url": "..."}]`。
- `mask` 是否與 `image` 同尺寸的 PNG。

上游對這類錯誤通常只返回通用文案，需要逐項排除。

### Q2: 爲什麼響應裏沒有 URL？

GPT Image、Qwen Image、MOSS Image 固定返回 `data[].b64_json`，需要自行解碼。不要依賴 `response_format=url`。

### Q3: 調用超時？

首次生成通常需要 10–60 秒，高質量、大尺寸會更慢。客戶端超時建議不低於 60 秒；流式預覽只能提前看到中間圖，最終仍需等待完成事件。

### Q4: `n` 可以傳多大？

服務端對 `n` 有上限保護，但每個模型的實際上限更小。GPT Image 建議 1–10；Wan 2.7 單次最多 4 張；超出模型限制可能返回 400 或只返回部分結果。

### Q5: 模型名帶 `-m`、`-b` 有什麼區別？

它們是不同站內模型，可能對應不同上游和可用分組。例如 `gpt-image-2.5-flare-m` 和 `gpt-image-2.5-flare-b` 不是同一個模型，不能互相替代。以模型廣場中的說明爲準。
