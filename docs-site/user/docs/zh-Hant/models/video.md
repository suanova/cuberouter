# 影片生成

> 通過異步任務接口提交影片生成請求，輪詢獲取狀態和影片地址。

影片生成是**異步任務**：提交接口立即返回 `task_id`，隨後輪詢查詢接口，直到任務成功或失敗。不同模型支援的生成模式、分辨率、時長和參考素材類型不同。

## 端點

| 用途 | 方法與路徑 |
| --- | --- |
| 提交任務（推薦） | `POST /v1/video/generations` |
| 查詢任務 | `GET /v1/video/generations/{task_id}` |
| OpenAI 影片兼容-提交 | `POST /v1/videos` |
| OpenAI 影片兼容-查詢 | `GET /v1/videos/{task_id}` |
| OpenAI 影片兼容-下載 | `GET /v1/videos/{task_id}/content` |
| Ark 風格-提交 | `POST /v1/videos/generations/tasks` |
| Ark 風格-查詢 | `GET /v1/videos/generations/tasks/{task_id}` |

請求頭：

```http
Authorization: Bearer sk-xxxxxxxx
Content-Type: application/json
```

> 新接入建議統一使用 `/v1/video/generations`。`/v1/videos` 適合已經在用 OpenAI 影片接口的客戶端；Ark 風格端點只對部分上游開放。
>
> 模型可用性受金鑰分組限制，實際可見型號以模型廣場爲準。

## 支援的模型

| 系列 | 站內型號 | 能力 |
| --- | --- | --- |
| Seedance | `seedance-2.0-a`、`seedance-2.5-a` | 文生影片、圖生影片、首尾幀、參考生影片 |
| Doubao Seedance | `doubao-seedance-2-0-h`、`doubao-seedance-2-5-h`、`doubao-seedance-2-5-260628` | 文生影片、圖生影片、參考生影片、原生音頻 |
| Wan 3.0 | `wan3.0-video-a`、`wan3.0-video`、`wan3.0-video-prime` | 文生影片、首尾幀、參考圖/影片/音頻 |
| MiniMax H3 | `minimax-h3-a`、`MiniMax-H3` | 文生影片、首尾幀、多模態參考 |
| Kling | `kling-v3`、`kling-v3-omni` | 文生影片、圖生影片、運動控制、多模態參考 |
| Vidu | `viduq3-pro`、`viduq3-turbo`（含 `-fmi` 變體） | 文生影片、圖生影片、參考生影片 |

## 提交任務

### 通用參數

```json
{
  "model": "seedance-2.0-a",
  "prompt": "a cat walking in the rain, cinematic",
  "size": "1280x720",
  "duration": 5
}
```

| 參數 | 類型 | 必填 | 說明 |
| --- | --- | :---: | --- |
| `model` | string | 是 | 站內影片模型 ID |
| `prompt` | string | 是* | 影片描述；使用 `content` 直接傳文本時可省略 |
| `image` | string | 否 | 單張圖片 URL，等價於 `images: [url]` |
| `images` | string[] | 否 | 圖片 URL 數組；不同數量的含義見各系列說明 |
| `input_reference` | string | 否 | 單張參考圖 URL，按圖生影片處理 |
| `content` | array | 否 | Ark 風格內容數組，用於首幀/尾幀/參考素材；只有部分系列支援 |
| `size` | string | 否 | 分辨率或尺寸，如 `1280x720`、`720p`、`480P` |
| `resolution` | string | 否 | 分辨率檔位，如 `480p`、`720p`、`1080p`、`768P`、`2K` |
| `ratio` | string | 否 | 畫面比例，如 `16:9`、`9:16`、`1:1`、`adaptive` |
| `duration` | integer | 否 | 輸出秒數，默認通常爲 `5` |
| `seconds` | string | 否 | `duration` 的字符串別名 |
| `metadata` | object | 否 | 模型專屬參數，如 `job_type`、`media`、`mode`、`sound` |

\* `prompt` 與 `content` 中的文本二選一；兩者都沒有會返回參數錯誤。

### 通用規則

- 圖片、影片、音頻素材 URL 必須是上游可訪問的**公網 HTTP(S) 地址**，不能是 localhost、內網或需要登入的頁面。
- 需要鑑權的素材，請先下載到對象存儲或 CDN，再傳公網 URL。
- 時長、分辨率會同時影響生成結果和費用，建議顯式傳入，不要依賴默認值。
- 不同模型對 `metadata` 的字段名不同；下面按系列說明。

### 提交響應

```json
{
  "id": "task_J7xRonpligwgybxmcTNY8HJELhFfIIdE",
  "task_id": "task_J7xRonpligwgybxmcTNY8HJELhFfIIdE",
  "status": "queued",
  "model": "seedance-2.0-a",
  "created_at": 1793093555
}
```

