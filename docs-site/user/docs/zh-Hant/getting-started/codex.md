# Codex

> 在 Codex 中使用 CubeRouter 的方法

Codex 是一個智能編碼工具，可以在終端中運行，透過自然語言命令交互幫助開發者快速完成代碼生成、調試、重構等任務。

## 步驟一：安裝 Codex

前提條件：

- 您需要安裝 [Node.js 18 或更新版本](https://nodejs.org/en/download/)
- macOS 用戶推薦使用 [nvm](https://github.com/nvm-sh/nvm) 或 [Homebrew](https://formulae.brew.sh/formula/node) 方式安裝 Node.js。不推薦直接安裝包安裝（後續可能會遇到權限問題）
- Windows 用戶還需安裝 [Git for Windows](https://git-scm.com/install/windows)

進入命令行界面，安裝 Codex：

```bash
npm install -g @openai/codex
```

運行如下命令，查看安裝結果，若顯示版本號則表示安裝成功：

```bash
codex --version
```

## 步驟二：獲取 CubeRouter API 金鑰和模型 ID

1. **註冊賬號**：訪問 CubeRouter 平臺，完成賬號註冊並登入（參考[註冊與登入](register-login.md)）
2. **獲取 API 金鑰**：在「API 金鑰」頁面創建一個新的 API 金鑰（參考[快速開始](quick-start.md#第二步創建-api-金鑰)）
3. **獲取模型 ID**：在「模型廣場」頁面，使用「端點類型」篩選出支援 Response 的可用模型，複製模型 ID，方便後續配置

## 步驟三：配置 Codex

::: warning 重要提醒
支援 macOS & Linux & Windows，注意不同系統配置文件路徑不一樣。
:::

編輯或新增 `config.toml` 文件（macOS & Linux 為 `~/.codex/config.toml`，Windows 為 `用戶目錄/.codex/config.toml`）

```toml
model_provider = "cuberouter"
model = "<you_selected_model_id>"  # e.g. gpt-5.6-terra

[model_providers.cuberouter]
name = "CubeRouter"
base_url = "<base_url>" # e.g. https://cuberouter.cn or https://cuberouter.com  
env_key = "CODEX_CUBEROUTER_API_KEY"
wire_api = "responses"

```

## 步驟四：開始使用 Codex

配置完成後，進入您的代碼工作目錄，在終端中執行以下命令即可開始使用 Codex。

```sh
export CODEX_CUBEROUTER_API_KEY=<your_cuberouter_api_key>
codex
```
