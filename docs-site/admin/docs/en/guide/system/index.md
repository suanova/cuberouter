# System Settings

System Settings configures all platform-level parameters of CubeRouter, including operation parameters, billing ratios, payment configuration, performance monitoring and security policies. Correct system configuration is the foundation of a stable platform.

Log in with a Root account, click **Settings** in the left navigation, or visit `/console/setting`.

::: warning Permission requirement
System Settings is only accessible to **Super Admin (Root)** — regular admins cannot modify it. Understand the impact before changing any configuration.
:::

## Quick Navigation

| Category | Description |
|------|------|
| [General Settings](#general-settings) | Site name, announcements, docs URL |
| [Billing & Ratios](#billing--ratios) | Model ratios, completion ratios, group ratios |
| [Registration & Security](#registration--security) | Open registration, email whitelist, captcha |
| [OAuth Configuration](#oauth-configuration) | Third-party login and custom OAuth |
| [Mail Server](#mail-server) | SMTP mail service |
| [Worker Configuration](#worker-configuration) | Async task concurrency |
| [Payment Settings](#payment-settings) | EPay, Stripe and top-up configuration |
| [Rate Limits](#rate-limits) | Global and per-group limits |
| [Chat Settings](#chat-settings) | Built-in chat |
| [Drawing Settings](#drawing-settings) | Midjourney and other drawing features |
| [Dashboard Settings](#dashboard-settings) | Dashboard display and statistics |
| [Model Settings](#model-settings) | Model display, behavior and sync |
| [Operation Settings](#operation-settings) | Initial quota, invite rewards, top-up |
| [Performance Monitoring](#performance-monitoring) | Real-time server metrics and maintenance |
| [Other Settings](#other-settings) | SSRF protection, Passkey, docs & about page |

## Overview

![System settings tab overview](/imgs/setting-tabs.png)

The top of the system settings page has multiple tabs — click a tab to switch to the corresponding configuration area:

![System settings - Registration](/imgs/system-setting-3.png)

![System settings - Security](/imgs/system-setting-4.png)

| Category | Main configuration |
|------|----------|
| General settings | Site name, announcements, docs URL, top-up link |
| Billing/ratios | Model ratios, completion ratios, group ratios |
| Payment settings | Payment integration and top-up |
| Rate limits | Request limiting policies |
| Chat/drawing/dashboard/model | Feature toggles and defaults |
| Operation settings | Server, login, registration, quota |
| Other settings | Logs, security, notifications |

---

## General Settings

![General settings tab](/imgs/setting-general.png)

The following options are available:

- **Site name**: shown in the browser tab and at the top of pages
- **Homepage announcement**: announcement text on the homepage, Markdown supported
- **Docs URL**: when filled, a **Docs** button appears in the left navigation of the homepage and jumps to this address
- **Top-up link**: custom redirect address for the top-up page

::: info Docs button
When the docs URL is left empty, the **Docs** button is hidden. See [Other Settings - Docs & About Page](#docs--about-page) for the about page.
:::

---

## Billing & Ratios

The ratio system is the core of CubeRouter billing. By setting different ratios, you can control billing standards for various models and user groups flexibly.

### Ratio System Overview

CubeRouter uses a three-layer ratio system to calculate quota consumption:

1. **ModelRatio** - defines the base billing multiplier of each AI model
2. **CompletionRatio** - additional billing adjustment for output tokens
3. **GroupRatio** - differentiated billing multipliers per user group

### Quota and Ratios

Ratios are the key parameter in calculating quota consumption. Quota is the internal billing unit — every API call is ultimately converted into quota points and deducted.

**Quota unit conversion:**

- 1 USD = 500,000 quota points
- Quota points are the base unit of internal billing
- User balances and consumption records are all measured in quota points

### Quota Calculation Formulas

#### Per-Token Models (Token-Based)

```text
Quota consumed = (input tokens + output tokens × completion ratio) × model ratio × group ratio
```

#### Per-Request Models (Fixed Price)

```text
Quota consumed = model fixed price × group ratio × quota unit (500,000)
```

#### Audio Models (handled internally)

```text
Quota consumed = (text input tokens + text output tokens × completion ratio + audio input tokens × audio ratio + audio output tokens × audio ratio × audio completion ratio) × model ratio × group ratio
```

#### Pre-Billing and Post-Billing

The platform bills in two phases:

1. **Pre-billing**: before the API call, quota is pre-deducted based on estimated tokens
2. **Post-billing**: after the call, quota is recalculated based on actual tokens
3. **Settlement**: if actual consumption differs from the pre-deduction, the balance is adjusted automatically

```text
Pre-deducted quota = estimated tokens × model ratio × group ratio
Actual quota = actual tokens × model ratio × group ratio
Adjustment = actual quota - pre-deducted quota
```

### Model Ratio Settings

![Billing and ratio tab](/imgs/setting-ratio.png)

Default ratios are pre-configured for common models:

| Model | Model ratio | Completion ratio | Official price (input) | Official price (output) |
| ------------- | -------- | -------- | ---------------- | ---------------- |
| gpt-4o | 1.25 | 4 | $2.5/1M tokens | $10/1M tokens |
| gpt-3.5-turbo | 0.25 | 1.33 | $0.5/1M tokens | $1.5/1M tokens |
| gpt-4o-mini | 0.075 | 4 | $0.15/1M tokens | $0.6/1M tokens |
| o1 | 7.5 | 4 | $15/1M tokens | $60/1M tokens |

**What ratios mean:**

- Model ratio: the multiplier relative to the base billing unit, reflecting cost differences between models
- Completion ratio: the multiplier of output tokens relative to input tokens, reflecting output cost differences
- Higher ratios consume more quota; lower ratios consume less

**How to set:**

1. Click the **Ratios** tab on the system settings page
2. Find the target model in the model ratio list

![Model ratio settings - page 1](/imgs/rate-setting-1.png)

3. Modify the following parameters:
   - **Input ratio**: billing ratio for input tokens
   - **Output ratio**: billing ratio for output tokens
   - **Completion ratio**: billing ratio for completion calls

![Model ratio settings - page 2](/imgs/rate-setting-2.png)

4. Click **Save**

![Model ratio settings - page 3](/imgs/rate-setting-3.png)

**Setting methods:**
1. JSON editing: edit the model ratio JSON directly
2. Visual editor: configure ratios through the graphical interface

### Completion Ratio Settings

The completion ratio adds an extra charge for output tokens, mainly to balance input/output cost differences between models.

| Model | Official price (input) | Official price (output) | Completion ratio | Description |
| ------------- | ---------------- | ---------------- | -------- | --------------- |
| gpt-4o | $2.5/1M tokens | $10/1M tokens | 4 | Output is 4x input |
| gpt-3.5-turbo | $0.5/1M tokens | $1.5/1M tokens | 1.33 | Output tokens billed at 1.33x the input rate |
| gpt-image-1 | $5/1M tokens | $40/1M tokens | 8 | Output is 8x input |
| gpt-4o-mini | $0.15/1M tokens | $0.6/1M tokens | 4 | Output is 4x input |
| Other models | 1 | 1 | 1 | Output is 1x input |

**Notes:**

- The completion ratio mainly affects output token billing
- 1 means output tokens are billed the same as input tokens
- Greater than 1 means output tokens cost more; less than 1 means they cost less

### Group Ratio Settings

Group ratios set differentiated billing multipliers per user group for flexible pricing.

#### Configuration

```json
{
  "vip": 0.5,
  "premium": 0.8,
  "standard": 1.0,
  "trial": 2.0
}
```

#### Precedence

1. Per-user ratio: a personal ratio set for a specific user
2. Group ratio: the ratio of the user's group
3. Default ratio: the system default (usually 1.0)

![Group ratio settings - page 4](/imgs/rate-setting-4.png)

Steps:

1. Find the **Group Ratios** area on the ratios page
2. Select the target group
3. Set the group's global ratio factor (e.g. 0.8 means 20% off)

![Group ratio settings - page 5](/imgs/rate-setting-5.png)

4. Click **Save**

Group ratios multiply with model ratios:

```text
Final cost = Token count × model ratio × group ratio
```

### Visual Ratio Editor

The visual editor provides an intuitive ratio management interface:

- Batch-edit model ratios
- Live preview of ratio configuration
- Conflict detection and warnings
- One-click sync of upstream ratios

### Models Without Ratios

For models without a configured ratio, the system:

1. Self-hosted mode: uses the default ratio of 37.5
2. Commercial mode: shows a "ratio or price not configured" error
3. Auto-detection: highlights unconfigured models in the admin UI

### Upstream Ratio Sync

The system can sync ratios from upstream channels automatically:

- Fetch upstream model ratios automatically
- Update local ratio configuration in batches
- Stay in sync with upstream pricing
- Manual adjustment and override supported

### FAQ

#### Q: How do I set a ratio for a new model?

A: Use the visual editor to add the model, or add it to the JSON configuration directly. Start with a conservative ratio and adjust based on actual usage.

#### Q: How do group ratios take effect?

A: The group ratio multiplies with the model ratio and affects the final quota consumption. The user's actual ratio = model ratio × group ratio.

#### Q: What is the completion ratio for?

A: It balances input/output token cost differences. Some models have much higher output costs than input costs, which the completion ratio compensates for.

#### Q: How do I batch-set ratios for similar models?

A: Use the visual editor's batch operations, or add similar models to the JSON configuration in bulk.

### Calculation Examples

#### Example 1: GPT-4, standard user conversation

Scenario:

- Input tokens: 1,000
- Output tokens: 500
- Model ratio: 15
- Completion ratio: 2
- Group ratio: 1.0 (standard user)

Calculation:

```
Quota = (1,000 + 500 × 2) × 15 × 1.0
      = (1,000 + 1,000) × 15
      = 2,000 × 15
      = 30,000 quota points
```

Equivalent USD cost: 30,000 ÷ 500,000 = $0.06

#### Example 2: GPT-3.5, VIP user conversation

Scenario:

- Input tokens: 2,000
- Output tokens: 1,000
- Model ratio: 0.25
- Completion ratio: 1.33
- Group ratio: 0.5 (VIP 50% discount)

Calculation:

```
Quota = (2,000 + 1,000 × 1.33) × 0.25 × 0.5
      = (2,000 + 1,330) × 0.125
      = 3,330 × 0.125
      = 416.25 quota points
```

Equivalent USD cost: 416.25 ÷ 500,000 = $0.00083

#### Example 3: Per-request model (e.g. Midjourney)

Scenario:

- Model fixed price: $0.02
- Group ratio: 1.0 (standard user)
- Quota unit: 500,000

Calculation:

```
Quota = 0.02 × 1.0 × 500,000
      = 10,000 quota points
```

Equivalent USD cost: 10,000 ÷ 500,000 = $0.02

---

## Registration & Security

![Registration and security tab](/imgs/setting-security.png)

The following options are available:

- **Open registration**: toggle whether new users can register on their own. Internal systems should close it and let admins create accounts
- **Email whitelist**: restrict registration to specific email domains
- **Email verification**: whether registration requires email verification; recommended for public services
- **Turnstile verification**: enter Cloudflare Turnstile Site Key and Secret Key to enable human verification

::: warning Registration security
- Enable email verification on public services to prevent malicious registrations
- Combine with the invite code system to control registration sources
:::

---

## OAuth Configuration

### Built-in OAuth Providers

![OAuth settings tab](/imgs/setting-oauth.png)

Fill in the Client ID and Client Secret for each third-party login platform you want to enable:

| Provider | Description |
|--------|------|
| **GitHub** | GitHub account login |
| **Discord** | Discord account login |
| **WeChat** | WeChat QR-code login |
| **Telegram** | Telegram account login |
| **OIDC** | Generic OpenID Connect identity providers |

Click **Save** and the corresponding third-party login buttons appear on the login page.

::: details OAuth setup (GitHub example)
1. Visit GitHub → Settings → Developer settings → OAuth Apps
2. Create a new app, enter the app name and callback address
3. Get the Client ID and Client Secret
4. Enter them in System Settings

**Callback address format**:
```
https://your-domain.com/api/oauth/github/callback
```
:::

### Important OIDC Note

::: warning OIDC registration
When OIDC is enabled, make sure to check **Allow new user registration** — otherwise new users logging in via OIDC cannot be created. Also check **Allow login via OIDC**. Enable other options as needed.
:::

### Custom OAuth Providers

Beyond the built-in providers, Root can add any custom login method that conforms to the OIDC standard.

![Custom OAuth provider list](/imgs/custom-oauth-list.png)

**Adding a custom OAuth provider:**

1. Click **Add Provider** to open the configuration dialog

![Add custom OAuth dialog](/imgs/custom-oauth-add.png)

2. Enter the OIDC provider's Discovery URL in the **Discovery URL** field (usually ending with `/.well-known/openid-configuration`) and click **Auto Discover** — the system fills in the Authorization Endpoint, Token Endpoint and other fields automatically

![Fields after auto-discovery](/imgs/custom-oauth-filled.png)

3. Enter the Client ID and Client Secret (from the OAuth provider's app management page)
4. Enter a display name (the button text users see on the login page)
5. Click **Save**; the provider's login entry appears on the login page

::: tip OIDC vs OAuth
OIDC is an extension of OAuth 2.0 that provides a standardized way to fetch user info. If your identity provider supports OIDC, prefer it over custom OAuth configuration.
:::

---

## Mail Server

Configure the mail server for sending verification codes, notifications and other emails.

![System settings - Mail server](/imgs/system-setting-2.png)

1. Find the **Mail Server** area on the system settings page
2. Fill in the following information:
   - **SMTP server address**: e.g. smtp.gmail.com
   - **SMTP port**: usually 465 (SSL) or 587 (TLS)
   - **Sender email**: the email address used for sending
   - **Sender name**: the name shown as the sender
   - **SMTP username**: usually the same as the sender email
   - **SMTP password**: an app-specific password or authorization code
3. Click **Test Email** to verify the configuration
4. Click **Save**

::: info Gmail app-specific passwords
Gmail and similar providers require app-specific passwords rather than your account password. Generate one in your email account settings.
:::

---

## Worker Configuration

Configures Worker parameters for processing asynchronous tasks.

![System settings - Worker configuration](/imgs/system-setting-1.png)

Workers handle asynchronous tasks such as:

- Email sending
- Statistics
- Scheduled jobs

Options:

- **Worker count**: number of concurrent task workers
- **Task queue size**: capacity of the pending task queue

::: info Worker count recommendation
Set the worker count based on server performance — typically 2-4x the CPU core count.
:::

---

## Payment Settings

Configures the payment methods and parameters supported by the platform.

![Payment settings page](/imgs/payment-setting.png)

### What is Yipay (Easy Pay)

`Yipay` is a generic term for the "third-party aggregated payment gateway" pattern — not a specific website or company. It can refer to commercial aggregated payment services or self-hosted/open-source gateways that follow the Yipay protocol style.

**Core role:** aggregate WeChat Pay, Alipay, bank cards and other channels, providing merchants with unified order creation, signature verification and callback interfaces.

::: warning Compliance notice
A gateway is not equivalent to a licensed payment institution; fund settlement and compliance depend on the licensed channels it connects to. Follow local regulatory and risk-control requirements.
:::

### EPay Configuration

EPay is a domestic aggregated payment platform supporting Alipay and WeChat Pay.

1. Click the **Payment** tab on the system settings page
2. Find the **EPay** area
3. Fill in the following:
   - **API address**: the interface address provided by EPay
   - **Merchant ID (PID)**: from the EPay dashboard
   - **Merchant key (KEY)**: from the EPay dashboard
4. Check **Enable EPay**
5. Click **Save**

Callbacks include a signature that the system validates before crediting automatically.

### Stripe Configuration

Stripe is an international credit card payment platform.

![Stripe configuration](/imgs/stripe.png)

1. Find the **Stripe** area on the payment settings page
2. Fill in the following:
   - **API key (Secret Key)**: from the Stripe dashboard
   - **Publishable Key**: from the Stripe dashboard
   - **Webhook signing secret**: obtained after configuring the webhook
   - **Product price ID**: the Stripe product's price ID
3. Check **Enable Stripe**
4. Click **Save**

::: info Stripe webhook
Stripe requires a webhook to receive payment status notifications: `https://your-domain.com/api/payment/stripe/webhook`
:::

### Other Payment Methods

The platform also supports the following payment methods, configured similarly:

- **Creem**: international payment platform
- **Waffo**: international payment platform

### Top-Up Methods

The **Top-Up Methods** field supports the following structure:

```json
[
  {
    "color": "rgba(var(--semi-blue-5), 1)",
    "name": "Alipay",
    "type": "alipay"
  },
  {
    "color": "rgba(var(--semi-green-5), 1)",
    "name": "WeChat",
    "type": "wxpay"
  },
  {
    "color": "rgba(var(--semi-green-5), 1)",
    "name": "Stripe",
    "type": "stripe",
    "min_topup": "50"
  },
  {
    "name": "Custom 1",
    "color": "black",
    "type": "custom1",
    "min_topup": "50"
  }
]
```

**Field reference**

- **name**: display text on the "select payment method" buttons (e.g. "Alipay/WeChat/Stripe/Custom 1")
- **color**: button/badge theme or border color. Any CSS color value is accepted; design tokens like `rgba(var(--semi-blue-5), 1)` are recommended
- **type**: channel identifier for backend routing and order creation
  - `stripe` → uses the Stripe gateway
  - Others (e.g. `alipay`, `wxpay`, `custom1`) → use the Yipay-style gateway and pass the value through as a channel parameter
- **min_topup**: minimum top-up amount (same currency as the page). When the entered amount is below this value, the page shows "The minimum top-up amount for this method is X" and blocks payment; the backend validates too
- **Order**: rendered left to right in array order

### Top-Up Amounts

#### Custom Top-Up Options

Set the selectable top-up amounts, e.g.:

```json
[10, 20, 50, 100, 200, 500]
```

These values appear in the "select top-up amount" area; users can click one directly.

#### Top-Up Discounts

Set discounts per top-up amount — key is the amount, value is the discount rate:

```json
{
  "100": 0.95,
  "200": 0.9,
  "500": 0.85
}
```

**Explanation:**

- **Key**: top-up amount (string)
- **Value**: discount rate (a decimal between 0 and 1; 0.95 means 95% of the price, i.e. 5% off)
- The system calculates the actual amount and savings automatically

::: info Top-up discounts
Discounts encourage users to top up more at once, improving retention.
:::

---

## Rate Limits

Configures API request frequency limits to prevent abuse and protect system stability.

![Rate limit settings page](/imgs/rate-limit-setting.png)

### Global Limits

1. Click the **Rate Limit** tab on the system settings page
2. Configure the global limit parameters:
   - **Requests per minute**: max requests per IP per minute
   - **Requests per hour**: max requests per IP per hour
   - **Requests per day**: max requests per IP per day
3. Click **Save**

### Per-Group Limits

Different user groups can have different limit policies:

1. Edit the group on the group management page
2. Set the group's rate limit parameters
3. All users in the group share this configuration

#### Per-Group Rate Limit Example

```json
{
  "default": [200, 100],
  "vip": [0, 1000]
}
```

**Explanation:**

- **Key**: group name
- **Value**: array of two numbers
  - First: requests per minute
  - Second: requests per hour
  - 0 means unlimited

**Example interpretation:**

- `default` group: max 200 req/min and 100 req/hour
- `vip` group: unlimited per minute, max 1000 req/hour

### Rate Limit Response

When a user exceeds the limit:

```json
{
  "error": {
    "message": "Rate limit exceeded. Please retry after 60 seconds.",
    "type": "rate_limit_error",
    "code": "rate_limit_exceeded"
  }
}
```

::: warning Rate limit recommendations
- Set lower limits for new users to prevent abuse
- VIP users can have higher limits
- Overly low limits can affect normal usage — configure based on actual business needs
:::

---

## Chat Settings

Configures the built-in chat feature.

![Chat settings page](/imgs/chat-setting.png)

### Chat App Configuration

1. Click the **Chat** tab on the system settings page
2. Configure the following options:
   - **Enable chat**: toggle the built-in chat
   - **Default model**: the model preselected on the chat page
   - **Max history messages**: conversation rounds retained
   - **Streaming output**: whether streaming is enabled by default
3. Click **Save**

::: info Notes
- **Max history messages**: affects context length and token consumption; more history costs more
- **Streaming output**: when enabled, users see content generated in real time
:::

### Chat Integration Variables

The following variables are available when configuring chat app integration:

- **`{key}`**: replaced with the API Key
- **`{address}`**: replaced with the server address (no trailing `/` or `/v1`)

**Example:**

Template:
```
https://{address}/v1
```

After replacement:
```
https://api.example.com/v1
```

::: info Automatic replacement
These variables are replaced automatically when importing configuration into chat apps with one click.
:::

### Chat App Integration

Integration parameters for third-party chat apps:

- **ChatGPT Next Web**: deployment address
- **Lobe Chat**: recommended settings
- **Other apps**: integration parameters

---

## Drawing Settings

Configures drawing features such as Midjourney.

![Drawing settings page](/imgs/drawing-setting.png)

### Midjourney Configuration

1. Click the **Drawing** tab on the system settings page
2. Configure Midjourney parameters:
   - **Enable Midjourney**: toggle the drawing feature
   - **Midjourney Proxy address**: the Midjourney-Proxy service address
   - **API key**: the Midjourney-Proxy key
   - **Timeout**: drawing task timeout (seconds)
3. Click **Save**

::: info Prerequisites
- Midjourney requires a separately deployed Midjourney-Proxy service
- Configure a drawing channel in Channel Management
- Add drawing models in Model Management
:::

### Drawing Billing

Billing rules for drawing tasks:

- **Per-request**: fixed quota per drawing
- **By duration**: billed by drawing time
- **By resolution**: billed by image resolution

---

## Dashboard Settings

Configures the dashboard display and statistics dimensions.

### Basic Settings

![Dashboard settings - page 1](/imgs/dashboard-setting-1.png)

1. Click the **Dashboard** tab on the system settings page
2. Configure display options:
   - **Show user statistics**: show user count statistics
   - **Show channel statistics**: show channel usage statistics
   - **Show model statistics**: show model call statistics

### Chart Settings

![Dashboard settings - page 2](/imgs/dashboard-setting-2.png)

3. Configure chart parameters:
   - **Default time range**: the default range shown on the dashboard
   - **Refresh interval**: automatic refresh interval
   - **Chart type**: line, bar or pie

### Advanced Options

![Dashboard settings - page 3](/imgs/dashboard-setting-3.png)

4. Configure advanced options:
   - **Data cache time**: statistics cache duration
   - **Show real-time data**: whether to show live statistics
5. Click **Save**

---

## Model Settings

Configures model display and behavior parameters.

### Display Settings

![Model settings - page 1](/imgs/model-setting-1.png)

1. Click the **Model** tab on the system settings page
2. Configure display options:
   - **Show model descriptions**: show descriptions in the model list
   - **Show model icons**: show vendor icons
   - **Group models by**: group by vendor or type

### Behavior Settings

![Model settings - page 2](/imgs/model-setting-2.png)

3. Configure behavior:
   - **Auto-disable failing models**: disable after consecutive failures
   - **Failure threshold**: failures that trigger auto-disable
   - **Auto-recovery time**: minutes until automatic recovery (minutes)

### Sync Settings

![Model settings - page 3](/imgs/model-setting-3.png)

4. Configure synchronization:
   - **Auto-sync upstream models**: periodically sync the latest model list
   - **Sync interval**: hours between automatic syncs
   - **Preserve custom config on sync**: don't overwrite manual modifications
5. Click **Save**

---

## Operation Settings

Configures platform operation parameters.

### Basic Operations

![Operation settings - page 1](/imgs/operation-1.png)

1. Click the **Operation** tab on the system settings page
2. Configure operation parameters:
   - **New user initial quota**: quota for newly registered users
   - **Invite reward quota**: reward quota for the inviter after a successful invitation
   - **Rebate ratio**: share (%) of top-ups by invited users that the inviter earns

### Top-Up Configuration

![Operation settings - page 2](/imgs/operation-2.png)

3. Configure top-up options:
   - **Minimum top-up amount**: minimum per transaction
   - **Top-up bonus ratio**: extra quota ratio granted on top-ups
   - **Top-up tiers**: preset top-up amount options

### Redemption Code Configuration

![Operation settings - page 3](/imgs/operation-3.png)

4. Configure redemption codes:
   - **Default validity**: default validity of codes (days)
   - **Per-user redemption limit**: how many times each user can redeem
5. Click **Save**

::: info Invite reward example

```text
Initial quota: 10,000
Inviter reward: 5,000
Invitee reward: 5,000

User A invites new user B:
- User A receives 5,000 quota
- User B registers with 10,000 + 5,000 = 15,000 quota
```

:::

---

## Performance Monitoring

Performance monitoring shows real-time server resource usage and provides system maintenance operations. Log in with a Root account and find the **Performance** tab in System Settings.

![Performance monitoring tab](/imgs/performance.png)

The page shows real-time CPU usage, memory usage, request volume and other metrics.

### Maintenance Operations

Find the operation buttons on the page and click one to execute immediately:

![Maintenance operation buttons](/imgs/performance-actions.png)

| Operation | Description | Use case |
| --- | --- | --- |
| Clear disk cache | Release disk cache space | When disk usage is high |
| Reset statistics | Zero the performance counters | When statistics need a fresh start |
| Force GC | Manually trigger Go garbage collection to free memory | When memory usage is abnormally high |
| Clean log files | Delete old log files to free disk space | When log files accumulate |

After clicking a button, the page reports the result and the metrics refresh.

---

## Other Settings

Other Settings contains homepage configuration, log export, SSRF protection, Passkey authentication, legal documents, automatic grouping and more.

### Homepage Configuration

![Other settings - page 1](/imgs/other-setting-1.png)

1. Click the **Other** tab on the system settings page
2. Configure homepage content:
   - **Homepage announcement**: announcement content (Markdown supported)
   - **Homepage background image**: background image URL
   - **Show statistics**: whether the homepage shows platform statistics

### Other Features

![Other settings - page 2](/imgs/other-setting-2.png)

3. Configure other features:
   - **Enable log export**: allow users to export their own usage logs
   - **Log retention days**: how many days of logs to keep before auto-cleaning
   - **Enable API docs**: whether to show the API docs entry
4. Click **Save**

### SSRF Protection

SSRF (Server-Side Request Forgery) protection defends the system against malicious requests.

| Setting | Description | Default |
|--------|------|--------|
| **Enable SSRF protection** | Whether SSRF protection is on | On |
| **Allow private IPs** | Whether private IPs are allowed | Off |
| **Domain filter mode** | `true` = whitelist, `false` = blacklist | Off |
| **IP filter mode** | `true` = whitelist, `false` = blacklist | Off |
| **Domain list** | Domain filters, wildcards like `*.example.com` supported | Empty |
| **IP list** | IP filters in CIDR format, e.g. `192.168.1.0/24` | Empty |
| **Allowed ports** | Ports allowed to connect | `80, 443, 8080, 8443` |
| **Apply IP filter to domains** | Also filter resolved IPs of domains (experimental) | Off |

::: warning SSRF notes
- With protection on, requests to private IPs are blocked
- Domain filters can be a whitelist (only listed domains) or blacklist (blocked domains)
- IP filters accept CIDR notation, e.g. `10.0.0.0/8`
:::

### Legal Documents

Configures the terms of service and privacy policy shown on the registration or settings pages.

| Setting | Description |
|--------|------|
| **Terms of service** | Content in Markdown format |
| **Privacy policy** | Content in Markdown format |

::: tip Recommendations
- Write legal documents in clear, plain language
- Update them regularly to stay compliant
- Include contact information in the documents
:::

### Passkey Settings

Passkey is a passwordless authentication method based on WebAuthn, supporting fingerprints, Face ID, hardware keys and more.

| Setting | Description | Default |
|--------|------|--------|
| **Enable Passkey** | Whether Passkey login is enabled | Off |
| **Display name** | Service name shown during authentication | CubeRouter |
| **RP ID** | Relying party identifier, usually the domain | Auto from server address |
| **Origins** | Allowed origin list | Auto from server address |
| **Allow insecure origins** | Whether HTTP origins are allowed (dev only) | Off |
| **User verification** | Verification level | `preferred` |
| **Authenticator attachment** | Authenticator preference | Empty |

::: details Passkey notes
**RP ID and origins**:
- RP ID is usually your domain (no scheme or port)
- Origins are a JSON array of full URLs
- Production must use HTTPS

**User verification**:
- `preferred`: prefer biometrics
- `required`: biometrics required
- `discouraged`: biometrics not preferred
:::

### Automatic Grouping

Controls the default group behavior for new users.

| Setting | Description | Default |
|--------|------|--------|
| **Use automatic grouping by default** | Whether new users are grouped automatically | Off |
| **Automatic group list** | Groups available for automatic assignment | `["default"]` |

::: info How it works
Automatic grouping assigns users to groups based on rules such as invite codes or registration sources. See [Group Management](../group/index) for details.
:::

### Docs & About Page

Root can customize both the **Docs** button in the left navigation and the **About** page in System Settings.

#### Configuring the Docs Link

1. Click the **General** tab on the system settings page
2. Find the **Docs URL** field

![Docs URL field](/imgs/setting-docs-url.png)

3. Enter the full URL of the docs site (e.g. `https://docs.example.com`)
4. Click **Save**
5. Back on the homepage, a **Docs** button appears in the left navigation and jumps to the address

![Docs button in the homepage navigation](/imgs/setting-docs-effect.png)

::: info When the button shows
When the docs URL is empty, the **Docs** button is hidden.
:::

#### Configuring the About Page

1. Click the **Other** tab on the system settings page
2. Find the **About** content editor

![About content editor](/imgs/setting-about-editor.png)

3. Enter Markdown-formatted content (headings, links, images, lists supported)
4. Click **Save**
5. Users who click **About** in the left navigation see the configured content

![About page rendering](/imgs/setting-about-effect.png)

---

## How-To Guide

### Modifying Configuration

1. Go to the corresponding settings tab
2. Find the item to change
3. Modify the value
4. Click **Save**
5. Follow the prompt to determine whether a service restart is needed

::: warning How changes take effect

| Configuration type | Effect |
|----------|----------|
| Operation settings | Immediate |
| Performance settings | Service restart required |
| Payment settings | Immediate |
| Security settings | Immediate |

:::

### Resetting Configuration

To restore default configuration:

1. Click the **Reset** button next to the item
2. Confirm the reset

### Environment Variables

Some sensitive configuration is best set via environment variables:

```bash
# Database
SQL_DSN=your-database-dsn

# Redis
REDIS_URL=redis://localhost:6379

# Stripe
STRIPE_SECRET_KEY=sk_xxx
STRIPE_WEBHOOK_SECRET=whsec_xxx

# GitHub OAuth
GITHUB_CLIENT_ID=xxx
GITHUB_CLIENT_SECRET=xxx
```

---

## Notes

::: danger Key security
- Keep API Keys and secrets safe
- Never commit secrets to the repository
- Rotate keys regularly
- Use environment variables instead of config files
:::

::: tip Configuration management
- Record the original value before changing anything
- Validate important changes in a test environment first
- Back up configuration regularly
- Test after every change
:::

::: info Service restart
The following changes require a restart:
- Server address and port
- Concurrent request count and connection pool size
- Redis configuration
- Log level
:::

---

**Related documents**: [Channel Management](../channel/index) | [Subscription Management](../subscription/index) | [User Management](../user/index) | [Group Management](../group/index)
