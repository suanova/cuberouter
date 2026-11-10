# 通用参数

> 各协议共享的字段语义、多模态输入方式和错误处理。协议专有字段见各协议页面。

不同协议的字段名不完全一样，但表达的是同一件事。下表是常见概念在四种文本协议中的对应关系：

| 概念 | OpenAI Chat | OpenAI Responses | Anthropic Messages | Gemini |
| --- | --- | --- | --- | --- |
| 模型 | `model` | `model` | `model` | 路径中的 `{model}` |
| 对话内容 | `messages` | `input` | `messages` | `contents` |
| 系统提示 | `messages[role=system]` | `instructions` | `system` | `systemInstruction` |
| 输出上限 | `max_tokens` / `max_completion_tokens` | `max_output_tokens` | `max_tokens`（必填） | `generationConfig.maxOutputTokens` |
| 温度 | `temperature` | `temperature` | `temperature` | `generationConfig.temperature` |
| 核采样 | `top_p` | `top_p` | `top_p` | `generationConfig.topP` |
| Top-K | `top_k`（部分模型） | — | `top_k` | `generationConfig.topK` |
| 停止词 | `stop` | — | `stop_sequences` | `generationConfig.stopSequences` |
| 流式 | `stream` | `stream` | `stream` | `:streamGenerateContent` |
| 工具调用 | `tools` / `tool_choice` | `tools` / `tool_choice` | `tools` / `tool_choice` | `tools` / `toolConfig` |
| 结构化输出 | `response_format` | `text.format` | `output_config` / `output_format` | `generationConfig.responseMimeType` / `responseSchema` |
| 思考控制 | `reasoning_effort` / `thinking` / `enable_thinking` | `reasoning.effort` | `thinking` | `generationConfig.thinkingConfig` |
| 返回用量 | `usage` | `usage` | `usage` | `usageMetadata` |

## 认证

所有协议使用同一套 API 密钥。常用请求头：

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

| 协议 | 可选认证方式 |
| --- | --- |
| OpenAI | `Authorization: Bearer` |
| Anthropic | `Authorization: Bearer` 或 `x-api-key` |
| Gemini | `Authorization: Bearer`、`x-goog-api-key` 或 URL 参数 `?key=` |
| 图像 / 视频 / 音频 | `Authorization: Bearer` |

## 通用字段

### `model`

必填。使用模型广场中的完整站内模型 ID。带后缀的型号（`-a`、`-b`、`-m`、`-h`、`-j` 等）是不同模型，不能互相替代。

### `stream`

是否流式返回，默认 `false`。开启后响应为 SSE：

```http
Content-Type: text/event-stream
```

不同协议的流式事件格式见各协议页面。注意：流式开启后，客户端应逐块读取，不要等待完整响应体。

### 输出上限

| 协议 | 字段 | 说明 |
| --- | --- | --- |
| OpenAI Chat | `max_tokens` 或 `max_completion_tokens` | 二选一，不能同时传；GPT-6 系列建议直接用 `max_completion_tokens` |
| OpenAI Responses | `max_output_tokens` | 包含可见输出和推理 token |
| Anthropic | `max_tokens` | **必填**，且必须大于 `thinking.budget_tokens` |
| Gemini | `generationConfig.maxOutputTokens` | 包含可见输出和思考 token |

输出上限只限制生成部分，不限制输入长度。推理模型如果上限太小，token 可能全部消耗在思考上，导致 `content` 为空；排查时先把上限调到至少 512，再逐步调整。

### 采样参数

| 字段 | 常见范围 | 说明 |
| --- | --- | --- |
| `temperature` | 0–2 | 越低越确定，越高越发散；部分推理模型在思考模式下不使用该参数 |
| `top_p` | 0–1 | 与 `temperature` 二选一调整即可，通常不要同时大幅修改 |
| `top_k` | 正整数 | Anthropic、Gemini 和部分开源模型支持；OpenAI 协议下模型支持情况不一致 |
| `stop` / `stop_sequences` | 字符串或数组 | 命中后停止生成；不同模型允许的条数不同，超出可能返回 400 |
| `n` / `candidateCount` | 正整数 | 返回多个候选；很多推理模型只返回 1 个，传更大的值可能被忽略 |

