# API Key Management

> A key (token) is the credential for API calls. Each key can be configured independently with its own permission scope and quota limit.

::: warning Important
An API key is the credential for calling the CubeRouter service. Keep it safe and never share it with others.
:::

Click **API Keys** in the left navigation, or go directly to `/keys`.

![API key list](/imgs-en/token-list.jpeg)

## What is an API Key?

An **API key** is a unique identifier used for:

- 🔐 **Authentication**: proving that you have the right to use the CubeRouter service
- 📊 **Usage tracking**: recording each key's usage and spending
- 🎯 **Access control**: limiting which models each key can access
- 💰 **Billing management**: billing based on usage

Each key acts like a "virtual account" with independently managed permissions and billing.

## Key List

The core of the API Keys page is the key list, which shows all created keys.

### List fields

| Field | Description |
| --- | --- |
| Name | A custom name for the key, to identify its purpose |
| Status | The key's current status (Enabled / Disabled) |
| API Key | The key string (copyable, show / hide) |
| Quota | The quota consumed by this key |
| Group | The channel group the key belongs to |
| Models | The model restrictions for the key |
| IP Restriction | The key's IP whitelist settings |
| Created | When the key was created |
| Last Used | When the key was last used |
| Expires | The key's expiration time |
| Actions | Available operations (disable / edit / delete, etc.) |

### Empty state

When no keys have been created, the page shows a friendly hint suggesting you create your first key to get started.

## Create a Key

Click the **Create API Key** button to create a new API key.

![Create API Key](/imgs-en/token-create.jpeg)

### Basic configuration

1. Click the **Create API Key** button
2. Fill in the key **Name**: name it by purpose (e.g. `prod`, `dev-test`, `team-member-name`)
3. Select a **Group**: different groups can access different models
4. Set **Expires at**: choose "never expire" or a preset (1 month / 1 day / 1 hour)
5. Set **Count**: create multiple keys at once (default is 1)
6. Enable **Unlimited Quota** to bypass quota limits; otherwise set the maximum quota available to this key
7. Click **Create**

### Advanced settings

Expand **Advanced Settings** to configure the following:

| Setting | Description |
| --- | --- |
| Model restrictions | Limit the key to calling specific models only; leave empty for no restriction |
| IP whitelist | Limit which source IPs may use the key (CIDR format); leave empty for no restriction |

### Save the key

After clicking **Create**, a dialog displays the full key — **copy and save it immediately**. Once the dialog is closed, the full key cannot be viewed again.

::: warning Important
The full key is displayed only once at creation — copy and save it immediately. The key grants full API access: never share it with others, and never commit it to a code repository or share it publicly.
:::

## Key Operations

Every key supports the following operations:

### Copy the key

- Copy the full API key with one click
- Makes it easy to drop into your app configuration quickly
- You get confirmation feedback after a successful copy

### Show / hide the key

- Keys are hidden by default (shown as `sk-****...****`)
- Click the eye icon to view the full key
- Click again to hide it

### Edit the key

- Change the key name
- Adjust the quota limit
- Update model restrictions
- Change the expiration time

### Disable / enable the key

- **Disable**: temporarily suspend the key without affecting data
- **Enable**: re-enable a disabled key
- Use cases: debugging, security control, temporary restriction

### Delete the key

- Permanently delete the key
- Deletion is irreversible
- Apps using the key will lose API access

## Quota Management

### Quota types

Each key has two kinds of quota:

- **Used quota**: the total quota the key has consumed so far
- **Remaining quota**: the quota the key can still use

### Quota limits

You can set a quota limit for each key:

- **Unlimited**: use the total account balance
- **Fixed quota**: set a specific quota limit
- **Real-time monitoring**: the key is automatically suspended when the limit is exceeded

## Status Explained

| Status | Description | Available |
| --- | --- | --- |
| Enabled | The key works normally and can call the API | ✅ Available |
| Disabled | The key is temporarily suspended | ❌ Not available |
| Expired | The key is past its validity period | ❌ Not available |
| Out of quota | The key's quota has been used up | ❌ Not available |

## Use Cases

### Scenario 1: Personal development

1. Create a key for development and testing
2. Set a small quota limit
3. Disable or delete it after development is done

### Scenario 2: Production environment

1. Create a dedicated production key
2. Configure an appropriate quota limit
3. Set an IP whitelist for better security
4. Monitor usage regularly

### Scenario 3: Team collaboration

1. Create an independent key for each team member
2. Easily track each person's usage
3. Delete the corresponding key when a member leaves

### Scenario 4: Multi-project management

1. Create an independent key for each project
2. Control each project's budget separately
3. Delete the corresponding key when a project ends
4. Clear cost allocation

## Best Practices

### Security recommendations

1. **Naming conventions**: use clear names to identify each key's purpose (e.g. `prod-api-v1`, `dev-test`)
2. **Regular rotation**: rotate production API keys regularly; every 3–6 months is a good practice
3. **Least privilege**: grant only the necessary permissions and limit the accessible model scope
4. **Monitoring & alerts**: set alerts on quota usage so you're notified as you approach the limit

### Usage recommendations

1. **Environment isolation**: use different keys for different environments (dev, test, production separated)
2. **Quota control**: set reasonable quota limits for each key to avoid unexpected overspending
3. **Timely cleanup**: regularly clean up keys that are no longer in use
4. **Keep records**: keep key information safe and record each key's purpose and creation time

## FAQ

### Q1: What if I forgot an API key?

An API key is only shown once at creation. If you've lost it, delete the old key, create a new one, and update your app configuration.

### Q2: What if a key's quota runs out?

Two options: edit the key to raise its quota limit, or create a new key and update your app configuration.

### Q3: How do I see a key's usage details?

Go to the **Usage Logs** page to view detailed call records for each key, filter logs by key, and view consumption statistics.

### Q4: Can multiple apps share one key?

Yes, but it's not recommended. Use an independent key per app for easier permission management, troubleshooting, and cost allocation.

### Q5: What if a key leaks?

Immediately disable or delete the leaked key, create a new one, update all app configurations, and review usage records for anomalies.

## Core Value

1. **Authentication**: secure access to the CubeRouter service
2. **Access management**: fine-grained access control
3. **Cost control**: independent quota management and monitoring
4. **Multi-project management**: independent management for multiple projects
5. **Team collaboration**: easy collaboration and permission distribution
6. **Security auditing**: complete usage records and traceability
