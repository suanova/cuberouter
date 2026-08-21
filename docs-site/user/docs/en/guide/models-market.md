# Model Square

> Model Square is CubeRouter's model showcase and pricing center — find your target model quickly and understand its pricing.

Click **Model Square** in the top navigation bar, or go directly to `/pricing`.

![Model Square](/imgs-en/models-market.jpeg)

## Feature Overview

- 📊 **Model browsing**: view all available AI models
- 🔍 **Smart filtering**: filter by group, provider, tags, billing type, and more
- 💰 **Transparent pricing**: clear pricing information for every model
- 📋 **Easy copying**: copy a model name with one click for API calls

## Filter Panel

The left filter panel offers multi-dimensional filtering to help you find your target model quickly. Click **Reset** to clear all filters and restore the default view.

- **Group**: filter by the group of your API key to see the models your current key can access
- **Provider**: filter by model provider (OpenAI, Claude, Gemini, etc.)
- **Model tags**: filter by tags (chat, code, image, etc.)
- **Pricing type**: pay-as-you-go / per-call pricing
- **Endpoint type**: filter by API endpoint type (Chat, Responses, Anthropic, Gemini, Rerank, Image, Embedding, Video, etc.)

## Model Display Area

### Search bar

Type a model name keyword in the search box (shortcut `⌘K`), and the list automatically shows matching models.

### Price display

The toolbar provides a `/1M` / `/1K` toggle to control whether prices are shown per million or per thousand tokens.

Each model displays the following information:

- **Model name**: the full model identifier (e.g. `deepseek-v4-flash`)
- **Price info** (pay-as-you-go models): input price and output price (e.g. `$0.1 / $0.1`)
- **Price info** (per-call models): a fixed price per call
- **Billing type tag**: pay-as-you-go / per-call pricing

### View toggle

- **Card view**: display models as cards (default)
- **Table view**: display models in a table

## Billing Type Explained

**Pay-as-you-go**:

- Billed by the actual number of tokens used
- Input tokens and output tokens have different prices
- Price = input tokens × input unit price + output tokens × output unit price

**Per-call pricing**:

- A fixed price per call, regardless of token count
- Price = number of calls × price per call

## Use Cases

### Find a specific model

1. Open Model Square
2. Type a model name keyword in the search box
3. View the list of matching models
4. Click the copy button to get the model name

### Check models available to your key

1. Open Model Square
2. Filter by **Group** to match the group of your API key
3. View the models accessible to that group
4. Copy the model name to use in API calls

### Compare model prices

1. Filter by provider or tag
2. Toggle the `/1M` / `/1K` price display
3. Compare input / output prices across models
4. Pick the model with the best value

## FAQ

### Q1: How do I quickly find the model I need?

Use the filters and search: first filter by tag or provider, then type a keyword in the search box to pinpoint your target model.

### Q2: What's the difference between pay-as-you-go and per-call pricing?

- **Pay-as-you-go**: billed by token count; price = input price + output price
- **Per-call pricing**: a fixed price per call, regardless of token count

### Q3: Why can't I see some models?

Possible reasons: insufficient permissions for your key's group, the model has been taken offline or is under maintenance, or your filters exclude it.

### Q4: How do I see detailed prices for a model?

Look at the model card: pay-as-you-go models show input / output prices, per-call models show the price per call, and you can toggle the `/1M` / `/1K` display unit.
