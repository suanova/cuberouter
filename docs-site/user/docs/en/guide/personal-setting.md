# Profile

> Manage your account's basic info, security settings, and third-party account bindings. After signing in, click your avatar in the top-right corner and select **Profile** from the dropdown menu, or go directly to `/profile`.

![Profile](/imgs-en/profile.jpeg)

## Basic Information

The top of the page shows an account summary: username, user ID, current group, current balance, total usage, and API request count.

### Language Preferences

Switch the interface display language — supports Simplified Chinese, Traditional Chinese, English, Français, Русский, 日本語, Tiếng Việt, and more. The choice is saved automatically.

## Third-Party Account Bindings

After binding a third-party account such as GitHub or Discord to your current account, you can sign in directly with the third-party account without entering a password.

Supported platforms include: Email, Phone, GitHub, Discord, LinuxDO, OIDC, Telegram, WeChat, and custom OAuth.

### Bind a third-party account

1. Find the platform you want to bind in the **Account Bindings** area
2. Click the **Bind** button for the corresponding platform
3. The page redirects to the platform's authorization page; sign in and click **Authorize**
4. After authorization you're redirected back automatically and the binding status changes to **Bound**
5. For already-bound platforms, click **Unbind** to remove the binding

## Security Settings

### Change Password

Click **Change Password**, enter your current password, new password, and new password confirmation in order, then click **Confirm** to finish.

### Two-Factor Authentication (2FA)

Once two-factor authentication is enabled, every sign-in requires a dynamic code from an authenticator app in addition to your password — an effective protection against account takeover.

#### Enable 2FA

1. Install an authenticator app on your phone (Google Authenticator or Microsoft Authenticator recommended)
2. Find **Two-Factor Authentication** in the **Security** area and click **Enable**
3. Scan the QR code shown on the page with the authenticator app; a 6-digit dynamic code appears in the app
4. Enter the 6-digit code from the app into the verification field and click **Confirm**
5. The system shows a set of backup codes — take a screenshot or write them down immediately, for emergency sign-in when you can't use the authenticator

::: warning Important
Backup codes are only shown once and cannot be viewed again after the dialog is closed. If you lose your backup codes and can't use the authenticator, you'll need to contact an admin for a reset.
:::

### Passkey Passwordless Login

Passkeys let you sign in using a device fingerprint, facial recognition, or a hardware security key — no password required.

#### Register a Passkey

1. Find **Passkey Login** in the **Security** area
2. Click **Enable Passkey**
3. The browser shows a system verification prompt; complete the fingerprint, face, or security key verification as prompted
4. Once verified, your Passkey is registered and can be used directly at your next sign-in

### Login Sessions

Shows the active login sessions for your account, including device type, IP address, and more. You can:

- **Sign out**: sign out a specific session
- **Sign out other sessions**: sign out all sessions except the current one

## Notification Settings

Configure how you receive system notifications, with support for multiple delivery channels.

- **Notification email**: set the email that receives notifications; leave empty to use the account email
- **Notification method**: options include Webhook, Gotify, Bark Push, Telegram, and more
- **Quota alert threshold**: send a reminder when your balance falls below the set value
- **Upstream model update notifications**: receive notifications when upstream models are updated
- **Message priority**: set the priority of pushed messages

Taking Webhook as an example:

1. Select the **Webhook** notification method
2. Fill in the Webhook URL (the endpoint that receives notifications)
3. Optionally set a Webhook Secret for signature verification
4. Click **Save Settings** to finish

Webhook push payloads are JSON, containing the event type, a timestamp, and detailed information.

## Pricing Settings

Controls whether calls to models without a set price are allowed.

### Configure the pricing policy

- **Accept unpriced models**: when enabled, all models can be called and unpriced models are billed at the default ratio; when disabled, only models with a set price can be called (recommended)

::: warning Important
Allowing unpriced models can lead to unexpectedly high consumption. We recommend keeping the default "reject unpriced models" setting.
:::

## IP Logging Settings

Controls whether the source IP address of API calls is recorded in the logs.

### Enable IP logging

- **Record IP address**: when enabled, you can see the source IP of each call on the Usage Logs page

IP logging helps with:

- Monitoring abnormal access
- Investigating security issues
- Analyzing traffic sources

## Delete Account

You can initiate account deletion at the bottom of the **Security** area. Before deleting, make sure you've handled any remaining balance and keys — once deleted, the data cannot be recovered.
