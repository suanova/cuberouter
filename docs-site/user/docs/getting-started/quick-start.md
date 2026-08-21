# 快速开始

> 本指南将帮助您快速上手 CubeRouter，从注册账号到发起第一次 API 调用，只需几分钟即可完成。

## 第一步：注册并登录账号

访问 CubeRouter 首页，点击「获取 API Key」按钮（或登录页的「没有账号？注册」链接），进入注册页面。

![注册页面](/imgs/register.jpeg)

**注册流程**：

1. 填写**用户名**：建议使用英文，用户名一旦注册不可修改
2. 填写**邮箱**：用于接收验证码，必填
3. 填写**密码**：8-20 位，建议包含字母和数字
4. 点击「注册」完成账号创建

::: tip 注册提示
- 用户名一旦注册不可修改，请谨慎选择
- 密码长度为 8-20 位，建议使用密码管理器妥善保管
- 如平台配置了第三方登录（GitHub、Discord、OIDC 等），也可一键注册登录
:::

注册成功后跳转到登录页，输入用户名和密码即可登录。

![登录页面](/imgs/login.jpeg)

登录成功后即可进入控制台（概览页）。

## 第二步：创建 API 密钥

在「API 密钥」页面创建您的第一个密钥。左侧导航点击「API 密钥」，或直接访问 `/keys`。

![创建 API 密钥](/imgs/token-create.jpeg)

**创建步骤**：

1. 点击「创建 API 密钥」按钮
2. 填写密钥「名称」：建议按用途命名（如 `default`、`prod`、`dev-test`）
3. 选择「分组」：不同分组可访问的模型不同
4. 设置「过期时间」：可选择永不过期或指定有效期
5. 设置「数量」：可批量创建多个密钥（默认为 1 个）
6. 展开「高级设置」可配置模型限制、IP 白名单等
7. 开启「无限配额」则不受配额限制，否则可设置该密钥可用的最高额度
8. 点击「创建」并**立即复制保存**密钥

::: warning 重要提醒
API 密钥仅在创建时完整显示一次，请立即复制保存。密钥具有完整的 API 调用权限，请勿泄露给他人，不要提交到代码仓库，建议使用环境变量或配置文件存储。
:::

创建后可在列表页查看和管理密钥：

![API 密钥列表](/imgs/token-list.jpeg)

## 第三步：添加额度

使用 API 前需要账户中有可用额度。左侧导航点击「钱包」，或直接访问 `/wallet`，可通过在线支付或兑换码为账户充值。详见[配额与充值](../guide/wallet.md)。

## 第四步：查看模型

点击顶部导航栏的「模型广场」查看所有可用模型及其价格。

![模型广场](/imgs/models-market.jpeg)

使用左侧「分组」筛选，可以查看当前 API 密钥所在分组的可用模型，复制模型名称即可用于 API 调用。详见[模型广场](../guide/models-market.md)。

## 第五步：发起第一次调用

CubeRouter 支持 OpenAI 兼容的 API。将平台地址作为 `base_url`，配合 API 密钥即可开始调用：

```bash
curl https://your-platform.com/v1/chat/completions \
  -H "Authorization: Bearer sk-你的API密钥" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "模型名称",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

API Base URL 为当前服务域名。更多代码示例（Python / Claude / Gemini）见[使用 API](#使用-api)。

## 第六步：选择客户端工具

CubeRouter 支持 OpenAI 兼容的 API，可以在多种工具中使用：

- [Claude Code](claude-code.md)
- [OpenCode](opencode.md)
- [OpenClaw](openclaw.md)

## 使用 API

### 游乐场在线测试

[游乐场](../guide/playground.md)是内置的在线测试工具，无需编写代码即可直接与模型对话，适合快速验证密钥是否可用。

### 获取 API 地址


使用当前域名作为你的客户端或代码中作为 `base_url`，配合密钥即可开始调用。

### 代码示例

**Python（OpenAI SDK）**：

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-xxxxxxxxxxxxxxxx",  # 平台颁发的密钥
    base_url="https://your-platform.com/v1"
)

response = client.chat.completions.create(
    model="模型名称",
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

### 支持的接口端点

| 接口 | 路径 | 说明 |
| --- | --- | --- |
| 聊天补全 | `POST /v1/chat/completions` | 对话生成，支持流式输出 |
| 文本补全 | `POST /v1/completions` | 传统补全接口 |
| 向量嵌入 | `POST /v1/embeddings` | 文本向量化 |
| 图像生成 | `POST /v1/images/generations` | 文生图 |
| 图像编辑 | `POST /v1/images/edits` | 图像编辑 |
| 语音转文字 | `POST /v1/audio/transcriptions` | Whisper 等 |
| 文字转语音 | `POST /v1/audio/speech` | TTS |
| 重排序 | `POST /v1/rerank` | 文档重排序 |
| Responses API | `POST /v1/responses` | OpenAI Responses 格式 |
| 实时对话 | `GET /v1/realtime`（WebSocket） | OpenAI Realtime API |
| 模型列表 | `GET /v1/models` | 查询可用模型 |
