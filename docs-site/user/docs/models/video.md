# 视频生成

> 通过异步任务接口提交视频生成请求，轮询获取状态和视频地址。

视频生成是**异步任务**：提交接口立即返回 `task_id`，随后轮询查询接口，直到任务成功或失败。不同模型支持的生成模式、分辨率、时长和参考素材类型不同。

## 端点

| 用途 | 方法与路径 |
| --- | --- |
| 提交任务（推荐） | `POST /v1/video/generations` |
| 查询任务 | `GET /v1/video/generations/{task_id}` |
| OpenAI 视频兼容-提交 | `POST /v1/videos` |
| OpenAI 视频兼容-查询 | `GET /v1/videos/{task_id}` |
| OpenAI 视频兼容-下载 | `GET /v1/videos/{task_id}/content` |
| Ark 风格-提交 | `POST /v1/videos/generations/tasks` |
| Ark 风格-查询 | `GET /v1/videos/generations/tasks/{task_id}` |

请求头：

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

> 新接入建议统一使用 `/v1/video/generations`。`/v1/videos` 适合已经在用 OpenAI 视频接口的客户端；Ark 风格端点只对部分上游开放。
>
> 模型可用性受密钥分组限制，实际可见型号以模型广场为准。

## 支持的模型

| 系列 | 站内型号 | 能力 |
| --- | --- | --- |
| Seedance | `seedance-2.0-a`、`seedance-2.5-a` | 文生视频、图生视频、首尾帧、参考生视频 |
| Doubao Seedance | `doubao-seedance-2-0-h`、`doubao-seedance-2-5-h`、`doubao-seedance-2-5-260628` | 文生视频、图生视频、参考生视频、原生音频 |
| Wan 3.0 | `wan3.0-video-a`、`wan3.0-video`、`wan3.0-video-prime` | 文生视频、首尾帧、参考图/视频/音频 |
| MiniMax H3 | `minimax-h3-a`、`MiniMax-H3` | 文生视频、首尾帧、多模态参考 |
| Kling | `kling-v3`、`kling-v3-omni` | 文生视频、图生视频、运动控制、多模态参考 |
| Vidu | `viduq3-pro`、`viduq3-turbo`（含 `-fmi` 变体） | 文生视频、图生视频、参考生视频 |

## 提交任务

### 通用参数

```json
{
  "model": "seedance-2.0-a",
  "prompt": "a cat walking in the rain, cinematic",
  "size": "1280x720",
  "duration": 5
}
```

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | :---: | --- |
| `model` | string | 是 | 站内视频模型 ID |
| `prompt` | string | 是* | 视频描述；使用 `content` 直接传文本时可省略 |
| `image` | string | 否 | 单张图片 URL，等价于 `images: [url]` |
| `images` | string[] | 否 | 图片 URL 数组；不同数量的含义见各系列说明 |
| `input_reference` | string | 否 | 单张参考图 URL，按图生视频处理 |
| `content` | array | 否 | Ark 风格内容数组，用于首帧/尾帧/参考素材；只有部分系列支持 |
| `size` | string | 否 | 分辨率或尺寸，如 `1280x720`、`720p`、`480P` |
| `resolution` | string | 否 | 分辨率档位，如 `480p`、`720p`、`1080p`、`768P`、`2K` |
| `ratio` | string | 否 | 画面比例，如 `16:9`、`9:16`、`1:1`、`adaptive` |
| `duration` | integer | 否 | 输出秒数，默认通常为 `5` |
| `seconds` | string | 否 | `duration` 的字符串别名 |
| `metadata` | object | 否 | 模型专属参数，如 `job_type`、`media`、`mode`、`sound` |

\* `prompt` 与 `content` 中的文本二选一；两者都没有会返回参数错误。

### 通用规则

- 图片、视频、音频素材 URL 必须是上游可访问的**公网 HTTP(S) 地址**，不能是 localhost、内网或需要登录的页面。
- 需要鉴权的素材，请先下载到对象存储或 CDN，再传公网 URL。
- 时长、分辨率会同时影响生成结果和费用，建议显式传入，不要依赖默认值。
- 不同模型对 `metadata` 的字段名不同；下面按系列说明。

