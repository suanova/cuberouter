# Anthropic 協議

> 使用 Anthropic Messages 格式調用 Claude 及其他兼容模型。

Anthropic 協議適合 Claude 系列、Claude Code，以及需要 Messages 原生格式的應用。非 Claude 模型也有一部分支援該協議，網關會把 Messages 請求轉換爲對應上游的格式。

## 端點

| 用途 | 方法與路徑 |
| --- | --- |
| Messages | `POST /v1/messages` |
| 統計 token | `POST /v1/messages/count_tokens` |

請求頭：

```http
x-api-key: sk-xxxxxxxx
anthropic-version: 2023-06-01
Content-Type: application/json
```

也支援 `Authorization: Bearer sk-xxxxxxxx`。`anthropic-version` 建議保留，部分客戶端會校驗。

## 支援的模型

| 系列 | 模型 |
| --- | --- |
| Claude | `claude-fabel-5-j`、`claude-fable-5`、`claude-fable-5-1`、`claude-fable-5-1-j`、`claude-haiku-5-5`、`claude-haiku-5-5-j`、`claude-opus-4-6-j`、`claude-opus-4-7-j`、`claude-opus-4-8-j`、`claude-opus-5`、`claude-opus-5-5`、`claude-opus-5-5-j`、`claude-opus-5-j`、`claude-sonnet-4-6-j`、`claude-sonnet-5`、`claude-sonnet-5-5`、`claude-sonnet-5-5-j` |
| DeepSeek | `deepseek-flash`、`deepseek-v4-1-flash-261017-h`、`deepseek-v4-flash-0731`、`deepseek-v4-flash-0731-a`、`deepseek-v4-flash-a`、`deepseek-v4-pro`、`deepseek-v4-pro-0813`、`deepseek-v4-pro-a`、`deepseek-v4.1-flash`、`deepseek-v4.1-flash-a` |
| Kimi | `kimi-k2.5-a`、`kimi-k2.6-a`、`kimi-k2.7-code`、`kimi-k2.7-code-a`、`kimi-k3`、`kimi-k3-1-pro-h`、`kimi-k3-a`、`kimi-k3-b`、`kimi-k3-j` |
| Qwen | `qwen3.6-plus`、`qwen3.7-max`、`qwen3.7-plus`、`qwen3.7-plus-a`、`qwen3.8-max`、`qwen3.8-max-a` |
| GLM | `glm-5-a`、`glm-5.1-a`、`glm-5.2`、`glm-5.2-a`、`glm-5.3`、`glm-5.3-z` |
| Doubao | `doubao-seed-2-1-pro-h`、`doubao-seed-2-1-turbo-h` |
| GPT | `gpt-5.4-m`、`gpt-5.5-m`、`gpt-5.6-luna-m`、`gpt-5.6-sol`、`gpt-5.6-sol-m`、`gpt-5.6-terra`、`gpt-5.6-terra-m`、`gpt-6-astra`、`gpt-6-astra-m`、`gpt-6-luna-m`、`gpt-6-sol-m`、`gpt-6.1-sol` |
| Gemini | `gemini-3.1-pro-preview`、`gemini-3.5-flash-b`、`gemini-3.7-flash-b`、`gemini-3.8-flash`、`gemini-3.8-flash-b` |

> 非 Claude 模型通過網關轉換支援 Messages 協議，參數支援取決於上游能力。需要最完整的功能時，優先使用該模型的原生協議。
> 模型可用性受金鑰分組限制，實際可見型號以模型廣場爲準。

## 請求

```json
{
  "model": "claude-opus-5-5-j",
  "max_tokens": 1024,
  "system": "你是一個簡潔的中文助手。",
  "messages": [
    {"role": "user", "content": "你好"}
  ],
  "temperature": 0.7,
  "stream": false
}
```

常用字段：

| 字段 | 類型 | 必填 | 說明 |
| --- | --- | :---: | --- |
| `model` | string | 是 | 站內模型 ID |
| `max_tokens` | integer | 是 | 輸出上限；必須大於 `thinking.budget_tokens` |
| `messages` | array | 是 | 對話消息 |
| `system` | string / array | 否 | 系統提示 |
| `temperature` | number | 否 | 採樣溫度 |
| `top_p` | number | 否 | 核採樣 |
| `top_k` | integer | 否 | Top-K |
| `stop_sequences` | string[] | 否 | 停止詞 |
| `stream` | boolean | 否 | 是否流式 |
| `tools` | array | 否 | 工具定義 |
| `tool_choice` | object | 否 | 工具選擇策略 |
| `thinking` | object | 否 | 思考配置 |
| `metadata` | object | 否 | 業務元數據，如 `user_id` |
| `output_config` | object | 否 | 輸出配置，部分模型用於指定 `effort` |

### 消息內容

`messages[].content` 可以是字符串，也可以是內容塊數組：

```json
{
  "role": "user",
  "content": [
    {"type": "text", "text": "描述這張圖片"},
    {
      "type": "image",
      "source": {
        "type": "base64",
        "media_type": "image/jpeg",
        "data": "<base64>"
      }
    }
  ]
}
```

