# User Management

User management is used to manage all user accounts on the platform, including profile viewing, permission management, quota adjustment and subscription management. Through user management, admins have full control over user status and entitlements.

Log in with an admin account, click **Users** in the left navigation, or visit `/console/user`.

## Quick Navigation

| Action | Description |
|------|------|
| [View a user](#view-user-details) | View detailed user information |
| [Edit a user](#edit-a-user) | Modify user information |
| [Quota adjustment](#quota-management) | Increase or decrease user quota |
| [Group change](#user-groups) | Change the user's permission group |
| [Subscription management](#user-subscriptions) | Manage user subscription status |

## Overview

![User list](/imgs/user-list.png)

The user list shows all registered users, including username, email, role, group, quota balance and status.

The user management page provides the following features:

| Feature | Description |
|------|------|
| **User list** | View all users and basic information |
| **Search users** | Search by username, email, etc. |
| **Edit users** | Modify user information |
| **Quota adjustment** | Increase, decrease or set user quota |
| **Subscription management** | View and adjust user subscriptions |
| **Enable/disable** | Control account status |
| **Delete users** | Remove user accounts |

## User Information

### Basic Fields

| Field | Description |
|------|------|
| **ID** | Unique user identifier, generated automatically |
| **Username** | Login name, set at registration |
| **Display name** | The user's display name, modifiable |
| **Email** | The user's email address |
| **User group** | The user's group, determines channel access |
| **Role** | The user's permission role, determines admin rights |
| **Status** | Current account status |

### Roles

| Role | Value | Permissions |
|------|:---:|----------|
| **Guest** | 0 | Visitor identity; can only browse public pages |
| **Common user** | 1 | Basic usage permissions; can use the API |
| **Ops** | 5 | Read-only ops view of the users they invited |
| **Admin** | 10 | Admin panel access; can manage common users |
| **Super admin (Root)** | 100 | Full permissions; manages all users and system settings |

::: warning Role allocation
- Keep the number of super admins strictly limited
- Assign the admin role as needed
- The ops role is a read-only view and cannot manage other users
- Common user is the default role
:::

### Status

| Status | Icon | Description | Impact |
|------|:----:|------|------|
| **Enabled** | 🟢 | Account healthy | All features work normally |
| **Disabled** | 🔴 | Account disabled | Cannot log in; API requests rejected |
| **Deleted** | ⚪ | Account deleted | Related data removed |

---

## Quota Management

### Quota Fields

![Edit user dialog](/imgs/user-edit.png)

| Field | Description |
|------|------|
| **Total quota** | The user's total quota (including used) |
| **Used quota** | Quota already consumed |
| **Remaining quota** | Quota still available |
| **Request count** | Cumulative API request count |

### Adjusting Quota

Admins can adjust a user's quota flexibly:

**Steps**:
1. Find the target user in the list
2. Click **Adjust Quota**
3. Choose an adjustment method
4. Enter the adjustment value
5. Confirm

**Adjustment methods**:

| Method | Description | Use case |
|------|------|----------|
| **Increase quota** | Adds the specified amount to the existing quota | Rewards, compensation, manual top-up |
| **Decrease quota** | Deducts the specified amount from the existing quota | Violation penalties, error correction |
| **Set quota** | Sets the quota directly to a value | Initialization, quota reset |

::: tip Quota adjustment records
Use the notes field to record the reason for each quota adjustment for later audit and reference.
:::

---

## User Groups

User groups control which channels a user can access — the core mechanism of permission management. See [Group Management](../group/index) for details.

### Changing a User's Group

1. Find the target user in the list
2. Click **Edit**
3. Modify the **Group** field
4. Save

::: warning Impact of group changes
Group changes take effect immediately and affect:
- The channel list the user can access
- The models visible in the user's Model Square
- The models the user can call
:::

---

## User Subscriptions

Admins can view and manage a user's subscription status.

### Viewing Subscription Status

| Information | Description |
|------|------|
| **Current subscription** | The user's active subscription plan |
| **Validity period** | The subscription's expiry date |
| **Subscription quota** | Quota included in the subscription and its reset cycle |

### Subscription Operations

| Action | Description |
|------|------|
| **View subscription** | View subscription details |
| **Cancel subscription** | Cancel the user's current subscription |
| **Manual renewal** | Manually renew or extend the subscription |

---

## How-To Guide

### View User Details

1. Find the target user in the list
2. Click the username or the **View** button
3. Review the detailed information:
   - Basic information
   - Quota usage
   - Request statistics
   - Login history
   - Subscription status

### Edit a User

1. Find the target user in the list and click the **Edit** button to open the dialog

![Edit user dialog](/imgs/user-edit.png)

2. Modify the information that needs updating:

| Field | Description |
|------|------|
| **Role** | Common user / Ops / Admin; takes effect immediately |
| **Group** | The channel group the user belongs to |
| **Quota balance** | Set the user's quota value directly |
| **Status** | Enable or disable the account; a disabled user cannot log in |

::: info Super admin note
**Super Admin (Root)** is outside the scope of this dialog: Root accounts cannot be assigned or demoted by other admins and can only manage themselves.
:::

3. Click **Save** to confirm

### Enable/Disable a User

1. Find the target user in the list
2. Click the status toggle or the **Disable/Enable** button
3. Confirm the operation

::: warning Impact of disabling
After a user is disabled:
- They immediately cannot log in
- API requests are rejected
- Existing tokens become invalid
- Subscription billing pauses (if applicable)
:::

### Delete a User

1. Find the target user in the list
2. Click the **Delete** button
3. Confirm the deletion

::: danger Deletion warning
Deleting a user **permanently removes** all of their data:
- Account information
- Quota records
- Request logs
- Subscription records
- Other related data

This operation is **irreversible** — be careful!
:::

---

## Search & Filter

### Search

1. Find the search box at the top of the user list
2. Enter a username or email keyword; the list filters in real time

![Filtered results after entering a keyword](/imgs/user-search.png)

Users can be searched by the following fields:

| Search field | Description |
|----------|------|
| Username | Exact or fuzzy match |
| Email address | Exact or fuzzy match |
| User ID | Exact match |

### Filters

The following filters are supported:

| Filter | Options |
|----------|------|
| Role | Common user / Ops / Admin / Super admin |
| Group | default / vip / custom groups |
| Status | Enabled / Disabled / Deleted |

---

## User Notes

Admins can attach notes to users:

| Property | Description |
|------|------|
| Visibility | Admin-only |
| Max length | 255 characters |
| Purpose | Management info, adjustment reasons, etc. |

---

## Third-Party Login Bindings

The user profile shows third-party login bindings:

| Field | Description | Binding condition |
|------|------|----------|
| **GitHub ID** | GitHub account binding | GitHub OAuth enabled in System Settings |
| **Discord ID** | Discord account binding | Discord OAuth enabled in System Settings |
| **WeChat ID** | WeChat account binding | WeChat login enabled in System Settings |
| **Telegram ID** | Telegram account binding | Telegram login enabled in System Settings |

::: info Third-party login configuration
Third-party login must be enabled in [System Settings](../system/index) first.
:::

---

## Invite System

Users can invite new users via invite links:

| Field | Description |
|------|------|
| **Invite code** | The user's personal invite code for generating invite links |
| **Invitee count** | Number of successfully invited users |
| **Invite quota** | Reward quota earned through invitations |

### How Invite Rewards Work

```text
Inviter shares invite link → New user registers → Both parties receive reward quota
```

::: tip Invite reward configuration
Invite rewards are configured in [System Settings - Operations](../system/index#operation-settings):
- Inviter reward quota
- Invitee reward quota
:::

---

## Common Scenarios

### Top-Up / Compensation

1. Find the target user
2. Click **Adjust Quota** → choose **Increase**
3. Enter the amount
4. Record the reason in the notes
5. Confirm

### Upgrading a User to VIP

1. Find the target user
2. Click **Edit**
3. Change the group to `vip`
4. Save

### Handling Rule Violations

1. Find the target user
2. Click **Disable**
3. Record the reason in the notes
4. Confirm

### Bulk Operations

For managing many users:
- Use search and filters to locate the target users
- Export the user list for batch processing
- Automate management via the API

---

## Notes

::: warning Permission management
- Be careful when granting Admin and Super Admin roles
- Audit admin accounts regularly
- Revoke permissions or disable accounts promptly for departing employees
:::

::: tip Quota management tips
- Record the reason in the notes for every quota adjustment
- Prefer the subscription system over manual adjustments for large amounts
- Watch for abnormal quota changes regularly
:::

::: info User data security
- User deletion is irreversible
- Back up necessary data before deleting
- Sensitive changes should have audit trails
:::

---

**Related documents**: [Subscription Management](../subscription/index) | [Channel Management](../channel/index) | [System Settings](../system/index)
