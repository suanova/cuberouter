# Channel Management

Channel management is the core feature of CubeRouter. It is used to configure and manage API channels for various AI model providers — each channel maps to a provider's API Key. Through channel management, admins can add, edit, test and monitor multiple AI providers flexibly.

Log in with an admin account, click **Channels** in the left navigation, or visit `/console/channel` directly.

::: warning Compliance notice
Upstream channels must be accounts, API Keys, model services or enterprise contracts that the deployment owner legally owns or is authorized to use. Load balancing, automatic failover, weighted random and multi-key management are designed for high availability and enterprise multi-account management — follow upstream terms of service, platform rules and regulatory requirements when using them.
:::

## Quick Navigation

| Action | Description |
|------|------|
| [Create a channel](#create-a-channel) | Add a new AI provider |
| [Edit a channel](#edit-a-channel) | Modify channel configuration |
| [Test a channel](#channel-testing) | Verify channel connectivity |
| [Batch operations](#batch-operations) | Manage multiple channels at once |
| [Advanced settings](./advanced) | Model mapping, weights, priority, parameter overrides |

## Overview

![Channel list](/imgs/channel-list.png)

The channel list shows all configured AI provider channels, including name, type, status (green = healthy / red = disabled), response time and used quota.

The channel management page provides the following core features:

| Feature | Description |
|------|------|
| **Channel list** | View all configured channels, including type, status and response time |
| **Add channel** | Create a new AI provider channel |
| **Edit channel** | Modify existing channel configuration |
| **Test channel** | Verify that a channel works |
| **Delete channel** | Remove unwanted channels |

## Channel Configuration

### Basic Configuration

1. Click the **Add Channel** button in the top-right corner of the channel list to open the configuration dialog
2. Select the provider type (e.g. OpenAI, Claude, Gemini)
3. Enter the channel name and API Key

![Add channel dialog (basic info)](/imgs/channel-add-basic.png)

| Field | Required | Description |
|------|:----:|------|
| **Channel Name** | ✓ | Display name for identification and management |
| **Channel Type** | ✓ | AI provider type |
| **API Key** | ✓ | The provider's API key |
| **Base URL** | | Custom API endpoint |
| **Model List** | ✓ | Models supported by this channel |
| **User Groups** | | User groups allowed to access this channel |

### Selecting Models

Check the models supported by this channel in the model list, or click **Fill Default Models** to populate automatically.

### Advanced Configuration

Expand the advanced configuration section as needed and fill in the optional fields:

![Add channel dialog (advanced config)](/imgs/channel-add-advanced.png)

| Option | Description |
| --- | --- |
| Base URL | Custom endpoint, used for proxies or private deployments |
| Priority | Higher values are selected first; default 0 |
| Weight | Random weight among channels with the same priority; default 0 |
| Model Mapping | Map requested model names to actual model names, JSON format |
| Parameter Override | Force-override certain request parameters, JSON format |
| Auto-disable | When enabled, automatically disables the channel after consecutive failures reach the threshold |

### Supported Provider Types

CubeRouter supports **40+** AI service providers:

::: details International providers

| Provider | Representative models | Highlights |
|--------|----------|------|
| **OpenAI** | GPT-4, GPT-4o, GPT-3.5 | Industry benchmark, full-featured |
| **Anthropic** | Claude 3.5 Sonnet, Claude 3 Opus | Long context, strong security |
| **Google** | Gemini 1.5 Pro, Gemini 1.5 Flash | Multimodal, long context |
| **Azure OpenAI** | GPT-4, GPT-3.5 | Enterprise SLA, strong compliance |
| **AWS Bedrock** | Claude, Llama, Titan | AWS ecosystem integration |
| **OpenRouter** | Unified access to 1000+ models | Transparent pricing, one-stop gateway |

:::

::: details Chinese providers

| Provider | Representative models | Highlights |
|--------|----------|------|
| **Alibaba Cloud** | Qwen (Tongyi Qianwen) | Strong Chinese language capability |
| **Baidu** | ERNIE (Wenxin Yiyan) | Knowledge graph support |
| **Zhipu AI** | GLM-4, GLM-3-Turbo | Open-source ecosystem |
| **Moonshot** | Kimi | Ultra-long context |
| **MiniMax** | abab6.5 | Multimodal generation |
| **DeepSeek** | DeepSeek Chat | High cost-performance |

:::

::: details Private deployments

Open-source models deployed locally or on private clouds:

- **Llama** series
- **Qwen** series
- **ChatGLM** series
- **Mistral** series
- Other models compatible with the OpenAI API format

:::

### Submit and Save

Click **Submit** to finish creating the channel. The new channel appears in the list.

## Multi-Key Mode

Multi-Key mode lets one channel configure multiple API Keys. The system rotates through them automatically, skips a Key after failures and re-enables it once it recovers.

### Configuring Multiple Keys

1. Click the **Edit** button next to the target channel in the list
2. Find the **Multi-Key Management** section in the edit dialog

![Multi-key configuration area](/imgs/channel-multikey.png)

3. Click **Add Key** to enter multiple API Keys one by one

### Selecting a Rotation Mode

| Rotation mode | Description |
| --- | --- |
| **Round Robin** | Uses each Key in order |
| **Weighted random** | Selects a Key randomly by weight |

::: tip Multi-Key best practices
- Configure 2-3 Keys to ensure availability
- Check each Key's usage and balance regularly
- Keys can come from different accounts to spread risk
:::

### Save Configuration

Click **Save** to apply the configuration.

## Channel Status

| Status | Icon | Description | Recommended action |
|------|:----:|------|----------|
| **Enabled** | 🟢 | Healthy, processing requests | None |
| **Disabled** | 🔴 | Manually disabled, not processing requests | Enable as needed |
| **Auto-disabled** | 🟡 | Automatically disabled after consecutive failures | Check Key validity or balance |

::: warning Auto-disable mechanism
When a channel fails consecutively and reaches the threshold, the system disables it automatically. Common causes:

- API Key expired or invalid
- Insufficient account balance
- Upstream service outage
- Request rate limit exceeded

An auto-disabled channel must be re-enabled manually, or it may recover automatically once the issue is fixed (depending on configuration).
:::

## Channel Testing

### Testing a Single Channel

1. Find the target channel in the list and click the **Test** button in its action column
2. Wait for the test request to complete — a dialog shows the response time and success/failure status

![Channel test result dialog](/imgs/channel-test.png)

A shorter response time means a faster channel. After a successful test, the response time is shown in the channel list so you can monitor channel performance.

### Batch Testing

Click **Test All Channels** at the top of the list to test everything at once.

::: tip Testing recommendations
- Test a new channel immediately after creation
- Test channels regularly to catch issues early
- Watch response time trends to evaluate service quality
:::

## Access Control

Channels support access control per user group:

| User group | Description |
|--------|------|
| **default** | Default group; all users can access |
| **vip** | VIP group; only VIP users can access |
| **Custom groups** | Groups created by admins |

::: info How access control works
When a user requests a model, the system filters available channels by the user's group. A channel is only considered for routing when its configured groups match the user's group.
:::

## Model Mapping

Model mapping lets you map the model name requested by the user to the model actually used:

```json
{
  "gpt-4": "gpt-4-turbo",
  "gpt-3.5-turbo": "gpt-3.5-turbo-16k"
}
```

**Common uses**:
- Keep compatibility after model upgrades (`gpt-4` → `gpt-4-turbo`)
- Unify model naming across providers
- Provide simplified model names to users

See [Advanced Settings - Model Mapping](./advanced#model-mapping)

## How-To Guide

### Create a Channel

1. Click the **Add Channel** button in the top-right corner of the page
2. Select the channel type
3. Fill in the required information:
   - Channel name
   - API Key
   - Supported models
4. Configure optional parameters:
   - User group permissions
   - Base URL (needed for proxies or private deployments)
5. Click **Submit** to save

### Edit a Channel

1. Find the target channel in the list
2. Click the **Edit** button
3. Modify the configuration that needs updating
4. Click **Save** to confirm

### Batch Operations

Select multiple channels and perform batch operations:

1. Check the checkboxes of multiple channels on the left side of the list
2. A batch operation toolbar appears at the top of the page

![Batch operation bar after selecting channels](/imgs/channel-batch.png)

3. Click the corresponding button to execute:

| Operation | Description |
|------|------|
| **Batch enable** | Enable multiple channels at once |
| **Batch disable** | Disable multiple channels at once |
| **Batch tag** | Apply a unified tag to selected channels for classification |
| **Batch delete** | Delete selected channels (irreversible) |

::: danger Batch delete warning
Batch delete is irreversible — be careful. It is recommended to disable first, observe, then delete once confirmed.
:::

## Notes

::: warning API Key security
- Never share API Keys publicly
- Rotate API Keys regularly
- Store Keys using environment variables or a secrets manager
:::

::: tip Operations tips
- **Balance monitoring**: check channel balances regularly to avoid service interruptions
- **Response time**: watch for response time changes to catch performance issues early
- **Multi-channel backup**: configure multiple channels for critical models
- **Tagging**: use tags to classify channels when managing many of them
:::

---

**Next step**: learn about [Channel Advanced Settings](./advanced) — model mapping, weight configuration, priority and parameter overrides.
