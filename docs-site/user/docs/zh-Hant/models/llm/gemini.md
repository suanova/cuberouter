# Gemini 協議

> 使用 Gemini 原生 `generateContent` 格式調用 Gemini 系列模型。

Gemini 協議適合需要原生多模態輸入、`safetySettings`、`thinkingConfig` 或 Google SDK 兼容性的應用。站內 Gemini 系列大多同時支援 OpenAI 協議；需要 Gemini 專有字段時使用本協議。

## 端點

| 用途 | 方法與路徑 |
| --- | --- |
| 生成內容 | `POST /v1beta/models/{model}:generateContent` |
| 流式生成 | `POST /v1beta/models/{model}:streamGenerateContent?alt=sse` |
| 模型列表 | `GET /v1beta/models` |

認證方式（三選一）：

```http
x-goog-api-key: sk-xxxxxxxx
```

```http
Authorization: Bearer sk-xxxxxxxx
```

```text
POST /v1beta/models/gemini-3.8-flash-b:generateContent?key=sk-xxxxxxxx
```

## 支援的模型

| 模型 | 說明 |
| --- | --- |
| `gemini-3.1-flash-lite-m` | 輕量經濟檔，低延遲高吞吐 |
| `gemini-3.1-pro-preview` | Pro 預覽版，強推理與多模態，1M 上下文 |
| `gemini-3.1-pro-preview-m` | Pro 預覽版（`-m` 變體） |
| `gemini-3.5-flash-b` | Flash 輕量旗艦，低延遲多模態 |
| `gemini-3.7-flash-b` | Flash 輕量旗艦 |
| `gemini-3.8-flash` | Flash 輕量旗艦 |
| `gemini-3.8-flash-b` | Flash 輕量旗艦（`-b` 變體） |

> `gemini-3.1-flash-lite-m` 和 `gemini-3.1-pro-preview-m` 只支援 OpenAI 與 Gemini 兩種協議；其餘型號還支援 Responses / Anthropic，具體見 [OpenAI 協議](/zh-Hant/models/llm/openai) 與 [Anthropic 協議](/zh-Hant/models/llm/anthropic)。
> 模型可用性受金鑰分組限制，實際可見型號以模型廣場爲準。

## 請求

```json
{
  "contents": [{
    "role": "user",
    "parts": [{"text": "你好，介紹一下你自己"}]
  }],
  "systemInstruction": {
    "parts": [{"text": "你是一個簡潔的中文助手。"}]
  },
  "generationConfig": {
    "temperature": 0.7,
    "maxOutputTokens": 1024
  }
}
```

### 頂層字段

| 字段 | 類型 | 說明 |
| --- | --- | --- |
| `contents` | array | 對話內容，元素包含 `role` 和 `parts` |
| `systemInstruction` | object | 系統指令，也接受 snake_case 寫法 `system_instruction` |
| `generationConfig` | object | 生成參數 |
| `safetySettings` | array | 安全過濾設置 |
| `tools` | array | 工具定義 |
| `toolConfig` | object | 工具調用配置 |
| `cachedContent` | string | 緩存內容名稱，部分模型支援 |

`contents[].role` 常用 `user` 和 `model`。多輪對話時，把模型上一輪迴復作爲 `role: "model"` 的內容放回 `contents`。

### `parts`

| 字段 | 說明 |
| --- | --- |
| `text` | 文本 |
| `inlineData` | 內聯二進制，包含 `mimeType` 和 `data`（base64） |
| `fileData` | 文件引用，包含 `mimeType` 和 `fileUri` |
| `functionCall` | 模型發起的函數調用 |
| `functionResponse` | 函數執行結果 |
| `thought` | 是否爲思考內容 |
| `thoughtSignature` | 思考簽名，多輪/工具調用時應原樣帶回 |
| `executableCode` / `codeExecutionResult` | 代碼執行工具的結果 |
| `videoMetadata` / `mediaResolution` | 影片元數據和媒體分辨率 |

圖片輸入示例：

```json
{
  "contents": [{
    "role": "user",
    "parts": [
      {"text": "描述這張圖片"},
      {"inlineData": {"mimeType": "image/jpeg", "data": "<base64>"}}
    ]
  }]
}
```

大文件可以用 `fileData`：

```json
{"fileData": {"mimeType": "image/jpeg", "fileUri": "https://generativelanguage.googleapis.com/v1beta/files/..."}}
```

`fileUri` 填 Gemini File API 返回的文件 URI，或上游明確支援的公網文件地址。`fileData` 的字段名使用 camelCase（`mimeType`、`fileUri`）。

### `generationConfig`

| 字段 | 類型 | 說明 |
| --- | --- | --- |
| `temperature` | number | 採樣溫度 |
| `topP` | number | 核採樣 |
| `topK` | number | Top-K |
| `maxOutputTokens` | integer | 輸出上限 |
| `candidateCount` | integer | 候選數量 |
| `stopSequences` | string[] | 停止詞 |
| `responseMimeType` | string | 例如 `application/json` |
| `responseSchema` | object | JSON Schema |
| `presencePenalty` | number | 存在懲罰 |
| `frequencyPenalty` | number | 頻率懲罰 |
| `seed` | integer | 隨機種子 |
| `responseModalities` | string[] | 輸出模態，如 `["TEXT"]`；支援圖像輸出的模型可加 `"IMAGE"` |
| `thinkingConfig` | object | 思考配置 |
| `mediaResolution` | string | 媒體分辨率 |
| `speechConfig` / `imageConfig` | object | 語音/圖像專用配置 |

