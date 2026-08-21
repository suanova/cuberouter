# Pricing Explained

> Understand CubeRouter's billing rules: pay-as-you-go and per-call billing, how quota is consumed, and how to reconcile your bill.

For the full model price list and filter browsing, see [Model Square](../guide/models-market.md).

## Billing unit: quota

The platform uses **quota** as its internal billing unit — all balances, spending, and bills are displayed in quota.

- Top-ups (online payment / redemption code / subscription plan / referral rewards) increase your account quota
- Every API call is converted into consumed quota according to the billing rules and deducted from your account balance

## Two billing types

| Type | Billing method | Applicable models |
| --- | --- | --- |
| **Pay-as-you-go** | Billed by the actual number of tokens used | Text models for chat, completions, embeddings, etc. |
| **Per-call billing** | A fixed price charged per call, unchanged by token count | Task models for image generation, video generation, TTS, etc. |

### Pay-as-you-go

- **Input price**: the unit price of input tokens (switchable between `/1M` or `/1K` display units in Model Square)
- **Output price**: the unit price of output tokens
- Actual consumption = input tokens × input unit price + output tokens × output unit price

Some models also support finer-grained pricing, shown as separate unit prices:

- **Cache hit (read)**: the unit price for tokens that hit the prompt cache is usually far lower than regular input
- **Cache creation**: the unit price for tokens written to the cache for the first time is usually slightly higher than regular input
- **Image / audio**: multimodal input and output tokens are billed at their own unit prices

::: tip Price display
Prices in Model Square are shown as USD unit prices (e.g. `$3 / 1M`); the final quota deducted is converted by the platform at a unified exchange rate. The actual deduction shown in the usage logs is what counts.
:::

### Per-call billing

Per-call models deduct a fixed quota per call, regardless of request length. For example:

- Image generation: priced "per generation"; generating multiple images accumulates by image count
- Video generation: priced by fixed duration or fixed specification
- Speech synthesis / recognition: priced per call or by duration

The exact specifications and prices follow the model's description in Model Square.

## Group ratios

The platform supports applying different billing ratios per **group**:

- Different groups may enjoy different discounts or ratios
- The group you select when creating an API key determines the billing ratio that applies to that key
- The actual consumption follows the deducted amount in the usage logs

## How to Reconcile Your Bill

| Where | What to check |
| --- | --- |
| **Usage Logs** | Input / output token counts and quota deducted for each call |
| **Task Logs** | Status and consumption of task-type calls (images, videos, etc.) |
| **Wallet** | Current account balance and top-up records |
| **Dashboard** | Usage statistics by model / user dimension |

If a deducted amount doesn't match your expectations, check the token breakdown of the call in the usage logs and contact the administrator to confirm the model pricing and group ratio configuration.

## FAQ

### Q1: Why do the token counts in the logs seem higher than the prices shown on the Model Square page?

Pay-as-you-go models may include cache tokens, multimodal tokens, etc., which are billed separately at their own unit prices. The "input / output" figures in the usage logs are aggregate values; you can view the breakdown in the log details.

### Q2: Do free calls still consume quota?

As long as a non-free model is called, quota is deducted according to the billing rules. The administrator can configure some models or groups as free, in which case the deduction is 0.

### Q3: What happens when the balance runs out?

When the balance is insufficient, API calls are rejected (a 401 / insufficient balance error is returned), and an in-progress streaming response may be interrupted. Please top up in time, or enable quota alerts under **Profile → Notification Settings**.