### 提交响应

```json
{
  "id": "task_J7xRonpligwgybxmcTNY8HJELhFfIIdE",
  "task_id": "task_J7xRonpligwgybxmcTNY8HJELhFfIIdE",
  "status": "queued",
  "model": "seedance-2.0-a",
  "created_at": 1793093555
}
```

保存 `task_id`，用于后续轮询。

## 查询任务

```bash
curl https://<base_url>/v1/video/generations/task_J7xRonpligwgybxmcTNY8HJELhFfIIdE \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY"
```

通用查询响应：

```json
{
  "code": "success",
  "data": {
    "task_id": "task_J7xRonpligwgybxmcTNY8HJELhFfIIdE",
    "status": "SUCCESS",
    "progress": "100%",
    "result_url": "https://cdn.example.com/result.mp4",
    "data": {}
  }
}
```

状态值：

| 状态 | 含义 |
| --- | --- |
| `SUBMITTED` / `QUEUED` | 已提交，等待处理 |
| `IN_PROGRESS` | 生成中 |
| `SUCCESS` | 成功，视频地址在 `result_url` |
| `FAILURE` | 失败，原因在 `fail_reason` 或 `error` 中 |

成功时优先读取 `data.result_url`；部分上游会在 `data.data` 中保留原始返回（例如 `creations`），可以作为兜底。

### OpenAI 视频兼容格式

`/v1/videos` 系列返回 OpenAI 视频对象：

```json
{
  "id": "task_...",
  "object": "video",
  "model": "seedance-2.0-a",
  "status": "completed",
  "progress": 100,
  "created_at": 1793093555,
  "completed_at": 1793093620,
  "seconds": "5",
  "size": "1280x720"
}
```

`status` 取值：`queued`、`in_progress`、`completed`、`failed`。任务成功后，可以通过 `GET /v1/videos/{id}/content` 下载视频字节。

## 各系列参数

### Seedance 2.0 / 2.5（`seedance-2.0-a`、`seedance-2.5-a`）

| 项目 | 说明 |
| --- | --- |
| 模式 | 文生、图生、首尾帧、参考生视频 |
| 分辨率 | `480P`、`720P`、`1080P`；也接受 `832x480`、`1280x720`、`1920x1080` 等 |
| 时长 | `2–30` 秒，默认 `5` |
| 比例 | `16:9`（默认）、`9:16`、`4:3`、`3:4`、`1:1`、`adaptive` |
| 图片 | `image` 或 `images`：1 张 = 图生视频，2 张 + `metadata.job_type=first_last_frame` = 首尾帧 |
| 参考素材 | `metadata.medias`，元素为 `{"type":"...","url":"..."}`；`type` 取 `image`、`video` 或 `audio` |
| 任务类型 | `metadata.job_type`：`text_to_video`、`image_to_video`、`first_last_frame`、`reference_to_video` |
| 数量 | `metadata.n` 只允许 `1` |

注意事项：

- 使用顶层 `size` / `duration`，**不要**用 `metadata.resolution` / `metadata.duration`，否则可能被拒绝。
- 该系列不支持 Ark 风格的 `content` 数组；请用 `prompt` + `images` + `metadata.medias`。
- `metadata.medias` 的 `url` 和 `media_id` 二选一；`url` 必须是公网地址。

示例：首尾帧

```json
{
  "model": "seedance-2.5-a",
  "prompt": "the flower blooms",
  "size": "720P",
  "duration": 5,
  "images": [
    "https://example.com/first.jpg",
    "https://example.com/last.jpg"
  ],
  "metadata": {"job_type": "first_last_frame"}
}
```

### Doubao Seedance（`doubao-seedance-2-0-h`、`doubao-seedance-2-5-h`、`doubao-seedance-2-5-260628`）

| 项目 | 说明 |
| --- | --- |
| 模式 | 文生、图生、参考生视频、原生音频 |
| 分辨率 | `480p`、`720p`、`1080p`；2.0 额外支持 `4k` |
| 时长 | 默认 `5` 秒；建议显式传入 |
| 比例 | `16:9`、`9:16`、`1:1`、`4:3`、`3:4`、`adaptive`（以实际支持为准） |
| 图片 | `image` 或 `images` |
| 精细控制 | `content` 数组，支持首帧/尾帧/参考图/参考视频/参考音频 |
| 音频 | `metadata.generate_audio`；2.5 默认原生音频开启 |

