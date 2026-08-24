# Group Management

Groups isolate channel access permissions and billing ratios for different users — the core mechanism of permission management. Users in different groups can only use the channels assigned to their group. Log in with an admin account and find the **Groups** entry in System Settings or the admin panel.

The group list shows all configured group names. Users and tokens can be assigned to a group, and channels can be restricted to specific groups.

## Quick Navigation

| Action | Description |
|------|------|
| [How groups are used](#how-groups-are-used) | User, token and channel dimensions |
| [Built-in groups](#built-in-groups) | Default group and custom groups |
| [Groups & channel access](#groups--channel-access) | How access control works |
| [Automatic grouping](#automatic-grouping) | Auto-assigning new users |

## How Groups Are Used

- **User group**: assign a group to a user when editing them in User Management
- **Token group**: assign a channel group to a token when creating it
- **Channel group**: enter the group names allowed to access a channel in the **Group** field when adding a channel

::: info Automatic token grouping
When a token's group is set to `auto`, the system automatically picks an available group by priority order — ideal for cross-group failover scenarios.
:::

## Built-in Groups

| Group | Description | Typical use |
|--------|------|----------|
| **default** | Default group | Initial group for all new users; basic permissions |
| **vip** | VIP group | Paid or special users; advanced permissions |
| **Custom groups** | Created by admins | Fine-grained permissions by business need |

## Groups & Channel Access

The user group determines which channels a user can access:

```text
User group ──linked to──▶ Channel ──contains──▶ Models
```

::: info Permission logic
1. A channel specifies the groups allowed to access it
2. When the user's group matches the channel's groups
3. The user can call models through that channel

For example:
- User A belongs to the `default` group
- Channel X is configured for the `vip` group
- User A cannot call models through channel X
:::

### Changing a User's Group

1. Find the target user in the user list
2. Click **Edit**
3. Modify the **Group** field
4. Save

::: warning Impact of group changes
Group changes take effect immediately and affect:
- The channel list the user can access
- The models visible in the user's Model Square
- The models the user can call
:::

## Group Ratios

Group ratios let you set differentiated billing multipliers per user group for flexible pricing:

```json
{
  "vip": 0.5,
  "premium": 0.8,
  "standard": 1.0,
  "trial": 2.0
}
```

Group ratios multiply with model ratios:

```text
Final cost = Token count × Model ratio × Group ratio
```

**Group ratio precedence**:

1. Per-user ratio: a personal ratio set for a specific user
2. Group ratio: the ratio of the user's group
3. Default ratio: the system default (usually 1.0)

See [System Settings - Billing & Ratios](../system/index#billing-ratios) for configuration details.

## Automatic Grouping

Controls the default group behavior for new users:

| Setting | Description | Default |
|--------|------|--------|
| **Use automatic grouping by default** | Whether new users are grouped automatically | Off |
| **Automatic group list** | The groups available for automatic assignment | `["default"]` |

::: info How automatic grouping works
Automatic grouping assigns users to specific groups based on rules such as invite codes or registration sources. After configuration, qualifying users join the specified group automatically.
:::

## Notes

::: warning Permission planning
- Group permission changes affect every member of the group — evaluate the impact before changing anything
- Create separate groups for different business scenarios to avoid mixing permissions
- Review channel grants per group regularly and clean up stale configuration
:::

---

**Related documents**: [Channel Management](../channel/index) | [User Management](../user/index) | [System Settings](../system/index)
