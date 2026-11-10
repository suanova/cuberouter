# 模型 API 参考

> 按协议和模型类型整理的调用参考：先选模型和协议，再查参数、示例与限制。

CubeRouter 把不同厂商的模型统一到同一个 Base URL 和 API 密钥下。你可以继续使用现有的 OpenAI、Anthropic 或 Gemini SDK，只需要替换 Base URL 和模型名。

::: tip 本文的地址写法
示例中的 `https://<base_url>` 指你当前访问的 CubeRouter 站点域名，例如 `https://cuberouter.cn`。API 路径统一以 `/v1` 或 `/v1beta` 开头。
:::

## 快速开始

### 1. 获取 API 密钥

登录站点后，在「API 密钥」页面创建一个密钥。密钥所属的**分组**决定了你能调用哪些模型；如果调用返回模型不存在或无权限，先检查密钥分组。

请求时把密钥放在请求头：

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

Anthropic 协议也接受 `x-api-key`，Gemini 协议也接受 `x-goog-api-key` 或 `?key=`，见各协议页面。

### 2. 选择模型

在「模型广场」中按端点类型、供应商或标签筛选，复制完整的模型 ID。调用时 `model` 必须使用站内模型 ID，而不是上游厂商的原始模型名。

同一个模型可能有多个带后缀的型号，例如 `-a`、`-b`、`-m`、`-h`、`-j`。它们是**不同的站内模型**，可能由不同上游提供，参数支持、可用分组也不同，不能互相替代。例如 `gpt-image-2.5-flare` 和 `gpt-image-2.5-flare-m` 是两个模型。

### 3. 第一个请求

```bash
curl https://<base_url>/v1/chat/completions \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-6-luna-m",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "你好，介绍一下你自己"}
    ]
  }'
```

把 `gpt-6-luna-m` 换成你在模型广场复制的模型 ID 即可。

## 协议总览

