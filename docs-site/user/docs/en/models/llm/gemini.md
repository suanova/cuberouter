# Gemini Protocol

> Call Gemini family models using the native Gemini `generateContent` format.

The Gemini protocol is suitable for applications that need native multimodal input, `safetySettings`, `thinkingConfig`, or Google SDK compatibility. Most Gemini models on the site also support the OpenAI protocol; use this protocol when you need Gemini-specific fields.

## Endpoints

| Purpose | Method and path |
| --- | --- |
| Generate content | `POST /v1beta/models/{model}:generateContent` |
| Streaming generation | `POST /v1beta/models/{model}:streamGenerateContent?alt=sse` |
| Model list | `GET /v1beta/models` |

Authentication (choose one of three):

```http
x-goog-api-key: sk-xxxxxxxx
```

```http
Authorization: Bearer sk-xxxxxxxx
```

```text
POST /v1beta/models/gemini-3.8-flash-b:generateContent?key=sk-xxxxxxxx
```

## Supported models

| Model | Description |
| --- | --- |
| `gemini-3.1-flash-lite-m` | Lightweight economy tier, low latency and high throughput |
| `gemini-3.1-pro-preview` | Pro preview, strong reasoning and multimodal, 1M context |
| `gemini-3.1-pro-preview-m` | Pro preview (`-m` variant) |
| `gemini-3.5-flash-b` | Flash lightweight flagship, low-latency multimodal |
| `gemini-3.7-flash-b` | Flash lightweight flagship |
| `gemini-3.8-flash` | Flash lightweight flagship |
| `gemini-3.8-flash-b` | Flash lightweight flagship (`-b` variant) |

> `gemini-3.1-flash-lite-m` and `gemini-3.1-pro-preview-m` support only the OpenAI and Gemini protocols; the other models also support Responses / Anthropic. See [OpenAI Protocol](/en/models/llm/openai) and [Anthropic Protocol](/en/models/llm/anthropic) for details.
> Model availability is limited by key group; the models actually visible in Model Square are authoritative.

## Request

```json
{
  "contents": [{
    "role": "user",
    "parts": [{"text": "Hello, introduce yourself"}]
  }],
  "systemInstruction": {
    "parts": [{"text": "You are a concise assistant."}]
  },
  "generationConfig": {
    "temperature": 0.7,
    "maxOutputTokens": 1024
  }
}
```

### Top-level fields

| Field | Type | Description |
| --- | --- | --- |
| `contents` | array | Conversation content; each element contains `role` and `parts` |
| `systemInstruction` | object | System instructions; the snake_case form `system_instruction` is also accepted |
| `generationConfig` | object | Generation parameters |
| `safetySettings` | array | Safety filter settings |
| `tools` | array | Tool definitions |
| `toolConfig` | object | Tool calling configuration |
| `cachedContent` | string | Cached content name; supported by some models |

`contents[].role` typically uses `user` and `model`. In multi-turn conversations, put the model's previous reply back into `contents` as content with `role: "model"`.

### `parts`

| Field | Description |
| --- | --- |
| `text` | Text |
| `inlineData` | Inline binary, containing `mimeType` and `data` (base64) |
| `fileData` | File reference, containing `mimeType` and `fileUri` |
| `functionCall` | A function call initiated by the model |
| `functionResponse` | Function execution result |
| `thought` | Whether this is thinking content |
| `thoughtSignature` | Thinking signature; pass back unchanged in multi-turn/tool-calling scenarios |
| `executableCode` / `codeExecutionResult` | Results of the code execution tool |
| `videoMetadata` / `mediaResolution` | Video metadata and media resolution |

Image input example:

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

Large files can use `fileData`:

```json
{"fileData": {"mimeType": "image/jpeg", "fileUri": "https://generativelanguage.googleapis.com/v1beta/files/..."}}
```

Set `fileUri` to the file URI returned by the Gemini File API, or to a public file URL explicitly supported by the upstream. `fileData` field names use camelCase (`mimeType`, `fileUri`).

### `generationConfig`

| Field | Type | Description |
| --- | --- | --- |
| `temperature` | number | Sampling temperature |
| `topP` | number | Nucleus sampling |
| `topK` | number | Top-K |
| `maxOutputTokens` | integer | Output limit |
| `candidateCount` | integer | Number of candidates |
| `stopSequences` | string[] | Stop sequences |
| `responseMimeType` | string | For example, `application/json` |
| `responseSchema` | object | JSON Schema |
| `presencePenalty` | number | Presence penalty |
| `frequencyPenalty` | number | Frequency penalty |
| `seed` | integer | Random seed |
| `responseModalities` | string[] | Output modalities, such as `["TEXT"]`; models that support image output can add `"IMAGE"` |
| `thinkingConfig` | object | Thinking configuration |
| `mediaResolution` | string | Media resolution |
| `speechConfig` / `imageConfig` | object | Speech/image-specific configuration |

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

