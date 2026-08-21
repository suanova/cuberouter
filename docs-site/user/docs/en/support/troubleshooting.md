# Troubleshooting

This guide provides troubleshooting steps and solutions for common issues to help you locate and resolve problems quickly.

> Error messages may appear in Chinese or English depending on your language setting; the following is organized by error type.

## Invalid API Key

When the request returns 401 or a key-invalid message:

1. Check that the API key was copied correctly, with no extra spaces or line breaks before or after
2. Confirm the key hasn't been disabled, deleted, or expired (check the status on the **API Keys** page)
3. Check the request header format: `Authorization: Bearer sk-xxx` (one space after `Bearer`)
4. Confirm the key's group has permission to use the target model

## Insufficient Quota

When the request returns 403 or an insufficient-quota message:

1. Check your current balance in **Wallet**
2. Top up when the balance is insufficient (online payment or redemption code)
3. If you use a subscription plan, confirm the subscription quota and its validity period

::: tip Deduction priority
When your account has both a balance and subscription quota, the platform deducts one of them first according to its configuration. You can learn about the deduction priority in **Subscription Plans**.
:::

## Rate Limit

When the request returns 429 or a too-many-requests message:

1. Reduce the request frequency, or add backoff intervals for retries
2. Check whether the rate limit comes from the upstream provider (some model vendors limit concurrency)
3. Confirm the rate limit configuration of the key's group and contact the administrator to adjust it if necessary

## Model Not Found

When the request returns 404 or a model-not-found message:

1. Check that the model name is exactly correct (case-sensitive, no vendor prefix)
2. Confirm on **Model Square** that the model is currently available
3. Confirm your group has permission to use the model

## Request Timeout or No Response

1. Check your network connection and API Base URL configuration
2. For long-running tasks (video generation, etc.), switch to the async task approach and poll the task status
3. Increase the client request timeout
4. If the issue persists, contact the administrator

## Streaming Response Interrupted

1. Check that the client correctly handles `finish_reason` and the stream-end marker
2. Confirm the network is stable and avoid the long connection being dropped by intermediate devices
3. Check the completion status and token breakdown of the request in **Usage Logs**

## Still Not Resolved

Please submit the issue via [Contact Support](../support/contact.md), attaching:

- The request ID from your usage logs
- The full error response content
- The key parameters of the reproduction request (redacted)
