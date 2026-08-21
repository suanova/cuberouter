# OpenCode

> 在 OpenCode 中使用 CubeRouter 的方法

OpenCode 既是一款在终端中运行的 CLI + TUI AI 编程代理工具，也提供 IDE 的插件集成，能够在不同开发环境下完成快速代码生成、调试、项目分析、文件操作与跨项目协作等任务。

## 步骤一：安装 OpenCode

安装 OpenCode 最简单的方式是使用官方安装脚本：

```bash
curl -fsSL https://opencode.ai/install | bash
```

你也可以使用 npm 安装：

```bash
npm install -g opencode-ai
```

## 步骤二：获取 CubeRouter API 密钥和模型 ID

1. **注册账号**：访问 CubeRouter 平台，完成账号注册并登录（参考[注册与登录](register-login.md)）
2. **获取 API 密钥**：在「API 密钥」页面创建一个新的 API 密钥（参考[快速开始](quick-start.md#第二步创建-api-密钥)）
3. **获取模型 ID**：在「模型广场」页面，使用「分组」筛选出 API 密钥对应分组的可用模型，复制模型 ID，方便后续配置

## 步骤三：配置 OpenCode

::: warning 重要提醒
支持 macOS & Linux & Windows，注意不同系统配置文件路径不一样。注意需保证修改的 JSON 文件格式正确性（比如多或少 `,`）。
:::

编辑或新增 `settings.json` 文件（macOS & Linux 为 `~/.config/opencode/opencode.json`，Windows 为 `用户目录\.config\opencode\opencode.json`），配置自定义 provider：

```jsonc
// 注意替换 your_platform_api_key 为您获取到的 API 密钥
// 注意替换 your_model_id 为您获取到的模型 ID
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

例如 API 密钥为 `sk-APIKEY123abc456def789`、模型 ID 为 `glm-5`，则配置为：

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

## 步骤四：开始使用 OpenCode

配置完成后，进入您的代码工作目录，在终端中执行 `opencode` 命令即可开始使用 OpenCode。选择 CubeRouter 提供商和对应模型，即可正常使用 OpenCode 进行开发。
