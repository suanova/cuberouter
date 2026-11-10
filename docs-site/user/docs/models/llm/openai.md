# OpenAI 协议

> 使用 OpenAI Chat Completions 和 Responses 格式调用站内模型。

OpenAI 协议是兼容性最广的文本协议。大多数文本模型都支持 `/v1/chat/completions`；较新的模型还支持 `/v1/responses`，适合 Codex CLI 和需要原生 Responses 能力的应用。

## 端点

| 用途 | 方法与路径 |
| --- | --- |
| Chat Completions | `POST /v1/chat/completions` |
| 传统文本补全 | `POST /v1/completions` |
| Responses | `POST /v1/responses` |
| Responses 压缩 | `POST /v1/responses/compact` |
| 模型列表 | `GET /v1/models` |

请求头：

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

## 支持的模型

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

> 上表来自站内模型元数据。模型可用性还受密钥分组限制；实际可见型号以模型广场为准。

## Chat Completions

### 请求

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

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `messages` | array | 对话消息，见下表 |
| `temperature` | number | 采样温度 |
| `top_p` | number | 核采样 |
| `max_tokens` / `max_completion_tokens` | integer | 输出上限，二选一 |
| `stream` | boolean | 是否流式 |
| `stream_options.include_usage` | boolean | 流式时是否单独返回 usage |
| `stop` | string / string[] | 停止词 |
| `response_format` | object | 文本或 JSON 输出 |
| `tools` / `tool_choice` | array / string | 工具调用 |
| `parallel_tool_calls` | boolean | 是否允许并行工具调用 |
| `reasoning_effort` | string | 推理强度，支持情况因模型而异 |
| `seed` | number | 随机种子 |

`messages[].role` 常用取值：

| role | 说明 |
| --- | --- |
| `system` | 系统提示 |
| `user` | 用户输入 |
| `assistant` | 模型历史回复 |
| `tool` | 工具执行结果，需要配合 `tool_call_id` |

### 响应

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
      "content": "你好！有什么可以帮你的？"
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

- `choices[].message.content` 是最终文本。
- 推理模型的思考内容可能在 `reasoning_content` 字段或 `content` 之前的独立块中返回；不同模型字段名可能不同。
- `usage` 中的 token 统计以响应为准。

### 流式

```bash
curl -N https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-6-luna-m",
    "messages": [{"role": "user", "content": "写一段产品介绍"}],
    "stream": true,
    "stream_options": {"include_usage": true}
  }'
```

每个 chunk 形如：

```text
data: {"id":"chatcmpl-...","choices":[{"index":0,"delta":{"content":"你"}}]}

data: {"id":"chatcmpl-...","choices":[{"index":0,"delta":{"content":"好"}}]}

data: [DONE]
```

`delta.content` 是增量文本；`delta.tool_calls` 是增量工具调用；最后一个带 `finish_reason` 的 chunk 通常包含 usage。

### 工具调用

```bash
curl https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-6-luna-m",
    "messages": [{"role": "user", "content": "北京今天天气怎么样？"}],
    "tools": [{
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "查询城市天气",
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

模型返回 `finish_reason="tool_calls"` 时，把 `tool_calls` 原样带回下一轮，并追加一条 `role="tool"`、`tool_call_id` 对应的消息。

### 图片输入

```bash
curl https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-6-astra-m",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "这张图片里有什么？"},
        {"type": "image_url", "image_url": {"url": "https://example.com/cat.jpg", "detail": "high"}}
      ]
    }]
  }'
```

图片 `url` 可以是公网 URL，也可以是 `data:image/jpeg;base64,...`。部分模型还支持视频和音频输入，见各系列说明。

## Responses API

Responses 是 OpenAI 的新一代文本接口，适合 Codex CLI、需要原生工具调用或多轮上下文管理的应用。

### 请求

```json
{
  "model": "gpt-6.1-sol",
  "instructions": "你是一个简洁的中文助手。",
  "input": "用三句话介绍 CubeRouter。",
  "max_output_tokens": 512,
  "reasoning": {"effort": "medium"},
  "stream": false
}
```

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

常用字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `input` | string / array | 输入内容 |
| `instructions` | string | 系统指令 |
| `max_output_tokens` | integer | 输出上限 |
| `reasoning.effort` | string | 思考强度 |
| `text.format` | object | 文本或 JSON Schema 输出 |
| `tools` / `tool_choice` | array / object | 工具调用 |
| `previous_response_id` | string | 关联上一轮响应，减少重复上下文 |
| `store` | boolean | 是否允许上游存储 |
| `include` | array | 额外返回内容，例如推理摘要 |
| `stream` | boolean | 是否流式 |

### 响应

```json
{
  "id": "resp_...",
  "object": "response",
  "status": "completed",
  "model": "gpt-6.1-sol",
  "output": [{
    "type": "message",
    "role": "assistant",
    "content": [{"type": "output_text", "text": "CubeRouter 是一个 AI 网关。"}]
  }],
  "usage": {
    "input_tokens": 20,
    "output_tokens": 30,
    "total_tokens": 50
  }
}
```

### 流式事件

Responses 的流式响应使用命名事件，常见顺序：

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

客户端应以 `response.completed` 作为结束标志，并从该事件的 `response.usage` 读取用量。

### Codex CLI

Codex 使用 Responses 协议。配置示例：

```toml
model_provider = "cuberouter"
model = "gpt-6.1-sol"

