# Image Generation

> Use the OpenAI Images-compatible API for text-to-image and image editing.

Image models on the site all use an OpenAI Images-style API: text-to-image uses `POST /v1/images/generations`, and image editing uses `POST /v1/images/edits`. Different models support different parameters and response formats; each series is described below.

## Endpoints

| Purpose | Method and path | Request format |
| --- | --- | --- |
| Text-to-image | `POST /v1/images/generations` | `application/json` |
| Image editing | `POST /v1/images/edits` | `multipart/form-data` or `application/json` |

Request headers:

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

When uploading files for image editing, use `multipart/form-data`. Do not set the boundary manually; let the HTTP client generate it.

## Supported Models

| Series | Site models | Text-to-image | Image editing | Notes |
| --- | --- | :---: | :---: | --- |
| GPT Image | `gpt-image-2.5-flare`, `gpt-image-2.5-flare-b`, `gpt-image-2.5-flare-m`, `gpt-image-2.5-sunburst-b`, `gpt-image-2.5-sunburst-m`, `gpt-image-2-m`, `gpt-image-2-b`, `gpt-image-1.5-m` | ✓ | ✓ | The 2.5 series is split between speed-first (`flare`) and editing-precision-first (`sunburst`) |
| Gemini Image | `gemini-3.1-flash-image-m`, `gemini-3-pro-image-m` | ✓ | ✓ | Nano Banana series, image generation and editing |
| Qwen Image | `qwen-image-2.1-turbo` | ✓ | ✓ | Fixed PNG output on the site |
| Wan Image | `wan2.7-image-a` | ✓ | — | Supports 1K/2K/4K, up to 4 images per request |
| MOSS Image | `moss-image` | ✓ | — | 512×512 to 2048×2048, supports batch generation and fixed seeds |

> Different suffixes within the same series are different models, and their parameters and available groups may differ. Use the full ID from Model Square when calling; actual availability is subject to Model Square.
>
> "Image editing" in the table means the model has editing capability; whether a specific model exposes `/v1/images/edits` depends on the actual response. The editing endpoint has been verified on `gpt-image-2.5-flare-m`, `gpt-image-2.5-sunburst-m`, and `gpt-image-2-m`.

## Text-to-Image

### Request Parameters

```json
{
  "model": "gpt-image-2.5-flare-m",
  "prompt": "An orange tabby cat wearing an astronaut helmet, flat illustration style, pure white background",
  "n": 1,
  "size": "1024x1024",
  "quality": "low",
  "output_format": "png"
}
```

| Parameter | Type | Required | Description |
| --- | --- | :---: | --- |
| `model` | string | Yes | Site image model ID |
| `prompt` | string | Yes | Image description; Chinese and English are supported |
| `n` | integer | No | Number of images to generate, default `1`; the model may impose a lower maximum |
| `size` | string | No | Size; format and allowed values vary by model |
| `quality` | string | No | `low` / `medium` / `high` / `auto`, mainly for GPT Image |
| `response_format` | string | No | `url` or `b64_json`; some models always return `b64_json` |
| `output_format` | string | No | `png` / `jpeg`; `webp` is not always supported |
| `background` | string | No | `transparent` / `opaque` / `auto`, mainly for GPT Image |
| `moderation` | string | No | `low` / `auto`, mainly for GPT Image |
| `output_compression` | integer | No | `0–100`, only effective for lossy formats such as `jpeg` |
| `stream` | boolean | No | Whether to return streaming previews; only some models support it |
| `partial_images` | integer | No | Number of streaming preview images, `0–3`; requires `stream=true` |
| `seed` | integer | No | Random seed; support varies by model |
| `num_inference_steps` | integer | No | Number of sampling steps; supported by some in-house models |
| `watermark` | boolean | No | Whether to add a watermark; support varies by model |

### Size (`size`)

- **GPT Image 2.5 / 2**: accepts only pixel values in the form `WIDTHxHEIGHT` or `auto`; it does **not** accept strings such as `1K`/`2K`/`4K` or `16:9`.
  - Width and height must be multiples of 16; `1920x1080` returns 400, while `1024x1024`, `1536x1024`, and `1024x1536` work.
  - Total pixel count has both upper and lower bounds: `768x768`, `1024x576`, and `4096x4096` are rejected.
  - In testing, `auto` returns `1254x1254`.
  - Convert aspect ratios to pixel dimensions yourself, e.g. use `1536x1024` for 3:2 and `1024x1536` for 2:3.
- **Wan 2.7 Image**: supports 1K/2K/4K tiers and multiple aspect ratios; see the model description in Model Square for the exact syntax.
- **MOSS Image**: supports 512×512 to 2048×2048; defaults to `1024x1024` when omitted.
- **Qwen Image / Gemini Image**: use the size syntax accepted by the upstream; `1024x1024` is common, and other sizes depend on the actual response.

### Response Format

```json
{
  "created": 1792496626,
  "data": [
    {"b64_json": "iVBORw0KGgo..."}
  ]
}
```

Or:

```json
{
  "created": 1792496626,
  "data": [
    {"url": "https://..."}
  ]
}
```

- On the site, GPT Image, Qwen Image, and MOSS Image always return `b64_json`; decode it from Base64 yourself.
- Some models may return token usage in `usage`; treat the response as authoritative.
- Base64 images are large (roughly 200KB of Base64 for a 1MB image); do not write them to regular logs.

## Image Editing

### multipart Upload

```bash
curl -sS https://<base_url>/v1/images/edits \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -F "model=gpt-image-2.5-sunburst-m" \
  -F "prompt=Replace the background with pure white, keeping the subject details" \
  -F "size=1024x1024" \
  -F "quality=low" \
  -F "image=@input.png;type=image/png"
```

With a mask:

