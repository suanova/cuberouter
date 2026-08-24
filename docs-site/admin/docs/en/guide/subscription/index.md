# Subscription Management

Subscription management is used to create and manage subscription plans. After purchasing a plan, users get service entitlements for a period, including quota and user group upgrades.

Log in with an admin account and visit `/console/subscription` directly (the admin view differs from the user purchase view).

## Quick Navigation

| Action | Description |
|------|------|
| [Create a plan](#creating-a-plan) | Create a new subscription plan |
| [Edit a plan](#editing-a-plan) | Modify plan configuration |
| [Publish / unpublish](#publish--unpublish-plans) | Control plan visibility |
| [Payment integration](#payment-integration) | Stripe and other payment configuration |
| [Quota settings](#quota-settings) | Quota and reset cycles |

## Overview

![Subscription plan list](/imgs/sub-admin-list.png)

The plan list shows all created plans, including name, price, validity period and status (published / unpublished).

The subscription management page provides the following core features:

| Feature | Description |
|------|------|
| **Plan list** | View all plans and details |
| **Create plan** | Create a new plan |
| **Edit plan** | Modify an existing plan |
| **Publish/unpublish** | Control plan availability |
| **Sorting** | Adjust display order |

## Plan Configuration

### Basic Information

![Create plan dialog](/imgs/sub-create.png)

| Field | Required | Description |
|------|:----:|------|
| **Plan title** | ✓ | Display name, e.g. "Monthly member", "Annual member" |
| **Plan subtitle** | | Supplementary description, e.g. "For individual users" |
| **Price** | ✓ | Subscription price |
| **Currency** | ✓ | Price currency, USD by default |
| **Status** | | Whether the plan is enabled |

::: tip Naming tips
- **Keep it simple**: e.g. "Monthly Basic", "Annual Pro"
- **Highlight value**: e.g. "Unlimited Monthly Card", "Premium Annual Membership"
- **Stay consistent**: keep a uniform style across plans
:::

### Validity Period

Plans support multiple validity units:

| Unit | Description | Use case |
|------|------|----------|
| **Hour** | Billed by the hour | Temporary trials, short-term testing |
| **Day** | Billed by the day | Short campaigns, trial plans |
| **Month** | Billed by the month | Standard subscription period |
| **Year** | Billed by the year | Long-term memberships, discounted plans |
| **Custom** | Custom seconds | Special period requirements |

**Examples**:

| Plan | Unit | Value | Actual validity |
|------|:----------:|:----:|:----------:|
| Trial | Day | 3 | 3 days |
| Monthly member | Month | 1 | 30 days |
| Quarterly member | Month | 3 | 90 days |
| Annual member | Year | 1 | 365 days |

::: warning Validity calculation
- A month counts as 30 days
- A year counts as 365 days
- For custom periods, enter seconds directly (e.g. 86400 = 1 day)
:::

### Quota Settings

Quota is the unit that measures AI service usage; a plan can include quota for the user.

| Field | Description |
|------|------|
| **Total quota** | Quota included in the plan; 0 means unlimited |
| **Quota reset cycle** | How often the quota resets |

**Reset cycle options**:

| Option | Description | Use case |
|------|------|----------|
| **No reset** | Runs out and stays out; no automatic recovery | One-time quota packs |
| **Daily** | Resets at 00:00 daily | High-frequency usage |
| **Weekly** | Resets every Monday | Recurring needs |
| **Monthly** | Resets on the 1st of each month | Standard subscription |
| **Custom** | Reset after a specified interval in seconds | Special needs |

::: info Reset behavior
- Reset restores quota to the initial value — it does not accumulate
- Unused quota does not carry over
- Reset time is based on the server timezone
:::

### User Group Upgrade

A plan can upgrade the user's group automatically after purchase:

| Setting | Description |
|------|------|
| **Upgrade group** | The group to upgrade to after purchase |
| **Leave empty** | No group upgrade; keep the current group |

::: tip Groups and channel access
The user group determines which channels a user can access. Upgrading groups via subscription enables:
- Common user → VIP user (unlock advanced models)
- Trial user → Full user (unlock more features)
:::

### Purchase Limits

| Field | Description |
|------|------|
| **Purchase limit per user** | Max purchases per user; 0 means unlimited |

**Use cases**:

| Limit | Use case |
|----------|----------|
| 0 (unlimited) | Standard plans; multiple purchases extend validity |
| 1 | Limited-time offers, new-user exclusives |
| N | Promotions that cap purchases |

---

## Payment Integration

Plans can integrate with multiple payment platforms for automated purchase. Platform API key configuration is covered in [System Settings - Payment](../system/index#payment-settings).

### Stripe Integration

Stripe is a mainstream international payment platform supporting credit cards and more.

**Setup steps**:

1. Create a product and price in Stripe
2. Get the Price ID (format: `price_xxxxx`)
3. Enter the **Stripe Price ID** in the plan
4. Make sure the Stripe API keys are configured in System Settings

::: details How to get a Stripe Price ID
1. Log in to the [Stripe Dashboard](https://dashboard.stripe.com/)
2. Go to the Products page
3. Select the product
4. Find the Price ID in the Pricing section
:::

### Other Payment Methods

The platform also supports **EPay (Yipay)** and other aggregated payment gateways — see [System Settings - Payment](../system/index#payment-settings) for details.

::: warning Payment configuration order
1. Configure the payment platform's API in System Settings first
2. Link the corresponding price/product ID in the plan next
3. Run a test payment last to verify everything works
:::

---

## How-To Guide

### Creating a Plan

1. Click **Create Plan** on the subscription management page to open the dialog
2. Fill in the basic information
   - Plan title and subtitle
   - Price and currency
3. Configure the validity period
   - Choose the unit
   - Set the value
4. Configure quota
   - Set total quota
   - Choose the reset cycle
5. Configure other options
   - User group upgrade
   - Purchase limits
   - Payment integration ID
6. Click **Submit**; the plan is created and unpublished by default

### Publish / Unpublish Plans

1. Find the target plan and click the **Publish** or **Unpublish** button

![Publish/unpublish operation](/imgs/sub-publish.png)

2. When published, users can see and buy the plan on the subscription page; when unpublished, the plan is hidden but existing subscriptions are unaffected

### Editing a Plan

1. Find the target plan in the list
2. Click **Edit**
3. Modify the configuration
4. Click **Save** to confirm

::: warning Impact of changes
- Changes apply to **new purchases** going forward
- **Existing subscribers** are unaffected and keep their original terms
- Adjusting entitlements of existing subscribers must be done manually via User Management
:::

### Sorting

- Set a sort value per plan; **higher values appear first**
- The user side displays plans ordered by sort value
- Give recommended plans a high sort value

**Sorting suggestions**:

| Plan | Sort value | Position |
|------|:------:|:--------:|
| Annual member (recommended) | 100 | 1st |
| Quarterly member | 50 | 2nd |
| Monthly member | 10 | 3rd |
| Trial | 1 | 4th |

---

## Managing User Subscriptions

### Admin View

Admins can view a user's subscription status on the user management page:

| Status | Description |
|------|------|
| 🟢 **Active subscription** | Currently valid |
| ⚪ **Expired subscription** | Past subscriptions in history |

### Manually Activating a Subscription for a User

1. Click **Manual Bind** on the subscription management page
2. Enter the target user's username or email and select a plan
3. Click **Confirm**; the system creates the subscription record, effective immediately

### User Side

Users can do the following on the subscription page:

| Feature | Description |
|------|------|
| View plans | Browse available plans |
| Buy a subscription | Select a plan and pay |
| View status | See the current subscription and validity |
| Quota check | View remaining quota |

---

## Example Plans

### Basic Plan

| Item | Value |
|--------|-----|
| Title | Monthly Basic |
| Price | $9.99 USD |
| Validity | 1 month |
| Total quota | 100,000 |
| Reset cycle | None |
| Group upgrade | basic |

### Premium Plan

| Item | Value |
|--------|-----|
| Title | Annual Pro |
| Price | $99.99 USD |
| Validity | 1 year |
| Total quota | 2,000,000 |
| Reset cycle | Monthly |
| Group upgrade | vip |
| Purchase limit | 1 |

### Trial Plan

| Item | Value |
|--------|-----|
| Title | 7-day Trial |
| Price | $1.99 USD |
| Validity | 7 days |
| Total quota | 10,000 |
| Reset cycle | None |
| Purchase limit | 1 |

---

## FAQ

::: details How do I set up a trial period?
Create a plan priced at 0 with a short validity period (e.g. 7 days) and a purchase limit of 1.
:::

::: details What happens when a subscription expires?
- The group reverts to the default group (if the subscription included an upgrade)
- Quota stops resetting; remaining quota can still be used
- The user can renew or buy a new plan
:::

::: details How does renewal work?
Buying the same plan again while the subscription is active extends the validity automatically.
:::

---

## Notes

::: warning Pricing
- Make sure the price matches the payment platform configuration
- The currency must be supported by the payment platform
- When changing a price, update the payment platform's price ID too
:::

::: tip Quota planning
- Calculate sensible quota based on model pricing
- Consider user habits when choosing a reset cycle
- Offer multiple quota tiers for different needs
:::

::: danger Unpublishing impact
- Once unpublished, **new users cannot buy** the plan
- **Existing subscribers are unaffected** and keep their entitlements
- For a full retirement, announce a transition period to users
:::

---

**Related documents**: [User Management](../user/index) | [System Settings - Payment](../system/index#payment-settings)
