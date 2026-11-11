# OpenAI Protocol

> Call site models using the OpenAI Chat Completions and Responses formats.

The OpenAI protocol is the most widely compatible text protocol. Most text models support `/v1/chat/completions`; newer models also support `/v1/responses`, which is suitable for Codex CLI and applications that need native Responses capabilities.

## Endpoints

| Purpose | Method and path |
| --- | --- |
| Chat Completions | `POST /v1/chat/completions` |
| Legacy text completions | `POST /v1/completions` |
| Responses | `POST /v1/responses` |
| Responses compaction | `POST /v1/responses/compact` |
| Model list | `GET /v1/models` |

Request headers:

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

## Supported models

### Chat Completions

| Family | Models |
| --- | --- |
| Claude | `claude-fabel-5-j`, `claude-fable-5`, `claude-fable-5-1`, `claude-fable-5-1-j`, `claude-haiku-5-5`, `claude-haiku-5-5-j`, `claude-opus-4-6-j`, `claude-opus-4-6-v1-aws-g`, `claude-opus-4-6-v1-aws-us`, `claude-opus-4-7-j`, `claude-opus-4-8-j`, `claude-opus-5`, `claude-opus-5-5`, `claude-opus-5-5-j`, `claude-opus-5-j`, `claude-sonnet-4-6-aws-g`, `claude-sonnet-4-6-aws-us`, `claude-sonnet-4-6-j`, `claude-sonnet-5`, `claude-sonnet-5-5`, `claude-sonnet-5-5-j` |
| GPT | `gpt-5.4-b`, `gpt-5.4-m`, `gpt-5.5-b`, `gpt-5.5-m`, `gpt-5.6-luna-b`, `gpt-5.6-luna-m`, `gpt-5.6-sol`, `gpt-5.6-sol-b`, `gpt-5.6-sol-m`, `gpt-5.6-terra`, `gpt-5.6-terra-b`, `gpt-5.6-terra-m`, `gpt-6-astra`, `gpt-6-astra-b`, `gpt-6-astra-m`, `gpt-6-luna`, `gpt-6-luna-b`, `gpt-6-luna-m`, `gpt-6-sol-b`, `gpt-6-sol-m`, `gpt-6.1-sol`, `gpt-6.1-sol-b` |
| Gemini | `gemini-3.1-flash-lite-m`, `gemini-3.1-pro-preview`, `gemini-3.1-pro-preview-m`, `gemini-3.5-flash-b`, `gemini-3.7-flash-b`, `gemini-3.8-flash`, `gemini-3.8-flash-b` |
| DeepSeek | `deepseek-flash`, `deepseek-v4-1-flash-261017-h`, `deepseek-v4-flash-0731`, `deepseek-v4-flash-0731-a`, `deepseek-v4-flash-a`, `deepseek-v4-pro`, `deepseek-v4-pro-0813`, `deepseek-v4-pro-a`, `deepseek-v4.1-flash`, `deepseek-v4.1-flash-a` |
| Kimi | `kimi-k2.5-a`, `kimi-k2.6-a`, `kimi-k2.7-code`, `kimi-k2.7-code-a`, `kimi-k2.8-preview-a`, `kimi-k3`, `kimi-k3-1-pro-h`, `kimi-k3-a`, `kimi-k3-b`, `kimi-k3-j` |
| Qwen | `qwen3.6-plus`, `qwen3.7-max`, `qwen3.7-plus`, `qwen3.7-plus-a`, `qwen3.8-max`, `qwen3.8-max-a` |
| GLM | `glm-5-a`, `glm-5.1-a`, `glm-5.2`, `glm-5.2-a`, `glm-5.3`, `glm-5.3-a`, `glm-5.3-z` |
| Doubao | `doubao-seed-2-1-pro-h`, `doubao-seed-2-1-turbo-h` |
| MiniMax | `minimax-m2.7-a`, `minimax-m3.1-flash-a` |

### Responses

