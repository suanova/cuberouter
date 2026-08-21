# Claude Code

> 在 Claude Code 中使用 CubeRouter 的方法

Claude Code 是一個智能編碼工具，可以在終端中運行，透過自然語言命令交互幫助開發者快速完成代碼生成、調試、重構等任務。

## 步驟一：安裝 Claude Code

前提條件：

- 您需要安裝 [Node.js 18 或更新版本](https://nodejs.org/en/download/)
- macOS 用戶推薦使用 [nvm](https://github.com/nvm-sh/nvm) 或 [Homebrew](https://formulae.brew.sh/formula/node) 方式安裝 Node.js。不推薦直接安裝包安裝（後續可能會遇到權限問題）
- Windows 用戶還需安裝 [Git for Windows](https://git-scm.com/install/windows)

進入命令行界面，安裝 Claude Code：

```bash
npm install -g @anthropic-ai/claude-code
```

運行如下命令，查看安裝結果，若顯示版本號則表示安裝成功：

```bash
claude --version
```

## 步驟二：獲取 CubeRouter API 金鑰和模型 ID

1. **註冊賬號**：訪問 CubeRouter 平臺，完成賬號註冊並登入（參考[註冊與登入](register-login.md)）
2. **獲取 API 金鑰**：在「API 金鑰」頁面創建一個新的 API 金鑰（參考[快速開始](quick-start.md#第二步創建-api-金鑰)）
3. **獲取模型 ID**：在「模型廣場」頁面，使用「分組」篩選出 API 金鑰對應分組的可用模型，複製模型 ID，方便後續配置

## 步驟三：配置 Claude Code

::: warning 重要提醒
支援 macOS & Linux & Windows，注意不同系統配置文件路徑不一樣。注意需保證修改的 JSON 文件格式正確性（比如多或少 `,`）。
:::

在 **macOS**、**Linux** 或 **Windows** 中透過修改配置文件設置環境變量：

```jsonc
// 編輯或新增 settings.json 文件
// macOS & Linux 為 ~/.claude/settings.json
// Windows 為 用戶目錄/.claude/settings.json
// 新增或修改裡面的 env 字段
// 注意替換 your_platform_api_key 為您獲取到的 API 金鑰
// 注意替換 your_model_id 為您獲取到的模型 ID
{
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "your_platform_api_key",
    "ANTHROPIC_BASE_URL": "https://your-platform.com",
    "API_TIMEOUT_MS": "3000000",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": 1,
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "your_model_id",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "your_model_id",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "your_model_id"
  }
}
```

再編輯或新增 `.claude.json` 文件（macOS & Linux 為 `~/.claude.json`，Windows 為 `用戶目錄/.claude.json`），新增 `hasCompletedOnboarding` 參數：

```json
{
  "hasCompletedOnboarding": true
}
```

## 步驟四：開始使用 Claude Code

配置完成後，進入您的代碼工作目錄，在終端中執行 `claude` 命令即可開始使用 Claude Code。

> 若遇到「Do you want to use this API key」提示，選擇 Yes 即可。

啟動後選擇信任 Claude Code 訪問文件夾裡的文件，即可正常使用 Claude Code 進行開發。
