# Redemption Code Management

Redemption code management is used to generate and manage quota redemption codes in batches, for campaign giveaways or user top-ups. Through redemption codes, admins can run marketing campaigns and user incentives flexibly.

Log in with an admin account, click **Redemption Codes** in the left navigation, or visit `/console/redemption`.

::: warning Compliance notice
Redemption code usage must stay within the authorized scope, platform rules and local laws and regulations. Redemption codes are only for internal testing, authorized customer delivery, campaign giveaways or accounting adjustments.
:::

## Quick Navigation

| Action | Description |
|------|------|
| [Create codes](#creating-codes) | Generate codes in batches |
| [Edit codes](#editing-codes) | Modify code configuration |
| [Code status](#code-status) | Status descriptions and management |
| [User redemption flow](#user-redemption-flow) | What users do |

## Overview

![Redemption code list](/imgs/redemption-list.png)

The list shows all generated codes, including the code value (partially masked), face value, usage status (unused / used) and creation time.

The redemption code management page provides the following features:

| Feature | Description |
|------|------|
| **Code list** | View all codes and their status |
| **Create codes** | Generate codes in batches |
| **Edit codes** | Modify code information |
| **Delete codes** | Remove unwanted codes |
| **Search codes** | Search by name or ID |

## Code Configuration

### Basic Information

![Generate code dialog](/imgs/redemption-create.png)

| Field | Required | Description |
|------|:----:|------|
| **Name** | ✓ | Identifier for the batch |
| **Quota** | ✓ | Quota value each code carries |
| **Quantity** | ✓ | Number of codes to generate |
| **Expiry** | | Validity period of the codes |

### Expiry Settings

| Option | Description | Use case |
|------|------|----------|
| **Never expires** | Codes valid indefinitely | Long-term campaigns, reserve quota |
| **Custom time** | Set a specific expiry time | Limited-time campaigns, promotions |

::: warning Expired codes
Expired codes can no longer be used and show as **Expired**. Filter them in the list and clean them up periodically.
:::

### Batch Generation Rules

After setting a quantity, the system generates that many codes:

- Each code has a **unique code Key**
- Codes in the same batch share the same name, quota and expiry
- Code Keys are random strings generated automatically

**Example**:

| Name | Quota | Quantity | Result |
|------|:----:|:----:|----------|
| New-user gift | 10,000 | 50 | 50 Keys, 10,000 quota each |
| Campaign reward | 5,000 | 100 | 100 Keys, 5,000 quota each |

---

## Code Status

| Status | Icon | Description | Operations |
|------|:----:|------|:------:|
| **Unused** | 🟢 | Available, waiting to be redeemed | Edit, delete, copy |
| **Used** | 🔵 | Already redeemed | View, delete |
| **Expired** | 🔴 | Past the expiry date | Delete |

::: tip Automatic status changes
- Status changes to **Used** automatically after a successful redemption; used codes cannot be restored
- Status changes to **Expired** automatically when the expiry time passes; expired codes can be reactivated by changing their expiry
:::

---

## How-To Guide

### Creating Codes

1. Click the **Generate Code** button on the list page to open the dialog
2. Fill in the code information:
   - **Name**: a batch name that's easy to recognize
   - **Face value**: the quota each code carries
   - **Quantity**: how many codes to generate this time
3. Click **Generate**; the system creates the batch and shows it in the list

![Code list after generation](/imgs/redemption-created.png)

::: tip Naming tips
Use meaningful names for easier management later:
- By campaign: `Double-11 campaign`, `New-user gift`
- By channel: `Official site`, `WeChat promo`
- By tier: `Trial quota`, `VIP reward`
:::

### Editing Codes

1. Find the target code in the list
2. Click **Edit**
3. Modify the configuration:
   - Name
   - Quota
   - Expiry
4. Click **Save** to confirm

::: warning Edit restrictions
- **Used** codes cannot be edited
- **Expired** codes can be reactivated by changing their expiry
:::

### Deleting Codes

1. Find the target code in the list
2. Click **Delete**
3. Confirm the deletion

::: danger Deletion warning
Deletion is **irreversible** — be careful. Recommendations:
- Keep used codes for a while as records
- Clean up expired codes in batches periodically
:::

### Copying / Exporting Codes

- Click **Copy** to copy the code Key to the clipboard
- Click **Export** to download the codes as a file for bulk distribution

---

## User Redemption Flow

### User Side

Users redeem quota as follows:

1. Log in to their account
2. Go to **Profile** → **Redemption Center**
3. Enter the code Key
4. Click **Redeem**
5. The system validates and credits the quota

### Redemption Rules

| Rule | Description |
|------|------|
| Single use | Each code can only be used once |
| No reuse | Used codes cannot be redeemed again |
| Expiry limit | Expired codes cannot be used |
| Instant credit | Quota is credited immediately after redemption |

### Redemption Failures

| Error message | Cause | Solution |
|----------|------|----------|
| Code does not exist | Wrong Key entered | Check that the copy is correct |
| Code already used | Already redeemed | Use another code |
| Code expired | Past the validity period | Contact an admin for a new code |

---

## Search & Filter

### Search

Search codes by:

| Search method | Description |
|----------|------|
| By name | Fuzzy match on the batch name |
| By ID | Exact match on the code ID |
| By Key | Match on the code Key |

### Status Filter

Filter by status label:

- All
- Unused
- Used
- Expired

---

## Statistics

The top of the code list shows a statistics overview:

| Statistic | Description |
|--------|------|
| **Total** | Total number of codes |
| **Used** | Number redeemed |
| **Unused** | Number available |
| **Expired** | Number expired |

---

## Use Cases

### Marketing Campaigns

| Scenario | Configuration suggestion |
|------|----------|
| New-user registration | Name: `New-user gift`, moderate quota, large quantity |
| Promotions | Set an expiry time aligned with the campaign |
| Partner promotion | Name by channel for tracking |

### User Incentives

| Scenario | Configuration suggestion |
|------|----------|
| Invite rewards | Name: `Invite reward`, higher quota |
| Campaign prizes | Limited quantity, set expiry |
| Compensation | Generate individually, note the reason |

### Testing

| Scenario | Configuration suggestion |
|------|----------|
| Feature testing | Name: `Test`, small quota |
| Development/debug | No expiry, generate fresh codes for testing (each code is still single-use) |

---

## Notes

::: warning Safe keeping
Code Keys have value — keep them safe:
- Don't share codes publicly
- Distribute through private channels
- Check unused code status regularly
:::

::: tip Batch management tips
- Use meaningful names
- Clean up used and expired codes regularly
- Generate large distributions in batches
:::

::: info Quota math
Code quota uses the same unit as system quota. After redemption, the quota is **added to** the user's current balance, not replacing it.
:::

---

**Related documents**: [User Management](../user/index) | [Subscription Management](../subscription/index)
