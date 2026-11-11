# Model API Reference

> An API reference organized by protocol and model type: choose a model and protocol first, then look up parameters, examples, and limits.

CubeRouter unifies models from different providers behind a single Base URL and API key. You can keep using your existing OpenAI, Anthropic, or Gemini SDK — just replace the Base URL and model name.

::: tip URL notation in this document
In the examples, `https://<base_url>` refers to the domain of the CubeRouter site you are currently using, for example `https://cuberouter.cn`. All API paths start with `/v1` or `/v1beta`.
:::

## Quick Start

### 1. Get an API Key

After signing in to the site, create a key on the **API Keys** page. The key's **group** determines which models you can call; if a request returns model not found or no permission, check the key group first.

Put the key in the request header:

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

The Anthropic protocol also accepts `x-api-key`, and the Gemini protocol also accepts `x-goog-api-key` or `?key=`. See the individual protocol pages.

### 2. Choose a Model

In **Model Square**, filter by endpoint type, provider, or tag, then copy the full model ID. In requests, `model` must be the site model ID, not the original model name from the upstream provider.

A model may have several suffixed variants, such as `-a`, `-b`, `-m`, `-h`, and `-j`. These are **different site models**, possibly provided by different upstreams, with different parameter support and available groups; they cannot be substituted for one another. For example, `gpt-image-2.5-flare` and `gpt-image-2.5-flare-m` are two different models.

### 3. Your First Request

```bash
curl https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-6-luna-m",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Hello, tell me about yourself"}
    ]
  }'
```

Replace `gpt-6-luna-m` with the model ID you copied from Model Square.

## Protocol Overview

