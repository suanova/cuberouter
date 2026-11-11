# Anthropic Protocol

> Call Claude and other compatible models using the Anthropic Messages format.

The Anthropic protocol is suitable for the Claude family, Claude Code, and applications that need the native Messages format. Some non-Claude models also support this protocol; the gateway converts Messages requests to the corresponding upstream format.

## Endpoints

| Purpose | Method and path |
| --- | --- |
| Messages | `POST /v1/messages` |
| Count tokens | `POST /v1/messages/count_tokens` |

Request headers:

```http
x-api-key: sk-xxxxxxxx
anthropic-version: 2023-06-01
Content-Type: application/json
```

`Authorization: Bearer sk-xxxxxxxx` is also supported. Keep `anthropic-version`; some clients validate it.

## Supported models

| Family | Models |
| --- | --- |
| Claude | `claude-fabel-5-j`, `claude-fable-5`, `claude-fable-5-1`, `claude-fable-5-1-j`, `claude-haiku-5-5`, `claude-haiku-5-5-j`, `claude-opus-4-6-j`, `claude-opus-4-7-j`, `claude-opus-4-8-j`, `claude-opus-5`, `claude-opus-5-5`, `claude-opus-5-5-j`, `claude-opus-5-j`, `claude-sonnet-4-6-j`, `claude-sonnet-5`, `claude-sonnet-5-5`, `claude-sonnet-5-5-j` |
| DeepSeek | `deepseek-flash`, `deepseek-v4-1-flash-261017-h`, `deepseek-v4-flash-0731`, `deepseek-v4-flash-0731-a`, `deepseek-v4-flash-a`, `deepseek-v4-pro`, `deepseek-v4-pro-0813`, `deepseek-v4-pro-a`, `deepseek-v4.1-flash`, `deepseek-v4.1-flash-a` |
| Kimi | `kimi-k2.5-a`, `kimi-k2.6-a`, `kimi-k2.7-code`, `kimi-k2.7-code-a`, `kimi-k3`, `kimi-k3-1-pro-h`, `kimi-k3-a`, `kimi-k3-b`, `kimi-k3-j` |
| Qwen | `qwen3.6-plus`, `qwen3.7-max`, `qwen3.7-plus`, `qwen3.7-plus-a`, `qwen3.8-max`, `qwen3.8-max-a` |
| GLM | `glm-5-a`, `glm-5.1-a`, `glm-5.2`, `glm-5.2-a`, `glm-5.3`, `glm-5.3-z` |
| Doubao | `doubao-seed-2-1-pro-h`, `doubao-seed-2-1-turbo-h` |
| GPT | `gpt-5.4-m`, `gpt-5.5-m`, `gpt-5.6-luna-m`, `gpt-5.6-sol`, `gpt-5.6-sol-m`, `gpt-5.6-terra`, `gpt-5.6-terra-m`, `gpt-6-astra`, `gpt-6-astra-m`, `gpt-6-luna-m`, `gpt-6-sol-m`, `gpt-6.1-sol` |
| Gemini | `gemini-3.1-pro-preview`, `gemini-3.5-flash-b`, `gemini-3.7-flash-b`, `gemini-3.8-flash`, `gemini-3.8-flash-b` |

> Non-Claude models support the Messages protocol through gateway conversion; parameter support depends on upstream capabilities. For the most complete feature set, prefer the model's native protocol.
> Model availability is limited by key group; the models actually visible in Model Square are authoritative.

## Request

```json
{
  "model": "claude-opus-5-5-j",
  "max_tokens": 1024,
  "system": "You are a concise assistant.",
  "messages": [
    {"role": "user", "content": "Hello"}
  ],
  "temperature": 0.7,
  "stream": false
}
```

Common fields:

| Field | Type | Required | Description |
| --- | --- | :---: | --- |
| `model` | string | Yes | Site model ID |
| `max_tokens` | integer | Yes | Output limit; must be greater than `thinking.budget_tokens` |
| `messages` | array | Yes | Conversation messages |
| `system` | string / array | No | System prompt |
| `temperature` | number | No | Sampling temperature |
| `top_p` | number | No | Nucleus sampling |
| `top_k` | integer | No | Top-K |
| `stop_sequences` | string[] | No | Stop sequences |
| `stream` | boolean | No | Whether to stream |
| `tools` | array | No | Tool definitions |
| `tool_choice` | object | No | Tool selection strategy |
| `thinking` | object | No | Thinking configuration |
| `metadata` | object | No | Business metadata, such as `user_id` |
| `output_config` | object | No | Output configuration; some models use it to specify `effort` |

### Message content

`messages[].content` can be a string or an array of content blocks:

