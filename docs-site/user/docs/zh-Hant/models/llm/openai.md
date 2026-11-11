# OpenAI 協議

> 使用 OpenAI Chat Completions 和 Responses 格式調用站內模型。

OpenAI 協議是兼容性最廣的文本協議。大多數文本模型都支援 `/v1/chat/completions`；較新的模型還支援 `/v1/responses`，適合 Codex CLI 和需要原生 Responses 能力的應用。

## 端點

| 用途 | 方法與路徑 |
| --- | --- |
| Chat Completions | `POST /v1/chat/completions` |
| 傳統文本補全 | `POST /v1/completions` |
| Responses | `POST /v1/responses` |
| Responses 壓縮 | `POST /v1/responses/compact` |
| 模型列表 | `GET /v1/models` |

請求頭：

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

## 支援的模型

### Chat Completions

| 系列 | 模型 |
| --- | --- |
| Claude | `claude-fabel-5-j`、`claude-fable-5`、`claude-fable-5-1`、`claude-fable-5-1-j`、`claude-haiku-5-5`、`claude-haiku-5-5-j`、`claude-opus-4-6-j`、`claude-opus-4-6-v1-aws-g`、`claude-opus-4-6-v1-aws-us`、`claude-opus-4-7-j`、`claude-opus-4-8-j`、`claude-opus-5`、`claude-opus-5-5`、`claude-opus-5-5-j`、`claude-opus-5-j`、`claude-sonnet-4-6-aws-g`、`claude-sonnet-4-6-aws-us`、`claude-sonnet-4-6-j`、`claude-sonnet-5`、`claude-sonnet-5-5`、`claude-sonnet-5-5-j` |
| GPT | `gpt-5.4-b`、`gpt-5.4-m`、`gpt-5.5-b`、`gpt-5.5-m`、`gpt-5.6-luna-b`、`gpt-5.6-luna-m`、`gpt-5.6-sol`、`gpt-5.6-sol-b`、`gpt-5.6-sol-m`、`gpt-5.6-terra`、`gpt-5.6-terra-b`、`gpt-5.6-terra-m`、`gpt-6-astra`、`gpt-6-astra-b`、`gpt-6-astra-m`、`gpt-6-luna`、`gpt-6-luna-b`、`gpt-6-luna-m`、`gpt-6-sol-b`、`gpt-6-sol-m`、`gpt-6.1-sol`、`gpt-6.1-sol-b` |
| Gemini | `gemini-3.1-flash-lite-m`、`gemini-3.1-pro-preview`、`gemini-3.1-pro-preview-m`、`gemini-3.5-flash-b`、`gemini-3.7-flash-b`、`gemini-3.8-flash`、`gemini-3.8-flash-b` |
| DeepSeek | `deepseek-flash`、`deepseek-v4-1-flash-261017-h`、`deepseek-v4-flash-0731`、`deepseek-v4-flash-0731-a`、`deepseek-v4-flash-a`、`deepseek-v4-pro`、`deepseek-v4-pro-0813`、`deepseek-v4-pro-a`、`deepseek-v4.1-flash`、`deepseek-v4.1-flash-a` |
| Kimi | `kimi-k2.5-a`、`kimi-k2.6-a`、`kimi-k2.7-code`、`kimi-k2.7-code-a`、`kimi-k2.8-preview-a`、`kimi-k3`、`kimi-k3-1-pro-h`、`kimi-k3-a`、`kimi-k3-b`、`kimi-k3-j` |
| Qwen | `qwen3.6-plus`、`qwen3.7-max`、`qwen3.7-plus`、`qwen3.7-plus-a`、`qwen3.8-max`、`qwen3.8-max-a` |
| GLM | `glm-5-a`、`glm-5.1-a`、`glm-5.2`、`glm-5.2-a`、`glm-5.3`、`glm-5.3-a`、`glm-5.3-z` |
| Doubao | `doubao-seed-2-1-pro-h`、`doubao-seed-2-1-turbo-h` |
| MiniMax | `minimax-m2.7-a`、`minimax-m3.1-flash-a` |

### Responses

