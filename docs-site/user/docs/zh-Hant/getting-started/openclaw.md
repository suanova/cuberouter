# OpenClaw

> 在 OpenClaw 中使用 CubeRouter 服務

OpenClaw 是一個在您自己的設備上運行的個人 AI 助手，可以連接到各種消息平臺。它可以透過 CubeRouter 服務使用各種模型。

## 步驟一：安裝 OpenClaw

> 詳細的安裝指南，請參考 [官方文檔](https://docs.openclaw.ai/zh-CN/install)

### 方式一：安裝腳本（推薦）

前置條件：

- [Node.js 22 或更新版本](https://nodejs.org/en/download/)

安裝 OpenClaw 最簡單的方法是使用官方安裝腳本：

::: code-group

```bash [macOS/Linux]
curl -fsSL https://openclaw.ai/install.sh | bash
```

```powershell [Windows (PowerShell)]
iwr -useb https://openclaw.ai/install.ps1 | iex
```

:::

### 方式二：全局安裝（手動）

前置條件：

- [Node.js 22 或更新版本](https://nodejs.org/en/download/)

::: code-group

```bash [pnpm（推薦）]
pnpm add -g openclaw@latest
pnpm approve-builds -g # 批准 openclaw、node-llama-cpp、sharp 等
```

```bash [npm]
npm install -g openclaw@latest
```

:::

## 步驟二：獲取 CubeRouter API 金鑰和模型 ID

1. **註冊賬號**：訪問 CubeRouter 平臺，完成賬號註冊並登入（參考[註冊與登入](register-login.md)）
2. **獲取 API 金鑰**：在「API 金鑰」頁面創建一個新的 API 金鑰（參考[快速開始](quick-start.md#第二步創建-api-金鑰)）
3. **獲取模型 ID**：在「模型廣場」頁面，使用「分組」篩選出 API 金鑰對應分組的可用模型，複製模型 ID，方便後續配置

## 步驟三：設置 OpenClaw

運行上述安裝命令後，配置過程將自動開始。如果沒有開始，您可以運行以下命令開始配置：

```bash
openclaw onboard --install-daemon
```

若之前已經初始化，您也可以運行 `openclaw config` 選擇 `model` 配置。

開始配置：

- `I understand this is powerful and inherently risky. Continue?`：選擇 `Yes`
- `Onboarding mode`：選擇 `Quick Start`
- `Model/auth provider`：選擇 `Custom Provider`

配置 CubeRouter 提供商：

- `API Base URL`：輸入 `https://your-platform.com/v1`
- `API Key`：輸入剛才創建的 API 金鑰
- `Endpoint compatibility`：選擇 `OpenAI-compatible`
- `Model ID`：輸入模型 ID

#### 完成設置

繼續完成剩餘的 OpenClaw 功能配置：

- `Select channel`：選擇並配置您需要的功能
- `Configure skills`：選擇並安裝您需要的功能
- 完成設置

#### 與機器人交互

設置完成後，CLI 會詢問您 `How do you want to hatch your bot?`

- 選擇 `Hatch in TUI (recommended)`

現在您可以在 Terminal UI 中開始與您的機器人聊天了。

OpenClaw 提供了更多渠道供您與機器人交互，如 Web UI、Discord、Slack 等。您可以透過參考官方文檔來設置這些渠道：[Channels Setup](https://docs.openclaw.ai/channels)

- 對於 Web UI，您可以透過打開終端中顯示的 `Web UI (with token)` 鏈接來訪問

#### 安裝後驗證

驗證一切是否正常工作：

```bash
openclaw doctor    # 檢查配置問題
openclaw status    # 查看網關狀態
openclaw dashboard # 瀏覽器打開 Dashboard
```

> 詳細的配置指南，請參考 [官方文檔](https://docs.openclaw.ai/zh-CN/start/getting-started)

::: warning 重要提醒
若配置不當或在沒有適當訪問控制的情況下部署，OpenClaw 可能會涉及安全風險。參考 [官方安全文檔](https://docs.openclaw.ai/gateway/security)
:::

## 高級配置

### 模型故障轉移

配置模型故障轉移以確保可靠性（`.openclaw/openclaw.json`）：

```json
{
  "agents": {
    "defaults": {
      "model": {
        "primary": "custom-provider/模型ID-A",
        "fallbacks": ["custom-provider/模型ID-B", "custom-provider/模型ID-C"]
      }
    }
  }
}
```

### 使用技能

> 技能是一個包含 SKILL.md 文件的文件夾。如果您想為 OpenClaw 代理添加新功能，[ClawHub](https://clawhub.ai/) 是查找安裝技能的最佳方法。

安裝 clawhub：

```bash
npm i -g clawhub
```

管理技能：

```bash
clawhub search "postgres backups"  # 搜索技能
clawhub install my-skill-pack      # 下載新技能
clawhub update --all               # 更新已安裝的技能
```

### 插件管理

> 插件是一個小的代碼模塊，透過額外功能（命令、工具和 Gateway RPC）擴展 OpenClaw。

```bash
openclaw plugins list                      # 查看已加載的插件
openclaw plugins install @openclaw/voice-call  # 安裝官方插件（示例：voice-call）
openclaw gateway restart                   # 重啟網關
```

## 故障排除

### 常見問題

1. **API 金鑰認證**
   - 確保您的 API 金鑰有效且沒有達到額度限制
   - 檢查 API 金鑰在環境中是否正確設置

2. **模型可用性**
   - 驗證模型在您的分組中是否可用
   - 檢查模型名稱（模型 ID）格式是否正確

3. **連接問題**
   - 確保 OpenClaw gateway 正在運行
   - 檢查到 CubeRouter 端點的網路連接

## 資源

- **OpenClaw 文檔**：[docs.openclaw.ai](https://docs.openclaw.ai/)
- **OpenClaw GitHub**：[github.com/openclaw/openclaw](https://github.com/openclaw/openclaw)
- **社區技能**：[awesome-openclaw-skills](https://github.com/VoltAgent/awesome-openclaw-skills)
