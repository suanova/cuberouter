# 音頻生成

> 使用 OpenAI Audio 兼容接口進行語音合成（TTS）和語音轉寫（ASR）。

站內音頻模型統一走 OpenAI Audio 風格接口：語音合成用 `POST /v1/audio/speech`，語音轉寫用 `POST /v1/audio/transcriptions`。不同模型的音色體系、輸出格式和流式能力不同。

## 端點

| 用途 | 方法與路徑 | 請求格式 |
| --- | --- | --- |
| 語音合成（TTS） | `POST /v1/audio/speech` | `application/json` |
| 語音轉寫（ASR） | `POST /v1/audio/transcriptions` | `multipart/form-data` |

請求頭：

```http
Authorization: Bearer sk-xxxxxxxx
```

## 支援的模型

| 類型 | 站內型號 | 說明 |
| --- | --- | --- |
| TTS | `speech-2.8-hd` | MiniMax Speech 2.8 HD，音質更高 |
| TTS | `speech-2.8-turbo` | MiniMax Speech 2.8 Turbo，低延遲 |
| TTS | `moss-tts-1.5-flash-2026-06-26` | MOSS TTS 1.5 Flash，中文語音合成，非流式 |
| ASR | `moss-transcribe-diarize-flash-2026-09-02` | MOSS 轉寫，支援時間戳和說話人分離，同步非流式 |

## 語音合成（TTS）

### 請求參數

```json
{
  "model": "speech-2.8-hd",
  "input": "你好，歡迎使用 CubeRouter。",
  "voice": "male-qn-qingse",
  "response_format": "mp3"
}
```

| 參數 | 類型 | 必填 | 說明 |
| --- | --- | :---: | --- |
| `model` | string | 是 | TTS 模型 ID |
| `input` | string | 是 | 待合成文本 |
| `voice` | string | 是 | 音色 ID；MiniMax 用 `voice_id`（如 `male-qn-qingse`），MOSS 用 MOSS 音色 ID |
| `response_format` | string | 否 | 輸出格式，默認 `mp3`；常見 `mp3`、`wav`、`pcm`、`flac`、`aac`、`opus` |
| `speed` | number | 否 | 語速，支援情況因模型而異 |
| `stream_format` | string | 否 | 設爲 `sse` 時返回 SSE 音頻流（MiniMax） |
| `metadata` | object | 否 | 廠商擴展參數，如音量、音高、情緒、採樣率等 |

### MiniMax Speech

- 音色使用 MiniMax `voice_id`，例如 `male-qn-qingse`。
- 情緒、語速、音量、音高、採樣率、比特率等通過 `metadata` 傳入：

```json
{
  "model": "speech-2.8-hd",
  "input": "今天天氣不錯。",
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

- 非流式請求返回音頻二進制，`Content-Type` 與 `response_format` 對應。
- 流式請求設置 `stream_format: "sse"`，響應爲 SSE：

```text
data: {"type":"speech.audio.delta","audio":"<base64>"}

data: {"type":"speech.audio.done","usage":{...}}
```

客戶端需要把 `speech.audio.delta` 的 Base64 音頻按順序拼接。

### MOSS TTS

- 音色填 MOSS 音色 ID；站點不會自動創建或克隆音色，需要先通過 MOSS 側獲取可用音色。
- 僅支援**非流式**輸出；`stream_format` 無效。
- 輸出格式支援 `mp3`、`wav`、`pcm`、`flac`、`aac`、`opus`。
- 文本里可以用 `[pause 0.1s]` 到 `[pause 10.0s]` 插入停頓（以 MOSS 上游支援爲準）。

```json
{
  "model": "moss-tts-1.5-flash-2026-06-26",
  "input": "你好，這是一段測試語音。",
  "voice": "<MOSS 音色 ID>",
  "response_format": "mp3"
}
```

### 返回

非流式成功時直接返回音頻字節：

```http
HTTP/1.1 200 OK
Content-Type: audio/mpeg
```

如果上游返回的是音頻 URL，網關可能返回 302 重定向。客戶端應跟隨重定向，例如 `curl -L`。

## 語音轉寫（ASR）

### 請求參數

```bash
curl -sS https://<base_url>/v1/audio/transcriptions \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -F "file=@meeting.wav" \
  -F "model=moss-transcribe-diarize-flash-2026-09-02" \
  -F "response_format=diarized_json" \
  -F "language=zh"
```

| 參數 | 類型 | 必填 | 說明 |
| --- | --- | :---: | --- |
| `file` | file | 是 | 音頻文件，推薦 WAV；上傳時不要手動設置 `Content-Type` |
| `model` | string | 是 | ASR 模型 ID |
| `response_format` | string | 否 | `json`、`diarized_json`、`text`、`srt`、`vtt`；需要說話人分段時用 `diarized_json` |
| `language` | string | 否 | 語言提示；MOSS 不約定固定枚舉，常見語言可用 `zh`、`en` 等 |
| `keyterms` | string | 否 | JSON 字符串數組序列化後的字符串，用於提示專有名詞，例如 `["CubeRouter"]` |

- MOSS ASR 只支援**同步非流式**轉寫，不支援 `async=true` 和 WebSocket 實時轉寫。
- 圖片、影片中的音軌需要先提取爲音頻文件再上傳。

### 返回

`response_format=json`：

```json
{
  "text": "今天討論了三個議題。"
}
```

`response_format=diarized_json`：

```json
{
  "text": "今天討論了三個議題。",
  "segments": [
    {
      "start": 0.0,
      "end": 3.2,
      "text": "今天討論了三個議題。",
      "speaker": "speaker_0"
    }
  ]
}
```

`text`、`srt`、`vtt` 返回純文本或字幕格式，不是 JSON。

## 代碼示例

### TTS（curl）

```bash
curl -L -sS https://<base_url>/v1/audio/speech \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "speech-2.8-hd",
    "input": "你好，歡迎使用 CubeRouter。",
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
    input="你好，歡迎使用 CubeRouter。",
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
    "input": "這是一段流式語音合成的示例。",
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

## 常見問題

### Q1: 返回 400，提示音色不存在？

- MiniMax：`voice` 必須是 MiniMax `voice_id`，例如 `male-qn-qingse`。
- MOSS：`voice` 必須是 MOSS 音色 ID，不能填 MiniMax 音色名。
- 不同模型的音色體系不通用，換模型時要同時換音色。

### Q2: 返回的是 JSON 或重定向，不是音頻？

- 先確認 `response_format` 是模型支援的格式。
- 如果上游返回 URL，網關可能 302 重定向，客戶端需要跟隨重定向（`curl -L`）。
- 檢查響應頭 `Content-Type` 是否以 `audio/` 開頭。

### Q3: MOSS TTS 能流式嗎？

不能。MOSS TTS 站內只支援非流式輸出；需要流式請用 MiniMax `speech-2.8-hd` / `speech-2.8-turbo`。

### Q4: 長文本合成失敗？

不同模型有單次文本長度限制。長文本建議分段合成後再拼接，或先在本地按標點切分，避免單次請求過長。

### Q5: 轉寫結果沒有時間戳或說話人？

`response_format` 需要設爲 `diarized_json`。`json` 只返回整體文本；`text`、`srt`、`vtt` 返回對應文本/字幕格式。

### Q6: 轉寫支援異步任務或實時 WebSocket 嗎？

站內 MOSS ASR 目前只支援同步非流式轉寫，不支援 `async=true` 和 WebSocket 實時轉寫。長音頻請拆分後分別上傳。
