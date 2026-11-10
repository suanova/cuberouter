# Gemini 协议

> 使用 Gemini 原生 `generateContent` 格式调用 Gemini 系列模型。

Gemini 协议适合需要原生多模态输入、`safetySettings`、`thinkingConfig` 或 Google SDK 兼容性的应用。站内 Gemini 系列大多同时支持 OpenAI 协议；需要 Gemini 专有字段时使用本协议。

## 端点

| 用途 | 方法与路径 |
| --- | --- |
| 生成内容 | `POST /v1beta/models/{model}:generateContent` |
| 流式生成 | `POST /v1beta/models/{model}:streamGenerateContent?alt=sse` |
| 模型列表 | `GET /v1beta/models` |

认证方式（三选一）：

```http
x-goog-api-key: sk-xxxxxxxx
```

```http
Authorization: Bearer sk-xxxxxxxx
```

```text
POST /v1beta/models/gemini-3.8-flash-b:generateContent?key=sk-xxxxxxxx
```

## 支持的模型

| 模型 | 说明 |
| --- | --- |
| `gemini-3.1-flash-lite-m` | 轻量经济档，低延迟高吞吐 |
| `gemini-3.1-pro-preview` | Pro 预览版，强推理与多模态，1M 上下文 |
| `gemini-3.1-pro-preview-m` | Pro 预览版（`-m` 变体） |
| `gemini-3.5-flash-b` | Flash 轻量旗舰，低延迟多模态 |
| `gemini-3.7-flash-b` | Flash 轻量旗舰 |
| `gemini-3.8-flash` | Flash 轻量旗舰 |
| `gemini-3.8-flash-b` | Flash 轻量旗舰（`-b` 变体） |

> `gemini-3.1-flash-lite-m` 和 `gemini-3.1-pro-preview-m` 只支持 OpenAI 与 Gemini 两种协议；其余型号还支持 Responses / Anthropic，具体见 [OpenAI 协议](/models/llm/openai) 与 [Anthropic 协议](/models/llm/anthropic)。
> 模型可用性受密钥分组限制，实际可见型号以模型广场为准。

## 请求

```json
{
  "contents": [{
    "role": "user",
    "parts": [{"text": "你好，介绍一下你自己"}]
  }],
  "systemInstruction": {
    "parts": [{"text": "你是一个简洁的中文助手。"}]
  },
  "generationConfig": {
    "temperature": 0.7,
    "maxOutputTokens": 1024
  }
}
```

### 顶层字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `contents` | array | 对话内容，元素包含 `role` 和 `parts` |
| `systemInstruction` | object | 系统指令，也接受 snake_case 写法 `system_instruction` |
| `generationConfig` | object | 生成参数 |
| `safetySettings` | array | 安全过滤设置 |
| `tools` | array | 工具定义 |
| `toolConfig` | object | 工具调用配置 |
| `cachedContent` | string | 缓存内容名称，部分模型支持 |

`contents[].role` 常用 `user` 和 `model`。多轮对话时，把模型上一轮回复作为 `role: "model"` 的内容放回 `contents`。

### `parts`

| 字段 | 说明 |
| --- | --- |
| `text` | 文本 |
| `inlineData` | 内联二进制，包含 `mimeType` 和 `data`（base64） |
| `fileData` | 文件引用，包含 `mimeType` 和 `fileUri` |
| `functionCall` | 模型发起的函数调用 |
| `functionResponse` | 函数执行结果 |
| `thought` | 是否为思考内容 |
| `thoughtSignature` | 思考签名，多轮/工具调用时应原样带回 |
| `executableCode` / `codeExecutionResult` | 代码执行工具的结果 |
| `videoMetadata` / `mediaResolution` | 视频元数据和媒体分辨率 |

图片输入示例：

```json
{
  "contents": [{
    "role": "user",
    "parts": [
      {"text": "描述这张图片"},
      {"inlineData": {"mimeType": "image/jpeg", "data": "<base64>"}}
    ]
  }]
}
```

大文件可以用 `fileData`：

```json
{"fileData": {"mimeType": "image/jpeg", "fileUri": "https://generativelanguage.googleapis.com/v1beta/files/..."}}
```

`fileUri` 填 Gemini File API 返回的文件 URI，或上游明确支持的公网文件地址。`fileData` 的字段名使用 camelCase（`mimeType`、`fileUri`）。

### `generationConfig`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `temperature` | number | 采样温度 |
| `topP` | number | 核采样 |
| `topK` | number | Top-K |
| `maxOutputTokens` | integer | 输出上限 |
| `candidateCount` | integer | 候选数量 |
| `stopSequences` | string[] | 停止词 |
| `responseMimeType` | string | 例如 `application/json` |
| `responseSchema` | object | JSON Schema |
| `presencePenalty` | number | 存在惩罚 |
| `frequencyPenalty` | number | 频率惩罚 |
| `seed` | integer | 随机种子 |
| `responseModalities` | string[] | 输出模态，如 `["TEXT"]`；支持图像输出的模型可加 `"IMAGE"` |
| `thinkingConfig` | object | 思考配置 |
| `mediaResolution` | string | 媒体分辨率 |
| `speechConfig` / `imageConfig` | object | 语音/图像专用配置 |