| Family | Models |
| --- | --- |
| Claude | `claude-fable-5`, `claude-fable-5-1`, `claude-fable-5-1-j`, `claude-haiku-5-5`, `claude-haiku-5-5-j`, `claude-opus-5`, `claude-opus-5-5`, `claude-opus-5-5-j`, `claude-sonnet-5`, `claude-sonnet-5-5`, `claude-sonnet-5-5-j` |
| GPT | `gpt-5.4-b`, `gpt-5.4-m`, `gpt-5.5-b`, `gpt-5.5-m`, `gpt-5.6-luna-b`, `gpt-5.6-luna-m`, `gpt-5.6-sol`, `gpt-5.6-sol-b`, `gpt-5.6-sol-m`, `gpt-5.6-terra`, `gpt-5.6-terra-b`, `gpt-5.6-terra-m`, `gpt-6-astra`, `gpt-6-astra-b`, `gpt-6-astra-m`, `gpt-6-luna`, `gpt-6-luna-b`, `gpt-6-luna-m`, `gpt-6-sol-b`, `gpt-6-sol-m`, `gpt-6.1-sol`, `gpt-6.1-sol-b` |
| Gemini | `gemini-3.1-pro-preview`, `gemini-3.5-flash-b`, `gemini-3.7-flash-b`, `gemini-3.8-flash`, `gemini-3.8-flash-b` |
| DeepSeek | `deepseek-flash`, `deepseek-v4-1-flash-261017-h`, `deepseek-v4-flash-0731`, `deepseek-v4-flash-0731-a`, `deepseek-v4-flash-a`, `deepseek-v4-pro`, `deepseek-v4-pro-0813`, `deepseek-v4-pro-a`, `deepseek-v4.1-flash`, `deepseek-v4.1-flash-a` |
| Kimi | `kimi-k2.7-code`, `kimi-k3`, `kimi-k3-1-pro-h`, `kimi-k3-a`, `kimi-k3-b` |
| Qwen | `qwen3.6-plus`, `qwen3.7-max`, `qwen3.7-plus`, `qwen3.7-plus-a`, `qwen3.8-max`, `qwen3.8-max-a` |
| GLM | `glm-5.2`, `glm-5.2-a`, `glm-5.3`, `glm-5.3-z` |
| Doubao | `doubao-seed-2-1-pro-h`, `doubao-seed-2-1-turbo-h` |

> The table above comes from site model metadata. Model availability is also limited by key group; the models actually visible in Model Square are authoritative.

## Chat Completions

### Request

```json
{
  "model": "gpt-6-luna-m",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Hello"}
  ],
  "temperature": 0.7,
  "max_completion_tokens": 1024,
  "stream": false
}
```

Common fields:

| Field | Type | Description |
| --- | --- | --- |
| `messages` | array | Conversation messages, see the table below |
| `temperature` | number | Sampling temperature |
| `top_p` | number | Nucleus sampling |
| `max_tokens` / `max_completion_tokens` | integer | Output limit; use one or the other |
| `stream` | boolean | Whether to stream |
| `stream_options.include_usage` | boolean | Whether to return usage separately when streaming |
| `stop` | string / string[] | Stop sequences |
| `response_format` | object | Text or JSON output |
| `tools` / `tool_choice` | array / string | Tool calling |
| `parallel_tool_calls` | boolean | Whether parallel tool calling is allowed |
| `reasoning_effort` | string | Reasoning effort; support varies by model |
| `seed` | number | Random seed |

Common `messages[].role` values:

| role | Description |
| --- | --- |
| `system` | System prompt |
| `user` | User input |
| `assistant` | Previous model replies |
| `tool` | Tool execution result; must be paired with `tool_call_id` |

### Response

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
      "content": "Hello! How can I help you?"
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

- `choices[].message.content` is the final text.
- Reasoning models may return reasoning content in the `reasoning_content` field or in a separate block before `content`; the field name can differ between models.
- Token counts in `usage` are authoritative as returned in the response.