### `thinkingConfig`

```json
{
  "generationConfig": {
    "thinkingConfig": {
      "thinkingBudget": 2048,
      "includeThoughts": true
    }
  }
}
```

| 字段 | 說明 |
| --- | --- |
| `thinkingBudget` | 思考 token 預算；不同模型允許範圍不同 |
| `thinkingLevel` | 思考檔位，部分 Gemini 3 模型使用；與 `thinkingBudget` 的可用性以模型爲準 |
| `includeThoughts` | 是否在響應中返回思考內容 |

### 安全設置

```json
{
  "safetySettings": [
    {"category": "HARM_CATEGORY_HARASSMENT", "threshold": "BLOCK_MEDIUM_AND_ABOVE"},
    {"category": "HARM_CATEGORY_HATE_SPEECH", "threshold": "BLOCK_MEDIUM_AND_ABOVE"}
  ]
}
```

類別和閾值以模型支援爲準。被安全策略攔截時，響應可能包含 `promptFeedback.blockReason` 或 `finishReason: "SAFETY"`。

### 工具調用

```json
{
  "tools": [{
    "functionDeclarations": [{
      "name": "get_weather",
      "description": "查詢城市天氣",
      "parameters": {
        "type": "object",
        "properties": {"city": {"type": "string"}},
        "required": ["city"]
      }
    }]
  }],
  "toolConfig": {
    "functionCallingConfig": {"mode": "AUTO"}
  }
}
```

`mode` 可取 `AUTO`、`ANY`、`NONE`；`ANY` 時可用 `allowedFunctionNames` 限定可調用的函數。

## 響應

```json
{
  "candidates": [{
    "content": {
      "role": "model",
      "parts": [{"text": "你好！有什麼可以幫你的？"}]
    },
    "finishReason": "STOP"
  }],
  "usageMetadata": {
    "promptTokenCount": 20,
    "candidatesTokenCount": 10,
    "totalTokenCount": 30
  }
}
```

- `candidates[0].content.parts` 是輸出內容，可能包含文本、函數調用或思考內容。
- `finishReason` 常見值：`STOP`、`MAX_TOKENS`、`SAFETY`、`RECITATION`。
- `usageMetadata` 可能包含 `cachedContentTokenCount`、`thoughtsTokenCount` 等字段。
- 請求被攔截時可能沒有 `candidates`，需要檢查 `promptFeedback`。

## 流式

```bash
curl -N "https://<base_url>/v1beta/models/gemini-3.8-flash-b:streamGenerateContent?alt=sse" \
  -H "x-goog-api-key: $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [{"role": "user", "parts": [{"text": "寫一段產品介紹"}]}]
  }'
```

每個 `data:` 是一個 `GenerateContentResponse`：

```text
data: {"candidates":[{"content":{"role":"model","parts":[{"text":"你"}]}}]}

data: {"candidates":[{"content":{"role":"model","parts":[{"text":"好"}]}}],"usageMetadata":{...}}
```

客戶端需要拼接 `parts[].text`；最後一個 chunk 通常包含完整的 `usageMetadata`。

## 各系列差異

| 模型 | 差異與建議 |
| --- | --- |
| `gemini-3.8-flash-b` / `gemini-3.8-flash` | 同時支援 OpenAI、Responses、Anthropic、Gemini 四種協議；原生 Gemini 字段最完整。 |
| `gemini-3.7-flash-b` / `gemini-3.5-flash-b` | 四種協議均支援；適合高併發、低延遲場景。 |
| `gemini-3.1-pro-preview` | 四種協議均支援；適合複雜推理和長文檔。 |
| `gemini-3.1-pro-preview-m` | 只支援 OpenAI 和 Gemini；需要 `safetySettings` 等原生字段時用 Gemini 協議。 |
| `gemini-3.1-flash-lite-m` | 只支援 OpenAI 和 Gemini；經濟檔，適合高吞吐任務。 |

## 常見問題

### Q1: 返回 404 或 `model not found`？

確認模型 ID 與模型廣場一致。Gemini 協議只支援上表中的型號；其他 Gemini 圖像模型（如 `gemini-3.1-flash-image-m`）通過 [圖像生成](/zh-Hant/models/image) 接口調用，不走 `generateContent`。

### Q2: 流式沒有返回？

`streamGenerateContent` 需要 `alt=sse`；不帶該參數時返回的是 JSON 數組流，不是 SSE。

### Q3: `inlineData` 和 `fileData` 怎麼選？

- 小文件或 base64 數據用 `inlineData`。
- 已上傳的文件用 `fileData`；`fileUri` 填 Gemini File API 返回的文件 URI，或上游明確支援的公網文件地址，字段名必須是 `mimeType` / `fileUri`。
- 不要用 OpenAI 的 `image_url` 格式調用 Gemini 協議。

### Q4: 返回 `finishReason: "SAFETY"` 沒有內容？

請求被安全策略攔截。可以調整 `safetySettings`（如果模型允許），或修改提示詞後重試。

### Q5: `thinkingBudget` 和 `thinkingLevel` 能同時用嗎？

不要同時傳。不同 Gemini 3 型號支援的字段不同，按模型說明選擇其一；不確定時先用 `thinkingBudget` 或留空走默認。
