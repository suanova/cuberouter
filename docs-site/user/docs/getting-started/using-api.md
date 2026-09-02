# 使用 API

本节介绍使用 CubeRouter 的 API 示例与支持的接口端点。

## 获取 API 地址

使用你当前域名作为下面示例的 `base_url`，配合密钥即可开始调用。

## 代码示例

**OpenAI Chat Comletions**：

```bash
curl https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "模型名称",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

**Responses API**：

```bash
curl https://<base_url>/v1/responses \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "模型名称",
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

## 支持的接口端点

| 接口 | 路径 | 说明 |
| --- | --- | --- |
| 聊天补全 | `POST /v1/chat/completions` | 对话生成，支持流式输出 |
| 文本补全 | `POST /v1/completions` | 传统补全接口 |
| 向量嵌入 | `POST /v1/embeddings` | 文本向量化 |
| 图像生成 | `POST /v1/images/generations` | 文生图 |
| 图像编辑 | `POST /v1/images/edits` | 图像编辑 |
| 视频生成 | `POST /v1/video/generations` | 文/图生视频，异步任务提交 |
| 视频任务查询 | `GET /v1/video/generations/{task_id}` | 查询视频任务状态与结果 |
| 语音转文字 | `POST /v1/audio/transcriptions` | Whisper 等 |
| 文字转语音 | `POST /v1/audio/speech` | TTS |
| 重排序 | `POST /v1/rerank` | 文档重排序 |
| Responses API | `POST /v1/responses` | OpenAI Responses 格式 |
| 实时对话 | `GET /v1/realtime`（WebSocket） | OpenAI Realtime API |
| 模型列表 | `GET /v1/models` | 查询可用模型 |

## 视频生成示例

视频生成为异步任务：提交后立即返回 `task_id`，再轮询查询端点直到 `status` 变为 `SUCCESS` 或 `FAILURE`，成功后从响应中取视频地址下载。以下脚本依赖 `curl` 与 `jq`，演示完整流程：

```bash
#!/bin/bash

# ===== 设置 =====
BASE_URL="<base_url>"
TOKEN_KEY="sk-你的API密钥"   # 平台颁发的 API 密钥
PROMPT="一只兔子在沙滩上奔跑"
MODEL="模型名称"             # 替换为模型广场中可用的视频模型（如 viduq3-pro）
DURATION=5                  # 视频时长（秒），按所选模型支持的规格填写
SIZE="540p"                 # 分辨率，按所选模型支持的规格填写

# 1. 提交生成视频任务
echo "提交视频生成请求..."
RESP_SUBMIT=$(curl -s -X POST https://$BASE_URL/v1/video/generations \
  -H "Authorization: Bearer $TOKEN_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"model\":\"$MODEL\",\"prompt\":\"$PROMPT\",\"duration\":$DURATION,\"size\":\"$SIZE\"}")

TASK_ID=$(echo "$RESP_SUBMIT" | jq -r '.task_id')
echo "任务 ID: $TASK_ID"

# 2. 轮询任务状态
while true; do
  RESP_QUERY=$(curl -s "https://$BASE_URL/v1/video/generations/$TASK_ID" \
    -H "Authorization: Bearer $TOKEN_KEY")

  STATUS=$(echo "$RESP_QUERY" | jq -r '.data.status')
  echo "当前状态: $STATUS"

  if [[ "$STATUS" == "SUCCESS" ]]; then
    # 优先取上游返回的 creations URL，为空时回退到网关的 result_url
    VIDEO_URL=$(echo "$RESP_QUERY" | jq -r '.data.data.creations[0].url // empty')
    if [[ -z "$VIDEO_URL" ]]; then
      VIDEO_URL=$(echo "$RESP_QUERY" | jq -r '.data.result_url // empty')
    fi
    echo "生成完成，开始下载: $VIDEO_URL"
    curl -o ./output.mp4 "$VIDEO_URL"
    echo "视频已保存到 ./output.mp4"
    break
  fi

  if [[ "$STATUS" == "FAILURE" ]]; then
    echo "任务失败: $STATUS"
    exit 1
  fi

  sleep 10
done
```
