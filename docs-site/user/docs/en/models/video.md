# Video Generation

> Submit video generation requests through the asynchronous task API and poll for status and the video URL.

Video generation is an **asynchronous task**: the submit endpoint returns a `task_id` immediately, then you poll the query endpoint until the task succeeds or fails. Models differ in the generation modes, resolutions, durations, and reference material types they support.

## Endpoints

| Purpose | Method and path |
| --- | --- |
| Submit task (recommended) | `POST /v1/video/generations` |
| Query task | `GET /v1/video/generations/{task_id}` |
| OpenAI video compatible - submit | `POST /v1/videos` |
| OpenAI video compatible - query | `GET /v1/videos/{task_id}` |
| OpenAI video compatible - download | `GET /v1/videos/{task_id}/content` |
| Ark-style - submit | `POST /v1/videos/generations/tasks` |
| Ark-style - query | `GET /v1/videos/generations/tasks/{task_id}` |

Request headers:

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

> For new integrations, we recommend using `/v1/video/generations` consistently. `/v1/videos` suits clients already using the OpenAI video API; Ark-style endpoints are only exposed for some upstreams.
>
> Model availability is limited by the key group; the models visible in Model Square are authoritative.

## Supported Models

| Series | Site models | Capabilities |
| --- | --- | --- |
| Seedance | `seedance-2.0-a`, `seedance-2.5-a` | Text-to-video, image-to-video, first/last frame, reference-to-video |
| Doubao Seedance | `doubao-seedance-2-0-h`, `doubao-seedance-2-5-h`, `doubao-seedance-2-5-260628` | Text-to-video, image-to-video, reference-to-video, native audio |
| Wan 3.0 | `wan3.0-video-a`, `wan3.0-video`, `wan3.0-video-prime` | Text-to-video, first/last frame, reference image/video/audio |
| MiniMax H3 | `minimax-h3-a`, `MiniMax-H3` | Text-to-video, first/last frame, multimodal reference |
| Kling | `kling-v3`, `kling-v3-omni` | Text-to-video, image-to-video, motion control, multimodal reference |
| Vidu | `viduq3-pro`, `viduq3-turbo` (including `-fmi` variants) | Text-to-video, image-to-video, reference-to-video |

## Submit Task

### Common Parameters

```json
{
  "model": "seedance-2.0-a",
  "prompt": "a cat walking in the rain, cinematic",
  "size": "1280x720",
  "duration": 5
}
```

| Parameter | Type | Required | Description |
| --- | --- | :---: | --- |
| `model` | string | Yes | Site video model ID |
| `prompt` | string | Yes* | Video description; may be omitted when text is passed directly via `content` |
| `image` | string | No | Single image URL, equivalent to `images: [url]` |
| `images` | string[] | No | Array of image URLs; see each series for the meaning of different counts |
| `input_reference` | string | No | Single reference image URL, handled as image-to-video |
| `content` | array | No | Ark-style content array for first frame/last frame/reference material; only some series support it |
| `size` | string | No | Resolution or dimensions, such as `1280x720`, `720p`, `480P` |
| `resolution` | string | No | Resolution tier, such as `480p`, `720p`, `1080p`, `768P`, `2K` |
| `ratio` | string | No | Aspect ratio, such as `16:9`, `9:16`, `1:1`, `adaptive` |
| `duration` | integer | No | Output duration in seconds, usually defaults to `5` |
| `seconds` | string | No | String alias for `duration` |
| `metadata` | object | No | Model-specific parameters, such as `job_type`, `media`, `mode`, `sound` |

\* Use either `prompt` or the text in `content`; if both are absent, a parameter error is returned.

### Common Rules

- Image, video, and audio material URLs must be **public HTTP(S) addresses** the upstream can access; they cannot point to localhost, intranet addresses, or pages that require login.
- For material that requires authentication, download it to object storage or a CDN first, then pass the public URL.
- Duration and resolution affect both the generation result and the cost; pass them explicitly rather than relying on defaults.
- Different models use different field names under `metadata`; each series is described below.

