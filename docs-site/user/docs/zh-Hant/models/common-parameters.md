# 通用參數

> 各協議共享的字段語義、多模態輸入方式和錯誤處理。協議專有字段見各協議頁面。

不同協議的字段名不完全一樣，但表達的是同一件事。下表是常見概念在四種文本協議中的對應關係：

| 概念 | OpenAI Chat | OpenAI Responses | Anthropic Messages | Gemini |
| --- | --- | --- | --- | --- |
| 模型 | `model` | `model` | `model` | 路徑中的 `{model}` |
| 對話內容 | `messages` | `input` | `messages` | `contents` |
| 系統提示 | `messages[role=system]` | `instructions` | `system` | `systemInstruction` |
| 輸出上限 | `max_tokens` / `max_completion_tokens` | `max_output_tokens` | `max_tokens`（必填） | `generationConfig.maxOutputTokens` |
| 溫度 | `temperature` | `temperature` | `temperature` | `generationConfig.temperature` |
| 核採樣 | `top_p` | `top_p` | `top_p` | `generationConfig.topP` |
| Top-K | `top_k`（部分模型） | — | `top_k` | `generationConfig.topK` |
| 停止詞 | `stop` | — | `stop_sequences` | `generationConfig.stopSequences` |
| 流式 | `stream` | `stream` | `stream` | `:streamGenerateContent` |
| 工具調用 | `tools` / `tool_choice` | `tools` / `tool_choice` | `tools` / `tool_choice` | `tools` / `toolConfig` |
| 結構化輸出 | `response_format` | `text.format` | `output_config` / `output_format` | `generationConfig.responseMimeType` / `responseSchema` |
| 思考控制 | `reasoning_effort` / `thinking` / `enable_thinking` | `reasoning.effort` | `thinking` | `generationConfig.thinkingConfig` |
| 返回用量 | `usage` | `usage` | `usage` | `usageMetadata` |

## 認證

所有協議使用同一套 API 金鑰。常用請求頭：

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

| 協議 | 可選認證方式 |
| --- | --- |
| OpenAI | `Authorization: Bearer` |
| Anthropic | `Authorization: Bearer` 或 `x-api-key` |
| Gemini | `Authorization: Bearer`、`x-goog-api-key` 或 URL 參數 `?key=` |
| 圖像 / 影片 / 音頻 | `Authorization: Bearer` |

## 通用字段

### `model`

必填。使用模型廣場中的完整站內模型 ID。帶後綴的型號（`-a`、`-b`、`-m`、`-h`、`-j` 等）是不同模型，不能互相替代。

### `stream`

是否流式返回，默認 `false`。開啓後響應爲 SSE：

```http
Content-Type: text/event-stream
```

不同協議的流式事件格式見各協議頁面。注意：流式開啓後，客戶端應逐塊讀取，不要等待完整響應體。

### 輸出上限

| 協議 | 字段 | 說明 |
| --- | --- | --- |
| OpenAI Chat | `max_tokens` 或 `max_completion_tokens` | 二選一，不能同時傳；GPT-6 系列建議直接用 `max_completion_tokens` |
| OpenAI Responses | `max_output_tokens` | 包含可見輸出和推理 token |
| Anthropic | `max_tokens` | **必填**，且必須大於 `thinking.budget_tokens` |
| Gemini | `generationConfig.maxOutputTokens` | 包含可見輸出和思考 token |

輸出上限只限制生成部分，不限制輸入長度。推理模型如果上限太小，token 可能全部消耗在思考上，導致 `content` 爲空；排查時先把上限調到至少 512，再逐步調整。

### 採樣參數

| 字段 | 常見範圍 | 說明 |
| --- | --- | --- |
| `temperature` | 0–2 | 越低越確定，越高越發散；部分推理模型在思考模式下不使用該參數 |
| `top_p` | 0–1 | 與 `temperature` 二選一調整即可，通常不要同時大幅修改 |
| `top_k` | 正整數 | Anthropic、Gemini 和部分開源模型支援；OpenAI 協議下模型支援情況不一致 |
| `stop` / `stop_sequences` | 字符串或數組 | 命中後停止生成；不同模型允許的條數不同，超出可能返回 400 |
| `n` / `candidateCount` | 正整數 | 返回多個候選；很多推理模型只返回 1 個，傳更大的值可能被忽略 |