### 随机性与复现

- `seed`：部分模型支持固定随机种子，但不同上游的复现保证不同；图像模型通常支持得更好。
- `temperature=0` 也不保证完全确定，尤其在多卡推理或上游有批处理时。

### 结构化输出

| 协议 | 字段 | 常见取值 |
| --- | --- | --- |
| OpenAI Chat | `response_format` | `{"type":"text"}`、`{"type":"json_object"}`、`{"type":"json_schema",...}` |
| OpenAI Responses | `text.format` | `{"type":"text"}`、`{"type":"json_schema",...}` |
| Anthropic | `output_config` / `output_format` | 以模型支持为准 |
| Gemini | `responseMimeType` + `responseSchema` | `application/json` + JSON Schema |

使用 `json_object` 时，通常需要提示词里出现 "json" 字样；使用 `json_schema` 时，schema 会约束输出结构。上游不支持时可能返回 400 或静默忽略，建议先用小请求验证。

### 工具调用

各协议都支持函数工具，但字段形态不同：

```jsonc
// OpenAI Chat
{
  "tools": [{
    "type": "function",
    "function": {
      "name": "get_weather",
      "description": "查询天气",
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
    "description": "查询天气",
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
      "description": "查询天气",
      "parameters": {"type": "object", "properties": {"city": {"type": "string"}}}
    }]
  }],
  "toolConfig": {"functionCallingConfig": {"mode": "AUTO"}}
}
```

`tool_choice` 常见取值：

- `auto`：模型自行决定是否调用工具。
- `none`：禁止调用工具。
- `required` / `any`：强制调用至少一个工具。
- 指定工具对象/名称：强制调用指定工具。

### 思考（推理）控制

不同厂商的字段名不同，网关会尽量转换，但不保证跨协议完全等价：

| 系列 | 常用字段 | 说明 |
| --- | --- | --- |
| GPT | `reasoning_effort` | `low` / `medium` / `high` 等；Responses 协议使用 `reasoning.effort` |
| DeepSeek | `thinking`、`reasoning_effort` | `thinking` 常用 `{"type":"enabled"}` / `{"type":"disabled"}` |
| Claude | `thinking` | `{"type":"enabled","budget_tokens":1024}`；需要小于 `max_tokens` |
| Gemini | `generationConfig.thinkingConfig` | `thinkingBudget`、`thinkingLevel`、`includeThoughts` |
| Qwen | `enable_thinking`、`thinking_budget` | 具体型号支持情况不同 |
| GLM / Doubao | `thinking` | 常见 `{"type":"enabled"}` / `{"type":"disabled"}` |
| Kimi | `thinking` / `enable_thinking` | K3、K2.8 等型号支持思考开关 |
| MiniMax | `reasoning_split` | 是否把思考内容分离返回 |

如果模型不支持某个思考字段，可能返回 400，也可能静默忽略。判断是否真的生效，看响应中的 reasoning/thinking 内容或 `usage` 里的 reasoning token。

### 其他常用字段

| 字段 | 说明 |
| --- | --- |
| `frequency_penalty` / `presence_penalty` | 降低重复或鼓励新话题；很多推理模型不支持 |
| `logprobs` / `top_logprobs` | 返回 token 对数概率；需要同时开启 |
| `logit_bias` | 调整指定 token 的概率；支持度因模型而异 |
| `user` | 业务侧用户标识，用于风控；不是登录用户名 |
| `metadata` | 厂商自定义参数或业务标签，不同模型含义不同 |
| `prompt_cache_key` | 提示缓存路由键，支持的上游可提高缓存命中 |
| `service_tier` | 服务等级；默认会被过滤，只有站点开启后才透传 |
| `store` | 是否允许上游存储请求/响应；默认允许，可通过站点设置关闭 |
| `safety_identifier` | 安全标识；默认过滤，需要站点开启 |

## 多模态输入

### OpenAI Chat

`messages[].content` 可以是字符串，也可以是内容块数组：