### Streaming

```bash
curl -N https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-6-luna-m",
    "messages": [{"role": "user", "content": "Write a product description"}],
    "stream": true,
    "stream_options": {"include_usage": true}
  }'
```

Each chunk looks like:

```text
data: {"id":"chatcmpl-...","choices":[{"index":0,"delta":{"content":"Hel"}}]}

data: {"id":"chatcmpl-...","choices":[{"index":0,"delta":{"content":"lo"}}]}

data: [DONE]
```

`delta.content` is incremental text; `delta.tool_calls` is an incremental tool call; the last chunk with `finish_reason` usually includes usage.

### Tool calling

```bash
curl https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-6-luna-m",
    "messages": [{"role": "user", "content": "What is the weather like in Beijing today?"}],
    "tools": [{
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Query city weather",
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

When the model returns `finish_reason="tool_calls"`, pass `tool_calls` back unchanged in the next turn and append a message with `role="tool"` whose `tool_call_id` matches.

### Image input

```bash
curl https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-6-astra-m",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "What is in this image?"},
        {"type": "image_url", "image_url": {"url": "https://example.com/cat.jpg", "detail": "high"}}
      ]
    }]
  }'
```

The image `url` can be a public URL or a `data:image/jpeg;base64,...` URI. Some models also support video and audio input; see the notes for each family.

## Responses API

Responses is OpenAI's next-generation text interface, suitable for Codex CLI and applications that need native tool calling or multi-turn context management.

### Request

```json
{
  "model": "gpt-6.1-sol",
  "instructions": "You are a concise assistant.",
  "input": "Introduce CubeRouter in three sentences.",
  "max_output_tokens": 512,
  "reasoning": {"effort": "medium"},
  "stream": false
}
```

`input` can be a string or an array of input items:

```json
{
  "input": [
    {"role": "user", "content": [
      {"type": "input_text", "text": "Describe this image"},
      {"type": "input_image", "image_url": "https://example.com/cat.jpg"}
    ]}
  ]
}
```

Common fields:

| Field | Type | Description |
| --- | --- | --- |
| `input` | string / array | Input content |
| `instructions` | string | System instructions |
| `max_output_tokens` | integer | Output limit |
| `reasoning.effort` | string | Reasoning effort |
| `text.format` | object | Text or JSON Schema output |
| `tools` / `tool_choice` | array / object | Tool calling |
| `previous_response_id` | string | Link to the previous response to reduce repeated context |
| `store` | boolean | Whether upstream storage is allowed |
| `include` | array | Additional returned content, such as reasoning summaries |
| `stream` | boolean | Whether to stream |

### Response

```json
{
  "id": "resp_...",
  "object": "response",
  "status": "completed",
  "model": "gpt-6.1-sol",
  "output": [{
    "type": "message",
    "role": "assistant",
    "content": [{"type": "output_text", "text": "CubeRouter is an AI gateway."}]
  }],
  "usage": {
    "input_tokens": 20,
    "output_tokens": 30,
    "total_tokens": 50
  }
}
```

### Streaming events

Responses streaming uses named events; a common order is:

```text
event: response.created
data: {"type":"response.created","response":{...}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"Hel"}

event: response.output_text.done
data: {"type":"response.output_text.done","text":"Hello"}

event: response.completed
data: {"type":"response.completed","response":{...}}
```

Clients should treat `response.completed` as the end marker and read usage from that event's `response.usage`.

### Codex CLI

Codex uses the Responses protocol. Configuration example:

```toml
model_provider = "cuberouter"
model = "gpt-6.1-sol"

[model_providers.cuberouter]
name = "CubeRouter"
base_url = "https://<base_url>"
env_key = "CODEX_CUBEROUTER_API_KEY"
wire_api = "responses"
```

For complete steps, see [Codex](/en/getting-started/codex).

## Python (OpenAI SDK)

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
    messages=[{"role": "user", "content": "Hello"}],
)
print(chat.choices[0].message.content)

# Responses
resp = client.responses.create(
    model="gpt-6.1-sol",
    input="Introduce CubeRouter in three sentences.",
)
print(resp.output_text)
```