### 隨機性與復現

- `seed`：部分模型支援固定隨機種子，但不同上游的復現保證不同；圖像模型通常支援得更好。
- `temperature=0` 也不保證完全確定，尤其在多卡推理或上游有批處理時。

### 結構化輸出

| 協議 | 字段 | 常見取值 |
| --- | --- | --- |
| OpenAI Chat | `response_format` | `{"type":"text"}`、`{"type":"json_object"}`、`{"type":"json_schema",...}` |
| OpenAI Responses | `text.format` | `{"type":"text"}`、`{"type":"json_schema",...}` |
| Anthropic | `output_config` / `output_format` | 以模型支援爲準 |
| Gemini | `responseMimeType` + `responseSchema` | `application/json` + JSON Schema |

使用 `json_object` 時，通常需要提示詞裏出現 "json" 字樣；使用 `json_schema` 時，schema 會約束輸出結構。上游不支援時可能返回 400 或靜默忽略，建議先用小請求驗證。

### 工具調用

各協議都支援函數工具，但字段形態不同：

```jsonc
// OpenAI Chat
{
  "tools": [{
    "type": "function",
    "function": {
      "name": "get_weather",
      "description": "查詢天氣",
      "parameters": {"type": "object", "properties": {"city": {"type": "string"}}}
    }
  }],
  "tool_choice": "auto"
}
```

```jsonc
// Anthropic
{
  "tools": [{
    "name": "get_weather",
    "description": "查詢天氣",
    "input_schema": {"type": "object", "properties": {"city": {"type": "string"}}}
  }],
  "tool_choice": {"type": "auto"}
}
```

```jsonc
// Gemini
{
  "tools": [{
    "functionDeclarations": [{
      "name": "get_weather",
      "description": "查詢天氣",
      "parameters": {"type": "object", "properties": {"city": {"type": "string"}}}
    }]
  }],
  "toolConfig": {"functionCallingConfig": {"mode": "AUTO"}}
}
```

`tool_choice` 常見取值：

- `auto`：模型自行決定是否調用工具。
- `none`：禁止調用工具。
- `required` / `any`：強制調用至少一個工具。
- 指定工具對象/名稱：強制調用指定工具。

### 思考（推理）控制

不同廠商的字段名不同，網關會盡量轉換，但不保證跨協議完全等價：

| 系列 | 常用字段 | 說明 |
| --- | --- | --- |
| GPT | `reasoning_effort` | `low` / `medium` / `high` 等；Responses 協議使用 `reasoning.effort` |
| DeepSeek | `thinking`、`reasoning_effort` | `thinking` 常用 `{"type":"enabled"}` / `{"type":"disabled"}` |
| Claude | `thinking` | `{"type":"enabled","budget_tokens":1024}`；需要小於 `max_tokens` |
| Gemini | `generationConfig.thinkingConfig` | `thinkingBudget`、`thinkingLevel`、`includeThoughts` |
| Qwen | `enable_thinking`、`thinking_budget` | 具體型號支援情況不同 |
| GLM / Doubao | `thinking` | 常見 `{"type":"enabled"}` / `{"type":"disabled"}` |
| Kimi | `thinking` / `enable_thinking` | K3、K2.8 等型號支援思考開關 |
| MiniMax | `reasoning_split` | 是否把思考內容分離返回 |

如果模型不支援某個思考字段，可能返回 400，也可能靜默忽略。判斷是否真的生效，看響應中的 reasoning/thinking 內容或 `usage` 裏的 reasoning token。

### 其他常用字段

| 字段 | 說明 |
| --- | --- |
| `frequency_penalty` / `presence_penalty` | 降低重複或鼓勵新話題；很多推理模型不支援 |
| `logprobs` / `top_logprobs` | 返回 token 對數概率；需要同時開啓 |
| `logit_bias` | 調整指定 token 的概率；支援度因模型而異 |
| `user` | 業務側用戶標識，用於風控；不是登入用戶名 |
| `metadata` | 廠商自定義參數或業務標籤，不同模型含義不同 |
| `prompt_cache_key` | 提示緩存路由鍵，支援的上游可提高緩存命中 |
| `service_tier` | 服務等級；默認會被過濾，只有站點開啓後才透傳 |
| `store` | 是否允許上游存儲請求/響應；默認允許，可通過站點設置關閉 |
| `safety_identifier` | 安全標識；默認過濾，需要站點開啓 |

