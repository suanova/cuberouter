# Quick Start

> This guide walks you through CubeRouter from account registration to your first API call — just a few minutes.

## Step 1: Register and Sign In

Visit the CubeRouter home page and click the **Get API Key** button (or the "No account? Sign up" link on the login page) to open the registration page.

![Register page](/imgs-en/register.jpeg)

**Registration steps**:

1. Fill in **Username**: English is recommended; usernames cannot be changed after registration
2. Fill in **Email**: used to receive verification codes (required)
3. Fill in **Password**: 8–20 characters; letters and digits are recommended
4. Click **Create account** to complete registration

::: tip Registration tips
- Usernames cannot be changed after registration — choose one carefully
- Passwords are 8–20 characters; we recommend storing yours in a password manager
- If the platform has third-party sign-in configured (GitHub, Discord, OIDC, etc.), you can also register and sign in with one click
:::

After a successful registration you are redirected to the login page — enter your username and password to sign in.

![Login page](/imgs-en/login.jpeg)

Once signed in, you land in the console (the Overview page).

## Step 2: Create an API Key

Create your first key on the **API Keys** page. Click **API Keys** in the left navigation, or go directly to `/keys`.

![Create API Key](/imgs-en/token-create.jpeg)

**Steps**:

1. Click the **Create API Key** button
2. Fill in the key **Name**: name it by purpose (e.g. `default`, `prod`, `dev-test`)
3. Select a **Group**: different groups can access different models
4. Set **Expires at**: choose "never expire" or a specific validity period
5. Set **Count**: create multiple keys at once (default is 1)
6. Expand **Advanced Settings** to configure model restrictions, IP whitelist, and more
7. Enable **Unlimited Quota** to bypass quota limits; otherwise set the maximum quota available to this key
8. Click **Create** and **copy & save the key immediately**

::: warning Important
The full API key is displayed only once at creation — copy and save it immediately. The key grants full API access: never share it with others, and never commit it to a code repository. Store it in an environment variable or a config file.
:::

After creation, you can view and manage keys in the list:

![API key list](/imgs-en/token-list.jpeg)

## Step 3: Add Quota

Your account needs available quota before using the API. Click **Wallet** in the left navigation, or go to `/wallet`, and top up via online payment or a redemption code. See [Quota & Top-Up](../guide/wallet.md).

## Step 4: Browse Models

Click **Model Square** in the top navigation bar to view all available models and their prices.

![Model Square](/imgs-en/models-market.jpeg)

Use the **Group** filter on the left to view the models available to your API key's group, and copy the model name to use in API calls. See [Model Square](../guide/models-market.md).

## Step 5: Make Your First Call

CubeRouter supports the OpenAI-compatible API. Use the platform URL as `base_url` together with your API key to start calling:

```bash
curl https://your-platform.com/v1/chat/completions \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "your-model-id",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

The API Base URL is the current service domain. For more code examples (Python / Claude / Gemini), see [Use the API](#use-the-api).

## Step 6: Pick a Client Tool

CubeRouter supports the OpenAI-compatible API and works with a wide range of tools:

- [Claude Code](./claude-code.md)
- [OpenCode](./opencode.md)
- [OpenClaw](./openclaw.md)

## Use the API

### Test online in the Playground

The [Playground](../guide/playground.md) is a built-in online testing tool — chat with models directly without writing any code, which is perfect for a quick check that your key works.

### Get the API Base URL


Use the current domain as `base_url` in your client or code, and start calling with your key.

### Code examples

**Python (OpenAI SDK)**:

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-xxxxxxxxxxxxxxxx",  # your platform-issued API key
    base_url="https://your-platform.com/v1"
)

response = client.chat.completions.create(
    model="your-model-id",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.choices[0].message.content)
```

**Claude native format**:

```bash
curl https://your-platform.com/v1/messages \
  -H "x-api-key: sk-xxxxxxxx" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model": "claude-3-5-sonnet-20241022", "max_tokens": 1024, "messages": [{"role": "user", "content": "Hello"}]}'
```

**Gemini native format**:

```bash
curl "https://your-platform.com/v1beta/models/gemini-1.5-pro:generateContent?key=sk-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{"contents": [{"parts": [{"text": "Hello"}]}]}'
```

### Supported endpoints

| Endpoint | Path | Description |
| --- | --- | --- |
| Chat completions | `POST /v1/chat/completions` | Conversational generation, streaming supported |
| Text completions | `POST /v1/completions` | Legacy completions endpoint |
| Embeddings | `POST /v1/embeddings` | Text vectorization |
| Image generation | `POST /v1/images/generations` | Text-to-image |
| Image editing | `POST /v1/images/edits` | Image editing |
| Speech-to-text | `POST /v1/audio/transcriptions` | Whisper, etc. |
| Text-to-speech | `POST /v1/audio/speech` | TTS |
| Reranking | `POST /v1/rerank` | Document reranking |
| Responses API | `POST /v1/responses` | OpenAI Responses format |
| Realtime | `GET /v1/realtime` (WebSocket) | OpenAI Realtime API |
| Model list | `GET /v1/models` | Query available models |