[model_providers.cuberouter]
name = "CubeRouter"
base_url = "https://<base_url>"
env_key = "CODEX_CUBEROUTER_API_KEY"
wire_api = "responses"
```

完整步骤见 [Codex](/getting-started/codex)。

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
    input="用三句话介绍 CubeRouter。",
)
print(resp.output_text)
```

## 各系列差异

| 系列 | 差异与建议 |
| --- | --- |
| Claude | 建议使用 Anthropic 协议。通过 OpenAI 协议调用时，网关会把 Chat/Responses 请求转换为 Anthropic Messages；`thinking` 等 Anthropic 专有字段不保证生效。 |
| GPT | 原生支持。GPT-6 系列建议使用 `max_completion_tokens`；部分上游会把 `max_tokens` 自动搬到 `max_completion_tokens`。`reasoning_effort`、`verbosity` 按模型支持。 |
| Gemini | 通过 OpenAI 协议调用时由网关转换。`safetySettings`、`thinkingConfig`、`responseModalities` 等 Gemini 原生字段需要走 Gemini 协议。 |
| DeepSeek | 支持 `thinking` 和 `reasoning_effort`。部分型号对 `frequency_penalty`、`presence_penalty`、`logit_bias`、`n`、`seed` 不生效或只返回 1 个候选；`stop` 可能有条数上限。 |
| Kimi | K3、K2.8 等型号支持思考开关；K2.8 Preview 还支持图像/视频输入。字段名以模型实际支持为准，常见为 `thinking`。 |
| Qwen | 支持 `enable_thinking`、`thinking_budget`（具体型号而定）；`vl_high_resolution_images` 用于提高图片解析精度。 |
| GLM | 支持 `thinking` 开关；GLM-5.3 系列支持工具调用和长上下文。 |
| Doubao | 支持 `thinking` 开关；Seed 2.1 系列支持文本/图片/视频输入。 |
| MiniMax | 支持 `reasoning_split` 分离思考内容；部分型号支持提示缓存。 |

### 已验证的 DeepSeek 差异

以 `deepseek-v4-1-flash-261017-h` 为例：

- `thinking` 只接受 `{"type":"enabled"}` / `{"type":"disabled"}`；`auto`、`adaptive` 会 400。
- `reasoning_effort` 支持 `none`、`minimal`、`low`、`medium`、`high`、`xhigh`、`max`；`ultra` 会 400。
- `thinking.disabled` 优先于 `reasoning_effort`：关闭思考时即使传 `high` 也不会思考。
- `max_tokens` 与 `max_completion_tokens` 不能同时传。
- `stop` 最多 16 个，超出会 400。
- `frequency_penalty`、`presence_penalty`、`logit_bias`、`n`、`seed`、`user` 在该型号上不生效或无可观测效果。
- `response_format=json_object` 时，提示词必须包含 "json" 字样。

## 常见问题

### Q1: 返回 404 `model not found` 或 `does not support`？

先确认模型 ID 与模型广场完全一致，再确认该模型支持当前协议。例如只支持 Anthropic 协议的模型调用 `/v1/chat/completions` 会返回 404 或 400。还要确认密钥分组包含该模型。

### Q2: 返回 400 `max_tokens and max_completion_tokens cannot be set at the same time`？

只传其中一个。GPT-6 系列建议直接使用 `max_completion_tokens`。

### Q3: 推理模型返回空 `content`？

输出上限太小，token 全部用在思考上。把 `max_tokens` / `max_completion_tokens` 调到至少 512，或关闭思考（如果模型支持）。

### Q4: 流式响应里没有 usage？

OpenAI Chat 需要传 `stream_options.include_usage=true`；部分模型不支持该字段，usage 会附在最后一个带 `finish_reason` 的 chunk 里。Responses 协议从 `response.completed` 事件读取 usage。

### Q5: Claude 模型应该用 OpenAI 协议还是 Anthropic 协议？

优先使用 Anthropic 协议，厂商专有字段支持最完整。已有 OpenAI 代码可以继续用 Chat Completions，网关会做转换。

### Q6: `service_tier` 传了没有效果？

`service_tier` 默认会被网关过滤，只有站点开启后才透传。`safety_identifier` 同理。
