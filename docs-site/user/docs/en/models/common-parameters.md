# Common Parameters

> Field semantics, multimodal input methods, and error handling shared across protocols. For protocol-specific fields, see the individual protocol pages.

Field names differ somewhat between protocols, but they express the same concepts. The table below maps common concepts across the four text protocols:

| Concept | OpenAI Chat | OpenAI Responses | Anthropic Messages | Gemini |
| --- | --- | --- | --- | --- |
| Model | `model` | `model` | `model` | `{model}` in the path |
| Conversation content | `messages` | `input` | `messages` | `contents` |
| System prompt | `messages[role=system]` | `instructions` | `system` | `systemInstruction` |
| Output limit | `max_tokens` / `max_completion_tokens` | `max_output_tokens` | `max_tokens` (required) | `generationConfig.maxOutputTokens` |
| Temperature | `temperature` | `temperature` | `temperature` | `generationConfig.temperature` |
| Nucleus sampling | `top_p` | `top_p` | `top_p` | `generationConfig.topP` |
| Top-K | `top_k` (some models) | — | `top_k` | `generationConfig.topK` |
| Stop sequences | `stop` | — | `stop_sequences` | `generationConfig.stopSequences` |
| Streaming | `stream` | `stream` | `stream` | `:streamGenerateContent` |
| Tool calling | `tools` / `tool_choice` | `tools` / `tool_choice` | `tools` / `tool_choice` | `tools` / `toolConfig` |
| Structured output | `response_format` | `text.format` | `output_config` / `output_format` | `generationConfig.responseMimeType` / `responseSchema` |
| Reasoning control | `reasoning_effort` / `thinking` / `enable_thinking` | `reasoning.effort` | `thinking` | `generationConfig.thinkingConfig` |
| Usage reporting | `usage` | `usage` | `usage` | `usageMetadata` |

## Authentication

All protocols use the same API key. Common request headers:

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

| Protocol | Authentication options |
| --- | --- |
| OpenAI | `Authorization: Bearer` |
| Anthropic | `Authorization: Bearer` or `x-api-key` |
| Gemini | `Authorization: Bearer`, `x-goog-api-key`, or the URL parameter `?key=` |
| Image / video / audio | `Authorization: Bearer` |

## Common Fields

### `model`

Required. Use the full site model ID from Model Square. Suffixed variants (`-a`, `-b`, `-m`, `-h`, `-j`, etc.) are different models and cannot be substituted for one another.

### `stream`

Whether to stream the response; defaults to `false`. When enabled, the response uses SSE:

```http
Content-Type: text/event-stream
```

See the individual protocol pages for streaming event formats. Note: with streaming enabled, clients should read chunk by chunk instead of waiting for the full response body.

### Output Limits

| Protocol | Field | Description |
| --- | --- | --- |
| OpenAI Chat | `max_tokens` or `max_completion_tokens` | Use one or the other, not both; for the GPT-6 family, prefer `max_completion_tokens` |
| OpenAI Responses | `max_output_tokens` | Includes visible output and reasoning tokens |
| Anthropic | `max_tokens` | **Required**, and must be greater than `thinking.budget_tokens` |
| Gemini | `generationConfig.maxOutputTokens` | Includes visible output and reasoning tokens |

The output limit constrains only the generated portion, not the input length. For reasoning models, if the limit is too low, all tokens may be spent on reasoning, leaving `content` empty; when troubleshooting, raise the limit to at least 512 first, then adjust gradually.

### Sampling Parameters

| Field | Common range | Description |
| --- | --- | --- |
| `temperature` | 0–2 | Lower values are more deterministic, higher values more diverse; some reasoning models do not use this parameter in reasoning mode |
| `top_p` | 0–1 | Tune either this or `temperature`; usually do not change both substantially at the same time |
| `top_k` | Positive integer | Supported by Anthropic, Gemini, and some open-source models; support varies across models under the OpenAI protocol |
| `stop` / `stop_sequences` | String or array | Stops generation when matched; the number of entries allowed varies by model, and exceeding it may return 400 |
| `n` / `candidateCount` | Positive integer | Returns multiple candidates; many reasoning models return only 1, and larger values may be ignored |

