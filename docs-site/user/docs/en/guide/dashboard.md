# Overview & Data Dashboard

> After signing in you land on the **Overview** page by default, which brings together core information such as account usage, API info, and announcements.

## Overview Page

Click **Overview** in the left navigation, or go directly to `/dashboard/overview`.

![Overview page](/imgs-en/dashboard.jpeg)

The Overview page shows the following:

- **Setup guide**: a three-step guide — create an API key, add quota, send a request — with a curl call example
- **Usage at a glance**: last 24h usage, historical usage, request count, credit remaining
- **Performance health**: service performance metrics
- **API Info**: API address and documentation entry point, copyable directly
- **Announcements**: the latest system announcements
- **FAQ**: common questions you can expand to view

## Data Dashboard

Click **Dashboard** in the left navigation, or go directly to `/dashboard/models`, to view account usage in charts.

![Data dashboard](/imgs-en/data-dashboard.jpeg)

The data dashboard provides the following statistics:

### Usage statistics

- **Request count**: total number of API requests
- **Trend chart**: a line chart showing how request counts change over time

### Resource consumption

- **Quota consumed**: the total quota amount consumed
- **Tokens consumed**: total token consumption
- **Trend visualization**: line charts showing quota / token consumption trends

### Performance metrics

- **Avg RPM**: requests per minute
- **Avg TPM**: tokens per minute

### Model data analysis

Analyze model usage from multiple dimensions:

- **Consumption distribution**: consumption distribution across time periods (bar chart)
- **Consumption trend**: how model consumption changes over time (line chart)
- **Call count distribution**: each model's share of total call counts (donut chart)
- **Call count ranking**: models ranked by call count (bar chart)

Hover over a chart to see detailed data for a specific date.

## Core Data

| Module | Data item | Description |
| --- | --- | --- |
| Usage at a glance | Last 24h usage | Quota amount consumed in the last day |
| Usage at a glance | Historical Usage | Cumulative quota consumed |
| Usage at a glance | Request Count | Total number of API requests |
| Usage at a glance | Credit remaining | Available account balance |
| Usage statistics | Request count | Total number of API requests |
| Resource consumption | Quota consumed | Total quota amount consumed |
| Resource consumption | Tokens consumed | Total token consumption |
| Performance metrics | Avg RPM | Requests per minute |
| Performance metrics | Avg TPM | Tokens per minute |

## Core Value

1. **Financial transparency**: view your balance and consumption history in real time
2. **Usage insights**: quantify API calls, token consumption, and performance metrics
3. **Trend analysis**: identify usage patterns and anomalies through multi-dimensional charts
4. **Model analysis**: understand each model's call frequency and consumption share
