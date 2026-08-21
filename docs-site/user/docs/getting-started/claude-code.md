# Claude Code

> 在 Claude Code 中使用 CubeRouter 的方法

Claude Code 是一个智能编码工具，可以在终端中运行，通过自然语言命令交互帮助开发者快速完成代码生成、调试、重构等任务。

## 步骤一：安装 Claude Code

前提条件：

- 您需要安装 [Node.js 18 或更新版本](https://nodejs.org/en/download/)
- macOS 用户推荐使用 [nvm](https://github.com/nvm-sh/nvm) 或 [Homebrew](https://formulae.brew.sh/formula/node) 方式安装 Node.js。不推荐直接安装包安装（后续可能会遇到权限问题）
- Windows 用户还需安装 [Git for Windows](https://git-scm.com/install/windows)

进入命令行界面，安装 Claude Code：

```bash
npm install -g @anthropic-ai/claude-code
```

运行如下命令，查看安装结果，若显示版本号则表示安装成功：

```bash
claude --version
```

## 步骤二：获取 CubeRouter API 密钥和模型 ID

1. **注册账号**：访问 CubeRouter 平台，完成账号注册并登录（参考[注册与登录](register-login.md)）
2. **获取 API 密钥**：在「API 密钥」页面创建一个新的 API 密钥（参考[快速开始](quick-start.md#第二步创建-api-密钥)）
3. **获取模型 ID**：在「模型广场」页面，使用「分组」筛选出 API 密钥对应分组的可用模型，复制模型 ID，方便后续配置

## 步骤三：配置 Claude Code

::: warning 重要提醒
支持 macOS & Linux & Windows，注意不同系统配置文件路径不一样。注意需保证修改的 JSON 文件格式正确性（比如多或少 `,`）。
:::

在 **macOS**、**Linux** 或 **Windows** 中通过修改配置文件设置环境变量：

```jsonc
// 编辑或新增 settings.json 文件
// macOS & Linux 为 ~/.claude/settings.json
// Windows 为 用户目录/.claude/settings.json
// 新增或修改里面的 env 字段
// 注意替换 your_platform_api_key 为您获取到的 API 密钥
// 注意替换 your_model_id 为您获取到的模型 ID
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

再编辑或新增 `.claude.json` 文件（macOS & Linux 为 `~/.claude.json`，Windows 为 `用户目录/.claude.json`），新增 `hasCompletedOnboarding` 参数：

```json
{
  "hasCompletedOnboarding": true
}
```

## 步骤四：开始使用 Claude Code

配置完成后，进入您的代码工作目录，在终端中执行 `claude` 命令即可开始使用 Claude Code。

> 若遇到「Do you want to use this API key」提示，选择 Yes 即可。

启动后选择信任 Claude Code 访问文件夹里的文件，即可正常使用 Claude Code 进行开发。
