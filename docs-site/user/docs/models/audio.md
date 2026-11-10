# 音频生成

> 使用 OpenAI Audio 兼容接口进行语音合成（TTS）和语音转写（ASR）。

站内音频模型统一走 OpenAI Audio 风格接口：语音合成用 `POST /v1/audio/speech`，语音转写用 `POST /v1/audio/transcriptions`。不同模型的音色体系、输出格式和流式能力不同。

## 端点

| 用途 | 方法与路径 | 请求格式 |
| --- | --- | --- |
| 语音合成（TTS） | `POST /v1/audio/speech` | `application/json` |
| 语音转写（ASR） | `POST /v1/audio/transcriptions` | `multipart/form-data` |

请求头：

```http
Authorization: Bearer sk-xxxxxxxx
```

## 支持的模型

| 类型 | 站内型号 | 说明 |
| --- | --- | --- |
| TTS | `speech-2.8-hd` | MiniMax Speech 2.8 HD，音质更高 |
| TTS | `speech-2.8-turbo` | MiniMax Speech 2.8 Turbo，低延迟 |
| TTS | `moss-tts-1.5-flash-2026-06-26` | MOSS TTS 1.5 Flash，中文语音合成，非流式 |
| ASR | `moss-transcribe-diarize-flash-2026-09-02` | MOSS 转写，支持时间戳和说话人分离，同步非流式 |

## 语音合成（TTS）

### 请求参数

```json
{
  "model": "speech-2.8-hd",
  "input": "你好，欢迎使用 CubeRouter。",
  "voice": "male-qn-qingse",
  "response_format": "mp3"
}
```

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | :---: | --- |
| `model` | string | 是 | TTS 模型 ID |
| `input` | string | 是 | 待合成文本 |
| `voice` | string | 是 | 音色 ID；MiniMax 用 `voice_id`（如 `male-qn-qingse`），MOSS 用 MOSS 音色 ID |
| `response_format` | string | 否 | 输出格式，默认 `mp3`；常见 `mp3`、`wav`、`pcm`、`flac`、`aac`、`opus` |
| `speed` | number | 否 | 语速，支持情况因模型而异 |
| `stream_format` | string | 否 | 设为 `sse` 时返回 SSE 音频流（MiniMax） |
| `metadata` | object | 否 | 厂商扩展参数，如音量、音高、情绪、采样率等 |

### MiniMax Speech

- 音色使用 MiniMax `voice_id`，例如 `male-qn-qingse`。
- 情绪、语速、音量、音高、采样率、比特率等通过 `metadata` 传入：

```json
{
  "model": "speech-2.8-hd",
  "input": "今天天气不错。",
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

- 非流式请求返回音频二进制，`Content-Type` 与 `response_format` 对应。
- 流式请求设置 `stream_format: "sse"`，响应为 SSE：

```text
data: {"type":"speech.audio.delta","audio":"<base64>"}

data: {"type":"speech.audio.done","usage":{...}}
```

客户端需要把 `speech.audio.delta` 的 Base64 音频按顺序拼接。

### MOSS TTS

- 音色填 MOSS 音色 ID；站点不会自动创建或克隆音色，需要先通过 MOSS 侧获取可用音色。
- 仅支持**非流式**输出；`stream_format` 无效。
- 输出格式支持 `mp3`、`wav`、`pcm`、`flac`、`aac`、`opus`。
- 文本里可以用 `[pause 0.1s]` 到 `[pause 10.0s]` 插入停顿（以 MOSS 上游支持为准）。

```json
{
  "model": "moss-tts-1.5-flash-2026-06-26",
  "input": "你好，这是一段测试语音。",
  "voice": "<MOSS 音色 ID>",
  "response_format": "mp3"
}
```

### 返回

非流式成功时直接返回音频字节：

```http
HTTP/1.1 200 OK
Content-Type: audio/mpeg
```

如果上游返回的是音频 URL，网关可能返回 302 重定向。客户端应跟随重定向，例如 `curl -L`。

## 语音转写（ASR）

### 请求参数

```bash
curl -sS https://<base_url>/v1/audio/transcriptions \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -F "file=@meeting.wav" \
  -F "model=moss-transcribe-diarize-flash-2026-09-02" \
  -F "response_format=diarized_json" \
  -F "language=zh"