### Randomness and Reproducibility

- `seed`: some models support a fixed random seed, but reproducibility guarantees differ across upstreams; image models generally support it better.
- `temperature=0` does not guarantee full determinism either, especially with multi-GPU inference or batching at the upstream.

### Structured Output

| Protocol | Field | Common values |
| --- | --- | --- |
| OpenAI Chat | `response_format` | `{"type":"text"}`, `{"type":"json_object"}`, `{"type":"json_schema",...}` |
| OpenAI Responses | `text.format` | `{"type":"text"}`, `{"type":"json_schema",...}` |
| Anthropic | `output_config` / `output_format` | Subject to model support |
| Gemini | `responseMimeType` + `responseSchema` | `application/json` + JSON Schema |

With `json_object`, the prompt usually needs to contain the word "json"; with `json_schema`, the schema constrains the output structure. If the upstream does not support it, the request may return 400 or the field may be silently ignored, so verify with a small request first.

### Tool Calling

All protocols support function tools, but the field shapes differ:

```jsonc
// OpenAI Chat
{
  "tools": [{
    "type": "function",
    "function": {
      "name": "get_weather",
      "description": "Get the weather",
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
    "description": "Get the weather",
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
      "description": "Get the weather",
      "parameters": {"type": "object", "properties": {"city": {"type": "string"}}}
    }]
  }],
  "toolConfig": {"functionCallingConfig": {"mode": "AUTO"}}
}
```

Common `tool_choice` values:

- `auto`: the model decides whether to call a tool.
- `none`: tool calls are disabled.
- `required` / `any`: force at least one tool call.
- A specific tool object/name: force a call to the specified tool.

### Reasoning Control

Field names differ by vendor; the gateway converts them where possible, but full cross-protocol equivalence is not guaranteed:

| Family | Common fields | Description |
| --- | --- | --- |
| GPT | `reasoning_effort` | `low` / `medium` / `high`, etc.; the Responses protocol uses `reasoning.effort` |
| DeepSeek | `thinking`, `reasoning_effort` | `thinking` commonly uses `{"type":"enabled"}` / `{"type":"disabled"}` |
| Claude | `thinking` | `{"type":"enabled","budget_tokens":1024}`; must be less than `max_tokens` |
| Gemini | `generationConfig.thinkingConfig` | `thinkingBudget`, `thinkingLevel`, `includeThoughts` |
| Qwen | `enable_thinking`, `thinking_budget` | Support varies by model |
| GLM / Doubao | `thinking` | Commonly `{"type":"enabled"}` / `{"type":"disabled"}` |
| Kimi | `thinking` / `enable_thinking` | K3, K2.8, and other models support a reasoning toggle |
| MiniMax | `reasoning_split` | Whether to return reasoning content separately |

If a model does not support a given reasoning field, the request may return 400 or the field may be silently ignored. To determine whether it actually takes effect, check the reasoning/thinking content in the response or the reasoning tokens in `usage`.

### Other Common Fields

| Field | Description |
| --- | --- |
| `frequency_penalty` / `presence_penalty` | Reduce repetition or encourage new topics; many reasoning models do not support them |
| `logprobs` / `top_logprobs` | Return token log probabilities; both must be enabled together |
| `logit_bias` | Adjust the probability of specified tokens; support varies by model |
| `user` | Business-side user identifier used for risk control; not the sign-in username |
| `metadata` | Vendor-defined parameters or business tags; meaning varies by model |
| `prompt_cache_key` | Prompt cache routing key; supporting upstreams can achieve higher cache hit rates |
| `service_tier` | Service tier; filtered out by default and passed through only when enabled on the site |
| `store` | Whether the upstream may store requests/responses; allowed by default, can be disabled in site settings |
| `safety_identifier` | Safety identifier; filtered out by default, requires enabling on the site |