保存 `task_id`，用於後續輪詢。

## 查詢任務

```bash
curl https://<base_url>/v1/video/generations/task_J7xRonpligwgybxmcTNY8HJELhFfIIdE \
  -H "Authorization: Bearer $CUBEROUTER_API_KEY"
```

通用查詢響應：

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

狀態值：

| 狀態 | 含義 |
| --- | --- |
| `SUBMITTED` / `QUEUED` | 已提交，等待處理 |
| `IN_PROGRESS` | 生成中 |
| `SUCCESS` | 成功，影片地址在 `result_url` |
| `FAILURE` | 失敗，原因在 `fail_reason` 或 `error` 中 |

成功時優先讀取 `data.result_url`；部分上游會在 `data.data` 中保留原始返回（例如 `creations`），可以作爲兜底。

### OpenAI 影片兼容格式

`/v1/videos` 系列返回 OpenAI 影片對象：

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

`status` 取值：`queued`、`in_progress`、`completed`、`failed`。任務成功後，可以通過 `GET /v1/videos/{id}/content` 下載影片字節。

## 各系列參數

### Seedance 2.0 / 2.5（`seedance-2.0-a`、`seedance-2.5-a`）

| 項目 | 說明 |
| --- | --- |
| 模式 | 文生、圖生、首尾幀、參考生影片 |
| 分辨率 | `480P`、`720P`、`1080P`；也接受 `832x480`、`1280x720`、`1920x1080` 等 |
| 時長 | `2–30` 秒，默認 `5` |
| 比例 | `16:9`（默認）、`9:16`、`4:3`、`3:4`、`1:1`、`adaptive` |
| 圖片 | `image` 或 `images`：1 張 = 圖生影片，2 張 + `metadata.job_type=first_last_frame` = 首尾幀 |
| 參考素材 | `metadata.medias`，元素爲 `{"type":"...","url":"..."}`；`type` 取 `image`、`video` 或 `audio` |
| 任務類型 | `metadata.job_type`：`text_to_video`、`image_to_video`、`first_last_frame`、`reference_to_video` |
| 數量 | `metadata.n` 只允許 `1` |

注意事項：

- 使用頂層 `size` / `duration`，**不要**用 `metadata.resolution` / `metadata.duration`，否則可能被拒絕。
- 該系列不支援 Ark 風格的 `content` 數組；請用 `prompt` + `images` + `metadata.medias`。
- `metadata.medias` 的 `url` 和 `media_id` 二選一；`url` 必須是公網地址。

示例：首尾幀

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

| 項目 | 說明 |
| --- | --- |
| 模式 | 文生、圖生、參考生影片、原生音頻 |
| 分辨率 | `480p`、`720p`、`1080p`；2.0 額外支援 `4k` |
| 時長 | 默認 `5` 秒；建議顯式傳入 |
| 比例 | `16:9`、`9:16`、`1:1`、`4:3`、`3:4`、`adaptive`（以實際支援爲準） |
| 圖片 | `image` 或 `images` |
| 精細控制 | `content` 數組，支援首幀/尾幀/參考圖/參考影片/參考音頻 |
| 音頻 | `metadata.generate_audio`；2.5 默認原生音頻開啓 |

注意事項：

- `prompt` 與 `content` 中的文本二選一。
- 2.5 和 2.0 的分辨率上限不同，4K 只有 2.0 支援。
- `content` 是 Ark 風格數組，和 Seedance（`seedance-*-a`）的用法不同。

示例：圖生影片

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

| 項目 | 說明 |
| --- | --- |
| 模式 | 文生、首幀、首尾幀、參考圖/影片/音頻 |
| 分辨率 | `480p`、`720p`、`1080p` |
| 時長 | `2–30` 秒，默認 `5` |
| 比例 | `21:9`、`16:9`、`4:3`、`1:1`、`3:4`、`9:16`、`adaptive` |
| 圖片 | `images`：1 張 = 首幀，2 張 = 首尾幀，3 張及以上 = 參考圖 |
| 完整素材控制 | `metadata.media`，支援 `first_frame`、`last_frame`、`reference_image`、`reference_video`、`reference_audio`、`file`、`link` |

素材限制：

- 首幀/尾幀與參考素材**互斥**，不能混用。
- 參考圖片 ≤ 10 張，參考影片 ≤ 5 段（合計 ≤ 15 秒），參考音頻 ≤ 5 段（合計 ≤ 15 秒）。
- 單次多模態素材合計最多 20 個。
- `file` 和 `link` 各限 1 個，且必須是公開可訪問的頁面。
- `wan3.0-video-a` 不支援 Ark 風格的 `content` 數組；用 `prompt` + `images` / `metadata.media`。

示例：參考圖生影片

