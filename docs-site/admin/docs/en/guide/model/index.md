# Model Management

Model management is used to manage the metadata and pricing of all models on the platform, and to configure how models are displayed in the Model Square, including model info, visibility and group permissions. Log in with an admin account, click **Models** in the left navigation, or visit `/console/models`.

::: warning Important note
This page only affects **display in the Model Square** — it does not affect actual model invocation or routing.

To configure real model invocation behavior (API integration, load balancing, failover, etc.), go to [Channel Management](../channel/index).
:::

## Quick Navigation

| Action | Description |
|------|------|
| [Add / Edit Models](#add-edit-models) | Configure model metadata and pricing |
| [Sync Upstream Models](#sync-upstream-models) | Sync the latest model list from providers |
| [Display Permissions](#display-permissions) | Control Model Square visibility |
| [Billing Types](#billing-types) | Per-request and per-token billing |

## Overview

![Model list](/imgs/model-list.png)

The model list shows all configured models, including name, type, input/output pricing and vendor.

The model management page provides the following features:

| Feature | Description |
|------|------|
| **Model list** | View all configured models |
| **Add model** | Create a new model display entry |
| **Edit model** | Modify model display information |
| **Enable/disable** | Control visibility in the Model Square |
| **Batch operations** | Modify display configuration in bulk |

## Display Configuration vs. Actual Invocation

| Item | Model management (display) | Channel management (invocation) |
|--------|------------------|------------------|
| Model availability | Controls whether the Model Square shows it | Controls whether it can actually be called |
| User permissions | Controls who can see the model | Controls who can call the model |
| Model pricing | Price information shown to users | Actual basis for quota billing |
| Channel binding | Read-only display | Actual configuration |

::: tip Configuration precedence
Actual invocation follows **Channel Management**. Display configuration in the Model Square only affects the user interface.
:::

## Model Configuration

### Basic Information

| Field | Required | Description |
|------|:----:|------|
| **Model ID** | ✓ | Unique model identifier, e.g. `gpt-4-turbo` |
| **Model Name** | ✓ | Display name in the Model Square |
| **Vendor** | ✓ | Vendor category the model belongs to |
| **Model Icon** | | Icon shown in the interface |

::: info Model ID note
The model ID must match the model name configured in Channel Management, otherwise users can't actually call it after selecting it in the Model Square.
:::

### Read-Only Display Fields

The following information is read automatically from **Channel Management** and is display-only in this page:

| Field | Source | Description |
|------|------|------|
| **Bound channels** | Channel management | Shows which channels configure this model |
| **Available groups** | Channel management | Shows the group permissions configured on the channels |
| **Billing type** | Channel management | Shows how the model is billed |

::: warning How to change this information?
These fields are determined by Channel Management. To modify them:
1. Go to [Channel Management](../channel/index)
2. Find the corresponding channel
3. Modify the channel's model list or user group configuration
:::

### Display Permissions

| Field | Description |
|------|------|
| **Enabled groups** | Which groups can see this model in the Model Square |
| **Tags** | Classification tags for easier filtering |

**Enabled group options**:

| Group | Description |
|------|------|
| **default** | Visible to all users |
| **vip** | Visible only to VIP users |
| **Custom groups** | Visible to the specified groups |

::: tip Display permission vs. invocation permission
- **Display permission**: controls whether the Model Square shows the model
- **Invocation permission**: determined by the user group settings in Channel Management

Keep both consistent to avoid users seeing a model they can't call.
:::

### Pricing Display Configuration

| Field | Description |
|------|------|
| **Billing type** | The billing method description shown to users |
| **Input price** | Displayed input token price ratio |
| **Output price** | Displayed output token price ratio |
| **Max context** | Displayed maximum context length |

::: warning Pricing note
Prices configured here are only for **display in the Model Square**, helping users compare models.

Actual charges follow Channel Management configuration or the system-wide pricing settings.
:::

### Endpoint Configuration

Endpoint configuration defines the feature types a model supports, for frontend display:

```json
{
  "chat": "/v1/chat/completions",
  "embeddings": "/v1/embeddings",
  "completions": "/v1/completions"
}
```

## Billing Types

Models support two billing methods:

| Type | Description | Display effect |
|------|------|----------|
| **Per-request** | Fixed quota per request | Suitable for chat scenarios |
| **Per-token** | Quota based on token count | Suitable for long text processing |

### Price Ratios

Price ratios display the relative cost of different models:

| Model | Input price | Output price | Description |
|------|:--------:|:--------:|------|
| GPT-3.5 | 1 | 1 | Baseline model |
| GPT-4 | 30 | 60 | 30x baseline input |
| Claude 3 | 15 | 75 | 15x baseline input |

---

## Model Square Display

### Enable/Disable Models

| Status | Effect in Model Square |
|------|--------------|
| 🟢 **Enabled** | Model is visible |
| 🔴 **Disabled** | Model is hidden |

### Tags

Tags categorize models in the Model Square:

| Tag type | Examples |
|----------|------|
| **By capability** | `chat`, `code`, `drawing`, `embedding` |
| **By trait** | `long-context`, `multimodal`, `high-speed` |
| **By use** | `general`, `professional`, `enterprise` |

### Batch Enable

Model management supports batch operations:

1. Select multiple models
2. Choose a batch operation type
3. Set the target user group
4. Confirm

---

## Add / Edit Models

1. Click the **Add Model** button, or click **Edit** on an existing model

![Add/edit model dialog](/imgs/model-edit.png)

2. Fill in the model information:
   - Model ID (must match Channel Management)
   - Model name and vendor
   - Input price and output price
3. Configure display permissions (enabled groups, tags)
4. Click **Save** to finish

### Batch Updating Display Groups

1. Select the models to update
2. Click **Batch Update Groups**
3. Choose the target group
4. Confirm

---

## Sync Upstream Models

The sync feature fetches the latest model list from each provider, previews the changes, and applies them only after your confirmation.

1. Click **Sync Upstream** on the model management page
2. The system requests the provider's model list and shows a preview dialog

![Upstream model sync preview dialog](/imgs/model-sync-preview.png)

3. The preview lists **New**, **Changed** and **Deleted** models separately
4. After confirming, click **Apply** and the model list updates

![Model list after sync](/imgs/model-synced.png)

::: tip Sync recommendation
After configuring channels, use model sync to quickly create the Model Square display entries.
:::

---

## Configuration Reference

| Goal | Where to configure |
|----------------|----------|
| Control whether a model shows in the Model Square | Model management → Enable/disable |
| Control which users can see a model | Model management → Enabled groups |
| Control which users can call a model | Channel management → User groups |
| Configure a model's API integration | Channel management → New channel |
| Set model price/billing | Channel management / System settings |

---

## Notes

::: warning Model ID consistency
The model ID in Model Management must match the model name in Channel Management, otherwise:
- Users may not be able to call the model after selecting it in the Model Square
- Bound channel display information may not render correctly
:::

::: tip Display vs. invocation configuration
1. Configure model invocation in Channel Management first
2. Configure the display in Model Management next
3. Keep the user groups consistent on both sides
:::

::: info Read-only fields
Bound channels, available groups and billing type are read from Channel Management and are display-only. To modify them, go to Channel Management.
:::

---

**Related documents**: [Channel Management](../channel/index) | [System Settings](../system/index)