### Submit Response

```json
{
  "id": "task_J7xRonpligwgybxmcTNY8HJELhFfIIdE",
  "task_id": "task_J7xRonpligwgybxmcTNY8HJELhFfIIdE",
  "status": "queued",
  "model": "seedance-2.0-a",
  "created_at": 1793093555
}
```

Save the `task_id` for subsequent polling.

## Query Task

```bash
curl https://<base_url>/v1/video/generations/task_J7xRonpligwgybxmcTNY8HJELhFfIIdE \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY"
```

Common query response:

```json
{
  "code": "success",
  "data": {
    "task_id": "task_J7xRonpligwgybxmcTNY8HJELhFfIIdE",
    "status": "SUCCESS",
    "progress": "100%",
    "result_url": "https://cdn.example.com/result.mp4",
    "data": {}
  }
}
```

Status values:

| Status | Meaning |
| --- | --- |
| `SUBMITTED` / `QUEUED` | Submitted, waiting to be processed |
| `IN_PROGRESS` | Generating |
| `SUCCESS` | Succeeded; the video URL is in `result_url` |
| `FAILURE` | Failed; the reason is in `fail_reason` or `error` |

On success, read `data.result_url` first; some upstreams keep the original response (e.g. `creations`) in `data.data` as a fallback.

### OpenAI Video Compatible Format

The `/v1/videos` family returns OpenAI video objects:

```json
{
  "id": "task_...",
  "object": "video",
  "model": "seedance-2.0-a",
  "status": "completed",
  "progress": 100,
  "created_at": 1793093555,
  "completed_at": 1793093620,
  "seconds": "5",
  "size": "1280x720"
}
```

`status` values: `queued`, `in_progress`, `completed`, `failed`. After the task succeeds, you can download the video bytes via `GET /v1/videos/{id}/content`.

## Parameters by Series

### Seedance 2.0 / 2.5 (`seedance-2.0-a`, `seedance-2.5-a`)

| Item | Description |
| --- | --- |
| Mode | Text-to-video, image-to-video, first/last frame, reference-to-video |
| Resolution | `480P`, `720P`, `1080P`; also accepts `832x480`, `1280x720`, `1920x1080`, etc. |
| Duration | `2–30` seconds, default `5` |
| Ratio | `16:9` (default), `9:16`, `4:3`, `3:4`, `1:1`, `adaptive` |
| Images | `image` or `images`: 1 image = image-to-video, 2 images + `metadata.job_type=first_last_frame` = first/last frame |
| Reference material | `metadata.medias`, with elements of the form `{"type":"...","url":"..."}`; `type` is `image`, `video`, or `audio` |
| Job type | `metadata.job_type`: `text_to_video`, `image_to_video`, `first_last_frame`, `reference_to_video` |
| Count | `metadata.n` only allows `1` |

Notes:

- Use top-level `size` / `duration`; do **not** use `metadata.resolution` / `metadata.duration`, or the request may be rejected.
- This series does not support the Ark-style `content` array; use `prompt` + `images` + `metadata.medias`.
- For `metadata.medias`, use either `url` or `media_id`; `url` must be a public address.

Example: first/last frame

```json
{
  "model": "seedance-2.5-a",
  "prompt": "the flower blooms",
  "size": "720P",
  "duration": 5,
  "images": [
    "https://example.com/first.jpg",
    "https://example.com/last.jpg"
  ],
  "metadata": {"job_type": "first_last_frame"}
}
```

### Doubao Seedance (`doubao-seedance-2-0-h`, `doubao-seedance-2-5-h`, `doubao-seedance-2-5-260628`)

| Item | Description |
| --- | --- |
| Mode | Text-to-video, image-to-video, reference-to-video, native audio |
| Resolution | `480p`, `720p`, `1080p`; 2.0 additionally supports `4k` |
| Duration | Defaults to `5` seconds; pass explicitly |
| Ratio | `16:9`, `9:16`, `1:1`, `4:3`, `3:4`, `adaptive` (subject to actual support) |
| Images | `image` or `images` |
| Fine-grained control | `content` array, supports first frame/last frame/reference image/reference video/reference audio |
| Audio | `metadata.generate_audio`; 2.5 enables native audio by default |