| 系列 | 模型 |
| --- | --- |
| Claude | `claude-fable-5`、`claude-fable-5-1`、`claude-fable-5-1-j`、`claude-haiku-5-5`、`claude-haiku-5-5-j`、`claude-opus-5`、`claude-opus-5-5`、`claude-opus-5-5-j`、`claude-sonnet-5`、`claude-sonnet-5-5`、`claude-sonnet-5-5-j` |
| GPT | `gpt-5.4-b`、`gpt-5.4-m`、`gpt-5.5-b`、`gpt-5.5-m`、`gpt-5.6-luna-b`、`gpt-5.6-luna-m`、`gpt-5.6-sol`、`gpt-5.6-sol-b`、`gpt-5.6-sol-m`、`gpt-5.6-terra`、`gpt-5.6-terra-b`、`gpt-5.6-terra-m`、`gpt-6-astra`、`gpt-6-astra-b`、`gpt-6-astra-m`、`gpt-6-luna`、`gpt-6-luna-b`、`gpt-6-luna-m`、`gpt-6-sol-b`、`gpt-6-sol-m`、`gpt-6.1-sol`、`gpt-6.1-sol-b` |
| Gemini | `gemini-3.1-pro-preview`、`gemini-3.5-flash-b`、`gemini-3.7-flash-b`、`gemini-3.8-flash`、`gemini-3.8-flash-b` |
| DeepSeek | `deepseek-flash`、`deepseek-v4-1-flash-261017-h`、`deepseek-v4-flash-0731`、`deepseek-v4-flash-0731-a`、`deepseek-v4-flash-a`、`deepseek-v4-pro`、`deepseek-v4-pro-0813`、`deepseek-v4-pro-a`、`deepseek-v4.1-flash`、`deepseek-v4.1-flash-a` |
| Kimi | `kimi-k2.7-code`、`kimi-k3`、`kimi-k3-1-pro-h`、`kimi-k3-a`、`kimi-k3-b` |
| Qwen | `qwen3.6-plus`、`qwen3.7-max`、`qwen3.7-plus`、`qwen3.7-plus-a`、`qwen3.8-max`、`qwen3.8-max-a` |
| GLM | `glm-5.2`、`glm-5.2-a`、`glm-5.3`、`glm-5.3-z` |
| Doubao | `doubao-seed-2-1-pro-h`、`doubao-seed-2-1-turbo-h` |

> 上表來自站內模型元數據。模型可用性還受金鑰分組限制；實際可見型號以模型廣場爲準。

## Chat Completions

### 請求

```json
{
  "model": "gpt-6-luna-m",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "你好"}
  ],
  "temperature": 0.7,
  "max_completion_tokens": 1024,
  "stream": false
}
```

常用字段：

| 字段 | 類型 | 說明 |
| --- | --- | --- |
| `messages` | array | 對話消息，見下表 |
| `temperature` | number | 採樣溫度 |
| `top_p` | number | 核採樣 |
| `max_tokens` / `max_completion_tokens` | integer | 輸出上限，二選一 |
| `stream` | boolean | 是否流式 |
| `stream_options.include_usage` | boolean | 流式時是否單獨返回 usage |
| `stop` | string / string[] | 停止詞 |
| `response_format` | object | 文本或 JSON 輸出 |
| `tools` / `tool_choice` | array / string | 工具調用 |
| `parallel_tool_calls` | boolean | 是否允許並行工具調用 |
| `reasoning_effort` | string | 推理強度，支援情況因模型而異 |
| `seed` | number | 隨機種子 |

`messages[].role` 常用取值：

| role | 說明 |
| --- | --- |
| `system` | 系統提示 |
| `user` | 用戶輸入 |
| `assistant` | 模型歷史回覆 |
| `tool` | 工具執行結果，需要配合 `tool_call_id` |

### 響應

```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "created": 1793093555,
  "model": "gpt-6-luna-m",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "你好！有什麼可以幫你的？"
    },
    "finish_reason": "stop"
  }],
  "usage": {
    "prompt_tokens": 20,
    "completion_tokens": 10,
    "total_tokens": 30
  }
}
```

- `choices[].message.content` 是最終文本。
- 推理模型的思考內容可能在 `reasoning_content` 字段或 `content` 之前的獨立塊中返回；不同模型字段名可能不同。
- `usage` 中的 token 統計以響應爲準。

### 流式