## Differences by family

| Family | Differences and recommendations |
| --- | --- |
| Claude | Prefer the Anthropic protocol. When called through the OpenAI protocol, the gateway converts Chat/Responses requests to Anthropic Messages; Anthropic-specific fields such as `thinking` are not guaranteed to work. |
| GPT | Native support. For the GPT-6 family, prefer `max_completion_tokens`; some upstreams automatically move `max_tokens` to `max_completion_tokens`. `reasoning_effort` and `verbosity` are supported per model. |
| Gemini | Calls through the OpenAI protocol are converted by the gateway. Gemini-native fields such as `safetySettings`, `thinkingConfig`, and `responseModalities` require the Gemini protocol. |
| DeepSeek | Supports `thinking` and `reasoning_effort`. On some models, `frequency_penalty`, `presence_penalty`, `logit_bias`, `n`, and `seed` have no effect or return only 1 candidate; `stop` may have a maximum number of entries. |
| Kimi | Models such as K3 and K2.8 support a thinking toggle; K2.8 Preview also supports image/video input. Field names depend on what the model actually supports; the common one is `thinking`. |
| Qwen | Supports `enable_thinking` and `thinking_budget` (depending on the model); `vl_high_resolution_images` increases image parsing resolution. |
| GLM | Supports a `thinking` toggle; the GLM-5.3 family supports tool calling and long context. |
| Doubao | Supports a `thinking` toggle; the Seed 2.1 family supports text/image/video input. |
| MiniMax | Supports `reasoning_split` to separate reasoning content; some models support prompt caching. |

### Verified DeepSeek differences

Using `deepseek-v4-1-flash-261017-h` as an example:

- `thinking` accepts only `{"type":"enabled"}` / `{"type":"disabled"}`; `auto` and `adaptive` return 400.
- `reasoning_effort` supports `none`, `minimal`, `low`, `medium`, `high`, `xhigh`, and `max`; `ultra` returns 400.
- `thinking.disabled` takes precedence over `reasoning_effort`: when thinking is disabled, passing `high` still does not enable reasoning.
- `max_tokens` and `max_completion_tokens` cannot be sent together.
- `stop` accepts at most 16 entries; more returns 400.
- `frequency_penalty`, `presence_penalty`, `logit_bias`, `n`, `seed`, and `user` have no effect or no observable effect on this model.
- With `response_format=json_object`, the prompt must contain the word "json".

## FAQ

### Q1: Getting 404 `model not found` or `does not support`?

First confirm the model ID matches Model Square exactly, then confirm the model supports the current protocol. For example, calling `/v1/chat/completions` with a model that only supports the Anthropic protocol returns 404 or 400. Also confirm your key group includes the model.

### Q2: Getting 400 `max_tokens and max_completion_tokens cannot be set at the same time`?

Send only one of them. For the GPT-6 family, prefer `max_completion_tokens` directly.

### Q3: Reasoning models return empty `content`?

The output limit is too small and all tokens are spent on reasoning. Raise `max_tokens` / `max_completion_tokens` to at least 512, or disable reasoning (if the model supports it).

### Q4: No usage in the streaming response?

OpenAI Chat requires `stream_options.include_usage=true`; some models do not support this field, and usage is attached to the last chunk with `finish_reason`. For the Responses protocol, read usage from the `response.completed` event.

### Q5: Should Claude models use the OpenAI protocol or the Anthropic protocol?

Prefer the Anthropic protocol, which has the most complete support for vendor-specific fields. Existing OpenAI code can keep using Chat Completions; the gateway converts it.

### Q6: `service_tier` has no effect when passed?

`service_tier` is filtered by the gateway by default and is only passed through after it is enabled on the site. The same applies to `safety_identifier`.
