# 使用 API

本節介紹使用 CubeRouter 的 API 示例與支援的接口端點。

## 獲取 API 地址

使用你的當前域名作為下面示例的 `base_url`，配合金鑰即可開始調用。

## 代碼示例

**OpenAI 兼容格式**：

```bash
curl https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "模型名稱",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

**Responses API**：

```bash
curl https://<base_url>/v1/responses \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "模型名稱",
    "input": "Hello!"
  }'
```

**Claude 原生格式**：

```bash
curl https://<base_url>/v1/messages \
  -H "x-api-key: sk-xxxxxxxx" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model": "claude-3-5-sonnet-20241022", "max_tokens": 1024, "messages": [{"role": "user", "content": "Hello"}]}'
```

**Gemini 原生格式**：

```bash
curl "https://<base_url>/v1beta/models/gemini-1.5-pro:generateContent?key=sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{"contents": [{"parts": [{"text": "Hello"}]}]}'
```

## 支援的接口端點

| 接口 | 路徑 | 說明 |
| --- | --- | --- |
| 聊天補全 | `POST /v1/chat/completions` | 對話生成，支援流式輸出 |
| 文本補全 | `POST /v1/completions` | 傳統補全接口 |
| 向量嵌入 | `POST /v1/embeddings` | 文本向量化 |
| 圖像生成 | `POST /v1/images/generations` | 文生圖 |
| 圖像編輯 | `POST /v1/images/edits` | 圖像編輯 |
| 影片生成 | `POST /v1/video/generations` | 文/圖生影片，異步任務提交 |
| 影片任務查詢 | `GET /v1/video/generations/{task_id}` | 查詢影片任務狀態與結果 |
| 語音轉文字 | `POST /v1/audio/transcriptions` | Whisper 等 |
| 文字轉語音 | `POST /v1/audio/speech` | TTS |
| 重排序 | `POST /v1/rerank` | 文檔重排序 |
| Responses API | `POST /v1/responses` | OpenAI Responses 格式 |
| 實時對話 | `GET /v1/realtime`（WebSocket） | OpenAI Realtime API |
| 模型列表 | `GET /v1/models` | 查詢可用模型 |

## 影片生成示例

影片生成是非同步任務：提交後立即返回 `task_id`，再輪詢查詢端點直到 `status` 變成 `SUCCESS` 或 `FAILURE`，成功後從響應中取得影片網址下載。以下腳本依賴 `curl` 與 `jq`，示範完整流程：

```bash
#!/bin/bash

# ===== 設定 =====
BASE_URL="<base_url>"
TOKEN_KEY="sk-xxxxxxxx"      # 平臺頒發的 API 金鑰
PROMPT="一隻兔子在沙灘上奔跑"
MODEL="模型名稱"             # 替換為模型廣場中可用的影片模型（如 viduq3-pro）
DURATION=5                  # 影片時長（秒），按所選模型支援的規格填寫
SIZE="540p"                 # 解析度，按所選模型支援的規格填寫

# 1. 提交影片生成任務
echo "提交影片生成請求..."
RESP_SUBMIT=$(curl -s -X POST https://$BASE_URL/v1/video/generations \
  -H "Authorization: Bearer $TOKEN_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"model\":\"$MODEL\",\"prompt\":\"$PROMPT\",\"duration\":$DURATION,\"size\":\"$SIZE\"}")

TASK_ID=$(echo "$RESP_SUBMIT" | jq -r '.task_id')
echo "任務 ID: $TASK_ID"

# 2. 輪詢任務狀態
while true; do
  RESP_QUERY=$(curl -s "https://$BASE_URL/v1/video/generations/$TASK_ID" \
    -H "Authorization: Bearer $TOKEN_KEY")

  STATUS=$(echo "$RESP_QUERY" | jq -r '.data.status')
  echo "當前狀態: $STATUS"

  if [[ "$STATUS" == "SUCCESS" ]]; then
    # 優先取上游返回的 creations URL，為空時回退到閘道的 result_url
    VIDEO_URL=$(echo "$RESP_QUERY" | jq -r '.data.data.creations[0].url // empty')
    if [[ -z "$VIDEO_URL" ]]; then
      VIDEO_URL=$(echo "$RESP_QUERY" | jq -r '.data.result_url // empty')
    fi
    echo "生成完成，開始下載: $VIDEO_URL"
    curl -o ./output.mp4 "$VIDEO_URL"
    echo "影片已儲存到 ./output.mp4"
    break
  fi

  if [[ "$STATUS" == "FAILURE" ]]; then
    echo "任務失敗: $STATUS"
    exit 1
  fi

  sleep 10
done
```