```bash
curl -N https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-6-luna-m",
    "messages": [{"role": "user", "content": "寫一段產品介紹"}],
    "stream": true,
    "stream_options": {"include_usage": true}
  }'
```

每個 chunk 形如：

```text
data: {"id":"chatcmpl-...","choices":[{"index":0,"delta":{"content":"你"}}]}

data: {"id":"chatcmpl-...","choices":[{"index":0,"delta":{"content":"好"}}]}

data: [DONE]
```

`delta.content` 是增量文本；`delta.tool_calls` 是增量工具調用；最後一個帶 `finish_reason` 的 chunk 通常包含 usage。

### 工具調用

```bash
curl https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-6-luna-m",
    "messages": [{"role": "user", "content": "北京今天天氣怎麼樣？"}],
    "tools": [{
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "查詢城市天氣",
        "parameters": {
          "type": "object",
          "properties": {"city": {"type": "string"}},
          "required": ["city"]
        }
      }
    }],
    "tool_choice": "auto"
  }'
```

模型返回 `finish_reason="tool_calls"` 時，把 `tool_calls` 原樣帶回下一輪，並追加一條 `role="tool"`、`tool_call_id` 對應的消息。

### 圖片輸入

```bash
curl https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-6-astra-m",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "這張圖片裏有什麼？"},
        {"type": "image_url", "image_url": {"url": "https://example.com/cat.jpg", "detail": "high"}}
      ]
    }]
  }'
```

圖片 `url` 可以是公網 URL，也可以是 `data:image/jpeg;base64,...`。部分模型還支援影片和音頻輸入，見各系列說明。

## Responses API

Responses 是 OpenAI 的新一代文本接口，適合 Codex CLI、需要原生工具調用或多輪上下文管理的應用。

### 請求

```json
{
  "model": "gpt-6.1-sol",
  "instructions": "你是一個簡潔的中文助手。",
  "input": "用三句話介紹 CubeRouter。",
  "max_output_tokens": 512,
  "reasoning": {"effort": "medium"},
  "stream": false
}
```

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

常用字段：

| 字段 | 類型 | 說明 |
| --- | --- | --- |
| `input` | string / array | 輸入內容 |
| `instructions` | string | 系統指令 |
| `max_output_tokens` | integer | 輸出上限 |
| `reasoning.effort` | string | 思考強度 |
| `text.format` | object | 文本或 JSON Schema 輸出 |
| `tools` / `tool_choice` | array / object | 工具調用 |
| `previous_response_id` | string | 關聯上一輪響應，減少重複上下文 |
| `store` | boolean | 是否允許上游存儲 |
| `include` | array | 額外返回內容，例如推理摘要 |
| `stream` | boolean | 是否流式 |

### 響應

```json
{
  "id": "resp_...",
  "object": "response",
  "status": "completed",
  "model": "gpt-6.1-sol",
  "output": [{
    "type": "message",
    "role": "assistant",
    "content": [{"type": "output_text", "text": "CubeRouter 是一個 AI 網關。"}]
  }],
  "usage": {
    "input_tokens": 20,
    "output_tokens": 30,
    "total_tokens": 50
  }
}
```

### 流式事件

Responses 的流式響應使用命名事件，常見順序：

```text
event: response.created
data: {"type":"response.created","response":{...}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"你"}

event: response.output_text.done
data: {"type":"response.output_text.done","text":"你好"}

event: response.completed
data: {"type":"response.completed","response":{...}}
```

客戶端應以 `response.completed` 作爲結束標誌，並從該事件的 `response.usage` 讀取用量。

### Codex CLI

Codex 使用 Responses 協議。配置示例：

```toml
model_provider = "cuberouter"
model = "gpt-6.1-sol"

[model_providers.cuberouter]
name = "CubeRouter"
base_url = "https://<base_url>"
env_key = "CODEX_CUBEROUTER_API_KEY"
wire_api = "responses"
```

完整步驟見 [Codex](/zh-Hant/getting-started/codex)。

## Python（OpenAI SDK）