Notes:

- Use either `prompt` or the text in `content`.
- 2.5 and 2.0 have different maximum resolutions; only 2.0 supports 4K.
- `content` is an Ark-style array, used differently from Seedance (`seedance-*-a`).

Example: image-to-video

```json
{
  "model": "doubao-seedance-2-5-h",
  "prompt": "the dog starts running across the grass",
  "image": "https://example.com/dog.jpg",
  "resolution": "720p",
  "ratio": "16:9",
  "duration": 5
}
```

### Wan 3.0 (`wan3.0-video-a`, `wan3.0-video`, `wan3.0-video-prime`)

| Item | Description |
| --- | --- |
| Mode | Text-to-video, first frame, first/last frame, reference image/video/audio |
| Resolution | `480p`, `720p`, `1080p` |
| Duration | `2–30` seconds, default `5` |
| Ratio | `21:9`, `16:9`, `4:3`, `1:1`, `3:4`, `9:16`, `adaptive` |
| Images | `images`: 1 image = first frame, 2 images = first/last frame, 3 or more = reference images |
| Full material control | `metadata.media`, supports `first_frame`, `last_frame`, `reference_image`, `reference_video`, `reference_audio`, `file`, `link` |

Material limits:

- First/last frame and reference materials are **mutually exclusive** and cannot be mixed.
- Reference images ≤ 10, reference videos ≤ 5 clips (15 seconds total), reference audio ≤ 5 clips (15 seconds total).
- Up to 20 multimodal materials per request in total.
- `file` and `link` are each limited to one and must be publicly accessible pages.
- `wan3.0-video-a` does not support the Ark-style `content` array; use `prompt` + `images` / `metadata.media`.

Example: reference-to-video

```json
{
  "model": "wan3.0-video-a",
  "prompt": "Make the cat in the reference image run on the grass, with the camera slowly following",
  "resolution": "720p",
  "duration": 5,
  "metadata": {
    "media": [
      {"type": "reference_image", "url": "https://example.com/cat.jpg"}
    ]
  }
}
```

### MiniMax H3 (`minimax-h3-a`, `MiniMax-H3`)

| Item | Description |
| --- | --- |
| Mode | Text-to-video, first frame, first/last frame, multimodal reference |
| Resolution | `768P`, `2K` |
| Duration | `4–15` seconds, default `5` |
| Ratio | `16:9`, `4:3`, `1:1`, `3:4`, `9:16`, `21:9` |
| Images | `images`: 1 image = first frame, 2 images = first/last frame, 3 or more = reference images |
| Full material control | `metadata.media`: `first_frame`, `last_frame`, `image_url`, `feature`, `driving_audio` |
| Watermark | `metadata.watermark=true` |

Material limits:

- First/last frame and reference materials are mutually exclusive.
- One first frame and one last frame; up to 9 reference images, 3 reference videos, and 3 reference audio clips.
- **Text-to-video must explicitly specify `ratio`, and it cannot be `adaptive`**; it defaults to `16:9` when omitted.
- Reference scenarios default to `adaptive`.

Example: text-to-video

```json
{
  "model": "minimax-h3-a",
  "prompt": "An orange tabby cat crouching on the windowsill watching the rain, cinematic, slow push-in",
  "resolution": "768P",
  "ratio": "16:9",
  "duration": 5
}
```

### Kling (`kling-v3`, `kling-v3-omni`)

| Item | Description |
| --- | --- |
| Capabilities | Text-to-video, image-to-video, motion control, multimodal reference (omni) |
| Images | `images`: 1 image = first frame, 2 images = first/last frame |
| Mode | `metadata.mode`: `std`, `pro`, `4k` |
| Sound | `metadata.sound`: `on`, `off` |
| Type | `metadata.kling_v3_type`: `t2v`, `i2v`, `motion_control`; inferred from the materials when omitted |
| Ratio | `metadata.aspect_ratio` |
| Motion control | `metadata.img_url`, `metadata.video_url` |