| 模型类型 | 协议 | 端点 | 参考 |
| --- | --- | --- | --- |
| 大语言模型 | OpenAI Chat Completions | `POST /v1/chat/completions` | [OpenAI 协议](/models/llm/openai) |
| 大语言模型 | OpenAI Responses | `POST /v1/responses` | [OpenAI 协议](/models/llm/openai#responses-api) |
| 大语言模型 | Anthropic Messages | `POST /v1/messages` | [Anthropic 协议](/models/llm/anthropic) |
| 大语言模型 | Gemini generateContent | `POST /v1beta/models/{model}:generateContent` | [Gemini 协议](/models/llm/gemini) |
| 图像生成 | OpenAI Images | `POST /v1/images/generations`、`POST /v1/images/edits` | [图像生成](/models/image) |
| 视频生成 | 异步任务 | `POST /v1/video/generations`、`GET /v1/video/generations/{task_id}` | [视频生成](/models/video) |
| 音频生成 | OpenAI Audio | `POST /v1/audio/speech`、`POST /v1/audio/transcriptions` | [音频生成](/models/audio) |
| 向量 / 重排序 | OpenAI Embeddings / Jina Rerank | `POST /v1/embeddings`、`POST /v1/rerank` | 本文档暂不展开 |

选择建议：

- **优先使用模型的原生协议**。Claude 用 Anthropic 协议、Gemini 用 Gemini 协议时，厂商专有字段支持最完整；用其它协议调用时，网关会做格式转换，个别参数可能被忽略或返回 400。
- **已有代码不想改**：继续用 OpenAI Chat Completions；大多数文本模型都支持这一协议。
- **Codex CLI**：使用 OpenAI Responses 协议。
- **Claude Code**：使用 Anthropic 协议，配置方法见 [Claude Code](/getting-started/claude-code)。

## 模型系列与协议支持

下表按系列汇总。具体到每个型号的支持情况，见各协议页面的模型表。

### 按场景选择（参考）

下表中的 `*` 表示同系列有多个后缀型号，实际调用时使用模型广场中的完整 ID。

| 场景 | 可优先考虑 | 说明 |
| --- | --- | --- |
| 高吞吐、低延迟 | `gpt-5.6-luna-*`、`gemini-3.5/3.7/3.8-flash-b`、`claude-haiku-5-5-*`、`deepseek-v4-flash-*`、`minimax-m3.1-flash-a` | 轻量/快速档，适合分类、摘要、实时交互 |
| 复杂推理、Agent | `claude-opus-5-5-*`、`gpt-6-astra-*`、`gemini-3.1-pro-preview*`、`deepseek-v4-pro-a`、`qwen3.8-max-a` | 旗舰档，适合长链路任务和高难度推理 |
| 编程、长程任务 | `claude-opus-5-5-*`、`gpt-6-sol-*`、`kimi-k2.7-code-*`、`glm-5.3-*`、`qwen3.8-max-a` | 按厂商定位整理，具体以模型广场说明为准 |
| 长上下文 | 带 `1M` 标签的型号，如 Claude 5.5、GPT 5.6/6、Gemini 3.1、DeepSeek V4、Kimi K3、Qwen 3.x、GLM 5.2/5.3 | 适合长文档、代码库分析 |
| 多模态输入 | `gemini-3.x`、`doubao-seed-2-1-*`、`deepseek-v4.1-flash-*`、`kimi-k3-*`、`qwen3.x` | 支持图片或视频输入 |

> 以上只是按模型定位给出的参考，不构成性能排序。实际选择还应结合可用分组、上下文长度、工具调用需求和实测效果。

### 大语言模型

| 系列 | 代表型号 | Chat | Responses | Anthropic | Gemini |
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

### 图像生成

| 系列 | 站内型号 | 文生图 | 图像编辑 |
| --- | --- | :---: | :---: |
| GPT Image | `gpt-image-2.5-flare-m`、`gpt-image-2.5-sunburst-m`、`gpt-image-2-m`、`gpt-image-1.5-m` 等 | ✓ | ✓ |
| Gemini Image | `gemini-3.1-flash-image-m`、`gemini-3-pro-image-m` | ✓ | ✓ |
| Qwen Image | `qwen-image-2.1-turbo` | ✓ | ✓ |
| Wan Image | `wan2.7-image-a` | ✓ | — |
| MOSS Image | `moss-image` | ✓ | — |

### 视频生成

| 系列 | 站内型号 | 能力 |
| --- | --- | --- |
| Seedance | `seedance-2.0-a`、`seedance-2.5-a` | 文生视频、图生视频、首尾帧、参考生视频 |
| Doubao Seedance | `doubao-seedance-2-0-h`、`doubao-seedance-2-5-h`、`doubao-seedance-2-5-260628` | 文生视频、图生视频、参考生视频、原生音频 |
| Wan 3.0 | `wan3.0-video-a`、`wan3.0-video`、`wan3.0-video-prime` | 文生视频、首尾帧、参考图/视频/音频 |
| MiniMax H3 | `minimax-h3-a`、`MiniMax-H3` | 文生视频、首尾帧、多模态参考 |
| Kling | `kling-v3`、`kling-v3-omni` | 文生视频、图生视频、运动控制、多模态参考 |
| Vidu | `viduq3-pro`、`viduq3-turbo`（含 `-fmi` 变体） | 文生视频、图生视频、参考生视频 |

### 音频生成

| 类型 | 站内型号 | 说明 |
| --- | --- | --- |
| 语音合成（TTS） | `speech-2.8-hd`、`speech-2.8-turbo`、`moss-tts-1.5-flash-2026-06-26` | 文本转语音 |
| 语音转写（ASR） | `moss-transcribe-diarize-flash-2026-09-02` | 转写、时间戳、说话人分离 |

## 通用约定

### 请求格式

- 请求体统一使用 UTF-8 JSON；图像编辑和语音转写使用 `multipart/form-data`。
- 所有端点都使用同一套 API 密钥，不需要为不同协议单独创建密钥。
- 模型名区分大小写，必须与模型广场中的完整 ID 一致。

### 流式响应

文本和语音合成支持流式返回，使用 SSE（`text/event-stream`）。不同协议的事件格式不同，见各协议页面。图像生成只有部分模型支持流式预览。

### 同步与异步

- 文本、图像、音频是同步请求：连接建立后等待结果返回。
- 视频生成是异步任务：提交后立即返回 `task_id`，再轮询查询接口获取状态和结果。

### 错误格式

不同协议的错误响应结构不同，但都包含可读的错误信息：

```jsonc
// OpenAI 协议
{"error": {"message": "...", "type": "...", "code": "..."}}

// Anthropic 协议
{"type": "error", "error": {"type": "...", "message": "..."}}

// Gemini 协议
{"error": {"code": 400, "message": "...", "status": "INVALID_ARGUMENT"}}

// 视频任务
{"code": "...", "message": "...", "data": null}
```

常见状态码：

| 状态码 | 含义 | 处理建议 |
| --- | --- | --- |
| 400 | 请求参数错误 | 检查模型名、必填参数和取值范围 |
| 401 | 密钥无效或缺失 | 检查 `Authorization` / `x-api-key` / `x-goog-api-key` |
| 403 | 无权限 | 检查密钥分组是否包含该模型 |
| 404 | 模型不存在或端点不支持 | 确认模型 ID，并确认该模型支持当前协议 |
| 429 | 限流或上游饱和 | 降低并发，稍后重试 |
| 500 / 502 / 503 | 上游或网关异常 | 稍后重试；持续失败请联系支持 |

### 模型可用性与超时

- 模型是否可用由**密钥所属分组**决定，模型广场会按分组展示可访问的模型。
- 图像生成通常需要 10–60 秒；视频生成通常需要数十秒到数分钟。客户端超时建议不低于 60 秒，视频轮询间隔建议 5–10 秒。
- 本文档只说明模型调用方式，不涉及价格与计费。价格以模型广场和实际账单为准。

## 下一步

- [通用参数](/models/common-parameters)：跨协议的公共字段、多模态输入和错误处理。
- [OpenAI 协议](/models/llm/openai)：Chat Completions 与 Responses。
- [Anthropic 协议](/models/llm/anthropic)：Messages API 与 Claude Code。
- [Gemini 协议](/models/llm/gemini)：generateContent 与原生多模态输入。
- [图像生成](/models/image)、[视频生成](/models/video)、[音频生成](/models/audio)：按模态查参数和示例。
