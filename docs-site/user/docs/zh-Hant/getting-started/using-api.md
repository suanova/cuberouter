# 使用 API

> 快速瞭解 API 地址、認證方式和可用端點。參數細節、模型差異和完整示例見 [模型 API 參考](/zh-Hant/models/)。

## 獲取 API 地址

使用當前站點域名作爲 `base_url`，配合「API 金鑰」頁面創建的金鑰即可調用。

```text
https://<base_url>
```

## 認證

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

不同協議也支援各自的原生認證頭：

| 協議 | 認證方式 |
| --- | --- |
| OpenAI | `Authorization: Bearer sk-...` |
| Anthropic | `x-api-key: sk-...` 或 `Authorization: Bearer sk-...` |
| Gemini | `x-goog-api-key: sk-...` 或 `?key=sk-...` |

## 快速示例

**OpenAI Chat Completions**：

```bash
curl https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "模型名稱",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

**OpenAI Responses**：

```bash
curl https://<base_url>/v1/responses \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "模型名稱",
    "input": "Hello!"
  }'
```

**Anthropic Messages**：

```bash
curl https://<base_url>/v1/messages \
  -H "x-api-key: sk-xxxxxxxx" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "模型名稱",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

**Gemini generateContent**：

```bash
curl "https://<base_url>/v1beta/models/模型名稱:generateContent" \
  -H "x-goog-api-key: sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}'
```

## 支援的接口端點

| 接口 | 路徑 | 詳細說明 |
| --- | --- | --- |
| 聊天補全 | `POST /v1/chat/completions` | [OpenAI 協議](/zh-Hant/models/llm/openai) |
| 文本補全 | `POST /v1/completions` | [OpenAI 協議](/zh-Hant/models/llm/openai) |
| Responses | `POST /v1/responses` | [OpenAI 協議](/zh-Hant/models/llm/openai#responses-api) |
| 實時對話 | `GET /v1/realtime`（WebSocket） | — |
| Claude 原生格式 | `POST /v1/messages` | [Anthropic 協議](/zh-Hant/models/llm/anthropic) |
| Gemini 原生格式 | `POST /v1beta/models/{model}:generateContent` | [Gemini 協議](/zh-Hant/models/llm/gemini) |
| 圖像生成 | `POST /v1/images/generations` | [圖像生成](/zh-Hant/models/image) |
| 圖像編輯 | `POST /v1/images/edits` | [圖像生成](/zh-Hant/models/image) |
| 影片生成 | `POST /v1/video/generations` | [影片生成](/zh-Hant/models/video) |
| 影片任務查詢 | `GET /v1/video/generations/{task_id}` | [影片生成](/zh-Hant/models/video) |
| 語音合成 | `POST /v1/audio/speech` | [音頻生成](/zh-Hant/models/audio) |
| 語音轉寫 | `POST /v1/audio/transcriptions` | [音頻生成](/zh-Hant/models/audio) |
| 向量嵌入 | `POST /v1/embeddings` | [模型 API 參考](/zh-Hant/models/) |
| 重排序 | `POST /v1/rerank` | [模型 API 參考](/zh-Hant/models/) |
| 模型列表 | `GET /v1/models` | [模型 API 參考](/zh-Hant/models/) |

## 下一步

- 查模型支援和參數差異：[模型 API 參考](/zh-Hant/models/)
- 查公共字段和錯誤處理：[通用參數](/zh-Hant/models/common-parameters)
- 在 Claude Code 中使用：[Claude Code](/zh-Hant/getting-started/claude-code)
- 在 Codex 中使用：[Codex](/zh-Hant/getting-started/codex)
