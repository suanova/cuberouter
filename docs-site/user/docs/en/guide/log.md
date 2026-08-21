# Usage Logs

> View the details of every API call, with filtering by time, model, token, and more. Click **Usage Logs** in the left navigation, or go directly to `/usage-logs/common`. Regular users can only see their own call records.

![Usage Logs](/imgs-en/log.jpeg)

## View Usage Records

Each row in the log list represents one call record and includes:

| Column | Description |
| --- | --- |
| **Time** | Call time |
| **Type** | Streaming / non-streaming |
| **Model** | The name of the model called |
| **Token** | Input / output token counts |
| **Cost** | The quota consumed by this call |
| **First token** | Time to first token |
| **Total time** | Total request duration |
| **IP** | The client IP that made the request |
| **Channel** | The upstream channel that actually handled the request |
| **Token name** | The name of the API key used |
| **Actions** | View the details of this call |

Click **Details** in the **Actions** column to view the full request and response content of the call.

## Search & Filtering

### Set filter conditions

1. Set filter conditions in the filter toolbar at the top of the logs page:
   - **Time range**: select start and end dates
   - **Type**: filter by call type (streaming / non-streaming)
   - **Model name**: enter a model name keyword
   - **Group**: select the group of the API key
   - **Token name / username**: filter by key name or username
   - **Channel ID / request ID / upstream request ID**: pinpoint a specific call
2. When done, click **Query** to refresh the list with the filtered results; click **Reset** to clear the conditions

### View filtered results

Once configured, the list automatically refreshes with the filtered results.

## Usage Summary

Summary statistics are shown at the top of the list:

- **Usage**: total quota consumed within the filtered range
- **RPM**: requests per minute
- **TPM**: tokens per minute

## Data Statistics

### Access the data dashboard

Click **Dashboard** in the left navigation, or go directly to `/dashboard/models`, to view daily API call volume and quota consumption trends as line or bar charts. Hover over a chart to see detailed data for a specific date. See [Overview & Data Dashboard](../guide/dashboard.md).