| Model type | Protocol | Endpoint | Reference |
| --- | --- | --- | --- |
| Large language model | OpenAI Chat Completions | `POST /v1/chat/completions` | [OpenAI Protocol](/en/models/llm/openai) |
| Large language model | OpenAI Responses | `POST /v1/responses` | [OpenAI Protocol](/en/models/llm/openai#responses-api) |
| Large language model | Anthropic Messages | `POST /v1/messages` | [Anthropic Protocol](/en/models/llm/anthropic) |
| Large language model | Gemini generateContent | `POST /v1beta/models/{model}:generateContent` | [Gemini Protocol](/en/models/llm/gemini) |
| Image generation | OpenAI Images | `POST /v1/images/generations`, `POST /v1/images/edits` | [Image Generation](/en/models/image) |
| Video generation | Asynchronous task | `POST /v1/video/generations`, `GET /v1/video/generations/{task_id}` | [Video Generation](/en/models/video) |
| Audio generation | OpenAI Audio | `POST /v1/audio/speech`, `POST /v1/audio/transcriptions` | [Audio Generation](/en/models/audio) |
| Embeddings / reranking | OpenAI Embeddings / Jina Rerank | `POST /v1/embeddings`, `POST /v1/rerank` | Not covered in this document yet |

Recommendations:

- **Prefer the model's native protocol.** Vendor-specific fields are best supported when Claude uses the Anthropic protocol and Gemini uses the Gemini protocol; when you call through another protocol, the gateway converts the format, and some parameters may be ignored or return 400.
- **Keep existing code unchanged**: continue using OpenAI Chat Completions; most text models support this protocol.
- **Codex CLI**: use the OpenAI Responses protocol.
- **Claude Code**: use the Anthropic protocol; see [Claude Code](/en/getting-started/claude-code) for configuration.

## Model Families and Protocol Support

The table below summarizes support by family. For support for each individual model, see the model tables on each protocol page.

### Choosing by Use Case (Reference)

`*` in the table below means the family includes multiple suffixed variants; use the full ID from Model Square in actual calls.

| Use case | Preferred candidates | Description |
| --- | --- | --- |
| High throughput, low latency | `gpt-5.6-luna-*`, `gemini-3.5/3.7/3.8-flash-b`, `claude-haiku-5-5-*`, `deepseek-v4-flash-*`, `minimax-m3.1-flash-a` | Lightweight/fast tier, suited to classification, summarization, and real-time interaction |
| Complex reasoning, agents | `claude-opus-5-5-*`, `gpt-6-astra-*`, `gemini-3.1-pro-preview*`, `deepseek-v4-pro-a`, `qwen3.8-max-a` | Flagship tier, suited to long multi-step tasks and highly complex reasoning |
| Coding, long-horizon tasks | `claude-opus-5-5-*`, `gpt-6-sol-*`, `kimi-k2.7-code-*`, `glm-5.3-*`, `qwen3.8-max-a` | Organized by vendor positioning; refer to Model Square descriptions for details |
| Long context | Models tagged `1M`, such as Claude 5.5, GPT 5.6/6, Gemini 3.1, DeepSeek V4, Kimi K3, Qwen 3.x, GLM 5.2/5.3 | Suited to long documents and codebase analysis |
| Multimodal input | `gemini-3.x`, `doubao-seed-2-1-*`, `deepseek-v4.1-flash-*`, `kimi-k3-*`, `qwen3.x` | Supports image or video input |

> The above are references based on model positioning only and do not constitute a performance ranking. Actual selection should also take into account available groups, context length, tool-calling requirements, and measured results.

### Large Language Models

| Family | Representative models | Chat | Responses | Anthropic | Gemini |
| --- | --- | :---: | :---: | :---: | :---: |
| Claude | `claude-opus-5-5-j`, `claude-sonnet-5-5-j`, `claude-haiku-5-5-j`, etc. | ✓ | Partial | ✓ | — |
| GPT | `gpt-6-astra-m`, `gpt-6-sol-m`, `gpt-5.6-terra-m`, etc. | ✓ | ✓ | Partial | — |
| Gemini | `gemini-3.8-flash-b`, `gemini-3.1-pro-preview`, etc. | ✓ | Partial | Partial | ✓ |
| DeepSeek | `deepseek-v4-pro-a`, `deepseek-v4.1-flash-a`, etc. | ✓ | ✓ | ✓ | — |
| Kimi | `kimi-k3-a`, `kimi-k3-j`, `kimi-k2.7-code-a`, etc. | ✓ | Partial | Partial | — |
| Qwen | `qwen3.8-max-a`, `qwen3.7-plus-a`, etc. | ✓ | ✓ | ✓ | — |
| GLM | `glm-5.3-z`, `glm-5.2-a`, etc. | ✓ | Partial | Partial | — |
| Doubao | `doubao-seed-2-1-pro-h`, `doubao-seed-2-1-turbo-h` | ✓ | ✓ | ✓ | — |
| MiniMax | `minimax-m3.1-flash-a`, `minimax-m2.7-a` | ✓ | — | — | — |

### Image Generation

| Family | Site models | Text-to-image | Image editing |
| --- | --- | :---: | :---: |
| GPT Image | `gpt-image-2.5-flare-m`, `gpt-image-2.5-sunburst-m`, `gpt-image-2-m`, `gpt-image-1.5-m`, etc. | ✓ | ✓ |
| Gemini Image | `gemini-3.1-flash-image-m`, `gemini-3-pro-image-m` | ✓ | ✓ |
| Qwen Image | `qwen-image-2.1-turbo` | ✓ | ✓ |
| Wan Image | `wan2.7-image-a` | ✓ | — |
| MOSS Image | `moss-image` | ✓ | — |

### Video Generation

| Family | Site models | Capabilities |
| --- | --- | --- |
| Seedance | `seedance-2.0-a`, `seedance-2.5-a` | Text-to-video, image-to-video, first/last frame, reference-to-video |
| Doubao Seedance | `doubao-seedance-2-0-h`, `doubao-seedance-2-5-h`, `doubao-seedance-2-5-260628` | Text-to-video, image-to-video, reference-to-video, native audio |
| Wan 3.0 | `wan3.0-video-a`, `wan3.0-video`, `wan3.0-video-prime` | Text-to-video, first/last frame, reference image/video/audio |
| MiniMax H3 | `minimax-h3-a`, `MiniMax-H3` | Text-to-video, first/last frame, multimodal reference |
| Kling | `kling-v3`, `kling-v3-omni` | Text-to-video, image-to-video, motion control, multimodal reference |
| Vidu | `viduq3-pro`, `viduq3-turbo` (including `-fmi` variants) | Text-to-video, image-to-video, reference-to-video |

### Audio Generation

| Type | Site models | Description |
| --- | --- | --- |
| Text-to-speech (TTS) | `speech-2.8-hd`, `speech-2.8-turbo`, `moss-tts-1.5-flash-2026-06-26` | Text to speech |
| Speech-to-text (ASR) | `moss-transcribe-diarize-flash-2026-09-02` | Transcription, timestamps, speaker diarization |

## General Conventions

### Request Format

- Request bodies use UTF-8 JSON; image editing and speech-to-text use `multipart/form-data`.
- All endpoints use the same API key; there is no need to create separate keys for different protocols.
- Model names are case-sensitive and must exactly match the full ID in Model Square.

### Streaming Responses

Text and text-to-speech support streaming responses over SSE (`text/event-stream`). Event formats differ by protocol; see the individual protocol pages. For image generation, only some models support streaming previews.

### Synchronous vs. Asynchronous

- Text, image, and audio are synchronous requests: the connection stays open until the result is returned.
- Video generation is an asynchronous task: submission returns a `task_id` immediately, then poll the lookup endpoint for status and results.

### Error Format

Error response structures differ by protocol, but all include a readable error message:

```jsonc
// OpenAI protocol
{"error": {"message": "...", "type": "...", "code": "..."}}

// Anthropic protocol
{"type": "error", "error": {"type": "...", "message": "..."}}

// Gemini protocol
{"error": {"code": 400, "message": "...", "status": "INVALID_ARGUMENT"}}

// Video task
{"code": "...", "message": "...", "data": null}
```

Common status codes:

| Status code | Meaning | Suggested action |
| --- | --- | --- |
| 400 | Invalid request parameters | Check the model name, required parameters, and value ranges |
| 401 | Invalid or missing key | Check `Authorization` / `x-api-key` / `x-goog-api-key` |
| 403 | No permission | Check whether the key's group includes the model |
| 404 | Model does not exist or endpoint not supported | Confirm the model ID and that the model supports the current protocol |
| 429 | Rate limited or upstream saturated | Reduce concurrency and retry later |
| 500 / 502 / 503 | Upstream or gateway error | Retry later; if it persists, contact support |

### Model Availability and Timeouts

- Model availability is determined by the **key's group**; Model Square shows accessible models by group.
- Image generation usually takes 10–60 seconds; video generation usually takes tens of seconds to several minutes. Set client timeouts to at least 60 seconds, and poll video tasks every 5–10 seconds.
- This document only covers how to call models and does not cover pricing or billing. Refer to Model Square and your actual bills for pricing.

## Next Steps

- [Common Parameters](/en/models/common-parameters): shared fields across protocols, multimodal input, and error handling.
- [OpenAI Protocol](/en/models/llm/openai): Chat Completions and Responses.
- [Anthropic Protocol](/en/models/llm/anthropic): Messages API and Claude Code.
- [Gemini Protocol](/en/models/llm/gemini): generateContent and native multimodal input.
- [Image Generation](/en/models/image), [Video Generation](/en/models/video), [Audio Generation](/en/models/audio): parameters and examples by modality.