| Field | Description |
| --- | --- |
| `thinkingBudget` | Thinking token budget; allowed ranges differ by model |
| `thinkingLevel` | Thinking level, used by some Gemini 3 models; availability with `thinkingBudget` depends on the model |
| `includeThoughts` | Whether to return thinking content in the response |

### Safety settings

```json
{
  "safetySettings": [
    {"category": "HARM_CATEGORY_HARASSMENT", "threshold": "BLOCK_MEDIUM_AND_ABOVE"},
    {"category": "HARM_CATEGORY_HATE_SPEECH", "threshold": "BLOCK_MEDIUM_AND_ABOVE"}
  ]
}
```

Supported categories and thresholds depend on the model. When blocked by a safety policy, the response may contain `promptFeedback.blockReason` or `finishReason: "SAFETY"`.

### Tool calling

```json
{
  "tools": [{
    "functionDeclarations": [{
      "name": "get_weather",
      "description": "Query city weather",
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

`mode` can be `AUTO`, `ANY`, or `NONE`; with `ANY`, use `allowedFunctionNames` to restrict which functions can be called.

## Response

```json
{
  "candidates": [{
    "content": {
      "role": "model",
      "parts": [{"text": "Hello! How can I help you?"}]
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

- `candidates[0].content.parts` is the output content and may contain text, function calls, or thinking content.
- Common `finishReason` values: `STOP`, `MAX_TOKENS`, `SAFETY`, `RECITATION`.
- `usageMetadata` may contain fields such as `cachedContentTokenCount` and `thoughtsTokenCount`.
- When a request is blocked, `candidates` may be absent; check `promptFeedback`.

## Streaming

```bash
curl -N "https://<base_url>/v1beta/models/gemini-3.8-flash-b:streamGenerateContent?alt=sse" \
  -H "x-goog-api-key: $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [{"role": "user", "parts": [{"text": "Write a product description"}]}]
  }'
```

Each `data:` line is a `GenerateContentResponse`:

```text
data: {"candidates":[{"content":{"role":"model","parts":[{"text":"Hel"}]}}]}

data: {"candidates":[{"content":{"role":"model","parts":[{"text":"lo"}]}}],"usageMetadata":{...}}
```

Clients need to concatenate `parts[].text`; the last chunk usually contains the complete `usageMetadata`.

## Differences by family

| Model | Differences and recommendations |
| --- | --- |
| `gemini-3.8-flash-b` / `gemini-3.8-flash` | Supports all four protocols: OpenAI, Responses, Anthropic, and Gemini; native Gemini fields are the most complete. |
| `gemini-3.7-flash-b` / `gemini-3.5-flash-b` | All four protocols are supported; suitable for high-concurrency, low-latency scenarios. |
| `gemini-3.1-pro-preview` | All four protocols are supported; suitable for complex reasoning and long documents. |
| `gemini-3.1-pro-preview-m` | Supports only OpenAI and Gemini; use the Gemini protocol when you need native fields such as `safetySettings`. |
| `gemini-3.1-flash-lite-m` | Supports only OpenAI and Gemini; economy tier, suitable for high-throughput tasks. |

## FAQ

### Q1: Getting 404 or `model not found`?

Confirm the model ID matches Model Square. The Gemini protocol supports only the models in the table above; other Gemini image models (such as `gemini-3.1-flash-image-m`) are called through the [Image generation](/en/models/image) endpoint and do not use `generateContent`.

### Q2: Streaming returns nothing?

`streamGenerateContent` requires `alt=sse`; without it, the response is a JSON array stream, not SSE.

### Q3: How do I choose between `inlineData` and `fileData`?

- Use `inlineData` for small files or base64 data.
- Use `fileData` for files that have already been uploaded; set `fileUri` to the file URI returned by the Gemini File API, or to a public file URL explicitly supported by the upstream. Field names must be `mimeType` / `fileUri`.
- Do not call the Gemini protocol with OpenAI's `image_url` format.

### Q4: Response has `finishReason: "SAFETY"` and no content?

The request was blocked by a safety policy. Adjust `safetySettings` (if the model allows it), or modify the prompt and retry.

### Q5: Can `thinkingBudget` and `thinkingLevel` be used at the same time?

Do not send both. Different Gemini 3 models support different fields; choose one according to the model documentation. If unsure, use `thinkingBudget` first or leave it empty to use the default.
