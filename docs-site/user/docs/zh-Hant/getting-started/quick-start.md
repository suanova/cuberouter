# 快速開始

> 本指南將幫助您快速上手 CubeRouter，從註冊賬號到發起第一次 API 調用，只需幾分鐘即可完成。

## 第一步：註冊並登入賬號

訪問 CubeRouter 首頁，點擊「獲取 API Key」按鈕（或登入頁的「沒有賬號？註冊」鏈接），進入註冊頁面。

![註冊頁面](/imgs/register.jpeg)

**註冊流程**：

1. 填寫**用戶名**：建議使用英文，用戶名一旦註冊不可修改
2. 填寫**郵箱**：用於接收驗證碼，必填
3. 填寫**密碼**：8-20 位，建議包含字母和數字
4. 點擊「註冊」完成賬號創建

::: tip 註冊提示
- 用戶名一旦註冊不可修改，請謹慎選擇
- 密碼長度為 8-20 位，建議使用密碼管理器妥善保管
- 如平臺配置了第三方登入（GitHub、Discord、OIDC 等），也可一鍵註冊登入
:::

註冊成功後跳轉到登入頁，輸入用戶名和密碼即可登入。

![登入頁面](/imgs/login.jpeg)

登入成功後即可進入控制台（概覽頁）。

## 第二步：創建 API 金鑰

在「API 金鑰」頁面創建您的第一個金鑰。左側導航點擊「API 金鑰」，或直接訪問 `/keys`。

![創建 API 金鑰](/imgs/token-create.jpeg)

**創建步驟**：

1. 點擊「創建 API 金鑰」按鈕
2. 填寫金鑰「名稱」：建議按用途命名（如 `default`、`prod`、`dev-test`）
3. 選擇「分組」：不同分組可訪問的模型不同
4. 設置「過期時間」：可選擇永不過期或指定有效期
5. 設置「數量」：可批量創建多個金鑰（默認為 1 個）
6. 展開「高級設置」可配置模型限制、IP 白名單等
7. 開啟「無限配額」則不受配額限制，否則可設置該金鑰可用的最高額度
8. 點擊「創建」並**立即複製保存**金鑰

::: warning 重要提醒
API 金鑰僅在創建時完整顯示一次，請立即複製保存。金鑰具有完整的 API 調用權限，請勿洩露給他人，不要提交到代碼倉庫，建議使用環境變量或配置文件存儲。
:::

創建後可在列表頁查看和管理金鑰：

![API 金鑰列表](/imgs/token-list.jpeg)

## 第三步：添加額度

使用 API 前需要賬戶中有可用額度。左側導航點擊「錢包」，或直接訪問 `/wallet`，可透過在線支付或兌換碼為賬戶充值。詳見[配額與充值](../guide/wallet.md)。

## 第四步：查看模型

點擊頂部導航欄的「模型廣場」查看所有可用模型及其價格。

![模型廣場](/imgs/models-market.jpeg)

使用左側「分組」篩選，可以查看當前 API 金鑰所在分組的可用模型，複製模型名稱即可用於 API 調用。詳見[模型廣場](../guide/models-market.md)。

## 第五步：發起第一次調用

CubeRouter 支援 OpenAI 兼容的 API。將平臺地址作為 `base_url`，配合 API 金鑰即可開始調用：

```bash
curl https://your-platform.com/v1/chat/completions \
  -H "Authorization: Bearer sk-你的API金鑰" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "模型名稱",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

API Base URL 為當前服務域名。更多代碼示例（Python / Claude / Gemini）見[使用 API](#使用-api)。

## 第六步：選擇客戶端工具

CubeRouter 支援 OpenAI 兼容的 API，可以在多種工具中使用：

- [Claude Code](claude-code.md)
- [OpenCode](opencode.md)
- [OpenClaw](openclaw.md)

## 使用 API

### 遊樂場在線測試

[遊樂場](../guide/playground.md)是內置的在線測試工具，無需編寫代碼即可直接與模型對話，適合快速驗證金鑰是否可用。

### 獲取 API 地址


使用當前域名作為你的客戶端或代碼中作為 `base_url`，配合金鑰即可開始調用。

### 代碼示例

**Python（OpenAI SDK）**：

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-xxxxxxxxxxxxxxxx",  # 平臺頒發的金鑰
    base_url="https://your-platform.com/v1"
)

response = client.chat.completions.create(
    model="模型名稱",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.choices[0].message.content)
```

**Claude 原生格式**：

```bash
curl https://your-platform.com/v1/messages \
  -H "x-api-key: sk-xxxxxxxx" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model": "claude-3-5-sonnet-20241022", "max_tokens": 1024, "messages": [{"role": "user", "content": "Hello"}]}'
```

**Gemini 原生格式**：

```bash
curl "https://your-platform.com/v1beta/models/gemini-1.5-pro:generateContent?key=sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{"contents": [{"parts": [{"text": "Hello"}]}]}'
```

### 支援的接口端點

| 接口 | 路徑 | 說明 |
| --- | --- | --- |
| 聊天補全 | `POST /v1/chat/completions` | 對話生成，支援流式輸出 |
| 文本補全 | `POST /v1/completions` | 傳統補全接口 |
| 向量嵌入 | `POST /v1/embeddings` | 文本向量化 |
| 圖像生成 | `POST /v1/images/generations` | 文生圖 |
| 圖像編輯 | `POST /v1/images/edits` | 圖像編輯 |
| 語音轉文字 | `POST /v1/audio/transcriptions` | Whisper 等 |
| 文字轉語音 | `POST /v1/audio/speech` | TTS |
| 重排序 | `POST /v1/rerank` | 文檔重排序 |
| Responses API | `POST /v1/responses` | OpenAI Responses 格式 |
| 實時對話 | `GET /v1/realtime`（WebSocket） | OpenAI Realtime API |
| 模型列表 | `GET /v1/models` | 查詢可用模型 |