## 多模態輸入

### OpenAI Chat

`messages[].content` 可以是字符串，也可以是內容塊數組：

```json
{
  "role": "user",
  "content": [
    {"type": "text", "text": "描述這張圖片"},
    {"type": "image_url", "image_url": {"url": "https://example.com/cat.jpg", "detail": "high"}}
  ]
}
```

| 內容塊 | 字段 | 說明 |
| --- | --- | --- |
| 文本 | `{"type":"text","text":"..."}` | 純文本 |
| 圖片 | `{"type":"image_url","image_url":{"url":"...","detail":"high"}}` | `url` 支援公網 URL 或 `data:image/...;base64,...` |
| 音頻 | `{"type":"input_audio","input_audio":{"data":"<base64>","format":"wav"}}` | 部分模型支援 |
| 文件 | `{"type":"file","file":{"file_data":"data:...","filename":"a.pdf"}}` | 部分模型支援 |
| 影片 | `{"type":"video_url","video_url":{"url":"https://..."}}` | 部分多模態模型支援 |

### OpenAI Responses

`input` 可以是字符串，也可以是輸入項數組：

```json
{
  "input": [
    {"role": "user", "content": [
      {"type": "input_text", "text": "描述這張圖片"},
      {"type": "input_image", "image_url": "https://example.com/cat.jpg"}
    ]}
  ]
}
```

常用輸入項：`input_text`、`input_image`、`input_file`，以及工具結果 `function_call_output`。

### Anthropic Messages

```json
{
  "role": "user",
  "content": [
    {"type": "text", "text": "描述這張圖片"},
    {
      "type": "image",
      "source": {"type": "base64", "media_type": "image/jpeg", "data": "<base64>"}
    }
  ]
}
```

也支援 URL 圖片：

```json
{"type": "image", "source": {"type": "url", "url": "https://example.com/cat.jpg"}}
```

### Gemini

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

`fileUri` 填 Gemini File API 返回的文件 URI，或上游明確支援的公網文件地址。`fileData` 的字段名使用 camelCase（`mimeType`、`fileUri`），與 Gemini 原生 API 一致。

## 流式響應

所有文本協議都支援 SSE。基本讀取方式：

```bash
curl -N https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-6-luna-m","messages":[{"role":"user","content":"寫一首短詩"}],"stream":true}'
```

| 協議 | 結束標誌 | 說明 |
| --- | --- | --- |
| OpenAI Chat | `data: [DONE]` | 每個 chunk 是增量 `choices[].delta` |
| OpenAI Responses | `response.completed` 事件 | 事件類型見 `event:` 字段 |
| Anthropic | `message_stop` 事件 | 還會有 `content_block_delta` 等事件 |
| Gemini | 連接關閉 | 每個 `data:` 是一個 `GenerateContentResponse` |

流式請求的用量可能出現在最後一個 chunk 或 `message_delta` / `response.completed` 事件中。OpenAI Chat 需要傳 `stream_options.include_usage=true` 纔會單獨返回 usage chunk；部分模型不支援該字段，usage 仍會附在最後一個帶 `finish_reason` 的 chunk 裏。

## 錯誤處理與重試

- **400**：參數錯誤或模型不支援該參數。按響應中的 `error.message` 修正；不要盲目重試。
- **401 / 403**：金鑰或分組權限問題。檢查金鑰是否有效、分組是否包含該模型。
- **404**：模型 ID 不存在，或該模型不支援當前協議。例如模型只支援 Anthropic 協議，卻調用了 `/v1/chat/completions`。
- **429**：限流或上游飽和。使用指數退避重試，並降低併發。
- **500 / 502 / 503**：上游或網關異常。可重試；持續失敗請聯繫支援，並附上請求時間、模型 ID 和響應中的 request id（如果有）。

::: warning
不要對 400 類錯誤做自動重試。參數錯誤通常不會因爲重試而恢復，反而會增加無效調用。
:::