```python
import os

from openai import OpenAI

client = OpenAI(
    api_key=os.environ["CUBEROUTER_API_KEY"],
    base_url="https://<base_url>/v1",
)

# Chat Completions
chat = client.chat.completions.create(
    model="gpt-6-luna-m",
    messages=[{"role": "user", "content": "你好"}],
)
print(chat.choices[0].message.content)

# Responses
resp = client.responses.create(
    model="gpt-6.1-sol",
    input="用三句話介紹 CubeRouter。",
)
print(resp.output_text)
```

## 各系列差異

| 系列 | 差異與建議 |
| --- | --- |
| Claude | 建議使用 Anthropic 協議。通過 OpenAI 協議調用時，網關會把 Chat/Responses 請求轉換爲 Anthropic Messages；`thinking` 等 Anthropic 專有字段不保證生效。 |
| GPT | 原生支援。GPT-6 系列建議使用 `max_completion_tokens`；部分上游會把 `max_tokens` 自動搬到 `max_completion_tokens`。`reasoning_effort`、`verbosity` 按模型支援。 |
| Gemini | 通過 OpenAI 協議調用時由網關轉換。`safetySettings`、`thinkingConfig`、`responseModalities` 等 Gemini 原生字段需要走 Gemini 協議。 |
| DeepSeek | 支援 `thinking` 和 `reasoning_effort`。部分型號對 `frequency_penalty`、`presence_penalty`、`logit_bias`、`n`、`seed` 不生效或只返回 1 個候選；`stop` 可能有條數上限。 |
| Kimi | K3、K2.8 等型號支援思考開關；K2.8 Preview 還支援圖像/影片輸入。字段名以模型實際支援爲準，常見爲 `thinking`。 |
| Qwen | 支援 `enable_thinking`、`thinking_budget`（具體型號而定）；`vl_high_resolution_images` 用於提高圖片解析精度。 |
| GLM | 支援 `thinking` 開關；GLM-5.3 系列支援工具調用和長上下文。 |
| Doubao | 支援 `thinking` 開關；Seed 2.1 系列支援文本/圖片/影片輸入。 |
| MiniMax | 支援 `reasoning_split` 分離思考內容；部分型號支援提示緩存。 |

### 已驗證的 DeepSeek 差異

以 `deepseek-v4-1-flash-261017-h` 爲例：

- `thinking` 只接受 `{"type":"enabled"}` / `{"type":"disabled"}`；`auto`、`adaptive` 會 400。
- `reasoning_effort` 支援 `none`、`minimal`、`low`、`medium`、`high`、`xhigh`、`max`；`ultra` 會 400。
- `thinking.disabled` 優先於 `reasoning_effort`：關閉思考時即使傳 `high` 也不會思考。
- `max_tokens` 與 `max_completion_tokens` 不能同時傳。
- `stop` 最多 16 個，超出會 400。
- `frequency_penalty`、`presence_penalty`、`logit_bias`、`n`、`seed`、`user` 在該型號上不生效或無可觀測效果。
- `response_format=json_object` 時，提示詞必須包含 "json" 字樣。

## 常見問題

### Q1: 返回 404 `model not found` 或 `does not support`？

先確認模型 ID 與模型廣場完全一致，再確認該模型支援當前協議。例如只支援 Anthropic 協議的模型調用 `/v1/chat/completions` 會返回 404 或 400。還要確認金鑰分組包含該模型。

### Q2: 返回 400 `max_tokens and max_completion_tokens cannot be set at the same time`？

只傳其中一個。GPT-6 系列建議直接使用 `max_completion_tokens`。

### Q3: 推理模型返回空 `content`？

輸出上限太小，token 全部用在思考上。把 `max_tokens` / `max_completion_tokens` 調到至少 512，或關閉思考（如果模型支援）。

### Q4: 流式響應裏沒有 usage？

OpenAI Chat 需要傳 `stream_options.include_usage=true`；部分模型不支援該字段，usage 會附在最後一個帶 `finish_reason` 的 chunk 裏。Responses 協議從 `response.completed` 事件讀取 usage。

### Q5: Claude 模型應該用 OpenAI 協議還是 Anthropic 協議？

優先使用 Anthropic 協議，廠商專有字段支援最完整。已有 OpenAI 代碼可以繼續用 Chat Completions，網關會做轉換。

### Q6: `service_tier` 傳了沒有效果？

`service_tier` 默認會被網關過濾，只有站點開啓後才透傳。`safety_identifier` 同理。