- `kling-v3` supports text-to-video, image-to-video, first/last frame, and motion control, and can generate videos with sound.
- `kling-v3-omni` focuses more on subject-consistent generation from multiple reference images/videos and on video editing.

### Vidu (`viduq3-pro`, `viduq3-turbo`)

| Item | Description |
| --- | --- |
| Mode | Text-to-video, image-to-video, reference-to-video |
| Resolution | `360p`, `540p`, `720p`, `1080p`; Q3 defaults to `720p` |
| Duration | Defaults to `5` seconds |
| Images | `image`, `images`, or `input_reference` |
| Notes | `pro` favors quality and `turbo` favors speed; `-fmi` variants are intended for specific organizations, and availability depends on the group |

## Polling Example

```bash
#!/bin/bash
set -e

BASE_URL="https://<base_url>"
API_KEY="$CUBEROUTER_API_KEY"

RESP=$(curl -sS "$BASE_URL/v1/video/generations" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "seedance-2.0-a",
    "prompt": "A rabbit running on the beach",
    "size": "1280x720",
    "duration": 5
  }')

TASK_ID=$(echo "$RESP" | jq -r '.task_id')
echo "task_id: $TASK_ID"

while true; do
  QUERY=$(curl -sS "$BASE_URL/v1/video/generations/$TASK_ID" \
    -H "Authorization: Bearer $API_KEY")

  STATUS=$(echo "$QUERY" | jq -r '.data.status')
  echo "status: $STATUS"

  case "$STATUS" in
    SUCCESS)
      URL=$(echo "$QUERY" | jq -r '.data.result_url')
      curl -L -o output.mp4 "$URL"
      echo "saved to output.mp4"
      break
      ;;
    FAILURE)
      echo "$QUERY" | jq -r '.data.fail_reason // .data.error // "task failed"'
      exit 1
      ;;
  esac

  sleep 5
done
```

## FAQ

### Q1: Submission succeeds but the task stays `IN_PROGRESS`?

Video generation usually takes tens of seconds to several minutes; complex models, long durations, and high resolutions take longer. Poll every 5–10 seconds and set the client total timeout to at least 10 minutes. If there is no progress for a long time, check the task log for the failure reason.

### Q2: I passed a material URL but the upstream cannot read it?

Materials must be public HTTP(S) addresses accessible to the upstream. URLs pointing to localhost, intranet addresses, pages that require login, or expired temporary signatures all fail. Put materials in object storage or a CDN first.

### Q3: Can first/last frame and reference materials be used together?

Wan 3.0 and MiniMax H3 both **do not allow** mixing first/last frame with reference materials. When you need a first/last frame, pass only the first/last frame; when you need reference images/videos/audio, use reference mode.

### Q4: Why does another model fail with the same parameters?

Different series use different `metadata` field names and value ranges. For example, Seedance (`seedance-*-a`) uses `metadata.medias` and `metadata.job_type`, Doubao Seedance uses `content`, and Wan uses `metadata.media`. Pass parameters according to the model's series; do not apply them across series.

### Q5: What is the difference between `/v1/videos` and `/v1/video/generations`?

- `/v1/video/generations` is the site's general task API; the response includes fields such as `task_id`, `status`, and `result_url`.
- `/v1/videos` is the OpenAI video-compatible API; it returns OpenAI video objects with status values `queued` / `in_progress` / `completed` / `failed`, and you can download via `/content` after success.
- Both provide the same generation capabilities; pick one.

### Q6: How do I troubleshoot after a task fails?

First check `fail_reason` or `error` in the query response, then compare with the task log. Common causes include inaccessible material URLs, parameters outside the model's range, and upstream content moderation rejection. Fix the issue and resubmit.
