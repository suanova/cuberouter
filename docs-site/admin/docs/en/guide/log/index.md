# Logs & Statistics

Logs & Statistics lets you review platform-wide API call records and quota consumption, with multi-dimensional analysis by user, model, channel and more. Log in with an admin account, click **Logs** in the left navigation, or visit `/console/log`.

## Quick Navigation

| Action | Description |
|------|------|
| [Log list](#overview) | Review platform-wide call records |
| [Search & filter](#search--filter) | Multi-dimensional log filtering |
| [Statistics panel](#statistics-panel) | Platform-wide call summaries |
| [Consumption trends](#platform-wide-consumption-trends) | Trends and user distribution |

## Overview

![Platform-wide log list](/imgs/admin-log.png)

The admin log list has two extra columns compared to the user view — **Username** and **Channel** — and shows every user's call records.

The log list shows the details of each API call:

| Field | Description |
|------|------|
| **User** | The caller's username (admins see all users) |
| **Channel** | The channel that handled the request (admins only) |
| **Token** | The API token used for the call |
| **Model** | The requested model |
| **Group** | The channel group the token belongs to |
| **Prompt/completion tokens** | Input and output token counts |
| **Cost** | Quota consumed by this call |
| **Response time** | Request latency |
| **Status** | Request result (success / failure and reason) |

## Search & Filter

1. Click the **Filter** button at the top of the log page to expand the filter area
2. Set the following conditions:
   - **Time range**: start and end dates
   - **Username**: username keyword
   - **Model**: model name keyword
   - **Channel**: a specific channel
   - **Token name**: token name keyword
3. Click **Search**; the list refreshes with the filtered results

![Expanded filter area](/imgs/admin-log-filter.png)

## Statistics Panel

The top of the log page shows a statistics summary of platform-wide call volumes.

![Statistics panel (total calls, total quota consumed, etc.)](/imgs/admin-log-stat.png)

The panel includes the following metrics:

| Metric | Description |
|------|------|
| **Total calls** | Total API calls in the time range |
| **Total quota consumed** | Total quota in the time range |
| **Total tokens** | Input + output tokens |
| **Success rate** | Share of successful requests |

## Platform-Wide Consumption Trends

1. Click **Dashboard** in the left navigation, or visit `/console` (admin view)

![Platform-wide consumption trend chart](/imgs/admin-dashboard.png)

2. The admin dashboard shows a platform-wide consumption trend line chart and each user's share of consumption
3. Hover over the chart to see detailed data for a specific date

## Usage Logs

Regular users see their own call records on the usage log page; admins see **all users'** logs:

![Usage log](/imgs/usage-log.png)

The usage log shows the token group, model and cost of each API call.

## Task Logs

Task logs show records of asynchronous generation tasks such as Suno:

![Task log](/imgs/task-log.png)

## Drawing Logs

Drawing logs show records of drawing tasks such as Midjourney:

![Drawing log](/imgs/drawing-log.png)

## Notes

::: tip Log operations tips
- Archive or clean up logs periodically as data volume grows
- Combine with the log retention setting in [System Settings - Other](../system/index#other-settings) to control storage
- Use filters first when troubleshooting to locate target requests
:::

::: warning Log visibility
- Regular users only see their own call records
- Admins see platform-wide logs (including username and channel columns)
- Manage sensitive information in logs per compliance requirements
:::

---

**Related documents**: [Dashboard](../system/index#dashboard-settings) | [System Settings](../system/index)