常用內容塊：

| 類型 | 說明 |
| --- | --- |
| `text` | 純文本 |
| `image` | 圖片，`source.type` 可以是 `base64` 或 `url` |
| `tool_use` | 模型發起的工具調用，出現在 assistant 消息中 |
| `tool_result` | 工具執行結果，出現在 user 消息中，需要 `tool_use_id` |
| `thinking` | 思考內容，出現在支援思考的模型響應中 |

### 工具調用

```json
{
  "tools": [{
    "name": "get_weather",
    "description": "查詢城市天氣",
    "input_schema": {
      "type": "object",
      "properties": {"city": {"type": "string"}},
      "required": ["city"]
    }
  }],
  "tool_choice": {"type": "auto"}
}
```

模型返回 `stop_reason="tool_use"` 時，從 `content` 中取出 `tool_use` 塊，執行後把結果作爲 `tool_result` 放回下一條 user 消息。

### 思考

```json
{
  "thinking": {"type": "enabled", "budget_tokens": 2048},
  "max_tokens": 4096
}
```

- `budget_tokens` 必須小於 `max_tokens`。
- 關閉思考：`{"type": "disabled"}`。
- 思考內容以 `thinking` 塊返回；部分模型還會返回 `signature`，多輪對話時應原樣帶回。

## 響應

```json
{
  "id": "msg_...",
  "type": "message",
  "role": "assistant",
  "model": "claude-opus-5-5-j",
  "content": [
    {"type": "text", "text": "你好！有什麼可以幫你的？"}
  ],
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 20,
    "output_tokens": 10
  }
}
```

- `content` 是內容塊數組，可能同時包含 `thinking`、`text`、`tool_use`。
- `stop_reason` 常見值：`end_turn`、`max_tokens`、`stop_sequence`、`tool_use`。
- `usage` 中可能包含緩存相關字段（如 `cache_read_input_tokens`），以響應爲準。

## 流式

```bash
curl -N https://<base_url>/v1/messages \
  -H "x-api-key: $CUBEROUTER_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-opus-5-5-j",
    "max_tokens": 512,
    "messages": [{"role": "user", "content": "寫一段產品介紹"}],
    "stream": true
  }'
```

常見事件順序：

```text
event: message_start
data: {"type":"message_start","message":{...}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"你"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":10}}

event: message_stop
data: {"type":"message_stop"}
```

工具調用時，`content_block_delta` 的 `delta.type` 爲 `input_json_delta`，需要把 `partial_json` 拼接成完整 JSON。

## 各系列差異

| 系列 | 差異與建議 |
| --- | --- |
| Claude | 原生協議，`thinking`、工具、圖片輸入支援最完整。建議 Claude 型號優先使用該協議。 |
| DeepSeek | 網關會把 Messages 請求轉換爲上游 Chat。`thinking` 支援 `enabled` / `disabled`；`output_config.effort` 可控制思考強度；`max_tokens` 生效。 |
| Kimi | K3 系列支援思考開關和工具調用；K2.7 Code 偏編程場景。 |
| Qwen / GLM / Doubao | 通過轉換支援 Messages；工具調用和思考字段按上游能力生效。 |
| GPT | 部分 `-m` 型號支援 Messages 協議，但由網關轉換；`thinking` 等 Anthropic 專有字段可能不生效。需要完整功能時用 OpenAI 或 Responses 協議。 |
| Gemini | 部分型號支援 Messages 協議，由網關轉換；原生 Gemini 字段（如 `safetySettings`）需要走 Gemini 協議。 |

## Claude Code

Claude Code 使用 Anthropic 協議。核心配置：

```json
{
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "your_platform_api_key",
    "ANTHROPIC_BASE_URL": "https://<base_url>",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "claude-haiku-5-5-j",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "claude-sonnet-5-5-j",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "claude-opus-5-5-j"
  }
}
```

`ANTHROPIC_BASE_URL` 只填域名，不要帶 `/v1`。完整步驟見 [Claude Code](/zh-Hant/getting-started/claude-code)。

## 常見問題

### Q1: 返回 400 `max_tokens: field required`？

Anthropic 協議要求 `max_tokens`。即使模型有默認輸出上限，也建議顯式傳入。

### Q2: `thinking.budget_tokens` 報錯？

`budget_tokens` 必須小於 `max_tokens`，且部分模型只接受特定範圍。先把 `max_tokens` 調到大於預算值再試。

### Q3: 返回 404 `model not support`？

該模型沒有聲明 Anthropic 端點。對照上面的模型表，或改用 OpenAI / Gemini 協議。

### Q4: Claude Code 返回 401 或一直重試？

檢查 `ANTHROPIC_AUTH_TOKEN` 是否是站內金鑰、`ANTHROPIC_BASE_URL` 是否只填域名（不帶 `/v1`），以及金鑰分組是否包含所選 Claude 模型。

### Q5: `count_tokens` 返回的 token 數與實際用量不一致？

`count_tokens` 是估算接口，不同模型、不同內容塊的計算方式可能不同；實際 token 統計以響應中的 `usage` 爲準。
