# Audio Generation

> Use the OpenAI Audio-compatible API for text-to-speech (TTS) and speech-to-text (ASR).

Audio models on the site all use an OpenAI Audio-style API: text-to-speech uses `POST /v1/audio/speech`, and speech-to-text uses `POST /v1/audio/transcriptions`. Models differ in their voice systems, output formats, and streaming capabilities.

## Endpoints

| Purpose | Method and path | Request format |
| --- | --- | --- |
| Text-to-speech (TTS) | `POST /v1/audio/speech` | `application/json` |
| Speech-to-text (ASR) | `POST /v1/audio/transcriptions` | `multipart/form-data` |

Request headers:

```http
Authorization: Bearer sk-xxxxxxxx
```

## Supported Models

| Type | Site model ID | Description |
| --- | --- | --- |
| TTS | `speech-2.8-hd` | MiniMax Speech 2.8 HD, higher audio quality |
| TTS | `speech-2.8-turbo` | MiniMax Speech 2.8 Turbo, low latency |
| TTS | `moss-tts-1.5-flash-2026-06-26` | MOSS TTS 1.5 Flash, Chinese text-to-speech, non-streaming |
| ASR | `moss-transcribe-diarize-flash-2026-09-02` | MOSS transcription, supports timestamps and speaker diarization, synchronous non-streaming |

## Text-to-Speech (TTS)

### Request Parameters

```json
{
  "model": "speech-2.8-hd",
  "input": "Hello, welcome to CubeRouter.",
  "voice": "male-qn-qingse",
  "response_format": "mp3"
}
```

| Parameter | Type | Required | Description |
| --- | --- | :---: | --- |
| `model` | string | Yes | TTS model ID |
| `input` | string | Yes | Text to synthesize |
| `voice` | string | Yes | Voice ID; MiniMax uses `voice_id` (e.g. `male-qn-qingse`), MOSS uses a MOSS voice ID |
| `response_format` | string | No | Output format, default `mp3`; common values are `mp3`, `wav`, `pcm`, `flac`, `aac`, `opus` |
| `speed` | number | No | Speech rate; support varies by model |
| `stream_format` | string | No | Set to `sse` to return an SSE audio stream (MiniMax) |
| `metadata` | object | No | Vendor extension parameters such as volume, pitch, emotion, and sample rate |

### MiniMax Speech

- Voices use MiniMax `voice_id`, for example `male-qn-qingse`.
- Pass emotion, speed, volume, pitch, sample rate, bitrate, etc. via `metadata`:

```json
{
  "model": "speech-2.8-hd",
  "input": "The weather is nice today.",
  "voice": "male-qn-qingse",
  "response_format": "mp3",
  "metadata": {
    "voice_setting": {
      "speed": 1.0,
      "vol": 1.0,
      "pitch": 0
    },
    "audio_setting": {
      "sample_rate": 32000,
      "bitrate": 128000,
      "format": "mp3"
    }
  }
}
```

- Non-streaming requests return audio bytes, and `Content-Type` corresponds to `response_format`.
- For streaming requests, set `stream_format: "sse"`; the response is SSE:

```text
data: {"type":"speech.audio.delta","audio":"<base64>"}

data: {"type":"speech.audio.done","usage":{...}}
```

The client must concatenate the Base64 audio from `speech.audio.delta` chunks in order.

### MOSS TTS

- Set the voice to a MOSS voice ID; the site does not create or clone voices automatically, so obtain available voices from MOSS first.
- Only **non-streaming** output is supported; `stream_format` has no effect.
- Supported output formats are `mp3`, `wav`, `pcm`, `flac`, `aac`, and `opus`.
- You can insert pauses in the text from `[pause 0.1s]` to `[pause 10.0s]` (subject to MOSS upstream support).

```json
{
  "model": "moss-tts-1.5-flash-2026-06-26",
  "input": "Hello, this is a test voice sample.",
  "voice": "<MOSS voice ID>",
  "response_format": "mp3"
}
```

### Response

A successful non-streaming request returns audio bytes directly:

```http
HTTP/1.1 200 OK
Content-Type: audio/mpeg
```

If the upstream returns an audio URL, the gateway may respond with a 302 redirect. Clients should follow redirects, for example with `curl -L`.

## Speech-to-Text (ASR)

### Request Parameters

