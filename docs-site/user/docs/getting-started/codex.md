# Codex

> 在 Codex 中使用 CubeRouter 的方法

Codex 是一个智能编码工具，可以在终端中运行，通过自然语言命令交互帮助开发者快速完成代码生成、调试、重构等任务。

## 步骤一：安装 Codex

前提条件：

- 您需要安装 [Node.js 18 或更新版本](https://nodejs.org/en/download/)
- macOS 用户推荐使用 [nvm](https://github.com/nvm-sh/nvm) 或 [Homebrew](https://formulae.brew.sh/formula/node) 方式安装 Node.js。不推荐直接安装包安装（后续可能会遇到权限问题）
- Windows 用户还需安装 [Git for Windows](https://git-scm.com/install/windows)

进入命令行界面，安装 Codex：

```bash
npm install -g @openai/codex
```

运行如下命令，查看安装结果，若显示版本号则表示安装成功：

```bash
codex --version
```

## 步骤二：获取 CubeRouter API 密钥和模型 ID

1. **注册账号**：访问 CubeRouter 平台，完成账号注册并登录（参考[注册与登录](register-login.md)）
2. **获取 API 密钥**：在「API 密钥」页面创建一个新的 API 密钥（参考[快速开始](quick-start.md#第二步创建-api-密钥)）
3. **获取模型 ID**：在「模型广场」页面，使用「端点类型」筛选出支持 Response 的可用模型，复制模型 ID，方便后续配置

## 步骤三：配置 Codex

::: warning 重要提醒
支持 macOS & Linux & Windows，注意不同系统配置文件路径不一样。
:::

编辑或新增 `config.toml` 文件（macOS & Linux 为 `~/.codex/config.toml`，Windows 为 `用户目录/.codex/config.toml`）

```toml
model_provider = "cuberouter"
model = "<you_selected_model_id>"  # e.g. gpt-5.6-terra

[model_providers.cuberouter]
name = "CubeRouter"
base_url = "<base_url>" # e.g. https://cuberouter.cn or https://cuberouter.com  
env_key = "CODEX_CUBEROUTER_API_KEY"
wire_api = "responses"

```

## 步骤四：开始使用 Codex

配置完成后，进入您的代码工作目录，在终端中执行以下命令即可开始使用 Codex。

```sh
export CODEX_CUBEROUTER_API_KEY=<your_cuberouter_api_key>
codex
```
