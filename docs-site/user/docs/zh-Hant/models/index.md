# 模型 API 參考

> 按協議和模型類型整理的調用參考：先選模型和協議，再查參數、示例與限制。

CubeRouter 把不同廠商的模型統一到同一個 Base URL 和 API 金鑰下。你可以繼續使用現有的 OpenAI、Anthropic 或 Gemini SDK，只需要替換 Base URL 和模型名。

::: tip 本文的地址寫法
示例中的 `https://<base_url>` 指你當前訪問的 CubeRouter 站點域名，例如 `https://cuberouter.cn`。API 路徑統一以 `/v1` 或 `/v1beta` 開頭。
:::

## 快速開始

### 1. 獲取 API 金鑰

登入站點後，在「API 金鑰」頁面創建一個金鑰。金鑰所屬的**分組**決定了你能調用哪些模型；如果調用返回模型不存在或無權限，先檢查金鑰分組。

請求時把金鑰放在請求頭：

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

Anthropic 協議也接受 `x-api-key`，Gemini 協議也接受 `x-goog-api-key` 或 `?key=`，見各協議頁面。

### 2. 選擇模型

在「模型廣場」中按端點類型、供應商或標籤篩選，複製完整的模型 ID。調用時 `model` 必須使用站內模型 ID，而不是上游廠商的原始模型名。

同一個模型可能有多個帶後綴的型號，例如 `-a`、`-b`、`-m`、`-h`、`-j`。它們是**不同的站內模型**，可能由不同上游提供，參數支援、可用分組也不同，不能互相替代。例如 `gpt-image-2.5-flare` 和 `gpt-image-2.5-flare-m` 是兩個模型。

### 3. 第一個請求

```bash
curl https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-6-luna-m",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "你好，介紹一下你自己"}
    ]
  }'
```

把 `gpt-6-luna-m` 換成你在模型廣場複製的模型 ID 即可。

## 協議總覽