```bash
curl -sS https://<base_url>/v1/audio/transcriptions \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -F "file=@meeting.wav" \
  -F "model=moss-transcribe-diarize-flash-2026-09-02" \
  -F "response_format=diarized_json" \
  -F "language=zh"
```

| Parameter | Type | Required | Description |
| --- | --- | :---: | --- |
| `file` | file | Yes | Audio file, WAV recommended; do not set `Content-Type` manually when uploading |
| `model` | string | Yes | ASR model ID |
| `response_format` | string | No | `json`, `diarized_json`, `text`, `srt`, `vtt`; use `diarized_json` when you need speaker segmentation |
| `language` | string | No | Language hint; MOSS does not define a fixed enum, and common languages can use `zh`, `en`, etc. |
| `keyterms` | string | No | A string containing a serialized JSON array, used to hint proper nouns, for example `["CubeRouter"]` |

- MOSS ASR only supports **synchronous non-streaming** transcription; it does not support `async=true` or WebSocket real-time transcription.
- Audio tracks in images or videos must be extracted to an audio file before upload.

### Response

`response_format=json`:

```json
{
  "text": "Today we discussed three topics."
}
```

`response_format=diarized_json`:

```json
{
  "text": "Today we discussed three topics.",
  "segments": [
    {
      "start": 0.0,
      "end": 3.2,
      "text": "Today we discussed three topics.",
      "speaker": "speaker_0"
    }
  ]
}
```

`text`, `srt`, and `vtt` return plain text or subtitle formats, not JSON.

## Code Examples

### TTS (curl)

```bash
curl -L -sS https://<base_url>/v1/audio/speech \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "speech-2.8-hd",
    "input": "Hello, welcome to CubeRouter.",
    "voice": "male-qn-qingse",
    "response_format": "mp3"
  }' \
  -o hello.mp3
```

### TTS (Python, OpenAI SDK)

```python
import os

from openai import OpenAI

client = OpenAI(
    api_key=os.environ["CUBEROUTER_API_KEY"],
    base_url="https://<base_url>/v1",
)

with client.audio.speech.with_streaming_response.create(
    model="speech-2.8-turbo",
    voice="male-qn-qingse",
    input="Hello, welcome to CubeRouter.",
    response_format="mp3",
) as response:
    response.stream_to_file("hello.mp3")
```

### TTS (SSE Streaming, MiniMax)

```bash
curl -N https://<base_url>/v1/audio/speech \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "speech-2.8-turbo",
    "input": "This is an example of streaming text-to-speech.",
    "voice": "male-qn-qingse",
    "response_format": "mp3",
    "stream_format": "sse"
  }'
```

### ASR (Python, OpenAI SDK)

```python
with open("meeting.wav", "rb") as f:
    result = client.audio.transcriptions.create(
        model="moss-transcribe-diarize-flash-2026-09-02",
        file=f,
        response_format="diarized_json",
        language="zh",
    )

print(result.text)
```

## FAQ

### Q1: Getting a 400 saying the voice does not exist?

- MiniMax: `voice` must be a MiniMax `voice_id`, for example `male-qn-qingse`.
- MOSS: `voice` must be a MOSS voice ID; do not use a MiniMax voice name.
- Voice systems are not interchangeable between models; when switching models, switch the voice as well.

### Q2: The response is JSON or a redirect, not audio?

- First confirm that `response_format` is a format the model supports.
- If the upstream returns a URL, the gateway may respond with a 302 redirect; the client needs to follow redirects (`curl -L`).
- Check whether the `Content-Type` response header starts with `audio/`.

### Q3: Can MOSS TTS stream?

No. On the site, MOSS TTS only supports non-streaming output; for streaming, use MiniMax `speech-2.8-hd` / `speech-2.8-turbo`.

### Q4: Long-text synthesis fails?

Different models have per-request text length limits. For long text, synthesize in segments and concatenate the results, or split the text locally by punctuation first to avoid an overly long single request.

### Q5: The transcription has no timestamps or speakers?

Set `response_format` to `diarized_json`. `json` returns only the full text; `text`, `srt`, and `vtt` return the corresponding text/subtitle formats.

### Q6: Does transcription support asynchronous tasks or real-time WebSocket?

MOSS ASR on the site currently only supports synchronous non-streaming transcription; it does not support `async=true` or WebSocket real-time transcription. Split long audio into parts and upload them separately.