注意事项：

- `prompt` 与 `content` 中的文本二选一。
- 2.5 和 2.0 的分辨率上限不同，4K 只有 2.0 支持。
- `content` 是 Ark 风格数组，和 Seedance（`seedance-*-a`）的用法不同。

示例：图生视频

```json
{
  "model": "doubao-seedance-2-5-h",
  "prompt": "the dog starts running across the grass",
  "image": "https://example.com/dog.jpg",
  "resolution": "720p",
  "ratio": "16:9",
  "duration": 5
}
```

### Wan 3.0（`wan3.0-video-a`、`wan3.0-video`、`wan3.0-video-prime`）

| 项目 | 说明 |
| --- | --- |
| 模式 | 文生、首帧、首尾帧、参考图/视频/音频 |
| 分辨率 | `480p`、`720p`、`1080p` |
| 时长 | `2–30` 秒，默认 `5` |
| 比例 | `21:9`、`16:9`、`4:3`、`1:1`、`3:4`、`9:16`、`adaptive` |
| 图片 | `images`：1 张 = 首帧，2 张 = 首尾帧，3 张及以上 = 参考图 |
| 完整素材控制 | `metadata.media`，支持 `first_frame`、`last_frame`、`reference_image`、`reference_video`、`reference_audio`、`file`、`link` |

素材限制：

- 首帧/尾帧与参考素材**互斥**，不能混用。
- 参考图片 ≤ 10 张，参考视频 ≤ 5 段（合计 ≤ 15 秒），参考音频 ≤ 5 段（合计 ≤ 15 秒）。
- 单次多模态素材合计最多 20 个。
- `file` 和 `link` 各限 1 个，且必须是公开可访问的页面。
- `wan3.0-video-a` 不支持 Ark 风格的 `content` 数组；用 `prompt` + `images` / `metadata.media`。

示例：参考图生视频

```json
{
  "model": "wan3.0-video-a",
  "prompt": "让参考图中的猫在草地上奔跑，镜头缓慢跟随",
  "resolution": "720p",
  "duration": 5,
  "metadata": {
    "media": [
      {"type": "reference_image", "url": "https://example.com/cat.jpg"}
    ]
  }
}
```

### MiniMax H3（`minimax-h3-a`、`MiniMax-H3`）

| 项目 | 说明 |
| --- | --- |
| 模式 | 文生、首帧、首尾帧、多模态参考 |
| 分辨率 | `768P`、`2K` |
| 时长 | `4–15` 秒，默认 `5` |
| 比例 | `16:9`、`4:3`、`1:1`、`3:4`、`9:16`、`21:9` |
| 图片 | `images`：1 张 = 首帧，2 张 = 首尾帧，3 张及以上 = 参考图 |
| 完整素材控制 | `metadata.media`：`first_frame`、`last_frame`、`image_url`、`feature`、`driving_audio` |
| 水印 | `metadata.watermark=true` |

素材限制：

- 首帧/尾帧与参考素材互斥。
- 首帧、尾帧各 1 张；参考图 ≤ 9 张；参考视频 ≤ 3 段；参考音频 ≤ 3 段。
- **文生视频必须显式指定 `ratio`，且不能为 `adaptive`**；不传时默认 `16:9`。
- 参考场景默认 `adaptive`。

示例：文生视频

```json
{
  "model": "minimax-h3-a",
  "prompt": "一只橘猫蹲在窗台上看雨，电影感，缓慢推近",
  "resolution": "768P",
  "ratio": "16:9",
  "duration": 5
}
```

### Kling（`kling-v3`、`kling-v3-omni`）

