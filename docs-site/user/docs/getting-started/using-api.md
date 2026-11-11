# 使用 API

> 快速了解 API 地址、认证方式和可用端点。参数细节、模型差异和完整示例见 [模型 API 参考](/models/)。

## 获取 API 地址

使用当前站点域名作为 `base_url`，配合「API 密钥」页面创建的密钥即可调用。

```text
https://<base_url>
```

## 认证

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

不同协议也支持各自的原生认证头：

| 协议 | 认证方式 |
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
    "model": "模型名称",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

**OpenAI Responses**：

```bash
curl https://<base_url>/v1/responses \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "模型名称",
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
    "model": "模型名称",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

**Gemini generateContent**：

```bash
curl "https://<base_url>/v1beta/models/模型名称:generateContent" \
  -H "x-goog-api-key: sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}'
```

## 支持的接口端点

| 接口 | 路径 | 详细说明 |
| --- | --- | --- |
| 聊天补全 | `POST /v1/chat/completions` | [OpenAI 协议](/models/llm/openai) |
| 文本补全 | `POST /v1/completions` | [OpenAI 协议](/models/llm/openai) |
| Responses | `POST /v1/responses` | [OpenAI 协议](/models/llm/openai#responses-api) |
| 实时对话 | `GET /v1/realtime`（WebSocket） | — |
| Claude 原生格式 | `POST /v1/messages` | [Anthropic 协议](/models/llm/anthropic) |
| Gemini 原生格式 | `POST /v1beta/models/{model}:generateContent` | [Gemini 协议](/models/llm/gemini) |
| 图像生成 | `POST /v1/images/generations` | [图像生成](/models/image) |
| 图像编辑 | `POST /v1/images/edits` | [图像生成](/models/image) |
| 视频生成 | `POST /v1/video/generations` | [视频生成](/models/video) |
| 视频任务查询 | `GET /v1/video/generations/{task_id}` | [视频生成](/models/video) |
| 语音合成 | `POST /v1/audio/speech` | [音频生成](/models/audio) |
| 语音转写 | `POST /v1/audio/transcriptions` | [音频生成](/models/audio) |
| 向量嵌入 | `POST /v1/embeddings` | [模型 API 参考](/models/) |
| 重排序 | `POST /v1/rerank` | [模型 API 参考](/models/) |
| 模型列表 | `GET /v1/models` | [模型 API 参考](/models/) |

## 下一步

- 查模型支持和参数差异：[模型 API 参考](/models/)
- 查公共字段和错误处理：[通用参数](/models/common-parameters)
- 在 Claude Code 中使用：[Claude Code](/getting-started/claude-code)
- 在 Codex 中使用：[Codex](/getting-started/codex)