```json
{
  "role": "user",
  "content": [
    {"type": "text", "text": "Describe this image"},
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

Common content blocks:

| Type | Description |
| --- | --- |
| `text` | Plain text |
| `image` | Image; `source.type` can be `base64` or `url` |
| `tool_use` | A tool call initiated by the model; appears in assistant messages |
| `tool_result` | Tool execution result; appears in user messages and requires `tool_use_id` |
| `thinking` | Thinking content; appears in responses from models that support thinking |

### Tool calling

```json
{
  "tools": [{
    "name": "get_weather",
    "description": "Query city weather",
    "input_schema": {
      "type": "object",
      "properties": {"city": {"type": "string"}},
      "required": ["city"]
    }
  }],
  "tool_choice": {"type": "auto"}
}
```

When the model returns `stop_reason="tool_use"`, take the `tool_use` block from `content`, execute it, and put the result back as a `tool_result` in the next user message.

### Thinking

```json
{
  "thinking": {"type": "enabled", "budget_tokens": 2048},
  "max_tokens": 4096
}
```

- `budget_tokens` must be less than `max_tokens`.
- To disable thinking: `{"type": "disabled"}`.
- Thinking content is returned as `thinking` blocks; some models also return `signature`, which should be passed back unchanged in multi-turn conversations.

## Response

```json
{
  "id": "msg_...",
  "type": "message",
  "role": "assistant",
  "model": "claude-opus-5-5-j",
  "content": [
    {"type": "text", "text": "Hello! How can I help you?"}
  ],
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 20,
    "output_tokens": 10
  }
}
```

- `content` is an array of content blocks and may contain `thinking`, `text`, and `tool_use` at the same time.
- Common `stop_reason` values: `end_turn`, `max_tokens`, `stop_sequence`, `tool_use`.
- `usage` may contain cache-related fields (such as `cache_read_input_tokens`); the response is authoritative.

## Streaming

```bash
curl -N https://<base_url>/v1/messages \
  -H "x-api-key: $CUBEROUTER_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-opus-5-5-j",
    "max_tokens": 512,
    "messages": [{"role": "user", "content": "Write a product description"}],
    "stream": true
  }'
```

Common event order:

```text
event: message_start
data: {"type":"message_start","message":{...}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hel"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":10}}

event: message_stop
data: {"type":"message_stop"}
```

During tool calling, `delta.type` of `content_block_delta` is `input_json_delta`; concatenate `partial_json` into complete JSON.

## Differences by family

| Family | Differences and recommendations |
| --- | --- |
| Claude | Native protocol with the most complete support for `thinking`, tools, and image input. For Claude models, prefer this protocol. |
| DeepSeek | The gateway converts Messages requests to upstream Chat. `thinking` supports `enabled` / `disabled`; `output_config.effort` controls thinking effort; `max_tokens` takes effect. |
| Kimi | The K3 family supports a thinking toggle and tool calling; K2.7 Code targets coding scenarios. |
| Qwen / GLM / Doubao | Supports Messages through conversion; tool calling and thinking fields work according to upstream capabilities. |
| GPT | Some `-m` models support the Messages protocol but are converted by the gateway; Anthropic-specific fields such as `thinking` may not work. For full functionality, use the OpenAI or Responses protocol. |
| Gemini | Some models support the Messages protocol through gateway conversion; Gemini-native fields (such as `safetySettings`) require the Gemini protocol. |

## Claude Code

Claude Code uses the Anthropic protocol. Core configuration:

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

For `ANTHROPIC_BASE_URL`, enter only the domain, without `/v1`. For complete steps, see [Claude Code](/en/getting-started/claude-code).

## FAQ

### Q1: Getting 400 `max_tokens: field required`?

The Anthropic protocol requires `max_tokens`. Even if a model has a default output limit, pass it explicitly.

### Q2: `thinking.budget_tokens` returns an error?

`budget_tokens` must be less than `max_tokens`, and some models accept only specific ranges. First raise `max_tokens` above the budget value, then retry.

### Q3: Getting 404 `model not support`?

The model does not declare an Anthropic endpoint. Check the model table above, or switch to the OpenAI / Gemini protocol.

### Q4: Claude Code returns 401 or keeps retrying?

Check that `ANTHROPIC_AUTH_TOKEN` is a key issued by the site, that `ANTHROPIC_BASE_URL` contains only the domain (without `/v1`), and that your key group includes the selected Claude model.

### Q5: The token count from `count_tokens` does not match actual usage?

`count_tokens` is an estimation endpoint, and the calculation may differ by model and content block; the `usage` in the response is authoritative for actual token counts.