```json
{
  "role": "user",
  "content": [
    {"type": "text", "text": "描述这张图片"},
    {"type": "image_url", "image_url": {"url": "https://example.com/cat.jpg", "detail": "high"}}
  ]
}
```

| 内容块 | 字段 | 说明 |
| --- | --- | --- |
| 文本 | `{"type":"text","text":"..."}` | 纯文本 |
| 图片 | `{"type":"image_url","image_url":{"url":"...","detail":"high"}}` | `url` 支持公网 URL 或 `data:image/...;base64,...` |
| 音频 | `{"type":"input_audio","input_audio":{"data":"<base64>","format":"wav"}}` | 部分模型支持 |
| 文件 | `{"type":"file","file":{"file_data":"data:...","filename":"a.pdf"}}` | 部分模型支持 |
| 视频 | `{"type":"video_url","video_url":{"url":"https://..."}}` | 部分多模态模型支持 |

### OpenAI Responses

`input` 可以是字符串，也可以是输入项数组：

```json
{
  "input": [
    {"role": "user", "content": [
      {"type": "input_text", "text": "描述这张图片"},
      {"type": "input_image", "image_url": "https://example.com/cat.jpg"}
    ]}
  ]
}
```

常用输入项：`input_text`、`input_image`、`input_file`，以及工具结果 `function_call_output`。

### Anthropic Messages

```json
{
  "role": "user",
  "content": [
    {"type": "text", "text": "描述这张图片"},
    {
      "type": "image",
      "source": {"type": "base64", "media_type": "image/jpeg", "data": "<base64>"}
    }
  ]
}
```

也支持 URL 图片：

```json
{"type": "image", "source": {"type": "url", "url": "https://example.com/cat.jpg"}}
```

### Gemini

```json
{
  "contents": [{
    "role": "user",
    "parts": [
      {"text": "描述这张图片"},
      {"inlineData": {"mimeType": "image/jpeg", "data": "<base64>"}}
    ]
  }]
}
```

大文件可以用 `fileData`：

```json
{"fileData": {"mimeType": "image/jpeg", "fileUri": "https://generativelanguage.googleapis.com/v1beta/files/..."}}
```

`fileUri` 填 Gemini File API 返回的文件 URI，或上游明确支持的公网文件地址。`fileData` 的字段名使用 camelCase（`mimeType`、`fileUri`），与 Gemini 原生 API 一致。

## 流式响应

所有文本协议都支持 SSE。基本读取方式：

```bash
curl -N https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-6-luna-m","messages":[{"role":"user","content":"写一首短诗"}],"stream":true}'
```

| 协议 | 结束标志 | 说明 |
| --- | --- | --- |
| OpenAI Chat | `data: [DONE]` | 每个 chunk 是增量 `choices[].delta` |
| OpenAI Responses | `response.completed` 事件 | 事件类型见 `event:` 字段 |
| Anthropic | `message_stop` 事件 | 还会有 `content_block_delta` 等事件 |
| Gemini | 连接关闭 | 每个 `data:` 是一个 `GenerateContentResponse` |

流式请求的用量可能出现在最后一个 chunk 或 `message_delta` / `response.completed` 事件中。OpenAI Chat 需要传 `stream_options.include_usage=true` 才会单独返回 usage chunk；部分模型不支持该字段，usage 仍会附在最后一个带 `finish_reason` 的 chunk 里。

## 错误处理与重试

- **400**：参数错误或模型不支持该参数。按响应中的 `error.message` 修正；不要盲目重试。
- **401 / 403**：密钥或分组权限问题。检查密钥是否有效、分组是否包含该模型。
- **404**：模型 ID 不存在，或该模型不支持当前协议。例如模型只支持 Anthropic 协议，却调用了 `/v1/chat/completions`。
- **429**：限流或上游饱和。使用指数退避重试，并降低并发。
- **500 / 502 / 503**：上游或网关异常。可重试；持续失败请联系支持，并附上请求时间、模型 ID 和响应中的 request id（如果有）。

::: warning
不要对 400 类错误做自动重试。参数错误通常不会因为重试而恢复，反而会增加无效调用。
:::