```json
{
  "model": "wan3.0-video-a",
  "prompt": "讓參考圖中的貓在草地上奔跑，鏡頭緩慢跟隨",
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

| 項目 | 說明 |
| --- | --- |
| 模式 | 文生、首幀、首尾幀、多模態參考 |
| 分辨率 | `768P`、`2K` |
| 時長 | `4–15` 秒，默認 `5` |
| 比例 | `16:9`、`4:3`、`1:1`、`3:4`、`9:16`、`21:9` |
| 圖片 | `images`：1 張 = 首幀，2 張 = 首尾幀，3 張及以上 = 參考圖 |
| 完整素材控制 | `metadata.media`：`first_frame`、`last_frame`、`image_url`、`feature`、`driving_audio` |
| 水印 | `metadata.watermark=true` |

素材限制：

- 首幀/尾幀與參考素材互斥。
- 首幀、尾幀各 1 張；參考圖 ≤ 9 張；參考影片 ≤ 3 段；參考音頻 ≤ 3 段。
- **文生影片必須顯式指定 `ratio`，且不能爲 `adaptive`**；不傳時默認 `16:9`。
- 參考場景默認 `adaptive`。

示例：文生影片

```json
{
  "model": "minimax-h3-a",
  "prompt": "一隻橘貓蹲在窗臺上看雨，電影感，緩慢推近",
  "resolution": "768P",
  "ratio": "16:9",
  "duration": 5
}
```

### Kling（`kling-v3`、`kling-v3-omni`）

| 項目 | 說明 |
| --- | --- |
| 能力 | 文生、圖生、運動控制、多模態參考（omni） |
| 圖片 | `images`：1 張 = 首幀，2 張 = 首尾幀 |
| 模式 | `metadata.mode`：`std`、`pro`、`4k` |
| 聲音 | `metadata.sound`：`on`、`off` |
| 類型 | `metadata.kling_v3_type`：`t2v`、`i2v`、`motion_control`；不傳時按素材自動推導 |
| 比例 | `metadata.aspect_ratio` |
| 運動控制 | `metadata.img_url`、`metadata.video_url` |

- `kling-v3` 支援文生、圖生、首尾幀和運動控制，可生成帶聲音的影片。
- `kling-v3-omni` 更偏向多參考圖/參考影片的主體一致性生成和影片編輯。

### Vidu（`viduq3-pro`、`viduq3-turbo`）

| 項目 | 說明 |
| --- | --- |
| 模式 | 文生、圖生、參考生影片 |
| 分辨率 | `360p`、`540p`、`720p`、`1080p`；Q3 默認 `720p` |
| 時長 | 默認 `5` 秒 |
| 圖片 | `image`、`images` 或 `input_reference` |
| 說明 | `pro` 偏質量，`turbo` 偏速度；`-fmi` 變體面向特定組織，可用性以分組爲準 |

## 輪詢示例

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
    "prompt": "一隻兔子在沙灘上奔跑",
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

## 常見問題

### Q1: 提交成功但一直 `IN_PROGRESS`？

影片生成通常需要數十秒到數分鐘，複雜模型、長時長、高分辨率會更慢。建議輪詢間隔 5–10 秒，客戶端總超時至少 10 分鐘。長時間無進展時檢查任務日誌中的失敗原因。

### Q2: 素材 URL 傳了但上游讀不到？

素材必須是上游可訪問的公網 HTTP(S) 地址。localhost、內網地址、需要登入的頁面、臨時簽名過期都會失敗。建議先把素材放到對象存儲或 CDN。

### Q3: 首幀/尾幀和參考素材能一起用嗎？

Wan 3.0 和 MiniMax H3 都**不允許**首幀/尾幀與參考素材混用。需要首尾幀時只傳首尾幀；需要參考圖/影片/音頻時走參考模式。

### Q4: 爲什麼同樣的參數，另一個模型報錯？

不同系列的 `metadata` 字段名和取值範圍不同。例如 Seedance（`seedance-*-a`）用 `metadata.medias` 和 `metadata.job_type`，Doubao Seedance 用 `content`，Wan 用 `metadata.media`。按模型所屬系列傳參，不要跨系列套用。

### Q5: `/v1/videos` 和 `/v1/video/generations` 有什麼區別？

- `/v1/video/generations` 是站內通用任務接口，響應包含 `task_id`、`status`、`result_url` 等字段。
- `/v1/videos` 是 OpenAI 影片兼容接口，返回 OpenAI 影片對象，狀態值爲 `queued` / `in_progress` / `completed` / `failed`，成功後可通過 `/content` 下載。
- 兩者最終生成能力相同，選一種即可。

### Q6: 任務失敗後怎麼排查？

先看查詢響應中的 `fail_reason` 或 `error`，再對照任務日誌。常見原因包括素材 URL 不可訪問、參數超出模型範圍、上游內容審覈不通過。修正後重新提交即可。