```bash
curl -sS https://<base_url>/v1/images/edits \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -F "model=gpt-image-2.5-sunburst-m" \
  -F "prompt=Replace the masked area with a blue circle" \
  -F "size=1024x1024" \
  -F "quality=low" \
  -F "image=@input.png;type=image/png" \
  -F "mask=@mask.png;type=image/png"
```

| Parameter | Type | Required | Description |
| --- | --- | :---: | --- |
| `image` | file | Yes | Image to edit, png/jpeg/webp; repeat this field to pass multiple reference images |
| `mask` | file | No | PNG with an alpha channel; transparent areas are editable, and dimensions must match `image` |
| `prompt` | string | Yes | Editing instruction |
| `n` | integer | No | Number of images to generate |
| `size` / `quality` / `background` / `output_format` / `output_compression` / `moderation` | — | No | Same as text-to-image |

::: warning
The GPT Image upstream does not support `input_fidelity`; passing it returns 400. When you do not need a mask, simply omit `mask` rather than passing an empty file or empty value.
:::

### JSON + `image_url`

GPT Image 2.5 and 2 support passing reference images as JSON; the images must be public URLs the upstream can access directly:

```bash
curl -sS https://<base_url>/v1/images/edits \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2.5-sunburst-m",
    "prompt": "Add a rainbow to the background",
    "images": [
      {"image_url": "https://example.com/input.png"}
    ]
  }'
```

Notes:

- `prompt` must be at the top level.
- Write reference images as `images: [{"image_url": "..."}]`, not `images: ["https://..."]`.
- Do not use a top-level `image_url` or `image` field.
- For images that require authentication, download them first and upload them via multipart.

## Differences by Series

| Series | Differences |
| --- | --- |
| GPT Image 2.5 | Text-to-image + editing; `size` accepts only pixel values; `quality`, `background`, `moderation`, and `output_compression` are available; `output_format=webp` returns 400 on verified models, so png/jpeg are recommended; always returns `b64_json`; supports `stream` + `partial_images` streaming previews. |
| GPT Image 2 | Text-to-image + editing; `size` rules are the same as 2.5; supports JSON `image_url` editing; always returns `b64_json`. |
| GPT Image 1.5 | Text-to-image + editing; parameters mostly follow the OpenAI Images standard; see Model Square for exact support. |
| Gemini Image | Text-to-image + editing; suitable for scenarios that need Gemini's image understanding/generation capabilities; size and aspect ratio syntax follow upstream support. |
| Qwen Image | Text-to-image + editing; the site always uses `output_format=png` and `response_format=b64_json`; `jpeg` from the client may also be overridden to PNG. |
| Wan Image | Text-to-image; supports 1K/2K/4K and multiple ratios, up to 4 images per request; editing is not supported. |
| MOSS Image | Text-to-image; supports 512–2048 resolution and batch generation; the site always uses 30 sampling steps, defaults to `1024x1024`, and returns `b64_json`; editing is not supported. |

## Code Examples

### Python (OpenAI SDK)

```python
import base64
import os

from openai import OpenAI

client = OpenAI(
    api_key=os.environ["CUBEROUTER_API_KEY"],
    base_url="https://<base_url>/v1",
)

resp = client.images.generate(
    model="gpt-image-2.5-flare-m",
    prompt="An orange tabby cat wearing an astronaut helmet, flat illustration style, pure white background",
    size="1024x1024",
    quality="low",
    n=1,
)

with open("out.png", "wb") as f:
    f.write(base64.b64decode(resp.data[0].b64_json))
```

### Image Editing (Python)

```python
with open("input.png", "rb") as image:
    resp = client.images.edit(
        model="gpt-image-2.5-sunburst-m",
        image=image,
        prompt="Replace the background with pure white, keeping the subject details",
        size="1024x1024",
        quality="low",
    )

with open("edited.png", "wb") as f:
    f.write(base64.b64decode(resp.data[0].b64_json))
```

### Text-to-Image (curl + jq Decoding)

```bash
curl -sS https://<base_url>/v1/images/generations \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2.5-flare-m",
    "prompt": "An orange tabby cat wearing an astronaut helmet, flat illustration style, pure white background",
    "size": "1024x1024",
    "quality": "low",
    "n": 1,
    "output_format": "png"
  }' \
  | jq -r '.data[0].b64_json' \
  | base64 -d > out.png
```

## FAQ

### Q1: Getting a 400 `Invalid request parameters`?

Check the following first:

- Whether `size` uses a syntax GPT Image does not accept (`1K`, `16:9`, `1920x1080`).
- Whether `output_format=webp` or `input_fidelity` was passed.
- For JSON editing, whether `prompt` is at the top level and reference images are written as `images: [{"image_url": "..."}]`.
- Whether `mask` is a PNG with the same dimensions as `image`.

The upstream usually returns only a generic message for these errors, so rule them out one by one.

### Q2: Why is there no URL in the response?

GPT Image, Qwen Image, and MOSS Image always return `data[].b64_json`; decode it yourself. Do not rely on `response_format=url`.

### Q3: Request timed out?

The first generation usually takes 10–60 seconds; high quality and large sizes take longer. Set the client timeout to at least 60 seconds. Streaming previews only show intermediate images early; you still have to wait for the completion event.

### Q4: How large can `n` be?

The server enforces an upper bound on `n`, but each model's actual limit is lower. GPT Image recommends 1–10; Wan 2.7 allows up to 4 images per request; exceeding the model limit may return 400 or only partial results.

### Q5: What is the difference between model names with `-m` and `-b`?

They are different site models that may map to different upstreams and available groups. For example, `gpt-image-2.5-flare-m` and `gpt-image-2.5-flare-b` are not the same model and are not interchangeable. See the descriptions in Model Square.
