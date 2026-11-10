# Use the API

> A quick overview of the API base URL, authentication, and available endpoints. For parameter details, model differences, and complete examples, see the [Model API Reference](/en/models/).

## Get the API Base URL

Use your current site domain as `base_url`, together with a key created on the **API Keys** page.

```text
https://<base_url>
```

## Authentication

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

Each protocol also accepts its native authentication header:

| Protocol | Authentication |
| --- | --- |
| OpenAI | `Authorization: Bearer sk-...` |
| Anthropic | `x-api-key: sk-...` or `Authorization: Bearer sk-...` |
| Gemini | `x-goog-api-key: sk-...` or `?key=sk-...` |

## Quick examples

**OpenAI Chat Completions**:

```bash
curl https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "your-model-id",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

**OpenAI Responses**:

```bash
curl https://<base_url>/v1/responses \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "your-model-id",
    "input": "Hello!"
  }'
```

**Anthropic Messages**:

```bash
curl https://<base_url>/v1/messages \
  -H "x-api-key: sk-xxxxxxxx" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "your-model-id",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

**Gemini generateContent**:

```bash
curl "https://<base_url>/v1beta/models/your-model-id:generateContent" \
  -H "x-goog-api-key: sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}'
```

## Supported endpoints

| Endpoint | Path | Details |
| --- | --- | --- |
| Chat completions | `POST /v1/chat/completions` | [OpenAI Protocol](/en/models/llm/openai) |
| Text completions | `POST /v1/completions` | [OpenAI Protocol](/en/models/llm/openai) |
| Responses | `POST /v1/responses` | [OpenAI Protocol](/en/models/llm/openai#responses-api) |
| Realtime | `GET /v1/realtime` (WebSocket) | — |
| Claude native format | `POST /v1/messages` | [Anthropic Protocol](/en/models/llm/anthropic) |
| Gemini native format | `POST /v1beta/models/{model}:generateContent` | [Gemini Protocol](/en/models/llm/gemini) |
| Image generation | `POST /v1/images/generations` | [Image Generation](/en/models/image) |
| Image editing | `POST /v1/images/edits` | [Image Generation](/en/models/image) |
| Video generation | `POST /v1/video/generations` | [Video Generation](/en/models/video) |
| Video task lookup | `GET /v1/video/generations/{task_id}` | [Video Generation](/en/models/video) |
| Text-to-speech | `POST /v1/audio/speech` | [Audio Generation](/en/models/audio) |
| Speech-to-text | `POST /v1/audio/transcriptions` | [Audio Generation](/en/models/audio) |
| Embeddings | `POST /v1/embeddings` | [Model API Reference](/en/models/) |
| Reranking | `POST /v1/rerank` | [Model API Reference](/en/models/) |
| Model list | `GET /v1/models` | [Model API Reference](/en/models/) |

## Next steps

- Find model support and parameter differences: [Model API Reference](/en/models/)
- Learn about common fields and error handling: [Common Parameters](/en/models/common-parameters)
- Use Claude Code: [Claude Code](/en/getting-started/claude-code)
- Use Codex: [Codex](/en/getting-started/codex)
