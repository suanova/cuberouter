# OpenCode

> 在 OpenCode 中使用 CubeRouter 的方法

OpenCode 既是一款在終端中運行的 CLI + TUI AI 編程代理工具，也提供 IDE 的插件集成，能夠在不同開發環境下完成快速代碼生成、調試、項目分析、文件操作與跨項目協作等任務。

## 步驟一：安裝 OpenCode

安裝 OpenCode 最簡單的方式是使用官方安裝腳本：

```bash
curl -fsSL https://opencode.ai/install | bash
```

你也可以使用 npm 安裝：

```bash
npm install -g opencode-ai
```

## 步驟二：獲取 CubeRouter API 金鑰和模型 ID

1. **註冊賬號**：訪問 CubeRouter 平臺，完成賬號註冊並登入（參考[註冊與登入](register-login.md)）
2. **獲取 API 金鑰**：在「API 金鑰」頁面創建一個新的 API 金鑰（參考[快速開始](quick-start.md#第二步創建-api-金鑰)）
3. **獲取模型 ID**：在「模型廣場」頁面，使用「分組」篩選出 API 金鑰對應分組的可用模型，複製模型 ID，方便後續配置

## 步驟三：配置 OpenCode

::: warning 重要提醒
支援 macOS & Linux & Windows，注意不同系統配置文件路徑不一樣。注意需保證修改的 JSON 文件格式正確性（比如多或少 `,`）。
:::

編輯或新增 `settings.json` 文件（macOS & Linux 為 `~/.config/opencode/opencode.json`，Windows 為 `用戶目錄\.config\opencode\opencode.json`），配置自定義 provider：

```jsonc
// 注意替換 your_platform_api_key 為您獲取到的 API 金鑰
// 注意替換 your_model_id 為您獲取到的模型 ID
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "cuberouter": {
      "options": {
        "baseURL": "https://your-platform.com/v1",
        "apiKey": "your_platform_api_key"
      },
      "models": {
        "your_model_id": {}
      }
    }
  },
  "model": "cuberouter/your_model_id"
}
```

例如 API 金鑰為 `sk-APIKEY123abc456def789`、模型 ID 為 `glm-5`，則配置為：

```json
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "cuberouter": {
      "options": {
        "baseURL": "https://your-platform.com/v1",
        "apiKey": "sk-APIKEY123abc456def789"
      },
      "models": {
        "glm-5": {}
      }
    }
  },
  "model": "cuberouter/glm-5"
}
```

## 步驟四：開始使用 OpenCode

配置完成後，進入您的代碼工作目錄，在終端中執行 `opencode` 命令即可開始使用 OpenCode。選擇 CubeRouter 提供商和對應模型，即可正常使用 OpenCode 進行開發。
