# Anthropic 协议

> 使用 Anthropic Messages 格式调用 Claude 及其他兼容模型。

Anthropic 协议适合 Claude 系列、Claude Code，以及需要 Messages 原生格式的应用。非 Claude 模型也有一部分支持该协议，网关会把 Messages 请求转换为对应上游的格式。

## 端点

| 用途 | 方法与路径 |
| --- | --- |
| Messages | `POST /v1/messages` |
| 统计 token | `POST /v1/messages/count_tokens` |

请求头：

```http
x-api-key: sk-xxxxxxxx
anthropic-version: 2023-06-01
Content-Type: application/json
```

也支持 `Authorization: Bearer sk-xxxxxxxx`。`anthropic-version` 建议保留，部分客户端会校验。

## 支持的模型

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

> 非 Claude 模型通过网关转换支持 Messages 协议，参数支持取决于上游能力。需要最完整的功能时，优先使用该模型的原生协议。
> 模型可用性受密钥分组限制，实际可见型号以模型广场为准。

## 请求

```json
{
  "model": "claude-opus-5-5-j",
  "max_tokens": 1024,
  "system": "你是一个简洁的中文助手。",
  "messages": [
    {"role": "user", "content": "你好"}
  ],
  "temperature": 0.7,
  "stream": false
}
```

常用字段：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | :---: | --- |
| `model` | string | 是 | 站内模型 ID |
| `max_tokens` | integer | 是 | 输出上限；必须大于 `thinking.budget_tokens` |
| `messages` | array | 是 | 对话消息 |
| `system` | string / array | 否 | 系统提示 |
| `temperature` | number | 否 | 采样温度 |
| `top_p` | number | 否 | 核采样 |
| `top_k` | integer | 否 | Top-K |
| `stop_sequences` | string[] | 否 | 停止词 |
| `stream` | boolean | 否 | 是否流式 |
| `tools` | array | 否 | 工具定义 |
| `tool_choice` | object | 否 | 工具选择策略 |
| `thinking` | object | 否 | 思考配置 |
| `metadata` | object | 否 | 业务元数据，如 `user_id` |
| `output_config` | object | 否 | 输出配置，部分模型用于指定 `effort` |

### 消息内容

`messages[].content` 可以是字符串，也可以是内容块数组：

```json
{
  "role": "user",
  "content": [
    {"type": "text", "text": "描述这张图片"},
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

常用内容块：

| 类型 | 说明 |
| --- | --- |
| `text` | 纯文本 |
| `image` | 图片，`source.type` 可以是 `base64` 或 `url` |
| `tool_use` | 模型发起的工具调用，出现在 assistant 消息中 |
| `tool_result` | 工具执行结果，出现在 user 消息中，需要 `tool_use_id` |
| `thinking` | 思考内容，出现在支持思考的模型响应中 |

### 工具调用

```json
{
  "tools": [{
    "name": "get_weather",
    "description": "查询城市天气",
    "input_schema": {
      "type": "object",
      "properties": {"city": {"type": "string"}},
      "required": ["city"]
    }
  }],
  "tool_choice": {"type": "auto"}
}
```

模型返回 `stop_reason="tool_use"` 时，从 `content` 中取出 `tool_use` 块，执行后把结果作为 `tool_result` 放回下一条 user 消息。

### 思考

```json
{
  "thinking": {"type": "enabled", "budget_tokens": 2048},
  "max_tokens": 4096
}
```

- `budget_tokens` 必须小于 `max_tokens`。
- 关闭思考：`{"type": "disabled"}`。
- 思考内容以 `thinking` 块返回；部分模型还会返回 `signature`，多轮对话时应原样带回。

## 响应

```json
{
  "id": "msg_...",
  "type": "message",
  "role": "assistant",
  "model": "claude-opus-5-5-j",
  "content": [
    {"type": "text", "text": "你好！有什么可以帮你的？"}
  ],
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 20,
    "output_tokens": 10
  }
}
```

- `content` 是内容块数组，可能同时包含 `thinking`、`text`、`tool_use`。
- `stop_reason` 常见值：`end_turn`、`max_tokens`、`stop_sequence`、`tool_use`。
- `usage` 中可能包含缓存相关字段（如 `cache_read_input_tokens`），以响应为准。

## 流式

```bash
curl -N https://<base_url>/v1/messages \
  -H "x-api-key: $CUBEROUTER_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-opus-5-5-j",
    "max_tokens": 512,
    "messages": [{"role": "user", "content": "写一段产品介绍"}],
    "stream": true
  }'
```

常见事件顺序：

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

工具调用时，`content_block_delta` 的 `delta.type` 为 `input_json_delta`，需要把 `partial_json` 拼接成完整 JSON。

## 各系列差异

| 系列 | 差异与建议 |
| --- | --- |
| Claude | 原生协议，`thinking`、工具、图片输入支持最完整。建议 Claude 型号优先使用该协议。 |
| DeepSeek | 网关会把 Messages 请求转换为上游 Chat。`thinking` 支持 `enabled` / `disabled`；`output_config.effort` 可控制思考强度；`max_tokens` 生效。 |
| Kimi | K3 系列支持思考开关和工具调用；K2.7 Code 偏编程场景。 |
| Qwen / GLM / Doubao | 通过转换支持 Messages；工具调用和思考字段按上游能力生效。 |
| GPT | 部分 `-m` 型号支持 Messages 协议，但由网关转换；`thinking` 等 Anthropic 专有字段可能不生效。需要完整功能时用 OpenAI 或 Responses 协议。 |
| Gemini | 部分型号支持 Messages 协议，由网关转换；原生 Gemini 字段（如 `safetySettings`）需要走 Gemini 协议。 |

## Claude Code

Claude Code 使用 Anthropic 协议。核心配置：

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

`ANTHROPIC_BASE_URL` 只填域名，不要带 `/v1`。完整步骤见 [Claude Code](/getting-started/claude-code)。

## 常见问题

### Q1: 返回 400 `max_tokens: field required`？

Anthropic 协议要求 `max_tokens`。即使模型有默认输出上限，也建议显式传入。

### Q2: `thinking.budget_tokens` 报错？

`budget_tokens` 必须小于 `max_tokens`，且部分模型只接受特定范围。先把 `max_tokens` 调到大于预算值再试。

### Q3: 返回 404 `model not support`？

该模型没有声明 Anthropic 端点。对照上面的模型表，或改用 OpenAI / Gemini 协议。

### Q4: Claude Code 返回 401 或一直重试？

检查 `ANTHROPIC_AUTH_TOKEN` 是否是站内密钥、`ANTHROPIC_BASE_URL` 是否只填域名（不带 `/v1`），以及密钥分组是否包含所选 Claude 模型。

### Q5: `count_tokens` 返回的 token 数与实际用量不一致？

`count_tokens` 是估算接口，不同模型、不同内容块的计算方式可能不同；实际 token 统计以响应中的 `usage` 为准。