## Multimodal Input

### OpenAI Chat

`messages[].content` can be a string or an array of content blocks:

```json
{
  "role": "user",
  "content": [
    {"type": "text", "text": "Describe this image"},
    {"type": "image_url", "image_url": {"url": "https://example.com/cat.jpg", "detail": "high"}}
  ]
}
```

| Content block | Field | Description |
| --- | --- | --- |
| Text | `{"type":"text","text":"..."}` | Plain text |
| Image | `{"type":"image_url","image_url":{"url":"...","detail":"high"}}` | `url` supports a public URL or `data:image/...;base64,...` |
| Audio | `{"type":"input_audio","input_audio":{"data":"<base64>","format":"wav"}}` | Supported by some models |
| File | `{"type":"file","file":{"file_data":"data:...","filename":"a.pdf"}}` | Supported by some models |
| Video | `{"type":"video_url","video_url":{"url":"https://..."}}` | Supported by some multimodal models |

### OpenAI Responses

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

Common input items: `input_text`, `input_image`, `input_file`, and the tool result `function_call_output`.

### Anthropic Messages

```json
{
  "role": "user",
  "content": [
    {"type": "text", "text": "Describe this image"},
    {
      "type": "image",
      "source": {"type": "base64", "media_type": "image/jpeg", "data": "<base64>"}
    }
  ]
}
```

URL images are also supported:

```json
{"type": "image", "source": {"type": "url", "url": "https://example.com/cat.jpg"}}
```

### Gemini

```json
{
  "contents": [{
    "role": "user",
    "parts": [
      {"text": "Describe this image"},
      {"inlineData": {"mimeType": "image/jpeg", "data": "<base64>"}}
    ]
  }]
}
```

For large files, use `fileData`:

```json
{"fileData": {"mimeType": "image/jpeg", "fileUri": "https://generativelanguage.googleapis.com/v1beta/files/..."}}
```

Set `fileUri` to the file URI returned by the Gemini File API, or to a public file URL explicitly supported by the upstream. `fileData` field names use camelCase (`mimeType`, `fileUri`), consistent with the native Gemini API.

## Streaming Responses

All text protocols support SSE. Basic read pattern:

```bash
curl -N https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-6-luna-m","messages":[{"role":"user","content":"Write a short poem"}],"stream":true}'
```

| Protocol | End marker | Description |
| --- | --- | --- |
| OpenAI Chat | `data: [DONE]` | Each chunk is an incremental `choices[].delta` |
| OpenAI Responses | `response.completed` event | See the `event:` field for the event type |
| Anthropic | `message_stop` event | Events such as `content_block_delta` are also sent |
| Gemini | Connection closed | Each `data:` is a `GenerateContentResponse` |

Usage for a streaming request may appear in the final chunk or in the `message_delta` / `response.completed` event. For OpenAI Chat, you must pass `stream_options.include_usage=true` to get a separate usage chunk; some models do not support this field, and usage is still attached to the final chunk that carries `finish_reason`.

## Error Handling and Retries

- **400**: invalid parameters, or the model does not support the parameter. Fix the request according to `error.message` in the response; do not retry blindly.
- **401 / 403**: key or group permission issue. Check that the key is valid and that its group includes the model.
- **404**: the model ID does not exist, or the model does not support the current protocol. For example, the model supports only the Anthropic protocol but was called via `/v1/chat/completions`.
- **429**: rate limited or upstream saturated. Retry with exponential backoff and reduce concurrency.
- **500 / 502 / 503**: upstream or gateway error. You can retry; if it keeps failing, contact support with the request time, model ID, and the request ID from the response (if any).

::: warning
Do not automatically retry 400-class errors. Parameter errors usually do not recover on retry and only add invalid calls.
:::