### `thinkingConfig`

```json
{
  "generationConfig": {
    "thinkingConfig": {
      "thinkingBudget": 2048,
      "includeThoughts": true
    }
  }
}
```

| 字段 | 说明 |
| --- | --- |
| `thinkingBudget` | 思考 token 预算；不同模型允许范围不同 |
| `thinkingLevel` | 思考档位，部分 Gemini 3 模型使用；与 `thinkingBudget` 的可用性以模型为准 |
| `includeThoughts` | 是否在响应中返回思考内容 |

### 安全设置

```json
{
  "safetySettings": [
    {"category": "HARM_CATEGORY_HARASSMENT", "threshold": "BLOCK_MEDIUM_AND_ABOVE"},
    {"category": "HARM_CATEGORY_HATE_SPEECH", "threshold": "BLOCK_MEDIUM_AND_ABOVE"}
  ]
}
```

类别和阈值以模型支持为准。被安全策略拦截时，响应可能包含 `promptFeedback.blockReason` 或 `finishReason: "SAFETY"`。

### 工具调用

```json
{
  "tools": [{
    "functionDeclarations": [{
      "name": "get_weather",
      "description": "查询城市天气",
      "parameters": {
        "type": "object",
        "properties": {"city": {"type": "string"}},
        "required": ["city"]
      }
    }]
  }],
  "toolConfig": {
    "functionCallingConfig": {"mode": "AUTO"}
  }
}
```

`mode` 可取 `AUTO`、`ANY`、`NONE`；`ANY` 时可用 `allowedFunctionNames` 限定可调用的函数。

## 响应

```json
{
  "candidates": [{
    "content": {
      "role": "model",
      "parts": [{"text": "你好！有什么可以帮你的？"}]
    },
    "finishReason": "STOP"
  }],
  "usageMetadata": {
    "promptTokenCount": 20,
    "candidatesTokenCount": 10,
    "totalTokenCount": 30
  }
}
```

- `candidates[0].content.parts` 是输出内容，可能包含文本、函数调用或思考内容。
- `finishReason` 常见值：`STOP`、`MAX_TOKENS`、`SAFETY`、`RECITATION`。
- `usageMetadata` 可能包含 `cachedContentTokenCount`、`thoughtsTokenCount` 等字段。
- 请求被拦截时可能没有 `candidates`，需要检查 `promptFeedback`。

## 流式

```bash
curl -N "https://<base_url>/v1beta/models/gemini-3.8-flash-b:streamGenerateContent?alt=sse" \
  -H "x-goog-api-key: $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [{"role": "user", "parts": [{"text": "写一段产品介绍"}]}]
  }'
```

每个 `data:` 是一个 `GenerateContentResponse`：

```text
data: {"candidates":[{"content":{"role":"model","parts":[{"text":"你"}]}}]}

data: {"candidates":[{"content":{"role":"model","parts":[{"text":"好"}]}}],"usageMetadata":{...}}
```

客户端需要拼接 `parts[].text`；最后一个 chunk 通常包含完整的 `usageMetadata`。

## 各系列差异

| 模型 | 差异与建议 |
| --- | --- |
| `gemini-3.8-flash-b` / `gemini-3.8-flash` | 同时支持 OpenAI、Responses、Anthropic、Gemini 四种协议；原生 Gemini 字段最完整。 |
| `gemini-3.7-flash-b` / `gemini-3.5-flash-b` | 四种协议均支持；适合高并发、低延迟场景。 |
| `gemini-3.1-pro-preview` | 四种协议均支持；适合复杂推理和长文档。 |
| `gemini-3.1-pro-preview-m` | 只支持 OpenAI 和 Gemini；需要 `safetySettings` 等原生字段时用 Gemini 协议。 |
| `gemini-3.1-flash-lite-m` | 只支持 OpenAI 和 Gemini；经济档，适合高吞吐任务。 |

## 常见问题

### Q1: 返回 404 或 `model not found`？

确认模型 ID 与模型广场一致。Gemini 协议只支持上表中的型号；其他 Gemini 图像模型（如 `gemini-3.1-flash-image-m`）通过 [图像生成](/models/image) 接口调用，不走 `generateContent`。

### Q2: 流式没有返回？

`streamGenerateContent` 需要 `alt=sse`；不带该参数时返回的是 JSON 数组流，不是 SSE。

### Q3: `inlineData` 和 `fileData` 怎么选？

- 小文件或 base64 数据用 `inlineData`。
- 已上传的文件用 `fileData`；`fileUri` 填 Gemini File API 返回的文件 URI，或上游明确支持的公网文件地址，字段名必须是 `mimeType` / `fileUri`。
- 不要用 OpenAI 的 `image_url` 格式调用 Gemini 协议。

### Q4: 返回 `finishReason: "SAFETY"` 没有内容？

请求被安全策略拦截。可以调整 `safetySettings`（如果模型允许），或修改提示词后重试。

### Q5: `thinkingBudget` 和 `thinkingLevel` 能同时用吗？

不要同时传。不同 Gemini 3 型号支持的字段不同，按模型说明选择其一；不确定时先用 `thinkingBudget` 或留空走默认。