| 模型類型 | 協議 | 端點 | 參考 |
| --- | --- | --- | --- |
| 大語言模型 | OpenAI Chat Completions | `POST /v1/chat/completions` | [OpenAI 協議](/zh-Hant/models/llm/openai) |
| 大語言模型 | OpenAI Responses | `POST /v1/responses` | [OpenAI 協議](/zh-Hant/models/llm/openai#responses-api) |
| 大語言模型 | Anthropic Messages | `POST /v1/messages` | [Anthropic 協議](/zh-Hant/models/llm/anthropic) |
| 大語言模型 | Gemini generateContent | `POST /v1beta/models/{model}:generateContent` | [Gemini 協議](/zh-Hant/models/llm/gemini) |
| 圖像生成 | OpenAI Images | `POST /v1/images/generations`、`POST /v1/images/edits` | [圖像生成](/zh-Hant/models/image) |
| 影片生成 | 異步任務 | `POST /v1/video/generations`、`GET /v1/video/generations/{task_id}` | [影片生成](/zh-Hant/models/video) |
| 音頻生成 | OpenAI Audio | `POST /v1/audio/speech`、`POST /v1/audio/transcriptions` | [音頻生成](/zh-Hant/models/audio) |
| 向量 / 重排序 | OpenAI Embeddings / Jina Rerank | `POST /v1/embeddings`、`POST /v1/rerank` | 本文檔暫不展開 |

選擇建議：

- **優先使用模型的原生協議**。Claude 用 Anthropic 協議、Gemini 用 Gemini 協議時，廠商專有字段支援最完整；用其它協議調用時，網關會做格式轉換，個別參數可能被忽略或返回 400。
- **已有代碼不想改**：繼續用 OpenAI Chat Completions；大多數文本模型都支援這一協議。
- **Codex CLI**：使用 OpenAI Responses 協議。
- **Claude Code**：使用 Anthropic 協議，配置方法見 [Claude Code](/zh-Hant/getting-started/claude-code)。

## 模型系列與協議支援

下表按系列彙總。具體到每個型號的支援情況，見各協議頁面的模型表。

### 按場景選擇（參考）

下表中的 `*` 表示同系列有多個後綴型號，實際調用時使用模型廣場中的完整 ID。

| 場景 | 可優先考慮 | 說明 |
| --- | --- | --- |
| 高吞吐、低延遲 | `gpt-5.6-luna-*`、`gemini-3.5/3.7/3.8-flash-b`、`claude-haiku-5-5-*`、`deepseek-v4-flash-*`、`minimax-m3.1-flash-a` | 輕量/快速檔，適合分類、摘要、實時交互 |
| 複雜推理、Agent | `claude-opus-5-5-*`、`gpt-6-astra-*`、`gemini-3.1-pro-preview*`、`deepseek-v4-pro-a`、`qwen3.8-max-a` | 旗艦檔，適合長鏈路任務和高難度推理 |
| 編程、長程任務 | `claude-opus-5-5-*`、`gpt-6-sol-*`、`kimi-k2.7-code-*`、`glm-5.3-*`、`qwen3.8-max-a` | 按廠商定位整理，具體以模型廣場說明爲準 |
| 長上下文 | 帶 `1M` 標籤的型號，如 Claude 5.5、GPT 5.6/6、Gemini 3.1、DeepSeek V4、Kimi K3、Qwen 3.x、GLM 5.2/5.3 | 適合長文檔、代碼庫分析 |
| 多模態輸入 | `gemini-3.x`、`doubao-seed-2-1-*`、`deepseek-v4.1-flash-*`、`kimi-k3-*`、`qwen3.x` | 支援圖片或影片輸入 |

> 以上只是按模型定位給出的參考，不構成性能排序。實際選擇還應結合可用分組、上下文長度、工具調用需求和實測效果。

### 大語言模型

| 系列 | 代表型號 | Chat | Responses | Anthropic | Gemini |
| --- | --- | :---: | :---: | :---: | :---: |
| Claude | `claude-opus-5-5-j`、`claude-sonnet-5-5-j`、`claude-haiku-5-5-j` 等 | ✓ | 部分 | ✓ | — |
| GPT | `gpt-6-astra-m`、`gpt-6-sol-m`、`gpt-5.6-terra-m` 等 | ✓ | ✓ | 部分 | — |
| Gemini | `gemini-3.8-flash-b`、`gemini-3.1-pro-preview` 等 | ✓ | 部分 | 部分 | ✓ |
| DeepSeek | `deepseek-v4-pro-a`、`deepseek-v4.1-flash-a` 等 | ✓ | ✓ | ✓ | — |
| Kimi | `kimi-k3-a`、`kimi-k3-j`、`kimi-k2.7-code-a` 等 | ✓ | 部分 | 部分 | — |
| Qwen | `qwen3.8-max-a`、`qwen3.7-plus-a` 等 | ✓ | ✓ | ✓ | — |
| GLM | `glm-5.3-z`、`glm-5.2-a` 等 | ✓ | 部分 | 部分 | — |
| Doubao | `doubao-seed-2-1-pro-h`、`doubao-seed-2-1-turbo-h` | ✓ | ✓ | ✓ | — |
| MiniMax | `minimax-m3.1-flash-a`、`minimax-m2.7-a` | ✓ | — | — | — |

### 圖像生成

| 系列 | 站內型號 | 文生圖 | 圖像編輯 |
| --- | --- | :---: | :---: |
| GPT Image | `gpt-image-2.5-flare-m`、`gpt-image-2.5-sunburst-m`、`gpt-image-2-m`、`gpt-image-1.5-m` 等 | ✓ | ✓ |
| Gemini Image | `gemini-3.1-flash-image-m`、`gemini-3-pro-image-m` | ✓ | ✓ |
| Qwen Image | `qwen-image-2.1-turbo` | ✓ | ✓ |
| Wan Image | `wan2.7-image-a` | ✓ | — |
| MOSS Image | `moss-image` | ✓ | — |

### 影片生成

| 系列 | 站內型號 | 能力 |
| --- | --- | --- |
| Seedance | `seedance-2.0-a`、`seedance-2.5-a` | 文生影片、圖生影片、首尾幀、參考生影片 |
| Doubao Seedance | `doubao-seedance-2-0-h`、`doubao-seedance-2-5-h`、`doubao-seedance-2-5-260628` | 文生影片、圖生影片、參考生影片、原生音頻 |
| Wan 3.0 | `wan3.0-video-a`、`wan3.0-video`、`wan3.0-video-prime` | 文生影片、首尾幀、參考圖/影片/音頻 |
| MiniMax H3 | `minimax-h3-a`、`MiniMax-H3` | 文生影片、首尾幀、多模態參考 |
| Kling | `kling-v3`、`kling-v3-omni` | 文生影片、圖生影片、運動控制、多模態參考 |
| Vidu | `viduq3-pro`、`viduq3-turbo`（含 `-fmi` 變體） | 文生影片、圖生影片、參考生影片 |

### 音頻生成

| 類型 | 站內型號 | 說明 |
| --- | --- | --- |
| 語音合成（TTS） | `speech-2.8-hd`、`speech-2.8-turbo`、`moss-tts-1.5-flash-2026-06-26` | 文本轉語音 |
| 語音轉寫（ASR） | `moss-transcribe-diarize-flash-2026-09-02` | 轉寫、時間戳、說話人分離 |

## 通用約定

### 請求格式

- 請求體統一使用 UTF-8 JSON；圖像編輯和語音轉寫使用 `multipart/form-data`。
- 所有端點都使用同一套 API 金鑰，不需要爲不同協議單獨創建金鑰。
- 模型名區分大小寫，必須與模型廣場中的完整 ID 一致。

### 流式響應

文本和語音合成支援流式返回，使用 SSE（`text/event-stream`）。不同協議的事件格式不同，見各協議頁面。圖像生成只有部分模型支援流式預覽。

### 同步與異步

- 文本、圖像、音頻是同步請求：連接建立後等待結果返回。
- 影片生成是異步任務：提交後立即返回 `task_id`，再輪詢查詢接口獲取狀態和結果。

### 錯誤格式

不同協議的錯誤響應結構不同，但都包含可讀的錯誤資訊：

```jsonc
// OpenAI 協議
{"error": {"message": "...", "type": "...", "code": "..."}}

// Anthropic 協議
{"type": "error", "error": {"type": "...", "message": "..."}}

// Gemini 協議
{"error": {"code": 400, "message": "...", "status": "INVALID_ARGUMENT"}}

// 影片任務
{"code": "...", "message": "...", "data": null}
```

常見狀態碼：

| 狀態碼 | 含義 | 處理建議 |
| --- | --- | --- |
| 400 | 請求參數錯誤 | 檢查模型名、必填參數和取值範圍 |
| 401 | 金鑰無效或缺失 | 檢查 `Authorization` / `x-api-key` / `x-goog-api-key` |
| 403 | 無權限 | 檢查金鑰分組是否包含該模型 |
| 404 | 模型不存在或端點不支援 | 確認模型 ID，並確認該模型支援當前協議 |
| 429 | 限流或上游飽和 | 降低併發，稍後重試 |
| 500 / 502 / 503 | 上游或網關異常 | 稍後重試；持續失敗請聯繫支援 |

### 模型可用性與超時

- 模型是否可用由**金鑰所屬分組**決定，模型廣場會按分組展示可訪問的模型。
- 圖像生成通常需要 10–60 秒；影片生成通常需要數十秒到數分鐘。客戶端超時建議不低於 60 秒，影片輪詢間隔建議 5–10 秒。
- 本文檔只說明模型調用方式，不涉及價格與計費。價格以模型廣場和實際賬單爲準。

## 下一步

- [通用參數](/zh-Hant/models/common-parameters)：跨協議的公共字段、多模態輸入和錯誤處理。
- [OpenAI 協議](/zh-Hant/models/llm/openai)：Chat Completions 與 Responses。
- [Anthropic 協議](/zh-Hant/models/llm/anthropic)：Messages API 與 Claude Code。
- [Gemini 協議](/zh-Hant/models/llm/gemini)：generateContent 與原生多模態輸入。
- [圖像生成](/zh-Hant/models/image)、[影片生成](/zh-Hant/models/video)、[音頻生成](/zh-Hant/models/audio)：按模態查參數和示例。