```

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | :---: | --- |
| `file` | file | 是 | 音频文件，推荐 WAV；上传时不要手动设置 `Content-Type` |
| `model` | string | 是 | ASR 模型 ID |
| `response_format` | string | 否 | `json`、`diarized_json`、`text`、`srt`、`vtt`；需要说话人分段时用 `diarized_json` |
| `language` | string | 否 | 语言提示；MOSS 不约定固定枚举，常见语言可用 `zh`、`en` 等 |
| `keyterms` | string | 否 | JSON 字符串数组序列化后的字符串，用于提示专有名词，例如 `["CubeRouter"]` |

- MOSS ASR 只支持**同步非流式**转写，不支持 `async=true` 和 WebSocket 实时转写。
- 图片、视频中的音轨需要先提取为音频文件再上传。

### 返回

`response_format=json`：

```json
{
  "text": "今天讨论了三个议题。"
}
```

`response_format=diarized_json`：

```json
{
  "text": "今天讨论了三个议题。",
  "segments": [
    {
      "start": 0.0,
      "end": 3.2,
      "text": "今天讨论了三个议题。",
      "speaker": "speaker_0"
    }
  ]
}
```

`text`、`srt`、`vtt` 返回纯文本或字幕格式，不是 JSON。

## 代码示例

### TTS（curl）

```bash
curl -L -sS https://<base_url>/v1/audio/speech \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "speech-2.8-hd",
    "input": "你好，欢迎使用 CubeRouter。",
    "voice": "male-qn-qingse",
    "response_format": "mp3"
  }' \
  -o hello.mp3
```

### TTS（Python，OpenAI SDK）

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
    input="你好，欢迎使用 CubeRouter。",
    response_format="mp3",
) as response:
    response.stream_to_file("hello.mp3")
```

### TTS（SSE 流式，MiniMax）

```bash
curl -N https://<base_url>/v1/audio/speech \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "speech-2.8-turbo",
    "input": "这是一段流式语音合成的示例。",
    "voice": "male-qn-qingse",
    "response_format": "mp3",
    "stream_format": "sse"
  }'
```

### ASR（Python，OpenAI SDK）

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

## 常见问题

### Q1: 返回 400，提示音色不存在？

- MiniMax：`voice` 必须是 MiniMax `voice_id`，例如 `male-qn-qingse`。
- MOSS：`voice` 必须是 MOSS 音色 ID，不能填 MiniMax 音色名。
- 不同模型的音色体系不通用，换模型时要同时换音色。

### Q2: 返回的是 JSON 或重定向，不是音频？

- 先确认 `response_format` 是模型支持的格式。
- 如果上游返回 URL，网关可能 302 重定向，客户端需要跟随重定向（`curl -L`）。
- 检查响应头 `Content-Type` 是否以 `audio/` 开头。

### Q3: MOSS TTS 能流式吗？

不能。MOSS TTS 站内只支持非流式输出；需要流式请用 MiniMax `speech-2.8-hd` / `speech-2.8-turbo`。

### Q4: 长文本合成失败？

不同模型有单次文本长度限制。长文本建议分段合成后再拼接，或先在本地按标点切分，避免单次请求过长。

### Q5: 转写结果没有时间戳或说话人？

`response_format` 需要设为 `diarized_json`。`json` 只返回整体文本；`text`、`srt`、`vtt` 返回对应文本/字幕格式。

### Q6: 转写支持异步任务或实时 WebSocket 吗？

站内 MOSS ASR 目前只支持同步非流式转写，不支持 `async=true` 和 WebSocket 实时转写。长音频请拆分后分别上传。
