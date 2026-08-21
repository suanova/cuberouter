# OpenClaw

> 在 OpenClaw 中使用 CubeRouter 服务

OpenClaw 是一个在您自己的设备上运行的个人 AI 助手，可以连接到各种消息平台。它可以通过 CubeRouter 服务使用各种模型。

## 步骤一：安装 OpenClaw

> 详细的安装指南，请参考 [官方文档](https://docs.openclaw.ai/zh-CN/install)

### 方式一：安装脚本（推荐）

前置条件：

- [Node.js 22 或更新版本](https://nodejs.org/en/download/)

安装 OpenClaw 最简单的方法是使用官方安装脚本：

::: code-group

```bash [macOS/Linux]
curl -fsSL https://openclaw.ai/install.sh | bash
```

```powershell [Windows (PowerShell)]
iwr -useb https://openclaw.ai/install.ps1 | iex
```

:::

### 方式二：全局安装（手动）

前置条件：

- [Node.js 22 或更新版本](https://nodejs.org/en/download/)

::: code-group

```bash [pnpm（推荐）]
pnpm add -g openclaw@latest
pnpm approve-builds -g # 批准 openclaw、node-llama-cpp、sharp 等
```

```bash [npm]
npm install -g openclaw@latest
```

:::

## 步骤二：获取 CubeRouter API 密钥和模型 ID

1. **注册账号**：访问 CubeRouter 平台，完成账号注册并登录（参考[注册与登录](register-login.md)）
2. **获取 API 密钥**：在「API 密钥」页面创建一个新的 API 密钥（参考[快速开始](quick-start.md#第二步创建-api-密钥)）
3. **获取模型 ID**：在「模型广场」页面，使用「分组」筛选出 API 密钥对应分组的可用模型，复制模型 ID，方便后续配置

## 步骤三：设置 OpenClaw

运行上述安装命令后，配置过程将自动开始。如果没有开始，您可以运行以下命令开始配置：

```bash
openclaw onboard --install-daemon
```

若之前已经初始化，您也可以运行 `openclaw config` 选择 `model` 配置。

开始配置：

- `I understand this is powerful and inherently risky. Continue?`：选择 `Yes`
- `Onboarding mode`：选择 `Quick Start`
- `Model/auth provider`：选择 `Custom Provider`

配置 CubeRouter 提供商：

- `API Base URL`：输入 `https://your-platform.com/v1`
- `API Key`：输入刚才创建的 API 密钥
- `Endpoint compatibility`：选择 `OpenAI-compatible`
- `Model ID`：输入模型 ID

#### 完成设置

继续完成剩余的 OpenClaw 功能配置：

- `Select channel`：选择并配置您需要的功能
- `Configure skills`：选择并安装您需要的功能
- 完成设置

#### 与机器人交互

设置完成后，CLI 会询问您 `How do you want to hatch your bot?`

- 选择 `Hatch in TUI (recommended)`

现在您可以在 Terminal UI 中开始与您的机器人聊天了。

OpenClaw 提供了更多渠道供您与机器人交互，如 Web UI、Discord、Slack 等。您可以通过参考官方文档来设置这些渠道：[Channels Setup](https://docs.openclaw.ai/channels)

- 对于 Web UI，您可以通过打开终端中显示的 `Web UI (with token)` 链接来访问

#### 安装后验证

验证一切是否正常工作：

```bash
openclaw doctor    # 检查配置问题
openclaw status    # 查看网关状态
openclaw dashboard # 浏览器打开 Dashboard
```

> 详细的配置指南，请参考 [官方文档](https://docs.openclaw.ai/zh-CN/start/getting-started)

::: warning 重要提醒
若配置不当或在没有适当访问控制的情况下部署，OpenClaw 可能会涉及安全风险。参考 [官方安全文档](https://docs.openclaw.ai/gateway/security)
:::

## 高级配置

### 模型故障转移

配置模型故障转移以确保可靠性（`.openclaw/openclaw.json`）：

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

> 技能是一个包含 SKILL.md 文件的文件夹。如果您想为 OpenClaw 代理添加新功能，[ClawHub](https://clawhub.ai/) 是查找安装技能的最佳方法。

安装 clawhub：

```bash
npm i -g clawhub
```

管理技能：

```bash
clawhub search "postgres backups"  # 搜索技能
clawhub install my-skill-pack      # 下载新技能
clawhub update --all               # 更新已安装的技能
```

### 插件管理

> 插件是一个小的代码模块，通过额外功能（命令、工具和 Gateway RPC）扩展 OpenClaw。

```bash
openclaw plugins list                      # 查看已加载的插件
openclaw plugins install @openclaw/voice-call  # 安装官方插件（示例：voice-call）
openclaw gateway restart                   # 重启网关
```

## 故障排除

### 常见问题

1. **API 密钥认证**
   - 确保您的 API 密钥有效且没有达到额度限制
   - 检查 API 密钥在环境中是否正确设置

2. **模型可用性**
   - 验证模型在您的分组中是否可用
   - 检查模型名称（模型 ID）格式是否正确

3. **连接问题**
   - 确保 OpenClaw gateway 正在运行
   - 检查到 CubeRouter 端点的网络连接

## 资源

- **OpenClaw 文档**：[docs.openclaw.ai](https://docs.openclaw.ai/)
- **OpenClaw GitHub**：[github.com/openclaw/openclaw](https://github.com/openclaw/openclaw)
- **社区技能**：[awesome-openclaw-skills](https://github.com/VoltAgent/awesome-openclaw-skills)
