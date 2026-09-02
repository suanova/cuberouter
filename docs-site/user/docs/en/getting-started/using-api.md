# Use the API

This page covers API examples and supported endpoints.

## Get the API Base URL

Use your current domain as `base_url` in the following examples, and start calling with your key.

## Code examples

**OpenAI-compatible format**:

```bash
curl https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "your-model-id",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

**Responses API**:

```bash
curl https://<base_url>/v1/responses \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "your-model-id",
    "input": "Hello!"
  }'
```

**Claude native format**:

```bash
curl https://<base_url>/v1/messages \
  -H "x-api-key: sk-xxxxxxxx" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model": "claude-3-5-sonnet-20241022",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello"}]}'
```

**Gemini native format**:

```bash
curl "https://<base_url>/v1beta/models/gemini-1.5-pro:generateContent?key=sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{"contents": [{"parts": [{"text": "Hello"}]}]}'
```

## Supported endpoints

| Endpoint | Path | Description |
| --- | --- | --- |
| Chat completions | `POST /v1/chat/completions` | Conversational generation, streaming supported |
| Text completions | `POST /v1/completions` | Legacy completions endpoint |
| Embeddings | `POST /v1/embeddings` | Text vectorization |
| Image generation | `POST /v1/images/generations` | Text-to-image |
| Image editing | `POST /v1/images/edits` | Image editing |
| Video generation | `POST /v1/video/generations` | Text-to-video / image-to-video, async task submission |
| Video task lookup | `GET /v1/video/generations/{task_id}` | Query video task status and result |
| Speech-to-text | `POST /v1/audio/transcriptions` | Whisper, etc. |
| Text-to-speech | `POST /v1/audio/speech` | TTS |
| Reranking | `POST /v1/rerank` | Document reranking |
| Responses API | `POST /v1/responses` | OpenAI Responses format |
| Realtime | `GET /v1/realtime` (WebSocket) | OpenAI Realtime API |
| Model list | `GET /v1/models` | Query available models |

## Video generation example

Video generation is an async task: submitting returns a `task_id` immediately, then poll the lookup endpoint until `status` becomes `SUCCESS` or `FAILURE`, and finally download the video from the response. The script below requires `curl` and `jq`:

```bash
#!/bin/bash

# ===== SETUP =====
BASE_URL="<base_url>"
TOKEN_KEY="sk-xxxxxxxx"      # your platform-issued API key
PROMPT="A rabbit running on the beach"
MODEL="your-video-model"     # pick a video model available in Model Square, e.g. viduq3-pro
DURATION=5                  # video duration in seconds, check what the model supports
SIZE="540p"                 # resolution, check what the model supports

# 1. Submit the video generation task
echo "Submitting video generation request..."
RESP_SUBMIT=$(curl -s -X POST https://$BASE_URL/v1/video/generations \
  -H "Authorization: Bearer $TOKEN_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"model\":\"$MODEL\",\"prompt\":\"$PROMPT\",\"duration\":$DURATION,\"size\":\"$SIZE\"}")

TASK_ID=$(echo "$RESP_SUBMIT" | jq -r '.task_id')
echo "Task ID: $TASK_ID"

# 2. Poll the task status
while true; do
  RESP_QUERY=$(curl -s "https://$BASE_URL/v1/video/generations/$TASK_ID" \
    -H "Authorization: Bearer $TOKEN_KEY")

  STATUS=$(echo "$RESP_QUERY" | jq -r '.data.status')
  echo "Current status: $STATUS"

  if [[ "$STATUS" == "SUCCESS" ]]; then
    # Prefer the upstream creations URL, fall back to the gateway result_url
    VIDEO_URL=$(echo "$RESP_QUERY" | jq -r '.data.data.creations[0].url // empty')
    if [[ -z "$VIDEO_URL" ]]; then
      VIDEO_URL=$(echo "$RESP_QUERY" | jq -r '.data.result_url // empty')
    fi
    echo "Video ready, downloading: $VIDEO_URL"
    curl -o ./output.mp4 "$VIDEO_URL"
    echo "Video saved to ./output.mp4"
    break
  fi

  if [[ "$STATUS" == "FAILURE" ]]; then
    echo "Task failed: $STATUS"
    exit 1
  fi

  sleep 10
done
```
