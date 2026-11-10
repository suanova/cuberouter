# CubeRouter Suno API 使用说明

CubeRouter 的 Suno 音乐生成接口，上游为星图（AstraFlow / ModelVerse），对外统一走 CubeRouter。
提供两套入口：

- **通用任务 API**（推荐自家客户端）：`POST /v1/tasks/cuberouter-suno`
- **Suno 风格 API**（兼容 Suno-API 习惯）：`POST /cuberouter/suno/submit/:action`

Base URL 以站点为准，本文示例使用 `https://cuberouter.cn`。所有请求都需要 CubeRouter 令牌：

```http
Authorization: Bearer <CUBEROUTER_TOKEN>
Content-Type: application/json
```

任务为**异步**：提交成功后拿 `task_id`，轮询到 `SUCCESS` 再取结果/产物。站点大约每 60 秒轮询一次上游，刚提交时会短暂显示 `NOT_START` / `QUEUED`，通常 1–2 分钟完成。

## 模型与价格（按次）

| 模型 | 用途 | 价格 |
|---|---|---|
| `suno-v6` / `suno-v6-mini` / `suno-v6-wild` | 音乐生成（v6） | ¥0.432 / 次 |
| `suno-v5.5` / `suno-v5` | 音乐生成（v5） | ¥0.432 / 次 |
| `suno-v4.5-all` / `suno-v4.5+` / `suno-v4.5` / `suno-v4` | 音乐生成（v4） | ¥0.432 / 次 |
| `suno-generate-lyrics` | 歌词生成 | ¥0.0144 / 次 |
| `suno-vocal-separation` | 人声/伴奏分轨 | ¥0.36 / 次 |

- 每次提交计一次费用；生成失败会自动退款（以任务终态为准）。
- 音乐任务一次通常返回 **2 首**歌曲，分轨任务返回 **人声 + 伴奏** 两条音频。
- `suno-boost-music-style` 上游当前不可用，**未开放**（调用会提示无可用渠道）。

---

## 一、通用任务 API

### 提交

`POST /v1/tasks/cuberouter-suno`

请求体：

| 字段 | 必选 | 说明 |
|---|---|---|
| `model` | 是 | 上表中的模型名（大小写敏感） |
| `prompt` | 是 | 灵感模式=描述；自定义模式=歌词；分轨模式可随便填一个占位字符串 |
| `metadata` | 否 | 扩展参数对象，见下 |

`metadata` 常用字段：

| 字段 | 说明 |
|---|---|
| `mode` | `inspiration`（默认）或 `custom` |
| `gpt_description_prompt` | 灵感模式提示词（覆盖 `prompt`） |
| `lyrics` | 自定义模式歌词（覆盖 `prompt`） |
| `tags` | 自定义模式风格标签，**custom 模式必填** |
| `title` | 自定义模式歌名 |
| `make_instrumental` | `true` 生成纯音乐 |
| `task` | `create`（默认）/ `cover` / `extend` / `upload-extend` |
| `audio_url` | 分轨任务的音频 URL |
| `parameters` | 高级上游参数直通（cover 的 `upload_url`、extend 的 `audio_id`/`default_param_flag` 等） |
| `input` | 高级上游 input 直通（upload-extend 的 `audio_url` 等） |

示例：

```bash
# 灵感模式
curl -sS https://cuberouter.cn/v1/tasks/cuberouter-suno \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"model":"suno-v6","prompt":"一首关于乡愁的简短民谣"}'

# 自定义歌词/风格
curl -sS https://cuberouter.cn/v1/tasks/cuberouter-suno \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{
    "model":"suno-v6",
    "prompt":"[Verse] 夕阳落在旧屋檐\n[Chorus] 归乡的路还在",
    "metadata":{"custom_mode":true,"tags":"pop, ballad","title":"归乡"}
  }'

# 纯音乐
curl -sS https://cuberouter.cn/v1/tasks/cuberouter-suno \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"model":"suno-v6-mini","prompt":"lofi rain","metadata":{"make_instrumental":true}}'

# 歌词
curl -sS https://cuberouter.cn/v1/tasks/cuberouter-suno \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"model":"suno-generate-lyrics","prompt":"写一首关于海的短歌词"}'

# 分轨（prompt 只用于通过网关校验，不会影响分轨）
curl -sS https://cuberouter.cn/v1/tasks/cuberouter-suno \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{
    "model":"suno-vocal-separation",
    "prompt":"vocal separation",
    "metadata":{"audio_url":"https://example.com/song.mp3"}
  }'
```

提交响应：

```json
{"created_at":1794300595,"id":"task_xxx","model":"suno-v6","status":"queued","task_id":"task_xxx"}
```

### 查询任务

`GET /v1/tasks/{task_id}`

```json
{
  "task_id": "task_xxx",
  "platform": "cuberouter-suno",
  "status": "SUCCESS",
  "progress": "100%",
  "fail_reason": "",
  "created_at": 1794300595,
  "finished_at": 1794300661
}
```

状态：`NOT_START` → `QUEUED` → `IN_PROGRESS` → `SUCCESS` / `FAILURE`。失败时看 `fail_reason`。

### 取产物（音乐/分轨）

`GET /v1/tasks/{task_id}/artifacts`

```json
{
  "task_id": "task_xxx",
  "artifacts": [
    {
      "key": "audio-...",
      "type": "audio",
      "mime_type": "audio/mpeg",
      "content_url": "https://cuberouter.cn/v1/tasks/task_xxx/artifacts/audio-.../content?access=..."
    }
  ]
}
```