| 项目 | 说明 |
| --- | --- |
| 能力 | 文生、图生、运动控制、多模态参考（omni） |
| 图片 | `images`：1 张 = 首帧，2 张 = 首尾帧 |
| 模式 | `metadata.mode`：`std`、`pro`、`4k` |
| 声音 | `metadata.sound`：`on`、`off` |
| 类型 | `metadata.kling_v3_type`：`t2v`、`i2v`、`motion_control`；不传时按素材自动推导 |
| 比例 | `metadata.aspect_ratio` |
| 运动控制 | `metadata.img_url`、`metadata.video_url` |

- `kling-v3` 支持文生、图生、首尾帧和运动控制，可生成带声音的视频。
- `kling-v3-omni` 更偏向多参考图/参考视频的主体一致性生成和视频编辑。

### Vidu（`viduq3-pro`、`viduq3-turbo`）

| 项目 | 说明 |
| --- | --- |
| 模式 | 文生、图生、参考生视频 |
| 分辨率 | `360p`、`540p`、`720p`、`1080p`；Q3 默认 `720p` |
| 时长 | 默认 `5` 秒 |
| 图片 | `image`、`images` 或 `input_reference` |
| 说明 | `pro` 偏质量，`turbo` 偏速度；`-fmi` 变体面向特定组织，可用性以分组为准 |

## 轮询示例

```bash
#!/bin/bash
set -e

BASE_URL="https://<base_url>"
API_KEY="$CUBEROUTER_API_KEY"

RESP=$(curl -sS "$BASE_URL/v1/video/generations" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "seedance-2.0-a",
    "prompt": "一只兔子在沙滩上奔跑",
    "size": "1280x720",
    "duration": 5
  }')

TASK_ID=$(echo "$RESP" | jq -r '.task_id')
echo "task_id: $TASK_ID"

while true; do
  QUERY=$(curl -sS "$BASE_URL/v1/video/generations/$TASK_ID" \
    -H "Authorization: Bearer $API_KEY")

  STATUS=$(echo "$QUERY" | jq -r '.data.status')
  echo "status: $STATUS"

  case "$STATUS" in
    SUCCESS)
      URL=$(echo "$QUERY" | jq -r '.data.result_url')
      curl -L -o output.mp4 "$URL"
      echo "saved to output.mp4"
      break
      ;;
    FAILURE)
      echo "$QUERY" | jq -r '.data.fail_reason // .data.error // "task failed"'
      exit 1
      ;;
  esac

  sleep 5
done
```

## 常见问题

### Q1: 提交成功但一直 `IN_PROGRESS`？

视频生成通常需要数十秒到数分钟，复杂模型、长时长、高分辨率会更慢。建议轮询间隔 5–10 秒，客户端总超时至少 10 分钟。长时间无进展时检查任务日志中的失败原因。

### Q2: 素材 URL 传了但上游读不到？

素材必须是上游可访问的公网 HTTP(S) 地址。localhost、内网地址、需要登录的页面、临时签名过期都会失败。建议先把素材放到对象存储或 CDN。

### Q3: 首帧/尾帧和参考素材能一起用吗？

Wan 3.0 和 MiniMax H3 都**不允许**首帧/尾帧与参考素材混用。需要首尾帧时只传首尾帧；需要参考图/视频/音频时走参考模式。

### Q4: 为什么同样的参数，另一个模型报错？

不同系列的 `metadata` 字段名和取值范围不同。例如 Seedance（`seedance-*-a`）用 `metadata.medias` 和 `metadata.job_type`，Doubao Seedance 用 `content`，Wan 用 `metadata.media`。按模型所属系列传参，不要跨系列套用。

### Q5: `/v1/videos` 和 `/v1/video/generations` 有什么区别？

- `/v1/video/generations` 是站内通用任务接口，响应包含 `task_id`、`status`、`result_url` 等字段。
- `/v1/videos` 是 OpenAI 视频兼容接口，返回 OpenAI 视频对象，状态值为 `queued` / `in_progress` / `completed` / `failed`，成功后可通过 `/content` 下载。
- 两者最终生成能力相同，选一种即可。

### Q6: 任务失败后怎么排查？

先看查询响应中的 `fail_reason` 或 `error`，再对照任务日志。常见原因包括素材 URL 不可访问、参数超出模型范围、上游内容审核不通过。修正后重新提交即可。