`content_url` 可以直接 GET/HEAD 下载（自带访问令牌，不要把自己的 Bearer 头带上也没关系）。音乐任务通常是 2 条，分轨任务通常是 Instrumental + Vocals 2 条。

### 取歌词正文

通用任务查询不返回 `data`；歌词在扩展查询里：

`GET /cuberouter-suno/tasks/{task_id}`

```json
{
  "task_id": "task_xxx",
  "status": "SUCCESS",
  "data": {
    "output": {
      "data": [
        {"title": "月下回声", "text": "[Verse 1]\n..."},
        {"title": "海潮替我说", "text": "[Verse 1]\n..."}
      ]
    }
  }
}
```

---

## 二、Suno 风格 API

路径不带 `/v1`，方便已有 Suno-API 习惯的客户端接入。

### 提交

`POST /cuberouter/suno/submit/:action`

`:action` 取值：

| action | 映射模型 |
|---|---|
| `music` | 音乐模型，默认 `suno-v6`；可用 `model` 指定上表 9 个音乐模型之一 |
| `lyrics` | `suno-generate-lyrics` |
| `separation` | `suno-vocal-separation` |
| `style` | `suno-boost-music-style`（上游不可用，会返回无可用渠道） |

请求体（music 为例）：

| 字段 | 说明 |
|---|---|
| `model` | 可选，音乐模型名；不填默认 `suno-v6` |
| `prompt` | 歌词（自定义模式）或描述（灵感模式） |
| `gpt_description_prompt` | 灵感模式提示词 |
| `tags` / `title` | 自定义模式风格/歌名（有 `tags` 或 `title` 会自动按自定义模式处理） |
| `custom_mode` | 显式指定是否自定义模式 |
| `make_instrumental` | 是否纯音乐 |
| `audio_url` | 分轨任务的音频地址（separation） |
| `metadata` / `parameters` / `input` | 高级参数直通，同通用 API |

示例：

```bash
# 音乐（灵感模式）
curl -sS https://cuberouter.cn/cuberouter/suno/submit/music \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"prompt":"一首关于乡愁的简短民谣"}'

# 音乐（自定义模式）
curl -sS https://cuberouter.cn/cuberouter/suno/submit/music \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"prompt":"[Verse] 归乡","tags":"pop, ballad","title":"归乡"}'

# 歌词
curl -sS https://cuberouter.cn/cuberouter/suno/submit/lyrics \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"prompt":"写一首关于海的短歌词"}'

# 分轨
curl -sS https://cuberouter.cn/cuberouter/suno/submit/separation \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"audio_url":"https://example.com/song.mp3"}'
```

提交响应（Suno 风格）：

```json
{"code":"success","message":"","data":"task_xxx"}
```

### 查询

`GET /cuberouter/suno/fetch/{task_id}`

```json
{
  "code": "success",
  "message": "",
  "data": {
    "task_id": "task_xxx",
    "platform": "cuberouter-suno",
    "status": "SUCCESS",
    "progress": "100%",
    "fail_reason": "",
    "submit_time": 1794300595,
    "finish_time": 1794300661,
    "data": {
      "output": {
        "urls": ["https://.../song.mp3", "https://.../song-2.mp3"]
      }
    }
  }
}
```

- 音乐任务：`data.data.output.urls` 是音频地址（外层 `data` 是任务视图，内层 `data` 是上游原始响应；临时 CDN，建议尽快转存）。
- 歌词任务：`data.data.output.data[].text` 是歌词正文。
- 分轨任务：`data.data.output.urls` 是 Instrumental / Vocals 两条音频。

### 错误响应

native 路由的请求/解码错误返回 HTTP 4xx 和统一信封：

```json
{"code":"invalid_request","message":"unsupported action: bogus","data":null}
```

渠道选择或上游提交失败时返回对应 HTTP 状态码（如 503），body 可能是：

```json
{"code":"server_error","message":"Task request failed","data":null}
```

详细原因看响应里的 request id 或站点日志。`style` action 因上游模型不可用会返回 503。

---

## 常见问题

**提交后一直 `NOT_START`？**
站点约每 60 秒轮询一轮上游，等 1–2 分钟再查；超过 5 分钟仍无变化可看任务 `fail_reason`。

**返回 `model_not_found` / 无可用渠道？**
当前令牌所在分组没有该模型，或模型名拼错。音乐模型先确认是上表 9 个之一。

**自定义模式报 `metadata.tags is required for custom mode`？**
自定义模式必须带 `tags`；如果只想写描述，请去掉 `tags`/`title` 并用灵感模式。

**分轨报 `audio_url is required`？**
分轨走 `metadata.audio_url`（通用 API）或顶层 `audio_url`（Suno 风格 API），必须是 http(s) 可访问的音频地址。

**音频地址能用多久？**
上游返回的是临时地址；拿任务后请通过 `/artifacts` 的 `content_url` 转存，或在有效期内尽快下载。

**cover / extend / upload-extend 怎么传？**
用 `metadata.task` 指定任务类型，并通过 `metadata.parameters` / `metadata.input` 透传上游字段（cover 需要 `parameters.upload_url`，extend 需要 `parameters.audio_id` + `parameters.default_param_flag`，upload-extend 需要 `input.audio_url`）。这些高级字段以星图上游约定为准，普通建歌不需要。
